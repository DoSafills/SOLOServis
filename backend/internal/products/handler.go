package products

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/DoSafills/SOLOServis/backend/internal/categories"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/DoSafills/SOLOServis/backend/internal/products/dto"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// Largos máximos de texto (columnas de product).
const (
	maxNameLength        = 255
	maxModelLength       = 150
	maxSKULength         = 100
	maxDescriptionLength = 5000
)

type Handler struct {
	repository Store
	logger     *slog.Logger
}

func NewHandler(repository Store, logger *slog.Logger) *Handler {
	return &Handler{repository: repository, logger: logger}
}

// writeFields son los campos comunes de POST y PUT.
type writeFields struct {
	categorySlug string
	categoryID   *int32
	brandID      *int32
	name         string
	model        string
	sku          string
	description  string
	attributes   map[string]json.RawMessage
}

// Create atiende POST /products: 201 con Location y el mismo cuerpo que GET.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var request dto.CreateProductRequest
	if err := httpx.DecodeJSON(w, r, &request, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteDecodeError(w, r, err)
		return
	}
	request.ImageURL = strings.TrimSpace(request.ImageURL)

	fields := writeFields{request.CategorySlug, request.CategoryID, request.BrandID, request.Name,
		request.Model, request.SKU, request.Description, request.Attributes}
	product, ok := h.validateWrite(w, r, &fields, func(v *httpx.Validator) {
		v.Required("imageUrl", request.ImageURL)
		v.Check(request.ImageURL == "" || isHTTPURL(request.ImageURL),
			"imageUrl", "format", "imageUrl debe ser una URL http(s) válida")
	})
	if !ok {
		return
	}
	product.ImageURL = request.ImageURL

	publicID, err := h.repository.Create(r.Context(), product)
	if err != nil {
		h.writeStoreError(w, r, err)
		return
	}

	detail, err := h.repository.GetDetail(r.Context(), publicID)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	w.Header().Set("Location", "/products/"+detail.ID)
	writeJSON(w, http.StatusCreated, detail)
}

// Update atiende PUT /products/{publicID}: reemplaza el producto completo.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	publicID, ok := parsePublicID(w, r)
	if !ok {
		return
	}

	var request dto.UpdateProductRequest
	if err := httpx.DecodeJSON(w, r, &request, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteDecodeError(w, r, err)
		return
	}

	fields := writeFields{request.CategorySlug, request.CategoryID, request.BrandID, request.Name,
		request.Model, request.SKU, request.Description, request.Attributes}
	product, ok := h.validateWrite(w, r, &fields, nil)
	if !ok {
		return
	}

	if err := h.repository.Update(r.Context(), publicID, product); err != nil {
		h.writeStoreError(w, r, err)
		return
	}

	detail, err := h.repository.GetDetail(r.Context(), publicID)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// Deactivate atiende DELETE /products/{publicID}: baja lógica, 204.
