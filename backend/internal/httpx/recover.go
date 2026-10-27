package httpx

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recover convierte un panic en un 500 genérico y registra el valor y el stack.
// Si la respuesta ya empezó a escribirse, solo registra (no puede cambiarla).
// http.ErrAbortHandler se relanza para respetar su semántica en net/http.
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := wrapResponseWriter(w)

			defer func() {
				value := recover()
				if value == nil {
					return
				}
				if value == http.ErrAbortHandler {
					panic(value)
				}

				logger.ErrorContext(r.Context(), "panic_recovered",
					slog.String("request_id", RequestIDFrom(r.Context())),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.String("panic", fmt.Sprint(value)),
					slog.String("stack", string(debug.Stack())),
				)

				if rec.wroteHeader {
					return
				}
				WriteProblem(rec, r, http.StatusInternalServerError, CodeInternalError, internalErrorDetail)
			}()

			next.ServeHTTP(rec, r)
		})
	}
}
