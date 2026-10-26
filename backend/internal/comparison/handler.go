package comparison

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/DoSafills/SOLOServis/backend/internal/authz"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// codeIncompatibleCategory es el código por campo de productos de otra categoría.
const codeIncompatibleCategory = "incompatible_category"

// Store es lo que el handler necesita de la capa de datos; *Repository la
// implementa y los tests usan un fake en memoria.
type Store interface {
	ResolveProducts(ctx context.Context, publicIDs []string) ([]Product, error)
	BuildMatrix(ctx context.Context, ids []int32) (Matrix, error)
	Compatible(ctx context.Context, p Product, limit, offset int) ([]ProductSummary, int64, error)
	Create(ctx context.Context, userID int32, name string, products []Product) (int32, error)
	List(ctx context.Context, userID int32, limit, offset int) ([]Summary, int64, error)
	Get(ctx context.Context, userID, id int32) (Header, error)
	Rename(ctx context.Context, userID, id int32, name string) error
	Delete(ctx context.Context, userID, id int32) error
	AddItem(ctx context.Context, userID, id int32, p Product) error
	RemoveItem(ctx context.Context, userID, id, productID int32) error
}

type Handler struct {
	store  Store
	logger *slog.Logger
}

func NewHandler(store Store, logger *slog.Logger) *Handler {
	return &Handler{store: store, logger: logger}
}

// --------------------------------------------------------------- públicos

// Compare atiende GET /comparisons/products/compare?ids=a,b,c: compara de 2 a
// MaxProducts productos de la misma categoría sin guardar nada.
func (h *Handler) Compare(w http.ResponseWriter, r *http.Request) {
	var v httpx.Validator
	ids := parseUUIDList(&v, "ids", r.URL.Query().Get("ids"), MinCompare)
	if !v.Valid() {
		writeInvalidQuery(w, r, v)
		return
	}

	products, err := h.store.ResolveProducts(r.Context(), ids)
	if errors.Is(err, ErrProductNotFound) {
		httpx.WriteProblem(w, r, http.StatusNotFound, httpx.CodeNotFound, "Uno de los productos no existe.")
		return
	}
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	if !sameCategory(products) {
		httpx.WriteProblem(w, r, http.StatusBadRequest, httpx.CodeInvalidParameter,
			"Los productos deben ser de la misma categoría.",
			httpx.FieldError{Field: "ids", Code: codeIncompatibleCategory, Message: "ids debe contener productos de la misma categoría"})
		return
	}

	matrix, err := h.store.BuildMatrix(r.Context(), internalIDs(products))
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, matrix)
}

// Compatible atiende GET /comparisons/compatible/{productId}: alternativas de
// la misma categoría, paginadas (X-Total-Count).
func (h *Handler) Compatible(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "productId")
	if !validUUID(key) {
		httpx.WriteProblem(w, r, http.StatusBadRequest, httpx.CodeInvalidParameter, "El id del producto no es válido.",
			httpx.FieldError{Field: "productId", Code: "format", Message: "productId debe ser un UUID"})
		return
	}
	var v httpx.Validator
	limit, offset := parsePage(&v, r.URL.Query())
	if !v.Valid() {
		writeInvalidQuery(w, r, v)
		return
	}

	products, err := h.store.ResolveProducts(r.Context(), []string{key})
	if errors.Is(err, ErrProductNotFound) {
		httpx.WriteProblem(w, r, http.StatusNotFound, httpx.CodeNotFound, "El producto no existe.")
		return
	}
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	summaries, total, err := h.store.Compatible(r.Context(), products[0], limit, offset)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	if summaries == nil {
		summaries = []ProductSummary{}
	}
	w.Header().Set("X-Total-Count", strconv.FormatInt(total, 10))
	writeJSON(w, http.StatusOK, summaries)
}

// ------------------------------------------------- guardadas (con sesión)

// Create atiende POST /comparisons/products: 201 + Location con la matriz.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(w, r)
	if !ok {
		return
	}
	var request CreateRequest
	if err := httpx.DecodeJSON(w, r, &request, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteDecodeError(w, r, err)
		return
	}

	var v httpx.Validator
	name := normalizeName(&v, request.Name, false)
	ids := checkUUIDs(&v, "productIds", request.ProductIDs, 1)
	if !v.Valid() {
		httpx.WriteValidation(w, r, v.Errors())
		return
	}

	products, err := h.store.ResolveProducts(r.Context(), ids)
	if errors.Is(err, ErrProductNotFound) {
		httpx.WriteValidation(w, r, []httpx.FieldError{{Field: "productIds", Code: "not_found",
			Message: "productIds contiene un producto que no existe"}})
		return
	}
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	if !sameCategory(products) {
		httpx.WriteValidation(w, r, []httpx.FieldError{{Field: "productIds", Code: codeIncompatibleCategory,
			Message: "productIds debe contener productos de la misma categoría"}})
		return
	}

	id, err := h.store.Create(r.Context(), userID, name, products)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	w.Header().Set("Location", "/comparisons/products/"+strconv.Itoa(int(id)))
	h.writeSaved(w, r, userID, id, http.StatusCreated)
}

