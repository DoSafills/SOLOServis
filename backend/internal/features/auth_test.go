package features

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
)

const testPassword = "Clave12345!"

// testPasswordHash se calcula una vez: pbkdf2 con 120 000 iteraciones.
var testPasswordHash = func() string {
	hash, err := passwordHash(testPassword)
	if err != nil {
		panic(err)
	}
	return hash
}()

// Reglas de issueTokens: lee el usuario y guarda la sesión.
var (
	userProfile = qRow("SELECT name,email FROM user_account", "Ana", "ana@mail.cl")
	saveSession = qExec("INSERT INTO user_session", 1)
)

func TestAuthAPI(t *testing.T) {
	register := `{"name":" Ana ","email":"Ana@Mail.cl","password":"` + testPassword + `"}`
	login := `{"email":"ana@mail.cl","password":"` + testPassword + `"}`
	insertUser := "INSERT INTO user_account"
	findUser := "SELECT id,password_hash FROM user_account"
	rotate := "DELETE FROM user_session WHERE refresh_token_hash"
	validate := "SELECT user_id FROM user_session"

	runCases(t, "auth-api", []apiCase{
		// POST /auth/register
		{name: "register/feliz", method: http.MethodPost, target: "/auth/register", body: register, token: "-",
			rules:      []rule{qRow(insertUser, int32(7)), userProfile, saveSession},
			wantStatus: http.StatusCreated, wantBody: `"accessToken"`},
		{name: "register/json malformado", method: http.MethodPost, target: "/auth/register", body: `{"name":`, token: "-",
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeMalformedJSON},
		{name: "register/campo desconocido", method: http.MethodPost, target: "/auth/register", body: `{"nombre":"Ana"}`, token: "-",
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeUnknownField, wantFields: []string{"nombre/unknown_field"}},
		{name: "register/sin content-type", method: http.MethodPost, target: "/auth/register", body: register, contentType: "text/plain", token: "-",
			wantStatus: http.StatusUnsupportedMediaType, wantCode: httpx.CodeUnsupportedMediaType},
		{name: "register/reglas", method: http.MethodPost, target: "/auth/register", token: "-",
			body:       `{"name":" ","email":"sin-arroba","password":"corta"}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"name/required", "email/format", "password/min"}},
		{name: "register/email duplicado", method: http.MethodPost, target: "/auth/register", body: register, token: "-",
			rules:      []rule{qFail(insertUser, pgError("23505", ""))},
			wantStatus: http.StatusConflict, wantCode: httpx.CodeConflict},
		{name: "register/error interno", method: http.MethodPost, target: "/auth/register", body: register, token: "-",
			rules:      []rule{qFail(insertUser, errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// POST /auth/login
		{name: "login/feliz", method: http.MethodPost, target: "/auth/login", body: login, token: "-",
			rules:      []rule{qRow(findUser, int32(7), testPasswordHash), userProfile, saveSession},
			wantStatus: http.StatusOK, wantBody: `"refreshToken"`},
		{name: "login/reglas", method: http.MethodPost, target: "/auth/login", body: `{"email":"","password":""}`, token: "-",
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"email/required", "password/required"}},
		{name: "login/usuario inexistente", method: http.MethodPost, target: "/auth/login", body: login, token: "-",
			rules:      []rule{qNoRows(findUser)},
			wantStatus: http.StatusUnauthorized, wantCode: httpx.CodeUnauthorized},
		{name: "login/contraseña incorrecta", method: http.MethodPost, target: "/auth/login", token: "-",
			body:       `{"email":"ana@mail.cl","password":"otra-clave-123"}`,
			rules:      []rule{qRow(findUser, int32(7), testPasswordHash)},
			wantStatus: http.StatusUnauthorized, wantCode: httpx.CodeUnauthorized},
		{name: "login/error interno", method: http.MethodPost, target: "/auth/login", body: login, token: "-",
			rules:      []rule{qFail(findUser, errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},
		{name: "login/usuario desactivado al emitir tokens", method: http.MethodPost, target: "/auth/login", body: login, token: "-",
			rules:      []rule{qRow(findUser, int32(7), testPasswordHash), qNoRows("SELECT name,email FROM user_account")},
			wantStatus: http.StatusNotFound, wantCode: httpx.CodeNotFound},
		{name: "login/falla al guardar la sesión", method: http.MethodPost, target: "/auth/login", body: login, token: "-",
			rules:      []rule{qRow(findUser, int32(7), testPasswordHash), userProfile, qFail("INSERT INTO user_session", errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// POST /auth/refresh
		{name: "refresh/feliz", method: http.MethodPost, target: "/auth/refresh", body: `{"refreshToken":"r"}`, token: "-",
			rules:      []rule{qRow(rotate, int32(7)), userProfile, saveSession},
			wantStatus: http.StatusOK, wantBody: `"accessToken"`},
		{name: "refresh/reglas", method: http.MethodPost, target: "/auth/refresh", body: `{}`, token: "-",
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"refreshToken/required"}},
		{name: "refresh/vencido", method: http.MethodPost, target: "/auth/refresh", body: `{"refreshToken":"r"}`, token: "-",
			rules:      []rule{qNoRows(rotate)},
			wantStatus: http.StatusUnauthorized, wantCode: httpx.CodeUnauthorized},
		{name: "refresh/error interno", method: http.MethodPost, target: "/auth/refresh", body: `{"refreshToken":"r"}`, token: "-",
			rules:      []rule{qFail(rotate, errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// POST /auth/logout
		{name: "logout/con bearer", method: http.MethodPost, target: "/auth/logout",
			rules:      []rule{qExec("DELETE FROM user_session WHERE access_token_hash", 1)},
			wantStatus: http.StatusNoContent},
		{name: "logout/con refreshToken en el body", method: http.MethodPost, target: "/auth/logout", body: `{"refreshToken":"r"}`, token: "-",
			rules:      []rule{qExec("DELETE FROM user_session WHERE access_token_hash", 1)},
			wantStatus: http.StatusNoContent},
		{name: "logout/sin sesión que cerrar", method: http.MethodPost, target: "/auth/logout", token: "-",
			wantStatus: http.StatusNoContent},
		{name: "logout/json malformado", method: http.MethodPost, target: "/auth/logout", body: `{"refreshToken":`, token: "-",
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeMalformedJSON},
		{name: "logout/error interno", method: http.MethodPost, target: "/auth/logout",
			rules:      []rule{qFail("DELETE FROM user_session", errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// GET /auth/validate
		{name: "validate/feliz", method: http.MethodGet, target: "/auth/validate",
			rules:      []rule{qRow(validate, int32(7))},
			wantStatus: http.StatusOK, wantBody: `"valid":true`},
		{name: "validate/sin token", method: http.MethodGet, target: "/auth/validate", token: "-",
			wantStatus: http.StatusUnauthorized, wantCode: httpx.CodeUnauthorized},
		{name: "validate/vencido", method: http.MethodGet, target: "/auth/validate",
			rules:      []rule{qNoRows(validate)},
			wantStatus: http.StatusUnauthorized, wantCode: httpx.CodeUnauthorized},
		{name: "validate/error interno", method: http.MethodGet, target: "/auth/validate",
			rules:      []rule{qFail(validate, errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},
	})
}

func TestCheckPassword(t *testing.T) {
	if !checkPassword(testPasswordHash, testPassword) {
		t.Fatal("la contraseña correcta no valida")
	}
	if checkPassword(testPasswordHash, strings.ToUpper(testPassword)) {
		t.Fatal("una contraseña distinta valida")
	}
	if checkPassword("md5$abc", testPassword) {
		t.Fatal("un hash con otro formato valida")
	}
}

// Reglas de #59 (TAL-49..57) sobre el formato de #56 (TAL-78): límite de body
// de auth, email con net/mail y largos máximos, todo antes de tocar la base.
func TestRegisterValidatesJSONAndEmailBeforeDatabaseAccess(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		contentType string
		wantStatus  int
		wantCode    string
	}{
		{"invalid email", `{"name":"Ana","email":"no-es-email","password":"Clave12345!"}`, "application/json", http.StatusUnprocessableEntity, httpx.CodeValidationFailed},
		{"email with display name", `{"name":"Ana","email":"Ana <ana@example.test>","password":"Clave12345!"}`, "application/json", http.StatusUnprocessableEntity, httpx.CodeValidationFailed},
		{"name too long", `{"name":"` + strings.Repeat("a", maxNameLength+1) + `","email":"ana@example.test","password":"Clave12345!"}`, "application/json", http.StatusUnprocessableEntity, httpx.CodeValidationFailed},
		{"password too long", `{"name":"Ana","email":"ana@example.test","password":"` + strings.Repeat("x", maxPasswordBytes+1) + `"}`, "application/json", http.StatusUnprocessableEntity, httpx.CodeValidationFailed},
		{"unsupported content type", `{"name":"Ana"}`, "text/plain", http.StatusUnsupportedMediaType, httpx.CodeUnsupportedMediaType},
		{"oversized body", `{"name":"Ana","email":"ana@example.test","password":"` + strings.Repeat("x", maxAuthBodyBytes) + `"}`, "application/json", http.StatusRequestEntityTooLarge, httpx.CodeBodyTooLarge},
		{"trailing JSON", `{"name":"Ana","email":"ana@example.test","password":"Clave12345!"} {}`, "application/json", http.StatusBadRequest, httpx.CodeMalformedJSON},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(test.body))
			request.Header.Set("Content-Type", test.contentType)
			recorder := httptest.NewRecorder()
			httpx.RequestID(http.HandlerFunc((&Handler{}).Register)).ServeHTTP(recorder, request)

			if recorder.Code != test.wantStatus || recorder.Header().Get("Content-Type") != httpx.ProblemContentType {
				t.Fatalf("response = %d %q %s", recorder.Code, recorder.Header().Get("Content-Type"), recorder.Body.String())
			}
			var problem httpx.Problem
			if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil || problem.Code != test.wantCode {
				t.Fatalf("problem = %+v (%v)", problem, err)
			}
		})
	}
}
