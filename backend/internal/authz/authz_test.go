package authz

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
)

const serviceToken = "token-interno-de-prueba-con-32-caracteres"

// fakeSessions asocia tokens (en claro, para legibilidad) a roles.
type fakeSessions struct {
	roles map[string]string
	err   error
}

func (f fakeSessions) SessionForAccessToken(_ context.Context, hash []byte) (Session, error) {
	if f.err != nil {
		return Session{}, f.err
	}
	for token, role := range f.roles {
		h := sha256.Sum256([]byte(token))
		if bytes.Equal(h[:], hash) {
			// El largo del token sirve como userID determinista en los tests.
			return Session{UserID: int32(len(token)), Role: role}, nil
		}
	}
	return Session{}, ErrNoSession
}

func serve(t *testing.T, sessions SessionStore, token, authorization string) (*httptest.ResponseRecorder, bool, string) {
	t.Helper()
	var logs bytes.Buffer
	a := New(sessions, token, slog.New(slog.NewJSONHandler(&logs, nil)))

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/products", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	httpx.RequestID(a.RequireAdmin(next)).ServeHTTP(rec, req)
	return rec, called, logs.String()
}

func TestRequireAdmin(t *testing.T) {
	sessions := fakeSessions{roles: map[string]string{"tok-admin": "admin", "tok-cliente": "client"}}

	tests := []struct {
		name          string
		serviceToken  string
		authorization string
		wantStatus    int
		wantCode      string
	}{
		{"sesión admin", "", "Bearer tok-admin", http.StatusNoContent, ""},
		{"esquema sin distinguir mayúsculas", "", "bearer tok-admin", http.StatusNoContent, ""},
		{"token de servicio", serviceToken, "Bearer " + serviceToken, http.StatusNoContent, ""},
		{"sin header", serviceToken, "", http.StatusUnauthorized, httpx.CodeUnauthorized},
		{"esquema Basic", serviceToken, "Basic dXNlcjpwYXNz", http.StatusUnauthorized, httpx.CodeUnauthorized},
		{"Bearer vacío", serviceToken, "Bearer   ", http.StatusUnauthorized, httpx.CodeUnauthorized},
		{"token desconocido", serviceToken, "Bearer otro", http.StatusUnauthorized, httpx.CodeUnauthorized},
		{"rol cliente", serviceToken, "Bearer tok-cliente", http.StatusForbidden, httpx.CodeForbidden},
		{"token de servicio corto se ignora", "corto", "Bearer corto", http.StatusUnauthorized, httpx.CodeUnauthorized},
		{"sin token de servicio configurado", "", "Bearer " + serviceToken, http.StatusUnauthorized, httpx.CodeUnauthorized},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec, called, _ := serve(t, sessions, tc.serviceToken, tc.authorization)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, quiero %d (%s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantCode == "" {
				if !called {
					t.Fatal("el handler protegido debía ejecutarse")
				}
				return
			}
			if called {
				t.Fatal("el handler protegido no debía ejecutarse")
			}
			var p httpx.Problem
			if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || p.Code != tc.wantCode {
				t.Fatalf("problem = %+v (%v)", p, err)
			}
			if tc.wantStatus == http.StatusUnauthorized && rec.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatal("un 401 debe incluir WWW-Authenticate: Bearer")
			}
		})
	}
}

func TestRequireAdmin_ErrorDeSesionesNoExponeDetalles(t *testing.T) {
	rec, called, logs := serve(t, fakeSessions{err: errors.New("dial tcp 10.0.0.5:5432: refused")}, "", "Bearer tok")

	if rec.Code != http.StatusInternalServerError || called {
		t.Fatalf("status = %d, called = %v", rec.Code, called)
	}
	if strings.Contains(rec.Body.String(), "10.0.0.5") || !strings.Contains(logs, "10.0.0.5") {
		t.Fatalf("el detalle debe ir al log y no al body: body=%s", rec.Body.String())
	}
}

func TestRequireSession(t *testing.T) {
	sessions := fakeSessions{roles: map[string]string{"tok-cliente": "client"}}
	a := New(sessions, serviceToken, slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)))

	var got Session
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = SessionFrom(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})

	for _, tc := range []struct {
		name, authorization string
		want                int
	}{
		{"sesión de cliente", "Bearer tok-cliente", http.StatusNoContent},
		{"sin token", "", http.StatusUnauthorized},
		{"token desconocido", "Bearer otro", http.StatusUnauthorized},
		// El token de servicio no representa a un usuario.
		{"token de servicio", "Bearer " + serviceToken, http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got = Session{}
			req := httptest.NewRequest(http.MethodGet, "/comparisons/products", nil)
			if tc.authorization != "" {
				req.Header.Set("Authorization", tc.authorization)
			}
			rec := httptest.NewRecorder()
			httpx.RequestID(a.RequireSession(next)).ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, quiero %d", rec.Code, tc.want)
			}
			if tc.want == http.StatusNoContent && (got.UserID != int32(len("tok-cliente")) || got.Role != "client") {
				t.Fatalf("sesión en el contexto = %+v", got)
			}
		})
	}

	if _, ok := SessionFrom(context.Background()); ok {
		t.Fatal("sin RequireSession no hay sesión en el contexto")
	}
}
