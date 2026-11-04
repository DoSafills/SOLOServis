package features

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/apiutil"
	"github.com/DoSafills/SOLOServis/backend/internal/authz"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB es lo que features usa de PostgreSQL. *pgxpool.Pool lo implementa; los
// tests usan un fake en memoria (estándar TAL-78 §5).
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
	Ping(ctx context.Context) error
}

type Handler struct {
	name    string
	db      DB
	repo    *Repository
	service *Service
	logger  *slog.Logger
	// requireSession es authz.RequireSession: la misma validación de sesión
	// que usan comparison y user (TAL-125, H4).
	requireSession func(http.Handler) http.Handler
}

func NewHandler(name string, db *pgxpool.Pool, logger *slog.Logger) *Handler {
	return newHandler(name, db, logger, authz.New(authz.NewPostgresSessions(db), "", logger).RequireSession)
}

func newHandler(name string, db DB, logger *slog.Logger, requireSession func(http.Handler) http.Handler) *Handler {
	repository := NewRepository(db)
	return &Handler{name: name, db: db, repo: repository, service: NewService(repository), logger: logger,
		requireSession: requireSession}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.db.Ping(ctx); err != nil {
		apiutil.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "service": h.name, "database": "disconnected"})
		return
	}
	apiutil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": h.name, "database": "connected"})
}

func (h *Handler) rows(w http.ResponseWriter, r *http.Request, query string, args ...any) {
	result, err := h.repo.QueryJSON(r.Context(), query, args...)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	apiutil.WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) created(w http.ResponseWriter, r *http.Request, query string, args ...any) {
	result, err := h.repo.QueryJSON(r.Context(), query, args...)
	if err != nil {
		h.dbError(w, r, err, http.StatusInternalServerError, "No se pudo crear el recurso.")
		return
	}
	if len(result) == 0 {
		httpx.WriteInternal(w, r, h.logger, errors.New("insert returned no rows"))
		return
	}
	apiutil.WriteJSON(w, http.StatusCreated, result[0])
}

func (h *Handler) one(w http.ResponseWriter, r *http.Request, notFound string, query string, args ...any) {
	result, err := h.repo.QueryJSON(r.Context(), query, args...)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	if len(result) == 0 {
		writeError(w, r, http.StatusNotFound, notFound)
		return
	}
	apiutil.WriteJSON(w, http.StatusOK, result[0])
}

// decode lee el body con httpx.DecodeJSON (TAL-78 §3): si falla ya respondió
// 400 (JSON malformado, campo desconocido o tipo incorrecto), 413 o 415.
func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	if err := httpx.DecodeJSON(w, r, target, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteDecodeError(w, r, err)
		return false
	}
	return true
}

// valid responde 422 con los errores por campo si v no es válido.
func valid(w http.ResponseWriter, r *http.Request, v *httpx.Validator) bool {
	if !v.Valid() {
		httpx.WriteValidation(w, r, v.Errors())
		return false
	}
	return true
}

func intParam(r *http.Request, key string) (int32, bool) {
	value, err := strconv.ParseInt(chi.URLParam(r, key), 10, 32)
	return int32(value), err == nil && value > 0
}

// writeError responde problem+json, como las APIs del estándar TAL-78
// (TAL-125, H3). El code sale del status.
func writeError(w http.ResponseWriter, r *http.Request, status int, message string) {
	httpx.WriteProblem(w, r, status, problemCode(status), message)
}

func problemCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return httpx.CodeInvalidParameter
	case http.StatusUnauthorized:
		return httpx.CodeUnauthorized
	case http.StatusForbidden:
		return httpx.CodeForbidden
	case http.StatusNotFound:
		return httpx.CodeNotFound
	case http.StatusConflict:
		return httpx.CodeConflict
	case http.StatusServiceUnavailable:
		return httpx.CodeServiceUnavailable
	default:
		return httpx.CodeInternalError
	}
}

