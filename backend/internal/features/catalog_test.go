package features

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

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

func characterizationOfferPayload(productID string, storeID int32, price float64) map[string]any {
	return map[string]any{
		"productId": productID,
		"storeId":   storeID,
		"price":     price,
	}
}

func createCharacterizationOffer(t *testing.T, pool *pgxpool.Pool, router http.Handler, payload map[string]any) int32 {
	t.Helper()
	response := characterizationRequest(t, router, http.MethodPost, "/offers", payload)
	if response.Code != http.StatusOK {
		t.Fatalf("POST /offers status = %d, want 200; body: %s", response.Code, response.Body.String())
	}
	var result struct {
		ID int32 `json:"id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode created offer: %v", err)
	}
	if result.ID <= 0 {
		t.Fatalf("created offer id = %d, want positive id", result.ID)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM product_offer WHERE id = $1`, result.ID); err != nil {
			t.Errorf("delete test offer %d: %v", result.ID, err)
		}
	})
	return result.ID
}

func TestCharacterization_CreateOffer(t *testing.T) {
	pool := characterizationPool(t)
	productID, _, storeID := characterizationIDs(t, pool)
	router := NewRouter("pricing-api", pool, "")

	// CreateOffer currently responds 200 and records the initial price history.
	offerID := createCharacterizationOffer(t, pool, router,
		characterizationOfferPayload(productID, storeID, 123.45))

	var historyCount int
	var historyPrice float64
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*), COALESCE(MAX(price), 0)
		FROM product_price_history WHERE product_offer_id = $1`, offerID).Scan(&historyCount, &historyPrice)
	if err != nil {
		t.Fatalf("query initial price history: %v", err)
	}
	if historyCount != 1 || historyPrice != 123.45 {
		t.Fatalf("initial history = (%d rows, price %v), want (1 row, 123.45)", historyCount, historyPrice)
	}
}

func TestCharacterization_CreateOfferDuplicate(t *testing.T) {
	pool := characterizationPool(t)
	productID, _, storeID := characterizationIDs(t, pool)
	router := NewRouter("pricing-api", pool, "")
	payload := characterizationOfferPayload(productID, storeID, 42.5)
	offerID := createCharacterizationOffer(t, pool, router, payload)

	response := characterizationRequest(t, router, http.MethodPost, "/offers", payload)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("duplicate POST /offers status = %d, want 400; body: %s", response.Code, response.Body.String())
	}

	var count int
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM product_offer o
		JOIN product p ON p.id = o.product_id
		WHERE p.public_id = $1::uuid AND o.store_id = $2`, productID, storeID).Scan(&count)
	if err != nil {
		t.Fatalf("count duplicate offers: %v", err)
	}
	if count != 1 {
		t.Fatalf("offer count after duplicate request = %d, want 1", count)
	}
	var historyCount int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM product_price_history WHERE product_offer_id = $1`, offerID).Scan(&historyCount); err != nil {
		t.Fatalf("count history after duplicate request: %v", err)
	}
	if historyCount != 1 {
		t.Fatalf("history count after duplicate request = %d, want 1", historyCount)
	}
}

func TestCharacterization_UpdateOfferPriceAddsHistory(t *testing.T) {
	pool := characterizationPool(t)
	productID, _, storeID := characterizationIDs(t, pool)
	router := NewRouter("pricing-api", pool, "")
	offerID := createCharacterizationOffer(t, pool, router,
		characterizationOfferPayload(productID, storeID, 123.45))

	response := characterizationRequest(t, router, http.MethodPut, "/offers/"+strconv.Itoa(int(offerID)), map[string]any{
		"price": 234.56,
	})
	if response.Code != http.StatusOK {
		t.Fatalf("PUT /offers/{id} status = %d, want 200; body: %s", response.Code, response.Body.String())
	}
	var result struct {
		Updated bool `json:"updated"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode updated offer: %v", err)
	}
	if !result.Updated {
		t.Fatalf("updated = false, want true")
	}
	var historyCount int
	var latestPrice float64
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*), COALESCE(MAX(price), 0)
		FROM product_price_history WHERE product_offer_id = $1`, offerID).Scan(&historyCount, &latestPrice)
	if err != nil {
		t.Fatalf("count updated price history: %v", err)
	}
	if historyCount != 2 {
		t.Fatalf("history count after price change = %d, want 2", historyCount)
	}
	if latestPrice != 234.56 {
		t.Fatalf("latest history price = %v, want 234.56", latestPrice)
	}
}

func TestCharacterization_UpdateOfferWithoutPriceChangeDoesNotAddHistory(t *testing.T) {
	pool := characterizationPool(t)
	productID, _, storeID := characterizationIDs(t, pool)
	router := NewRouter("pricing-api", pool, "")
	offerID := createCharacterizationOffer(t, pool, router,
		characterizationOfferPayload(productID, storeID, 123.45))

	response := characterizationRequest(t, router, http.MethodPut, "/offers/"+strconv.Itoa(int(offerID)), map[string]any{
		"price":      123.45,
		"stock":      4,
		"productUrl": "https://example.test/offer",
		"condition":  "used",
	})
	if response.Code != http.StatusOK {
		t.Fatalf("PUT /offers/{id} status = %d, want 200; body: %s", response.Code, response.Body.String())
	}
	var historyCount int
	var stock int32
	var productURL, condition string
	err := pool.QueryRow(context.Background(), `
		SELECT (SELECT COUNT(*) FROM product_price_history WHERE product_offer_id = $1),
		       stock, product_url, condition
		FROM product_offer WHERE id = $1`, offerID).Scan(&historyCount, &stock, &productURL, &condition)
	if err != nil {
		t.Fatalf("query offer after non-price update: %v", err)
	}
	if historyCount != 1 {
		t.Fatalf("history count without price change = %d, want 1", historyCount)
	}
	if stock != 4 || productURL != "https://example.test/offer" || condition != "used" {
		t.Fatalf("updated fields = (stock %d, URL %q, condition %q), want (4, example URL, used)", stock, productURL, condition)
	}
}

func TestCharacterization_CreateOfferRejectsInvalidConditionAndStock(t *testing.T) {
	pool := characterizationPool(t)
	productID, _, storeID := characterizationIDs(t, pool)
	router := NewRouter("pricing-api", pool, "")

	for _, test := range []struct {
		name    string
		changes map[string]any
	}{
		{name: "invalid condition", changes: map[string]any{"condition": "damaged"}},
		{name: "negative stock", changes: map[string]any{"stock": -1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			payload := characterizationOfferPayload(productID, storeID, 99)
			for key, value := range test.changes {
				payload[key] = value
			}
			response := characterizationRequest(t, router, http.MethodPost, "/offers", payload)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("POST /offers status = %d, want 400; body: %s", response.Code, response.Body.String())
			}
		})
	}

	var count int
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM product_offer o
		JOIN product p ON p.id = o.product_id
		WHERE p.public_id = $1::uuid AND o.store_id = $2`, productID, storeID).Scan(&count)
	if err != nil {
		t.Fatalf("count offers after invalid requests: %v", err)
	}
	if count != 0 {
		t.Fatalf("offer count after invalid requests = %d, want 0", count)
	}
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

