package services

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Handler struct{ repository *Repository }

func NewHandler(repository *Repository) *Handler { return &Handler{repository: repository} }

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	services, err := h.repository.List(r.Context(), r.URL.Query().Get("category"))
	if err != nil {
		http.Error(w, "failed to list services", http.StatusInternalServerError)
		return
	}
	writeJSON(w, services)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	var publicID pgtype.UUID
	if err := publicID.Scan(chi.URLParam(r, "publicID")); err != nil {
		http.Error(w, "invalid service id", http.StatusBadRequest)
		return
	}
	service, err := h.repository.Get(r.Context(), publicID)
	if err != nil {
		http.Error(w, "service not found", http.StatusNotFound)
		return
	}
	writeJSON(w, service)
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
