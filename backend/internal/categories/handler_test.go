package categories

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
)

// errBoom simula un error de base de datos con detalle interno.
var errBoom = errors.New(`pq: relation "categories" does not exist`)

type fakeRepository struct {
	items  []CategorySummary
	total  int64
	detail CategoryDetail
	err    error

	listCalls int
	gotParams ListParams
	getCalls  int
	gotSlug   string
}

func (f *fakeRepository) List(_ context.Context, params ListParams) ([]CategorySummary, int64, error) {
	f.listCalls++
	f.gotParams = params
	return f.items, f.total, f.err
}

func (f *fakeRepository) GetBySlug(_ context.Context, slug string) (CategoryDetail, error) {
	f.getCalls++
	f.gotSlug = slug
	if f.err != nil {
		return CategoryDetail{}, f.err
	}
	return f.detail, nil
}

// serve monta el handler como en el router real (RequestID + rutas chi)
// y devuelve la respuesta junto con lo que se registró en el log.
func serve(t *testing.T, repo Repository, target string) (*httptest.ResponseRecorder, string) {
	t.Helper()
	var logs bytes.Buffer
	h := NewHandler(repo, slog.New(slog.NewJSONHandler(&logs, nil)))

	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Get("/categories", h.List)
	r.Get("/categories/{slug}", h.GetBySlug)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec, logs.String()
}

func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder) httpx.Problem {
	t.Helper()
	if got := rec.Header().Get("Content-Type"); got != httpx.ProblemContentType {
		t.Fatalf("Content-Type = %q, quiero %q", got, httpx.ProblemContentType)
	}
	var p httpx.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("body no es un Problem válido: %v (%s)", err, rec.Body.String())
	}
	if p.RequestID == "" || p.RequestID != rec.Header().Get(httpx.RequestIDHeader) {
		t.Fatalf("request_id = %q, header = %q", p.RequestID, rec.Header().Get(httpx.RequestIDHeader))
	}
	return p
}

func requireJSON(t *testing.T, rec *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, quiero 200 (%s)", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), dst); err != nil {
		t.Fatalf("body inválido: %v (%s)", err, rec.Body.String())
	}
}

func strPtr(s string) *string { return &s }

func TestList_CasoFeliz(t *testing.T) {
	repo := &fakeRepository{
		items: []CategorySummary{
			{Slug: "notebooks", Name: "Notebooks", ItemType: "product", Description: strPtr("Portátiles"), AttributeCount: 6},
			{Slug: "televisores", Name: "Televisores", ItemType: "product", AttributeCount: 5},
		},
		total: 4,
	}

	rec, _ := serve(t, repo, "/categories")

	var got CategoryPage
	requireJSON(t, rec, &got)
	want := CategoryPage{Items: repo.items, Total: 4, Limit: DefaultLimit, Offset: 0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("body = %+v, quiero %+v", got, want)
	}
	if repo.gotParams != (ListParams{Limit: DefaultLimit}) {
		t.Fatalf("params al repositorio = %+v", repo.gotParams)
	}
}

func TestList_FiltroYPaginacion(t *testing.T) {
	repo := &fakeRepository{items: []CategorySummary{}, total: 1}

	rec, _ := serve(t, repo, "/categories?itemType=service&limit=100&offset=5")

	var got CategoryPage
	requireJSON(t, rec, &got)
	if got.Limit != 100 || got.Offset != 5 || got.Total != 1 {
		t.Fatalf("paginación en body = %+v", got)
	}
	p := repo.gotParams
	if p.ItemType == nil || *p.ItemType != ItemTypeService || p.Limit != 100 || p.Offset != 5 {
		t.Fatalf("params al repositorio = %+v", p)
	}
}

func TestList_SinResultadosDevuelveArregloVacio(t *testing.T) {
	rec, _ := serve(t, &fakeRepository{items: nil}, "/categories?offset=1000")

	var got CategoryPage
	requireJSON(t, rec, &got)
	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Fatalf("items debe ser [] y no null: %s", rec.Body.String())
	}
}

func TestList_ParametrosInvalidos(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantField string
		wantCode  string
	}{
		{"tipo inexistente", "itemType=foo", "itemType", "one_of"},
		{"tipo vacío", "itemType=", "itemType", "one_of"},
		{"tipo en mayúsculas", "itemType=PRODUCT", "itemType", "one_of"},
		{"limit cero", "limit=0", "limit", "min"},
		{"limit negativo", "limit=-1", "limit", "min"},
		{"limit no numérico", "limit=abc", "limit", "invalid_type"},
		{"limit sobre el máximo", "limit=101", "limit", "max"},
		{"limit enorme", "limit=1000000", "limit", "max"},
		{"limit desborda int", "limit=99999999999999999999", "limit", "invalid_type"},
		{"offset negativo", "offset=-1", "offset", "min"},
		{"offset no numérico", "offset=1.5", "offset", "invalid_type"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepository{}
			rec, _ := serve(t, repo, "/categories?"+tc.query)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, quiero 400 (%s)", rec.Code, rec.Body.String())
			}
			p := decodeProblem(t, rec)
			if p.Code != httpx.CodeInvalidParameter || p.Instance != "/categories" {
				t.Fatalf("problem = %+v", p)
			}
			if len(p.Errors) != 1 || p.Errors[0].Field != tc.wantField || p.Errors[0].Code != tc.wantCode {
				t.Fatalf("errors = %+v, quiero %s/%s", p.Errors, tc.wantField, tc.wantCode)
			}
			if repo.listCalls != 0 {
				t.Fatal("no debe consultar el repositorio con parámetros inválidos")
			}
		})
	}
}

