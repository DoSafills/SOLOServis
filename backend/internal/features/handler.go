package features

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/apiutil"
	"github.com/DoSafills/SOLOServis/backend/internal/authz"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	name    string
	db      *pgxpool.Pool
	repo    *Repository
	service *Service
	logger  *slog.Logger
	// requireSession es authz.RequireSession: la misma validación de sesión
	// que usan comparison y user (TAL-125, H4).
	requireSession func(http.Handler) http.Handler
}

func NewHandler(name string, db *pgxpool.Pool, logger *slog.Logger) *Handler {
	repository := NewRepository(db)
	return &Handler{name: name, db: db, repo: repository, service: NewService(repository), logger: logger,
		requireSession: authz.New(authz.NewPostgresSessions(db), "", logger).RequireSession}
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
		h.dbError(w, r, err, http.StatusInternalServerError, "database operation failed")
		return
	}
	apiutil.WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) created(w http.ResponseWriter, r *http.Request, query string, args ...any) {
	result, err := h.repo.QueryJSON(r.Context(), query, args...)
	if err != nil {
		h.dbError(w, r, err, http.StatusBadRequest, "could not create resource")
		return
	}
	if len(result) == 0 {
		writeError(w, r, http.StatusInternalServerError, "database returned no resource")
		return
	}
	apiutil.WriteJSON(w, http.StatusCreated, result[0])
}

func (h *Handler) one(w http.ResponseWriter, r *http.Request, query string, args ...any) {
	result, err := h.repo.QueryJSON(r.Context(), query, args...)
	if err != nil {
		h.dbError(w, r, err, http.StatusInternalServerError, "database operation failed")
		return
	}
	if len(result) == 0 {
		writeError(w, r, http.StatusNotFound, "resource not found")
		return
	}
	apiutil.WriteJSON(w, http.StatusOK, result[0])
}

func decode(r *http.Request, target any) bool {
	return apiutil.DecodeJSON(r, target) == nil
}

func intParam(r *http.Request, key string) (int32, bool) {
	value, err := strconv.ParseInt(chi.URLParam(r, key), 10, 32)
	return int32(value), err == nil && value > 0
}

func queryInt(r *http.Request, key string) (int32, bool) {
	value, err := strconv.ParseInt(r.URL.Query().Get(key), 10, 32)
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

// dbError responde status y message, salvo que err sea una falla para llegar a
// la base (httpx.IsUnavailable): entonces 503. Así una base caída no se
// confunde con un error del cliente (400/404) ni con una sesión vencida (401),
// que haría que el frontend cierre la sesión del usuario. Los 5xx quedan en el
// log con su request_id (httpx.WriteInternal).
func (h *Handler) dbError(w http.ResponseWriter, r *http.Request, err error, status int, message string) {
	if err != nil && (httpx.IsUnavailable(err) || status >= http.StatusInternalServerError) {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	writeError(w, r, status, message)
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
