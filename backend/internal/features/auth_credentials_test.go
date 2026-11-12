package features

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DoSafills/SOLOServis/backend/internal/authz"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// TAL-162: las credenciales se guardan hasheadas y el cierre de sesión usa el
// hash, nunca el token en claro.

// recordingDB conserva la última consulta para poder afirmar qué se envió a
// PostgreSQL sin imprimir secretos.
type recordingDB struct {
	*fakeDB
	sql  string
	args []any
}

func (r *recordingDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	r.sql, r.args = sql, args
	return r.fakeDB.Exec(ctx, sql, args...)
}

func (r *recordingDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	r.sql, r.args = sql, args
	return r.fakeDB.Query(ctx, sql, args...)
}

func (r *recordingDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	r.sql, r.args = sql, args
	return r.fakeDB.QueryRow(ctx, sql, args...)
}

func authRouter(t *testing.T, db DB) http.Handler {
	t.Helper()
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	h := newHandler("auth-api", db, logger, authz.New(fakeSessions{}, "", logger).RequireSession)
	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	registerRoutes(r, "auth-api", h, authz.New(fakeSessions{}, serviceToken, logger).RequireAdmin)
	return r
}

func TestSessionTokensAreStoredHashed(t *testing.T) {
	db := &recordingDB{fakeDB: &fakeDB{t: t, rules: []rule{
		qRow("SELECT id,password_hash FROM user_account", int32(7), testPasswordHash),
		userProfile,
		qExec("INSERT INTO user_session", 1),
	}}}
	router := authRouter(t, db)

	body := fmt.Sprintf(`{"email":"ana@mail.cl","password":%q}`, testPassword)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var issued struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &issued); err != nil {
		t.Fatalf("respuesta ilegible: %v", err)
	}
	if issued.AccessToken == "" || issued.RefreshToken == "" {
		t.Fatal("el login debía emitir ambos tokens")
	}

	if !strings.Contains(db.sql, "INSERT INTO user_session") {
		t.Fatalf("última consulta = %q", db.sql)
	}
	if len(db.args) != 3 {
		t.Fatalf("args = %v", db.args)
	}

	var hashes [][]byte
	for _, arg := range db.args {
		if raw, ok := arg.(string); ok && (raw == issued.AccessToken || raw == issued.RefreshToken) {
			t.Fatal("un token en claro llegó a la base")
		}
		if hash, ok := arg.([]byte); ok {
			hashes = append(hashes, hash)
		}
	}
	if len(hashes) != 2 {
		t.Fatalf("se esperaban dos hashes, args = %v", db.args)
	}
	if !bytes.Equal(hashes[0], tokenHash(issued.AccessToken)) {
		t.Fatal("el access token no se guardó con su hash")
	}
	if !bytes.Equal(hashes[1], tokenHash(issued.RefreshToken)) {
		t.Fatal("el refresh token no se guardó con su hash")
	}
	if bytes.Equal(hashes[0], hashes[1]) {
		t.Fatal("access y refresh no pueden compartir hash")
	}
}

func TestLogoutDeletesSessionByTokenHash(t *testing.T) {
	token := "un-token-de-sesion-cualquiera"
	db := &recordingDB{fakeDB: &fakeDB{t: t, rules: []rule{
		qExec("DELETE FROM user_session", 1),
	}}}
	router := authRouter(t, db)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(db.sql, "access_token_hash=$1 OR refresh_token_hash=$1") {
		t.Fatalf("la baja debía filtrar por hash: %s", db.sql)
	}
	if len(db.args) != 1 {
		t.Fatalf("args = %v", db.args)
	}
	if raw, ok := db.args[0].(string); ok && raw == token {
		t.Fatal("el token en claro llegó a la consulta de logout")
	}
	hash, ok := db.args[0].([]byte)
	if !ok || !bytes.Equal(hash, tokenHash(token)) {
		t.Fatalf("se esperaba el hash SHA-256, args = %v", db.args)
	}
}
