package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DoSafills/SOLOServis/backend/internal/auth"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
)

// Los casos usan rutas que responden antes de consultar la base de datos,
// por eso los routers se construyen con un pool nil. wantCode es el code del
// problem+json (Product API desde TAL-70, Store API desde TAL-73); wantBody
// fija el body exacto cuando no hay problem+json (p. ej. preflight vacío).
func TestRouters_CableadoSinCambiarRespuestas(t *testing.T) {
	const frontendURL = "http://localhost:5173"

	tests := []struct {
		name        string
		router      http.Handler
		method      string
		path        string
		body        string
		contentType string
		wantStatus  int
		wantBody    string
		wantCode    string
	}{
		{"product preflight CORS", NewProductRouter(nil, frontendURL), http.MethodOptions, "/products", "", "", 204, "", ""},
		{"product id inválido", NewProductRouter(nil, frontendURL), http.MethodGet, "/products/no-es-uuid", "", "", 400, "", httpx.CodeInvalidParameter},
		{"product create sin token", NewProductRouter(nil, frontendURL), http.MethodPost, "/products", `{"nombre":"x"}`, "application/json", 401, "", httpx.CodeUnauthorized},
		{"product update sin token", NewProductRouter(nil, frontendURL), http.MethodPut, "/products/x", `{}`, "application/json", 401, "", httpx.CodeUnauthorized},
		{"product delete sin token", NewProductRouter(nil, frontendURL), http.MethodDelete, "/products/x", "", "", 401, "", httpx.CodeUnauthorized},
		{"store preflight CORS", NewStoreRouter(nil, frontendURL), http.MethodOptions, "/stores", "", "", 204, "", ""},
		{"store id inválido", NewStoreRouter(nil, frontendURL), http.MethodGet, "/stores/abc", "", "", 400, "", httpx.CodeInvalidParameter},
		{"store offers sin product", NewStoreRouter(nil, frontendURL), http.MethodGet, "/stores/offers", "", "", 400, "", httpx.CodeInvalidParameter},
		{"store escritura no existe", NewStoreRouter(nil, frontendURL), http.MethodPost, "/stores", `{}`, "application/json", 405, "", httpx.CodeMethodNotAllowed},
		{"search parámetro inválido", NewSearchRouter(nil, frontendURL), http.MethodGet, "/search/products?limit=0", "", "", 400, "", httpx.CodeInvalidParameter},
		{"pricing id inválido", NewPricingRouter(nil, frontendURL), http.MethodGet, "/offers/abc", "", "", 400, "", httpx.CodeInvalidParameter},
		{"ingestion sin token", NewIngestionRouter(nil, frontendURL), http.MethodPost, "/ingestion/runs", `{}`, "application/json", 401, "", httpx.CodeUnauthorized},
		{"ingestion lectura sin token", NewIngestionRouter(nil, frontendURL), http.MethodGet, "/ingestion/runs", "", "", 401, "", httpx.CodeUnauthorized},
		{"pricing POST sin token", NewPricingRouter(nil, frontendURL), http.MethodPost, "/offers", `{}`, "application/json", 401, "", httpx.CodeUnauthorized},
		{"pricing PUT sin token", NewPricingRouter(nil, frontendURL), http.MethodPut, "/offers/1", `{}`, "application/json", 401, "", httpx.CodeUnauthorized},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			rec := httptest.NewRecorder()

			tc.router.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, quiero %d (%s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantCode != "" {
				var p httpx.Problem
				if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || p.Code != tc.wantCode ||
					rec.Header().Get("Content-Type") != httpx.ProblemContentType {
					t.Fatalf("problem = %+v (%v), quiero code %q", p, err, tc.wantCode)
				}
			} else if rec.Body.String() != tc.wantBody {
				t.Fatalf("body = %q, quiero %q", rec.Body.String(), tc.wantBody)
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != frontendURL {
				t.Fatalf("CORS alterado: Access-Control-Allow-Origin = %q", got)
			}
			if !httpx.ValidRequestID(rec.Header().Get(httpx.RequestIDHeader)) {
				t.Fatalf("falta X-Request-ID válido: %q", rec.Header().Get(httpx.RequestIDHeader))
			}
		})
	}
}

