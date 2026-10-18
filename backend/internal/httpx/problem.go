// Package httpx implementa el estándar de API de SOLOServis (TAL-78):
// errores RFC 9457, decodificación JSON estricta, request_id, logs de acceso
// y recuperación ante panics. Solo depende de la biblioteca estándar.
package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ProblemContentType es el media type de RFC 9457.
const ProblemContentType = "application/problem+json"

// Códigos internos estables (ver docs/estandares/TAL-78-estandar-api.md).
const (
	CodeMalformedJSON        = "malformed_json"
	CodeUnknownField         = "unknown_field"
	CodeInvalidType          = "invalid_type"
	CodeInvalidParameter     = "invalid_parameter"
	CodeNotFound             = "not_found"
	CodeConflict             = "conflict"
	CodeBodyTooLarge         = "body_too_large"
	CodeUnsupportedMediaType = "unsupported_media_type"
	CodeValidationFailed     = "validation_failed"
	CodeInternalError        = "internal_error"
)

const internalErrorDetail = "Ocurrió un error interno. Repórtalo indicando el request_id."

// FieldError describe un error asociado a un campo del request.
type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Problem es el cuerpo de error RFC 9457 con las extensiones del estándar.
type Problem struct {
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Status    int          `json:"status"`
	Detail    string       `json:"detail"`
	Instance  string       `json:"instance"`
	Code      string       `json:"code"`
	RequestID string       `json:"request_id"`
	Errors    []FieldError `json:"errors,omitempty"`
}

// WriteProblem responde un error con el formato estándar.
func WriteProblem(w http.ResponseWriter, r *http.Request, status int, code, detail string, fieldErrors ...FieldError) {
	problem := Problem{
		Type:      "about:blank",
		Title:     http.StatusText(status),
		Status:    status,
		Detail:    detail,
		Instance:  r.URL.Path,
		Code:      code,
		RequestID: RequestIDFrom(r.Context()),
		Errors:    fieldErrors,
	}

	w.Header().Set("Content-Type", ProblemContentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem)
}

// WriteValidation responde 422 con la lista de errores por campo.
func WriteValidation(w http.ResponseWriter, r *http.Request, fieldErrors []FieldError) {
	WriteProblem(w, r, http.StatusUnprocessableEntity, CodeValidationFailed,
		"La solicitud no cumple las reglas de validación.", fieldErrors...)
}

// WriteInternal registra el error real y responde 500 con un mensaje genérico.
// El detalle de err nunca llega al cliente.
func WriteInternal(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	logger.ErrorContext(r.Context(), "internal_error",
		slog.String("request_id", RequestIDFrom(r.Context())),
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.String("error", errorString(err)),
	)
	WriteProblem(w, r, http.StatusInternalServerError, CodeInternalError, internalErrorDetail)
}

func errorString(err error) string {
	if err == nil {
		return "<nil>"
	}
	return err.Error()
}
