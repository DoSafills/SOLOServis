package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestNewCORS_OrigenConfigurado comprueba que con FRONTEND_URL definida se
// respondan las cabeceras CORS completas y que el request siga al handler.
// Recibe el *testing.T y no devuelve nada.
func TestNewCORS_OrigenConfigurado(t *testing.T) {
	called := false
	handler := NewCORS("http://localhost:8443/")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/products", nil))

	if !called || rec.Code != http.StatusOK {
		t.Fatalf("handler llamado = %v, status = %d", called, rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:8443" {
		t.Errorf("Allow-Origin = %q, quiero el origen sin la barra final", got)
	}
	if got := rec.Header().Get("Access-Control-Expose-Headers"); got != "X-Total-Count" {
		t.Errorf("Expose-Headers = %q, quiero X-Total-Count", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != corsAllowedMethods {
		t.Errorf("Allow-Methods = %q, quiero %q", got, corsAllowedMethods)
	}
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Errorf("Vary = %q, quiero Origin", got)
	}
}

// TestNewCORS_SinOrigen comprueba que en modo mismo origen (FRONTEND_URL
// vacía, como en Kubernetes) no se mande Allow-Origin vacío, que es inválido.
// Recibe el *testing.T y no devuelve nada.
func TestNewCORS_SinOrigen(t *testing.T) {
	handler := NewCORS("")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/products", nil))

	if _, ok := rec.Header()["Access-Control-Allow-Origin"]; ok {
		t.Fatalf("Allow-Origin no debería venir: %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, quiero 200", rec.Code)
	}
}

// TestNewCORS_Preflight comprueba que OPTIONS responda 204 sin llegar al
// handler, que es lo que espera el navegador antes de un POST o PUT.
// Recibe el *testing.T y no devuelve nada.
func TestNewCORS_Preflight(t *testing.T) {
	called := false
	handler := NewCORS("http://localhost:8443")(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "/cart/items", nil))

	if called {
		t.Error("el handler no debería ejecutarse en un preflight")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, quiero 204", rec.Code)
	}
}
