package comparison

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/authz"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
)

const (
	uuidA = "11111111-1111-1111-1111-111111111111"
	uuidB = "22222222-2222-2222-2222-222222222222"
	uuidC = "33333333-3333-3333-3333-333333333333"
	// La sesión del token "tok-ana" es del usuario 7.
	tokenAna = "tok-ana"
	userAna  = int32(7)
)

var errBoom = errors.New(`relation "product_comparison" does not exist`)

// fakeSessions resuelve un único token de prueba.
type fakeSessions struct{}

func (fakeSessions) SessionForAccessToken(_ context.Context, hash []byte) (authz.Session, error) {
	want := sha256.Sum256([]byte(tokenAna))
	if bytes.Equal(hash, want[:]) {
		return authz.Session{UserID: userAna, Role: "client"}, nil
	}
	return authz.Session{}, authz.ErrNoSession
}

type fakeStore struct {
	products  map[string]Product
	matrix    Matrix
	summaries []Summary
	total     int64
	header    Header

	resolveErr, matrixErr, writeErr, getErr error

	gotUser     int32
	gotName     string
	gotProducts []Product
	gotIDs      []int32
	writes      int
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		products: map[string]Product{
			uuidA: {ID: 1, PublicID: uuidA, CategoryID: 5},
			uuidB: {ID: 2, PublicID: uuidB, CategoryID: 5},
			uuidC: {ID: 3, PublicID: uuidC, CategoryID: 9},
		},
		matrix: Matrix{Products: []ProductSummary{}, Attributes: []AttributeRow{}, Specifications: []SpecificationRow{}},
		header: Header{ID: 4, Name: "Mis refris", ProductIDs: []int32{1, 2}},
	}
}

func (f *fakeStore) ResolveProducts(_ context.Context, ids []string) ([]Product, error) {
	if f.resolveErr != nil {
		return nil, f.resolveErr
	}
	out := []Product{}
	for _, id := range ids {
		p, ok := f.products[id]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrProductNotFound, id)
		}
		out = append(out, p)
	}
	return out, nil
}
func (f *fakeStore) BuildMatrix(_ context.Context, ids []int32) (Matrix, error) {
	f.gotIDs = ids
	return f.matrix, f.matrixErr
}
func (f *fakeStore) Compatible(_ context.Context, p Product, limit, offset int) ([]ProductSummary, int64, error) {
	f.gotProducts = []Product{p}
	return []ProductSummary{{ID: uuidB}}, 1, f.matrixErr
}
func (f *fakeStore) Create(_ context.Context, userID int32, name string, products []Product) (int32, error) {
	f.writes++
	f.gotUser, f.gotName, f.gotProducts = userID, name, products
	return 4, f.writeErr
}
func (f *fakeStore) List(_ context.Context, userID int32, limit, offset int) ([]Summary, int64, error) {
	f.gotUser = userID
	return f.summaries, f.total, f.getErr
}
func (f *fakeStore) Get(_ context.Context, userID, id int32) (Header, error) {
	f.gotUser = userID
	return f.header, f.getErr
}
func (f *fakeStore) Rename(_ context.Context, userID, id int32, name string) error {
	f.writes++
	f.gotUser, f.gotName = userID, name
	return f.writeErr
}
func (f *fakeStore) Delete(_ context.Context, userID, id int32) error {
	f.writes++
	f.gotUser = userID
	return f.writeErr
}
func (f *fakeStore) AddItem(_ context.Context, userID, id int32, p Product) error {
	f.writes++
	f.gotUser, f.gotProducts = userID, []Product{p}
	return f.writeErr
}
func (f *fakeStore) RemoveItem(_ context.Context, userID, id, productID int32) error {
	f.writes++
	f.gotUser, f.gotIDs = userID, []int32{productID}
	return f.writeErr
}