// assertRoutingProblem verifica que rec sea un problem+json del estándar
// TAL-78 con el status, el code y el detail dados y con request_id. Recibe el
// test, la respuesta grabada y lo esperado; no devuelve nada y falla el test
// si algo no coincide.
func assertRoutingProblem(t *testing.T, rec *httptest.ResponseRecorder, status int, code, detail string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, quiero %d (%s)", rec.Code, status, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != httpx.ProblemContentType {
		t.Fatalf("Content-Type = %q, quiero %q", got, httpx.ProblemContentType)
	}
	var p httpx.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("body no es un Problem: %v (%s)", err, rec.Body.String())
	}
	if p.Status != status || p.Code != code || p.Detail != detail || !httpx.ValidRequestID(p.RequestID) {
		t.Fatalf("problem = %+v, quiero status %d, code %q, detail %q y request_id", p, status, code, detail)
	}
}

// Métodos no registrados (TAL-245): 405 problem+json con Allow, que lista los
// métodos con que la ruta sí responde.
func TestRouters_MetodosNoRegistradosResponden405(t *testing.T) {
	tests := []struct {
		name   string
		router http.Handler
		method string
		path   string
		allow  string
	}{
		{"PATCH products", NewProductRouter(nil, "http://localhost:5173"), http.MethodPatch, "/products/00000000-0000-0000-0000-000000000001", "GET, PUT, DELETE"},
		{"POST stores", NewStoreRouter(nil, "http://localhost:5173"), http.MethodPost, "/stores", "GET"},
		{"PATCH stores", NewStoreRouter(nil, "http://localhost:5173"), http.MethodPatch, "/stores/1", "GET"},
		{"POST services", NewServiceRouter(nil, "http://localhost:5173"), http.MethodPost, "/services", "GET"},
		{"POST search", NewSearchRouter(nil, "http://localhost:5173"), http.MethodPost, "/search/products", "GET"},
		{"DELETE search", NewSearchRouter(nil, "http://localhost:5173"), http.MethodDelete, "/search/products", "GET"},
		{"GET offers", NewPricingRouter(nil, "http://localhost:5173"), http.MethodGet, "/offers", "POST"},
		{"PATCH comparisons", NewComparisonRouter(nil, "http://localhost:5173"), http.MethodPatch, "/comparisons/products/1", "GET, PUT, DELETE"},
		{"GET auth login", auth.NewRouter(nil, "http://localhost:5173"), http.MethodGet, "/auth/login", "POST"},
		{"DELETE ingestion run", NewIngestionRouter(nil, "http://localhost:5173"), http.MethodDelete, "/ingestion/runs/1", "GET"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tc.router.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`)))
			assertRoutingProblem(t, rec, http.StatusMethodNotAllowed, httpx.CodeMethodNotAllowed, httpx.MethodNotAllowedDetail)
			if got := rec.Header().Get("Allow"); got != tc.allow {
				t.Fatalf("Allow = %q, quiero %q", got, tc.allow)
			}
		})
	}
}

// Rutas inexistentes dentro de una API (TAL-245): 404 problem+json en vez del
// "404 page not found" en text/plain de chi. Incluye la raíz del prefijo de
// cada API sin ruta propia y rutas más largas que cualquiera registrada.
func TestRouters_RutaInexistenteResponde404(t *testing.T) {
	const frontendURL = "http://localhost:5173"
	tests := []struct {
		name   string
		router http.Handler
		method string
		path   string
	}{
		{"product ruta larga", NewProductRouter(nil, frontendURL), http.MethodGet, "/products/x/y/z"},
		{"store ruta larga", NewStoreRouter(nil, frontendURL), http.MethodGet, "/stores/1/offers/x"},
		{"service ruta larga", NewServiceRouter(nil, frontendURL), http.MethodGet, "/services/x/y"},
		{"user ruta larga", NewUserRouter(nil, frontendURL), http.MethodGet, "/users/1/x"},
		{"pricing ruta larga", NewPricingRouter(nil, frontendURL), http.MethodGet, "/offers/1/x"},
		{"search raíz", NewSearchRouter(nil, frontendURL), http.MethodGet, "/search"},
		{"comparison raíz", NewComparisonRouter(nil, frontendURL), http.MethodGet, "/comparisons"},
		{"ingestion raíz", NewIngestionRouter(nil, frontendURL), http.MethodGet, "/ingestion"},
		{"ingestion POST inexistente", NewIngestionRouter(nil, frontendURL), http.MethodPost, "/ingestion/x"},
		{"auth raíz", auth.NewRouter(nil, frontendURL), http.MethodGet, "/auth"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tc.router.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			assertRoutingProblem(t, rec, http.StatusNotFound, httpx.CodeNotFound, httpx.RouteNotFoundDetail)
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != frontendURL {
				t.Fatalf("CORS alterado: Access-Control-Allow-Origin = %q", got)
			}
		})
	}
}

func TestRouters_PropaganRequestIDValido(t *testing.T) {
	for name, router := range map[string]http.Handler{
		"product": NewProductRouter(nil, "http://localhost:5173"),
		"store":   NewStoreRouter(nil, "http://localhost:5173"),
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodOptions, "/", nil)
			req.Header.Set(httpx.RequestIDHeader, "cliente-123")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if got := rec.Header().Get(httpx.RequestIDHeader); got != "cliente-123" {
				t.Fatalf("X-Request-ID = %q, quiero cliente-123", got)
			}
		})
	}
}

// Las rutas de categorías (TAL-71) están cableadas en Product API y son
// solo de lectura. Los casos responden antes de consultar la base de datos.
func TestProductRouter_CategoriasSoloLectura(t *testing.T) {
	router := NewProductRouter(nil, "http://localhost:5173")

	req := httptest.NewRequest(http.MethodGet, "/categories/Slug_Invalido", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Header().Get("Content-Type") != httpx.ProblemContentType {
		t.Fatalf("GET slug inválido = %d %q, quiero 400 problem+json", rec.Code, rec.Header().Get("Content-Type"))
	}

	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/categories"},
		{http.MethodPut, "/categories/refrigeradores"},
		{http.MethodPatch, "/categories/refrigeradores"},
		{http.MethodDelete, "/categories/refrigeradores"},
		{http.MethodPost, "/categories/refrigeradores"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`)))
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, quiero 405", rec.Code)
			}
		})
	}
}

