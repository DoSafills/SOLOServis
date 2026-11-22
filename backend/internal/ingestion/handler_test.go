package ingestion

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/DoSafills/SOLOServis/backend/internal/categories"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/DoSafills/SOLOServis/backend/internal/pricing"
	"github.com/DoSafills/SOLOServis/backend/internal/products"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var errBoom = errors.New(`relation "scrape_run" does not exist`)

const newProductID = "99999999-9999-9999-9999-999999999999"

func floatp(f float64) *float64 { return &f }
func int32p(i int32) *int32     { return &i }
func strp(s string) *string     { return &s }

func mustJSON(t *testing.T, value string) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// refrigeradores: fila en categories con su hoja de nivel 3 y un requerido.
var refrigeradores = products.WriteCategory{
	ID: int32p(1), CategoryID: int32p(5), Slug: strp("refrigeradores"), ItemType: categories.ItemTypeProduct,
	Attributes: []categories.Attribute{
		{ID: 10, Key: "capacidad_litros", DataType: "integer", Required: true, Min: floatp(50), Max: floatp(1500)},
	},
}

// ------------------------------------------------------------------ fakes

// fakeTx cumple pgx.Tx para los dobles de prueba. Solo se ejercitan Commit y
// Rollback; los demás métodos del pgx.Tx embebido no se llaman.
type fakeTx struct {
	pgx.Tx
	commits   int
	rollbacks int
	commitErr error
}

// Commit simula el cierre de la transacción del ítem y cuenta la llamada.
func (t *fakeTx) Commit(context.Context) error {
	t.commits++
	return t.commitErr
}

// Rollback simula la reversión de la transacción del ítem.
func (t *fakeTx) Rollback(context.Context) error {
	t.rollbacks++
	return nil
}

type fakeRuns struct {
	run                Run
	storeID            int32
	targetErr, openErr error
	finishErr, getErr  error
	existing           map[string]ExistingProduct // por sku, "marca|modelo" o "tienda|url"
	findErr            error
	images             map[int32]string // imagen activa por id de producto
	addImageErr        error
	saveBatchErr       error // error del INSERT por lote de scraped_data
	beginErr           error // error al abrir la transacción del ítem

	imageCalls  []string // "id|url" de cada llamada a AddImageIfMissing
	opened      []any
	counters    []int
	saved       []string // external_id de cada fila guardada, en orden
	batches     int      // llamadas a SaveScrapedBatch
	singles     int      // llamadas a SaveScraped (respaldo)
	brands      []string
	finished    []any
	baseline    []int32
	baselineErr error
	txs         []*fakeTx // transacciones abiertas por Processor
}

func (f *fakeRuns) OpenRun(_ context.Context, name string, baseURL *string, storeID int32, scraperType string) (int32, error) {
	f.opened = []any{name, baseURL, storeID, scraperType}
	return 3, f.openErr
}
func (f *fakeRuns) GetRun(context.Context, int32) (Run, error) { return f.run, f.getErr }
func (f *fakeRuns) ListRuns(context.Context, int, int) ([]Run, int64, error) {
	return []Run{f.run}, 1, f.getErr
}
func (f *fakeRuns) FinishRun(_ context.Context, id int32, status string, msg *string) error {
	f.finished = []any{id, status, msg}
	return f.finishErr
}
func (f *fakeRuns) ItemsTarget(context.Context, int32) (int32, error) { return f.storeID, f.targetErr }
func (f *fakeRuns) AddCounters(_ context.Context, _ int32, found, processed, failed int) error {
	f.counters = []int{found, processed, failed}
	return nil
}

// RecentFound devuelve el historial que dejó el test en `baseline`.
func (f *fakeRuns) RecentFound(_ context.Context, _, _ int32, _ int) ([]int32, error) {
	return f.baseline, f.baselineErr
}

// SaveScraped simula el guardado de una fila de scraped_data (el respaldo
// cuando falla el lote): cuenta la llamada y anota su external_id.
func (f *fakeRuns) SaveScraped(_ context.Context, _ int32, externalID, _ *string, _ json.RawMessage) error {
	f.singles++
	id := ""
	if externalID != nil {
		id = *externalID
	}
	f.saved = append(f.saved, id)
	return nil
}

// SaveScrapedBatch simula el INSERT por lote: cuenta la llamada y, si
// saveBatchErr no está definido, anota el external_id de cada fila en orden.
func (f *fakeRuns) SaveScrapedBatch(_ context.Context, _ int32, items []ScrapedItem) error {
	f.batches++
	if f.saveBatchErr != nil {
		return f.saveBatchErr
	}
	for _, item := range items {
		id := ""
		if item.ExternalID != nil {
			id = *item.ExternalID
		}
		f.saved = append(f.saved, id)
	}
	return nil
}

// BeginTx simula la apertura de la transacción del ítem (TAL-191).
func (f *fakeRuns) BeginTx(context.Context) (pgx.Tx, error) {
	if f.beginErr != nil {
		return nil, f.beginErr
	}
	tx := &fakeTx{}
	f.txs = append(f.txs, tx)
	return tx, nil
}

// FindProductTx simula la búsqueda de identidad dentro de la transacción:
// sku, marca+modelo, tienda+URL y, por último, marca+nombre+categoría+tienda
// (TAL-191).
func (f *fakeRuns) FindProductTx(_ context.Context, _ pgx.Tx, sku, brand, model, name, categorySlug string, storeID int32, productURL string) (ExistingProduct, bool, error) {
	if f.findErr != nil {
		return ExistingProduct{}, false, f.findErr
	}
	if p, ok := f.existing[sku]; ok && sku != "" {
		return p, true, nil
	}
	if p, ok := f.existing[brand+"|"+model]; ok && brand != "" && model != "" {
		return p, true, nil
	}
	path, _, _ := strings.Cut(productURL, "?")
	if p, ok := f.existing[fmt.Sprintf("%d|%s", storeID, path)]; ok && productURL != "" {
		return p, true, nil
	}
	nameKey := fmt.Sprintf("%s|%s|%s|%d", strings.ToLower(brand), strings.ToLower(name), categorySlug, storeID)
	p, ok := f.existing[nameKey]
	return p, ok && brand != "" && name != "" && categorySlug != "", nil
}

