package features

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/authz"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/DoSafills/SOLOServis/backend/internal/ratelimit"
	"github.com/go-chi/chi/v5"
)

// Pruebas de los límites de la Auth API (TAL-201). A diferencia de runCases,
// cada prueba reutiliza el mismo Handler entre requests: los contadores de
// fallos viven en él y deben persistir de un request al siguiente.

const findUserSQL = "SELECT id,password_hash FROM user_account"

// newAuthTestRouter arma la Auth API sobre db con un único Handler y devuelve
// ambos, para que la prueba pueda ajustar los límites del Handler.
func newAuthTestRouter(db *fakeDB) (*Handler, http.Handler) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	h := newHandler("auth-api", db, logger, authz.New(fakeSessions{}, "", logger).RequireSession)
	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	registerRoutes(r, "auth-api", h, authz.New(fakeSessions{}, serviceToken, logger).RequireAdmin)
	return h, r
}

// postAuth envía un POST JSON a target y devuelve la respuesta grabada.
func postAuth(r http.Handler, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// loginBody arma el JSON de login para email y password.
func loginBody(email, password string) string {
	return `{"email":"` + email + `","password":"` + password + `"}`
}

// wantRateLimited verifica un 429 rate_limited con Retry-After entre min y
// max segundos. Es un rango porque la ventana corre mientras la prueba
// calcula PBKDF2 (con -race, cerca de medio segundo por intento).
func wantRateLimited(t *testing.T, rec *httptest.ResponseRecorder, min, max int) {
	t.Helper()
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, quiero 429 (%s)", rec.Code, rec.Body.String())
	}
	if p := decodeProblem(t, rec); p.Code != httpx.CodeRateLimited {
		t.Fatalf("code = %q, quiero %q", p.Code, httpx.CodeRateLimited)
	}
	got, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || got < min || got > max {
		t.Fatalf("Retry-After = %q, quiero entre %d y %d", rec.Header().Get("Retry-After"), min, max)
	}
}

// TestLoginBloqueoPorFallos: con los valores de producción, el intento
// fallido número maxLoginFailures+1 del mismo email responde 429 sin tocar la
// base, aunque cambie mayúsculas; otro email sigue pudiendo intentar.
func TestLoginBloqueoPorFallos(t *testing.T) {
	db := &fakeDB{t: t, rules: []rule{qRow(findUserSQL, int32(7), testPasswordHash)}}
	_, r := newAuthTestRouter(db)

	for i := 1; i <= maxLoginFailures; i++ {
		rec := postAuth(r, "/auth/login", loginBody("ana@mail.cl", "clave-incorrecta"))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("intento %d: status = %d, quiero 401 (%s)", i, rec.Code, rec.Body.String())
		}
	}

	rules := db.rules
	db.rules = nil // bloqueado: no debe consultar la base ni calcular PBKDF2
	wantRateLimited(t, postAuth(r, "/auth/login", loginBody("Ana@Mail.CL", testPassword)),
		1, 900) // lo que queda de la ventana de 15 minutos
	db.rules = rules

	if rec := postAuth(r, "/auth/login", loginBody("otra@mail.cl", "clave-incorrecta")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("otro email: status = %d, quiero 401", rec.Code)
	}
}