// dbError traduce un error de la base según TAL-78 §3.5:
//   - err nil o pgx.ErrNoRows ("no hay fila"): status y message (404, o 401
//     en auth); si status es 5xx se trata como error interno;
//   - errInvalidID: 400 invalid_parameter;
//   - SQLSTATE 23505 (unique): 409;
//   - SQLSTATE 23503 (FK) y 23514 (check): 422 con el campo afectado;
//   - cualquier otro: 500 (503 si la base no responde) con el detalle en el
//     log junto a su request_id (httpx.WriteInternal). Así una base caída no
//     se confunde con un error del cliente ni con una sesión vencida (401),
//     que haría que el frontend cierre la sesión del usuario.
func (h *Handler) dbError(w http.ResponseWriter, r *http.Request, err error, status int, message string) {
	var pgErr *pgconn.PgError
	switch {
	case err == nil || errors.Is(err, pgx.ErrNoRows):
		if status >= http.StatusInternalServerError {
			httpx.WriteInternal(w, r, h.logger, errors.Join(errors.New(message), err))
			return
		}
		writeError(w, r, status, message)
	case errors.Is(err, errInvalidID):
		writeError(w, r, http.StatusBadRequest, "El identificador no tiene un formato válido.")
	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		if status != http.StatusConflict {
			message = "El recurso ya existe."
		}
		writeError(w, r, http.StatusConflict, message)
	case errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23514"):
		httpx.WriteValidation(w, r, []httpx.FieldError{constraintField(pgErr)})
	default:
		httpx.WriteInternal(w, r, h.logger, err)
	}
}

// referenceError responde un id del body que no apunta a un recurso:
// 422 con el campo (TAL-78 §2). Los demás errores siguen dbError.
func (h *Handler) referenceError(w http.ResponseWriter, r *http.Request, err error, field string) {
	switch {
	case errors.Is(err, errInvalidID):
		httpx.WriteValidation(w, r, []httpx.FieldError{{Field: field, Code: "format",
			Message: field + " debe ser un id numérico o un UUID"}})
	case errors.Is(err, pgx.ErrNoRows):
		httpx.WriteValidation(w, r, []httpx.FieldError{{Field: field, Code: "not_found",
			Message: field + " no existe"}})
	default:
		h.dbError(w, r, err, http.StatusInternalServerError, "No se pudo resolver "+field+".")
	}
}

// remoteError responde 503 cuando otra API no contesta y deja el motivo en
// el log con su request_id; al cliente no le llega el detalle.
func (h *Handler) remoteError(w http.ResponseWriter, r *http.Request, err error, message string) {
	h.logger.WarnContext(r.Context(), "remote_service_unavailable",
		slog.String("request_id", httpx.RequestIDFrom(r.Context())),
		slog.String("path", r.URL.Path),
		slog.String("error", err.Error()),
	)
	writeError(w, r, http.StatusServiceUnavailable, message)
}

// constraintField arma el error por campo de una violación de FK o CHECK. La
// columna sale del detalle de PostgreSQL ("Key (user_id)=(9) is not present
// …") y se expone en camelCase, como el resto de la API.
func constraintField(pgErr *pgconn.PgError) httpx.FieldError {
	column := pgErr.ColumnName
	if start := strings.Index(pgErr.Detail, "Key ("); column == "" && start >= 0 {
		rest := pgErr.Detail[start+len("Key ("):]
		if end := strings.Index(rest, ")"); end > 0 {
			column = rest[:end]
		}
	}
	field := camelCase(column)
	if field == "" {
		field = "body"
	}
	if pgErr.Code == "23503" {
		return httpx.FieldError{Field: field, Code: "not_found", Message: field + " no existe"}
	}
	return httpx.FieldError{Field: field, Code: "format", Message: field + " no cumple las reglas"}
}

func camelCase(snake string) string {
	parts := strings.Split(snake, "_")
	for i := 1; i < len(parts); i++ {
		if parts[i] != "" {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

func badRequest(w http.ResponseWriter, r *http.Request, message string) {
	writeError(w, r, http.StatusBadRequest, message)
}

// sessionUserID es el usuario de la sesión que validó requireSession. El
// userId del body o del query se ignora: confiar en él permitía leer o
// modificar los datos de otro usuario (TAL-105 H1). Los inputs aún aceptan
// el campo userId porque el frontend lo envía y el decoder rechaza campos
// desconocidos.
func sessionUserID(r *http.Request) int32 {
	session, _ := authz.SessionFrom(r.Context())
	return session.UserID
}

func rawJSON(value json.RawMessage) string {
	if len(value) == 0 {
		return "{}"
	}
	return string(value)
}