func (h *Handler) Deactivate(w http.ResponseWriter, r *http.Request) {
	publicID, ok := parsePublicID(w, r)
	if !ok {
		return
	}
	if err := h.repository.Deactivate(r.Context(), publicID); err != nil {
		h.writeStoreError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// validateWrite aplica, en el orden del estándar TAL-78, las reglas de los
// campos, resuelve la categoría y valida los atributos contra su definición.
// Si algo falla ya respondió (422 o 500) y devuelve ok = false.
func (h *Handler) validateWrite(w http.ResponseWriter, r *http.Request, f *writeFields, extra func(*httpx.Validator)) (ProductWrite, bool) {
	f.categorySlug = strings.TrimSpace(f.categorySlug)
	f.name = strings.TrimSpace(f.name)
	f.model = strings.TrimSpace(f.model)
	f.sku = strings.TrimSpace(f.sku)
	f.description = strings.TrimSpace(f.description)

	var v httpx.Validator
	v.Required("name", f.name)
	v.MaxLength("name", f.name, maxNameLength)
	v.MaxLength("model", f.model, maxModelLength)
	v.MaxLength("sku", f.sku, maxSKULength)
	v.MaxLength("description", f.description, maxDescriptionLength)
	v.Check(f.brandID == nil || *f.brandID > 0, "brandId", "min", "brandId debe ser mayor que 0")
	switch {
	case f.categorySlug != "" && f.categoryID != nil:
		v.Add("categoryId", "one_of", "envía categorySlug o categoryId, no ambos")
	case f.categorySlug == "" && f.categoryID == nil:
		v.Add("categorySlug", "required", "categorySlug es obligatorio")
	case f.categorySlug != "":
		v.Check(categories.ValidSlug(f.categorySlug), "categorySlug", "format",
			"categorySlug debe tener entre 1 y 64 caracteres: minúsculas, dígitos o guiones")
	default:
		v.Check(*f.categoryID > 0, "categoryId", "min", "categoryId debe ser mayor que 0")
	}
	if extra != nil {
		extra(&v)
	}
	if !v.Valid() {
		httpx.WriteValidation(w, r, v.Errors())
		return ProductWrite{}, false
	}

	ref := CategoryRef{Slug: f.categorySlug}
	field := "categorySlug"
	if f.categoryID != nil {
		ref = CategoryRef{LegacyID: *f.categoryID}
		field = "categoryId"
	}
	category, err := h.repository.ResolveCategory(r.Context(), ref)
	if errors.Is(err, ErrCategoryNotFound) {
		httpx.WriteValidation(w, r, []httpx.FieldError{{Field: field, Code: "not_found", Message: field + " no existe"}})
		return ProductWrite{}, false
	}
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return ProductWrite{}, false
	}
	if category.ItemType != categories.ItemTypeProduct {
		httpx.WriteValidation(w, r, []httpx.FieldError{{Field: field, Code: "one_of",
			Message: field + " debe ser una categoría de productos"}})
		return ProductWrite{}, false
	}
	if category.LegacyID == nil {
		// product.category_id es obligatorio mientras conviva el modelo anterior.
		httpx.WriteValidation(w, r, []httpx.FieldError{{Field: field, Code: "not_found",
			Message: field + " no está habilitada para productos (sin categoría equivalente en product_category)"}})
		return ProductWrite{}, false
	}

	values, fieldErrors := categories.ValidateAttributes(category.Attributes, f.attributes)
	if len(fieldErrors) > 0 {
		httpx.WriteValidation(w, r, fieldErrors)
		return ProductWrite{}, false
	}

	return ProductWrite{
		LegacyCategoryID: *category.LegacyID,
		CategoryRef:      category.ID,
		BrandID:          f.brandID,
		Name:             f.name,
		Model:            f.model,
		SKU:              f.sku,
		Description:      f.description,
		Attributes:       values,
	}, true
}

// writeStoreError mapea errores de escritura según TAL-78 §3.5.
func (h *Handler) writeStoreError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, ErrNotFound) {
		httpx.WriteProblem(w, r, http.StatusNotFound, httpx.CodeNotFound, "El producto no existe.")
		return
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			httpx.WriteProblem(w, r, http.StatusConflict, httpx.CodeConflict,
				"Ya existe un producto con ese SKU.")
			return
		case "23503":
			if field := constraintField(pgErr.ConstraintName); field != "" {
				httpx.WriteValidation(w, r, []httpx.FieldError{{Field: field, Code: "not_found", Message: field + " no existe"}})
				return
			}
		}
	}
	httpx.WriteInternal(w, r, h.logger, err)
}

// List atiende GET /products?category=&limit=&offset=. category acepta el id
// de product_category (incluye subcategorías) o el slug de categories.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	params, fieldErrors := parseListParams(r.URL.Query())
	if len(fieldErrors) > 0 {
		httpx.WriteProblem(w, r, http.StatusBadRequest, httpx.CodeInvalidParameter,
			"Los parámetros de la consulta no son válidos.", fieldErrors...)
		return
	}

	items, total, err := h.repository.List(r.Context(), params)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	if items == nil {
		items = []dto.ProductListItem{}
	}
	w.Header().Set("X-Total-Count", strconv.FormatInt(total, 10))
	writeJSON(w, http.StatusOK, items)
}

