package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS_LocalFrontendOrigins(t *testing.T) {
	handler := CORS("http://localhost:8443")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name       string
		origin     string
		wantOrigin string
	}{
		{"localhost", "http://localhost:8443", "http://localhost:8443"},
		{"loopback", "http://127.0.0.1:8443", "http://127.0.0.1:8443"},
		{"private LAN address", "http://192.168.1.9:8443", "http://192.168.1.9:8443"},
		{"public address rejected", "http://203.0.113.9:8443", ""},
		{"wrong port rejected", "http://192.168.1.9:8080", ""},
		{"other origin rejected", "https://example.com", ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Origin", test.origin)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != test.wantOrigin {
				t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, test.wantOrigin)
			}
		})
	}
}

func TestCORS_DoesNotAllowPrivateOriginsForPublicFrontend(t *testing.T) {
	handler := CORS("https://soloservis.example.com")(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://192.168.1.9:8443")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
	}
}
