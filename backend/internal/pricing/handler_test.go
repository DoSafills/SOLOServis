package pricing

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

	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const testProductID = "11111111-2222-3333-4444-555555555555"

// errBoom simula un error de base de datos con detalle interno.
var errBoom = errors.New(`relation "product_offer" does not exist`)

func strp(s string) *string { return &s }

type fakeStore struct {
	offer    Offer
	offers   []Offer
	points   []PricePoint
	ppoints  []ProductPricePoint
	total    int64
	watchers []Watcher

	inserted    bool
	previous    *string
	storeExists bool

	resolveErr, getErr, writeErr, listErr error

	gotWrite   *OfferWrite
	gotOfferID int32
	gotFilter  *HistoryFilter
	gotDrop    []string
	writes     int
}

func (f *fakeStore) ResolveProduct(_ context.Context, key string) (int32, error) {
	return 7, f.resolveErr
}
func (f *fakeStore) StoreExists(context.Context, int32) (bool, error) { return f.storeExists, nil }
func (f *fakeStore) GetOffer(_ context.Context, id int32) (Offer, error) {
	f.gotOfferID = id
	return f.offer, f.getErr
}
func (f *fakeStore) ListProductOffers(context.Context, int32) ([]Offer, error) {
	return f.offers, f.listErr
}
func (f *fakeStore) BestOffer(context.Context, int32) (Offer, error) { return f.offer, f.getErr }
func (f *fakeStore) UpsertOffer(_ context.Context, w OfferWrite) (int32, bool, *string, error) {
	f.writes++
	f.gotWrite = &w
	return 42, f.inserted, f.previous, f.writeErr
}
func (f *fakeStore) UpdateOffer(_ context.Context, id int32, w OfferWrite) (string, error) {
	f.writes++
	f.gotWrite = &w
	f.gotOfferID = id
	previous := ""
	if f.previous != nil {
		previous = *f.previous
	}
	return previous, f.writeErr
}
func (f *fakeStore) PriceDropWatchers(_ context.Context, _ int32, oldPrice, newPrice string) ([]Watcher, error) {
	f.gotDrop = []string{oldPrice, newPrice}
	return f.watchers, nil
}
func (f *fakeStore) OfferHistory(_ context.Context, _ int32, filter HistoryFilter) ([]PricePoint, int64, error) {
	f.gotFilter = &filter
	return f.points, f.total, f.listErr
}
func (f *fakeStore) ProductHistory(_ context.Context, _ int32, filter HistoryFilter) ([]ProductPricePoint, int64, error) {
	f.gotFilter = &filter
	return f.ppoints, f.total, f.listErr
}

type fakeNotifier struct {
	sent []Notification
	err  error
}

func (n *fakeNotifier) Notify(_ context.Context, notification Notification) error {
	n.sent = append(n.sent, notification)
	return n.err
}

func serve(t *testing.T, store OfferStore, notifier Notifier, method, target, body string) (*httptest.ResponseRecorder, string) {
	t.Helper()
	var logs bytes.Buffer
	if notifier == nil {
		notifier = &fakeNotifier{}
	}
	h := NewHandler(store, notifier, slog.New(slog.NewJSONHandler(&logs, nil)))

	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Get("/offers/product/{productId}", h.ListProductOffers)
	r.Get("/offers/product/{productId}/best", h.BestOffer)
	r.Get("/offers/product/{productId}/price-history", h.ProductHistory)
	r.Get("/offers/{id}", h.GetOffer)
	r.Get("/offers/{id}/price-history", h.OfferHistory)
	r.Post("/offers", h.CreateOffer)
	r.Put("/offers/{id}", h.UpdateOffer)

	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec, logs.String()
}

func requireStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, quiero %d (%s)", rec.Code, want, rec.Body.String())
	}
}

func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder) httpx.Problem {
	t.Helper()
	if got := rec.Header().Get("Content-Type"); got != httpx.ProblemContentType {
		t.Fatalf("Content-Type = %q, quiero problem+json (%s)", got, rec.Body.String())
	}
	var p httpx.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("body no es un Problem: %v", err)
	}
	if p.RequestID == "" || p.RequestID != rec.Header().Get(httpx.RequestIDHeader) {
		t.Fatalf("request_id = %q, header = %q", p.RequestID, rec.Header().Get(httpx.RequestIDHeader))
	}
	return p
}

