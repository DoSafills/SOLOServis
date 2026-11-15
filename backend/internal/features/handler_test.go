package features

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
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

const (
	productUUID = "11111111-2222-3333-4444-555555555555"
	serviceUUID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	// userToken es el access token de la sesión del usuario 7.
	userToken = "token-de-usuario"
)

// serviceToken es el INTERNAL_API_TOKEN de los tests (mínimo 32 caracteres).
var serviceToken = strings.Repeat("s", authz.MinServiceTokenLength)

// errBoom simula un error de base de datos con detalle interno.
var errBoom = errors.New(`relation "cart_item" does not exist`)

// Reglas comunes: el usuario de la sesión, un producto y un servicio por UUID.
var (
	sessionUser = qRow("FROM user_account WHERE id = $1", int32(7))
	productRow  = qRow("FROM product WHERE public_id", int32(10))
	serviceRow  = qRow("SELECT id FROM service WHERE public_id", int32(20))
)

// --- base falsa -------------------------------------------------------------

// rule es la respuesta del fake a las consultas cuyo SQL contiene sql.
type rule struct {
	sql      string
	row      []any    // valores de QueryRow(...).Scan
	rows     []string // filas JSON de Query (QueryJSON)
	affected int64    // filas afectadas de Exec
	err      error
}

func qRow(sql string, values ...any) rule         { return rule{sql: sql, row: values} }
func qRows(sql string, values ...string) rule     { return rule{sql: sql, rows: values} }
func qExec(sql string, affected int64) rule       { return rule{sql: sql, affected: affected} }
func qFail(sql string, err error) rule            { return rule{sql: sql, err: err} }
func qNoRows(sql string) rule                     { return qFail(sql, pgx.ErrNoRows) }
func pgError(code, detail string) *pgconn.PgError { return &pgconn.PgError{Code: code, Detail: detail} }

// fakeDB responde cada consulta con la primera regla cuyo fragmento aparece en
// el SQL. Una consulta sin regla hace fallar el test: cada caso declara
// exactamente qué toca de la base. Hace también de transacción (Begin
// devuelve el mismo fake); los métodos de pgx.Tx que no se usan quedan sin
// implementar.
type fakeDB struct {
	pgx.Tx
	t         *testing.T
	rules     []rule
	commitErr error
	pingErr   error

	// Última consulta recibida, para probar que la entrada del cliente viaja
	// como argumento y nunca se concatena al SQL.
	lastSQL  string
	lastArgs []any
}

func (f *fakeDB) match(sql string) rule {
	for _, r := range f.rules {
		if strings.Contains(sql, r.sql) {
			return r
		}
	}
	f.t.Errorf("consulta sin regla en el fake: %s", sql)
	return rule{err: errors.New("consulta inesperada")}
}

func (f *fakeDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.lastSQL, f.lastArgs = sql, args
	r := f.match(sql)
	return pgconn.NewCommandTag(fmt.Sprintf("UPDATE %d", r.affected)), r.err
}

func (f *fakeDB) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	f.lastSQL, f.lastArgs = sql, args
	r := f.match(sql)
	if r.err != nil {
		return nil, r.err
	}
	return &fakeRows{values: r.rows}, nil
}

func (f *fakeDB) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	f.lastSQL, f.lastArgs = sql, args
	return fakeRow(f.match(sql))
}

func (f *fakeDB) Begin(context.Context) (pgx.Tx, error) { return f, nil }
func (f *fakeDB) Commit(context.Context) error          { return f.commitErr }
func (f *fakeDB) Rollback(context.Context) error        { return nil }
func (f *fakeDB) Ping(context.Context) error            { return f.pingErr }

type fakeRow rule

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, d := range dest {
		target := reflect.ValueOf(d).Elem()
		target.Set(reflect.ValueOf(r.row[i]).Convert(target.Type()))
	}
	return nil
}

type fakeRows struct {
	pgx.Rows
	values []string
	next   int
}

func (r *fakeRows) Next() bool { r.next++; return r.next <= len(r.values) }
func (r *fakeRows) Close()     {}
func (r *fakeRows) Err() error { return nil }