// serve monta las rutas como registerComparisonRoutes, con RequireSession real.
func serve(t *testing.T, store Store, method, target, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	h := NewHandler(store, logger)
	session := authz.New(fakeSessions{}, "", logger).RequireSession

	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Get("/comparisons/products/compare", h.Compare)
	r.Get("/comparisons/compatible/{productId}", h.Compatible)
	r.Group(func(r chi.Router) {
		r.Use(session)
		r.Post("/comparisons/products", h.Create)
		r.Get("/comparisons/products", h.List)
		r.Get("/comparisons/products/{id}", h.Get)
		r.Put("/comparisons/products/{id}", h.Rename)
		r.Delete("/comparisons/products/{id}", h.Delete)
		r.Post("/comparisons/products/{id}/items", h.AddItem)
		r.Delete("/comparisons/products/{id}/items/{productId}", h.RemoveItem)
	})

	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func requireStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, quiero %d (%s)", rec.Code, want, rec.Body.String())
	}
}

func problem(t *testing.T, rec *httptest.ResponseRecorder) (string, []string) {
	t.Helper()
	var p httpx.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || rec.Header().Get("Content-Type") != httpx.ProblemContentType {
		t.Fatalf("no es problem+json: %s", rec.Body.String())
	}
	if p.RequestID == "" || p.RequestID != rec.Header().Get(httpx.RequestIDHeader) {
		t.Fatalf("request_id = %q", p.RequestID)
	}
	fields := []string{}
	for _, e := range p.Errors {
		fields = append(fields, e.Field+"/"+e.Code)
	}
	return p.Code, fields
}

// --------------------------------------------------------------- /compare

func TestCompare(t *testing.T) {
	store := newFakeStore()
	rec := serve(t, store, http.MethodGet, "/comparisons/products/compare?ids="+uuidB+","+strings.ToUpper(uuidA), "", "")
	requireStatus(t, rec, http.StatusOK)
	if !reflect.DeepEqual(store.gotIDs, []int32{2, 1}) {
		t.Fatalf("la matriz debe respetar el orden pedido: %v", store.gotIDs)
	}
	if !strings.Contains(rec.Body.String(), `"attributes":[]`) {
		t.Fatalf("body = %s", rec.Body.String())
	}

	for _, tc := range []struct{ query, want string }{
		{"", "ids/required"},
		{"ids=" + uuidA, "ids/min"},
		{"ids=" + strings.Repeat(uuidA+",", 5) + uuidA, "ids/max"},
		{"ids=" + uuidA + ",no-uuid", "ids/format"},
		{"ids=" + uuidA + "," + uuidA, "ids/unique"},
		{"ids=" + uuidA + "," + uuidC, "ids/incompatible_category"},
	} {
		t.Run(tc.query, func(t *testing.T) {
			rec := serve(t, newFakeStore(), http.MethodGet, "/comparisons/products/compare?"+tc.query, "", "")
			requireStatus(t, rec, http.StatusBadRequest)
			if code, fields := problem(t, rec); code != httpx.CodeInvalidParameter || !reflect.DeepEqual(fields, []string{tc.want}) {
				t.Fatalf("problem = %s %v", code, fields)
			}
		})
	}

	rec = serve(t, newFakeStore(), http.MethodGet, "/comparisons/products/compare?ids="+uuidA+",44444444-4444-4444-4444-444444444444", "", "")
	requireStatus(t, rec, http.StatusNotFound)

	broken := newFakeStore()
	broken.matrixErr = errBoom
	rec = serve(t, broken, http.MethodGet, "/comparisons/products/compare?ids="+uuidA+","+uuidB, "", "")
	requireStatus(t, rec, http.StatusInternalServerError)
	if strings.Contains(rec.Body.String(), "relation") {
		t.Fatal("el body expone el error interno")
	}
}

func TestCompatible(t *testing.T) {
	store := newFakeStore()
	rec := serve(t, store, http.MethodGet, "/comparisons/compatible/"+uuidA+"?limit=5", "", "")
	requireStatus(t, rec, http.StatusOK)
	if rec.Header().Get("X-Total-Count") != "1" || store.gotProducts[0].ID != 1 {
		t.Fatalf("total = %s producto = %+v", rec.Header().Get("X-Total-Count"), store.gotProducts)
	}
	requireStatus(t, serve(t, newFakeStore(), http.MethodGet, "/comparisons/compatible/abc", "", ""), http.StatusBadRequest)
	requireStatus(t, serve(t, newFakeStore(), http.MethodGet, "/comparisons/compatible/"+uuidA+"?limit=0", "", ""), http.StatusBadRequest)
	requireStatus(t, serve(t, newFakeStore(), http.MethodGet, "/comparisons/compatible/44444444-4444-4444-4444-444444444444", "", ""), http.StatusNotFound)
}