// TestLoginExitoReiniciaFallos: un login correcto borra la cuenta de fallos
// del email, así los fallos anteriores no se suman a los siguientes.
func TestLoginExitoReiniciaFallos(t *testing.T) {
	db := &fakeDB{t: t, rules: []rule{qRow(findUserSQL, int32(7), testPasswordHash), userProfile, pruneSessions, saveSession}}
	h, r := newAuthTestRouter(db)
	h.loginFailures = ratelimit.NewFailures(2, time.Minute)

	postAuth(r, "/auth/login", loginBody("ana@mail.cl", "clave-incorrecta"))
	if rec := postAuth(r, "/auth/login", loginBody("ana@mail.cl", testPassword)); rec.Code != http.StatusOK {
		t.Fatalf("login correcto: status = %d, quiero 200 (%s)", rec.Code, rec.Body.String())
	}
	postAuth(r, "/auth/login", loginBody("ana@mail.cl", "clave-incorrecta"))
	if rec := postAuth(r, "/auth/login", loginBody("ana@mail.cl", "clave-incorrecta")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("tras el reinicio solo van 2 fallos: status = %d, quiero 401", rec.Code)
	}
	wantRateLimited(t, postAuth(r, "/auth/login", loginBody("ana@mail.cl", testPassword)), 1, 60)
}

// TestLoginEmailInexistenteCuenta: un email que no existe también suma
// fallos, para que el bloqueo no revele qué cuentas existen.
func TestLoginEmailInexistenteCuenta(t *testing.T) {
	db := &fakeDB{t: t, rules: []rule{qNoRows(findUserSQL)}}
	h, r := newAuthTestRouter(db)
	h.loginFailures = ratelimit.NewFailures(2, time.Minute)

	for i := 1; i <= 2; i++ {
		if rec := postAuth(r, "/auth/login", loginBody("nadie@mail.cl", testPassword)); rec.Code != http.StatusUnauthorized {
			t.Fatalf("intento %d: status = %d, quiero 401", i, rec.Code)
		}
	}
	wantRateLimited(t, postAuth(r, "/auth/login", loginBody("nadie@mail.cl", testPassword)), 1, 60)
}

// TestLoginErrorDeBaseNoCuenta: un error interno de la base no es culpa del
// cliente y no suma fallos.
func TestLoginErrorDeBaseNoCuenta(t *testing.T) {
	db := &fakeDB{t: t, rules: []rule{qFail(findUserSQL, errBoom)}}
	h, r := newAuthTestRouter(db)
	h.loginFailures = ratelimit.NewFailures(1, time.Minute)

	for i := 1; i <= 2; i++ {
		if rec := postAuth(r, "/auth/login", loginBody("ana@mail.cl", testPassword)); rec.Code != http.StatusInternalServerError {
			t.Fatalf("intento %d: status = %d, quiero 500", i, rec.Code)
		}
	}
}

// TestPasswordSinCupo: con todos los cupos de PBKDF2 ocupados, login y
// registro responden 429 con Retry-After sin calcular el hash; el login
// rechazado así no cuenta como fallo del email.
func TestPasswordSinCupo(t *testing.T) {
	db := &fakeDB{t: t, rules: []rule{qRow(findUserSQL, int32(7), testPasswordHash)}}
	h, r := newAuthTestRouter(db)
	h.passwordSlots = ratelimit.NewSlots(1, 10*time.Millisecond)
	if !h.passwordSlots.Acquire(context.Background()) {
		t.Fatal("no se pudo ocupar el cupo")
	}

	wantRateLimited(t, postAuth(r, "/auth/login", loginBody("ana@mail.cl", testPassword)), 2, 2)
	if blocked, _ := h.loginFailures.Blocked("ana@mail.cl"); blocked {
		t.Fatal("un 429 por falta de cupo no debe contar como fallo")
	}

	db.rules = nil // el registro no debe llegar al INSERT
	register := `{"name":"Ana","email":"ana@mail.cl","password":"` + testPassword + `"}`
	wantRateLimited(t, postAuth(r, "/auth/register", register), 2, 2)

	h.passwordSlots.Release()
	db.rules = []rule{qRow(findUserSQL, int32(7), testPasswordHash), userProfile, pruneSessions, saveSession}
	if rec := postAuth(r, "/auth/login", loginBody("ana@mail.cl", testPassword)); rec.Code != http.StatusOK {
		t.Fatalf("con el cupo libre: status = %d, quiero 200 (%s)", rec.Code, rec.Body.String())
	}
}
