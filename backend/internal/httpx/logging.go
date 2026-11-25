package httpx

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// NewLogger crea un logger JSON estructurado.
func NewLogger(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
}

// ParseLevel interpreta LOG_LEVEL (debug, info, warn, error). Por defecto info.
func ParseLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// AccessLog registra una línea por request: request_id, método, path (sin
// query string), status, duración, bytes y, si la respuesta fue un
// problem+json, su `code` estable (TAL-250). Nunca registra bodies ni headers.
// Recibe el logger donde escribir y devuelve el middleware.
func AccessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := wrapResponseWriter(w)
			info := &accessInfo{}
			r = r.WithContext(context.WithValue(r.Context(), accessInfoKey{}, info))

			// defer: también registra requests abortados por panic.
			defer func() {
				status := rec.Status()
				attrs := []slog.Attr{
					slog.String("request_id", RequestIDFrom(r.Context())),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", status),
					slog.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000),
					slog.Int64("bytes", rec.bytes),
				}
				if info.code != "" {
					attrs = append(attrs, slog.String("code", info.code))
				}
				logger.LogAttrs(r.Context(), levelForStatus(status), "http_request", attrs...)
			}()

			next.ServeHTTP(rec, r)
		})
	}
}

// accessInfo guarda lo que el handler sabe de la respuesta y el AccessLog no
// puede deducir del status. Viaja por el contexto (y no en el ResponseWriter)
// para que un middleware que envuelva el writer no lo esconda.
type accessInfo struct {
	code string
}

type accessInfoKey struct{}

// recordProblemCode anota el `code` del problem+json para la línea de acceso.
// Recibe el contexto del request y el code; no hace nada si el request no pasó
// por AccessLog. Conserva el primero, igual que el status.
func recordProblemCode(ctx context.Context, code string) {
	if info, ok := ctx.Value(accessInfoKey{}).(*accessInfo); ok && info.code == "" {
		info.code = code
	}
}

// levelForStatus devuelve el nivel de log de la línea de acceso según el
// status: ERROR para 5xx, WARN para 4xx e INFO para el resto.
func levelForStatus(status int) slog.Level {
	switch {
	case status >= 500:
		return slog.LevelError
	case status >= 400:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}

// responseRecorder captura status y bytes escritos sin alterar la respuesta.
type responseRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int64
	wroteHeader bool
}

// wrapResponseWriter reutiliza el recorder si ya existe en la cadena, para que
// AccessLog vea el status que escribe Recover.
func wrapResponseWriter(w http.ResponseWriter) *responseRecorder {
	if rec, ok := w.(*responseRecorder); ok {
		return rec
	}
	return &responseRecorder{ResponseWriter: w}
}

// WriteHeader guarda el primer status escrito y lo pasa al writer original.
func (r *responseRecorder) WriteHeader(status int) {
	if !r.wroteHeader {
		r.status = status
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(status)
}

// Write cuenta los bytes del body; si el handler no escribió status, fija 200
// como hace net/http. Devuelve lo mismo que el writer original.
func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += int64(n)
	return n, err
}

// Status devuelve el status escrito (200 si el handler no escribió nada).
func (r *responseRecorder) Status() int {
	if !r.wroteHeader {
		return http.StatusOK
	}
	return r.status
}

// Unwrap permite a http.ResponseController llegar al writer original.
func (r *responseRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// Flush mantiene el soporte de streaming si el writer original lo tiene.
func (r *responseRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		if !r.wroteHeader {
			r.WriteHeader(http.StatusOK)
		}
		flusher.Flush()
	}
}
