package services

import (
	"context"

	"github.com/DoSafills/SOLOServis/backend/internal/database/dbutil"
	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/DoSafills/SOLOServis/backend/internal/services/dto"
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

// List devuelve los servicios activos, o solo el indicado si publicID es
// válido. Carga ofertas, especificaciones e historial de todos los
// servicios con una consulta por tabla, sin importar cuántos sean.
func (r *Repository) List(ctx context.Context, publicID pgtype.UUID) ([]dto.ServiceDetail, error) {
	rows, err := r.queries.ListServices(ctx, publicID)
	if err != nil {
		return nil, err
	}

	ids := make([]int32, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}

	offers, err := r.queries.ListServiceOffers(ctx, ids)
	if err != nil {
		return nil, err
	}

	specifications, err := r.queries.ListServiceSpecifications(ctx, ids)
	if err != nil {
		return nil, err
	}

	history, err := r.queries.ListServicePriceHistory(ctx, ids)
	if err != nil {
		return nil, err
	}

	offersByService := dbutil.GroupBy(offers, func(row generated.ListServiceOffersRow) int32 { return row.ServiceID })
	specificationsByService := dbutil.GroupBy(specifications, func(row generated.ListServiceSpecificationsRow) int32 { return row.ServiceID })
	historyByService := dbutil.GroupBy(history, func(row generated.ListServicePriceHistoryRow) int32 { return row.ServiceID })

	result := make([]dto.ServiceDetail, len(rows))
	for i, row := range rows {
		result[i] = dto.FromService(
			row,
			offersByService[row.ID],
			specificationsByService[row.ID],
			historyByService[row.ID],
		)
	}

	return result, nil
}
