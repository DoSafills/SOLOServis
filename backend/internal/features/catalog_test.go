package features

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Los tests de caracterización de Pricing API (CreateOffer, UpdateOffer,
// historial) se movieron a internal/pricing en TAL-74, actualizados al
// comportamiento corregido. Aquí quedan los de Ingestion API.

func characterizationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("create test database pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	return pool
}

func characterizationIDs(t *testing.T, pool *pgxpool.Pool) (string, int32, int32) {
	t.Helper()
	var productID string
	var categoryID, storeID int32
	err := pool.QueryRow(context.Background(), `
		SELECT p.public_id::text, p.category_id, s.id
		FROM product p
		CROSS JOIN store s
		WHERE NOT EXISTS (
			SELECT 1 FROM product_offer o
			WHERE o.product_id = p.id AND o.store_id = s.id
		)
		ORDER BY p.id, s.id
		LIMIT 1`).Scan(&productID, &categoryID, &storeID)
	if err != nil {
		t.Fatalf("select unused demo product/store pair: %v", err)
	}
	return productID, categoryID, storeID
}

func characterizationRequest(t *testing.T, handler http.Handler, method, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("encode request: %v", err)
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestCharacterization_IngestOfferUnknownStore(t *testing.T) {
	storeAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/stores/2147483647" {
			t.Errorf("Store API request = %s %s, want GET /stores/2147483647", r.Method, r.URL.Path)
		}
		http.Error(w, "store not found", http.StatusNotFound)
	}))
	defer storeAPI.Close()
	t.Setenv("STORE_API_URL", storeAPI.URL)

	handler := &Handler{}
	response := characterizationRequest(t, http.HandlerFunc(handler.IngestOffer), http.MethodPost, "/ingestion/offers", map[string]any{
		"productId": "not-used-after-store-validation",
		"storeId":   int64(2147483647),
		"price":     10,
	})
	if response.Code != http.StatusBadGateway {
		t.Fatalf("POST /ingestion/offers status = %d, want 502; body: %s", response.Code, response.Body.String())
	}
}

func TestCharacterization_IngestProductWithoutImage(t *testing.T) {
	pool := characterizationPool(t)
	_, categoryID, _ := characterizationIDs(t, pool)
	handler := NewRouter("ingestion-api", pool, "")
	sku := fmt.Sprintf("characterization-%d", time.Now().UnixNano())
	response := characterizationRequest(t, handler, http.MethodPost, "/ingestion/products", map[string]any{
		"categoryId": categoryID,
		"name":       "Characterization test product",
		"sku":        sku,
	})
	if response.Code != http.StatusCreated {
		t.Fatalf("POST /ingestion/products status = %d, want 201; body: %s", response.Code, response.Body.String())
	}
	var result struct {
		ProductID string `json:"productId"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode ingested product: %v", err)
	}
	if result.ProductID == "" {
		t.Fatal("ingested product response has no productId")
	}

	var productID int32
	if err := pool.QueryRow(context.Background(),
		`SELECT id FROM product WHERE public_id = $1::uuid`, result.ProductID).Scan(&productID); err != nil {
		t.Fatalf("find ingested product: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM product WHERE id = $1`, productID); err != nil {
			t.Errorf("delete test product %d: %v", productID, err)
		}
	})
	var imageCount int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM product_image WHERE product_id = $1`, productID).Scan(&imageCount); err != nil {
		t.Fatalf("count product images: %v", err)
	}
	if imageCount != 0 {
		t.Fatalf("product image count = %d, want 0", imageCount)
	}
}

func TestCharacterization_IngestProductWithImageDelegatesToProductAPI(t *testing.T) {
	var receivedMethod, receivedPath string
	productAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod, receivedPath = r.Method, r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"productId":"fake-product"}`))
	}))
	defer productAPI.Close()
	t.Setenv("PRODUCT_API_URL", productAPI.URL)

	handler := &Handler{}
	response := characterizationRequest(t, http.HandlerFunc(handler.IngestProduct), http.MethodPost, "/ingestion/products", map[string]any{
		"categoryId": 1,
		"name":       "Product API characterization",
		"imageUrl":   "https://example.test/image.png",
	})
	if response.Code != http.StatusCreated {
		t.Fatalf("POST /ingestion/products status = %d, want 201; body: %s", response.Code, response.Body.String())
	}
	if receivedMethod != http.MethodPost || receivedPath != "/products" {
		t.Fatalf("Product API request = %s %s, want POST /products", receivedMethod, receivedPath)
	}
}
