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

func TestRegisterValidatesJSONAndEmailBeforeDatabaseAccess(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		contentType string
		wantStatus  int
		wantCode    string
	}{
		{"invalid email", `{"name":"Ana","email":"no-es-email","password":"Clave12345!"}`, "application/json", http.StatusBadRequest, httpx.CodeInvalidParameter},
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
