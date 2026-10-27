package stores

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
	"github.com/DoSafills/SOLOServis/backend/internal/stores/dto"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const testProductID = "11111111-2222-3333-4444-555555555555"

// errBoom simula un error de base de datos con detalle interno.
var errBoom = errors.New(`relation "store" does not exist`)

type fakeReader struct {
	stores []dto.Store
	detail dto.StoreDetail
	offers []dto.Offer
	total  int64
	err    error

	gotPage    *Page
	gotStoreID int32
	gotProduct pgtype.UUID
	gotFilter  *OfferFilter
	calls      int
}

func (f *fakeReader) List(_ context.Context, page Page) ([]dto.Store, int64, error) {
	f.calls++
	f.gotPage = &page
	return f.stores, f.total, f.err
}

func (f *fakeReader) GetDetail(_ context.Context, id int32) (dto.StoreDetail, error) {
	f.calls++
	f.gotStoreID = id
	return f.detail, f.err
}

func (f *fakeReader) ListProductOffers(_ context.Context, id pgtype.UUID, filter OfferFilter) ([]dto.Offer, int64, error) {
	f.calls++
	f.gotProduct = id
	f.gotFilter = &filter
	return f.offers, f.total, f.err
}

func (f *fakeReader) ListStoreOffers(_ context.Context, id int32, filter OfferFilter) ([]dto.Offer, int64, error) {
	f.calls++
	f.gotStoreID = id
	f.gotFilter = &filter
	return f.offers, f.total, f.err
}

// serve monta el handler con las mismas rutas que el router real.
func serve(t *testing.T, reader Reader, target string) (*httptest.ResponseRecorder, string) {
	t.Helper()
	var logs bytes.Buffer
	h := NewHandler(reader, slog.New(slog.NewJSONHandler(&logs, nil)))

	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Get("/stores", h.List)
	r.Get("/stores/offers", h.ListProductOffers)
	r.Get("/stores/{id}", h.GetByID)
	r.Get("/stores/{id}/offers", h.ListStoreOffers)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
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
		t.Fatalf("Content-Type = %q, quiero problem+json", got)
	}
	var p httpx.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("body no es un Problem: %v (%s)", err, rec.Body.String())
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

// ------------------------------------------------------------------ GET /stores

func TestList_CasoFeliz(t *testing.T) {
	reader := &fakeReader{stores: []dto.Store{{ID: "3", Name: "Ripley", ReviewCount: 2, ProductCount: 5}}, total: 7}

	rec, _ := serve(t, reader, "/stores?limit=10&offset=0")

	requireStatus(t, rec, http.StatusOK)
	if rec.Header().Get("X-Total-Count") != "7" {
		t.Fatalf("X-Total-Count = %q", rec.Header().Get("X-Total-Count"))
	}
	var got []dto.Store
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || !reflect.DeepEqual(got, reader.stores) {
		t.Fatalf("el body debe seguir siendo un arreglo de tiendas: %s", rec.Body.String())
	}
	if *reader.gotPage != (Page{Limit: 10}) {
		t.Fatalf("page = %+v", *reader.gotPage)
	}
}

func TestList_PorDefectoYVacio(t *testing.T) {
	reader := &fakeReader{}
	rec, _ := serve(t, reader, "/stores")

	requireStatus(t, rec, http.StatusOK)
	if strings.TrimSpace(rec.Body.String()) != "[]" || *reader.gotPage != (Page{Limit: DefaultLimit}) {
		t.Fatalf("body = %s page = %+v", rec.Body.String(), *reader.gotPage)
	}
}

func TestList_ErrorInternoNoExponeDetalles(t *testing.T) {
	rec, logs := serve(t, &fakeReader{err: errBoom}, "/stores")

	requireStatus(t, rec, http.StatusInternalServerError)
	p := decodeProblem(t, rec)
	if p.Code != httpx.CodeInternalError || strings.Contains(rec.Body.String(), "relation") {
		t.Fatalf("problem = %+v", p)
	}
	if !strings.Contains(logs, "relation") || !strings.Contains(logs, p.RequestID) {
		t.Fatalf("el log debe tener el error y el request_id: %s", logs)
	}
}

// ------------------------------------------------------------- GET /stores/{id}

func TestGetByID(t *testing.T) {
	detail := dto.StoreDetail{Store: dto.Store{ID: "3", Name: "Ripley"}, Products: []dto.StoreProduct{}, Locations: []dto.StoreLocation{}}

	t.Run("caso feliz", func(t *testing.T) {
		reader := &fakeReader{detail: detail}
		rec, _ := serve(t, reader, "/stores/3")
		requireStatus(t, rec, http.StatusOK)
		if reader.gotStoreID != 3 || !strings.Contains(rec.Body.String(), `"products":[]`) {
			t.Fatalf("id = %d body = %s", reader.gotStoreID, rec.Body.String())
		}
	})
	for _, path := range []string{"/stores/abc", "/stores/0", "/stores/-1", "/stores/99999999999"} {
		t.Run("id inválido "+path, func(t *testing.T) {
			reader := &fakeReader{}
			rec, _ := serve(t, reader, path)
			requireStatus(t, rec, http.StatusBadRequest)
			if p := decodeProblem(t, rec); p.Code != httpx.CodeInvalidParameter ||
				!reflect.DeepEqual(fieldCodes(p), []string{"id/format"}) || reader.calls != 0 {
				t.Fatalf("problem = %+v calls = %d", p, reader.calls)
			}
		})
	}
	t.Run("inexistente", func(t *testing.T) {
		rec, _ := serve(t, &fakeReader{err: ErrNotFound}, "/stores/3")
		requireStatus(t, rec, http.StatusNotFound)
		if p := decodeProblem(t, rec); p.Code != httpx.CodeNotFound {
			t.Fatalf("problem = %+v", p)
		}
	})
	t.Run("error de BD no se disfraza de 404", func(t *testing.T) {
		rec, _ := serve(t, &fakeReader{err: errBoom}, "/stores/3")
		requireStatus(t, rec, http.StatusInternalServerError)
	})
}