// Product API expone el modelo nuevo de categorías (TAL-71) en /categories y
// conserva el árbol anterior en /product-categories mientras migra el frontend.
func TestProductRouter_RutasRegistradas(t *testing.T) {
	want := []string{
		"GET /categories",
		"GET /categories/artworks/{key}",
		"GET /categories/{slug}",
		"GET /health",
		"GET /product-categories",
		"GET /products",
		"GET /products/{publicID}",
		"GET /products/{publicID}/reviews",
		"POST /products/duplicate-candidates",
		"POST /products",
		"PUT /products/{publicID}",
		"DELETE /products/{publicID}",
	}

	got := map[string]bool{}
	walk := func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		got[method+" "+route] = true
		return nil
	}
	if err := chi.Walk(NewProductRouter(nil, "http://localhost:5173"), walk); err != nil {
		t.Fatal(err)
	}

	for _, route := range want {
		if !got[route] {
			t.Errorf("falta la ruta %s", route)
		}
	}
	if len(got) != len(want) {
		t.Errorf("rutas registradas = %v, quiero exactamente %v", got, want)
	}
}

// Store API (TAL-73): solo lectura, con las ofertas por producto y por tienda.
func TestStoreRouter_RutasRegistradas(t *testing.T) {
	want := []string{
		"GET /health",
		"GET /stores",
		"GET /stores/offers",
		"GET /stores/{id}",
		"GET /stores/{id}/offers",
	}

	got := map[string]bool{}
	walk := func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		got[method+" "+route] = true
		return nil
	}
	if err := chi.Walk(NewStoreRouter(nil, "http://localhost:5173"), walk); err != nil {
		t.Fatal(err)
	}

	for _, route := range want {
		if !got[route] {
			t.Errorf("falta la ruta %s", route)
		}
	}
	if len(got) != len(want) {
		t.Errorf("rutas registradas = %v, quiero exactamente %v", got, want)
	}
}

// Pricing API (TAL-74): rutas propias con el estándar; escrituras protegidas.
func TestPricingRouter_RutasRegistradas(t *testing.T) {
	want := []string{
		"GET /health",
		"GET /offers/product/{productId}",
		"GET /offers/product/{productId}/best",
		"GET /offers/product/{productId}/price-history",
		"GET /offers/{id}",
		"GET /offers/{id}/price-history",
		"POST /offers",
		"PUT /offers/{id}",
	}

	got := map[string]bool{}
	walk := func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		got[method+" "+route] = true
		return nil
	}
	if err := chi.Walk(NewPricingRouter(nil, "http://localhost:5173"), walk); err != nil {
		t.Fatal(err)
	}

	for _, route := range want {
		if !got[route] {
			t.Errorf("falta la ruta %s", route)
		}
	}
	if len(got) != len(want) {
		t.Errorf("rutas registradas = %v, quiero exactamente %v", got, want)
	}
}

