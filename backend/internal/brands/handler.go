package brands

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/DoSafills/SOLOServis/backend/internal/brands/dto"
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
		http.Error(w, "failed to list brands", http.StatusInternalServerError)
		return
	}

	brands := make([]dto.Brand, 0, len(rows))

	for _, row := range rows {
		brands = append(brands, dto.FromBrand(row))
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(brands); err != nil {
		return
	}
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idParam, 10, 32)
	if err != nil {
		http.Error(w, "invalid brand id", http.StatusBadRequest)
		return
	}

	brand, err := h.repository.GetByID(r.Context(), int32(id))
	if err != nil {
		http.Error(w, "brand not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(dto.FromBrand(brand)); err != nil {
		return
	}
}
