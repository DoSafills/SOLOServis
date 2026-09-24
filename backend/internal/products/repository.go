package products

import (
	"context"

	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/DoSafills/SOLOServis/backend/internal/products/dto"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

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
	var brandID pgtype.Int4
	if request.BrandID != nil {
		brandID = pgtype.Int4{Int32: *request.BrandID, Valid: true}
	}

	product, err := queries.CreateProduct(ctx, generated.CreateProductParams{
		CategoryID:  request.CategoryID,
		BrandID:     brandID,
		Name:        request.Name,
		Model:       pgtype.Text{String: request.Model, Valid: request.Model != ""},
		Sku:         pgtype.Text{String: request.SKU, Valid: request.SKU != ""},
		Description: pgtype.Text{String: request.Description, Valid: request.Description != ""},
	})
	if err != nil {
		return dto.CreatedProduct{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO product_image (product_id, image_url, alt_text, sort_order)
		VALUES ($1, $2, $3, 0)
	`, product.ID, request.ImageURL, request.Name)
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

func (r *Repository) List(ctx context.Context) ([]generated.ListProductsRow, error) {
	return r.queries.ListProducts(ctx)
}

func (r *Repository) GetByPublicID(ctx context.Context, publicID pgtype.UUID) (generated.Product, error) {
	return r.queries.GetProductByPublicID(ctx, publicID)
}

func (r *Repository) ListOffers(ctx context.Context, productID int32) ([]generated.ListProductOffersRow, error) {
	return r.queries.ListProductOffers(ctx, productID)
}

func (r *Repository) ListImages(ctx context.Context, productID int32) ([]generated.ProductImage, error) {
	return r.queries.ListProductImages(ctx, productID)
}
func (r *Repository) GetDetailByPublicID(ctx context.Context, publicID pgtype.UUID) (generated.GetProductDetailByPublicIDRow, error) {
	return r.queries.GetProductDetailByPublicID(ctx, publicID)
}
