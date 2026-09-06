package products

import (
	"context"

	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	queries *generated.Queries
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		queries: generated.New(db),
	}
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