// List atiende GET /comparisons/products: las comparaciones del usuario.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(w, r)
	if !ok {
		return
	}
	var v httpx.Validator
	limit, offset := parsePage(&v, r.URL.Query())
	if !v.Valid() {
		writeInvalidQuery(w, r, v)
		return
	}
	summaries, total, err := h.store.List(r.Context(), userID, limit, offset)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	if summaries == nil {
		summaries = []Summary{}
	}
	w.Header().Set("X-Total-Count", strconv.FormatInt(total, 10))
	writeJSON(w, http.StatusOK, summaries)
}

// Get atiende GET /comparisons/products/{id}. Una comparación de otro usuario
// responde 404, igual que una inexistente, para no revelar que existe.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(w, r)
	if !ok {
		return
	}
	id, ok := parseComparisonID(w, r)
	if !ok {
		return
	}
	h.writeSaved(w, r, userID, id, http.StatusOK)
}

// Rename atiende PUT /comparisons/products/{id}: {"name": "..."}.
func (h *Handler) Rename(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(w, r)
	if !ok {
		return
	}
	id, ok := parseComparisonID(w, r)
	if !ok {
		return
	}
	var request RenameRequest
	if err := httpx.DecodeJSON(w, r, &request, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteDecodeError(w, r, err)
		return
	}
	var v httpx.Validator
	name := normalizeName(&v, request.Name, true)
	if !v.Valid() {
		httpx.WriteValidation(w, r, v.Errors())
		return
	}
	if err := h.store.Rename(r.Context(), userID, id, name); err != nil {
		h.writeStoreError(w, r, err)
		return
	}
	h.writeSaved(w, r, userID, id, http.StatusOK)
}

// Delete atiende DELETE /comparisons/products/{id}: 204.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(w, r)
	if !ok {
		return
	}
	id, ok := parseComparisonID(w, r)
	if !ok {
		return
	}
	if err := h.store.Delete(r.Context(), userID, id); err != nil {
		h.writeStoreError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AddItem atiende POST /comparisons/products/{id}/items: {"productId": "..."}.
func (h *Handler) AddItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(w, r)
	if !ok {
		return
	}
	id, ok := parseComparisonID(w, r)
	if !ok {
		return
	}
	var request AddItemRequest
	if err := httpx.DecodeJSON(w, r, &request, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteDecodeError(w, r, err)
		return
	}
	var v httpx.Validator
	productID := strings.TrimSpace(request.ProductID)
	v.Required("productId", productID)
	v.Check(productID == "" || validUUID(productID), "productId", "format", "productId debe ser un UUID")
	if !v.Valid() {
		httpx.WriteValidation(w, r, v.Errors())
		return
	}

	products, err := h.store.ResolveProducts(r.Context(), []string{productID})
	if errors.Is(err, ErrProductNotFound) {
		httpx.WriteValidation(w, r, []httpx.FieldError{{Field: "productId", Code: "not_found", Message: "productId no existe"}})
		return
	}
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	if err := h.store.AddItem(r.Context(), userID, id, products[0]); err != nil {
		h.writeStoreError(w, r, err)
		return
	}
	w.Header().Set("Location", "/comparisons/products/"+strconv.Itoa(int(id)))
	h.writeSaved(w, r, userID, id, http.StatusCreated)
}

// RemoveItem atiende DELETE /comparisons/products/{id}/items/{productId}: 204.
func (h *Handler) RemoveItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(w, r)
	if !ok {
		return
	}
	id, ok := parseComparisonID(w, r)
	if !ok {
		return
	}
	key := chi.URLParam(r, "productId")
	if !validUUID(key) {
		httpx.WriteProblem(w, r, http.StatusBadRequest, httpx.CodeInvalidParameter, "El id del producto no es válido.",
			httpx.FieldError{Field: "productId", Code: "format", Message: "productId debe ser un UUID"})
		return
	}
	products, err := h.store.ResolveProducts(r.Context(), []string{key})
	if errors.Is(err, ErrProductNotFound) {
		httpx.WriteProblem(w, r, http.StatusNotFound, httpx.CodeNotFound, "El producto no está en la comparación.")
		return
	}
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	if err := h.store.RemoveItem(r.Context(), userID, id, products[0].ID); err != nil {
		h.writeStoreError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeSaved responde la comparación guardada completa (cabecera + matriz).
func (h *Handler) writeSaved(w http.ResponseWriter, r *http.Request, userID, id int32, status int) {
	header, err := h.store.Get(r.Context(), userID, id)
	if err != nil {
		h.writeStoreError(w, r, err)
		return
	}
	matrix, err := h.store.BuildMatrix(r.Context(), header.ProductIDs)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	writeJSON(w, status, SavedComparison{
		ID: header.ID, Name: header.Name, CreatedAt: header.CreatedAt, UpdatedAt: header.UpdatedAt, Matrix: matrix,
	})
}

// writeStoreError mapea errores de dominio según TAL-78 §3.5.
func (h *Handler) writeStoreError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrComparisonNotFound):
		httpx.WriteProblem(w, r, http.StatusNotFound, httpx.CodeNotFound, "La comparación no existe.")
	case errors.Is(err, ErrItemNotFound):
		httpx.WriteProblem(w, r, http.StatusNotFound, httpx.CodeNotFound, "El producto no está en la comparación.")
	case errors.Is(err, ErrIncompatibleCategory):
		httpx.WriteValidation(w, r, []httpx.FieldError{{Field: "productId", Code: codeIncompatibleCategory,
			Message: "productId es de otra categoría que la comparación"}})
	case errors.Is(err, ErrComparisonFull):
		httpx.WriteValidation(w, r, []httpx.FieldError{{Field: "productId", Code: "max",
			Message: fmt.Sprintf("una comparación admite hasta %d productos", MaxProducts)}})
	case errors.Is(err, ErrAlreadyIncluded):
		httpx.WriteProblem(w, r, http.StatusConflict, httpx.CodeConflict, "El producto ya está en la comparación.")
	default:
		httpx.WriteInternal(w, r, h.logger, err)
	}
}