func fieldCodes(p httpx.Problem) []string {
	out := make([]string, 0, len(p.Errors))
	for _, e := range p.Errors {
		out = append(out, e.Field+"/"+e.Code)
	}
	return out
}

// ------------------------------------------------------------- POST /offers

func TestCreateOffer_NuevaConDefaults(t *testing.T) {
	store := &fakeStore{inserted: true, storeExists: true, offer: Offer{ID: 42, Price: "123.45"}}
	notifier := &fakeNotifier{}

	rec, _ := serve(t, store, notifier, http.MethodPost, "/offers",
		`{"productId":"`+testProductID+`","storeId":3,"price":123.45}`)

	requireStatus(t, rec, http.StatusCreated)
	if rec.Header().Get("Location") != "/offers/42" || !strings.Contains(rec.Body.String(), `"price":"123.45"`) {
		t.Fatalf("Location = %q body = %s", rec.Header().Get("Location"), rec.Body.String())
	}
	want := OfferWrite{ProductID: 7, StoreID: 3, Price: "123.45", Currency: "CLP", ShippingCost: "0",
		Available: true, Condition: "new"}
	if !reflect.DeepEqual(*store.gotWrite, want) {
		t.Fatalf("write = %+v, quiero %+v", *store.gotWrite, want)
	}
	if len(notifier.sent) != 0 || store.gotDrop != nil {
		t.Fatal("una oferta nueva no tiene precio anterior: no se buscan avisos")
	}
}

func TestCreateOffer_ActualizaYAvisaBajadaDePrecio(t *testing.T) {
	store := &fakeStore{storeExists: true, previous: strp("200.00"), offer: Offer{ID: 42, Price: "150.00"},
		watchers: []Watcher{{UserID: 9, ProductID: testProductID, ProductName: "TV", TargetPrice: "180.00"}}}
	notifier := &fakeNotifier{}

	rec, _ := serve(t, store, notifier, http.MethodPost, "/offers", `{
		"productId":"`+testProductID+`","storeId":3,"price":"150","listPrice":199990,
		"shippingCost":2990.5,"currency":"CLP","available":false,"stock":4,"condition":"used",
		"productUrl":"https://example.test/tv"}`)

	requireStatus(t, rec, http.StatusOK)
	if rec.Header().Get("Location") != "" {
		t.Fatal("una actualización no lleva Location")
	}
	w := store.gotWrite
	if w.Price != "150" || *w.ListPrice != "199990" || w.ShippingCost != "2990.5" || w.Available ||
		*w.Stock != 4 || w.Condition != "used" || *w.ProductURL != "https://example.test/tv" {
		t.Fatalf("write = %+v", *w)
	}
	if !reflect.DeepEqual(store.gotDrop, []string{"200.00", "150.00"}) || len(notifier.sent) != 1 {
		t.Fatalf("drop = %v, avisos = %+v", store.gotDrop, notifier.sent)
	}
	n := notifier.sent[0]
	if n.UserID != 9 || n.Type != "price_drop" || n.Data["oldPrice"] != "200.00" || n.Data["price"] != "150.00" ||
		n.Data["targetPrice"] != "180.00" || n.Data["offerId"] != int32(42) {
		t.Fatalf("aviso = %+v", n)
	}
}

func TestCreateOffer_FalloDelAvisoNoRompeLaRespuesta(t *testing.T) {
	store := &fakeStore{storeExists: true, previous: strp("200.00"), offer: Offer{ID: 42, Price: "150.00"},
		watchers: []Watcher{{UserID: 9}}}

	rec, logs := serve(t, store, &fakeNotifier{err: errors.New("notification api caída")}, http.MethodPost, "/offers",
		`{"productId":"`+testProductID+`","storeId":3,"price":150}`)

	requireStatus(t, rec, http.StatusOK)
	if !strings.Contains(logs, "price_drop_notification_failed") {
		t.Fatalf("el fallo debe quedar en el log: %s", logs)
	}
}

