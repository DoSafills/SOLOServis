package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DoSafills/SOLOServis/backend/internal/products"
	"github.com/DoSafills/SOLOServis/backend/internal/services"
	"github.com/DoSafills/SOLOServis/backend/internal/stores"
)

type fakePinger struct {
	err error
}

func (f fakePinger) Ping(_ context.Context) error {
	return f.err
}

type fakeProducts struct {
	products.Repository
}
type fakeServices struct {
	services.Repository
}
type fakeStores struct {
	stores.Repository
}

func TestHealthEndpoint(t *testing.T) {
	tests := []struct {
		name               string
		pinger             Pinger
		wantStatus         int
		wantStatusField    string
		wantDatabaseField  string
	}{
		{
			name:              "healthy",
			pinger:            fakePinger{err: nil},
			wantStatus:        http.StatusOK,
			wantStatusField:   "ok",
			wantDatabaseField: "connected",
		},
		{
			name:              "unhealthy",
			pinger:            fakePinger{err: context.DeadlineExceeded},
			wantStatus:        http.StatusServiceUnavailable,
			wantStatusField:   "error",
			wantDatabaseField: "unavailable",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, mux := NewRouterWithAPI(tc.pinger, Repositories{
				Products: fakeProducts{},
				Services: fakeServices{},
				Stores:   fakeStores{},
			}, "http://localhost:3000")

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			mux.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("status: want %d got %d", tc.wantStatus, w.Code)
			}

			var resp HealthResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode: %v", err)
			}

			if resp.Status != tc.wantStatusField {
				t.Fatalf("body.status: want %q got %q", tc.wantStatusField, resp.Status)
			}
			if resp.Service != ServiceName {
				t.Fatalf("body.service: want %q got %q", ServiceName, resp.Service)
			}
			if resp.Database != tc.wantDatabaseField {
				t.Fatalf("body.database: want %q got %q", tc.wantDatabaseField, resp.Database)
			}
		})
	}
}

func TestOpenAPISpec(t *testing.T) {
	_, mux := NewRouterWithAPI(fakePinger{}, Repositories{
		Products: fakeProducts{},
		Services: fakeServices{},
		Stores:   fakeStores{},
	}, "http://localhost:3000")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("openapi.json: want 200 got %d", w.Code)
	}

	var doc map[string]any
	if err := json.NewDecoder(w.Body).Decode(&doc); err != nil {
		t.Fatalf("decode openapi: %v", err)
	}

	info, _ := doc["info"].(map[string]any)
	if info["title"] != APITitle {
		t.Errorf("info.title: want %q got %q", APITitle, info["title"])
	}
	if info["version"] != APIVersion {
		t.Errorf("info.version: want %q got %q", APIVersion, info["version"])
	}

	paths, _ := doc["paths"].(map[string]any)
	expectedPaths := []string{
		"/health",
		"/products",
		"/products/{publicID}",
		"/services",
		"/services/{publicID}",
		"/stores",
	}
	for _, p := range expectedPaths {
		if _, ok := paths[p]; !ok {
			t.Errorf("missing path %q in spec", p)
		}
	}
	// The docs routes (/openapi.json, /openapi.yaml, /docs) are added lazily by huma
	// when the spec is built, so they may not appear in the paths map here.

	healthPath, _ := paths["/health"].(map[string]any)
	healthGet, _ := healthPath["get"].(map[string]any)
	responses, _ := healthGet["responses"].(map[string]any)
	if _, ok := responses["200"]; !ok {
		t.Errorf("health.get.responses missing 200")
	}
	if _, ok := responses["503"]; !ok {
		t.Errorf("health.get.responses missing 503")
	}

	components, _ := doc["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	if _, ok := schemas["ProductOffer"]; !ok {
		t.Errorf("missing ProductOffer schema")
	}
	if _, ok := schemas["ServiceOffer"]; !ok {
		t.Errorf("missing ServiceOffer schema")
	}
	if _, ok := schemas["HealthResponse"]; !ok {
		t.Errorf("missing HealthResponse schema")
	}
}

func TestCORSHeaders(t *testing.T) {
	_, mux := NewRouterWithAPI(fakePinger{}, Repositories{
		Products: fakeProducts{},
		Services: fakeServices{},
		Stores:   fakeStores{},
	}, "https://app.example.com")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/products", nil)
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("preflight: want 204 got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "https://app.example.com" {
		t.Errorf("ACAO: want https://app.example.com got %q", w.Header().Get("Access-Control-Allow-Origin"))
	}
	if w.Header().Get("Vary") != "Origin" {
		t.Errorf("Vary: want Origin got %q", w.Header().Get("Vary"))
	}
}