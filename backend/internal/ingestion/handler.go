package ingestion

import (
	"context"
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
// INGESTION_API_TOKEN, el token del scraper (ingestionOnly en el router, TAL-199).
type Handler struct {
	runs      RunStore
	processor *Processor
	notifier  Notifier
	logger    *slog.Logger
}

// NewHandler arma el handler HTTP de ingestion con el almacén de ejecuciones,
// el procesador de ítems, el notificador de administradores y el logger.
// Un `notifier` nulo desactiva las alertas sin romper las rutas.
func NewHandler(runs RunStore, processor *Processor, notifier Notifier, logger *slog.Logger) *Handler {
	return &Handler{runs: runs, processor: processor, notifier: notifier, logger: logger}
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
	run, err := h.runs.GetRun(r.Context(), id)
	if err != nil {
		h.writeRunError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, h.processor.Process(r.Context(), id, storeID, run.Source, request.Items))
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
	counters, hasCounters := finishCounters(&v, request)
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
	if hasCounters {
		h.checkDegradation(r.Context(), id, counters)
	}
	h.writeRun(w, r, id)
}

// checkDegradation evalúa la corrida recién cerrada y avisa a los
// administradores cuando la tienda entregó mucho menos de lo habitual.
//
// Es deliberadamente tolerante: si no hay tienda, si el historial no se puede
// leer o si Notification API falla, deja un warning en el log y la corrida
// sigue cerrada. Perder una alerta nunca puede tumbar una ingesta.
func (h *Handler) checkDegradation(ctx context.Context, id int32, counters Counters) {
	if h.notifier == nil {
		return
	}
	thresholds := ThresholdsFromEnv()
	run, err := h.runs.GetRun(ctx, id)
	if err != nil || run.StoreID == nil {
		if err != nil {
			h.logger.Warn("ingestion_degradation_without_run", "runId", id, "error", err)
		}
		return
	}
	baseline, err := h.runs.RecentFound(ctx, *run.StoreID, id, thresholds.BaselineRuns)
	if err != nil {
		h.logger.Warn("ingestion_degradation_without_baseline", "runId", id, "error", err)
		baseline = nil
	}
	verdict := EvaluateDegradation(counters, baseline, thresholds)
	if !verdict.Degraded {
		return
	}

	notification := AdminNotification{
		Type:  NotificationTypeDegraded,
		Title: fmt.Sprintf("La tienda %s dejó de entregar productos", run.Source),
		Message: fmt.Sprintf(
			"La corrida %d de %s quedó degradada: %s.",
			id, run.Source, strings.Join(verdict.Reasons, "; "),
		),
		Data: map[string]any{
			"runId":     id,
			"storeId":   *run.StoreID,
			"source":    run.Source,
			"counters":  counters,
			"reasons":   verdict.Reasons,
			"status":    run.Status,
			"scraper":   run.ScraperType,
			"startedAt": run.StartedAt,
		},
	}
	if err := h.notifier.NotifyAdmins(ctx, notification); err != nil {
		h.logger.Warn("ingestion_degradation_notification_failed", "runId", id, "error", err)
	}
}

// finishCounters valida los contadores opcionales del cierre de corrida.
//
// Devuelve los contadores y si el cliente envió al menos uno. Un valor
// negativo o mayor que MaxCounterValue es un error de validación por campo;
// los campos ausentes quedan en cero y no alteran la evaluación.
func finishCounters(v *httpx.Validator, request FinishRequest) (Counters, bool) {
	var counters Counters
	entries := []struct {
		field  string
		value  *int
		target *int
	}{
		{"itemsFound", request.ItemsFound, &counters.Found},
		{"itemsSent", request.ItemsSent, &counters.Sent},
		{"itemsRejected", request.ItemsRejected, &counters.Rejected},
		{"itemsDuplicated", request.ItemsDuplicated, &counters.Duplicated},
		{"pagesFailed", request.PagesFailed, &counters.PagesFailed},
	}
	provided := false
	for _, entry := range entries {
		if entry.value == nil {
			continue
		}
		provided = true
		switch {
		case *entry.value < 0:
			v.Add(entry.field, "min", entry.field+" no puede ser negativo")
		case *entry.value > MaxCounterValue:
			v.Add(entry.field, "max", fmt.Sprintf("%s no puede superar %d", entry.field, MaxCounterValue))
		default:
			*entry.target = *entry.value
		}
	}
	return counters, provided
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

// writeRun responde 200 con la ejecución indicada, leída de la base.
func (h *Handler) writeRun(w http.ResponseWriter, r *http.Request, id int32) {
	run, err := h.runs.GetRun(r.Context(), id)
	if err != nil {
		h.writeRunError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

// writeRunError traduce un error de ejecución a su respuesta problem+json:
// 404 si no existe, 409 si está cerrada o sin tienda, 500 en el resto.
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

// parseRunID lee el id de la ejecución desde la ruta. Si no es un entero
// positivo responde 400 y devuelve false.
func parseRunID(w http.ResponseWriter, r *http.Request) (int32, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
	if err != nil || id <= 0 {
		httpx.WriteProblem(w, r, http.StatusBadRequest, httpx.CodeInvalidParameter, "El id de la ejecución no es válido.",
			httpx.FieldError{Field: "id", Code: "format", Message: "id debe ser un entero positivo"})
		return 0, false
	}
	return int32(id), true
}

// writeJSON escribe body como JSON con el código de estado indicado.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
