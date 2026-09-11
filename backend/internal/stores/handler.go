package stores

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	repository *Repository
}

func NewHandler(repository *Repository) *Handler {
	return &Handler{repository: repository}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	stores, err := h.repository.List(r.Context())
	if err != nil {
		http.Error(w, "failed to list stores", http.StatusInternalServerError)
		return
	}
	writeJSON(w, stores)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		http.Error(w, "invalid store id", http.StatusBadRequest)
		return
	}

	store, err := h.repository.Get(r.Context(), int32(id))
	if err != nil {
		http.Error(w, "store not found", http.StatusNotFound)
		return
	}
	writeJSON(w, store)
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
