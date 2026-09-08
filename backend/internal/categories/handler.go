package categories

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/DoSafills/SOLOServis/backend/internal/categories/dto"
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
		http.Error(w, "failed to list categories", http.StatusInternalServerError)
		return
	}

	categories := make([]dto.Category, 0, len(rows))

	for _, row := range rows {
		categories = append(categories, dto.FromCategory(row))
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(categories); err != nil {
		return
	}
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idParam, 10, 32)
	if err != nil {
		http.Error(w, "invalid category id", http.StatusBadRequest)
		return
	}

	category, err := h.repository.GetByID(r.Context(), int32(id))
	if err != nil {
		http.Error(w, "category not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(dto.FromCategory(category)); err != nil {
		return
	}
}