// AddImageIfMissing simula al repositorio: registra la llamada, devuelve
// addImageErr si está definido y guarda la imagen solo si el producto no
// tiene una. Devuelve true si la guardó.
func (f *fakeRuns) AddImageIfMissingTx(_ context.Context, _ pgx.Tx, productID int32, imageURL string) (bool, error) {
	f.imageCalls = append(f.imageCalls, fmt.Sprintf("%d|%s", productID, imageURL))
	if f.addImageErr != nil {
		return false, f.addImageErr
	}
	if _, ok := f.images[productID]; ok {
		return false, nil
	}
	if f.images == nil {
		f.images = map[int32]string{}
	}
	f.images[productID] = imageURL
	return true, nil
}
func (f *fakeRuns) EnsureBrandTx(_ context.Context, _ pgx.Tx, name string) (int32, error) {
	f.brands = append(f.brands, name)
	return 77, nil
}
func (f *fakeRuns) ProductIDTx(context.Context, pgx.Tx, pgtype.UUID) (int32, error) { return 500, nil }

type fakeCatalog struct {
	category      products.WriteCategory
	resolveErr    error
	resolveErrors map[string]error
	createErr     error
	created       []products.ProductWrite
	resolved      []products.CategoryRef
}

func (c *fakeCatalog) ResolveCategory(_ context.Context, ref products.CategoryRef) (products.WriteCategory, error) {
	c.resolved = append(c.resolved, ref)
	if err := c.resolveErrors[ref.Slug]; err != nil {
		return products.WriteCategory{}, err
	}
	return c.category, c.resolveErr
}
func (c *fakeCatalog) CreateTx(_ context.Context, _ pgx.Tx, w products.ProductWrite) (pgtype.UUID, error) {
	c.created = append(c.created, w)
	var id pgtype.UUID
	_ = id.Scan(newProductID)
	return id, c.createErr
}

type fakeOffers struct {
	previous     *string
	upsertErr    error
	watchers     []pricing.Watcher
	written      []pricing.OfferWrite
	getCalls     int // llamadas a GetOffer
	watcherCalls int // llamadas a PriceDropWatchers
}

func (o *fakeOffers) UpsertOfferTx(_ context.Context, _ pgx.Tx, w pricing.OfferWrite) (int32, bool, *string, error) {
	o.written = append(o.written, w)
	return int32(40 + len(o.written)), o.previous == nil, o.previous, o.upsertErr
}

// GetOffer simula la lectura de la oferta guardada y cuenta la llamada.
func (o *fakeOffers) GetOffer(_ context.Context, id int32) (pricing.Offer, error) {
	o.getCalls++
	return pricing.Offer{ID: id, Price: "90.00"}, nil
}

// PriceDropWatchers simula la consulta de usuarios a avisar y cuenta la llamada.
func (o *fakeOffers) PriceDropWatchers(context.Context, int32, string, string) ([]pricing.Watcher, error) {
	o.watcherCalls++
	return o.watchers, nil
}

type fakeNotifier struct{ sent []pricing.Notification }

func (n *fakeNotifier) Notify(_ context.Context, notification pricing.Notification) error {
	n.sent = append(n.sent, notification)
	return nil
}

// fakeAdminNotifier registra los avisos de extracción degradada (TAL-211).
type fakeAdminNotifier struct {
	sent []AdminNotification
	err  error
}

// NotifyAdmins acumula el aviso y devuelve el error que el test configure.
func (n *fakeAdminNotifier) NotifyAdmins(_ context.Context, notification AdminNotification) error {
	n.sent = append(n.sent, notification)
	return n.err
}

type env struct {
	runs     *fakeRuns
	catalog  *fakeCatalog
	offers   *fakeOffers
	notifier *fakeNotifier
	alerts   *fakeAdminNotifier
	logs     *bytes.Buffer
}

func newEnv() *env {
	return &env{
		runs:     &fakeRuns{storeID: 2, run: Run{ID: 3, Source: "Ripley scraper", Status: StatusRunning}, existing: map[string]ExistingProduct{}},
		catalog:  &fakeCatalog{category: refrigeradores},
		offers:   &fakeOffers{},
		notifier: &fakeNotifier{},
		alerts:   &fakeAdminNotifier{},
		logs:     &bytes.Buffer{},
	}
}

func (e *env) serve(t *testing.T, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	logger := slog.New(slog.NewJSONHandler(e.logs, nil))
	processor := NewProcessor(e.runs, e.catalog, e.offers, e.notifier, logger)
	h := NewHandler(e.runs, processor, e.alerts, logger)
	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Post("/ingestion/runs", h.OpenRun)
	r.Get("/ingestion/runs", h.ListRuns)
	r.Get("/ingestion/runs/{id}", h.GetRun)
	r.Post("/ingestion/runs/{id}/items", h.AddItems)
	r.Post("/ingestion/runs/{id}/finish", h.FinishRun)

	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func requireStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, quiero %d (%s)", rec.Code, want, rec.Body.String())
	}
}

func problemFields(t *testing.T, rec *httptest.ResponseRecorder) []string {
	t.Helper()
	var p httpx.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || p.RequestID == "" {
		t.Fatalf("no es problem+json válido: %s", rec.Body.String())
	}
	out := []string{}
	for _, e := range p.Errors {
		out = append(out, e.Field+"/"+e.Code)
	}
	return out
}

func batch(t *testing.T, e *env, items string) BatchResult {
	t.Helper()
	rec := e.serve(t, http.MethodPost, "/ingestion/runs/3/items", `{"items":[`+items+`]}`)
	requireStatus(t, rec, http.StatusOK)
	var result BatchResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func codes(r ItemResult) []string {
	out := []string{}
	for _, e := range r.Errors {
		out = append(out, e.Field+"/"+e.Code)
	}
	return out
}

// ------------------------------------------------------------ ejecuciones

func TestOpenRun(t *testing.T) {
	e := newEnv()
	rec := e.serve(t, http.MethodPost, "/ingestion/runs",
		`{"source":{"name":" Ripley scraper ","baseUrl":"https://simple.ripley.cl"},"storeId":2}`)
	requireStatus(t, rec, http.StatusCreated)
	if rec.Header().Get("Location") != "/ingestion/runs/3" {
		t.Fatalf("Location = %q", rec.Header().Get("Location"))
	}
	if e.runs.opened[0] != "Ripley scraper" || *e.runs.opened[1].(*string) != "https://simple.ripley.cl" ||
		e.runs.opened[2] != int32(2) || e.runs.opened[3] != DefaultScraper {
		t.Fatalf("OpenRun = %v", e.runs.opened)
	}

	for _, tc := range []struct {
		name, body string
		openErr    error
		status     int
		want       []string
	}{
		{"campos inválidos", `{"source":{"baseUrl":"ftp://x"},"storeId":0}`, nil, 422,
			[]string{"source.name/required", "source.baseUrl/format", "storeId/min"}},
		{"tienda inexistente", `{"source":{"name":"x"},"storeId":9}`, ErrStoreNotFound, 422, []string{"storeId/not_found"}},
		{"fuente de otra tienda", `{"source":{"name":"x"},"storeId":9}`, ErrSourceStoreMismatch, 409, []string{}},
		{"fuente inactiva", `{"source":{"name":"x"},"storeId":9}`, ErrSourceInactive, 409, []string{}},
		{"campo desconocido", `{"source":{"name":"x"},"storeId":9,"extra":1}`, nil, 400, []string{"extra/unknown_field"}},
		{"error interno", `{"source":{"name":"x"},"storeId":9}`, errBoom, 500, []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv()
			e.runs.openErr = tc.openErr
			rec := e.serve(t, http.MethodPost, "/ingestion/runs", tc.body)
			requireStatus(t, rec, tc.status)
			if got := problemFields(t, rec); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("errores = %v, quiero %v", got, tc.want)
			}
			if strings.Contains(rec.Body.String(), "relation") {
				t.Fatal("el body expone el error interno")
			}
		})
	}
}

