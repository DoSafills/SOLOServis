package services

import (
	"net/http"

	"github.com/DoSafills/SOLOServis/backend/internal/http/httpx"
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
	services, err := h.repository.List(r.Context(), pgtype.UUID{})
	if err != nil {
		http.Error(w, "failed to list services", http.StatusInternalServerError)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, services)
}

func (h *Handler) GetByPublicID(w http.ResponseWriter, r *http.Request) {
	uuid, ok := httpx.UUIDParam(w, r, "publicID", "invalid service id")
	if !ok {
		return
	}

	services, err := h.repository.List(r.Context(), uuid)
	if err != nil {
		http.Error(w, "failed to load service", http.StatusInternalServerError)
		return
	}

	if len(services) == 0 {
		http.Error(w, "service not found", http.StatusNotFound)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, services[0])
}
