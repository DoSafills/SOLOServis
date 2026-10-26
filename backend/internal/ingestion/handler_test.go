package ingestion

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var errBoom = errors.New(`relation "scrape_run" does not exist`)

const newProductID = "99999999-9999-9999-9999-999999999999"

func floatp(f float64) *float64 { return &f }
func int32p(i int32) *int32     { return &i }
func strp(s string) *string     { return &s }

// refrigeradores: categoría nueva enlazada a la anterior, con un requerido.
var refrigeradores = products.WriteCategory{
	ID: int32p(1), Slug: strp("refrigeradores"), ItemType: categories.ItemTypeProduct, LegacyID: int32p(5),
	Attributes: []categories.Attribute{
		{ID: 10, Key: "capacidad_litros", DataType: "integer", Required: true, Min: floatp(50), Max: floatp(1500)},
	},
}

// ------------------------------------------------------------------ fakes

type fakeRuns struct {
	run                Run
	storeID            int32
	targetErr, openErr error
	finishErr, getErr  error
	existing           map[string]ExistingProduct // por sku o "marca|modelo"
	findErr            error

	opened   []any
	counters []int
	saved    []string
	brands   []string
	finished []any
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
func (f *fakeRuns) SaveScraped(_ context.Context, _ int32, externalID, _ *string, _ json.RawMessage) error {
	id := ""
	if externalID != nil {
		id = *externalID
	}
	f.saved = append(f.saved, id)
	return nil
}
func (f *fakeRuns) FindProduct(_ context.Context, sku, brand, model string) (ExistingProduct, bool, error) {
	if f.findErr != nil {
		return ExistingProduct{}, false, f.findErr
	}
	key := sku
	if key == "" {
		key = brand + "|" + model
	}
	p, ok := f.existing[key]
	return p, ok, nil
}
func (f *fakeRuns) EnsureBrand(_ context.Context, name string) (int32, error) {
	f.brands = append(f.brands, name)
	return 77, nil
}
func (f *fakeRuns) ProductID(context.Context, pgtype.UUID) (int32, error) { return 500, nil }

type fakeCatalog struct {
	category   products.WriteCategory
	resolveErr error
	createErr  error
	created    []products.ProductWrite
}

func (c *fakeCatalog) ResolveCategory(context.Context, products.CategoryRef) (products.WriteCategory, error) {
	return c.category, c.resolveErr
}
func (c *fakeCatalog) Create(_ context.Context, w products.ProductWrite) (pgtype.UUID, error) {
	c.created = append(c.created, w)
	var id pgtype.UUID
	_ = id.Scan(newProductID)
	return id, c.createErr
}

type fakeOffers struct {
	previous  *string
	upsertErr error
	watchers  []pricing.Watcher
	written   []pricing.OfferWrite
}

func (o *fakeOffers) UpsertOffer(_ context.Context, w pricing.OfferWrite) (int32, bool, *string, error) {
	o.written = append(o.written, w)
	return int32(40 + len(o.written)), o.previous == nil, o.previous, o.upsertErr
}
func (o *fakeOffers) GetOffer(_ context.Context, id int32) (pricing.Offer, error) {
	return pricing.Offer{ID: id, Price: "90.00"}, nil
}
func (o *fakeOffers) PriceDropWatchers(context.Context, int32, string, string) ([]pricing.Watcher, error) {
	return o.watchers, nil
}

type fakeNotifier struct{ sent []pricing.Notification }

func (n *fakeNotifier) Notify(_ context.Context, notification pricing.Notification) error {
	n.sent = append(n.sent, notification)
	return nil
}

type env struct {
	runs     *fakeRuns
	catalog  *fakeCatalog
	offers   *fakeOffers
	notifier *fakeNotifier
	logs     *bytes.Buffer
}

func newEnv() *env {
	return &env{
		runs:     &fakeRuns{storeID: 2, run: Run{ID: 3, Source: "Ripley scraper", Status: StatusRunning}, existing: map[string]ExistingProduct{}},
		catalog:  &fakeCatalog{category: refrigeradores},
		offers:   &fakeOffers{},
		notifier: &fakeNotifier{},
		logs:     &bytes.Buffer{},
	}
}

func (e *env) serve(t *testing.T, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	logger := slog.New(slog.NewJSONHandler(e.logs, nil))
	processor := NewProcessor(e.runs, e.catalog, e.offers, e.notifier, logger)
	h := NewHandler(e.runs, processor, logger)
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
	if created.LegacyCategoryID != 5 || *created.CategoryRef != 1 || *created.BrandID != 77 || created.SKU != "LG-1" ||
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

func TestAddItems_ProductoExistenteSoloRegistraLaOferta(t *testing.T) {
	e := newEnv()
	e.runs.existing["LG-1"] = ExistingProduct{ID: 8, PublicID: "existente-sku", Active: true}
	e.runs.existing["Samsung|RT38"] = ExistingProduct{ID: 9, PublicID: "existente-modelo", Active: true}
	e.offers.previous = strp("100.00")
	e.offers.watchers = []pricing.Watcher{{UserID: 4, ProductName: "Refri"}}

	result := batch(t, e, `{"product":{"sku":"LG-1","name":"cualquiera"},"offer":{"price":90}},
		{"product":{"name":"x","brand":"Samsung","model":"RT38"},"offer":{"price":90}}`)

	if result.Processed != 2 || result.Results[0].Status != ItemUpdated || result.Results[0].ProductID != "existente-sku" ||
		result.Results[1].ProductID != "existente-modelo" {
		t.Fatalf("resultado = %+v", result)
	}
	if len(e.catalog.created) != 0 {
		t.Fatal("un producto existente no se recrea ni se modifica")
	}
	if e.offers.written[0].ProductID != 8 || e.offers.written[1].ProductID != 9 {
		t.Fatalf("ofertas = %+v", e.offers.written)
	}
	if len(e.notifier.sent) != 2 || e.notifier.sent[0].Data["oldPrice"] != "100.00" {
		t.Fatalf("la bajada de precio debe avisar como en Pricing: %+v", e.notifier.sent)
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
		}, []string{"product.categorySlug/not_found"}},
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
