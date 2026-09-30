package products

import (
	"context"

	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/DoSafills/SOLOServis/backend/internal/products/dto"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MaxRowLimit is the largest page size a client may request. It bounds the work
// a single listing request can force on the database.
const MaxRowLimit int32 = 100

// ListFilter narrows and paginates a product listing. Empty or zero fields are
// ignored, so the zero value reproduces the unfiltered, unpaginated listing.
type ListFilter struct {
	Search   string
	Category string
	Brand    string
	Limit    int32
	Offset   int32
}

// Repository is the persistence port of the products module. The handler
// depends on this interface rather than on the concrete implementation so the
// transport layer can be tested without PostgreSQL.
type Repository interface {
	// List returns the active products matching the filter, ordered by id.
	List(ctx context.Context, filter ListFilter) ([]generated.ListProductsFilteredRow, error)

	// ListImages returns the active images of several products in one query.
	ListImages(ctx context.Context, productIDs []int32) ([]generated.ProductImage, error)

	// ListOffers returns the offers of several products in one query, ordered
	// by price within each product.
	ListOffers(ctx context.Context, productIDs []int32) ([]generated.ListProductOffersByProductIDsRow, error)

	// GetDetailByPublicID returns a product joined with its brand, category and
	// rating summary. It returns pgx.ErrNoRows when absent.
	GetDetailByPublicID(ctx context.Context, publicID pgtype.UUID) (generated.GetProductDetailByPublicIDRow, error)

	// ListSpecifications returns the category specifications of one product.
	ListSpecifications(ctx context.Context, productID int32) ([]generated.ListProductSpecificationsRow, error)

	// ListPriceHistory returns the price history of one product, oldest first.
	ListPriceHistory(ctx context.Context, productID int32) ([]generated.ListProductPriceHistoryRow, error)

	// Create inserts a product and its first image in a single transaction.
	Create(ctx context.Context, request dto.CreateProductRequest) (dto.CreatedProduct, error)

	// Update replaces the editable fields of an active product. It returns
	// pgx.ErrNoRows when the product does not exist or is already inactive.
	Update(ctx context.Context, publicID pgtype.UUID, request dto.UpdateProductRequest) (generated.Product, error)

	// Deactivate soft deletes an active product, returning the stored row.
	// It returns pgx.ErrNoRows when the product does not exist or is already
	// inactive.
	Deactivate(ctx context.Context, publicID pgtype.UUID) (generated.Product, error)
}

// repository is the PostgreSQL implementation of Repository.
type repository struct {
	db      *pgxpool.Pool
	queries *generated.Queries
}

// NewRepository returns a Repository backed by the given connection pool.
func NewRepository(db *pgxpool.Pool) Repository {
	return &repository{
		db:      db,
		queries: generated.New(db),
	}
}

func (r *repository) List(ctx context.Context, filter ListFilter) ([]generated.ListProductsFilteredRow, error) {
	params := generated.ListProductsFilteredParams{
		Search:   nullableText(filter.Search),
		Category: nullableText(filter.Category),
		Brand:    nullableText(filter.Brand),
	}

	// A NULL limit means "no limit" in PostgreSQL, which keeps the previous
	// behaviour for clients that do not paginate.
	if filter.Limit > 0 {
		params.RowLimit = pgtype.Int4{Int32: filter.Limit, Valid: true}
	}

	if filter.Offset > 0 {
		params.RowOffset = pgtype.Int4{Int32: filter.Offset, Valid: true}
	}

	rows, err := r.queries.ListProductsFiltered(ctx, params)
	if err != nil {
		return nil, err
	}

	return rows, nil
}

func (r *repository) ListImages(ctx context.Context, productIDs []int32) ([]generated.ProductImage, error) {
	if len(productIDs) == 0 {
		return nil, nil
	}

	return r.queries.ListProductImagesByProductIDs(ctx, productIDs)
}

func (r *repository) ListOffers(ctx context.Context, productIDs []int32) ([]generated.ListProductOffersByProductIDsRow, error) {
	if len(productIDs) == 0 {
		return nil, nil
	}

	return r.queries.ListProductOffersByProductIDs(ctx, productIDs)
}

func (r *repository) GetDetailByPublicID(ctx context.Context, publicID pgtype.UUID) (generated.GetProductDetailByPublicIDRow, error) {
	return r.queries.GetProductDetailByPublicID(ctx, publicID)
}

func (r *repository) ListSpecifications(ctx context.Context, productID int32) ([]generated.ListProductSpecificationsRow, error) {
	return r.queries.ListProductSpecifications(ctx, productID)
}

func (r *repository) ListPriceHistory(ctx context.Context, productID int32) ([]generated.ListProductPriceHistoryRow, error) {
	return r.queries.ListProductPriceHistory(ctx, productID)
}

func (r *repository) Create(ctx context.Context, request dto.CreateProductRequest) (dto.CreatedProduct, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return dto.CreatedProduct{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	queries := r.queries.WithTx(tx)

	product, err := queries.CreateProduct(ctx, generated.CreateProductParams{
		CategoryID:  request.CategoryID,
		BrandID:     nullableInt4(request.BrandID),
		Name:        request.Name,
		Model:       nullableText(request.Model),
		Sku:         nullableText(request.SKU),
		Description: nullableText(request.Description),
	})
	if err != nil {
		return dto.CreatedProduct{}, err
	}

	const insertImage = `
		INSERT INTO product_image (product_id, image_url, alt_text, sort_order)
		VALUES ($1, $2, $3, 0)
	`

	if _, err := tx.Exec(ctx, insertImage, product.ID, request.ImageURL, request.Name); err != nil {
		return dto.CreatedProduct{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return dto.CreatedProduct{}, err
	}

	return dto.CreatedProduct{
		ID:       product.PublicID.String(),
		Name:     product.Name,
		ImageURL: request.ImageURL,
	}, nil
}

func (r *repository) Update(ctx context.Context, publicID pgtype.UUID, request dto.UpdateProductRequest) (generated.Product, error) {
	return r.queries.UpdateProductByPublicID(ctx, generated.UpdateProductByPublicIDParams{
		PublicID:    publicID,
		CategoryID:  request.CategoryID,
		BrandID:     nullableInt4(request.BrandID),
		Name:        request.Name,
		Model:       nullableText(request.Model),
		Sku:         nullableText(request.SKU),
		Description: nullableText(request.Description),
	})
}

func (r *repository) Deactivate(ctx context.Context, publicID pgtype.UUID) (generated.Product, error) {
	return r.queries.DeactivateProductByPublicID(ctx, publicID)
}

func nullableInt4(value *int32) pgtype.Int4 {
	if value == nil {
		return pgtype.Int4{}
	}

	return pgtype.Int4{Int32: *value, Valid: true}
}

func nullableText(value string) pgtype.Text {
	if value == "" {
		return pgtype.Text{}
	}

	return pgtype.Text{String: value, Valid: true}
}
