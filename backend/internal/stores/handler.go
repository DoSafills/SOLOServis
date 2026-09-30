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

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
    id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
    if err != nil {
        http.Error(w, "invalid store id", http.StatusBadRequest)
        return
    }

    store, err := h.repository.GetByID(r.Context(), int32(id))
    if err != nil {
        http.Error(w, "store not found", http.StatusNotFound)
        return
    }

    products, err := h.repository.ListProducts(r.Context(), int32(id))
    if err != nil {
        http.Error(w, "failed to list store products", http.StatusInternalServerError)
        return
    }

    locations, err := h.repository.ListLocations(r.Context(), int32(id))
    if err != nil {
        http.Error(w, "failed to list store locations", http.StatusInternalServerError)
        return
    }

    result := dto.StoreDetail{
        Store:     dto.FromStoreDetail(store),
        Products:  make([]dto.StoreProduct, 0, len(products)),
        Locations: make([]dto.StoreLocation, 0, len(locations)),
    }

    for _, product := range products {
        result.Products = append(result.Products, dto.FromStoreProduct(product))
    }

    for _, location := range locations {
        result.Locations = append(result.Locations, dto.FromStoreLocation(location))
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(result)
}
