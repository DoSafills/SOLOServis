package httpx

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// logLines decodifica cada línea JSON del buffer de logs.
func logLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var lines []map[string]any
	for _, raw := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if raw == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(raw), &entry); err != nil {
			t.Fatalf("línea de log no es JSON: %v (%s)", err, raw)
		}
		lines = append(lines, entry)
	}
	return lines
}

// chain arma la cadena estándar: RequestID → AccessLog → Recover → h.
func chain(logger *slog.Logger, h http.Handler) http.Handler {
	return RequestID(AccessLog(logger)(Recover(logger)(h)))
}

func TestParseLevel(t *testing.T) {
	tests := map[string]slog.Level{
		"": slog.LevelInfo, "info": slog.LevelInfo, "DEBUG": slog.LevelDebug,
		" warn ": slog.LevelWarn, "warning": slog.LevelWarn, "error": slog.LevelError, "otro": slog.LevelInfo,
	}
	for input, want := range tests {
		if got := ParseLevel(input); got != want {
			t.Errorf("ParseLevel(%q) = %v, quiero %v", input, got, want)
		}
	}
}

func TestAccessLog_CamposYNivel(t *testing.T) {
	tests := []struct {
		name      string
		handler   http.HandlerFunc
		wantCode  int
		wantLevel string
		wantBytes float64
	}{
		{"200 sin WriteHeader explícito", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("hola")) }, 200, "INFO", 4},
		{"handler que no escribe nada", func(http.ResponseWriter, *http.Request) {}, 200, "INFO", 0},
		{"204", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }, 204, "INFO", 0},
		{"404 es WARN", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "x", http.StatusNotFound) }, 404, "WARN", 2},
		{"503 es ERROR", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }, 503, "ERROR", 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			logger := NewLogger(&logs, slog.LevelInfo)
			req := httptest.NewRequest(http.MethodGet, "/products/abc?token=secreto123", nil)
			req.Header.Set("Authorization", "Bearer token-super-secreto")
			req.Header.Set(RequestIDHeader, "req-log-1")
			rec := httptest.NewRecorder()

			RequestID(AccessLog(logger)(tc.handler)).ServeHTTP(rec, req)

			if rec.Code != tc.wantCode {
				t.Fatalf("status de la respuesta alterado: %d, quiero %d", rec.Code, tc.wantCode)
			}
			lines := logLines(t, &logs)
			if len(lines) != 1 {
				t.Fatalf("quiero 1 línea de log, hay %d: %s", len(lines), logs.String())
			}
			entry := lines[0]
			checks := map[string]any{
				"level": tc.wantLevel, "msg": "http_request", "request_id": "req-log-1",
				"method": "GET", "path": "/products/abc", "status": float64(tc.wantCode), "bytes": tc.wantBytes,
			}
			for key, want := range checks {
				if entry[key] != want {
					t.Errorf("log[%s] = %v, quiero %v", key, entry[key], want)
				}
			}
			for _, key := range []string{"time", "duration_ms"} {
				if _, ok := entry[key]; !ok {
					t.Errorf("falta %s en el log", key)
				}
			}
			for _, leak := range []string{"secreto123", "token-super-secreto", "Bearer"} {
				if strings.Contains(logs.String(), leak) {
					t.Errorf("el log expone %q: %s", leak, logs.String())
				}
			}
		})
	}
}

