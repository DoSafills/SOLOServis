package features

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DoSafills/SOLOServis/backend/internal/authz"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
)

// TAL-198: la entrada del cliente nunca se concatena al SQL. Estos tests
// atacan las dos superficies reales: el login/registro (validación antes de
// tocar la base) y la creación de notificaciones (payload guardado como
// argumento).

var injectionPayloads = []string{
	`' OR 1=1--`,
	`'; DROP TABLE notification;--`,
	`" OR ""="`,
	`1' UNION SELECT password_hash FROM user_account--`,
}

func TestInjectionPayloadsAreRejectedBeforeDatabaseAccess(t *testing.T) {
	cases := []apiCase{}
	for i, payload := range injectionPayloads {
		body := fmt.Sprintf(`{"email":%q,"password":"Clave12345!"}`, payload)
		cases = append(cases,
			apiCase{
				name:       fmt.Sprintf("login/%d", i),
				method:     http.MethodPost,
				target:     "/auth/login",
				body:       body,
				token:      "-",
				wantStatus: http.StatusUnprocessableEntity,
				wantCode:   httpx.CodeValidationFailed,
				wantFields: []string{"email/format"},
			},
			apiCase{
				name:       fmt.Sprintf("register/%d", i),
				method:     http.MethodPost,
				target:     "/auth/register",
				body:       fmt.Sprintf(`{"name":"Ana","email":%q,"password":"Clave12345!"}`, payload),
				token:      "-",
				wantStatus: http.StatusUnprocessableEntity,
				wantCode:   httpx.CodeValidationFailed,
				wantFields: []string{"email/format"},
			},
		)
	}

	// Sin reglas: si el handler llegara a consultar la base, fakeDB fallaría.
	runCases(t, "auth-api", cases)
}

func TestInjectionPayloadStaysAParameter(t *testing.T) {
	payload := `' OR 1=1; DROP TABLE notification;--`

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	db := &fakeDB{t: t, rules: []rule{
		qRows("INSERT INTO notification", `{"id":1,"userId":7}`),
	}}
	h := newHandler("notification-api", db, logger, authz.New(fakeSessions{}, "", logger).RequireSession)
	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	registerRoutes(r, "notification-api", h, authz.New(fakeSessions{}, serviceToken, logger).RequireAdmin)

	// El token de servicio entra como admin; "type" es texto libre y es la
	// peor superficie: si algo se concatenara, sería esta.
	body := fmt.Sprintf(
		`{"userId":7,"type":%q,"title":"alerta","message":%q}`,
		payload, payload,
	)
	req := httptest.NewRequest(http.MethodPost, "/notifications", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+serviceToken)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, quiero 201: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(db.lastSQL, "OR 1=1") || strings.Contains(db.lastSQL, "DROP TABLE") {
		t.Fatalf("la entrada se concatenó al SQL:\n%s", db.lastSQL)
	}
	if !strings.Contains(fmt.Sprint(db.lastArgs), "OR 1=1") {
		t.Fatalf("el payload debía viajar como argumento, args = %v", db.lastArgs)
	}
	if !strings.Contains(db.lastSQL, "$1") {
		t.Fatalf("la consulta debía usar placeholders: %s", db.lastSQL)
	}
}
