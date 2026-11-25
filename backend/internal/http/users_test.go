package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/DoSafills/SOLOServis/backend/internal/authz"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const testPublicID = "3f1b2c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d"

// errProfileBoom simula una falla de PostgreSQL en los tests.
var errProfileBoom = errors.New("boom")

// fakeProfileRow imita pgx.Row: Scan copia los valores declarados en el test.
type fakeProfileRow struct {
	values []any
	err    error
}

// Scan copia values en dest, como pgx.Row. Devuelve err si el caso simula una
// falla (por ejemplo pgx.ErrNoRows).
func (r fakeProfileRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, d := range dest {
		target := reflect.ValueOf(d).Elem()
		target.Set(reflect.ValueOf(r.values[i]).Convert(target.Type()))
	}
	return nil
}

// fakeProfileDB responde QueryRow con la fila del caso y guarda la última
// consulta para comprobar que los datos viajan parametrizados.
type fakeProfileDB struct {
	row      []any
	rowErr   error
	affected int64
	execErr  error
	lastSQL  string
	lastArgs []any
}

func (f *fakeProfileDB) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	f.lastSQL, f.lastArgs = sql, args
	return fakeProfileRow{values: f.row, err: f.rowErr}
}

func (f *fakeProfileDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.lastSQL, f.lastArgs = sql, args
	return pgconn.NewCommandTag(fmt.Sprintf("UPDATE %d", f.affected)), f.execErr
}

// fakeProfileSessions acepta cualquier bearer token como el usuario 7.
type fakeProfileSessions struct{}

func (fakeProfileSessions) SessionForAccessToken(context.Context, []byte) (authz.Session, error) {
	return authz.Session{UserID: 7, Role: "client"}, nil
}

// profileTestRouter monta los endpoints de perfil con una sesión falsa, como
// los monta registerUserRoutes pero sin PostgreSQL real.
func profileTestRouter(t *testing.T, db profileDB) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	h := newProfileHandler(db, logger)
	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	session := authz.New(fakeProfileSessions{}, "", logger).RequireSession
	r.With(session).Patch("/users/me", h.PatchMe)
	r.With(session).Post("/users/me/avatar", h.UploadAvatar)
	r.Get("/users/{publicID}/avatar", h.GetAvatar)
	return r
}

// assertProblem comprueba el status y, si se pide, el code y los campos del
// problem+json.
func assertProblem(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode string, wantFields []string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, quiero %d (%s)", rec.Code, wantStatus, rec.Body.String())
	}
	if wantCode == "" {
		return
	}
	var p httpx.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("body no es un Problem: %v (%s)", err, rec.Body.String())
	}
	if p.Code != wantCode {
		t.Fatalf("code = %q, quiero %q (%s)", p.Code, wantCode, rec.Body.String())
	}
	if wantFields == nil {
		return
	}
	got := make([]string, 0, len(p.Errors))
	for _, e := range p.Errors {
		got = append(got, e.Field+"/"+e.Code)
	}
	if !reflect.DeepEqual(got, wantFields) {
		t.Fatalf("errors = %v, quiero %v", got, wantFields)
	}
}

