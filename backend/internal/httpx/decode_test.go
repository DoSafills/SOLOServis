package httpx

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type decodeTarget struct {
	Name       string `json:"name"`
	CategoryID int32  `json:"categoryId"`
}

func TestDecodeJSON(t *testing.T) {
	const limit = 64

	tests := []struct {
		name        string
		contentType string
		body        string
		maxBytes    int64
		wantStatus  int // 0 = sin error
		wantCode    string
		wantField   string
	}{
		{name: "caso feliz", contentType: "application/json", body: `{"name":"TV","categoryId":3}`},
		{name: "content-type con charset", contentType: "application/json; charset=utf-8", body: `{"name":"TV"}`},
		{name: "content-type en mayúsculas", contentType: "Application/JSON", body: `{"name":"TV"}`},
		{name: "body exactamente en el límite", contentType: "application/json", body: `{"name":"` + strings.Repeat("a", limit-11) + `"}`, maxBytes: limit},
		{name: "body excedido", contentType: "application/json", body: `{"name":"` + strings.Repeat("a", 200) + `"}`, maxBytes: limit, wantStatus: 413, wantCode: CodeBodyTooLarge},
		{name: "datos extra que exceden el límite", contentType: "application/json", body: `{"name":"x"}` + strings.Repeat(" ", 200) + `{}`, maxBytes: limit, wantStatus: 413, wantCode: CodeBodyTooLarge},
		{name: "límite por defecto si maxBytes es 0", contentType: "application/json", body: `{"name":"` + strings.Repeat("a", int(DefaultMaxBodyBytes)) + `"}`, wantStatus: 413, wantCode: CodeBodyTooLarge},
		{name: "content-type incorrecto", contentType: "text/plain", body: `{"name":"TV"}`, wantStatus: 415, wantCode: CodeUnsupportedMediaType},
		{name: "content-type ausente", contentType: "", body: `{"name":"TV"}`, wantStatus: 415, wantCode: CodeUnsupportedMediaType},
		{name: "content-type form", contentType: "application/x-www-form-urlencoded", body: `name=TV`, wantStatus: 415, wantCode: CodeUnsupportedMediaType},
		{name: "content-type inválido", contentType: "application/json; charset", body: `{}`, wantStatus: 415, wantCode: CodeUnsupportedMediaType},
		{name: "campo desconocido", contentType: "application/json", body: `{"name":"TV","nombre":"x"}`, wantStatus: 400, wantCode: CodeUnknownField, wantField: "nombre"},
		{name: "json malformado", contentType: "application/json", body: `{"name":"TV"`, wantStatus: 400, wantCode: CodeMalformedJSON},
		{name: "json con sintaxis inválida", contentType: "application/json", body: `{name:TV}`, wantStatus: 400, wantCode: CodeMalformedJSON},
		{name: "body vacío", contentType: "application/json", body: ``, wantStatus: 400, wantCode: CodeMalformedJSON},
		{name: "datos extra después del objeto", contentType: "application/json", body: `{"name":"TV"}{"name":"x"}`, wantStatus: 400, wantCode: CodeMalformedJSON},
		{name: "basura después del objeto", contentType: "application/json", body: `{"name":"TV"} xyz`, wantStatus: 400, wantCode: CodeMalformedJSON},
		{name: "tipo incorrecto", contentType: "application/json", body: `{"name":5}`, wantStatus: 400, wantCode: CodeInvalidType, wantField: "name"},
		{name: "número fuera de rango", contentType: "application/json", body: `{"categoryId":99999999999}`, wantStatus: 400, wantCode: CodeInvalidType, wantField: "categoryId"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/products", strings.NewReader(tc.body))
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			rec := httptest.NewRecorder()

			var dst decodeTarget
			err := DecodeJSON(rec, req, &dst, tc.maxBytes)

			if tc.wantStatus == 0 {
				if err != nil {
					t.Fatalf("error inesperado: %v", err)
				}
				return
			}

			var decodeErr *DecodeError
			if !errors.As(err, &decodeErr) {
				t.Fatalf("error = %v, quiero *DecodeError", err)
			}
			if decodeErr.Status != tc.wantStatus || decodeErr.Code != tc.wantCode || decodeErr.Field != tc.wantField {
				t.Fatalf("got {status:%d code:%s field:%q}, quiero {status:%d code:%s field:%q}",
					decodeErr.Status, decodeErr.Code, decodeErr.Field, tc.wantStatus, tc.wantCode, tc.wantField)
			}
		})
	}
}

func TestDecodeJSON_AsignaValores(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/products", strings.NewReader(`{"name":"TV","categoryId":3}`))
	req.Header.Set("Content-Type", "application/json")

	var dst decodeTarget
	if err := DecodeJSON(httptest.NewRecorder(), req, &dst, 0); err != nil {
		t.Fatal(err)
	}
	if dst.Name != "TV" || dst.CategoryID != 3 {
		t.Fatalf("dst = %+v", dst)
	}
}

func TestWriteDecodeError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantErrors int
	}{
		{"413", &DecodeError{Status: 413, Code: CodeBodyTooLarge, Detail: "grande"}, 413, CodeBodyTooLarge, 0},
		{"415", &DecodeError{Status: 415, Code: CodeUnsupportedMediaType, Detail: "tipo"}, 415, CodeUnsupportedMediaType, 0},
		{"400 con campo", &DecodeError{Status: 400, Code: CodeUnknownField, Detail: "campo", Field: "nombre"}, 400, CodeUnknownField, 1},
		{"error ajeno se trata como 400", errors.New("otro"), 400, CodeMalformedJSON, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/products", nil)
			rec := withRequestID(func(w http.ResponseWriter, r *http.Request) {
				WriteDecodeError(w, r, tc.err)
			}, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, quiero %d", rec.Code, tc.wantStatus)
			}
			p := decodeProblem(t, rec)
			if p.Code != tc.wantCode || len(p.Errors) != tc.wantErrors || p.RequestID == "" {
				t.Fatalf("problem = %+v", p)
			}
			if tc.wantErrors == 1 && (p.Errors[0].Field != "nombre" || p.Errors[0].Code != CodeUnknownField) {
				t.Fatalf("errors = %+v", p.Errors)
			}
		})
	}
}
