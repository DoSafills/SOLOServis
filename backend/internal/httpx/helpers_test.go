package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// decodeProblem lee un Problem de la respuesta y verifica el Content-Type.
func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder) Problem {
	t.Helper()
	if got := rec.Header().Get("Content-Type"); got != ProblemContentType {
		t.Fatalf("Content-Type = %q, quiero %q", got, ProblemContentType)
	}
	var p Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("body no es un Problem válido: %v (%s)", err, rec.Body.String())
	}
	return p
}

// withRequestID ejecuta h detrás del middleware RequestID.
func withRequestID(h http.HandlerFunc, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	RequestID(h).ServeHTTP(rec, req)
	return rec
}

func TestWriteProblem_Formato(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/products/abc?token=secreto", nil)
	req.Header.Set(RequestIDHeader, "req-123")

	rec := withRequestID(func(w http.ResponseWriter, r *http.Request) {
		WriteProblem(w, r, http.StatusNotFound, CodeNotFound, "El producto no existe.")
	}, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, quiero 404", rec.Code)
	}
	p := decodeProblem(t, rec)
	want := Problem{
		Type: "about:blank", Title: "Not Found", Status: 404, Detail: "El producto no existe.",
		Instance: "/products/abc", Code: CodeNotFound, RequestID: "req-123",
	}
	if p.Type != want.Type || p.Title != want.Title || p.Status != want.Status || p.Detail != want.Detail ||
		p.Instance != want.Instance || p.Code != want.Code || p.RequestID != want.RequestID {
		t.Fatalf("problem = %+v, quiero %+v", p, want)
	}
	if strings.Contains(rec.Body.String(), "\"errors\"") {
		t.Fatalf("errors debe omitirse si está vacío: %s", rec.Body.String())
	}
}

func TestWriteValidation_ErroresPorCampo(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/products", nil)
	fieldErrors := []FieldError{
		{Field: "name", Code: "required", Message: "name es obligatorio"},
		{Field: "categoryId", Code: "min", Message: "categoryId debe ser mayor que 0"},
	}

	rec := withRequestID(func(w http.ResponseWriter, r *http.Request) {
		WriteValidation(w, r, fieldErrors)
	}, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, quiero 422", rec.Code)
	}
	p := decodeProblem(t, rec)
	if p.Code != CodeValidationFailed || len(p.Errors) != 2 || p.Errors[0] != fieldErrors[0] || p.Errors[1] != fieldErrors[1] {
		t.Fatalf("problem = %+v", p)
	}
	if p.RequestID == "" {
		t.Fatal("request_id vacío")
	}
}

func TestWriteInternal_NoExponeDetalles(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	secret := errors.New(`pq: password authentication failed for user "postgres" at postgres://postgres:postgres@db:5432`)
	req := httptest.NewRequest(http.MethodGet, "/stores/1", nil)

	rec := withRequestID(func(w http.ResponseWriter, r *http.Request) {
		WriteInternal(w, r, logger, secret)
	}, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, quiero 500", rec.Code)
	}
	body := rec.Body.String()
	for _, leak := range []string{"password", "postgres", "pq:"} {
		if strings.Contains(body, leak) {
			t.Fatalf("la respuesta expone %q: %s", leak, body)
		}
	}
	p := decodeProblem(t, rec)
	if p.Code != CodeInternalError || p.RequestID == "" || p.RequestID != rec.Header().Get(RequestIDHeader) {
		t.Fatalf("problem = %+v", p)
	}

	var entry map[string]any
	if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
		t.Fatalf("log no es JSON: %v (%s)", err, logs.String())
	}
	if entry["level"] != "ERROR" || entry["error"] != secret.Error() || entry["request_id"] != p.RequestID {
		t.Fatalf("log = %v", entry)
	}
}