func (r *fakeRows) Scan(dest ...any) error {
	*dest[0].(*[]byte) = []byte(r.values[r.next-1])
	return nil
}

// fakeSessions reconoce solo userToken: el usuario 7 sin rol admin.
type fakeSessions struct{}

func (fakeSessions) SessionForAccessToken(_ context.Context, hash []byte) (authz.Session, error) {
	if bytes.Equal(hash, tokenHash(userToken)) {
		return authz.Session{UserID: 7, Role: "user"}, nil
	}
	return authz.Session{}, authz.ErrNoSession
}

// --- arnés por endpoint -----------------------------------------------------

// apiCase es un request a una API de features contra la base falsa. Los
// casos mínimos del estándar TAL-78 §5 (feliz, entrada inválida, inexistente
// y error interno) se escriben como filas de esta tabla.
type apiCase struct {
	name        string
	method      string
	target      string
	body        string // JSON; vacío = sin body
	contentType string // por defecto application/json si hay body
	token       string // por defecto userToken; "-" = sin Authorization
	rules       []rule
	commitErr   error
	remote      map[string]string // path → JSON de las otras APIs; un path ausente responde 503

	wantStatus int
	wantCode   string   // code del problem; vacío en respuestas exitosas
	wantFields []string // "campo/code" de errors[], en orden
	wantBody   string   // fragmento del body exitoso
}

func runCases(t *testing.T, api string, cases []apiCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, ok := tc.remote[r.URL.Path]
				if !ok {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(body))
			}))
			defer remote.Close()
			for _, env := range []string{"PRODUCT_API_URL", "PRICING_API_URL", "STORE_API_URL", "SERVICE_API_URL"} {
				t.Setenv(env, remote.URL)
			}

			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, nil))
			db := &fakeDB{t: t, rules: tc.rules, commitErr: tc.commitErr}
			h := newHandler(api, db, logger, authz.New(fakeSessions{}, "", logger).RequireSession)
			r := chi.NewRouter()
			r.Use(httpx.RequestID)
			registerRoutes(r, api, h, authz.New(fakeSessions{}, serviceToken, logger).RequireAdmin)

			var body *strings.Reader
			if tc.body != "" {
				body = strings.NewReader(tc.body)
			} else {
				body = strings.NewReader("")
			}
			req := httptest.NewRequest(tc.method, tc.target, body)
			if tc.body != "" {
				contentType := tc.contentType
				if contentType == "" {
					contentType = "application/json"
				}
				req.Header.Set("Content-Type", contentType)
			}
			switch tc.token {
			case "-":
			case "":
				req.Header.Set("Authorization", "Bearer "+userToken)
			default:
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, quiero %d (%s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantCode == "" {
				checkSuccess(t, rec, tc.wantBody)
				return
			}
			p := decodeProblem(t, rec)
			if p.Code != tc.wantCode {
				t.Fatalf("code = %q, quiero %q (%s)", p.Code, tc.wantCode, rec.Body.String())
			}
			if tc.wantFields != nil {
				if got := fieldCodes(p); !reflect.DeepEqual(got, tc.wantFields) {
					t.Fatalf("errors = %v, quiero %v", got, tc.wantFields)
				}
			}
			if rec.Code >= http.StatusInternalServerError {
				if strings.Contains(rec.Body.String(), "relation") {
					t.Fatalf("la respuesta expone el error interno: %s", rec.Body.String())
				}
				if !strings.Contains(logs.String(), p.RequestID) {
					t.Fatalf("el log no registra el error con su request_id: %s", logs.String())
				}
			}
		})
	}
}

func checkSuccess(t *testing.T, rec *httptest.ResponseRecorder, wantBody string) {
	t.Helper()
	if rec.Code == http.StatusNoContent {
		if rec.Body.Len() != 0 {
			t.Fatalf("204 con body: %s", rec.Body.String())
		}
		return
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, quiero application/json", got)
	}
	if !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("body no es JSON: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), wantBody) {
		t.Fatalf("body = %s, quiero que contenga %s", rec.Body.String(), wantBody)
	}
}