func TestPatchMe(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		withoutJWT bool
		db         *fakeProfileDB
		wantStatus int
		wantCode   string
		wantFields []string
		wantBody   string
	}{
		{
			name:       "feliz con avatar",
			body:       `{"name":" Ana "}`,
			db:         &fakeProfileDB{row: []any{int32(7), "Ana", "ana@mail.cl", testPublicID, true}},
			wantStatus: http.StatusOK,
			wantBody:   `"avatarUrl":"/users/` + testPublicID + `/avatar"`,
		},
		{
			name:       "feliz sin avatar",
			body:       `{"name":"Ana"}`,
			db:         &fakeProfileDB{row: []any{int32(7), "Ana", "ana@mail.cl", testPublicID, false}},
			wantStatus: http.StatusOK,
			wantBody:   `"avatarUrl":null`,
		},
		{
			name:       "nombre vacío",
			body:       `{"name":"   "}`,
			db:         &fakeProfileDB{},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   httpx.CodeValidationFailed,
			wantFields: []string{"name/required"},
		},
		{
			name:       "nombre demasiado largo",
			body:       `{"name":"` + strings.Repeat("a", maxProfileNameLength+1) + `"}`,
			db:         &fakeProfileDB{},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   httpx.CodeValidationFailed,
			wantFields: []string{"name/max_length"},
		},
		{
			name:       "sin sesión",
			body:       `{"name":"Ana"}`,
			withoutJWT: true,
			db:         &fakeProfileDB{},
			wantStatus: http.StatusUnauthorized,
			wantCode:   httpx.CodeUnauthorized,
		},
		{
			name:       "usuario inactivo",
			body:       `{"name":"Ana"}`,
			db:         &fakeProfileDB{rowErr: pgx.ErrNoRows},
			wantStatus: http.StatusNotFound,
			wantCode:   httpx.CodeNotFound,
		},
		{
			name:       "error interno",
			body:       `{"name":"Ana"}`,
			db:         &fakeProfileDB{rowErr: errProfileBoom},
			wantStatus: http.StatusInternalServerError,
			wantCode:   httpx.CodeInternalError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := profileTestRouter(t, tc.db)
			req := httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			if !tc.withoutJWT {
				req.Header.Set("Authorization", "Bearer test")
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assertProblem(t, rec, tc.wantStatus, tc.wantCode, tc.wantFields)
			if tc.wantBody != "" && !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Fatalf("body = %s, quiero que contenga %s", rec.Body.String(), tc.wantBody)
			}
		})
	}
}

