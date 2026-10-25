package search

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
	"github.com/go-chi/chi/v5"
)

// errBoom simula un error de base de datos con detalle interno.
var errBoom = errors.New(`relation "product" does not exist`)

func floatp(f float64) *float64 { return &f }

// refrigeradores replica las definiciones sembradas (TAL-71).
var refrigeradores = []categories.Attribute{
	{ID: 10, Key: "capacidad_litros", DataType: "integer", Filterable: true, Min: floatp(50), Max: floatp(1500)},
	{ID: 11, Key: "eficiencia_energetica", DataType: "enum", Filterable: true, AllowedValues: []string{"A+++", "A++", "A+", "A"}},
	{ID: 12, Key: "no_frost", DataType: "boolean", Filterable: true},
	{ID: 13, Key: "peso_kg", DataType: "decimal", Filterable: false},
	{ID: 14, Key: "color", DataType: "string", Filterable: true},
}

type fakeSearcher struct {
	defs      []categories.Attribute
	defsErr   error
	results   []ProductResult
	total     int64
	facets    Facets
	err       error
	gotQuery  *Query
	gotDefs   []categories.Attribute
	gotText   string
	gotType   *string
	dbTouched int
}

func (f *fakeSearcher) CategoryDefinitions(context.Context, string) ([]categories.Attribute, error) {
	f.dbTouched++
	return f.defs, f.defsErr
}
func (f *fakeSearcher) SearchProducts(_ context.Context, q Query) ([]ProductResult, int64, error) {
	f.dbTouched++
	f.gotQuery = &q
	return f.results, f.total, f.err
}
func (f *fakeSearcher) SearchFacets(_ context.Context, q Query, defs []categories.Attribute) (Facets, error) {
	f.dbTouched++
	f.gotQuery = &q
	f.gotDefs = defs
	return f.facets, f.err
}
func (f *fakeSearcher) SearchCategories(_ context.Context, text string, itemType *string) ([]CategoryResult, error) {
	f.dbTouched++
	f.gotText, f.gotType = text, itemType
	return []CategoryResult{{Slug: "refrigeradores"}}, f.err
}
func (f *fakeSearcher) SearchBrands(_ context.Context, text string) ([]BrandResult, error) {
	f.dbTouched++
	f.gotText = text
	return []BrandResult{{ID: 1, Name: "LG"}}, f.err
}

