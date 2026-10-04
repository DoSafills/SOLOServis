package stores

import (
	"context"

	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
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

func (r *Repository) List(ctx context.Context) ([]generated.ListStoresRow, error) {
	return r.queries.ListStores(ctx)
}

func (r *Repository) GetByID(ctx context.Context, id int32) (generated.GetStoreByIDRow, error) {
	return r.queries.GetStoreByID(ctx, id)
}

func (r *Repository) ListProducts(ctx context.Context, storeID int32) ([]generated.ListStoreProductsRow, error) {
	return r.queries.ListStoreProducts(ctx, storeID)
}

func (r *Repository) ListLocations(ctx context.Context, storeID int32) ([]generated.ListStoreLocationsRow, error) {
	return r.queries.ListStoreLocations(ctx, storeID)
}