func TestList_AcumulaErroresDeVariosParametros(t *testing.T) {
	rec, _ := serve(t, &fakeRepository{}, "/categories?itemType=x&limit=0&offset=-2")

	p := decodeProblem(t, rec)
	if rec.Code != http.StatusBadRequest || len(p.Errors) != 3 {
		t.Fatalf("status = %d, errors = %+v", rec.Code, p.Errors)
	}
}

func TestList_ErrorInternoNoExponeDetalles(t *testing.T) {
	rec, logs := serve(t, &fakeRepository{err: errBoom}, "/categories")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, quiero 500", rec.Code)
	}
	p := decodeProblem(t, rec)
	if p.Code != httpx.CodeInternalError {
		t.Fatalf("code = %q", p.Code)
	}
	if strings.Contains(rec.Body.String(), "relation") {
		t.Fatalf("el body expone el error interno: %s", rec.Body.String())
	}
	if !strings.Contains(logs, "relation") || !strings.Contains(logs, p.RequestID) {
		t.Fatalf("el log debe registrar el error y el request_id: %s", logs)
	}
}

func TestGetBySlug_CasoFeliz(t *testing.T) {
	lo, hi := 50.0, 1500.0
	repo := &fakeRepository{detail: CategoryDetail{
		Slug: "refrigeradores", Name: "Refrigeradores", ItemType: "product",
		Attributes: []Attribute{
			{Key: "capacidad_litros", Label: "Capacidad", DataType: "integer", Unit: strPtr("L"),
				Required: true, Comparable: true, Filterable: true, Min: &lo, Max: &hi, DisplayOrder: 1},
			{Key: "eficiencia_energetica", Label: "Eficiencia energética", DataType: "enum",
				Comparable: true, AllowedValues: []string{"A+", "A"}, DisplayOrder: 2},
		},
	}}

	rec, _ := serve(t, repo, "/categories/refrigeradores")

	var got CategoryDetail
	requireJSON(t, rec, &got)
	if !reflect.DeepEqual(got, repo.detail) {
		t.Fatalf("body = %+v, quiero %+v", got, repo.detail)
	}
	if repo.gotSlug != "refrigeradores" {
		t.Fatalf("slug al repositorio = %q", repo.gotSlug)
	}

	// Contrato estable: las restricciones que no aplican van como null.
	var raw struct {
		Description json.RawMessage              `json:"description"`
		Attributes  []map[string]json.RawMessage `json:"attributes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if string(raw.Description) != "null" {
		t.Fatalf("description = %s, quiero null", raw.Description)
	}
	nulls := map[int][]string{
		0: {"maxLength", "allowedValues"},      // integer
		1: {"unit", "min", "max", "maxLength"}, // enum
	}
	for i, keys := range nulls {
		for _, key := range keys {
			if v, ok := raw.Attributes[i][key]; !ok || string(v) != "null" {
				t.Fatalf("attributes[%d].%s = %s, quiero null presente", i, key, v)
			}
		}
	}
}

func TestGetBySlug_SinAtributosDevuelveArregloVacio(t *testing.T) {
	repo := &fakeRepository{detail: CategoryDetail{Slug: "vacia", Name: "Vacía", ItemType: "service"}}

	rec, _ := serve(t, repo, "/categories/vacia")

	var got CategoryDetail
	requireJSON(t, rec, &got)
	if !strings.Contains(rec.Body.String(), `"attributes":[]`) {
		t.Fatalf("attributes debe ser []: %s", rec.Body.String())
	}
}

func TestGetBySlug_SlugInvalido(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"mayúsculas", "/categories/Refrigeradores"},
		{"guion bajo", "/categories/refri_geradores"},
		{"demasiado largo", "/categories/" + strings.Repeat("a", 65)},
		{"espacio codificado", "/categories/refri%20geradores"},
		{"comilla", "/categories/a'b"},
		{"punto", "/categories/a.b"},
		{"acento", "/categories/fibra-%C3%B3ptica"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepository{}
			rec, _ := serve(t, repo, tc.path)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, quiero 400 (%s)", rec.Code, rec.Body.String())
			}
			p := decodeProblem(t, rec)
			if p.Code != httpx.CodeInvalidParameter || len(p.Errors) != 1 ||
				p.Errors[0].Field != "slug" || p.Errors[0].Code != "format" {
				t.Fatalf("problem = %+v", p)
			}
			if repo.getCalls != 0 {
				t.Fatal("no debe consultar el repositorio con un slug inválido")
			}
		})
	}
}

func TestGetBySlug_Inexistente(t *testing.T) {
	rec, _ := serve(t, &fakeRepository{err: ErrNotFound}, "/categories/no-existe")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, quiero 404", rec.Code)
	}
	p := decodeProblem(t, rec)
	if p.Code != httpx.CodeNotFound || p.Instance != "/categories/no-existe" {
		t.Fatalf("problem = %+v", p)
	}
}

func TestGetBySlug_ErrorInternoNoExponeDetalles(t *testing.T) {
	rec, logs := serve(t, &fakeRepository{err: errBoom}, "/categories/notebooks")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, quiero 500", rec.Code)
	}
	p := decodeProblem(t, rec)
	if p.Code != httpx.CodeInternalError || strings.Contains(rec.Body.String(), "relation") {
		t.Fatalf("problem = %+v body = %s", p, rec.Body.String())
	}
	if !strings.Contains(logs, "relation") || !strings.Contains(logs, p.RequestID) {
		t.Fatalf("el log debe registrar el error y el request_id: %s", logs)
	}
}
