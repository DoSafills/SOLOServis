// Package integration prueba las APIs contra PostgreSQL tal como corren en
// Kubernetes: un router por dominio (allapis.NewDomainRouter) y un pool por
// proceso (TAL-133).
package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DoSafills/SOLOServis/backend/internal/allapis"
	"github.com/jackc/pgx/v5/pgxpool"
)

const internalToken = "token-interno-de-integracion-con-32-chars"

// Con la base caída, cada API responde 503 sin entrar en pánico y sin mostrar
// el error interno. En particular, las rutas con sesión no responden 401: el
// frontend cerraría la sesión del usuario por una falla del servidor.
func TestBaseCaida_CadaAPIResponde503(t *testing.T) {
	t.Setenv("INTERNAL_API_TOKEN", internalToken)
	// Puerto cerrado: cada consulta falla al conectar (pgxpool conecta al usarse).
	pool, err := pgxpool.New(context.Background(), "postgres://soloservis:x@127.0.0.1:1/soloservis?connect_timeout=2")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	const product = "0976cfb5-d003-4918-ad55-e72e6df8b797"
	const session = "Bearer token-de-sesion-cualquiera"
	cases := []struct {
		domain, api, method, path, auth, body string
	}{
		{"comercial", "product", "GET", "/products", "", ""},
		{"comercial", "product", "GET", "/categories", "", ""},
		{"comercial", "store", "GET", "/stores", "", ""},
		{"comercial", "service", "GET", "/services", "", ""},
		{"comercial", "service", "GET", "/services/" + product, "", ""},
		{"comercial", "pricing", "GET", "/offers/product/" + product, "", ""},
		{"comercial", "search", "GET", "/search/products?q=a", "", ""},
		{"comercial", "comparison", "GET", "/comparisons/compatible/" + product, "", ""},
		{"comercial", "comparison", "GET", "/comparisons/products", session, ""},
		{"comercial", "favorites", "GET", "/favorites", session, ""},
		{"comercial", "cart", "GET", "/cart", session, ""},
		{"comercial", "review", "GET", "/reviews/product/" + product, "", ""},
		{"comercial", "watchlist", "GET", "/watchlist?userId=1", "", ""},
		{"comercial", "notification", "GET", "/notifications?userId=1", "", ""},
		{"comercial", "notification", "POST", "/notifications", "", `{"userId":1,"type":"t","title":"t","message":"m"}`},
		{"comercial", "/health", "GET", "/health", "", ""},
		{"usuarios", "auth", "POST", "/auth/login", "", `{"email":"ana@test.cl","password":"Clave12345!"}`},
		{"usuarios", "auth", "POST", "/auth/register", "", `{"name":"Ana","email":"ana@test.cl","password":"Clave12345!"}`},
		{"usuarios", "auth", "GET", "/auth/validate", session, ""},
		{"usuarios", "user", "GET", "/users/1", session, ""},
		{"usuarios", "/health", "GET", "/health", "", ""},
		{"ingestion", "ingestion", "GET", "/ingestion/runs", "Bearer " + internalToken, ""},
		{"ingestion", "ingestion", "GET", "/ingestion/runs", session, ""},
		{"ingestion", "/health", "GET", "/health", "", ""},
	}

	routers := map[string]http.Handler{}
	for domain, apis := range allapis.Domains {
		routers[domain] = allapis.NewDomainRouter(apis)(pool, "http://localhost:5173")
	}
	covered := map[string]bool{}
	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.path, strings.NewReader(c.body))
		if c.body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if c.auth != "" {
			req.Header.Set("Authorization", c.auth)
		}
		rec := httptest.NewRecorder()
		routers[c.domain].ServeHTTP(rec, req)

		body := rec.Body.String()
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s %s %s = %d %s, quiero 503", c.domain, c.method, c.path, rec.Code, body)
		}
		for _, leak := range []string{"127.0.0.1", "dial", "failed to connect", "soloservis:x"} {
			if strings.Contains(body, leak) {
				t.Errorf("%s %s filtra el error interno (%q): %s", c.method, c.path, leak, body)
			}
		}
		covered[c.api] = true
	}
	for _, apis := range allapis.Domains {
		for _, api := range apis {
			if !covered[api] {
				t.Errorf("la API %s no tiene caso con la base caída", api)
			}
		}
	}
}

// Las APIs del estándar TAL-78 responden problem+json con un código estable.
func TestBaseCaida_ProblemJSON(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), "postgres://x:x@127.0.0.1:1/x?connect_timeout=2")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	r := allapis.NewDomainRouter(allapis.Domains["comercial"])(pool, "http://localhost:5173")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/products", nil))
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q", ct)
	}
	for _, want := range []string{`"status":503`, `"code":"service_unavailable"`, `"request_id":"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("falta %s en %s", want, rec.Body.String())
		}
	}
}
