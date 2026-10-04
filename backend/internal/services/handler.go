package services

import (
	"encoding/json"
	"net/http"

	"github.com/DoSafills/SOLOServis/backend/internal/services/dto"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Handler struct {
	repository *Repository
}

func NewHandler(repository *Repository) *Handler {
	return &Handler{
		repository: repository,
	}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.repository.List(r.Context())
	if err != nil {
		http.Error(w, "failed to list services", http.StatusInternalServerError)
		return
	}

	result := make([]dto.ServiceDetail, 0, len(rows))

	for _, row := range rows {
		offers, err := h.repository.ListOffers(r.Context(), row.ID)
		if err != nil {
			http.Error(w, "failed to load service offers", http.StatusInternalServerError)
			return
		}

		specifications, err := h.repository.ListSpecifications(r.Context(), row.ID)
		if err != nil {
			http.Error(w, "failed to load service specifications", http.StatusInternalServerError)
			return
		}

		history, err := h.repository.ListPriceHistory(r.Context(), row.ID)
		if err != nil {
			http.Error(w, "failed to load service price history", http.StatusInternalServerError)
			return
		}

		result = append(
			result,
			dto.FromServiceList(row, offers, specifications, history),
		)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (h *Handler) GetByPublicID(w http.ResponseWriter, r *http.Request) {
	publicID := chi.URLParam(r, "publicID")

	var uuid pgtype.UUID

	if err := uuid.Scan(publicID); err != nil {
		http.Error(w, "invalid service id", http.StatusBadRequest)
		return
	}

	service, err := h.repository.GetByPublicID(r.Context(), uuid)
	if err != nil {
		http.Error(w, "service not found", http.StatusNotFound)
		return
	}

	offers, err := h.repository.ListOffers(r.Context(), service.ID)
	if err != nil {
		http.Error(w, "failed to load service offers", http.StatusInternalServerError)
		return
	}

	specifications, err := h.repository.ListSpecifications(r.Context(), service.ID)
	if err != nil {
		http.Error(w, "failed to load service specifications", http.StatusInternalServerError)
		return
	}

	history, err := h.repository.ListPriceHistory(r.Context(), service.ID)
	if err != nil {
		http.Error(w, "failed to load service price history", http.StatusInternalServerError)
		return
	}

	result := h.repository.ToDetail(
		service,
		offers,
		specifications,
		history,
	)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