// userID toma el usuario de la sesión (authz.RequireSession). Nunca del body.
func (h *Handler) userID(w http.ResponseWriter, r *http.Request) (int32, bool) {
	session, ok := authz.SessionFrom(r.Context())
	if !ok {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Se requiere iniciar sesión.")
		return 0, false
	}
	return session.UserID, true
}

// ------------------------------------------------------------- utilidades

func sameCategory(products []Product) bool {
	for _, p := range products[1:] {
		if p.CategoryID != products[0].CategoryID {
			return false
		}
	}
	return true
}

func internalIDs(products []Product) []int32 {
	ids := make([]int32, len(products))
	for i, p := range products {
		ids[i] = p.ID
	}
	return ids
}

func normalizeName(v *httpx.Validator, name string, required bool) string {
	name = strings.TrimSpace(name)
	if required {
		v.Required("name", name)
	} else if name == "" {
		name = DefaultName
	}
	v.MaxLength("name", name, MaxNameLength)
	return name
}

// parseUUIDList valida una lista separada por coma (query).
func parseUUIDList(v *httpx.Validator, field, raw string, min int) []string {
	if strings.TrimSpace(raw) == "" {
		v.Add(field, "required", field+" es obligatorio (UUIDs separados por coma)")
		return nil
	}
	parts := strings.Split(raw, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return checkUUIDs(v, field, parts, min)
}

// checkUUIDs exige entre min y MaxProducts UUIDs distintos.
func checkUUIDs(v *httpx.Validator, field string, ids []string, min int) []string {
	switch {
	case len(ids) < min:
		v.Add(field, "min", fmt.Sprintf("%s debe tener al menos %d productos", field, min))
		return nil
	case len(ids) > MaxProducts:
		v.Add(field, "max", fmt.Sprintf("%s admite hasta %d productos", field, MaxProducts))
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.ToLower(strings.TrimSpace(id))
		if !validUUID(id) {
			v.Add(field, "format", field+" debe contener UUIDs")
			return nil
		}
		if seen[id] {
			v.Add(field, "unique", field+" no puede repetir productos")
			return nil
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func validUUID(value string) bool {
	var uuid pgtype.UUID
	return len(value) == 36 && uuid.Scan(value) == nil
}

func parseComparisonID(w http.ResponseWriter, r *http.Request) (int32, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
	if err != nil || id <= 0 {
		httpx.WriteProblem(w, r, http.StatusBadRequest, httpx.CodeInvalidParameter, "El id de la comparación no es válido.",
			httpx.FieldError{Field: "id", Code: "format", Message: "id debe ser un entero positivo"})
		return 0, false
	}
	return int32(id), true
}

func parsePage(v *httpx.Validator, query url.Values) (int, int) {
	limit, offset := DefaultLimit, 0
	if query.Has("limit") {
		value, err := strconv.Atoi(query.Get("limit"))
		switch {
		case err != nil:
			v.Add("limit", "invalid_type", "limit debe ser un número entero")
		case value < 1:
			v.Add("limit", "min", "limit debe ser mayor o igual a 1")
		case value > MaxLimit:
			v.Add("limit", "max", fmt.Sprintf("limit no puede superar %d", MaxLimit))
		default:
			limit = value
		}
	}
	if query.Has("offset") {
		value, err := strconv.Atoi(query.Get("offset"))
		switch {
		case err != nil:
			v.Add("offset", "invalid_type", "offset debe ser un número entero")
		case value < 0:
			v.Add("offset", "min", "offset debe ser mayor o igual a 0")
		default:
			offset = value
		}
	}
	return limit, offset
}

func writeInvalidQuery(w http.ResponseWriter, r *http.Request, v httpx.Validator) {
	httpx.WriteProblem(w, r, http.StatusBadRequest, httpx.CodeInvalidParameter,
		"Los parámetros de la consulta no son válidos.", v.Errors()...)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