// ----------------------------------------------------------- con sesión

func TestGuardadas_RequierenSesion(t *testing.T) {
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/comparisons/products", `{"productIds":["` + uuidA + `"]}`},
		{http.MethodGet, "/comparisons/products", ""},
		{http.MethodGet, "/comparisons/products/4", ""},
		{http.MethodPut, "/comparisons/products/4", `{"name":"x"}`},
		{http.MethodDelete, "/comparisons/products/4", ""},
		{http.MethodPost, "/comparisons/products/4/items", `{"productId":"` + uuidA + `"}`},
		{http.MethodDelete, "/comparisons/products/4/items/" + uuidA, ""},
	} {
		for _, token := range []string{"", "otro"} {
			store := newFakeStore()
			rec := serve(t, store, tc.method, tc.path, token, tc.body)
			requireStatus(t, rec, http.StatusUnauthorized)
			if store.writes != 0 {
				t.Fatalf("%s %s sin sesión no debe escribir", tc.method, tc.path)
			}
		}
	}
}

func TestCreate(t *testing.T) {
	store := newFakeStore()
	store.header.CreatedAt = time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
	rec := serve(t, store, http.MethodPost, "/comparisons/products", tokenAna, `{"productIds":["`+uuidA+`","`+uuidB+`"]}`)

	requireStatus(t, rec, http.StatusCreated)
	if rec.Header().Get("Location") != "/comparisons/products/4" || !strings.Contains(rec.Body.String(), `"name":"Mis refris"`) {
		t.Fatalf("Location = %q body = %s", rec.Header().Get("Location"), rec.Body.String())
	}
	if store.gotUser != userAna || store.gotName != DefaultName || len(store.gotProducts) != 2 {
		t.Fatalf("user = %d name = %q products = %+v", store.gotUser, store.gotName, store.gotProducts)
	}

	for _, tc := range []struct {
		name, body string
		status     int
		want       []string
	}{
		{"el userId del body ya no se acepta", `{"userId":99,"productIds":["` + uuidA + `"]}`, 400, []string{"userId/unknown_field"}},
		{"sin productos", `{"productIds":[]}`, 422, []string{"productIds/min"}},
		{"demasiados", `{"productIds":["` + strings.Repeat(uuidA+`","`, 5) + uuidA + `"]}`, 422, []string{"productIds/max"}},
		{"nombre largo", `{"name":"` + strings.Repeat("x", 151) + `","productIds":["` + uuidA + `"]}`, 422, []string{"name/max_length"}},
		{"otra categoría", `{"productIds":["` + uuidA + `","` + uuidC + `"]}`, 422, []string{"productIds/incompatible_category"}},
		{"inexistente", `{"productIds":["44444444-4444-4444-4444-444444444444"]}`, 422, []string{"productIds/not_found"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeStore()
			rec := serve(t, store, http.MethodPost, "/comparisons/products", tokenAna, tc.body)
			requireStatus(t, rec, tc.status)
			if _, fields := problem(t, rec); !reflect.DeepEqual(fields, tc.want) || store.writes != 0 {
				t.Fatalf("errores = %v writes = %d", fields, store.writes)
			}
		})
	}
}