func TestCreateOffer_ErroresDeDecodificacion(t *testing.T) {
	tests := []struct {
		name, body string
		wantStatus int
		wantCode   string
	}{
		{"json malformado", `{"price":`, 400, httpx.CodeMalformedJSON},
		{"campo desconocido", `{"precio":1}`, 400, httpx.CodeUnknownField},
		{"precio no numérico", `{"price":"caro"}`, 400, httpx.CodeInvalidType},
		{"precio booleano", `{"price":true}`, 400, httpx.CodeInvalidType},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeStore{}
			rec, _ := serve(t, store, nil, http.MethodPost, "/offers", tc.body)
			requireStatus(t, rec, tc.wantStatus)
			if p := decodeProblem(t, rec); p.Code != tc.wantCode || store.writes != 0 {
				t.Fatalf("problem = %+v, writes = %d", p, store.writes)
			}
		})
	}
}

// Reemplaza a los tests de caracterización que aceptaban datos inválidos
// (precio ausente = 0, condición o stock inválidos llegaban a la BD).
func TestCreateOffer_Reglas(t *testing.T) {
	base := `"productId":"` + testProductID + `","storeId":3`
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"precio ausente ya no es 0", `{` + base + `}`, []string{"price/required"}},
		{"precio negativo", `{` + base + `,"price":-1}`, []string{"price/min"}},
		{"tres decimales", `{` + base + `,"price":1.005}`, []string{"price/format"}},
		{"notación exponencial", `{` + base + `,"price":1e3}`, []string{"price/format"}},
		{"excede NUMERIC(12,2)", `{` + base + `,"price":12345678901}`, []string{"price/format"}},
		{"listPrice negativo", `{` + base + `,"price":1,"listPrice":-5}`, []string{"listPrice/min"}},
		{"shippingCost inválido", `{` + base + `,"price":1,"shippingCost":1.234}`, []string{"shippingCost/format"}},
		{"moneda inválida", `{` + base + `,"price":1,"currency":"US1"}`, []string{"currency/format"}},
		{"condición inválida", `{` + base + `,"price":1,"condition":"damaged"}`, []string{"condition/one_of"}},
		{"stock negativo", `{` + base + `,"price":1,"stock":-1}`, []string{"stock/min"}},
		{"url no http", `{` + base + `,"price":1,"productUrl":"ftp://x.test/a"}`, []string{"productUrl/format"}},
		{"sin productId ni storeId", `{"price":1}`, []string{"productId/required", "storeId/min"}},
		{"productId inválido", `{"productId":"abc","storeId":3,"price":1}`, []string{"productId/format"}},
		{"storeId cero", `{"productId":"` + testProductID + `","storeId":0,"price":1}`, []string{"storeId/min"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeStore{storeExists: true}
			rec, _ := serve(t, store, nil, http.MethodPost, "/offers", tc.body)

			requireStatus(t, rec, http.StatusUnprocessableEntity)
			if got := fieldCodes(decodeProblem(t, rec)); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("errores = %v, quiero %v", got, tc.want)
			}
			if store.writes != 0 {
				t.Fatal("no debe escribir con datos inválidos")
			}
		})
	}
}

func TestCreateOffer_ReferenciasYErroresDeBD(t *testing.T) {
	body := `{"productId":"` + testProductID + `","storeId":3,"price":10}`
	tests := []struct {
		name       string
		store      *fakeStore
		wantStatus int
		wantCode   string
		wantFields []string
	}{
		{"producto inexistente", &fakeStore{resolveErr: ErrProductNotFound, storeExists: true}, 422, httpx.CodeValidationFailed, []string{"productId/not_found"}},
		{"tienda inexistente", &fakeStore{storeExists: false}, 422, httpx.CodeValidationFailed, []string{"storeId/not_found"}},
		{"error al resolver producto", &fakeStore{resolveErr: errBoom, storeExists: true}, 500, httpx.CodeInternalError, []string{}},
		{"URL duplicada", &fakeStore{storeExists: true, writeErr: &pgconn.PgError{Code: "23505"}}, 409, httpx.CodeConflict, []string{}},
		{"error al guardar", &fakeStore{storeExists: true, writeErr: errBoom}, 500, httpx.CodeInternalError, []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec, _ := serve(t, tc.store, nil, http.MethodPost, "/offers", body)
			requireStatus(t, rec, tc.wantStatus)
			p := decodeProblem(t, rec)
			if p.Code != tc.wantCode || !reflect.DeepEqual(fieldCodes(p), tc.wantFields) || strings.Contains(rec.Body.String(), "relation") {
				t.Fatalf("problem = %+v", p)
			}
		})
	}
}

