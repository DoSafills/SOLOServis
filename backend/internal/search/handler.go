package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/DoSafills/SOLOServis/backend/internal/categories"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
)

// Searcher es lo que el handler necesita de la capa de datos; *Repository la
// implementa y los tests usan un fake en memoria.
type Searcher interface {
	CategoryDefinitions(ctx context.Context, slug string) ([]categories.Attribute, error)
	SearchProducts(ctx context.Context, q Query) ([]ProductResult, int64, error)
	SearchFacets(ctx context.Context, q Query, defs []categories.Attribute) (Facets, error)
	SearchCategories(ctx context.Context, text string, itemType *string) ([]CategoryResult, error)
	SearchBrands(ctx context.Context, text string) ([]BrandResult, error)
}

type Handler struct {
	searcher Searcher
	logger   *slog.Logger
}

func NewHandler(searcher Searcher, logger *slog.Logger) *Handler {
	return &Handler{searcher: searcher, logger: logger}
}

// moneyPattern replica NUMERIC(12,2): hasta 10 enteros y 2 decimales.
var moneyPattern = regexp.MustCompile(`^\d{1,10}(\.\d{1,2})?$`)

// Products atiende GET /search/products: arreglo de resultados + X-Total-Count.
func (h *Handler) Products(w http.ResponseWriter, r *http.Request) {
	q, _, ok := h.parseQuery(w, r, true)
	if !ok {
		return
	}
	results, total, err := h.searcher.SearchProducts(r.Context(), q)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	if results == nil {
		results = []ProductResult{}
	}
	w.Header().Set("X-Total-Count", strconv.FormatInt(total, 10))
	writeJSON(w, results)
}

// Facets atiende GET /search/facets: total, precios, marcas y atributos
// filtrables de los productos que cumplen los mismos filtros que /search/products.
func (h *Handler) Facets(w http.ResponseWriter, r *http.Request) {
	q, defs, ok := h.parseQuery(w, r, false)
	if !ok {
		return
	}
	facets, err := h.searcher.SearchFacets(r.Context(), q, defs)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	writeJSON(w, facets)
}

// Categories atiende GET /search/categories?q=&itemType=.
func (h *Handler) Categories(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	var v httpx.Validator
	text := strings.TrimSpace(query.Get("q"))
	v.MaxLength("q", text, MaxQueryLength)
	var itemType *string
	if query.Has("itemType") {
		value := query.Get("itemType")
		if value == categories.ItemTypeProduct || value == categories.ItemTypeService {
			itemType = &value
		} else {
			v.Add("itemType", "one_of", "itemType debe ser product o service")
		}
	}
	if !v.Valid() {
		writeInvalidQuery(w, r, v)
		return
	}
	results, err := h.searcher.SearchCategories(r.Context(), text, itemType)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	writeJSON(w, results)
}

// Brands atiende GET /search/brands?q=.
func (h *Handler) Brands(w http.ResponseWriter, r *http.Request) {
	var v httpx.Validator
	text := strings.TrimSpace(r.URL.Query().Get("q"))
	v.MaxLength("q", text, MaxQueryLength)
	if !v.Valid() {
		writeInvalidQuery(w, r, v)
		return
	}
	results, err := h.searcher.SearchBrands(r.Context(), text)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	writeJSON(w, results)
}

// parseQuery valida los parámetros de búsqueda (400 invalid_parameter) y, si
// hay slug de categoría, carga sus definiciones para validar los attr.*.
// paginated indica si se aceptan sort, limit y offset.
func (h *Handler) parseQuery(w http.ResponseWriter, r *http.Request, paginated bool) (Query, []categories.Attribute, bool) {
	query := r.URL.Query()
	q := Query{Sort: "name", Limit: DefaultLimit}
	var v httpx.Validator

	text := strings.TrimSpace(query.Get("q"))
	v.MaxLength("q", text, MaxQueryLength)
	q.Words = strings.Fields(text)
	v.Check(len(q.Words) <= MaxWords, "q", "max", fmt.Sprintf("q admite hasta %d palabras", MaxWords))

	if query.Has("category") {
		category := query.Get("category")
		if id, err := strconv.ParseInt(category, 10, 32); err == nil {
			legacyID := int32(id)
			v.Check(legacyID > 0, "category", "min", "category debe ser mayor que 0")
			q.LegacyCategoryID = &legacyID
		} else if categories.ValidSlug(category) {
			q.CategorySlug = &category
		} else {
			v.Add("category", "format", "category debe ser un id numérico o un slug de categoría")
		}
	}

	if query.Has("brand") {
		parts := strings.Split(query.Get("brand"), ",")
		v.Check(len(parts) <= MaxBrands, "brand", "max", fmt.Sprintf("brand admite hasta %d marcas", MaxBrands))
		for _, part := range parts {
			id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 32)
			if err != nil || id <= 0 {
				v.Add("brand", "format", "brand debe ser una lista de ids positivos separados por coma")
				break
			}
			q.BrandIDs = append(q.BrandIDs, int32(id))
		}
	}

	q.MinPrice = parseMoney(&v, query, "minPrice")
	q.MaxPrice = parseMoney(&v, query, "maxPrice")
	if q.MinPrice != nil && q.MaxPrice != nil {
		low, _ := strconv.ParseFloat(*q.MinPrice, 64)
		high, _ := strconv.ParseFloat(*q.MaxPrice, 64)
		v.Check(low <= high, "maxPrice", "min", "maxPrice debe ser mayor o igual a minPrice")
	}

	if query.Has("available") {
		switch query.Get("available") {
		case "true", "false":
			q.Available = query.Get("available") == "true"
		default:
			v.Add("available", "one_of", "available debe ser true o false")
		}
	}

	if paginated {
		if query.Has("sort") {
			q.Sort = query.Get("sort")
			if _, ok := orderBySQL[q.Sort]; !ok {
				v.Add("sort", "one_of", "sort debe ser name, price_asc, price_desc o newest")
			}
		}
		parsePage(&v, query, &q)
	}

	attrKeys := attributeParams(query)
	if len(attrKeys) > 0 && q.CategorySlug == nil {
		v.Add("category", "required", "los filtros attr.* requieren category con el slug de la categoría")
	}

	// Las definiciones solo se cargan con los demás parámetros válidos.
	var defs []categories.Attribute
	if v.Valid() && q.CategorySlug != nil {
		var err error
		defs, err = h.searcher.CategoryDefinitions(r.Context(), *q.CategorySlug)
		if errors.Is(err, ErrCategoryNotFound) {
			v.Add("category", "not_found", "category no existe")
		} else if err != nil {
			httpx.WriteInternal(w, r, h.logger, err)
			return Query{}, nil, false
		} else {
			q.Attributes = parseAttributeFilters(&v, query, attrKeys, defs)
		}
	}

	if !v.Valid() {
		writeInvalidQuery(w, r, v)
		return Query{}, nil, false
	}
	return q, defs, true
}