func TestCharacterization_CreateOfferMissingPriceMeansZero(t *testing.T) {
	pool := characterizationPool(t)
	productID, _, storeID := characterizationIDs(t, pool)
	router := NewRouter("pricing-api", pool, "")
	offerID := createCharacterizationOffer(t, pool, router, map[string]any{
		"productId": productID,
		"storeId":   storeID,
	})

	var price float64
	if err := pool.QueryRow(context.Background(),
		`SELECT price FROM product_offer WHERE id = $1`, offerID).Scan(&price); err != nil {
		t.Fatalf("query offer with omitted price: %v", err)
	}
	if price != 0 {
		t.Fatalf("price for omitted field = %v, want 0", price)
	}
}

func TestCharacterization_CreateOfferCurrencyIsForcedToCLP(t *testing.T) {
	pool := characterizationPool(t)
	productID, _, storeID := characterizationIDs(t, pool)
	router := NewRouter("pricing-api", pool, "")
	offerID := createCharacterizationOffer(t, pool, router, map[string]any{
		"productId": productID,
		"storeId":   storeID,
		"price":     22.5,
		"currency":  "USD",
	})

	var currency string
	if err := pool.QueryRow(context.Background(),
		`SELECT currency FROM product_offer WHERE id = $1`, offerID).Scan(&currency); err != nil {
		t.Fatalf("query offer currency: %v", err)
	}
	if currency != "CLP" {
		t.Fatalf("stored currency = %q, want CLP", currency)
	}
}

func TestCharacterization_CreateOfferRejectsInvalidInputWithoutDatabase(t *testing.T) {
	handler := &Handler{}
	for _, test := range []struct {
		name    string
		payload map[string]any
	}{
		{
			name: "negative price",
			payload: map[string]any{
				"productId": "unused",
				"storeId":   1,
				"price":     -1,
			},
		},
		{
			name: "non-positive store id",
			payload: map[string]any{
				"productId": "unused",
				"storeId":   0,
				"price":     1,
			},
		},
		{
			name: "invalid currency format",
			payload: map[string]any{
				"productId": "unused",
				"storeId":   1,
				"price":     1,
				"currency":  "US1",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := characterizationRequest(t, http.HandlerFunc(handler.CreateOffer), http.MethodPost, "/offers", test.payload)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("POST /offers status = %d, want 400; body: %s", response.Code, response.Body.String())
			}
		})
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
