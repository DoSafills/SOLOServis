package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDoJSON_UsesGatewayAndServiceToken(t *testing.T) {
	t.Setenv("PRODUCT_API_URL", "")
	t.Setenv(GatewayURLEnv, "")
	t.Setenv(internalToken, "internal-test-token")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/products" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer internal-test-token" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}
		var input map[string]string
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if input["name"] != "Lavadora" {
			t.Errorf("request body = %v", input)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"product-1"}`))
	}))
	defer server.Close()
	t.Setenv(GatewayURLEnv, server.URL)

	var output struct {
		ID string `json:"id"`
	}
	client := NewClient(time.Second)
	err := client.DoJSON(context.Background(), "PRODUCT_API_URL", "http://localhost:8081",
		http.MethodPost, "/products", map[string]string{"name": "Lavadora"}, &output)
	if err != nil {
		t.Fatal(err)
	}
	if output.ID != "product-1" {
		t.Fatalf("output = %+v", output)
	}
}

func TestDoJSON_ServiceURLOverridesGateway(t *testing.T) {
	t.Setenv(GatewayURLEnv, "")
	t.Setenv("INTERNAL_API_TOKEN", "")

	gatewayCalled := make(chan struct{}, 1)
	gateway := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		gatewayCalled <- struct{}{}
	}))
	defer gateway.Close()
	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stores/7" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer service.Close()

	t.Setenv(GatewayURLEnv, gateway.URL)
	t.Setenv("STORE_API_URL", service.URL)
	if err := NewClient(time.Second).DoJSON(context.Background(), "STORE_API_URL", "http://localhost:8082",
		http.MethodGet, "/stores/7", nil, nil); err != nil {
		t.Fatal(err)
	}
	select {
	case <-gatewayCalled:
		t.Fatal("gateway recibió una llamada; se esperaba la URL específica del servicio")
	default:
	}
}

func TestDoJSON_ReturnsErrorForNonSuccessStatus(t *testing.T) {
	t.Setenv("STORE_API_URL", "")
	t.Setenv(GatewayURLEnv, "")
	t.Setenv("INTERNAL_API_TOKEN", "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()
	t.Setenv(GatewayURLEnv, server.URL)

	err := NewClient(time.Second).DoJSON(context.Background(), "STORE_API_URL", "http://localhost:8082",
		http.MethodGet, "/stores/7", nil, nil)
	if err == nil || err.Error() != "service returned HTTP 502" {
		t.Fatalf("err = %v; se esperaba error HTTP 502", err)
	}
}