func serve(t *testing.T, searcher Searcher, target string) (*httptest.ResponseRecorder, string) {
	t.Helper()
	var logs bytes.Buffer
	h := NewHandler(searcher, slog.New(slog.NewJSONHandler(&logs, nil)))
	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Get("/search/products", h.Products)
	r.Get("/search/facets", h.Facets)
	r.Get("/search/categories", h.Categories)
	r.Get("/search/brands", h.Brands)
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

func fieldCodes(t *testing.T, rec *httptest.ResponseRecorder) []string {
	t.Helper()
	var p httpx.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || p.Code != httpx.CodeInvalidParameter {
		t.Fatalf("problem = %+v (%v)", p, err)
	}
	if p.RequestID == "" || p.RequestID != rec.Header().Get(httpx.RequestIDHeader) {
		t.Fatalf("request_id = %q", p.RequestID)
	}
	out := []string{}
	for _, e := range p.Errors {
		out = append(out, e.Field+"/"+e.Code)
	}
	return out
}

// -------------------------------------------------------- /search/products

func TestProducts_CasoFelizYDefaults(t *testing.T) {
	s := &fakeSearcher{results: []ProductResult{{ID: "x", Name: "Refrigerador"}}, total: 12}

	rec, _ := serve(t, s, "/search/products")

	requireStatus(t, rec, http.StatusOK)
	if rec.Header().Get("X-Total-Count") != "12" || !strings.HasPrefix(rec.Body.String(), "[") {
		t.Fatalf("headers/body = %s %s", rec.Header().Get("X-Total-Count"), rec.Body.String())
	}
	q := *s.gotQuery
	if q.Sort != "name" || q.Limit != DefaultLimit || q.Offset != 0 || len(q.Words) != 0 || q.Available {
		t.Fatalf("query = %+v", q)
	}
}

func TestProducts_TodosLosFiltros(t *testing.T) {
	s := &fakeSearcher{defs: refrigeradores}

	rec, _ := serve(t, s, "/search/products?q=+lg++no%20frost+&category=refrigeradores&brand=1,%202"+
		"&minPrice=100000&maxPrice=500000.50&available=true&sort=price_asc&limit=50&offset=10"+
		"&attr.capacidad_litros.min=300&attr.capacidad_litros.max=600&attr.eficiencia_energetica=A%2B"+
		"&attr.no_frost=true&attr.color=Gris")

	requireStatus(t, rec, http.StatusOK)
	q := *s.gotQuery
	if !reflect.DeepEqual(q.Words, []string{"lg", "no", "frost"}) || *q.CategorySlug != "refrigeradores" ||
		!reflect.DeepEqual(q.BrandIDs, []int32{1, 2}) || *q.MinPrice != "100000" || *q.MaxPrice != "500000.50" ||
		!q.Available || q.Sort != "price_asc" || q.Limit != 50 || q.Offset != 10 {
		t.Fatalf("query = %+v", q)
	}
	// Ordenados por nombre de parámetro: determinista.
	want := []AttributeFilter{
		{AttributeID: 10, Op: OpMax, Value: "600"},
		{AttributeID: 10, Op: OpMin, Value: "300"},
		{AttributeID: 14, Op: OpEqualInsensitive, Value: "Gris"},
		{AttributeID: 11, Op: OpEqual, Value: "A+"},
		{AttributeID: 12, Op: OpEqual, Value: "true"},
	}
	if !reflect.DeepEqual(q.Attributes, want) {
		t.Fatalf("filtros =\n%+v\nquiero\n%+v", q.Attributes, want)
	}
}

func TestProducts_IgualdadNumericaYCategoriaAntigua(t *testing.T) {
	s := &fakeSearcher{defs: refrigeradores}
	rec, _ := serve(t, s, "/search/products?category=refrigeradores&attr.capacidad_litros=350")
	requireStatus(t, rec, http.StatusOK)
	if !reflect.DeepEqual(s.gotQuery.Attributes, []AttributeFilter{{AttributeID: 10, Op: OpEqualNumeric, Value: "350"}}) {
		t.Fatalf("filtros = %+v", s.gotQuery.Attributes)
	}

	s = &fakeSearcher{}
	rec, _ = serve(t, s, "/search/products?category=5")
	requireStatus(t, rec, http.StatusOK)
	if *s.gotQuery.LegacyCategoryID != 5 || s.dbTouched != 1 {
		t.Fatalf("una categoría antigua no carga definiciones: %+v (%d)", *s.gotQuery, s.dbTouched)
	}
}

func TestProducts_ParametrosInvalidos(t *testing.T) {
	tests := []struct{ query, want string }{
		{"q=" + strings.Repeat("x", 201), "q/max_length"},
		{"q=a+b+c+d+e+f", "q/max"},
		{"category=Bad_Slug", "category/format"},
		{"category=0", "category/min"},
		{"brand=1,x", "brand/format"},
		{"brand=0", "brand/format"},
		{"brand=" + strings.Repeat("1,", 20) + "1", "brand/max"},
		{"minPrice=-1", "minPrice/format"},
		{"maxPrice=1.999", "maxPrice/format"},
		{"minPrice=10&maxPrice=5", "maxPrice/min"},
		{"available=si", "available/one_of"},
		{"sort=relevance", "sort/one_of"},
		{"limit=101", "limit/max"},
		{"limit=0", "limit/min"},
		{"offset=-1", "offset/min"},
		{"attr.capacidad_litros=300", "category/required"},
		{"category=5&attr.no_frost=true", "category/required"},
	}
	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			s := &fakeSearcher{defs: refrigeradores}
			rec, _ := serve(t, s, "/search/products?"+tc.query)
			requireStatus(t, rec, http.StatusBadRequest)
			if got := fieldCodes(t, rec); !reflect.DeepEqual(got, []string{tc.want}) {
				t.Fatalf("errores = %v, quiero [%s]", got, tc.want)
			}
			if s.dbTouched != 0 {
				t.Fatal("con parámetros inválidos no se consulta la base")
			}
		})
	}
}

func TestProducts_FiltrosDeAtributoInvalidos(t *testing.T) {
	tests := []struct{ attr, want string }{
		{"attr.no_existe=1", "attr.no_existe/unknown_field"},
		{"attr.peso_kg=65", "attr.peso_kg/not_filterable"},
		{"attr.capacidad_litros=abc", "attr.capacidad_litros/invalid_type"},
		{"attr.capacidad_litros.min=3.5", "attr.capacidad_litros.min/invalid_type"},
		{"attr.no_frost=si", "attr.no_frost/invalid_type"},
		{"attr.no_frost.min=1", "attr.no_frost.min/invalid_type"},
		{"attr.eficiencia_energetica=B", "attr.eficiencia_energetica/one_of"},
		{"attr.color=", "attr.color/required"},
	}
	for _, tc := range tests {
		t.Run(tc.attr, func(t *testing.T) {
			s := &fakeSearcher{defs: refrigeradores}
			rec, _ := serve(t, s, "/search/products?category=refrigeradores&"+tc.attr)
			requireStatus(t, rec, http.StatusBadRequest)
			if got := fieldCodes(t, rec); !reflect.DeepEqual(got, []string{tc.want}) {
				t.Fatalf("errores = %v, quiero [%s]", got, tc.want)
			}
			if s.gotQuery != nil {
				t.Fatal("no debe buscar con filtros inválidos")
			}
		})
	}
}

