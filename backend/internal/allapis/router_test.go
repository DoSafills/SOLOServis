package allapis

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// Cada ruta de cada API debe llegar a esa misma API: si alguien agrega un
// prefijo nuevo sin registrarlo en Prefixes, este test falla.
func TestAPIFor_CadaRutaLlegaASuAPI(t *testing.T) {
	for api, router := range Routers(nil, "http://localhost:8443") {
		walk := func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
			if route == "/health" {
				return nil
			}
			if got := APIFor(route); got != api {
				t.Errorf("%s %s lo atiende %q, quiero %q", method, route, got, api)
			}
			return nil
		}
		if err := chi.Walk(router, walk); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAPIFor_SegmentoCompleto(t *testing.T) {
	cases := map[string]string{
		"/products":                     "product",
		"/products/abc":                 "product",
		"/product-categories":           "product",
		"/productsx":                    "",
		"/offers/product/abc/best":      "pricing",
		"/search/products":              "search",
		"/comparisons/services/1/items": "comparison",
		"/":                             "",
		"/nada":                         "",
	}
	for path, want := range cases {
		if got := APIFor(path); got != want {
			t.Errorf("APIFor(%q) = %q, quiero %q", path, got, want)
		}
	}
}

// Rutas que responden antes de consultar la base: comprueban que el request
// llega intacto (misma ruta, mismos middlewares) al router de la API.
func TestNewRouter_EntregaElRequestASuAPI(t *testing.T) {
	r := NewRouter(nil, "http://localhost:8443")
	cases := []struct {
		method, path string
		status       int
		code         string
	}{
		{http.MethodGet, "/categories/NO_VALIDO", http.StatusBadRequest, `"code":"invalid_parameter"`},
		{http.MethodGet, "/ingestion/runs", http.StatusUnauthorized, `"code":"unauthorized"`},
		{http.MethodGet, "/comparisons/products", http.StatusUnauthorized, `"code":"unauthorized"`},
		{http.MethodGet, "/no-existe", http.StatusNotFound, `"code":"not_found"`},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(c.method, c.path, nil))
		if rec.Code != c.status || !strings.Contains(rec.Body.String(), c.code) {
			t.Errorf("%s %s = %d %s, quiero %d %s", c.method, c.path, rec.Code, rec.Body.String(), c.status, c.code)
		}
		if rec.Header().Get("X-Request-Id") == "" && c.status != http.StatusNotFound {
			t.Errorf("%s %s sin X-Request-Id: no pasó por los middlewares de la API", c.method, c.path)
		}
	}
}

// El preflight CORS lo responde el middleware de la API dueña.
func TestNewRouter_PreflightCORS(t *testing.T) {
	r := NewRouter(nil, "http://localhost:8443")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "/cart/items", nil))
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:8443" {
		t.Fatalf("preflight = %d %v", rec.Code, rec.Header())
	}
}
