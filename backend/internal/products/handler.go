package products

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/DoSafills/SOLOServis/backend/internal/http/httpx"
	"github.com/DoSafills/SOLOServis/backend/internal/products/dto"
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

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var request dto.CreateProductRequest

	if err := httpx.DecodeJSON(r, &request); err != nil {
		http.Error(w, "invalid product payload", http.StatusBadRequest)
		return
	}

	request.Name = strings.TrimSpace(request.Name)
	request.ImageURL = strings.TrimSpace(request.ImageURL)

	if request.CategoryID <= 0 || request.Name == "" || !isHTTPURL(request.ImageURL) {
		http.Error(w, "categoryId, name and a valid http(s) imageUrl are required", http.StatusBadRequest)
		return
	}

	product, err := h.repository.Create(r.Context(), request)
	if err != nil {
		http.Error(w, "failed to create product", http.StatusInternalServerError)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, product)
}

func isHTTPURL(value string) bool {
	parsed, err := url.ParseRequestURI(value)

	return err == nil &&
		(parsed.Scheme == "http" || parsed.Scheme == "https") &&
		parsed.Host != ""
}

// List devuelve el catálogo completo o, si viene ?category=<id>,
// los productos de esa categoría y de todas sus subcategorías.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	var categoryID pgtype.Int4

	if category := r.URL.Query().Get("category"); category != "" {
		id, err := strconv.ParseInt(category, 10, 32)
		if err != nil || id <= 0 {
			http.Error(w, "invalid category id", http.StatusBadRequest)
			return
		}

		categoryID = pgtype.Int4{Int32: int32(id), Valid: true}
	}

	products, err := h.repository.List(r.Context(), categoryID)
	if err != nil {
		http.Error(w, "failed to list products", http.StatusInternalServerError)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, products)
}

func (h *Handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := h.repository.ListCategories(r.Context())
	if err != nil {
		http.Error(w, "failed to list categories", http.StatusInternalServerError)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, categories)
}

func (h *Handler) ListReviews(w http.ResponseWriter, r *http.Request) {
	uuid, ok := httpx.UUIDParam(w, r, "publicID", "invalid product id")
	if !ok {
		return
	}

	reviews, err := h.repository.ListReviews(r.Context(), uuid)
	if err != nil {
		writeError(w, err, "failed to load product reviews")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, reviews)
}

func (h *Handler) GetByPublicID(w http.ResponseWriter, r *http.Request) {
	uuid, ok := httpx.UUIDParam(w, r, "publicID", "invalid product id")
	if !ok {
		return
	}

	product, err := h.repository.GetDetail(r.Context(), uuid)
	if err != nil {
		writeError(w, err, "failed to load product")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, product)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	uuid, ok := httpx.UUIDParam(w, r, "publicID", "invalid product id")
	if !ok {
		return
	}

	var request dto.UpdateProductRequest

	if err := httpx.DecodeJSON(r, &request); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	product, err := h.repository.UpdateByPublicID(r.Context(), request.ToParams(uuid))
	if err != nil {
		writeError(w, err, "failed to update product")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, product)
}

func (h *Handler) Deactivate(w http.ResponseWriter, r *http.Request) {
	uuid, ok := httpx.UUIDParam(w, r, "publicID", "invalid product id")
	if !ok {
		return
	}

	product, err := h.repository.DeactivateByPublicID(r.Context(), uuid)
	if err != nil {
		writeError(w, err, "failed to deactivate product")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, product)
}

// writeError responde 404 si el producto no existe y 500 ante cualquier otro
// fallo, en vez de reportar todo error de base de datos como "no encontrado".
func writeError(w http.ResponseWriter, err error, message string) {
	if errors.Is(err, ErrNotFound) {
		http.Error(w, "product not found", http.StatusNotFound)
		return
	}

	http.Error(w, message, http.StatusInternalServerError)
}
