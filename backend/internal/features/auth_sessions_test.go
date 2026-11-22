package features

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/ratelimit"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Pruebas de TAL-200: vencimiento absoluto de las sesiones (H5), poda y tope
// por usuario (H6), login sin enumeración por tiempo (H2) y PBKDF2 de la
// librería estándar (H15).

// query es una consulta que recibió sequenceDB.
type query struct {
	sql  string
	args []any
}

// sequenceDB es un fakeDB que guarda todas las consultas en orden, para
// afirmar qué se ejecutó y con qué argumentos.
type sequenceDB struct {
	*fakeDB
	queries []query
}

// Exec registra la consulta y la delega en el fake.
func (s *sequenceDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	s.queries = append(s.queries, query{sql, args})
	return s.fakeDB.Exec(ctx, sql, args...)
}

// Query registra la consulta y la delega en el fake.
func (s *sequenceDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	s.queries = append(s.queries, query{sql, args})
	return s.fakeDB.Query(ctx, sql, args...)
}

// QueryRow registra la consulta y la delega en el fake.
func (s *sequenceDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	s.queries = append(s.queries, query{sql, args})
	return s.fakeDB.QueryRow(ctx, sql, args...)
}

// find devuelve la posición y la consulta que contiene fragment, o -1.
func (s *sequenceDB) find(fragment string) (int, query) {
	for i, q := range s.queries {
		if strings.Contains(q.sql, fragment) {
			return i, q
		}
	}
	return -1, query{}
}

// issuedSession es lo que importa de la respuesta de login o refresh.
type issuedSession struct {
	ExpiresIn        int       `json:"expiresIn"`
	RefreshExpiresAt time.Time `json:"refreshExpiresAt"`
}

// TestVencimientoDeLaSesion: login y registro crean sesiones de 30 días; un
// refresh hereda lo que le quedaba a la sesión anterior y el access token no
// dura más que la sesión (TAL-200, H5).
func TestVencimientoDeLaSesion(t *testing.T) {
	findUser := qRow(findUserSQL, int32(7), testPasswordHash)
	rotate := func(remaining float64) rule {
		return qRow("DELETE FROM user_session WHERE refresh_token_hash", int32(7), remaining)
	}
	tests := []struct {
		name        string
		target      string
		body        string
		rules       []rule
		wantStatus  int
		wantSession time.Duration
		wantAccess  time.Duration
	}{
		{"login dura 30 días", "/auth/login", loginBody("ana@mail.cl", testPassword),
			[]rule{findUser}, http.StatusOK, sessionLifetime, accessTokenLifetime},
		{"registro dura 30 días", "/auth/register", `{"name":"Ana","email":"ana@mail.cl","password":"` + testPassword + `"}`,
			[]rule{qRow("INSERT INTO user_account", int32(7))}, http.StatusCreated, sessionLifetime, accessTokenLifetime},
		{"refresh hereda lo que quedaba", "/auth/refresh", `{"refreshToken":"r"}`,
			[]rule{rotate(5 * 24 * 3600)}, http.StatusOK, 5 * 24 * time.Hour, accessTokenLifetime},
		{"el access no dura más que la sesión", "/auth/refresh", `{"refreshToken":"r"}`,
			[]rule{rotate(120)}, http.StatusOK, 2 * time.Minute, 2 * time.Minute},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := &sequenceDB{fakeDB: &fakeDB{t: t, rules: append(tc.rules, userProfile, pruneSessions, saveSession)}}
			rec := postAuth(authRouter(t, db), tc.target, tc.body)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, quiero %d (%s)", rec.Code, tc.wantStatus, rec.Body.String())
			}

			_, insert := db.find("INSERT INTO user_session")
			if len(insert.args) != 5 {
				t.Fatalf("args del INSERT = %v", insert.args)
			}
			if got := insert.args[3].(float64); got != tc.wantAccess.Seconds() {
				t.Fatalf("access_expires_at = NOW()+%vs, quiero %vs", got, tc.wantAccess.Seconds())
			}
			if got := insert.args[4].(float64); got != tc.wantSession.Seconds() {
				t.Fatalf("refresh_expires_at = NOW()+%vs, quiero %vs", got, tc.wantSession.Seconds())
			}

			var issued issuedSession
			if err := json.Unmarshal(rec.Body.Bytes(), &issued); err != nil {
				t.Fatalf("respuesta ilegible: %v", err)
			}
			if issued.ExpiresIn != int(tc.wantAccess.Seconds()) {
				t.Fatalf("expiresIn = %d, quiero %v", issued.ExpiresIn, tc.wantAccess.Seconds())
			}
			if drift := time.Until(issued.RefreshExpiresAt) - tc.wantSession; math.Abs(drift.Seconds()) > 5 {
				t.Fatalf("refreshExpiresAt = %v, quiero ahora + %v", issued.RefreshExpiresAt, tc.wantSession)
			}
		})
	}
}

