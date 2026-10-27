package ingestion

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
)

// Handler atiende Ingestion API. Todas las rutas exigen admin o
// INTERNAL_API_TOKEN (authz.RequireAdmin en el router).
type Handler struct {
	runs      RunStore
	processor *Processor
	logger    *slog.Logger
}

func NewHandler(runs RunStore, processor *Processor, logger *slog.Logger) *Handler {
	return &Handler{runs: runs, processor: processor, logger: logger}
}

// OpenRun atiende POST /ingestion/runs: 201 + Location con la ejecución.
func (h *Handler) OpenRun(w http.ResponseWriter, r *http.Request) {
	var request OpenRunRequest
	if err := httpx.DecodeJSON(w, r, &request, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteDecodeError(w, r, err)
		return
	}
	name := strings.TrimSpace(request.Source.Name)
	baseURL := strings.TrimSpace(request.Source.BaseURL)
	scraperType := strings.TrimSpace(request.ScraperType)
	if scraperType == "" {
		scraperType = DefaultScraper
	}

	var v httpx.Validator
	v.Required("source.name", name)
	v.MaxLength("source.name", name, MaxSourceName)
	v.Check(baseURL == "" || isHTTPURL(baseURL), "source.baseUrl", "format", "source.baseUrl debe ser una URL http(s) válida")
	v.Check(request.StoreID > 0, "storeId", "min", "storeId debe ser mayor que 0")
	v.MaxLength("scraperType", scraperType, MaxScraperType)
	if !v.Valid() {
		httpx.WriteValidation(w, r, v.Errors())
		return
	}

	var base *string
	if baseURL != "" {
		base = &baseURL
	}
	id, err := h.runs.OpenRun(r.Context(), name, base, request.StoreID, scraperType)
	switch {
	case errors.Is(err, ErrStoreNotFound):
		httpx.WriteValidation(w, r, []httpx.FieldError{{Field: "storeId", Code: "not_found", Message: "storeId no existe"}})
		return
	case errors.Is(err, ErrSourceStoreMismatch):
		httpx.WriteProblem(w, r, http.StatusConflict, httpx.CodeConflict, "La fuente ya está asociada a otra tienda.")
		return
	case errors.Is(err, ErrSourceInactive):
		httpx.WriteProblem(w, r, http.StatusConflict, httpx.CodeConflict, "La fuente está desactivada.")
		return
	case err != nil:
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}

	run, err := h.runs.GetRun(r.Context(), id)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	w.Header().Set("Location", "/ingestion/runs/"+strconv.Itoa(int(id)))
	writeJSON(w, http.StatusCreated, run)
}

// AddItems atiende POST /ingestion/runs/{id}/items: procesa el lote y
// responde un resultado por ítem (200 aunque algunos fallen).
func (h *Handler) AddItems(w http.ResponseWriter, r *http.Request) {
	id, ok := parseRunID(w, r)
	if !ok {
		return
	}
	var request ItemsRequest
	if err := httpx.DecodeJSON(w, r, &request, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteDecodeError(w, r, err)
		return
	}
	var v httpx.Validator
	v.Check(len(request.Items) >= 1, "items", "min", "items debe tener al menos 1 elemento")
	v.Check(len(request.Items) <= MaxBatchItems, "items", "max", fmt.Sprintf("items admite hasta %d elementos por lote", MaxBatchItems))
	if !v.Valid() {
		httpx.WriteValidation(w, r, v.Errors())
		return
	}

	storeID, err := h.runs.ItemsTarget(r.Context(), id)
	if err != nil {
		h.writeRunError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, h.processor.Process(r.Context(), id, storeID, request.Items))
}

// FinishRun atiende POST /ingestion/runs/{id}/finish.
func (h *Handler) FinishRun(w http.ResponseWriter, r *http.Request) {
	id, ok := parseRunID(w, r)
	if !ok {
		return
	}
	var request FinishRequest
	if err := httpx.DecodeJSON(w, r, &request, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteDecodeError(w, r, err)
		return
	}
	message := strings.TrimSpace(request.ErrorMessage)
	var v httpx.Validator
	v.Check(request.Status == StatusSuccess || request.Status == StatusFailed, "status", "one_of", "status debe ser success o failed")
	v.MaxLength("errorMessage", message, MaxErrorMessage)
	if !v.Valid() {
		httpx.WriteValidation(w, r, v.Errors())
		return
	}

	var msg *string
	if message != "" {
		msg = &message
	}
	if err := h.runs.FinishRun(r.Context(), id, request.Status, msg); err != nil {
		h.writeRunError(w, r, err)
		return
	}
	h.writeRun(w, r, id)
}

// GetRun atiende GET /ingestion/runs/{id}.
func (h *Handler) GetRun(w http.ResponseWriter, r *http.Request) {
	id, ok := parseRunID(w, r)
	if !ok {
		return
	}
	h.writeRun(w, r, id)
}

// ListRuns atiende GET /ingestion/runs?limit=&offset= (X-Total-Count).
func (h *Handler) ListRuns(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	limit, offset := DefaultListLimit, 0
	var v httpx.Validator
	if query.Has("limit") {
		value, err := strconv.Atoi(query.Get("limit"))
		switch {
		case err != nil:
			v.Add("limit", "invalid_type", "limit debe ser un número entero")
		case value < 1:
			v.Add("limit", "min", "limit debe ser mayor o igual a 1")
		case value > MaxListLimit:
			v.Add("limit", "max", fmt.Sprintf("limit no puede superar %d", MaxListLimit))
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
	if !v.Valid() {
		httpx.WriteProblem(w, r, http.StatusBadRequest, httpx.CodeInvalidParameter,
			"Los parámetros de la consulta no son válidos.", v.Errors()...)
		return
	}

	runs, total, err := h.runs.ListRuns(r.Context(), limit, offset)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	if runs == nil {
		runs = []Run{}
	}
	w.Header().Set("X-Total-Count", strconv.FormatInt(total, 10))
	writeJSON(w, http.StatusOK, runs)
}

func (h *Handler) writeRun(w http.ResponseWriter, r *http.Request, id int32) {
	run, err := h.runs.GetRun(r.Context(), id)
	if err != nil {
		h.writeRunError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (h *Handler) writeRunError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrRunNotFound):
		httpx.WriteProblem(w, r, http.StatusNotFound, httpx.CodeNotFound, "La ejecución no existe.")
	case errors.Is(err, ErrRunClosed):
		httpx.WriteProblem(w, r, http.StatusConflict, httpx.CodeConflict, "La ejecución ya está cerrada.")
	case errors.Is(err, ErrRunWithoutStore):
		httpx.WriteProblem(w, r, http.StatusConflict, httpx.CodeConflict,
			"La fuente de la ejecución no está asociada a una tienda.")
	default:
		httpx.WriteInternal(w, r, h.logger, err)
	}
}

func parseRunID(w http.ResponseWriter, r *http.Request) (int32, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
	if err != nil || id <= 0 {
		httpx.WriteProblem(w, r, http.StatusBadRequest, httpx.CodeInvalidParameter, "El id de la ejecución no es válido.",
			httpx.FieldError{Field: "id", Code: "format", Message: "id debe ser un entero positivo"})
		return 0, false
	}
	return int32(id), true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