func parseListParams(query url.Values) (ListParams, []httpx.FieldError) {
	params := ListParams{Limit: DefaultListLimit}
	var v httpx.Validator

	if query.Has("category") {
		category := query.Get("category")
		if id, err := strconv.ParseInt(category, 10, 32); err == nil {
			legacyID := int32(id)
			v.Check(legacyID > 0, "category", "min", "category debe ser mayor que 0")
			params.LegacyCategoryID = &legacyID
		} else if categories.ValidSlug(category) {
			params.CategorySlug = &category
		} else {
			v.Add("category", "format", "category debe ser un id numérico o un slug de categoría")
		}
	}

	if query.Has("limit") {
		limit, err := strconv.Atoi(query.Get("limit"))
		switch {
		case err != nil:
			v.Add("limit", "invalid_type", "limit debe ser un número entero")
		case limit < 1:
			v.Add("limit", "min", "limit debe ser mayor o igual a 1")
		case limit > MaxListLimit:
			v.Add("limit", "max", fmt.Sprintf("limit no puede superar %d", MaxListLimit))
		default:
			params.Limit = limit
		}
	}

	if query.Has("offset") {
		offset, err := strconv.Atoi(query.Get("offset"))
		switch {
		case err != nil:
			v.Add("offset", "invalid_type", "offset debe ser un número entero")
		case offset < 0:
			v.Add("offset", "min", "offset debe ser mayor o igual a 0")
		default:
			params.Offset = offset
		}
	}

	return params, v.Errors()
}

// GetByPublicID atiende GET /products/{publicID}. Un producto dado de baja
// responde 404.
func (h *Handler) GetByPublicID(w http.ResponseWriter, r *http.Request) {
	publicID, ok := parsePublicID(w, r)
	if !ok {
		return
	}

	detail, err := h.repository.GetDetail(r.Context(), publicID)
	if errors.Is(err, ErrNotFound) {
		httpx.WriteProblem(w, r, http.StatusNotFound, httpx.CodeNotFound, "El producto no existe.")
		return
	}
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// ListCategories atiende GET /product-categories (árbol del modelo anterior).
func (h *Handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	rows, err := h.repository.ListCategories(r.Context())
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}

	result := make([]dto.ProductCategory, 0, len(rows))
	for _, row := range rows {
		result = append(result, dto.FromProductCategory(row))
	}
	writeJSON(w, http.StatusOK, result)
}

// ListReviews atiende GET /products/{publicID}/reviews.
func (h *Handler) ListReviews(w http.ResponseWriter, r *http.Request) {
	publicID, ok := parsePublicID(w, r)
	if !ok {
		return
	}

	rows, err := h.repository.ListReviewsByPublicID(r.Context(), publicID)
	if errors.Is(err, ErrNotFound) {
		httpx.WriteProblem(w, r, http.StatusNotFound, httpx.CodeNotFound, "El producto no existe.")
		return
	}
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}

	reviews := make([]dto.ProductReview, 0, len(rows))
	for _, row := range rows {
		reviews = append(reviews, dto.FromProductReview(row))
	}
	writeJSON(w, http.StatusOK, reviews)
}

func parsePublicID(w http.ResponseWriter, r *http.Request) (pgtype.UUID, bool) {
	var publicID pgtype.UUID
	if err := publicID.Scan(chi.URLParam(r, "publicID")); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, httpx.CodeInvalidParameter,
			"El id del producto no es válido.",
			httpx.FieldError{Field: "publicID", Code: "format", Message: "publicID debe ser un UUID"})
		return pgtype.UUID{}, false
	}
	return publicID, true
}

func isHTTPURL(value string) bool {
	parsed, err := url.ParseRequestURI(value)

	return err == nil &&
		(parsed.Scheme == "http" || parsed.Scheme == "https") &&
		parsed.Host != ""
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
