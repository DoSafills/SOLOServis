package stores

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/DoSafills/SOLOServis/backend/internal/stores/dto"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Handler struct {
	repository Reader
	logger     *slog.Logger
}

func NewHandler(repository Reader, logger *slog.Logger) *Handler {
	return &Handler{repository: repository, logger: logger}
}

// List atiende GET /stores?limit=&offset=.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	var v httpx.Validator
	page := parsePage(&v, r.URL.Query())
	if !v.Valid() {
		writeInvalidQuery(w, r, v)
		return
	}

	stores, total, err := h.repository.List(r.Context(), page)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	if stores == nil {
		stores = []dto.Store{}
	}
	writeList(w, stores, total)
}

// GetByID atiende GET /stores/{id}: tienda con productos disponibles y sucursales.
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	storeID, ok := parseStoreID(w, r)
	if !ok {
		return
	}

	detail, err := h.repository.GetDetail(r.Context(), storeID)
	if errors.Is(err, ErrNotFound) {
		writeNotFound(w, r, "La tienda no existe.")
		return
	}
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	writeJSON(w, detail)
}

// ListProductOffers atiende GET /stores/offers?product={publicID}&available=:
// las ofertas de un producto en todas las tiendas, para compararlas.
func (h *Handler) ListProductOffers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	var v httpx.Validator

	var productID pgtype.UUID
	if !query.Has("product") || query.Get("product") == "" {
		v.Add("product", "required", "product es obligatorio (UUID del producto)")
	} else if err := productID.Scan(query.Get("product")); err != nil {
		v.Add("product", "format", "product debe ser un UUID")
	}
	filter := parseOfferFilter(&v, query)
	if !v.Valid() {
		writeInvalidQuery(w, r, v)
		return
	}

	offers, total, err := h.repository.ListProductOffers(r.Context(), productID, filter)
	if errors.Is(err, ErrProductNotFound) {
		writeNotFound(w, r, "El producto no existe.")
		return
	}
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	writeOffers(w, offers, total)
}

// ListStoreOffers atiende GET /stores/{id}/offers?available=&limit=&offset=.
func (h *Handler) ListStoreOffers(w http.ResponseWriter, r *http.Request) {
	storeID, ok := parseStoreID(w, r)
	if !ok {
		return
	}
	var v httpx.Validator
	filter := parseOfferFilter(&v, r.URL.Query())
	if !v.Valid() {
		writeInvalidQuery(w, r, v)
		return
	}

	offers, total, err := h.repository.ListStoreOffers(r.Context(), storeID, filter)
	if errors.Is(err, ErrNotFound) {
		writeNotFound(w, r, "La tienda no existe.")
		return
	}
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	writeOffers(w, offers, total)
}

func parseStoreID(w http.ResponseWriter, r *http.Request) (int32, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
	if err != nil || id <= 0 {
		httpx.WriteProblem(w, r, http.StatusBadRequest, httpx.CodeInvalidParameter,
			"El id de la tienda no es válido.",
			httpx.FieldError{Field: "id", Code: "format", Message: "id debe ser un entero positivo"})
		return 0, false
	}
	return int32(id), true
}

func parseOfferFilter(v *httpx.Validator, query url.Values) OfferFilter {
	filter := OfferFilter{Page: parsePage(v, query)}
	if query.Has("available") {
		switch query.Get("available") {
		case "true", "false":
			available := query.Get("available") == "true"
			filter.Available = &available
		default:
			v.Add("available", "one_of", "available debe ser true o false")
		}
	}
	return filter
}

func parsePage(v *httpx.Validator, query url.Values) Page {
	page := Page{Limit: DefaultLimit}

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
			page.Limit = limit
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
			page.Offset = offset
		}
	}

	return page
}

func writeInvalidQuery(w http.ResponseWriter, r *http.Request, v httpx.Validator) {
	httpx.WriteProblem(w, r, http.StatusBadRequest, httpx.CodeInvalidParameter,
		"Los parámetros de la consulta no son válidos.", v.Errors()...)
}

func writeNotFound(w http.ResponseWriter, r *http.Request, detail string) {
	httpx.WriteProblem(w, r, http.StatusNotFound, httpx.CodeNotFound, detail)
}

func writeOffers(w http.ResponseWriter, offers []dto.Offer, total int64) {
	if offers == nil {
		offers = []dto.Offer{}
	}
	writeList(w, offers, total)
}

func writeList(w http.ResponseWriter, body any, total int64) {
	w.Header().Set("X-Total-Count", strconv.FormatInt(total, 10))
	writeJSON(w, body)
}

func writeJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(body)
}