// attributeParams devuelve, ordenados, los parámetros attr.<key>[.min|.max].
func attributeParams(query url.Values) []string {
	keys := []string{}
	for name := range query {
		if strings.HasPrefix(name, "attr.") {
			keys = append(keys, name)
		}
	}
	sort.Strings(keys)
	return keys
}

// parseAttributeFilters valida cada attr.* contra la definición: el atributo
// debe existir y ser filterable (TAL-71); el valor, cumplir su data_type.
func parseAttributeFilters(v *httpx.Validator, query url.Values, params []string, defs []categories.Attribute) []AttributeFilter {
	byKey := make(map[string]categories.Attribute, len(defs))
	for _, def := range defs {
		byKey[def.Key] = def
	}

	filters := make([]AttributeFilter, 0, len(params))
	for _, param := range params {
		key, bound := strings.TrimPrefix(param, "attr."), ""
		if base, ok := strings.CutSuffix(key, ".min"); ok {
			key, bound = base, OpMin
		} else if base, ok := strings.CutSuffix(key, ".max"); ok {
			key, bound = base, OpMax
		}
		value := strings.TrimSpace(query.Get(param))

		def, ok := byKey[key]
		switch {
		case !ok:
			v.Add(param, "unknown_field", param+" no es un atributo de la categoría")
			continue
		case !def.Filterable:
			v.Add(param, "not_filterable", param+" no es filtrable")
			continue
		case value == "":
			v.Add(param, "required", param+" necesita un valor")
			continue
		}

		numeric := def.DataType == "integer" || def.DataType == "decimal"
		if bound != "" && !numeric {
			v.Add(param, "invalid_type", ".min y .max solo aplican a atributos numéricos")
			continue
		}

		filter := AttributeFilter{AttributeID: def.ID, Op: bound, Value: value}
		switch def.DataType {
		case "integer":
			if _, err := strconv.ParseInt(value, 10, 64); err != nil {
				v.Add(param, "invalid_type", param+" debe ser un número entero")
				continue
			}
		case "decimal":
			if n, err := strconv.ParseFloat(value, 64); err != nil || math.IsInf(n, 0) || math.IsNaN(n) {
				v.Add(param, "invalid_type", param+" debe ser un número")
				continue
			}
		case "boolean":
			if value != "true" && value != "false" {
				v.Add(param, "invalid_type", param+" debe ser true o false")
				continue
			}
			filter.Op = OpEqual
		case "enum":
			if !slices.Contains(def.AllowedValues, value) {
				v.Add(param, "one_of", fmt.Sprintf("%s debe ser uno de: %s", param, strings.Join(def.AllowedValues, ", ")))
				continue
			}
			filter.Op = OpEqual
		case "string":
			if utf8.RuneCountInString(value) > 255 {
				v.Add(param, "max_length", param+" no puede superar 255 caracteres")
				continue
			}
			filter.Op = OpEqualInsensitive
		}
		if numeric && bound == "" {
			filter.Op = OpEqualNumeric
		}
		filters = append(filters, filter)
	}
	return filters
}

func parseMoney(v *httpx.Validator, query url.Values, field string) *string {
	if !query.Has(field) {
		return nil
	}
	value := strings.TrimSpace(query.Get(field))
	if !moneyPattern.MatchString(value) {
		v.Add(field, "format", field+" debe ser un monto no negativo con hasta 2 decimales")
		return nil
	}
	return &value
}

func parsePage(v *httpx.Validator, query url.Values, q *Query) {
	if query.Has("limit") {
		limit, err := strconv.Atoi(query.Get("limit"))
		switch {
		case err != nil:
			v.Add("limit", "invalid_type", "limit debe ser un número entero")
		case limit < 1:
			v.Add("limit", "min", "limit debe ser mayor o igual a 1")
		case limit > MaxLimit:
			v.Add("limit", "max", fmt.Sprintf("limit no puede superar %d", MaxLimit))
		default:
			q.Limit = limit
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
			q.Offset = offset
		}
	}
}

func writeInvalidQuery(w http.ResponseWriter, r *http.Request, v httpx.Validator) {
	httpx.WriteProblem(w, r, http.StatusBadRequest, httpx.CodeInvalidParameter,
		"Los parámetros de la consulta no son válidos.", v.Errors()...)
}

func writeJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(body)
}