// Search API (TAL-75): búsqueda pública propia + historial y búsquedas
// guardadas (con sesión) que siguen en features.
func TestSearchRouter_RutasRegistradas(t *testing.T) {
	want := []string{
		"GET /health",
		"GET /search/products",
		"GET /search/popular-products",
		"GET /search/presets",
		"GET /search/presets/{slug}",
		"GET /search/alternatives",
		"GET /search/facets",
		"GET /search/categories",
		"GET /search/brands",
		"POST /search/history",
		"GET /search/history",
		"POST /search/saved",
		"GET /search/saved",
		"DELETE /search/saved/{id}",
	}

	got := map[string]bool{}
	walk := func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		got[method+" "+route] = true
		return nil
	}
	if err := chi.Walk(NewSearchRouter(nil, "http://localhost:5173"), walk); err != nil {
		t.Fatal(err)
	}

	for _, route := range want {
		if !got[route] {
			t.Errorf("falta la ruta %s", route)
		}
	}
	if len(got) != len(want) {
		t.Errorf("rutas registradas = %v, quiero exactamente %v", got, want)
	}
}

// Comparison API (TAL-76): comparación pública, guardadas con sesión y las de
// servicios que siguen en features.
func TestComparisonRouter_RutasRegistradas(t *testing.T) {
	want := []string{
		"GET /health",
		"GET /comparisons/products/compare",
		"POST /comparisons/assistant",
		"GET /comparisons/compatible/{productId}",
		"POST /comparisons/products",
		"GET /comparisons/products",
		"GET /comparisons/products/{id}",
		"PUT /comparisons/products/{id}",
		"DELETE /comparisons/products/{id}",
		"POST /comparisons/products/{id}/items",
		"DELETE /comparisons/products/{id}/items/{productId}",
		"POST /comparisons/services",
		"GET /comparisons/services/{id}",
		"DELETE /comparisons/services/{id}",
		"POST /comparisons/services/{id}/items",
		"DELETE /comparisons/services/{id}/items/{serviceId}",
	}

	got := map[string]bool{}
	walk := func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		got[method+" "+route] = true
		return nil
	}
	if err := chi.Walk(NewComparisonRouter(nil, "http://localhost:5173"), walk); err != nil {
		t.Fatal(err)
	}

	for _, route := range want {
		if !got[route] {
			t.Errorf("falta la ruta %s", route)
		}
	}
	if len(got) != len(want) {
		t.Errorf("rutas registradas = %v, quiero exactamente %v", got, want)
	}
}

// Ingestion API (TAL-77): todas las rutas exigen admin o INGESTION_API_TOKEN (TAL-199).
func TestIngestionRouter_RutasRegistradas(t *testing.T) {
	want := []string{
		"GET /health",
		"POST /ingestion/runs",
		"GET /ingestion/runs",
		"GET /ingestion/runs/{id}",
		"POST /ingestion/runs/{id}/items",
		"POST /ingestion/runs/{id}/services",
		"POST /ingestion/runs/{id}/finish",
	}

	got := map[string]bool{}
	walk := func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		got[method+" "+route] = true
		return nil
	}
	if err := chi.Walk(NewIngestionRouter(nil, "http://localhost:5173"), walk); err != nil {
		t.Fatal(err)
	}

	for _, route := range want {
		if !got[route] {
			t.Errorf("falta la ruta %s", route)
		}
	}
	if len(got) != len(want) {
		t.Errorf("rutas registradas = %v, quiero exactamente %v", got, want)
	}
}

// TestIngestionRouter_AceptaElTokenDeIngestion comprueba que /ingestion/* deja
// pasar INGESTION_API_TOKEN (TAL-199, H3): el body vacío llega al handler y
// responde 422 sin consultar la base. Que INTERNAL_API_TOKEN ya no sirva se
// prueba con PostgreSQL (integration), porque cae en la búsqueda de sesión.
func TestIngestionRouter_AceptaElTokenDeIngestion(t *testing.T) {
	const ingestionToken = "token-de-ingestion-de-prueba-con-32-chars"
	t.Setenv("INGESTION_API_TOKEN", ingestionToken)
	t.Setenv("INTERNAL_API_TOKEN", "")
	router := NewIngestionRouter(nil, "http://localhost:5173")

	req := httptest.NewRequest(http.MethodPost, "/ingestion/runs", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ingestionToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, quiero 422; body = %s", rec.Code, rec.Body.String())
	}
}
