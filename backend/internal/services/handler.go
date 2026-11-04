package services

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/DoSafills/SOLOServis/backend/internal/services/dto"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Store es lo que el handler necesita de la base. *Repository lo implementa;
// los tests usan un fake en memoria (estándar TAL-78 §5).
type Store interface {
	List(ctx context.Context) ([]generated.ListServicesRow, error)
	GetByPublicID(ctx context.Context, publicID pgtype.UUID) (generated.GetServiceByPublicIDRow, error)
	ListOffers(ctx context.Context, serviceID int32) ([]generated.ListServiceOffersRow, error)
	ListSpecifications(ctx context.Context, serviceID int32) ([]generated.ListServiceSpecificationsRow, error)
	ListPriceHistory(ctx context.Context, serviceID int32) ([]generated.ListServicePriceHistoryRow, error)
}

type Handler struct {
	repository Store
	logger     *slog.Logger
}

func NewHandler(repository Store, logger *slog.Logger) *Handler {
	return &Handler{
		repository: repository,
		logger:     logger,
	}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.repository.List(r.Context())
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}

	result := make([]dto.ServiceDetail, 0, len(rows))

	for _, row := range rows {
		offers, specifications, history, err := h.related(r.Context(), row.ID)
		if err != nil {
			httpx.WriteInternal(w, r, h.logger, err)
			return
		}

		result = append(
			result,
			dto.FromServiceList(row, offers, specifications, history),
		)
	}

	writeJSON(w, result)
}

func (h *Handler) GetByPublicID(w http.ResponseWriter, r *http.Request) {
	var uuid pgtype.UUID
	if err := uuid.Scan(chi.URLParam(r, "publicID")); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, httpx.CodeInvalidParameter,
			"El id del servicio debe ser un UUID válido.")
		return
	}

	service, err := h.repository.GetByPublicID(r.Context(), uuid)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteProblem(w, r, http.StatusNotFound, httpx.CodeNotFound, "Servicio no encontrado.")
		return
	}
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}

	offers, specifications, history, err := h.related(r.Context(), service.ID)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}

	writeJSON(w, dto.FromService(service, offers, specifications, history))
}

// related carga las ofertas, especificaciones e historial de un servicio.
func (h *Handler) related(ctx context.Context, serviceID int32) (
	[]generated.ListServiceOffersRow,
	[]generated.ListServiceSpecificationsRow,
	[]generated.ListServicePriceHistoryRow,
	error,
) {
	offers, err := h.repository.ListOffers(ctx, serviceID)
	if err != nil {
		return nil, nil, nil, err
	}
	specifications, err := h.repository.ListSpecifications(ctx, serviceID)
	if err != nil {
		return nil, nil, nil, err
	}
	history, err := h.repository.ListPriceHistory(ctx, serviceID)
	if err != nil {
		return nil, nil, nil, err
	}
	return offers, specifications, history, nil
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
