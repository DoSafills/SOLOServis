package chiroute

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
)

// newTestRouter arma un router como el de una API: RequestID, un Group, un
// parámetro de ruta y un subrouter montado con Mount (que necesita su propia
// llamada a UseProblemFallbacks). No recibe nada; devuelve el router.
func newTestRouter() *chi.Mux {
	ok := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }
	r := chi.NewRouter()
	UseProblemFallbacks(r)
	r.Use(httpx.RequestID)
	r.Get("/items", ok)
	r.Group(func(r chi.Router) {
		r.Post("/items", ok)
		r.Delete("/items/{id}", ok)
	})
	r.Get("/files/{name}", ok)

	sub := chi.NewRouter()
	UseProblemFallbacks(sub)
	sub.Get("/reports", ok)
	sub.Put("/reports", ok)
	r.Mount("/admin", sub)
	return r
}

func TestUseProblemFallbacks(t *testing.T) {
	tests := []struct {
		name, method, path string
		status             int
		code, detail       string
		allow              string
	}{
		{"ruta inexistente", http.MethodGet, "/nada", http.StatusNotFound, httpx.CodeNotFound, httpx.RouteNotFoundDetail, ""},
		{"más larga que una ruta", http.MethodGet, "/items/1/x", http.StatusNotFound, httpx.CodeNotFound, httpx.RouteNotFoundDetail, ""},
		{"inexistente bajo Mount", http.MethodGet, "/admin/nada", http.StatusNotFound, httpx.CodeNotFound, httpx.RouteNotFoundDetail, ""},
		{"método no permitido, rutas en Group", http.MethodPut, "/items", http.StatusMethodNotAllowed, httpx.CodeMethodNotAllowed, httpx.MethodNotAllowedDetail, "GET, POST"},
		{"método no permitido con parámetro", http.MethodGet, "/items/7", http.StatusMethodNotAllowed, httpx.CodeMethodNotAllowed, httpx.MethodNotAllowedDetail, "DELETE"},
		{"método no permitido bajo Mount", http.MethodDelete, "/admin/reports", http.StatusMethodNotAllowed, httpx.CodeMethodNotAllowed, httpx.MethodNotAllowedDetail, "GET, PUT"},
		// chi enruta con RawPath cuando existe: %2F no separa segmentos.
		{"método no permitido con RawPath", http.MethodPost, "/files/a%2Fb", http.StatusMethodNotAllowed, httpx.CodeMethodNotAllowed, httpx.MethodNotAllowedDetail, "GET"},
	}

	router := newTestRouter()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))

			if rec.Code != tc.status {
				t.Fatalf("status = %d, quiero %d (%s)", rec.Code, tc.status, rec.Body.String())
			}
			if got := rec.Header().Get("Content-Type"); got != httpx.ProblemContentType {
				t.Fatalf("Content-Type = %q, quiero %q", got, httpx.ProblemContentType)
			}
			var p httpx.Problem
			if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
				t.Fatalf("body no es un Problem: %v (%s)", err, rec.Body.String())
			}
			if p.Status != tc.status || p.Code != tc.code || p.Detail != tc.detail || p.Instance != requestPath(tc.path) {
				t.Fatalf("problem = %+v, quiero status %d, code %q, detail %q", p, tc.status, tc.code, tc.detail)
			}
			if !httpx.ValidRequestID(p.RequestID) || p.RequestID != rec.Header().Get(httpx.RequestIDHeader) {
				t.Fatalf("request_id = %q, X-Request-ID = %q", p.RequestID, rec.Header().Get(httpx.RequestIDHeader))
			}
			if got := rec.Header().Get("Allow"); got != tc.allow {
				t.Fatalf("Allow = %q, quiero %q", got, tc.allow)
			}
		})
	}
}

func TestAllowedMethods_RutaInexistenteDevuelveVacio(t *testing.T) {
	got := AllowedMethods(newTestRouter(), httptest.NewRequest(http.MethodGet, "/nada", nil))
	if len(got) != 0 {
		t.Fatalf("AllowedMethods = %v, quiero vacío", got)
	}
}

// requestPath devuelve la ruta decodificada de target, la que WriteProblem
// pone en instance. Recibe la ruta del request y devuelve URL.Path.
func requestPath(target string) string {
	return httptest.NewRequest(http.MethodGet, target, nil).URL.Path
}
