package http

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TAL-70 de punta a punta: router real de Product API contra PostgreSQL con
// todas las migraciones. Se omite sin TEST_DATABASE_URL.
func TestIntegration_ProductAPIEscrituraConAtributos(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL no definida: se omite el test de integración")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	const serviceToken = "token-interno-de-integracion-con-32-chars"
	t.Setenv("INTERNAL_API_TOKEN", serviceToken)
	router := NewProductRouter(pool, "http://localhost:5173")

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	adminToken := createSession(t, pool, "admin", "admin-"+suffix)
	clientToken := createSession(t, pool, "client", "client-"+suffix)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM product WHERE sku LIKE $1`, "e2e-%-"+suffix)
	})

	do := func(method, path, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	expect := func(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
		t.Helper()
		if rec.Code != status {
			t.Fatalf("status = %d, quiero %d (%s)", rec.Code, status, rec.Body.String())
		}
		if code != "" {
			var p httpx.Problem
			if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || p.Code != code {
				t.Fatalf("problem = %+v (%v), quiero %s", p, err, code)
			}
		}
	}

	create := fmt.Sprintf(`{
		"categorySlug": "refrigeradores",
		"name": "Refrigerador E2E",
		"sku": "e2e-admin-%s",
		"imageUrl": "https://example.test/e2e.png",
		"attributes": {"capacidad_litros": 300, "no_frost": true}
	}`, suffix)

	t.Run("sin token 401, cliente 403", func(t *testing.T) {
		expect(t, do(http.MethodPost, "/products", "", create), http.StatusUnauthorized, httpx.CodeUnauthorized)
		expect(t, do(http.MethodPost, "/products", clientToken, create), http.StatusForbidden, httpx.CodeForbidden)
	})

	var location string
	t.Run("admin crea con atributos", func(t *testing.T) {
		rec := do(http.MethodPost, "/products", adminToken, create)
		expect(t, rec, http.StatusCreated, "")
		location = rec.Header().Get("Location")
		var body struct {
			ID           string                     `json:"id"`
			CategorySlug string                     `json:"categorySlug"`
			Attributes   map[string]json.RawMessage `json:"attributes"`
			Images       []json.RawMessage          `json:"images"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if location != "/products/"+body.ID || body.CategorySlug != "refrigeradores" || len(body.Images) != 1 ||
			string(body.Attributes["capacidad_litros"]) != "300" || string(body.Attributes["no_frost"]) != "true" {
			t.Fatalf("Location = %q body = %s", location, rec.Body.String())
		}
	})

	t.Run("sku duplicado 409", func(t *testing.T) {
		expect(t, do(http.MethodPost, "/products", adminToken, create), http.StatusConflict, httpx.CodeConflict)
	})

	t.Run("GET devuelve lo mismo que el POST", func(t *testing.T) {
		rec := do(http.MethodGet, location, "", "")
		expect(t, rec, http.StatusOK, "")
		if !strings.Contains(rec.Body.String(), `"attributes":{"capacidad_litros":300,"no_frost":true}`) {
			t.Fatalf("body = %s", rec.Body.String())
		}
	})

	t.Run("atributos inválidos 422", func(t *testing.T) {
		rec := do(http.MethodPut, location, adminToken,
			`{"categorySlug":"refrigeradores","name":"x","attributes":{"capacidad_litros":"300","color":5}}`)
		expect(t, rec, http.StatusUnprocessableEntity, httpx.CodeValidationFailed)
		if !strings.Contains(rec.Body.String(), `"field":"attributes.capacidad_litros","code":"invalid_type"`) ||
			!strings.Contains(rec.Body.String(), `"field":"attributes.color","code":"invalid_type"`) {
			t.Fatalf("body = %s", rec.Body.String())
		}
	})

	t.Run("admin edita y reemplaza atributos", func(t *testing.T) {
		rec := do(http.MethodPut, location, adminToken,
			`{"categorySlug":"refrigeradores","name":"Refrigerador E2E v2","attributes":{"capacidad_litros":450}}`)
		expect(t, rec, http.StatusOK, "")
		if !strings.Contains(rec.Body.String(), `"attributes":{"capacidad_litros":450}`) ||
			!strings.Contains(rec.Body.String(), `"name":"Refrigerador E2E v2"`) {
			t.Fatalf("body = %s", rec.Body.String())
		}
	})

	t.Run("token de servicio con categoryId antiguo (Ingestion)", func(t *testing.T) {
		var lavadoID int32
		if err := pool.QueryRow(ctx, `SELECT id FROM product_category WHERE name = 'Lavado'`).Scan(&lavadoID); err != nil {
			t.Fatal(err)
		}
		rec := do(http.MethodPost, "/products", serviceToken, fmt.Sprintf(
			`{"categoryId": %d, "name": "Lavadora E2E", "sku": "e2e-ingestion-%s", "imageUrl": "https://example.test/l.png"}`,
			lavadoID, suffix))
		expect(t, rec, http.StatusCreated, "")
		if !strings.Contains(rec.Body.String(), `"categorySlug":null`) || !strings.Contains(rec.Body.String(), `"attributes":{}`) {
			t.Fatalf("body = %s", rec.Body.String())
		}
	})

	t.Run("listado por slug con X-Total-Count", func(t *testing.T) {
		rec := do(http.MethodGet, "/products?category=refrigeradores&limit=100", "", "")
		expect(t, rec, http.StatusOK, "")
		var items []map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
			t.Fatalf("el listado debe ser un arreglo: %v", err)
		}
		if rec.Header().Get("X-Total-Count") != fmt.Sprint(len(items)) || !strings.Contains(rec.Body.String(), "Refrigerador E2E v2") {
			t.Fatalf("X-Total-Count = %s, items = %d", rec.Header().Get("X-Total-Count"), len(items))
		}
		// Compatibilidad: Search API y el frontend leen estas claves.
		for _, key := range []string{"id", "name", "brand", "model", "categoryId", "category", "subcategory",
			"description", "rating", "reviewCount", "specs", "images", "offers", "categorySlug", "attributes"} {
			if _, ok := items[0][key]; !ok {
				t.Fatalf("falta la clave %q en el listado: %v", key, items[0])
			}
		}
	})

	t.Run("admin da de baja 204 y luego 404", func(t *testing.T) {
		rec := do(http.MethodDelete, location, adminToken, "")
		expect(t, rec, http.StatusNoContent, "")
		expect(t, do(http.MethodGet, location, "", ""), http.StatusNotFound, httpx.CodeNotFound)
		expect(t, do(http.MethodDelete, location, adminToken, ""), http.StatusNotFound, httpx.CodeNotFound)
	})
}

// createSession crea un usuario con rol y una sesión vigente; devuelve el
// access token en claro (Auth API guarda solo su SHA-256).
func createSession(t *testing.T, pool *pgxpool.Pool, role, name string) string {
	t.Helper()
	ctx := context.Background()
	token := "tok-" + name
	access := sha256.Sum256([]byte(token))
	refresh := sha256.Sum256([]byte("refresh-" + token))

	var userID int32
	if err := pool.QueryRow(ctx, `
		INSERT INTO user_account (role, name, email, password_hash)
		VALUES ($1::user_role, $2, $3, 'no-se-usa') RETURNING id`,
		role, name, name+"@example.test").Scan(&userID); err != nil {
		t.Fatalf("crear usuario %s: %v", role, err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_account WHERE id = $1`, userID)
	})
	if _, err := pool.Exec(ctx, `
		INSERT INTO user_session (user_id, access_token_hash, refresh_token_hash, access_expires_at, refresh_expires_at)
		VALUES ($1, $2, $3, NOW() + interval '1 hour', NOW() + interval '1 day')`,
		userID, access[:], refresh[:]); err != nil {
		t.Fatalf("crear sesión: %v", err)
	}
	return token
}