func TestValidator(t *testing.T) {
	tests := []struct {
		name     string
		run      func(v *Validator)
		wantErrs []FieldError
	}{
		{
			name:     "sin errores",
			run:      func(v *Validator) { v.Required("name", "TV"); v.MaxLength("name", "TV", 10) },
			wantErrs: nil,
		},
		{
			name:     "required con solo espacios",
			run:      func(v *Validator) { v.Required("name", "   ") },
			wantErrs: []FieldError{{"name", "required", "name es obligatorio"}},
		},
		{
			name:     "max_length cuenta caracteres, no bytes",
			run:      func(v *Validator) { v.MaxLength("name", "ñññ", 3); v.MaxLength("model", "abcd", 3) },
			wantErrs: []FieldError{{"model", "max_length", "model no puede superar 3 caracteres"}},
		},
		{
			name: "check y add acumulan en orden",
			run: func(v *Validator) {
				v.Check(false, "categoryId", "min", "categoryId debe ser mayor que 0")
				v.Check(true, "brandId", "min", "no debe aparecer")
				v.Add("imageUrl", "format", "imageUrl debe ser http(s)")
			},
			wantErrs: []FieldError{
				{"categoryId", "min", "categoryId debe ser mayor que 0"},
				{"imageUrl", "format", "imageUrl debe ser http(s)"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var v Validator
			tc.run(&v)
			got := v.Errors()
			if v.Valid() != (len(tc.wantErrs) == 0) || len(got) != len(tc.wantErrs) {
				t.Fatalf("errores = %+v, quiero %+v", got, tc.wantErrs)
			}
			for i := range got {
				if got[i] != tc.wantErrs[i] {
					t.Fatalf("errores[%d] = %+v, quiero %+v", i, got[i], tc.wantErrs[i])
				}
			}
		})
	}
}

func TestRequestID(t *testing.T) {
	tests := []struct {
		name       string
		header     string
		wantReused bool
	}{
		{"sin header genera uno", "", false},
		{"header válido se respeta", "abc-DEF-123", true},
		{"largo máximo exacto se respeta", strings.Repeat("a", MaxRequestIDLength), true},
		{"largo excesivo se reemplaza", strings.Repeat("a", MaxRequestIDLength+1), false},
		{"salto de línea se reemplaza", "abc\r\nSet-Cookie: x=1", false},
		{"inyección en logs se reemplaza", "abc\n{\"level\":\"ERROR\"}", false},
		{"caracteres fuera del charset se reemplazan", "abc_def.ghi", false},
		{"unicode se reemplaza", "ñandú", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.header != "" {
				// Header map directo: Header.Set no valida, igual que un cliente malicioso.
				req.Header[http.CanonicalHeaderKey(RequestIDHeader)] = []string{tc.header}
			}
			var seen string
			rec := withRequestID(func(w http.ResponseWriter, r *http.Request) {
				seen = RequestIDFrom(r.Context())
			}, req)

			got := rec.Header().Get(RequestIDHeader)
			if got != seen {
				t.Fatalf("header %q distinto del contexto %q", got, seen)
			}
			if !ValidRequestID(got) {
				t.Fatalf("request_id devuelto inválido: %q", got)
			}
			if tc.wantReused && got != tc.header {
				t.Fatalf("request_id = %q, quiero %q", got, tc.header)
			}
			if !tc.wantReused && got == tc.header {
				t.Fatalf("debió reemplazarse %q", tc.header)
			}
		})
	}
}

func TestRequestID_GeneraValoresDistintos(t *testing.T) {
	a := withRequestID(func(http.ResponseWriter, *http.Request) {}, httptest.NewRequest(http.MethodGet, "/", nil))
	b := withRequestID(func(http.ResponseWriter, *http.Request) {}, httptest.NewRequest(http.MethodGet, "/", nil))
	idA, idB := a.Header().Get(RequestIDHeader), b.Header().Get(RequestIDHeader)
	if len(idA) != 32 || idA == idB {
		t.Fatalf("ids generados = %q, %q", idA, idB)
	}
}
