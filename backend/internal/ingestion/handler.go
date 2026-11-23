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

// FinishRun atiende POST /ingestion/runs/{id}/finish: valida el body, cierra
// la corrida con su estado real (finalStatus) y responde la corrida como el
// GET. Con los contadores del scraper, o si ingestion rechazó todos los ítems,
// evalúa además si hay que avisar a los administradores (TAL-211, TAL-250).
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
	counters := finishCounters(&v, request)
	if !v.Valid() {
		httpx.WriteValidation(w, r, v.Errors())
		return
	}

	run, err := h.runs.GetRun(r.Context(), id)
	if err != nil {
		h.writeRunError(w, r, err)
		return
	}
	status, message := finalStatus(request.Status, message, run)
	if status != request.Status {
		h.logger.Warn("ingestion_run_all_items_failed", "runId", id, "recordsFailed", run.RecordsFailed)
	}
	var msg *string
	if message != "" {
		msg = &message
	}
	if err := h.runs.FinishRun(r.Context(), id, status, msg); err != nil {
		h.writeRunError(w, r, err)
		return
	}
	h.checkDegradation(r.Context(), id, counters)
	h.writeRun(w, r, id)
}

// finalStatus decide con qué estado y mensaje se cierra la corrida.
//
// Recibe el estado y el mensaje que pidió el scraper y la corrida tal como
// está en scrape_run. Si el scraper dice success pero ingestion no procesó
// ningún ítem y rechazó al menos uno, la corrida se cierra failed: el scraper
// no ve el resultado de cada ítem y antes quedaban corridas "exitosas" con
// 26/26 ítems fallidos (TAL-250). El mensaje del scraper, si trae uno, se
// conserva a continuación del nuestro. Devuelve el estado y el mensaje finales.
func finalStatus(requested, message string, run Run) (string, string) {
	if requested != StatusSuccess || run.RecordsProcessed > 0 || run.RecordsFailed <= 0 {
		return requested, message
	}
	reason := fmt.Sprintf("Ingestion rechazó los %d ítems de la corrida.", run.RecordsFailed)
	if message != "" {
		reason += " " + message
	}
	if runes := []rune(reason); len(runes) > MaxErrorMessage {
		reason = string(runes[:MaxErrorMessage])
	}
	return StatusFailed, reason
}

// checkDegradation evalúa la corrida recién cerrada y avisa a los
// administradores cuando la tienda entregó mucho menos de lo habitual o
// cuando ingestion rechazó todos sus ítems. Recibe el id de la corrida y los
// contadores del scraper (en cero si no los envió).
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
	verdict := allItemsFailed(run)
	if counters.Found > 0 {
		baseline, err := h.runs.RecentFound(ctx, *run.StoreID, id, thresholds.BaselineRuns)
		if err != nil {
			h.logger.Warn("ingestion_degradation_without_baseline", "runId", id, "error", err)
			baseline = nil
		}
		scraper := EvaluateDegradation(counters, baseline, thresholds)
		verdict.Reasons = append(verdict.Reasons, scraper.Reasons...)
		verdict.Degraded = verdict.Degraded || scraper.Degraded
	}
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
// Devuelve los contadores. Un valor negativo o mayor que MaxCounterValue es
// un error de validación por campo; los campos ausentes quedan en cero y no
// alteran la evaluación.
func finishCounters(v *httpx.Validator, request FinishRequest) Counters {
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
	for _, entry := range entries {
		if entry.value == nil {
			continue
		}
		switch {
		case *entry.value < 0:
			v.Add(entry.field, "min", entry.field+" no puede ser negativo")
		case *entry.value > MaxCounterValue:
			v.Add(entry.field, "max", fmt.Sprintf("%s no puede superar %d", entry.field, MaxCounterValue))
		default:
			*entry.target = *entry.value
		}
	}
	return counters
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