func TestFinishGetList(t *testing.T) {
	e := newEnv()
	requireStatus(t, e.serve(t, http.MethodPost, "/ingestion/runs/3/finish", `{"status":"failed","errorMessage":"timeout"}`), http.StatusOK)
	if e.runs.finished[1] != StatusFailed || *e.runs.finished[2].(*string) != "timeout" {
		t.Fatalf("finish = %v", e.runs.finished)
	}

	rec := newEnv().serve(t, http.MethodPost, "/ingestion/runs/3/finish", `{"status":"done"}`)
	requireStatus(t, rec, http.StatusUnprocessableEntity)
	if got := problemFields(t, rec); !reflect.DeepEqual(got, []string{"status/one_of"}) {
		t.Fatalf("errores = %v", got)
	}
	for err, status := range map[error]int{ErrRunClosed: 409, ErrRunNotFound: 404} {
		e := newEnv()
		e.runs.finishErr = err
		requireStatus(t, e.serve(t, http.MethodPost, "/ingestion/runs/3/finish", `{"status":"success"}`), status)
	}

	requireStatus(t, newEnv().serve(t, http.MethodGet, "/ingestion/runs/3", ""), http.StatusOK)
	requireStatus(t, newEnv().serve(t, http.MethodGet, "/ingestion/runs/x", ""), http.StatusBadRequest)
	missing := newEnv()
	missing.runs.getErr = ErrRunNotFound
	requireStatus(t, missing.serve(t, http.MethodGet, "/ingestion/runs/3", ""), http.StatusNotFound)

	rec = newEnv().serve(t, http.MethodGet, "/ingestion/runs?limit=5", "")
	requireStatus(t, rec, http.StatusOK)
	if rec.Header().Get("X-Total-Count") != "1" {
		t.Fatalf("X-Total-Count = %s", rec.Header().Get("X-Total-Count"))
	}
	requireStatus(t, newEnv().serve(t, http.MethodGet, "/ingestion/runs?limit=101", ""), http.StatusBadRequest)
}

// ------------------------------------------------------------------ TAL-211

// withStore deja la corrida del test asociada a una tienda, que es lo que la
// alerta necesita para encontrar el historial.
func withStore(e *env) *env {
	storeID := int32(2)
	e.runs.run.StoreID = &storeID
	return e
}

// TestFinishRunAlertsWhenDegraded comprueba el camino completo: el scraper
// manda contadores, ingestion los evalúa y avisa a los administradores.
func TestFinishRunAlertsWhenDegraded(t *testing.T) {
	e := withStore(newEnv())
	e.runs.baseline = []int32{30, 30, 30, 30, 30}

	body := `{"status":"success","itemsFound":30,"itemsSent":6,"itemsRejected":1,"pagesFailed":0}`
	requireStatus(t, e.serve(t, http.MethodPost, "/ingestion/runs/3/finish", body), http.StatusOK)

	if len(e.alerts.sent) != 1 {
		t.Fatalf("avisos = %d; quiero 1 (log: %s)", len(e.alerts.sent), e.logs.String())
	}
	alert := e.alerts.sent[0]
	if alert.Type != NotificationTypeDegraded {
		t.Fatalf("type = %q", alert.Type)
	}
	if !strings.Contains(alert.Title, "Ripley scraper") {
		t.Fatalf("title = %q", alert.Title)
	}
	if !strings.Contains(alert.Message, "mediana de 30") {
		t.Fatalf("message = %q", alert.Message)
	}
	if alert.Data["runId"] != int32(3) {
		t.Fatalf("data.runId = %v", alert.Data["runId"])
	}
}

// TestFinishRunWithoutCountersDoesNotAlert deja claro que el contrato viejo
// sigue funcionando: sin contadores no se evalúa nada.
func TestFinishRunWithoutCountersDoesNotAlert(t *testing.T) {
	e := withStore(newEnv())
	e.runs.baseline = []int32{30}

	requireStatus(t, e.serve(t, http.MethodPost, "/ingestion/runs/3/finish", `{"status":"success"}`), http.StatusOK)

	if len(e.alerts.sent) != 0 {
		t.Fatalf("avisos = %d; quiero 0", len(e.alerts.sent))
	}
}

// TestFinishRunHealthyRunDoesNotAlert evita alertar por una corrida normal.
func TestFinishRunHealthyRunDoesNotAlert(t *testing.T) {
	e := withStore(newEnv())
	e.runs.baseline = []int32{28, 29, 30}

	body := `{"status":"success","itemsFound":30,"itemsSent":29,"itemsRejected":1,"pagesFailed":1}`
	requireStatus(t, e.serve(t, http.MethodPost, "/ingestion/runs/3/finish", body), http.StatusOK)

	if len(e.alerts.sent) != 0 {
		t.Fatalf("avisos = %d; quiero 0", len(e.alerts.sent))
	}
}

// TestFinishRunRejectsInvalidCounters valida los contadores opcionales: un
// negativo o un valor absurdo es 422 con el campo culpable, no 500.
func TestFinishRunRejectsInvalidCounters(t *testing.T) {
	tests := []struct {
		body  string
		field string
	}{
		{`{"status":"success","itemsSent":-1}`, "itemsSent/min"},
		{`{"status":"success","itemsFound":1000001}`, "itemsFound/max"},
		{`{"status":"success","itemsDuplicated":-1}`, "itemsDuplicated/min"},
		{`{"status":"success","itemsDuplicated":1000001}`, "itemsDuplicated/max"},
	}
	for _, test := range tests {
		t.Run(test.field, func(t *testing.T) {
			rec := withStore(newEnv()).serve(t, http.MethodPost, "/ingestion/runs/3/finish", test.body)
			requireStatus(t, rec, http.StatusUnprocessableEntity)
			if got := problemFields(t, rec); !reflect.DeepEqual(got, []string{test.field}) {
				t.Fatalf("errores = %v; quiero %v", got, test.field)
			}
		})
	}
}

