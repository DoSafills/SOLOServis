package allapis

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
)

// Rutas inexistentes y métodos no permitidos dentro de una API, pedidos al
// proceso de cada dominio (TAL-245). Son los casos que se vieron en el
// clúster respondiendo text/plain o 405 sin cuerpo.
func TestNewDomainRouter_ErroresDeRutaSonProblemJSON(t *testing.T) {
	tests := []struct {
		domain, method, path string
		status               int
		code, detail, allow  string
	}{
		{"usuarios", http.MethodGet, "/auth", http.StatusNotFound, httpx.CodeNotFound, httpx.RouteNotFoundDetail, ""},
		{"usuarios", http.MethodGet, "/users/1/x", http.StatusNotFound, httpx.CodeNotFound, httpx.RouteNotFoundDetail, ""},
		{"usuarios", http.MethodGet, "/auth/login", http.StatusMethodNotAllowed, httpx.CodeMethodNotAllowed, httpx.MethodNotAllowedDetail, "POST"},
		{"usuarios", http.MethodDelete, "/users", http.StatusMethodNotAllowed, httpx.CodeMethodNotAllowed, httpx.MethodNotAllowedDetail, "GET"},
		{"comercial", http.MethodGet, "/search", http.StatusNotFound, httpx.CodeNotFound, httpx.RouteNotFoundDetail, ""},
		{"comercial", http.MethodGet, "/products/x/y/z", http.StatusNotFound, httpx.CodeNotFound, httpx.RouteNotFoundDetail, ""},
		{"comercial", http.MethodGet, "/reviews", http.StatusNotFound, httpx.CodeNotFound, httpx.RouteNotFoundDetail, ""},
		{"comercial", http.MethodGet, "/offers", http.StatusMethodNotAllowed, httpx.CodeMethodNotAllowed, httpx.MethodNotAllowedDetail, "POST"},
		{"comercial", http.MethodDelete, "/search/products", http.StatusMethodNotAllowed, httpx.CodeMethodNotAllowed, httpx.MethodNotAllowedDetail, "GET"},
		{"comercial", http.MethodPatch, "/cart/items/1", http.StatusMethodNotAllowed, httpx.CodeMethodNotAllowed, httpx.MethodNotAllowedDetail, "PUT, DELETE"},
		{"ingestion", http.MethodGet, "/ingestion", http.StatusNotFound, httpx.CodeNotFound, httpx.RouteNotFoundDetail, ""},
		{"ingestion", http.MethodDelete, "/ingestion/runs/1", http.StatusMethodNotAllowed, httpx.CodeMethodNotAllowed, httpx.MethodNotAllowedDetail, "GET"},
	}

	routers := map[string]http.Handler{}
	for domain, apis := range Domains {
		routers[domain] = NewDomainRouter(apis)(nil, "http://localhost:8443")
	}
	for _, tc := range tests {
		t.Run(tc.domain+" "+tc.method+" "+tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			routers[tc.domain].ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))

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
			if p.Code != tc.code || p.Detail != tc.detail || !httpx.ValidRequestID(p.RequestID) {
				t.Fatalf("problem = %+v, quiero code %q, detail %q y request_id", p, tc.code, tc.detail)
			}
			if got := rec.Header().Get("Allow"); got != tc.allow {
				t.Fatalf("Allow = %q, quiero %q", got, tc.allow)
			}
		})
	}
}

// La raíz de un prefijo sin ruta propia responde 404 en todos los procesos,
// pero el del dueño de la API debe distinguirse de los demás: scripts/testing/
// salud.py compara status, Content-Type y cuerpo sin request_id para saber a
// qué proceso llegó el gateway.
func TestNewDomainRouter_El404DelDuenoSeDistingue(t *testing.T) {
	for _, path := range []string{"/auth", "/search", "/comparisons", "/reviews", "/ingestion"} {
		t.Run(path, func(t *testing.T) {
			owner := domainOf(APIFor(path))
			if owner == "" {
				t.Fatalf("ningún dominio sirve %s", path)
			}
			fingerprints := map[string]string{}
			for domain, apis := range Domains {
				rec := httptest.NewRecorder()
				NewDomainRouter(apis)(nil, "http://localhost:8443").ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
				fingerprints[domain] = fingerprint(t, rec)
			}
			for domain, fp := range fingerprints {
				if domain != owner && fp == fingerprints[owner] {
					t.Errorf("%s responde igual que %s, el dueño: %s", domain, owner, fp)
				}
			}
		})
	}
}

// domainOf devuelve el dominio que sirve api, o "" si ninguno. Recibe el
// nombre de la API.
func domainOf(api string) string {
	for domain, apis := range Domains {
		if slices.Contains(apis, api) {
			return domain
		}
	}
	return ""
}

// fingerprint resume una respuesta como lo hace salud.py: status,
// Content-Type y cuerpo sin request_id. Recibe el test y la respuesta grabada;
// devuelve el resumen y falla el test si el cuerpo no es JSON.
func fingerprint(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body no es JSON: %v (%s)", err, rec.Body.String())
	}
	delete(body, "request_id")
	normalized, _ := json.Marshal(body)
	return fmt.Sprintf("%d %s %s", rec.Code, rec.Header().Get("Content-Type"), normalized)
}
