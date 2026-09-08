package pricehistory

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

func (r *Repository) ListByOfferID(ctx context.Context, offerID int32) ([]generated.ProductPriceHistory, error) {
	return r.queries.ListPriceHistoryByOfferID(ctx, offerID)
}