// TestFinishRunSurvivesNotificationFailure comprueba que una alerta que no se
// puede entregar deja un warning y no cambia la respuesta del cierre.
func TestFinishRunSurvivesNotificationFailure(t *testing.T) {
	e := withStore(newEnv())
	e.alerts.err = errors.New("notification caída")

	body := `{"status":"success","itemsFound":10,"itemsSent":0}`
	requireStatus(t, e.serve(t, http.MethodPost, "/ingestion/runs/3/finish", body), http.StatusOK)

	if !strings.Contains(e.logs.String(), "ingestion_degradation_notification_failed") {
		t.Fatalf("log = %s", e.logs.String())
	}
}

// TestFinishRunWithoutStoreSkipsAlert no puede avisar si la fuente no tiene
// tienda: no hay historial que comparar y la corrida se cierra igual.
func TestFinishRunWithoutStoreSkipsAlert(t *testing.T) {
	e := newEnv()
	body := `{"status":"success","itemsFound":10,"itemsSent":0}`
	requireStatus(t, e.serve(t, http.MethodPost, "/ingestion/runs/3/finish", body), http.StatusOK)

	if len(e.alerts.sent) != 0 {
		t.Fatalf("avisos = %d; quiero 0", len(e.alerts.sent))
	}
}

// ------------------------------------------------------------------ lotes

func TestAddItems_ValidacionDelLoteYEstadoDeLaEjecucion(t *testing.T) {
	rec := newEnv().serve(t, http.MethodPost, "/ingestion/runs/3/items", `{"items":[]}`)
	requireStatus(t, rec, http.StatusUnprocessableEntity)
	if got := problemFields(t, rec); !reflect.DeepEqual(got, []string{"items/min"}) {
		t.Fatalf("errores = %v", got)
	}
	many := strings.Repeat(`{"product":{"name":"x"},"offer":{"price":1}},`, MaxBatchItems) + `{"product":{"name":"x"},"offer":{"price":1}}`
	rec = newEnv().serve(t, http.MethodPost, "/ingestion/runs/3/items", `{"items":[`+many+`]}`)
	requireStatus(t, rec, http.StatusUnprocessableEntity)

	for err, status := range map[error]int{ErrRunNotFound: 404, ErrRunClosed: 409, ErrRunWithoutStore: 409, errBoom: 500} {
		e := newEnv()
		e.runs.targetErr = err
		rec := e.serve(t, http.MethodPost, "/ingestion/runs/3/items", `{"items":[{"product":{"name":"x"},"offer":{"price":1}}]}`)
		requireStatus(t, rec, status)
		if len(e.offers.written) != 0 {
			t.Fatal("no debe procesar ítems si la ejecución no los acepta")
		}
	}
}

func TestAddItems_ProductoNuevo(t *testing.T) {
	e := newEnv()
	result := batch(t, e, `{"externalId":"rip-1","product":{"sku":"LG-1","name":"Refri LG","brand":"LG","model":"GT35",
		"categorySlug":"refrigeradores","imageUrl":"https://x.test/lg.png","attributes":{"capacidad_litros":350}},
		"offer":{"price":349990,"shippingFree":true,"productUrl":"https://x.test/p/1"}}`)

	r := result.Results[0]
	if result.Processed != 1 || r.Status != ItemCreated || r.ProductID != newProductID || r.OfferID != 41 || r.ExternalID != "rip-1" {
		t.Fatalf("resultado = %+v", result)
	}
	created := e.catalog.created[0]
	if created.CategoryID != 5 || *created.CategoryRef != 1 || *created.BrandID != 77 || created.SKU != "LG-1" ||
		len(created.Attributes) != 1 || created.Attributes[0].Text != "350" {
		t.Fatalf("producto creado = %+v", created)
	}
	offer := e.offers.written[0]
	if offer.ProductID != 500 || offer.StoreID != 2 || offer.Price != "349990" || !offer.ShippingFree || !offer.Available {
		t.Fatalf("oferta = %+v", offer)
	}
	if !reflect.DeepEqual(e.runs.counters, []int{1, 1, 0}) || !reflect.DeepEqual(e.runs.saved, []string{"rip-1"}) ||
		!reflect.DeepEqual(e.runs.brands, []string{"LG"}) {
		t.Fatalf("contadores = %v guardados = %v marcas = %v", e.runs.counters, e.runs.saved, e.runs.brands)
	}
}

// TestAddItems_CategoriasVigentesYNueva comprueba que los cuatro slugs que ya
// enviaba el scraper sigan ingresando y que un slug nuevo con fila en
// categories (lavadoras) también cree el producto en su hoja.
func TestAddItems_CategoriasVigentesYNueva(t *testing.T) {
	casos := []struct {
		slug       string
		category   products.WriteCategory
		attributes string
	}{
		{"refrigeradores", products.WriteCategory{ID: int32p(1), CategoryID: int32p(11), Slug: strp("refrigeradores"),
			ItemType:   categories.ItemTypeProduct,
			Attributes: refrigeradores.Attributes}, `{"capacidad_litros":350}`},
		{"notebooks", products.WriteCategory{ID: int32p(2), CategoryID: int32p(12), Slug: strp("notebooks"),
			ItemType:   categories.ItemTypeProduct,
			Attributes: []categories.Attribute{}}, `{}`},
		{"televisores", products.WriteCategory{ID: int32p(3), CategoryID: int32p(13), Slug: strp("televisores"),
			ItemType:   categories.ItemTypeProduct,
			Attributes: []categories.Attribute{}}, `{}`},
		{"celulares", products.WriteCategory{ID: int32p(4), CategoryID: int32p(14), Slug: strp("celulares"),
			ItemType:   categories.ItemTypeProduct,
			Attributes: []categories.Attribute{}}, `{}`},
		{"lavadoras", products.WriteCategory{ID: int32p(9), CategoryID: int32p(20), Slug: strp("lavadoras"),
			ItemType: categories.ItemTypeProduct,
			Attributes: []categories.Attribute{
				{ID: 40, Key: "capacidad_kg", DataType: "integer", Required: true, Min: floatp(5), Max: floatp(30)},
			}}, `{"capacidad_kg":12}`},
	}

	for _, caso := range casos {
		t.Run(caso.slug, func(t *testing.T) {
			e := newEnv()
			e.catalog.category = caso.category

			item := fmt.Sprintf(`{"product":{"name":"Producto","categorySlug":%q,"attributes":%s},"offer":{"price":1}}`,
				caso.slug, caso.attributes)
			result := batch(t, e, item)

			if result.Results[0].Status != ItemCreated || len(e.catalog.created) != 1 {
				t.Fatalf("resultado = %+v, quiero producto creado", result.Results[0])
			}
			created := e.catalog.created[0]
			if created.CategoryID != *caso.category.CategoryID ||
				created.CategoryRef == nil || *created.CategoryRef != *caso.category.ID {
				t.Fatalf("producto creado = %+v, quiero hoja %d ref %d",
					created, *caso.category.CategoryID, *caso.category.ID)
			}
		})
	}
}

