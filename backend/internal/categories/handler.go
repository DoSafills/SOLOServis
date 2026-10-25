package categories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
)

// Límites de paginación de GET /categories.
const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// slugPattern replica el CHECK de categories.slug.
var slugPattern = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)

// ValidSlug indica si s cumple el formato de categories.slug.
func ValidSlug(s string) bool {
	return slugPattern.MatchString(s)
}

// Repository es lo que el handler necesita de la capa de datos (TAL-78 §5).
type Repository interface {
	List(ctx context.Context, params ListParams) ([]CategorySummary, int64, error)
	GetBySlug(ctx context.Context, slug string) (CategoryDetail, error)
}

type Handler struct {
	repository Repository
	logger     *slog.Logger
}

func NewHandler(repository Repository, logger *slog.Logger) *Handler {
	return &Handler{repository: repository, logger: logger}
}

// List atiende GET /categories?itemType=&limit=&offset=.
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
		items = []CategorySummary{}
	}

	writeJSON(w, CategoryPage{Items: items, Total: total, Limit: params.Limit, Offset: params.Offset})
}

// GetBySlug atiende GET /categories/{slug}.
func (h *Handler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !slugPattern.MatchString(slug) {
		httpx.WriteProblem(w, r, http.StatusBadRequest, httpx.CodeInvalidParameter,
			"El slug de la categoría no es válido.",
			httpx.FieldError{Field: "slug", Code: "format",
				Message: "slug debe tener entre 1 y 64 caracteres: minúsculas, dígitos o guiones"})
		return
	}

	detail, err := h.repository.GetBySlug(r.Context(), slug)
	if errors.Is(err, ErrNotFound) {
		httpx.WriteProblem(w, r, http.StatusNotFound, httpx.CodeNotFound, "La categoría no existe.")
		return
	}
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	if detail.Attributes == nil {
		detail.Attributes = []Attribute{}
	}

	writeJSON(w, detail)
}

func parseListParams(query url.Values) (ListParams, []httpx.FieldError) {
	params := ListParams{Limit: DefaultLimit}
	var v httpx.Validator

	if query.Has("itemType") {
		itemType := query.Get("itemType")
		if itemType == ItemTypeProduct || itemType == ItemTypeService {
			params.ItemType = &itemType
		} else {
			v.Add("itemType", "one_of", "itemType debe ser product o service")
		}
	}

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

func writeJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(body)
}
