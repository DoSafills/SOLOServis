package stores

import (
	"context"

	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is the persistence port of the stores module.
type Repository interface {
	// List returns the active stores, ordered by id.
	List(ctx context.Context) ([]generated.ListStoresRow, error)
}

// repository is the PostgreSQL implementation of Repository.
type repository struct {
	queries *generated.Queries
}

// NewRepository returns a Repository backed by the given connection pool.
func NewRepository(db *pgxpool.Pool) Repository {
	return &repository{queries: generated.New(db)}
}

func (r *repository) List(ctx context.Context) ([]generated.ListStoresRow, error) {
	return r.queries.ListStores(ctx)
}