func TestAddItems_ProductoExistenteSoloRegistraLaOferta(t *testing.T) {
	e := newEnv()
	e.runs.existing["LG-1"] = ExistingProduct{ID: 8, PublicID: "existente-sku", Active: true}
	e.runs.existing["Samsung|RT38"] = ExistingProduct{ID: 9, PublicID: "existente-modelo", Active: true}
	e.offers.previous = strp("100.00")
	e.offers.watchers = []pricing.Watcher{{UserID: 4, ProductName: "Refri"}}

	// El tercero trae un SKU propio de la tienda que no existe: se encuentra
	// igual por marca + modelo y no se duplica.
	result := batch(t, e, `{"product":{"sku":"LG-1","name":"cualquiera"},"offer":{"price":90}},
		{"product":{"name":"x","brand":"Samsung","model":"RT38"},"offer":{"price":90}},
		{"product":{"sku":"SKU-DE-LA-TIENDA","name":"x","brand":"Samsung","model":"RT38"},"offer":{"price":90}}`)

	if result.Processed != 3 || result.Results[0].Status != ItemUpdated || result.Results[0].ProductID != "existente-sku" ||
		result.Results[1].ProductID != "existente-modelo" || result.Results[2].ProductID != "existente-modelo" {
		t.Fatalf("resultado = %+v", result)
	}
	if len(e.catalog.created) != 0 {
		t.Fatal("un producto existente no se recrea ni se modifica")
	}
	if e.offers.written[0].ProductID != 8 || e.offers.written[1].ProductID != 9 {
		t.Fatalf("ofertas = %+v", e.offers.written)
	}
	if len(e.notifier.sent) != 3 || e.notifier.sent[0].Data["oldPrice"] != "100.00" {
		t.Fatalf("la bajada de precio debe avisar como en Pricing: %+v", e.notifier.sent)
	}
}

// TAL-169: el scraper no envía SKU ni modelo. La identidad que siempre trae es
// la tienda + la URL de la oferta, así que repetir un ítem no duplica el producto.
func TestAddItems_SinSkuNiModeloSeReconocePorTiendaYURL(t *testing.T) {
	e := newEnv()
	e.runs.existing["2|https://tienda.test/p/refri-1"] = ExistingProduct{ID: 12, PublicID: "existente-url", Active: true,
		OfferURL: "https://tienda.test/p/refri-1?pos=4"}

	result := batch(t, e, `{"product":{"name":"Refri Midea 174L","brand":"Midea","categorySlug":"refrigeradores",
		"attributes":{"capacidad_litros":174}},"offer":{"price":199990,"productUrl":" https://tienda.test/p/refri-1?pos=9 "}},
		{"product":{"name":"Refri Midea 515L","brand":"Midea","categorySlug":"refrigeradores",
		"attributes":{"capacidad_litros":515}},"offer":{"price":599990,"productUrl":"https://tienda.test/p/refri-2"}},
		{"product":{"name":"Refri sin URL","brand":"Midea","categorySlug":"refrigeradores",
		"attributes":{"capacidad_litros":300}},"offer":{"price":299990}}`)

	if result.Processed != 3 || result.Results[0].Status != ItemUpdated || result.Results[0].ProductID != "existente-url" {
		t.Fatalf("la misma tienda + URL debe reutilizar el producto: %+v", result)
	}
	if result.Results[1].Status != ItemCreated || result.Results[2].Status != ItemCreated || len(e.catalog.created) != 2 {
		t.Fatalf("otra URL o un ítem sin URL crean un producto nuevo: %+v", result)
	}
	if w := e.offers.written[0]; w.ProductID != 12 || *w.ProductURL != "https://tienda.test/p/refri-1?pos=4" {
		t.Fatalf("debe actualizar la oferta existente (su URL guardada), no crear otra: %+v", w)
	}
	if *e.offers.written[1].ProductURL != "https://tienda.test/p/refri-2" {
		t.Fatalf("un producto nuevo conserva la URL que envió el scraper: %+v", e.offers.written[1])
	}
}