// TestPodaDeSesiones: antes de guardar la sesión nueva se borran las
// vencidas del usuario y las que excedan el tope, dejando lugar para la nueva
// (TAL-200, H6).
func TestPodaDeSesiones(t *testing.T) {
	db := &sequenceDB{fakeDB: &fakeDB{t: t, rules: []rule{
		qRow(findUserSQL, int32(7), testPasswordHash), userProfile, pruneSessions, saveSession,
	}}}
	if rec := postAuth(authRouter(t, db), "/auth/login", loginBody("ana@mail.cl", testPassword)); rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}

	pruneAt, prune := db.find(pruneSessions.sql)
	insertAt, _ := db.find("INSERT INTO user_session")
	if pruneAt < 0 || insertAt < 0 || pruneAt > insertAt {
		t.Fatalf("la poda debe ir antes del INSERT: poda en %d, INSERT en %d", pruneAt, insertAt)
	}
	for _, fragment := range []string{"refresh_expires_at<=NOW()", "ORDER BY id DESC LIMIT $2"} {
		if !strings.Contains(prune.sql, fragment) {
			t.Fatalf("la poda no contiene %q: %s", fragment, prune.sql)
		}
	}
	if fmt.Sprint(prune.args) != fmt.Sprint([]any{int32(7), maxSessionsPerUser - 1}) {
		t.Fatalf("args de la poda = %v, quiero [7 %d]", prune.args, maxSessionsPerUser-1)
	}
}

// TestLoginSinEnumeracion: un email inexistente recorre el mismo camino que
// una contraseña incorrecta. Con el único cupo de PBKDF2 ocupado, los dos
// responden 429: prueba que ambos calculan el hash, y por eso tardan lo mismo
// (TAL-200, H2). Con cupo, los dos responden el mismo 401.
func TestLoginSinEnumeracion(t *testing.T) {
	tests := []struct {
		name  string
		rules []rule
	}{
		{"email existente, contraseña incorrecta", []rule{qRow(findUserSQL, int32(7), testPasswordHash)}},
		{"email inexistente o cuenta inactiva", []rule{qNoRows(findUserSQL)}},
	}
	var bodies []string
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, r := newAuthTestRouter(&fakeDB{t: t, rules: tc.rules})
			body := loginBody("ana@mail.cl", "clave-incorrecta")

			h.passwordSlots = ratelimit.NewSlots(1, 10*time.Millisecond)
			if !h.passwordSlots.Acquire(context.Background()) {
				t.Fatal("no se pudo ocupar el cupo")
			}
			wantRateLimited(t, postAuth(r, "/auth/login", body), 2, 2)
			h.passwordSlots.Release()

			rec := postAuth(r, "/auth/login", body)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, quiero 401 (%s)", rec.Code, rec.Body.String())
			}
			p := decodeProblem(t, rec)
			bodies = append(bodies, p.Code+" "+p.Detail)
		})
	}
	if len(bodies) == 2 && bodies[0] != bodies[1] {
		t.Fatalf("las respuestas difieren: %q y %q", bodies[0], bodies[1])
	}
}

// TestDummyPasswordHash: el hash ficticio usa el formato y las iteraciones de
// passwordHash, así su PBKDF2 cuesta lo mismo que el de una cuenta real, y
// se calcula una sola vez.
func TestDummyPasswordHash(t *testing.T) {
	hash := dummyPasswordHash()
	parts := strings.Split(hash, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" || parts[1] != fmt.Sprint(passwordIterations) {
		t.Fatalf("hash ficticio con otro formato: %q", hash)
	}
	if dummyPasswordHash() != hash {
		t.Fatal("el hash ficticio debe calcularse una sola vez")
	}
}

// legacyPasswordHash es el hash de testPassword que generó la implementación
// propia de PBKDF2 anterior a TAL-200 (sal fija "sal-fija-de-16-b").
const legacyPasswordHash = "pbkdf2-sha256$120000$c2FsLWZpamEtZGUtMTYtYg$UiZ6zTP29S0mrWRPfzgOmWGS/KYdcbLh3iSFEovaq8I"

// TestCheckPasswordCompatible: crypto/pbkdf2 valida los hashes que ya están
// guardados en user_account y rechaza los mal formados (TAL-200, H15).
func TestCheckPasswordCompatible(t *testing.T) {
	tests := []struct {
		name     string
		encoded  string
		password string
		want     bool
	}{
		{"hash de la implementación anterior", legacyPasswordHash, testPassword, true},
		{"hash anterior con otra contraseña", legacyPasswordHash, testPassword + "x", false},
		{"hash nuevo", testPasswordHash, testPassword, true},
		{"otro algoritmo", "pbkdf2-sha1$120000$c2Fs$c2Fs", testPassword, false},
		{"iteraciones bajo el mínimo", "pbkdf2-sha256$1000$c2FsLWZpamEtZGUtMTYtYg$UiZ6zTP29S0mrWRPfzgOmWGS/KYdcbLh3iSFEovaq8I", testPassword, false},
		{"sal en otro base64", "pbkdf2-sha256$120000$c2Fs!$UiZ6zTP29S0mrWRPfzgOmWGS/KYdcbLh3iSFEovaq8I", testPassword, false},
		{"hash vacío", "pbkdf2-sha256$120000$c2FsLWZpamEtZGUtMTYtYg$", testPassword, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := checkPassword(tc.encoded, tc.password); got != tc.want {
				t.Fatalf("checkPassword = %v, quiero %v", got, tc.want)
			}
		})
	}
}
