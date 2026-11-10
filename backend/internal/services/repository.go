package services

import (
	"context"

	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/DoSafills/SOLOServis/backend/internal/services/dto"
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

func (r *Repository) List(ctx context.Context) ([]generated.ListServicesRow, error) {
	return r.queries.ListServices(ctx)
}

func (r *Repository) GetByPublicID(ctx context.Context, publicID pgtype.UUID) (generated.GetServiceByPublicIDRow, error) {
	return r.queries.GetServiceByPublicID(ctx, publicID)
}

func (r *Repository) ListOffers(ctx context.Context, serviceID int32) ([]generated.ListServiceOffersRow, error) {
	return r.queries.ListServiceOffers(ctx, serviceID)
}

func (r *Repository) ListSpecifications(ctx context.Context, serviceID int32) ([]generated.ListServiceSpecificationsRow, error) {
	return r.queries.ListServiceSpecifications(ctx, serviceID)
}

func (r *Repository) ListPriceHistory(ctx context.Context, serviceID int32) ([]generated.ListServicePriceHistoryRow, error) {
	return r.queries.ListServicePriceHistory(ctx, serviceID)
}

func (r *Repository) ToDetail(
	service generated.GetServiceByPublicIDRow,
	offers []generated.ListServiceOffersRow,
	specifications []generated.ListServiceSpecificationsRow,
	history []generated.ListServicePriceHistoryRow,
) dto.ServiceDetail {
	return dto.FromService(service, offers, specifications, history)
}