// TAL-243: un producto existente sin imagen activa toma la imagen que envía la
// tienda; una imagen existente (curada por el admin) no se reemplaza.
func TestAddItems_ImagenDeProductoExistente(t *testing.T) {
	const existente = int32(8)
	for _, tc := range []struct {
		name       string
		image      string // product.imageUrl del ítem
		stored     string // imagen activa que ya tiene el producto ("" = ninguna)
		addErr     error
		wantStatus string
		wantCodes  []string
		wantCalls  []string
		wantImage  string // imagen del producto después del lote ("" = ninguna)
		wantOffers int
	}{
		{name: "producto sin imagen + ítem con imagen: la guarda",
			image: " https://img.test/refri.jpg ", wantStatus: ItemUpdated, wantCodes: []string{},
			wantCalls: []string{"8|https://img.test/refri.jpg"}, wantImage: "https://img.test/refri.jpg", wantOffers: 1},
		{name: "producto con imagen + ítem con otra: no la cambia",
			image: "https://img.test/otra.jpg", stored: "https://img.test/curada.jpg", wantStatus: ItemUpdated,
			wantCodes: []string{}, wantCalls: []string{"8|https://img.test/otra.jpg"},
			wantImage: "https://img.test/curada.jpg", wantOffers: 1},
		{name: "ítem sin imagen: no cambia nada",
			wantStatus: ItemUpdated, wantCodes: []string{}, wantCalls: nil, wantOffers: 1},
		{name: "error al guardar la imagen: el ítem falla como error interno",
			image: "https://img.test/refri.jpg", addErr: errBoom, wantStatus: ItemFailed,
			wantCodes: []string{"/internal_error"}, wantCalls: []string{"8|https://img.test/refri.jpg"}, wantOffers: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv()
			e.runs.existing["LG-1"] = ExistingProduct{ID: existente, PublicID: "existente-sku", Active: true}
			if tc.stored != "" {
				e.runs.images = map[int32]string{existente: tc.stored}
			}
			e.runs.addImageErr = tc.addErr
			image := ""
			if tc.image != "" {
				image = fmt.Sprintf(`,"imageUrl":%q`, tc.image)
			}

			result := batch(t, e, `{"product":{"sku":"LG-1","name":"Refri"`+image+`},"offer":{"price":90}}`)

			r := result.Results[0]
			if r.Status != tc.wantStatus || !reflect.DeepEqual(codes(r), tc.wantCodes) {
				t.Fatalf("resultado = %+v", r)
			}
			if !reflect.DeepEqual(e.runs.imageCalls, tc.wantCalls) {
				t.Fatalf("llamadas a AddImageIfMissing = %v, quiero %v", e.runs.imageCalls, tc.wantCalls)
			}
			if got := e.runs.images[existente]; got != tc.wantImage {
				t.Fatalf("imagen del producto = %q, quiero %q", got, tc.wantImage)
			}
			if len(e.offers.written) != tc.wantOffers || len(e.catalog.created) != 0 {
				t.Fatalf("ofertas = %d, productos creados = %d", len(e.offers.written), len(e.catalog.created))
			}
			if tc.addErr != nil {
				body, _ := json.Marshal(result)
				if strings.Contains(string(body), "relation") {
					t.Fatal("el resultado expone el error interno")
				}
				if !strings.Contains(e.logs.String(), "relation") || !strings.Contains(e.logs.String(), `"stage":"add_image"`) {
					t.Fatalf("el error interno debe quedar en el log con su etapa: %s", e.logs.String())
				}
			}
		})
	}
}

// Un producto nuevo guarda su imagen al crearse (Create); no pasa por
// AddImageIfMissing.
func TestAddItems_ProductoNuevoNoUsaAddImageIfMissing(t *testing.T) {
	e := newEnv()
	result := batch(t, e, `{"product":{"name":"Nuevo","categorySlug":"refrigeradores","imageUrl":"https://img.test/n.jpg",
		"attributes":{"capacidad_litros":200}},"offer":{"price":1}}`)
	if result.Results[0].Status != ItemCreated || len(e.runs.imageCalls) != 0 ||
		e.catalog.created[0].ImageURL != "https://img.test/n.jpg" {
		t.Fatalf("resultado = %+v, llamadas = %v", result.Results[0], e.runs.imageCalls)
	}
}

func TestAddItems_ItemsInvalidosNoDetienenElLote(t *testing.T) {
	e := newEnv()
	e.runs.existing["BAJA"] = ExistingProduct{ID: 1, PublicID: "x", Active: false}

	result := batch(t, e, `
		{"product":{"name":""},"offer":{"price":-1,"condition":"roto"}},
		{"product":{"name":"Sin categoría"},"offer":{"price":1}},
		{"product":{"name":"Falta atributo","categorySlug":"refrigeradores"},"offer":{"price":1}},
		{"product":{"name":"Dado de baja","sku":"BAJA"},"offer":{"price":1}},
		{"product":{"name":"Bueno","categorySlug":"refrigeradores","attributes":{"capacidad_litros":200}},"offer":{"price":1}}`)

	want := [][]string{
		{"product.name/required", "offer.price/min", "offer.condition/one_of"},
		{"product.categorySlug/required"},
		{"product.attributes.capacidad_litros/required"},
		{"product/conflict"},
		{},
	}
	for i, r := range result.Results {
		if !reflect.DeepEqual(codes(r), want[i]) {
			t.Fatalf("ítem %d: errores = %v, quiero %v", i, codes(r), want[i])
		}
	}
	if result.Received != 5 || result.Failed != 4 || result.Processed != 1 || result.Results[4].Status != ItemCreated {
		t.Fatalf("resumen = %+v", result)
	}
	if len(e.offers.written) != 1 || !reflect.DeepEqual(e.runs.counters, []int{5, 1, 4}) || len(e.runs.saved) != 5 {
		t.Fatalf("ofertas = %d contadores = %v guardados = %d", len(e.offers.written), e.runs.counters, len(e.runs.saved))
	}
}

func TestAddItems_ErroresDeCategoriaYBaseDeDatos(t *testing.T) {
	item := `{"product":{"sku":"N-1","name":"Nuevo","categorySlug":"refrigeradores","attributes":{"capacidad_litros":200}},"offer":{"price":1}}`
	for _, tc := range []struct {
		name  string
		setup func(*env)
		want  []string
	}{
		{"categoría inexistente", func(e *env) { e.catalog.resolveErr = products.ErrCategoryNotFound }, []string{"product.categorySlug/not_found"}},
		{"categoría de servicios", func(e *env) {
			e.catalog.category = products.WriteCategory{ID: int32p(4), ItemType: categories.ItemTypeService}
		}, []string{"product.categorySlug/not_enabled"}},
		{"categoría sin hoja de nivel 3", func(e *env) {
			e.catalog.category = products.WriteCategory{ID: int32p(7), Slug: strp("celulares"),
				ItemType: categories.ItemTypeProduct}
		}, []string{"product.categorySlug/not_enabled"}},
		{"sku duplicado en carrera", func(e *env) { e.catalog.createErr = &pgconn.PgError{Code: "23505"} }, []string{"product.sku/conflict"}},
		{"error al buscar", func(e *env) { e.runs.findErr = errBoom }, []string{"/internal_error"}},
		{"error al registrar la oferta", func(e *env) { e.offers.upsertErr = errBoom }, []string{"/internal_error"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv()
			tc.setup(e)
			result := batch(t, e, item)
			r := result.Results[0]
			if r.Status != ItemFailed || !reflect.DeepEqual(codes(r), tc.want) {
				t.Fatalf("resultado = %+v", r)
			}
			body, _ := json.Marshal(result)
			if strings.Contains(string(body), "relation") {
				t.Fatal("el resultado expone el error interno")
			}
			if tc.want[0] == "/internal_error" && !strings.Contains(e.logs.String(), "relation") {
				t.Fatal("el error interno debe quedar en el log")
			}
		})
	}
}

