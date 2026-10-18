package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
)

// Los casos usan rutas que responden antes de consultar la base de datos,
// por eso los routers se construyen con un pool nil.
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
	}{
		{"product preflight CORS", NewProductRouter(nil, frontendURL), http.MethodOptions, "/products", "", "", 204, ""},
		{"product id inválido", NewProductRouter(nil, frontendURL), http.MethodGet, "/products/no-es-uuid", "", "", 400, "invalid product id\n"},
		{"product create con payload inválido", NewProductRouter(nil, frontendURL), http.MethodPost, "/products", `{"nombre":"x"}`, "application/json", 400, "invalid product payload\n"},
		{"product update con id inválido", NewProductRouter(nil, frontendURL), http.MethodPut, "/products/x", `{}`, "application/json", 400, "invalid product id\n"},
		{"product delete con id inválido", NewProductRouter(nil, frontendURL), http.MethodDelete, "/products/x", "", "", 400, "invalid product id\n"},
		{"store preflight CORS", NewStoreRouter(nil, frontendURL), http.MethodOptions, "/stores", "", "", 204, ""},
		{"store id inválido", NewStoreRouter(nil, frontendURL), http.MethodGet, "/stores/abc", "", "", 400, "invalid store id\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			rec := httptest.NewRecorder()

			tc.router.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus || rec.Body.String() != tc.wantBody {
				t.Fatalf("respuesta = %d %q, quiero %d %q", rec.Code, rec.Body.String(), tc.wantStatus, tc.wantBody)
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
