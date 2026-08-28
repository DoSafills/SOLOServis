package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/product-api/internal/db"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// Handler agrupa las dependencias necesarias para atender las rutas HTTP.
type Handler struct {
	q *db.Queries
}

func NewHandler(pool *pgxpool.Pool) *Handler {
	return &Handler{q: db.New(pool)}
}

// Routes registra las rutas de la API en el router recibido.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/health", h.Health)
	r.Get("/products", h.ListProducts)
	r.Get("/products/{id}", h.GetProduct)
	r.Get("/products/{id}/offers", h.ListOffers)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ListProducts -> GET /api/v1/products?page=1&page_size=20&search=&category_id=
func (h *Handler) ListProducts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	page := parseIntDefault(q.Get("page"), 1)
	if page < 1 {
		page = 1
	}
	pageSize := parseIntDefault(q.Get("page_size"), defaultPageSize)
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	var categoryID pgtype.Int8
	if raw := q.Get("category_id"); raw != "" {
		if v, err := strconv.ParseInt(raw, 10, 64); err == nil {
			categoryID = pgtype.Int8{Int64: v, Valid: true}
		}
	}

	var search pgtype.Text
	if raw := q.Get("search"); raw != "" {
		search = pgtype.Text{String: raw, Valid: true}
	}

	total, err := h.q.CountProducts(ctx, db.CountProductsParams{
		CategoryID: categoryID,
		Search:     search,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo contar los productos")
		return
	}

	rows, err := h.q.ListProducts(ctx, db.ListProductsParams{
		Limit:      int32(pageSize),
		Offset:     int32((page - 1) * pageSize),
		CategoryID: categoryID,
		Search:     search,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo obtener el listado de productos")
		return
	}

	items := make([]ProductSummary, 0, len(rows))
	for _, row := range rows {
		items = append(items, toProductSummary(row))
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize != 0 {
		totalPages++
	}

	writeJSON(w, http.StatusOK, PaginatedProducts{
		Data:       items,
		Page:       page,
		PageSize:   pageSize,
		TotalItems: total,
		TotalPages: totalPages,
	})
}

// GetProduct -> GET /api/v1/products/{id}
// Incluye las ofertas (precio por tienda) del producto.
func (h *Handler) GetProduct(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "id de producto inválido")
		return
	}

	product, err := h.q.GetProductByID(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "producto no encontrado")
		return
	}

	offerRows, err := h.q.ListOffersByProductID(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudieron obtener las ofertas")
		return
	}

	offers := make([]OfferDTO, 0, len(offerRows))
	for _, o := range offerRows {
		offers = append(offers, toOfferDTO(o))
	}

	writeJSON(w, http.StatusOK, ProductDetail{
		ID:          product.ID,
		PublicID:    uuidString(product.PublicID),
		Name:        product.Name,
		Model:       textPtr(product.Model),
		SKU:         textPtr(product.Sku),
		Description: textPtr(product.Description),
		Brand:       textPtr(product.BrandName),
		Category:    textPtr(product.CategoryName),
		Offers:      offers,
	})
}

// ListOffers -> GET /api/v1/products/{id}/offers
// Mismo producto, precios de las distintas tiendas que lo venden.
func (h *Handler) ListOffers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "id de producto inválido")
		return
	}

	// Verifica que el producto exista antes de listar sus ofertas.
	if _, err := h.q.GetProductByID(ctx, id); err != nil {
		writeError(w, http.StatusNotFound, "producto no encontrado")
		return
	}

	rows, err := h.q.ListOffersByProductID(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudieron obtener las ofertas")
		return
	}

	offers := make([]OfferDTO, 0, len(rows))
	for _, o := range rows {
		offers = append(offers, toOfferDTO(o))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"product_id": id,
		"offers":     offers,
	})
}

func parseIntDefault(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