func TestAddItems_CodigosDeCategoria(t *testing.T) {
	item := func(slug string) string {
		return `{"product":{"name":"Producto","categorySlug":` + mustJSON(t, slug) + `},"offer":{"price":1}}`
	}
	for _, tc := range []struct {
		name  string
		slug  string
		setup func(*env)
		want  string
	}{
		{name: "vacía", slug: " ", want: "product.categorySlug/required"},
		{name: "desconocida", slug: "categoria-inexistente", setup: func(e *env) {
			e.catalog.resolveErrors = map[string]error{"categoria-inexistente": products.ErrCategoryNotFound}
		}, want: "product.categorySlug/not_found"},
		{name: "sin hoja de nivel 3", slug: "celulares", setup: func(e *env) {
			e.catalog.category = products.WriteCategory{ID: int32p(7), Slug: strp("celulares"),
				ItemType: categories.ItemTypeProduct}
		}, want: "product.categorySlug/not_enabled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv()
			if tc.setup != nil {
				tc.setup(e)
			}
			result := batch(t, e, item(tc.slug))
			if got := codes(result.Results[0]); !reflect.DeepEqual(got, []string{tc.want}) {
				t.Fatalf("errores = %v, quiero %s", got, tc.want)
			}
			if result.Failed != 1 || result.Processed != 0 {
				t.Fatalf("el ítem debe fallar sin detener el lote: %+v", result)
			}
		})
	}
}

func TestAddItems_NormalizaSlugAntesDeResolverCategoria(t *testing.T) {
	tests := []struct {
		name       string
		slug       string
		wantSlug   string
		wantError  string
		resolveErr error
	}{
		{name: "nombre de categoría", slug: "Refrigeradores", wantSlug: "refrigeradores"},
		{name: "espacios y mayúsculas", slug: "  REFRIGERADORES ", wantSlug: "refrigeradores"},
		{name: "notebooks", slug: "Notebooks", wantSlug: "notebooks"},
		{
			name:     "refrigeración se enruta a refrigeradores",
			slug:     "refrigeración",
			wantSlug: "refrigeradores",
		},
		{name: "telefonía se enruta a celulares", slug: "telefonía", wantSlug: "celulares"},
		{name: "computación sin palabra clave va a computadores de escritorio", slug: "computación", wantSlug: "computadores-de-escritorio"},
		{name: "computadores va a computadores de escritorio", slug: "computadores", wantSlug: "computadores-de-escritorio"},
		{
			name:       "desconocido",
			slug:       "desconocida",
			wantSlug:   "desconocida",
			wantError:  "product.categorySlug/not_found",
			resolveErr: products.ErrCategoryNotFound,
		},
		{
			name:       "electrodomésticos es demasiado amplio para enrutar",
			slug:       "electrodomésticos",
			wantSlug:   "electrodomesticos",
			wantError:  "product.categorySlug/not_found",
			resolveErr: products.ErrCategoryNotFound,
		},
		{name: "vacío", slug: "", wantError: "product.categorySlug/required"},
		{name: "solo espacios", slug: "   ", wantError: "product.categorySlug/required"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv()
			e.catalog.resolveErrors = map[string]error{}
			if tc.resolveErr != nil {
				e.catalog.resolveErrors[tc.wantSlug] = tc.resolveErr
			}

			item := `{"product":{"name":"Producto de prueba","categorySlug":` +
				mustJSON(t, tc.slug) +
				`,"attributes":{"capacidad_litros":200}},"offer":{"price":1}}`
			result := batch(t, e, item)
			gotErrors := codes(result.Results[0])

			if tc.wantError != "" {
				if len(gotErrors) != 1 || gotErrors[0] != tc.wantError {
					t.Fatalf("errores = %v, quiero %q", gotErrors, tc.wantError)
				}
			} else if result.Results[0].Status != ItemCreated {
				t.Fatalf("resultado = %+v, quiero producto creado", result.Results[0])
			}

			if tc.wantSlug == "" {
				if len(e.catalog.resolved) != 0 {
					t.Fatalf("se llamó a ResolveCategory con slug vacío: %+v", e.catalog.resolved)
				}
				return
			}
			if len(e.catalog.resolved) != 1 || e.catalog.resolved[0].Slug != tc.wantSlug {
				t.Fatalf("slugs enviados a ResolveCategory = %+v, quiero %q", e.catalog.resolved, tc.wantSlug)
			}
		})
	}
}

// TestAddItems_ClasificaComputacionPorNombre verifica que un ítem de la
// categoría computacion termine en notebooks, en computadores de escritorio o
// en accesorios-de-computacion según las palabras de su nombre, que es lo que
// resuelve ResolveCategory.
func TestAddItems_ClasificaComputacionPorNombre(t *testing.T) {
	tests := []struct {
		name     string
		product  string
		wantSlug string
	}{
		{name: "MacBook Air", product: "MacBook Air M2 13\"", wantSlug: "notebooks"},
		{name: "notebook", product: "Notebook Lenovo IdeaPad 3", wantSlug: "notebooks"},
		{name: "laptop gamer", product: "Laptop gamer ASUS TUF", wantSlug: "notebooks"},
		{name: "PC gamer", product: "PC Gamer Ryzen 5 RTX 4060", wantSlug: "computadores-de-escritorio"},
		{name: "iMac", product: "iMac 24\" M3", wantSlug: "computadores-de-escritorio"},
		{name: "mochila para notebook", product: "Mochila para notebook 15.6\"", wantSlug: "accesorios-de-computacion"},
		{name: "monitor portátil", product: "Monitor portátil 15.6\"", wantSlug: "accesorios-de-computacion"},
		{name: "sin palabras clave", product: "Armado de equipo Ryzen 7", wantSlug: "computadores-de-escritorio"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv()
			item := `{"product":{"name":` + mustJSON(t, tc.product) +
				`,"categorySlug":"computacion","attributes":{"capacidad_litros":200}},"offer":{"price":1}}`
			result := batch(t, e, item)
			if result.Results[0].Status != ItemCreated {
				t.Fatalf("resultado = %+v, quiero producto creado", result.Results[0])
			}
			if len(e.catalog.resolved) != 1 || e.catalog.resolved[0].Slug != tc.wantSlug {
				t.Fatalf("slugs enviados a ResolveCategory = %+v, quiero %q", e.catalog.resolved, tc.wantSlug)
			}
		})
	}
}

