package stores

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/DoSafills/SOLOServis/backend/internal/http/httpx"
	"github.com/DoSafills/SOLOServis/backend/internal/stores/dto"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
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

	result := make([]dto.Store, len(rows))
	for i, row := range rows {
		result[i] = dto.FromStore(row)
	}

	httpx.WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
	if err != nil || id <= 0 {
		http.Error(w, "invalid store id", http.StatusBadRequest)
		return
	}

	row, err := h.repository.Get(r.Context(), int32(id))
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "store not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to load store", http.StatusInternalServerError)
		return
	}

	// GetStore y ListStores devuelven las mismas columnas.
	httpx.WriteJSON(w, http.StatusOK, dto.FromStore(generated.ListStoresRow(row)))
}