// ---------------------------------------------- GET /stores/offers?product=

func TestListProductOffers_CasoFeliz(t *testing.T) {
	offers := []dto.Offer{{OfferID: 1, ProductID: testProductID, Price: "349990", Available: true,
		Store: &dto.StoreSummary{ID: "3", Name: "Ripley"}}}
	reader := &fakeReader{offers: offers, total: 4}

	rec, _ := serve(t, reader, "/stores/offers?product="+testProductID+"&available=true&limit=2")

	requireStatus(t, rec, http.StatusOK)
	if rec.Header().Get("X-Total-Count") != "4" ||
		!strings.Contains(rec.Body.String(), `"store":{"id":"3","name":"Ripley"`) {
		t.Fatalf("headers/body = %s %s", rec.Header().Get("X-Total-Count"), rec.Body.String())
	}
	if reader.gotProduct.String() != testProductID || reader.gotFilter.Available == nil ||
		!*reader.gotFilter.Available || reader.gotFilter.Limit != 2 {
		t.Fatalf("product = %s filter = %+v", reader.gotProduct.String(), *reader.gotFilter)
	}
}

func TestListProductOffers_SinFiltroDeDisponibilidad(t *testing.T) {
	reader := &fakeReader{}
	rec, _ := serve(t, reader, "/stores/offers?product="+testProductID)

	requireStatus(t, rec, http.StatusOK)
	if reader.gotFilter.Available != nil || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("filter = %+v body = %s", *reader.gotFilter, rec.Body.String())
	}
}

func TestListProductOffers_ParametrosInvalidos(t *testing.T) {
	tests := []struct {
		query string
		want  []string
	}{
		{"", []string{"product/required"}},
		{"product=", []string{"product/required"}},
		{"product=no-es-uuid", []string{"product/format"}},
		{"product=" + testProductID + "&available=si", []string{"available/one_of"}},
		{"product=" + testProductID + "&limit=101", []string{"limit/max"}},
		{"product=" + testProductID + "&offset=-1", []string{"offset/min"}},
		{"product=x&available=1&limit=0", []string{"product/format", "available/one_of", "limit/min"}},
	}
	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			reader := &fakeReader{}
			rec, _ := serve(t, reader, "/stores/offers?"+tc.query)

			requireStatus(t, rec, http.StatusBadRequest)
			p := decodeProblem(t, rec)
			if p.Code != httpx.CodeInvalidParameter || !reflect.DeepEqual(fieldCodes(p), tc.want) {
				t.Fatalf("errores = %v, quiero %v", fieldCodes(p), tc.want)
			}
			if reader.calls != 0 {
				t.Fatal("no debe consultar el repositorio con parámetros inválidos")
			}
		})
	}
}

func TestListProductOffers_ProductoInexistenteYErrorInterno(t *testing.T) {
	rec, _ := serve(t, &fakeReader{err: ErrProductNotFound}, "/stores/offers?product="+testProductID)
	requireStatus(t, rec, http.StatusNotFound)
	if p := decodeProblem(t, rec); p.Code != httpx.CodeNotFound {
		t.Fatalf("problem = %+v", p)
	}

	rec, _ = serve(t, &fakeReader{err: errBoom}, "/stores/offers?product="+testProductID)
	requireStatus(t, rec, http.StatusInternalServerError)
	if strings.Contains(rec.Body.String(), "relation") {
		t.Fatal("el body expone el error interno")
	}
}

// -------------------------------------------------------- GET /stores/{id}/offers

func TestListStoreOffers(t *testing.T) {
	t.Run("caso feliz con no disponibles", func(t *testing.T) {
		reader := &fakeReader{offers: []dto.Offer{{OfferID: 9, Available: false}}, total: 1}
		rec, _ := serve(t, reader, "/stores/3/offers?available=false&offset=5")
		requireStatus(t, rec, http.StatusOK)
		if reader.gotStoreID != 3 || *reader.gotFilter.Available || reader.gotFilter.Offset != 5 ||
			strings.Contains(rec.Body.String(), `"store"`) {
			t.Fatalf("filter = %+v body = %s", *reader.gotFilter, rec.Body.String())
		}
	})
	t.Run("id inválido", func(t *testing.T) {
		reader := &fakeReader{}
		rec, _ := serve(t, reader, "/stores/x/offers")
		requireStatus(t, rec, http.StatusBadRequest)
		if reader.calls != 0 {
			t.Fatal("no debe consultar el repositorio")
		}
	})
	t.Run("parámetro inválido", func(t *testing.T) {
		rec, _ := serve(t, &fakeReader{}, "/stores/3/offers?limit=abc")
		requireStatus(t, rec, http.StatusBadRequest)
		if got := fieldCodes(decodeProblem(t, rec)); !reflect.DeepEqual(got, []string{"limit/invalid_type"}) {
			t.Fatalf("errores = %v", got)
		}
	})
	t.Run("tienda inexistente", func(t *testing.T) {
		rec, _ := serve(t, &fakeReader{err: ErrNotFound}, "/stores/3/offers")
		requireStatus(t, rec, http.StatusNotFound)
	})
	t.Run("error interno", func(t *testing.T) {
		rec, _ := serve(t, &fakeReader{err: errBoom}, "/stores/3/offers")
		requireStatus(t, rec, http.StatusInternalServerError)
	})
}
