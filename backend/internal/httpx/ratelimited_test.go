package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestWriteRateLimited fija la forma del 429 (TAL-201): problem+json con
// code rate_limited, request_id y Retry-After en segundos enteros
// redondeados hacia arriba, nunca menor que 1.
func TestWriteRateLimited(t *testing.T) {
	cases := []struct {
		name       string
		retryAfter time.Duration
		want       string
	}{
		{name: "segundos exactos", retryAfter: 900 * time.Second, want: "900"},
		{name: "fracción redondea hacia arriba", retryAfter: 1500 * time.Millisecond, want: "2"},
		{name: "cero queda en 1", retryAfter: 0, want: "1"},
		{name: "negativo queda en 1", retryAfter: -time.Second, want: "1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
			RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				WriteRateLimited(w, r, tc.retryAfter, "Demasiados intentos.")
			})).ServeHTTP(rec, req)

			if rec.Code != http.StatusTooManyRequests {
				t.Fatalf("status = %d, quiero 429", rec.Code)
			}
			if got := rec.Header().Get("Retry-After"); got != tc.want {
				t.Fatalf("Retry-After = %q, quiero %q", got, tc.want)
			}
			if got := rec.Header().Get("Content-Type"); got != ProblemContentType {
				t.Fatalf("Content-Type = %q, quiero %q", got, ProblemContentType)
			}
			var p Problem
			if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
				t.Fatalf("body no es un Problem: %v", err)
			}
			if p.Code != CodeRateLimited || p.Status != http.StatusTooManyRequests || p.RequestID == "" {
				t.Fatalf("problem = %+v", p)
			}
		})
	}
}
