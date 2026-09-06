package products

import (
	"encoding/json"
	"net/http"

	"github.com/DoSafills/SOLOServis/backend/internal/products/dto"
	"github.com/go-chi/chi/v5"
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
	rows, err := h.repository.List(r.Context())
	if err != nil {
		http.Error(w, "failed to list products", http.StatusInternalServerError)
		return
	}

	products := make([]dto.ProductListItem, 0, len(rows))

	for _, row := range rows {
		products = append(products, dto.FromListProduct(row))
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(products); err != nil {
		return
	}
}

func (h *Handler) GetByPublicID(w http.ResponseWriter, r *http.Request) {
	publicID := chi.URLParam(r, "publicID")

	var uuid pgtype.UUID
	if err := uuid.Scan(publicID); err != nil {
		http.Error(w, "invalid product id", http.StatusBadRequest)
		return
	}

	product, err := h.repository.GetDetailByPublicID(r.Context(), uuid)
	if err != nil {
		http.Error(w, "product not found", http.StatusNotFound)
		return
	}

	images, err := h.repository.ListImages(r.Context(), product.ID)
	if err != nil {
		http.Error(w, "failed to load products images", http.StatusInternalServerError)
		return
	}

	offers, err := h.repository.ListOffers(r.Context(), product.ID)
	if err != nil {
		http.Error(w, "failed to load product offers", http.StatusInternalServerError)
		return
	}

	result := dto.FromProduct(product)

	result.Images = make([]dto.ProductImage, 0, len(images))
	for _, image := range images {
		result.Images = append(result.Images, dto.FromProductImage(image))
	}

	result.Offers = make([]dto.ProductOffer, 0, len(offers))
	for _, offer := range offers {
		result.Offers = append(result.Offers, dto.FromProductOffer(offer))
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(result); err != nil {
		return
	}
}