// ----------------------------------------------------------- PUT /offers/{id}

func TestUpdateOffer(t *testing.T) {
	t.Run("solo precio: no apaga la oferta", func(t *testing.T) {
		store := &fakeStore{offer: Offer{ID: 5, Price: "234.56"}}
		rec, _ := serve(t, store, nil, http.MethodPut, "/offers/5", `{"price":234.56}`)
		requireStatus(t, rec, http.StatusOK)
		if store.gotOfferID != 5 || !store.gotWrite.Available || store.gotWrite.Condition != "new" || store.gotWrite.Price != "234.56" {
			t.Fatalf("write = %+v", *store.gotWrite)
		}
	})
	t.Run("producto y tienda no se cambian por PUT", func(t *testing.T) {
		rec, _ := serve(t, &fakeStore{}, nil, http.MethodPut, "/offers/5", `{"price":1,"storeId":2}`)
		requireStatus(t, rec, http.StatusBadRequest)
		if p := decodeProblem(t, rec); p.Code != httpx.CodeUnknownField {
			t.Fatalf("problem = %+v", p)
		}
	})
	t.Run("id inválido", func(t *testing.T) {
		rec, _ := serve(t, &fakeStore{}, nil, http.MethodPut, "/offers/abc", `{"price":1}`)
		requireStatus(t, rec, http.StatusBadRequest)
	})
	t.Run("regla incumplida", func(t *testing.T) {
		rec, _ := serve(t, &fakeStore{}, nil, http.MethodPut, "/offers/5", `{"stock":2}`)
		requireStatus(t, rec, http.StatusUnprocessableEntity)
	})
	t.Run("inexistente", func(t *testing.T) {
		rec, _ := serve(t, &fakeStore{writeErr: ErrOfferNotFound}, nil, http.MethodPut, "/offers/5", `{"price":1}`)
		requireStatus(t, rec, http.StatusNotFound)
	})
	t.Run("error interno", func(t *testing.T) {
		rec, _ := serve(t, &fakeStore{writeErr: errBoom}, nil, http.MethodPut, "/offers/5", `{"price":1}`)
		requireStatus(t, rec, http.StatusInternalServerError)
	})
}

// ------------------------------------------------------------------ lectura

