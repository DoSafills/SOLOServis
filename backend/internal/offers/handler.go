package offers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/DoSafills/SOLOServis/backend/internal/offers/dto"
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

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.repository.List(r.Context())
	if err != nil {
		http.Error(w, "failed to list product offers", http.StatusInternalServerError)
		return
	}

	offers := make([]dto.ProductOffer, 0, len(rows))

	for _, row := range rows {
		offers = append(offers, dto.FromListOffersRow(row))
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(offers); err != nil {
		return
	}
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idParam, 10, 32)
	if err != nil {
		http.Error(w, "invalid product offer id", http.StatusBadRequest)
		return
	}

	offer, err := h.repository.GetByID(r.Context(), int32(id))
	if err != nil {
		http.Error(w, "product offer not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(dto.FromGetOfferByIDRow(offer)); err != nil {
		return
	}
}