// pngBytes devuelve una imagen PNG mínima reconocible por
// http.DetectContentType.
func pngBytes(extra int) []byte {
	return append([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, make([]byte, extra)...)
}

// avatarRequest arma un POST multipart con el campo avatar y los bytes dados.
// Un field vacío omite el archivo, para probar el 422 por falta de imagen.
func avatarRequest(t *testing.T, field string, data []byte, withoutJWT bool) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if field == "" {
		if err := writer.WriteField("otro", "sin avatar"); err != nil {
			t.Fatalf("WriteField: %v", err)
		}
	} else {
		part, err := writer.CreateFormFile(field, "avatar.png")
		if err != nil {
			t.Fatalf("CreateFormFile: %v", err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatalf("escribir avatar: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("cerrar multipart: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/users/me/avatar", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if !withoutJWT {
		req.Header.Set("Authorization", "Bearer test")
	}
	return req
}

func TestUploadAvatar(t *testing.T) {
	image := pngBytes(64)
	tests := []struct {
		name       string
		field      string
		data       []byte
		withoutJWT bool
		jsonBody   bool
		db         *fakeProfileDB
		wantStatus int
		wantCode   string
		wantFields []string
	}{
		{
			name:       "feliz",
			field:      "avatar",
			data:       image,
			db:         &fakeProfileDB{row: []any{testPublicID}},
			wantStatus: http.StatusOK,
		},
		{
			name:       "sin archivo",
			field:      "",
			db:         &fakeProfileDB{},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   httpx.CodeValidationFailed,
			wantFields: []string{"avatar/required"},
		},
		{
			name:       "no es imagen",
			field:      "avatar",
			data:       []byte("esto no es una imagen"),
			db:         &fakeProfileDB{},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   httpx.CodeValidationFailed,
			wantFields: []string{"avatar/format"},
		},
		{
			name:       "supera 2 MB",
			field:      "avatar",
			data:       pngBytes(maxAvatarBytes),
			db:         &fakeProfileDB{},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   httpx.CodeValidationFailed,
			wantFields: []string{"avatar/max_size"},
		},
		{
			name:       "content-type json",
			jsonBody:   true,
			db:         &fakeProfileDB{},
			wantStatus: http.StatusUnsupportedMediaType,
			wantCode:   httpx.CodeUnsupportedMediaType,
		},
		{
			name:       "sin sesión",
			field:      "avatar",
			data:       image,
			withoutJWT: true,
			db:         &fakeProfileDB{},
			wantStatus: http.StatusUnauthorized,
			wantCode:   httpx.CodeUnauthorized,
		},
		{
			name:       "usuario inactivo",
			field:      "avatar",
			data:       image,
			db:         &fakeProfileDB{rowErr: pgx.ErrNoRows},
			wantStatus: http.StatusNotFound,
			wantCode:   httpx.CodeNotFound,
		},
		{
			name:       "error interno",
			field:      "avatar",
			data:       image,
			db:         &fakeProfileDB{rowErr: errProfileBoom},
			wantStatus: http.StatusInternalServerError,
			wantCode:   httpx.CodeInternalError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := profileTestRouter(t, tc.db)
			var req *http.Request
			if tc.jsonBody {
				req = httptest.NewRequest(http.MethodPost, "/users/me/avatar", strings.NewReader(`{"avatar":"x"}`))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer test")
			} else {
				req = avatarRequest(t, tc.field, tc.data, tc.withoutJWT)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assertProblem(t, rec, tc.wantStatus, tc.wantCode, tc.wantFields)
			if tc.wantStatus == http.StatusOK {
				if !strings.Contains(rec.Body.String(), `"avatarUrl":"/users/`+testPublicID+`/avatar"`) {
					t.Fatalf("body = %s", rec.Body.String())
				}
				if len(tc.db.lastArgs) != 3 {
					t.Fatalf("args = %v", tc.db.lastArgs)
				}
				if got, ok := tc.db.lastArgs[0].([]byte); !ok || !bytes.Equal(got, tc.data) {
					t.Fatal("la imagen no viajó como bytea parametrizado")
				}
				if got := tc.db.lastArgs[1]; got != "image/png" {
					t.Fatalf("mime = %v, quiero image/png", got)
				}
			}
		})
	}
}

func TestGetAvatar(t *testing.T) {
	image := pngBytes(64)
	etag := avatarETag(image)

	t.Run("feliz con cache", func(t *testing.T) {
		db := &fakeProfileDB{row: []any{image, "image/png"}}
		router := profileTestRouter(t, db)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/"+testPublicID+"/avatar", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("ETag"); got != etag {
			t.Fatalf("ETag = %q, quiero %q", got, etag)
		}
		if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "public") {
			t.Fatalf("Cache-Control = %q", got)
		}
		if got := rec.Header().Get("Content-Type"); got != "image/png" {
			t.Fatalf("Content-Type = %q", got)
		}
		if !bytes.Equal(rec.Body.Bytes(), image) {
			t.Fatal("el body no es la imagen guardada")
		}
	})

	t.Run("sin cambios", func(t *testing.T) {
		db := &fakeProfileDB{row: []any{image, "image/png"}}
		router := profileTestRouter(t, db)
		req := httptest.NewRequest(http.MethodGet, "/users/"+testPublicID+"/avatar", nil)
		req.Header.Set("If-None-Match", etag)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotModified {
			t.Fatalf("status = %d, quiero 304", rec.Code)
		}
		if rec.Body.Len() != 0 {
			t.Fatalf("304 con body: %s", rec.Body.String())
		}
	})

	t.Run("uuid inválido", func(t *testing.T) {
		router := profileTestRouter(t, &fakeProfileDB{})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/no-es-uuid/avatar", nil))
		assertProblem(t, rec, http.StatusNotFound, httpx.CodeNotFound, nil)
	})

	t.Run("sin foto", func(t *testing.T) {
		router := profileTestRouter(t, &fakeProfileDB{rowErr: pgx.ErrNoRows})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/"+testPublicID+"/avatar", nil))
		assertProblem(t, rec, http.StatusNotFound, httpx.CodeNotFound, nil)
	})

	t.Run("error interno", func(t *testing.T) {
		router := profileTestRouter(t, &fakeProfileDB{rowErr: errProfileBoom})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/"+testPublicID+"/avatar", nil))
		assertProblem(t, rec, http.StatusInternalServerError, httpx.CodeInternalError, nil)
	})
}
