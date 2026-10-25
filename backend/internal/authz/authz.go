// Package authz protege los endpoints de escritura (TAL-70): solo pasan
// usuarios con rol admin (token de sesión de Auth API) o servicios internos
// que presenten INTERNAL_API_TOKEN (p. ej. Ingestion API).
package authz

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MinServiceTokenLength evita aceptar tokens internos triviales.
const MinServiceTokenLength = 32

// ErrNoSession indica que el token no corresponde a una sesión vigente.
var ErrNoSession = errors.New("no active session")

// SessionStore resuelve el rol del usuario dueño de un access token.
type SessionStore interface {
	RoleForAccessToken(ctx context.Context, tokenHash []byte) (string, error)
}

// PostgresSessions lee las sesiones que emite Auth API (tabla user_session).
type PostgresSessions struct {
	db *pgxpool.Pool
}

func NewPostgresSessions(db *pgxpool.Pool) PostgresSessions {
	return PostgresSessions{db: db}
}

func (s PostgresSessions) RoleForAccessToken(ctx context.Context, tokenHash []byte) (string, error) {
	var role string
	err := s.db.QueryRow(ctx, `
		SELECT u.role::text
		FROM user_session s
		JOIN user_account u ON u.id = s.user_id
		WHERE s.access_token_hash = $1
		  AND s.access_expires_at > NOW()
		  AND u.active`, tokenHash).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNoSession
	}
	return role, err
}

type Authorizer struct {
	sessions     SessionStore
	serviceToken []byte
	logger       *slog.Logger
}

// New crea el autorizador. Un serviceToken más corto que
// MinServiceTokenLength se ignora: solo quedan habilitadas las sesiones admin.
func New(sessions SessionStore, serviceToken string, logger *slog.Logger) *Authorizer {
	a := &Authorizer{sessions: sessions, logger: logger}
	if len(serviceToken) >= MinServiceTokenLength {
		a.serviceToken = []byte(serviceToken)
	} else if serviceToken != "" {
		logger.Warn("INTERNAL_API_TOKEN ignorado: debe tener al menos 32 caracteres")
	}
	return a
}

// RequireAdmin responde 401 sin token válido y 403 si el usuario no es admin.
func (a *Authorizer) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			unauthorized(w, r)
			return
		}

		if a.serviceToken != nil && subtle.ConstantTimeCompare([]byte(token), a.serviceToken) == 1 {
			next.ServeHTTP(w, r)
			return
		}

		hash := sha256.Sum256([]byte(token))
		role, err := a.sessions.RoleForAccessToken(r.Context(), hash[:])
		switch {
		case errors.Is(err, ErrNoSession):
			unauthorized(w, r)
		case err != nil:
			httpx.WriteInternal(w, r, a.logger, err)
		case role != "admin":
			httpx.WriteProblem(w, r, http.StatusForbidden, httpx.CodeForbidden,
				"Se requiere rol de administrador para esta operación.")
		default:
			next.ServeHTTP(w, r)
		}
	})
}

func unauthorized(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	httpx.WriteProblem(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized,
		"Se requiere un token de acceso válido (Authorization: Bearer <token>).")
}

func bearerToken(r *http.Request) string {
	value := r.Header.Get("Authorization")
	if len(value) > 7 && strings.EqualFold(value[:7], "Bearer ") {
		return strings.TrimSpace(value[7:])
	}
	return ""
}