func TestAddItems_ResuelveAliasSlugYOrigenDesconocido(t *testing.T) {
	tests := []struct {
		name         string
		original     string
		wantSlug     string
		unrecognized bool
	}{
		{name: "alias explícito", original: "Línea blanca refrigeración", wantSlug: "refrigeradores"},
		{name: "slug directo normalizado", original: "  REFRIGERADORES ", wantSlug: "refrigeradores"},
		{name: "origen desconocido", original: "Audio desconocido", wantSlug: "audio-desconocido", unrecognized: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv()
			e.catalog.resolveErrors = map[string]error{}
			if tc.unrecognized {
				e.catalog.resolveErrors[tc.wantSlug] = products.ErrCategoryNotFound
			}
			item := `{"product":{"name":"Producto","categorySlug":` +
				mustJSON(t, tc.original) +
				`,"attributes":{"capacidad_litros":200}},"offer":{"price":1}}`
			result := batch(t, e, item)

			if len(e.catalog.resolved) != 1 || e.catalog.resolved[0].Slug != tc.wantSlug {
				t.Fatalf("categorías resueltas = %+v, quiero %q", e.catalog.resolved, tc.wantSlug)
			}
			if tc.unrecognized {
				fieldErrors := codes(result.Results[0])
				if result.Results[0].Status != ItemFailed ||
					!reflect.DeepEqual(fieldErrors, []string{"product.categorySlug/not_found"}) ||
					len(e.catalog.created) != 0 || len(e.offers.written) != 0 {
					t.Fatalf("el origen desconocido debe rechazarse sin crear producto ni oferta: %+v", result.Results[0])
				}
				message := result.Results[0].Errors[0].Message
				if !strings.Contains(message, "Ripley scraper") || !strings.Contains(message, tc.original) ||
					!strings.Contains(e.logs.String(), "Ripley scraper") || !strings.Contains(e.logs.String(), tc.original) {
					t.Fatalf("la tienda y el valor original deben quedar en respuesta y log: %s / %s", message, e.logs.String())
				}
				return
			}
			if result.Results[0].Status != ItemCreated || len(e.catalog.created) != 1 {
				t.Fatalf("categoría reconocida no creada: %+v", result.Results[0])
			}
		})
	}
}

func TestAddItems_RechazaSeparadoresDeSlugInvalidos(t *testing.T) {
	for _, slug := range []string{
		"-refrigeradores",
		"refrigeradores-",
		"refrigeradores--",
		"internet__fibra_optica",
	} {
		t.Run(slug, func(t *testing.T) {
			e := newEnv()
			item := `{"product":{"name":"Producto de prueba","categorySlug":` +
				mustJSON(t, slug) +
				`,"attributes":{"capacidad_litros":200}},"offer":{"price":1}}`
			result := batch(t, e, item)
			if got := codes(result.Results[0]); len(got) != 1 || got[0] != "product.categorySlug/format" {
				t.Fatalf("errores = %v, quiero product.categorySlug/format", got)
			}
			if len(e.catalog.resolved) != 0 {
				t.Fatalf("no debe buscar un slug inválido: %+v", e.catalog.resolved)
			}
		})
	}
}

// TAL-205: solo una bajada de precio consulta la oferta y los usuarios a
// avisar. Si el precio sube o queda igual (lo habitual en cada corrida), el
// ítem se ahorra esas dos consultas y no se envía ningún aviso.
func TestAddItems_AvisoSoloSiElPrecioBaja(t *testing.T) {
	for _, tc := range []struct {
		name      string
		previous  string
		price     string
		wantCalls int
	}{
		{"baja", "100.00", "90", 1},
		{"baja en centavos", "90.01", "90", 1},
		{"igual", "90.00", "90", 0},
		{"sube", "80.00", "90", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv()
			e.runs.existing["LG-1"] = ExistingProduct{ID: 8, PublicID: "existente", Active: true}
			e.offers.previous = strp(tc.previous)
			e.offers.watchers = []pricing.Watcher{{UserID: 4, ProductName: "Refri"}}

			result := batch(t, e, `{"product":{"sku":"LG-1","name":"x"},"offer":{"price":`+tc.price+`}}`)

			if result.Results[0].Status != ItemUpdated {
				t.Fatalf("resultado = %+v", result.Results[0])
			}
			if e.offers.getCalls != tc.wantCalls || e.offers.watcherCalls != tc.wantCalls || len(e.notifier.sent) != tc.wantCalls {
				t.Fatalf("GetOffer = %d, PriceDropWatchers = %d, avisos = %d; quiero %d de cada uno",
					e.offers.getCalls, e.offers.watcherCalls, len(e.notifier.sent), tc.wantCalls)
			}
		})
	}
}

// TAL-205: scraped_data se guarda con un solo INSERT por lote y en el orden
// recibido, incluidos los ítems fallidos. Si ese INSERT falla, se guarda ítem
// por ítem para no perder los demás, y el lote responde igual.
func TestAddItems_ScrapedDataEnUnSoloInsert(t *testing.T) {
	items := `{"externalId":"a","product":{"name":""},"offer":{"price":1}},
		{"externalId":"b","product":{"name":"Bueno","categorySlug":"refrigeradores","attributes":{"capacidad_litros":200}},"offer":{"price":1}},
		{"externalId":"c","product":{"name":"Sin categoría"},"offer":{"price":1}}`
	for _, tc := range []struct {
		name        string
		batchErr    error
		wantSingles int
	}{
		{"lote", nil, 0},
		{"respaldo ítem por ítem", errBoom, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv()
			e.runs.saveBatchErr = tc.batchErr

			result := batch(t, e, items)

			if result.Received != 3 || result.Processed != 1 || result.Failed != 2 {
				t.Fatalf("resumen = %+v", result)
			}
			if e.runs.batches != 1 || e.runs.singles != tc.wantSingles || !reflect.DeepEqual(e.runs.saved, []string{"a", "b", "c"}) {
				t.Fatalf("lotes = %d, sueltos = %d, guardados = %v", e.runs.batches, e.runs.singles, e.runs.saved)
			}
			if tc.batchErr != nil && !strings.Contains(e.logs.String(), "ingestion_save_scraped_batch_failed") {
				t.Fatal("el fallo del lote debe quedar en el log")
			}
		})
	}
}

// TestPriceDropped compara precios decimales sin redondeo y, si alguno no se
// puede leer, deja que la consulta de avisos decida (devuelve true).
func TestPriceDropped(t *testing.T) {
	for _, tc := range []struct {
		previous, current string
		want              bool
	}{
		{"100.00", "99.99", true},
		{"100.00", "100", false},
		{"100", "100.01", false},
		{"1234567890.99", "1234567890.98", true},
		{"no-es-numero", "10", true},
		{"10", "", true},
	} {
		if got := priceDropped(tc.previous, tc.current); got != tc.want {
			t.Errorf("priceDropped(%q, %q) = %v, quiero %v", tc.previous, tc.current, got, tc.want)
		}
	}
}
