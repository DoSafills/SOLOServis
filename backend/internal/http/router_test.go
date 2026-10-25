package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
		{"store escritura no existe", NewStoreRouter(nil, frontendURL), http.MethodPost, "/stores", `{}`, "application/json", 405, "", ""},
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
		"GET /categories/{slug}",
		"GET /health",
		"GET /product-categories",
		"GET /products",
		"GET /products/{publicID}",
		"GET /products/{publicID}/reviews",
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