func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder) httpx.Problem {
	t.Helper()
	if got := rec.Header().Get("Content-Type"); got != httpx.ProblemContentType {
		t.Fatalf("Content-Type = %q, quiero problem+json (%s)", got, rec.Body.String())
	}
	var p httpx.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("body no es un Problem: %v (%s)", err, rec.Body.String())
	}
	if p.RequestID == "" || p.RequestID != rec.Header().Get(httpx.RequestIDHeader) {
		t.Fatalf("request_id = %q, header = %q", p.RequestID, rec.Header().Get(httpx.RequestIDHeader))
	}
	return p
}

func fieldCodes(p httpx.Problem) []string {
	out := make([]string, 0, len(p.Errors))
	for _, e := range p.Errors {
		out = append(out, e.Field+"/"+e.Code)
	}
	return out
}

// remotes arma las respuestas de las otras APIs a partir de pares path, JSON.
func remotes(pairs ...string) map[string]string {
	out := make(map[string]string, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		out[pairs[i]] = pairs[i+1]
	}
	return out
}

// --- helpers compartidos ----------------------------------------------------

func TestDBError_MapeaSegunTAL78(t *testing.T) {
	timeout := &net.OpError{Op: "dial", Err: errors.New("i/o timeout")}
	tests := []struct {
		name       string
		err        error
		status     int
		wantStatus int
		wantCode   string
		wantFields []string
	}{
		{"sin fila usa el status pedido", pgx.ErrNoRows, http.StatusNotFound, 404, httpx.CodeNotFound, nil},
		{"sin fila en auth es 401", pgx.ErrNoRows, http.StatusUnauthorized, 401, httpx.CodeUnauthorized, nil},
		{"err nil (0 filas afectadas)", nil, http.StatusNotFound, 404, httpx.CodeNotFound, nil},
		{"err nil con status 5xx es interno", nil, http.StatusInternalServerError, 500, httpx.CodeInternalError, nil},
		{"id mal formado es 400", errInvalidID, http.StatusNotFound, 400, httpx.CodeInvalidParameter, nil},
		{"unique es 409", pgError("23505", ""), http.StatusInternalServerError, 409, httpx.CodeConflict, nil},
		{"FK es 422 con el campo", pgError("23503", `Key (user_id)=(99) is not present in table "user_account".`),
			http.StatusInternalServerError, 422, httpx.CodeValidationFailed, []string{"userId/not_found"}},
		{"check es 422", &pgconn.PgError{Code: "23514", ColumnName: "target_price"},
			http.StatusInternalServerError, 422, httpx.CodeValidationFailed, []string{"targetPrice/format"}},
		{"base caída es 503", timeout, http.StatusNotFound, 503, httpx.CodeServiceUnavailable, nil},
		{"otro error es 500, nunca 404", errBoom, http.StatusNotFound, 500, httpx.CodeInternalError, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			h := &Handler{logger: slog.New(slog.NewJSONHandler(&logs, nil))}
			r := chi.NewRouter()
			r.Use(httpx.RequestID)
			r.Get("/", func(w http.ResponseWriter, r *http.Request) { h.dbError(w, r, tc.err, tc.status, "mensaje") })
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, quiero %d", rec.Code, tc.wantStatus)
			}
			p := decodeProblem(t, rec)
			if p.Code != tc.wantCode {
				t.Fatalf("code = %q, quiero %q", p.Code, tc.wantCode)
			}
			if tc.wantFields != nil && !reflect.DeepEqual(fieldCodes(p), tc.wantFields) {
				t.Fatalf("errors = %v, quiero %v", fieldCodes(p), tc.wantFields)
			}
			if rec.Code >= 500 && !strings.Contains(logs.String(), p.RequestID) {
				t.Fatalf("el 5xx no quedó en el log: %s", logs.String())
			}
		})
	}
}

func TestHealth(t *testing.T) {
	for _, tc := range []struct {
		name    string
		pingErr error
		want    int
	}{
		{"base conectada", nil, http.StatusOK},
		{"base caída", errBoom, http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHandler("cart-api", &fakeDB{t: t, pingErr: tc.pingErr}, slog.Default(), nil)
			rec := httptest.NewRecorder()
			h.Health(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
			if rec.Code != tc.want {
				t.Fatalf("status = %d, quiero %d", rec.Code, tc.want)
			}
		})
	}
}