func TestGetOffer(t *testing.T) {
	rec, _ := serve(t, &fakeStore{offer: Offer{ID: 5, ProductID: testProductID}}, nil, http.MethodGet, "/offers/5", "")
	requireStatus(t, rec, http.StatusOK)
	// Cart API lee productId de esta respuesta.
	if !strings.Contains(rec.Body.String(), `"productId":"`+testProductID+`"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
	for _, tc := range []struct {
		path   string
		store  *fakeStore
		status int
	}{
		{"/offers/0", &fakeStore{}, 400},
		{"/offers/x", &fakeStore{}, 400},
		{"/offers/5", &fakeStore{getErr: ErrOfferNotFound}, 404},
		{"/offers/5", &fakeStore{getErr: errBoom}, 500},
	} {
		rec, _ := serve(t, tc.store, nil, http.MethodGet, tc.path, "")
		requireStatus(t, rec, tc.status)
	}
}

func TestListProductOffersYBest(t *testing.T) {
	for _, key := range []string{testProductID, "12"} {
		rec, _ := serve(t, &fakeStore{}, nil, http.MethodGet, "/offers/product/"+key, "")
		requireStatus(t, rec, http.StatusOK)
		if strings.TrimSpace(rec.Body.String()) != "[]" {
			t.Fatalf("sin ofertas debe responder [], no %s", rec.Body.String())
		}
	}
	for _, key := range []string{"abc", "0", "-3"} {
		rec, _ := serve(t, &fakeStore{}, nil, http.MethodGet, "/offers/product/"+key, "")
		requireStatus(t, rec, http.StatusBadRequest)
		if got := fieldCodes(decodeProblem(t, rec)); !reflect.DeepEqual(got, []string{"productId/format"}) {
			t.Fatalf("errores = %v", got)
		}
	}
	rec, _ := serve(t, &fakeStore{resolveErr: ErrProductNotFound}, nil, http.MethodGet, "/offers/product/"+testProductID, "")
	requireStatus(t, rec, http.StatusNotFound)
	rec, _ = serve(t, &fakeStore{listErr: errBoom}, nil, http.MethodGet, "/offers/product/"+testProductID, "")
	requireStatus(t, rec, http.StatusInternalServerError)

	rec, _ = serve(t, &fakeStore{offer: Offer{ID: 3, TotalPrice: "120.00"}}, nil, http.MethodGet, "/offers/product/"+testProductID+"/best", "")
	requireStatus(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"totalPrice":"120.00"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
	rec, _ = serve(t, &fakeStore{getErr: ErrNoAvailable}, nil, http.MethodGet, "/offers/product/"+testProductID+"/best", "")
	requireStatus(t, rec, http.StatusNotFound)
	if p := decodeProblem(t, rec); p.Detail != "El producto no tiene ofertas disponibles." {
		t.Fatalf("problem = %+v", p)
	}
}

func TestHistorial(t *testing.T) {
	t.Run("oferta con filtros", func(t *testing.T) {
		store := &fakeStore{points: []PricePoint{{ID: 1, Price: "10.00"}}, total: 9}
		rec, _ := serve(t, store, nil, http.MethodGet, "/offers/5/price-history?from=2026-01-01&to=2026-12-31&limit=500&offset=2", "")
		requireStatus(t, rec, http.StatusOK)
		f := store.gotFilter
		if rec.Header().Get("X-Total-Count") != "9" || *f.From != "2026-01-01" || *f.To != "2026-12-31" || f.Limit != 500 || f.Offset != 2 {
			t.Fatalf("filtro = %+v total = %s", *f, rec.Header().Get("X-Total-Count"))
		}
	})
	t.Run("producto por defecto", func(t *testing.T) {
		store := &fakeStore{ppoints: []ProductPricePoint{{OfferID: 1, StoreName: "Ripley"}}, total: 1}
		rec, _ := serve(t, store, nil, http.MethodGet, "/offers/product/"+testProductID+"/price-history", "")
		requireStatus(t, rec, http.StatusOK)
		if store.gotFilter.Limit != DefaultHistoryLimit || store.gotFilter.From != nil ||
			!strings.Contains(rec.Body.String(), `"storeName":"Ripley"`) {
			t.Fatalf("filtro = %+v body = %s", *store.gotFilter, rec.Body.String())
		}
	})
	for _, tc := range []struct{ query, want string }{
		{"from=2026-13-01", "from/format"},
		{"to=ayer", "to/format"},
		{"from=2026-05-02&to=2026-05-01", "to/min"},
		{"limit=501", "limit/max"},
		{"limit=0", "limit/min"},
		{"offset=-1", "offset/min"},
	} {
		t.Run("inválido "+tc.query, func(t *testing.T) {
			rec, _ := serve(t, &fakeStore{}, nil, http.MethodGet, "/offers/5/price-history?"+tc.query, "")
			requireStatus(t, rec, http.StatusBadRequest)
			if got := fieldCodes(decodeProblem(t, rec)); !reflect.DeepEqual(got, []string{tc.want}) {
				t.Fatalf("errores = %v", got)
			}
		})
	}
	t.Run("oferta inexistente ya no responde []", func(t *testing.T) {
		rec, _ := serve(t, &fakeStore{listErr: ErrOfferNotFound}, nil, http.MethodGet, "/offers/5/price-history", "")
		requireStatus(t, rec, http.StatusNotFound)
	})
}
