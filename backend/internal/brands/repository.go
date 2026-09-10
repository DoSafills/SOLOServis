package brands

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

func (r *Repository) List(ctx context.Context) ([]generated.Brand, error) {
	return r.queries.ListBrands(ctx)
}

func (r *Repository) GetByID(ctx context.Context, id int32) (generated.Brand, error) {
	return r.queries.GetBrandByID(ctx, id)
}
