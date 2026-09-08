package offers

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

func (r *Repository) List(ctx context.Context) ([]generated.ListOffersRow, error) {
	return r.queries.ListOffers(ctx)
}

func (r *Repository) GetByID(ctx context.Context, id int32) (generated.GetOfferByIDRow, error) {
	return r.queries.GetOfferByID(ctx, id)
}
