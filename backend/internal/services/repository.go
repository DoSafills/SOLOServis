package services

import (
	"context"

	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is the persistence port of the services module. The handler
// depends on this interface so the transport layer can be tested without
// PostgreSQL.
type Repository interface {
	// List returns the active services, ordered by id.
	List(ctx context.Context) ([]generated.ListServicesRow, error)

	// GetByPublicID returns a single active service. It returns
	// pgx.ErrNoRows when absent.
	GetByPublicID(ctx context.Context, publicID pgtype.UUID) (generated.GetServiceByPublicIDRow, error)

	// ListOffersByServiceIDs returns the offers of several services in one
	// query, ordered by price within each service.
	ListOffersByServiceIDs(ctx context.Context, serviceIDs []int32) ([]generated.ListServiceOffersByServiceIDsRow, error)

	// ListSpecificationsByServiceIDs returns the specifications of several
	// services in one query.
	ListSpecificationsByServiceIDs(ctx context.Context, serviceIDs []int32) ([]generated.ListServiceSpecificationsByServiceIDsRow, error)

	// ListPriceHistoryByServiceIDs returns the price history of several
	// services in one query, oldest first within each service.
	ListPriceHistoryByServiceIDs(ctx context.Context, serviceIDs []int32) ([]generated.ListServicePriceHistoryByServiceIDsRow, error)
}

// repository is the PostgreSQL implementation of Repository.
type repository struct {
	queries *generated.Queries
}

// NewRepository returns a Repository backed by the given connection pool.
func NewRepository(db *pgxpool.Pool) Repository {
	return &repository{queries: generated.New(db)}
}

func (r *repository) List(ctx context.Context) ([]generated.ListServicesRow, error) {
	return r.queries.ListServices(ctx)
}

func (r *repository) GetByPublicID(ctx context.Context, publicID pgtype.UUID) (generated.GetServiceByPublicIDRow, error) {
	return r.queries.GetServiceByPublicID(ctx, publicID)
}

func (r *repository) ListOffersByServiceIDs(ctx context.Context, serviceIDs []int32) ([]generated.ListServiceOffersByServiceIDsRow, error) {
	if len(serviceIDs) == 0 {
		return nil, nil
	}

	return r.queries.ListServiceOffersByServiceIDs(ctx, serviceIDs)
}

func (r *repository) ListSpecificationsByServiceIDs(ctx context.Context, serviceIDs []int32) ([]generated.ListServiceSpecificationsByServiceIDsRow, error) {
	if len(serviceIDs) == 0 {
		return nil, nil
	}

	return r.queries.ListServiceSpecificationsByServiceIDs(ctx, serviceIDs)
}

func (r *repository) ListPriceHistoryByServiceIDs(ctx context.Context, serviceIDs []int32) ([]generated.ListServicePriceHistoryByServiceIDsRow, error) {
	if len(serviceIDs) == 0 {
		return nil, nil
	}

	return r.queries.ListServicePriceHistoryByServiceIDs(ctx, serviceIDs)
}
