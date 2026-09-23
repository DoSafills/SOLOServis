package products

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

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
	products, err := h.listProducts(r)
	if err != nil {
		if errors.Is(err, errInvalidCategory) {
			http.Error(w, "invalid category id", http.StatusBadRequest)
			return
		}

		http.Error(w, "failed to list products", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(products); err != nil {
		return
	}
}

var errInvalidCategory = errors.New("invalid category id")

// listProducts devuelve el catálogo completo o, si viene ?category=<id>,
// los productos de esa categoría y de todas sus subcategorías.
func (h *Handler) listProducts(r *http.Request) ([]dto.ProductListItem, error) {
	category := r.URL.Query().Get("category")

	if category == "" {
		rows, err := h.repository.List(r.Context())
		if err != nil {
			return nil, err
		}

		products := make([]dto.ProductListItem, 0, len(rows))
		for _, row := range rows {
			products = append(products, dto.FromListProduct(row))
		}

		return products, nil
	}

	categoryID, err := strconv.ParseInt(category, 10, 32)
	if err != nil || categoryID <= 0 {
		return nil, errInvalidCategory
	}

	rows, err := h.repository.ListByCategory(r.Context(), int32(categoryID))
	if err != nil {
		return nil, err
	}

	products := make([]dto.ProductListItem, 0, len(rows))
	for _, row := range rows {
		products = append(products, dto.FromListProductByCategory(row))
	}

	return products, nil
}

func (h *Handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	rows, err := h.repository.ListCategories(r.Context())
	if err != nil {
		http.Error(w, "failed to list categories", http.StatusInternalServerError)
		return
	}

	categories := make([]dto.ProductCategory, 0, len(rows))
	for _, row := range rows {
		categories = append(categories, dto.FromProductCategory(row))
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(categories); err != nil {
		return
	}
}

func (h *Handler) ListReviews(w http.ResponseWriter, r *http.Request) {
	publicID := chi.URLParam(r, "publicID")

	var uuid pgtype.UUID
	if err := uuid.Scan(publicID); err != nil {
		http.Error(w, "invalid product id", http.StatusBadRequest)
		return
	}

	product, err := h.repository.GetByPublicID(r.Context(), uuid)
	if err != nil {
		http.Error(w, "product not found", http.StatusNotFound)
		return
	}

	rows, err := h.repository.ListReviews(r.Context(), product.ID)
	if err != nil {
		http.Error(w, "failed to load product reviews", http.StatusInternalServerError)
		return
	}

	reviews := make([]dto.ProductReview, 0, len(rows))
	for _, row := range rows {
		reviews = append(reviews, dto.FromProductReview(row))
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(reviews); err != nil {
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

	specifications, err := h.repository.ListSpecifications(r.Context(), product.ID)
	if err != nil {
		http.Error(w, "failed to load product specifications", http.StatusInternalServerError)
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

	result.Specifications = make([]dto.ProductSpecification, 0, len(specifications))
	for _, spec := range specifications {
		result.Specifications = append(result.Specifications, dto.FromProductSpecification(spec))
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(result); err != nil {
		return
	}
}
