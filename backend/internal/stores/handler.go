package stores

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/DoSafills/SOLOServis/backend/internal/stores/dto"
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
		http.Error(w, "failed to list stores", http.StatusInternalServerError)
		return
	}

	stores := make([]dto.Store, 0, len(rows))

	for _, row := range rows {
		stores = append(stores, dto.FromStore(row))
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(stores); err != nil {
		return
	}
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idParam, 10, 32)
	if err != nil {
		http.Error(w, "invalid store id", http.StatusBadRequest)
		return
	}

	store, err := h.repository.GetByID(r.Context(), int32(id))
	if err != nil {
		http.Error(w, "store not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(dto.FromStore(store)); err != nil {
		return
	}
}