func TestListGetRenameDelete(t *testing.T) {
	store := newFakeStore()
	store.summaries, store.total = []Summary{{ID: 4, Name: "Mis refris"}}, 1
	rec := serve(t, store, http.MethodGet, "/comparisons/products?limit=10", tokenAna, "")
	requireStatus(t, rec, http.StatusOK)
	if rec.Header().Get("X-Total-Count") != "1" || store.gotUser != userAna {
		t.Fatalf("total = %s user = %d", rec.Header().Get("X-Total-Count"), store.gotUser)
	}

	rec = serve(t, newFakeStore(), http.MethodGet, "/comparisons/products/4", tokenAna, "")
	requireStatus(t, rec, http.StatusOK)

	ajena := newFakeStore()
	ajena.getErr = ErrComparisonNotFound
	requireStatus(t, serve(t, ajena, http.MethodGet, "/comparisons/products/4", tokenAna, ""), http.StatusNotFound)
	requireStatus(t, serve(t, newFakeStore(), http.MethodGet, "/comparisons/products/x", tokenAna, ""), http.StatusBadRequest)

	store = newFakeStore()
	requireStatus(t, serve(t, store, http.MethodPut, "/comparisons/products/4", tokenAna, `{"name":"  Nuevo  "}`), http.StatusOK)
	if store.gotName != "Nuevo" {
		t.Fatalf("name = %q", store.gotName)
	}
	rec = serve(t, newFakeStore(), http.MethodPut, "/comparisons/products/4", tokenAna, `{"name":" "}`)
	requireStatus(t, rec, http.StatusUnprocessableEntity)

	requireStatus(t, serve(t, newFakeStore(), http.MethodDelete, "/comparisons/products/4", tokenAna, ""), http.StatusNoContent)
	missing := newFakeStore()
	missing.writeErr = ErrComparisonNotFound
	requireStatus(t, serve(t, missing, http.MethodDelete, "/comparisons/products/4", tokenAna, ""), http.StatusNotFound)
}

func TestItems(t *testing.T) {
	store := newFakeStore()
	rec := serve(t, store, http.MethodPost, "/comparisons/products/4/items", tokenAna, `{"productId":"`+uuidB+`"}`)
	requireStatus(t, rec, http.StatusCreated)
	if store.gotUser != userAna || store.gotProducts[0].ID != 2 {
		t.Fatalf("user = %d product = %+v", store.gotUser, store.gotProducts)
	}

	for _, tc := range []struct {
		name   string
		err    error
		body   string
		status int
		want   []string
	}{
		{"otra categoría", ErrIncompatibleCategory, uuidC, 422, []string{"productId/incompatible_category"}},
		{"llena", ErrComparisonFull, uuidB, 422, []string{"productId/max"}},
		{"repetido", ErrAlreadyIncluded, uuidB, 409, []string{}},
		{"comparación ajena", ErrComparisonNotFound, uuidB, 404, []string{}},
		{"producto inexistente", nil, "44444444-4444-4444-4444-444444444444", 422, []string{"productId/not_found"}},
		{"uuid inválido", nil, "abc", 422, []string{"productId/format"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeStore()
			store.writeErr = tc.err
			rec := serve(t, store, http.MethodPost, "/comparisons/products/4/items", tokenAna, `{"productId":"`+tc.body+`"}`)
			requireStatus(t, rec, tc.status)
			if _, fields := problem(t, rec); !reflect.DeepEqual(fields, tc.want) {
				t.Fatalf("errores = %v", fields)
			}
		})
	}

	store = newFakeStore()
	requireStatus(t, serve(t, store, http.MethodDelete, "/comparisons/products/4/items/"+uuidB, tokenAna, ""), http.StatusNoContent)
	if !reflect.DeepEqual(store.gotIDs, []int32{2}) {
		t.Fatalf("producto quitado = %v", store.gotIDs)
	}
	notIn := newFakeStore()
	notIn.writeErr = ErrItemNotFound
	requireStatus(t, serve(t, notIn, http.MethodDelete, "/comparisons/products/4/items/"+uuidB, tokenAna, ""), http.StatusNotFound)
	requireStatus(t, serve(t, newFakeStore(), http.MethodDelete, "/comparisons/products/4/items/abc", tokenAna, ""), http.StatusBadRequest)

	broken := newFakeStore()
	broken.writeErr = errBoom
	rec = serve(t, broken, http.MethodDelete, "/comparisons/products/4/items/"+uuidB, tokenAna, "")
	requireStatus(t, rec, http.StatusInternalServerError)
}
