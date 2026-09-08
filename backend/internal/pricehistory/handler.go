package pricehistory

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/DoSafills/SOLOServis/backend/internal/pricehistory/dto"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	repository *Repository
}

func NewHandler(repository *Repository) *Handler {
	return &Handler{
		repository: repository,
	}
}

func (h *Handler) ListByOfferID(w http.ResponseWriter, r *http.Request) {
	offerIDParam := chi.URLParam(r, "offerId")

	offerID, err := strconv.ParseInt(offerIDParam, 10, 32)
	if err != nil {
		http.Error(w, "invalid offer id", http.StatusBadRequest)
		return
	}

	rows, err := h.repository.ListByOfferID(r.Context(), int32(offerID))
	if err != nil {
		http.Error(w, "failed to list price history", http.StatusInternalServerError)
		return
	}

	history := make([]dto.PricePoint, 0, len(rows))

	for _, row := range rows {
		history = append(history, dto.FromProductPriceHistory(row))
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(history); err != nil {
		return
	}
}
