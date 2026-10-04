package features

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/apiutil"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	name    string
	db      *pgxpool.Pool
	repo    *Repository
	service *Service
}

type sessionUserIDKey struct{}

func (h *Handler) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			apiutil.WriteError(w, http.StatusUnauthorized, "bearer token is required")
			return
		}

		var userID int32
		err := h.db.QueryRow(r.Context(), `SELECT s.user_id FROM user_session s
			JOIN user_account u ON u.id=s.user_id
			WHERE s.access_token_hash=$1 AND s.access_expires_at>NOW() AND u.active`,
			tokenHash(token)).Scan(&userID)
		if err != nil {
			apiutil.WriteError(w, http.StatusUnauthorized, "invalid or expired access token")
			return
		}

		ctx := context.WithValue(r.Context(), sessionUserIDKey{}, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func NewHandler(name string, db *pgxpool.Pool) *Handler {
	repository := NewRepository(db)
	return &Handler{name: name, db: db, repo: repository, service: NewService(repository)}
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
		apiutil.WriteError(w, http.StatusInternalServerError, "database operation failed")
		return
	}
	apiutil.WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) created(w http.ResponseWriter, r *http.Request, query string, args ...any) {
	result, err := h.repo.QueryJSON(r.Context(), query, args...)
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "could not create resource")
		return
	}
	if len(result) == 0 {
		apiutil.WriteError(w, http.StatusInternalServerError, "database returned no resource")
		return
	}
	apiutil.WriteJSON(w, http.StatusCreated, result[0])
}

func (h *Handler) one(w http.ResponseWriter, r *http.Request, query string, args ...any) {
	result, err := h.repo.QueryJSON(r.Context(), query, args...)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "database operation failed")
		return
	}
	if len(result) == 0 {
		apiutil.WriteError(w, http.StatusNotFound, "resource not found")
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

func badRequest(w http.ResponseWriter, message string) {
	apiutil.WriteError(w, http.StatusBadRequest, message)
}

func normalizeUserID(r *http.Request, userID int32) int32 {
	if authenticatedID, ok := r.Context().Value(sessionUserIDKey{}).(int32); ok {
		return authenticatedID
	}
	if userID == 0 {
		userID, _ = queryInt(r, "userId")
	}
	return userID
}

func requestUserID(r *http.Request) (int32, bool) {
	if userID, ok := r.Context().Value(sessionUserIDKey{}).(int32); ok {
		return userID, true
	}
	return queryInt(r, "userId")
}

func rawJSON(value json.RawMessage) string {
	if len(value) == 0 {
		return "{}"
	}
	return string(value)
}