func TestAccessLog_NoRegistraBody(t *testing.T) {
	var logs bytes.Buffer
	logger := NewLogger(&logs, slog.LevelInfo)
	req := httptest.NewRequest(http.MethodPost, "/products", strings.NewReader(`{"password":"hunter2"}`))
	handler := func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"sku":"respuesta-privada"}`)) }

	RequestID(AccessLog(logger)(http.HandlerFunc(handler))).ServeHTTP(httptest.NewRecorder(), req)

	for _, leak := range []string{"hunter2", "respuesta-privada"} {
		if strings.Contains(logs.String(), leak) {
			t.Fatalf("el log expone el body (%q): %s", leak, logs.String())
		}
	}
}

func TestRecover_PanicDevuelve500SinDetalles(t *testing.T) {
	var logs bytes.Buffer
	logger := NewLogger(&logs, slog.LevelInfo)
	const secret = "dial tcp postgres://postgres:postgres@db:5432 refused"
	req := httptest.NewRequest(http.MethodGet, "/stores/7", nil)
	rec := httptest.NewRecorder()

	chain(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		panic(secret)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, quiero 500", rec.Code)
	}
	body := rec.Body.String()
	for _, leak := range []string{"postgres", "dial tcp", "goroutine", ".go:"} {
		if strings.Contains(body, leak) {
			t.Fatalf("la respuesta expone %q: %s", leak, body)
		}
	}
	p := decodeProblem(t, rec)
	if p.Code != CodeInternalError || p.Status != 500 || p.RequestID != rec.Header().Get(RequestIDHeader) {
		t.Fatalf("problem = %+v", p)
	}

	lines := logLines(t, &logs)
	if len(lines) != 2 {
		t.Fatalf("quiero 2 líneas (panic + acceso), hay %d: %s", len(lines), logs.String())
	}
	panicLog, accessLog := lines[0], lines[1]
	if panicLog["msg"] != "panic_recovered" || panicLog["panic"] != secret || panicLog["request_id"] != p.RequestID {
		t.Fatalf("log de panic = %v", panicLog)
	}
	if stack, _ := panicLog["stack"].(string); !strings.Contains(stack, "goroutine") {
		t.Fatalf("el log no incluye el stack: %v", panicLog["stack"])
	}
	if accessLog["msg"] != "http_request" || accessLog["status"] != float64(500) || accessLog["level"] != "ERROR" {
		t.Fatalf("log de acceso = %v", accessLog)
	}
}

func TestRecover_PanicTrasEscribirNoModificaRespuesta(t *testing.T) {
	var logs bytes.Buffer
	logger := NewLogger(&logs, slog.LevelInfo)
	rec := httptest.NewRecorder()

	chain(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("parcial"))
		panic("después de escribir")
	})).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK || rec.Body.String() != "parcial" {
		t.Fatalf("respuesta alterada: %d %q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(logs.String(), "panic_recovered") {
		t.Fatalf("el panic no se registró: %s", logs.String())
	}
}

func TestRecover_RelanzaErrAbortHandler(t *testing.T) {
	logger := NewLogger(&bytes.Buffer{}, slog.LevelInfo)
	defer func() {
		if value := recover(); value != http.ErrAbortHandler {
			t.Fatalf("recover = %v, quiero http.ErrAbortHandler", value)
		}
	}()

	Recover(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestRecover_SinPanicNoInterfiere(t *testing.T) {
	var logs bytes.Buffer
	logger := NewLogger(&logs, slog.LevelInfo)
	rec := httptest.NewRecorder()

	chain(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid product id\n"))
	})).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/products/x", nil))

	if rec.Code != http.StatusBadRequest || rec.Body.String() != "invalid product id\n" ||
		rec.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatalf("respuesta alterada: %d %q %q", rec.Code, rec.Body.String(), rec.Header().Get("Content-Type"))
	}
	if strings.Contains(logs.String(), "panic_recovered") {
		t.Fatalf("log de panic inesperado: %s", logs.String())
	}
}

func TestResponseRecorder_Flush(t *testing.T) {
	rec := httptest.NewRecorder()
	wrapped := wrapResponseWriter(rec)
	wrapped.Flush()
	if !rec.Flushed || wrapped.Status() != http.StatusOK {
		t.Fatalf("flushed=%v status=%d", rec.Flushed, wrapped.Status())
	}
	if http.NewResponseController(wrapped).Flush() != nil {
		t.Fatal("ResponseController no pudo hacer Flush a través de Unwrap")
	}
}
