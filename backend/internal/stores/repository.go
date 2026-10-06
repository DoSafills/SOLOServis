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

func (r *Repository) Get(ctx context.Context, id int32) (generated.GetStoreRow, error) {
	return r.queries.GetStore(ctx, id)
}