func TestProducts_CategoriaInexistenteYErrores(t *testing.T) {
	rec, _ := serve(t, &fakeSearcher{defsErr: ErrCategoryNotFound}, "/search/products?category=no-existe")
	requireStatus(t, rec, http.StatusBadRequest)
	if got := fieldCodes(t, rec); !reflect.DeepEqual(got, []string{"category/not_found"}) {
		t.Fatalf("errores = %v", got)
	}

	rec, logs := serve(t, &fakeSearcher{defsErr: errBoom}, "/search/products?category=refrigeradores")
	requireStatus(t, rec, http.StatusInternalServerError)
	if strings.Contains(rec.Body.String(), "relation") || !strings.Contains(logs, "relation") {
		t.Fatalf("el detalle debe ir solo al log: %s", rec.Body.String())
	}

	rec, _ = serve(t, &fakeSearcher{err: errBoom}, "/search/products")
	requireStatus(t, rec, http.StatusInternalServerError)

	rec, _ = serve(t, &fakeSearcher{}, "/search/products?q=nada")
	requireStatus(t, rec, http.StatusOK)
	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("sin resultados debe responder [], no %s", rec.Body.String())
	}
}

// ---------------------------------------------------------- /search/facets

func TestFacets(t *testing.T) {
	s := &fakeSearcher{defs: refrigeradores, facets: Facets{Total: 3, Brands: []BrandFacet{}, Attributes: []AttributeFacet{}}}
	rec, _ := serve(t, s, "/search/facets?category=refrigeradores&q=lg&attr.no_frost=true")
	requireStatus(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"total":3`) || len(s.gotDefs) != len(refrigeradores) ||
		len(s.gotQuery.Attributes) != 1 {
		t.Fatalf("body = %s defs = %d", rec.Body.String(), len(s.gotDefs))
	}

	// Las facetas no paginan: sort/limit no son parámetros válidos aquí, se ignoran.
	s = &fakeSearcher{facets: Facets{Brands: []BrandFacet{}, Attributes: []AttributeFacet{}}}
	rec, _ = serve(t, s, "/search/facets?limit=9999")
	requireStatus(t, rec, http.StatusOK)

	rec, _ = serve(t, &fakeSearcher{}, "/search/facets?minPrice=x")
	requireStatus(t, rec, http.StatusBadRequest)
	rec, _ = serve(t, &fakeSearcher{err: errBoom}, "/search/facets")
	requireStatus(t, rec, http.StatusInternalServerError)
}

// ------------------------------------------------ /search/categories, brands

func TestCategoriesYBrands(t *testing.T) {
	s := &fakeSearcher{}
	rec, _ := serve(t, s, "/search/categories?q=%20refri%20&itemType=product")
	requireStatus(t, rec, http.StatusOK)
	if s.gotText != "refri" || *s.gotType != "product" || !strings.Contains(rec.Body.String(), `"slug":"refrigeradores"`) {
		t.Fatalf("text = %q type = %v body = %s", s.gotText, s.gotType, rec.Body.String())
	}

	rec, _ = serve(t, &fakeSearcher{}, "/search/categories?itemType=otro")
	requireStatus(t, rec, http.StatusBadRequest)
	if got := fieldCodes(t, rec); !reflect.DeepEqual(got, []string{"itemType/one_of"}) {
		t.Fatalf("errores = %v", got)
	}

	s = &fakeSearcher{}
	rec, _ = serve(t, s, "/search/brands?q=lg")
	requireStatus(t, rec, http.StatusOK)
	if s.gotText != "lg" || !strings.Contains(rec.Body.String(), `"name":"LG"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}

	rec, _ = serve(t, &fakeSearcher{}, "/search/brands?q="+strings.Repeat("x", 201))
	requireStatus(t, rec, http.StatusBadRequest)
	rec, _ = serve(t, &fakeSearcher{err: errBoom}, "/search/brands")
	requireStatus(t, rec, http.StatusInternalServerError)
	rec, _ = serve(t, &fakeSearcher{err: errBoom}, "/search/categories")
	requireStatus(t, rec, http.StatusInternalServerError)
}
