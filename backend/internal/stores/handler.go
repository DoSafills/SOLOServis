package stores

import (
	"encoding/json"
	"net/http"

	"github.com/DoSafills/SOLOServis/backend/internal/stores/dto"
)

type Handler struct {
	repository *Repository
}

func NewHandler(repository *Repository) *Handler {
	return &Handler{repository: repository}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.repository.List(r.Context())
	if err != nil {
		http.Error(w, "failed to list stores", http.StatusInternalServerError)
		return
	}

	result := make([]dto.Store, 0, len(rows))
	for _, row := range rows {
		result = append(result, dto.FromStore(row))
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
