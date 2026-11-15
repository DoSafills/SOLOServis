package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/DoSafills/SOLOServis/backend/internal/services/dto"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const testServiceID = "11111111-2222-3333-4444-555555555555"

// errBoom simula un error de base de datos con detalle interno.
var errBoom = errors.New(`relation "service" does not exist`)

type fakeStore struct {
	services []generated.ListServicesRow
	service  generated.GetServiceByPublicIDRow
	offers   []generated.ListServiceOffersRow
	specs    []generated.ListServiceSpecificationsRow
	history  []generated.ListServicePriceHistoryRow

	err        error // lo devuelven List y GetByPublicID
	relatedErr error // lo devuelven ListOffers, ListSpecifications y ListPriceHistory

	gotPublicID pgtype.UUID
}

func (f *fakeStore) List(context.Context) ([]generated.ListServicesRow, error) {
	return f.services, f.err
}

func (f *fakeStore) GetByPublicID(_ context.Context, id pgtype.UUID) (generated.GetServiceByPublicIDRow, error) {
	f.gotPublicID = id
	return f.service, f.err
}

func (f *fakeStore) ListOffers(context.Context, int32) ([]generated.ListServiceOffersRow, error) {
	return f.offers, f.relatedErr
}

func (f *fakeStore) ListSpecifications(context.Context, int32) ([]generated.ListServiceSpecificationsRow, error) {
	return f.specs, f.relatedErr
}

func (f *fakeStore) ListPriceHistory(context.Context, int32) ([]generated.ListServicePriceHistoryRow, error) {
	return f.history, f.relatedErr
}

func uuidOf(t *testing.T, value string) pgtype.UUID {
	t.Helper()
	var id pgtype.UUID
	if err := id.Scan(value); err != nil {
		t.Fatal(err)
	}
	return id
}

// serve monta el handler con las mismas rutas que el router real.
func serve(t *testing.T, store Store, target string) (*httptest.ResponseRecorder, string) {
	t.Helper()
	var logs bytes.Buffer
	h := NewHandler(store, slog.New(slog.NewJSONHandler(&logs, nil)))

	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Get("/services", h.List)
	r.Get("/services/{publicID}", h.GetByPublicID)

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

func TestList_Feliz(t *testing.T) {
	store := &fakeStore{
		services: []generated.ListServicesRow{{ID: 7, PublicID: uuidOf(t, testServiceID), Name: "Fibra 600", CategoryName: "Internet"}},
		offers:   []generated.ListServiceOffersRow{{ID: 3, ServiceID: 7, ProviderName: "Movistar"}},
		specs:    []generated.ListServiceSpecificationsRow{{Name: "velocidad", Value: "600 Mbps"}},
	}
	rec, _ := serve(t, store, "/services")
	requireStatus(t, rec, http.StatusOK)
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q", got)
	}
	var body []dto.ServiceDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 1 || body[0].ID != testServiceID || body[0].Provider != "Movistar" || body[0].Specs["velocidad"] != "600 Mbps" {
		t.Fatalf("body = %+v", body)
	}
}

func TestList_VacioDevuelveArreglo(t *testing.T) {
	rec, _ := serve(t, &fakeStore{}, "/services")
	requireStatus(t, rec, http.StatusOK)
	if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
		t.Fatalf("body = %s, quiero []", got)
	}
}

func TestGetByPublicID_Feliz(t *testing.T) {
	store := &fakeStore{
		service: generated.GetServiceByPublicIDRow{ID: 7, PublicID: uuidOf(t, testServiceID), Name: "Fibra 600", CategoryName: "Internet"},
		offers:  []generated.ListServiceOffersRow{{ID: 3, ServiceID: 7, ProviderName: "Movistar"}},
	}
	rec, _ := serve(t, store, "/services/"+testServiceID)
	requireStatus(t, rec, http.StatusOK)
	if store.gotPublicID != uuidOf(t, testServiceID) {
		t.Fatalf("GetByPublicID recibió %v", store.gotPublicID)
	}
	var body dto.ServiceDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.ID != testServiceID || body.Name != "Fibra 600" || body.OfferID != 3 {
		t.Fatalf("body = %+v", body)
	}
}

func TestGetByPublicID_UUIDInvalido(t *testing.T) {
	rec, _ := serve(t, &fakeStore{}, "/services/no-es-uuid")
	requireStatus(t, rec, http.StatusBadRequest)
	if p := decodeProblem(t, rec); p.Code != httpx.CodeInvalidParameter {
		t.Fatalf("code = %q", p.Code)
	}
}

func TestGetByPublicID_Inexistente(t *testing.T) {
	rec, _ := serve(t, &fakeStore{err: pgx.ErrNoRows}, "/services/"+testServiceID)
	requireStatus(t, rec, http.StatusNotFound)
	if p := decodeProblem(t, rec); p.Code != httpx.CodeNotFound {
		t.Fatalf("code = %q", p.Code)
	}
}

func TestErrorInterno_NoExponeDetalles(t *testing.T) {
	tests := []struct {
		name   string
		store  *fakeStore
		target string
	}{
		{"List falla", &fakeStore{err: errBoom}, "/services"},
		{"List falla al cargar ofertas", &fakeStore{
			services:   []generated.ListServicesRow{{ID: 7}},
			relatedErr: errBoom,
		}, "/services"},
		{"GetByPublicID falla", &fakeStore{err: errBoom}, "/services/" + testServiceID},
		{"GetByPublicID falla al cargar ofertas", &fakeStore{relatedErr: errBoom}, "/services/" + testServiceID},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec, logs := serve(t, tc.store, tc.target)
			requireStatus(t, rec, http.StatusInternalServerError)
			p := decodeProblem(t, rec)
			if p.Code != httpx.CodeInternalError {
				t.Fatalf("code = %q", p.Code)
			}
			if strings.Contains(rec.Body.String(), "relation") {
				t.Fatalf("la respuesta expone el error interno: %s", rec.Body.String())
			}
			if !strings.Contains(logs, "relation") || !strings.Contains(logs, p.RequestID) {
				t.Fatalf("el log no registra el error con su request_id: %s", logs)
			}
		})
	}
}
