package products

import (
	"context"
	"errors"

	"github.com/DoSafills/SOLOServis/backend/internal/database/dbutil"
	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/DoSafills/SOLOServis/backend/internal/products/dto"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("product not found")

type Repository struct {
	db      *pgxpool.Pool
	queries *generated.Queries
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		db:      db,
		queries: generated.New(db),
	}
}

func (r *Repository) Create(ctx context.Context, request dto.CreateProductRequest) (dto.CreatedProduct, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return dto.CreatedProduct{}, err
	}
	defer tx.Rollback(ctx)

	queries := r.queries.WithTx(tx)

	product, err := queries.CreateProduct(ctx, request.ToParams())
	if err != nil {
		return dto.CreatedProduct{}, err
	}

	err = queries.CreateProductImage(ctx, generated.CreateProductImageParams{
		ProductID: product.ID,
		ImageUrl:  request.ImageURL,
		AltText:   dbutil.NullableText(request.Name),
	})
	if err != nil {
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

// List devuelve el catálogo, opcionalmente filtrado por categoría
// (incluye sus subcategorías).
func (r *Repository) List(ctx context.Context, categoryID pgtype.Int4) ([]dto.ProductListItem, error) {
	_, items, err := r.list(ctx, generated.ListProductsParams{CategoryID: categoryID})
	return items, err
}

func (r *Repository) GetDetail(ctx context.Context, publicID pgtype.UUID) (dto.ProductDetail, error) {
	rows, items, err := r.list(ctx, generated.ListProductsParams{PublicID: publicID})
	if err != nil {
		return dto.ProductDetail{}, err
	}

	if len(rows) == 0 {
		return dto.ProductDetail{}, ErrNotFound
	}

	specifications, err := r.queries.ListProductSpecifications(ctx, rows[0].ID)
	if err != nil {
		return dto.ProductDetail{}, err
	}

	priceHistory, err := r.queries.ListProductPriceHistory(ctx, rows[0].ID)
	if err != nil {
		return dto.ProductDetail{}, err
	}

	return dto.NewProductDetail(items[0], specifications, priceHistory), nil
}

// list carga los productos con sus imágenes y ofertas en tres consultas en
// total, sin importar cuántos productos haya. También devuelve las filas
// para que el llamador pueda usar los ids internos.
func (r *Repository) list(ctx context.Context, params generated.ListProductsParams) ([]generated.ListProductsRow, []dto.ProductListItem, error) {
	rows, err := r.queries.ListProducts(ctx, params)
	if err != nil {
		return nil, nil, err
	}

	ids := make([]int32, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}

	images, err := r.queries.ListProductImages(ctx, ids)
	if err != nil {
		return nil, nil, err
	}

	offers, err := r.queries.ListProductOffers(ctx, ids)
	if err != nil {
		return nil, nil, err
	}

	imagesByProduct := dbutil.GroupBy(images, func(image generated.ProductImage) int32 { return image.ProductID })
	offersByProduct := dbutil.GroupBy(offers, func(offer generated.ListProductOffersRow) int32 { return offer.ProductID })

	items := make([]dto.ProductListItem, len(rows))
	for i, row := range rows {
		items[i] = dto.FromListProduct(row, imagesByProduct[row.ID], offersByProduct[row.ID])
	}

	return rows, items, nil
}

func (r *Repository) ListCategories(ctx context.Context) ([]dto.ProductCategory, error) {
	rows, err := r.queries.ListProductCategories(ctx)
	if err != nil {
		return nil, err
	}

	categories := make([]dto.ProductCategory, len(rows))
	for i, row := range rows {
		categories[i] = dto.FromProductCategory(row)
	}

	return categories, nil
}

func (r *Repository) ListReviews(ctx context.Context, publicID pgtype.UUID) ([]dto.ProductReview, error) {
	product, err := r.queries.GetProductByPublicID(ctx, publicID)
	if err != nil {
		return nil, notFound(err)
	}

	rows, err := r.queries.ListProductReviews(ctx, product.ID)
	if err != nil {
		return nil, err
	}

	reviews := make([]dto.ProductReview, len(rows))
	for i, row := range rows {
		reviews[i] = dto.FromProductReview(row)
	}

	return reviews, nil
}

func (r *Repository) UpdateByPublicID(ctx context.Context, params generated.UpdateProductByPublicIDParams) (generated.Product, error) {
	product, err := r.queries.UpdateProductByPublicID(ctx, params)
	return product, notFound(err)
}

func (r *Repository) DeactivateByPublicID(ctx context.Context, publicID pgtype.UUID) (generated.Product, error) {
	product, err := r.queries.DeactivateProductByPublicID(ctx, publicID)
	return product, notFound(err)
}

// notFound traduce "sin filas" a ErrNotFound para que el handler pueda
// distinguirlo de un fallo de la base de datos.
func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}

	return err
}
