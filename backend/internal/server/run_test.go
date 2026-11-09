package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewHTTPServerConfiguresExecutionBounds(t *testing.T) {
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	server := NewHTTPServer(":8080", handler)

	if server.ReadHeaderTimeout != 5*time.Second ||
		server.ReadTimeout != 15*time.Second ||
		server.WriteTimeout != 35*time.Second ||
		server.IdleTimeout != 60*time.Second {
		t.Fatalf("unexpected server timeouts: header=%s read=%s write=%s idle=%s",
			server.ReadHeaderTimeout, server.ReadTimeout, server.WriteTimeout, server.IdleTimeout)
	}
	if server.MaxHeaderBytes != 1<<20 {
		t.Fatalf("MaxHeaderBytes = %d, want 1 MiB", server.MaxHeaderBytes)
	}

	recorder := httptest.NewRecorder()
	server.Handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("configured server handler returned %d", recorder.Code)
	}
}
