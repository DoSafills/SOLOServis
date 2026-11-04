package products

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/DoSafills/SOLOServis/backend/internal/categories"
	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/DoSafills/SOLOServis/backend/internal/products/dto"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

const testProductID = "11111111-2222-3333-4444-555555555555"

// errBoom simula un error de base de datos con detalle interno.
var errBoom = errors.New(`relation "product" does not exist`)

func int32p(v int32) *int32     { return &v }
func strp(v string) *string     { return &v }
func floatp(v float64) *float64 { return &v }

// refrigeradores replica la categoría sembrada (TAL-71) con dos atributos.
var refrigeradores = WriteCategory{
	ID: int32p(1), Slug: strp("refrigeradores"), ItemType: categories.ItemTypeProduct, LegacyID: int32p(5),
	Attributes: []categories.Attribute{
		{ID: 10, Key: "capacidad_litros", DataType: "integer", Required: true, Min: floatp(50), Max: floatp(1500), DisplayOrder: 1},
		{ID: 11, Key: "no_frost", DataType: "boolean", DisplayOrder: 2},
	},
}

type fakeStore struct {
	items    []dto.ProductListItem
	total    int64
	detail   dto.ProductDetail
	category WriteCategory

	listErr, detailErr, resolveErr, writeErr error

	gotList     *ListParams
	gotRef      *CategoryRef
	gotWrite    *ProductWrite
	gotPublicID pgtype.UUID
	writes      int
}

func (f *fakeStore) List(_ context.Context, p ListParams) ([]dto.ProductListItem, int64, error) {
	f.gotList = &p
	return f.items, f.total, f.listErr
}

func (f *fakeStore) GetDetail(_ context.Context, id pgtype.UUID) (dto.ProductDetail, error) {
	f.gotPublicID = id
	return f.detail, f.detailErr
}

func (f *fakeStore) ResolveCategory(_ context.Context, ref CategoryRef) (WriteCategory, error) {
	f.gotRef = &ref
	return f.category, f.resolveErr
}

func (f *fakeStore) Create(_ context.Context, p ProductWrite) (pgtype.UUID, error) {
	f.writes++
	f.gotWrite = &p
	var id pgtype.UUID
	_ = id.Scan(testProductID)
	return id, f.writeErr
}

func (f *fakeStore) Update(_ context.Context, id pgtype.UUID, p ProductWrite) error {
	f.writes++
	f.gotWrite = &p
	f.gotPublicID = id
	return f.writeErr
}

func (f *fakeStore) Deactivate(_ context.Context, id pgtype.UUID) error {
	f.writes++
	f.gotPublicID = id
	return f.writeErr
}

func (f *fakeStore) ListCategories(context.Context) ([]generated.ListProductCategoriesRow, error) {
	return []generated.ListProductCategoriesRow{{ID: 5, Name: "Refrigeracion"}}, f.listErr
}

func (f *fakeStore) ListReviewsByPublicID(context.Context, pgtype.UUID) ([]generated.ListProductReviewsRow, error) {
	return nil, f.detailErr
}

// serve monta el handler con las mismas rutas que el router real.
func serve(t *testing.T, store Store, method, target, body string) (*httptest.ResponseRecorder, string) {
	t.Helper()
	var logs bytes.Buffer
	h := NewHandler(store, slog.New(slog.NewJSONHandler(&logs, nil)))

	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Get("/product-categories", h.ListCategories)
	r.Get("/products", h.List)
	r.Get("/products/{publicID}", h.GetByPublicID)
	r.Post("/products", h.Create)
	r.Put("/products/{publicID}", h.Update)
	r.Delete("/products/{publicID}", h.Deactivate)
	r.Get("/products/{publicID}/reviews", h.ListReviews)

	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec, logs.String()
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

func requireStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, quiero %d (%s)", rec.Code, want, rec.Body.String())
	}
}

// ---------------------------------------------------------------- GET /products

func TestList_CasoFeliz(t *testing.T) {
	store := &fakeStore{items: []dto.ProductListItem{{ID: testProductID, Name: "Refrigerador"}}, total: 38}

	rec, _ := serve(t, store, http.MethodGet, "/products", "")

	requireStatus(t, rec, http.StatusOK)
	if rec.Header().Get("X-Total-Count") != "38" {
		t.Fatalf("X-Total-Count = %q", rec.Header().Get("X-Total-Count"))
	}
	var got []dto.ProductListItem
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || len(got) != 1 || got[0].ID != testProductID {
		t.Fatalf("body debe seguir siendo un arreglo de productos: %s", rec.Body.String())
	}
	if *store.gotList != (ListParams{Limit: DefaultListLimit}) {
		t.Fatalf("params = %+v", *store.gotList)
	}
}

func TestList_FiltrosYPaginacion(t *testing.T) {
	tests := []struct {
		query string
		check func(ListParams) bool
	}{
		{"category=5", func(p ListParams) bool {
			return p.LegacyCategoryID != nil && *p.LegacyCategoryID == 5 && p.CategorySlug == nil
		}},
		{"category=refrigeradores", func(p ListParams) bool { return p.CategorySlug != nil && *p.CategorySlug == "refrigeradores" }},
		{"limit=10&offset=20", func(p ListParams) bool { return p.Limit == 10 && p.Offset == 20 }},
	}
	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			store := &fakeStore{}
			rec, _ := serve(t, store, http.MethodGet, "/products?"+tc.query, "")
			requireStatus(t, rec, http.StatusOK)
			if !tc.check(*store.gotList) {
				t.Fatalf("params = %+v", *store.gotList)
			}
		})
	}
}

func TestList_SinResultadosDevuelveArregloVacio(t *testing.T) {
	rec, _ := serve(t, &fakeStore{}, http.MethodGet, "/products?offset=500", "")

	requireStatus(t, rec, http.StatusOK)
	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("body = %s, quiero []", rec.Body.String())
	}
}

func TestList_ParametrosInvalidos(t *testing.T) {
	tests := []struct{ query, want string }{
		{"category=Refri_Geradores", "category/format"},
		{"category=0", "category/min"},
		{"category=-3", "category/min"},
		{"limit=0", "limit/min"},
		{"limit=101", "limit/max"},
		{"limit=1000000", "limit/max"},
		{"limit=abc", "limit/invalid_type"},
		{"offset=-1", "offset/min"},
	}
	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			store := &fakeStore{}
			rec, _ := serve(t, store, http.MethodGet, "/products?"+tc.query, "")

			requireStatus(t, rec, http.StatusBadRequest)
			p := decodeProblem(t, rec)
			if p.Code != httpx.CodeInvalidParameter || !reflect.DeepEqual(fieldCodes(p), []string{tc.want}) {
				t.Fatalf("problem = %+v", p)
			}
			if store.gotList != nil {
				t.Fatal("no debe consultar el repositorio con parámetros inválidos")
			}
		})
	}
}

func TestList_ErrorInternoNoExponeDetalles(t *testing.T) {
	rec, logs := serve(t, &fakeStore{listErr: errBoom}, http.MethodGet, "/products", "")

	requireStatus(t, rec, http.StatusInternalServerError)
	p := decodeProblem(t, rec)
	if p.Code != httpx.CodeInternalError || strings.Contains(rec.Body.String(), "relation") {
		t.Fatalf("problem = %+v", p)
	}
	if !strings.Contains(logs, "relation") || !strings.Contains(logs, p.RequestID) {
		t.Fatalf("el log debe tener el error y el request_id: %s", logs)
	}
}

// --------------------------------------------------------- GET /products/{id}

func TestGetByPublicID(t *testing.T) {
	detail := dto.ProductDetail{ID: testProductID, Name: "Refrigerador", CategorySlug: strp("refrigeradores"),
		Attributes: map[string]json.RawMessage{"capacidad_litros": json.RawMessage(`350`)}}

	t.Run("caso feliz", func(t *testing.T) {
		rec, _ := serve(t, &fakeStore{detail: detail}, http.MethodGet, "/products/"+testProductID, "")
		requireStatus(t, rec, http.StatusOK)
		if !strings.Contains(rec.Body.String(), `"categorySlug":"refrigeradores"`) ||
			!strings.Contains(rec.Body.String(), `"attributes":{"capacidad_litros":350}`) {
			t.Fatalf("body = %s", rec.Body.String())
		}
	})
	t.Run("id inválido", func(t *testing.T) {
		rec, _ := serve(t, &fakeStore{}, http.MethodGet, "/products/no-es-uuid", "")
		requireStatus(t, rec, http.StatusBadRequest)
		if p := decodeProblem(t, rec); p.Code != httpx.CodeInvalidParameter {
			t.Fatalf("problem = %+v", p)
		}
	})
	t.Run("inexistente o dado de baja", func(t *testing.T) {
		rec, _ := serve(t, &fakeStore{detailErr: ErrNotFound}, http.MethodGet, "/products/"+testProductID, "")
		requireStatus(t, rec, http.StatusNotFound)
		if p := decodeProblem(t, rec); p.Code != httpx.CodeNotFound {
			t.Fatalf("problem = %+v", p)
		}
	})
	t.Run("error interno", func(t *testing.T) {
		rec, _ := serve(t, &fakeStore{detailErr: errBoom}, http.MethodGet, "/products/"+testProductID, "")
		requireStatus(t, rec, http.StatusInternalServerError)
		if strings.Contains(rec.Body.String(), "relation") {
			t.Fatal("el body expone el error interno")
		}
	})
}

// ------------------------------------------------------------- POST /products

const validCreateBody = `{
	"categorySlug": "refrigeradores",
	"brandId": 3,
	"name": "  Refrigerador LG 350L  ",
	"model": "GT35",
	"sku": "LG-GT35",
	"description": "No frost",
	"imageUrl": "https://example.test/lg.png",
	"attributes": {"capacidad_litros": 350, "no_frost": true}
}`

func TestCreate_CasoFeliz(t *testing.T) {
	store := &fakeStore{category: refrigeradores, detail: dto.ProductDetail{ID: testProductID, Name: "Refrigerador LG 350L"}}

	rec, _ := serve(t, store, http.MethodPost, "/products", validCreateBody)

	requireStatus(t, rec, http.StatusCreated)
	if rec.Header().Get("Location") != "/products/"+testProductID {
		t.Fatalf("Location = %q", rec.Header().Get("Location"))
	}
	var got dto.ProductDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.ID != testProductID {
		t.Fatalf("el body debe tener la forma del GET: %s", rec.Body.String())
	}
	if *store.gotRef != (CategoryRef{Slug: "refrigeradores"}) {
		t.Fatalf("categoría resuelta = %+v", *store.gotRef)
	}
	want := ProductWrite{
		LegacyCategoryID: 5, CategoryRef: refrigeradores.ID, BrandID: int32p(3),
		Name: "Refrigerador LG 350L", Model: "GT35", SKU: "LG-GT35", Description: "No frost",
		ImageURL: "https://example.test/lg.png",
		Attributes: []categories.Value{
			{AttributeID: 10, Key: "capacidad_litros", DataType: "integer", Text: "350"},
			{AttributeID: 11, Key: "no_frost", DataType: "boolean", Text: "true"},
		},
	}
	if !reflect.DeepEqual(*store.gotWrite, want) {
		t.Fatalf("write =\n%+v\nquiero\n%+v", *store.gotWrite, want)
	}
}

func TestCreate_CategoriaAntiguaPorID(t *testing.T) {
	// Ingestion API todavía envía categoryId (product_category) sin atributos.
	lavado := WriteCategory{ItemType: categories.ItemTypeProduct, LegacyID: int32p(3), Attributes: []categories.Attribute{}}
	store := &fakeStore{category: lavado, detail: dto.ProductDetail{ID: testProductID}}

	rec, _ := serve(t, store, http.MethodPost, "/products",
		`{"categoryId": 3, "name": "Lavadora", "imageUrl": "https://example.test/l.png"}`)

	requireStatus(t, rec, http.StatusCreated)
	if *store.gotRef != (CategoryRef{LegacyID: 3}) || store.gotWrite.LegacyCategoryID != 3 || store.gotWrite.CategoryRef != nil {
		t.Fatalf("ref = %+v write = %+v", *store.gotRef, *store.gotWrite)
	}
}

func TestCreate_ErroresDeDecodificacion(t *testing.T) {
	tests := []struct {
		name, body, contentType string
		wantStatus              int
		wantCode                string
	}{
		{"json malformado", `{"name":`, "application/json", 400, httpx.CodeMalformedJSON},
		{"campo desconocido", `{"nombre":"x"}`, "application/json", 400, httpx.CodeUnknownField},
		{"tipo incorrecto", `{"name": 5}`, "application/json", 400, httpx.CodeInvalidType},
		{"attributes no es objeto", `{"attributes": [1]}`, "application/json", 400, httpx.CodeInvalidType},
		{"content-type incorrecto", `{}`, "text/plain", 415, httpx.CodeUnsupportedMediaType},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeStore{category: refrigeradores}
			var logs bytes.Buffer
			h := NewHandler(store, slog.New(slog.NewJSONHandler(&logs, nil)))
			req := httptest.NewRequest(http.MethodPost, "/products", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.contentType)
			rec := httptest.NewRecorder()
			httpx.RequestID(http.HandlerFunc(h.Create)).ServeHTTP(rec, req)

			requireStatus(t, rec, tc.wantStatus)
			if p := decodeProblem(t, rec); p.Code != tc.wantCode {
				t.Fatalf("problem = %+v", p)
			}
			if store.writes != 0 || store.gotRef != nil {
				t.Fatal("no debe tocar el repositorio")
			}
		})
	}
}

func TestCreate_ReglasDeCampos(t *testing.T) {
	long := strings.Repeat("x", 256)
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"sin nombre ni categoría ni imagen", `{}`, []string{"name/required", "categorySlug/required", "imageUrl/required"}},
		{"ambas categorías", `{"name":"x","categorySlug":"refrigeradores","categoryId":5,"imageUrl":"https://e.test/a.png"}`, []string{"categoryId/one_of"}},
		{"slug con formato inválido", `{"name":"x","categorySlug":"Refri Geradores","imageUrl":"https://e.test/a.png"}`, []string{"categorySlug/format"}},
		{"categoryId cero", `{"name":"x","categoryId":0,"imageUrl":"https://e.test/a.png"}`, []string{"categoryId/min"}},
		{"imageUrl no http", `{"name":"x","categorySlug":"refrigeradores","imageUrl":"ftp://e.test/a.png"}`, []string{"imageUrl/format"}},
		{"brandId cero", `{"name":"x","categorySlug":"refrigeradores","brandId":0,"imageUrl":"https://e.test/a.png"}`, []string{"brandId/min"}},
		{"nombre demasiado largo", `{"name":"` + long + `","categorySlug":"refrigeradores","imageUrl":"https://e.test/a.png"}`, []string{"name/max_length"}},
		{"nombre solo espacios", `{"name":"   ","categorySlug":"refrigeradores","imageUrl":"https://e.test/a.png"}`, []string{"name/required"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeStore{category: refrigeradores}
			rec, _ := serve(t, store, http.MethodPost, "/products", tc.body)

			requireStatus(t, rec, http.StatusUnprocessableEntity)
			p := decodeProblem(t, rec)
			if p.Code != httpx.CodeValidationFailed || !reflect.DeepEqual(fieldCodes(p), tc.want) {
				t.Fatalf("errores = %v, quiero %v", fieldCodes(p), tc.want)
			}
			if store.gotRef != nil || store.writes != 0 {
				t.Fatal("con reglas incumplidas no se consulta la categoría ni se escribe")
			}
		})
	}
}

func TestCreate_CategoriaYAtributos(t *testing.T) {
	servicio := WriteCategory{ID: int32p(4), Slug: strp("internet-fibra-optica"), ItemType: categories.ItemTypeService,
		Attributes: []categories.Attribute{}}
	sinLegado := WriteCategory{ID: int32p(9), Slug: strp("nueva"), ItemType: categories.ItemTypeProduct,
		Attributes: []categories.Attribute{}}

	tests := []struct {
		name       string
		store      *fakeStore
		attributes string
		want       []string
	}{
		{"categoría inexistente", &fakeStore{resolveErr: ErrCategoryNotFound}, `{}`, []string{"categorySlug/not_found"}},
		{"categoría de servicios", &fakeStore{category: servicio}, `{}`, []string{"categorySlug/one_of"}},
		{"categoría sin equivalente antiguo", &fakeStore{category: sinLegado}, `{}`, []string{"categorySlug/not_found"}},
		{"atributo requerido ausente", &fakeStore{category: refrigeradores}, `{"no_frost": true}`, []string{"attributes.capacidad_litros/required"}},
		{"atributo con tipo incorrecto", &fakeStore{category: refrigeradores}, `{"capacidad_litros": "350"}`, []string{"attributes.capacidad_litros/invalid_type"}},
		{"atributo fuera de rango y key desconocida", &fakeStore{category: refrigeradores}, `{"capacidad_litros": 10, "color": "rojo"}`,
			[]string{"attributes.capacidad_litros/min", "attributes.color/unknown_field"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"name":"x","categorySlug":"refrigeradores","imageUrl":"https://e.test/a.png","attributes":` + tc.attributes + `}`
			rec, _ := serve(t, tc.store, http.MethodPost, "/products", body)

			requireStatus(t, rec, http.StatusUnprocessableEntity)
			if got := fieldCodes(decodeProblem(t, rec)); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("errores = %v, quiero %v", got, tc.want)
			}
			if tc.store.writes != 0 {
				t.Fatal("no debe escribir con atributos inválidos")
			}
		})
	}
}

func TestCreate_ErroresDeBaseDeDatos(t *testing.T) {
	tests := []struct {
		name       string
		store      *fakeStore
		wantStatus int
		wantCode   string
		wantFields []string
	}{
		{"sku duplicado", &fakeStore{category: refrigeradores, writeErr: &pgconn.PgError{Code: "23505", ConstraintName: "product_sku_key"}},
			409, httpx.CodeConflict, []string{}},
		{"marca inexistente", &fakeStore{category: refrigeradores, writeErr: &pgconn.PgError{Code: "23503", ConstraintName: "product_brand_id_fkey"}},
			422, httpx.CodeValidationFailed, []string{"brandId/not_found"}},
		{"otra FK no se disfraza de 422", &fakeStore{category: refrigeradores, writeErr: &pgconn.PgError{Code: "23503", ConstraintName: "otra_fkey"}},
			500, httpx.CodeInternalError, []string{}},
		{"error al resolver categoría", &fakeStore{resolveErr: errBoom}, 500, httpx.CodeInternalError, []string{}},
		{"error al guardar", &fakeStore{category: refrigeradores, writeErr: errBoom}, 500, httpx.CodeInternalError, []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec, _ := serve(t, tc.store, http.MethodPost, "/products", validCreateBody)

			requireStatus(t, rec, tc.wantStatus)
			p := decodeProblem(t, rec)
			if p.Code != tc.wantCode || !reflect.DeepEqual(fieldCodes(p), tc.wantFields) {
				t.Fatalf("problem = %+v", p)
			}
			if strings.Contains(rec.Body.String(), "relation") || strings.Contains(rec.Body.String(), "fkey") {
				t.Fatalf("el body expone detalles internos: %s", rec.Body.String())
			}
		})
	}
}

// ------------------------------------------------- PUT y DELETE /products/{id}

func TestUpdate(t *testing.T) {
	body := `{"categorySlug":"refrigeradores","name":"Nuevo nombre","attributes":{"capacidad_litros":400}}`

	t.Run("caso feliz", func(t *testing.T) {
		store := &fakeStore{category: refrigeradores, detail: dto.ProductDetail{ID: testProductID, Name: "Nuevo nombre"}}
		rec, _ := serve(t, store, http.MethodPut, "/products/"+testProductID, body)

		requireStatus(t, rec, http.StatusOK)
		if store.gotPublicID.String() != testProductID || store.gotWrite.Name != "Nuevo nombre" ||
			len(store.gotWrite.Attributes) != 1 || store.gotWrite.Attributes[0].Text != "400" {
			t.Fatalf("write = %+v", store.gotWrite)
		}
		if !strings.Contains(rec.Body.String(), `"name":"Nuevo nombre"`) {
			t.Fatalf("el body debe tener la forma del GET: %s", rec.Body.String())
		}
	})
	t.Run("imageUrl no se acepta en PUT", func(t *testing.T) {
		rec, _ := serve(t, &fakeStore{category: refrigeradores}, http.MethodPut, "/products/"+testProductID,
			`{"categorySlug":"refrigeradores","name":"x","imageUrl":"https://e.test/a.png"}`)
		requireStatus(t, rec, http.StatusBadRequest)
		if p := decodeProblem(t, rec); p.Code != httpx.CodeUnknownField {
			t.Fatalf("problem = %+v", p)
		}
	})
	t.Run("id inválido", func(t *testing.T) {
		rec, _ := serve(t, &fakeStore{}, http.MethodPut, "/products/x", body)
		requireStatus(t, rec, http.StatusBadRequest)
	})
	t.Run("regla incumplida", func(t *testing.T) {
		rec, _ := serve(t, &fakeStore{category: refrigeradores}, http.MethodPut, "/products/"+testProductID,
			`{"categorySlug":"refrigeradores","name":"x"}`)
		requireStatus(t, rec, http.StatusUnprocessableEntity)
		if got := fieldCodes(decodeProblem(t, rec)); !reflect.DeepEqual(got, []string{"attributes.capacidad_litros/required"}) {
			t.Fatalf("errores = %v", got)
		}
	})
	t.Run("inexistente", func(t *testing.T) {
		rec, _ := serve(t, &fakeStore{category: refrigeradores, writeErr: ErrNotFound}, http.MethodPut, "/products/"+testProductID, body)
		requireStatus(t, rec, http.StatusNotFound)
	})
	t.Run("error interno", func(t *testing.T) {
		rec, _ := serve(t, &fakeStore{category: refrigeradores, writeErr: errBoom}, http.MethodPut, "/products/"+testProductID, body)
		requireStatus(t, rec, http.StatusInternalServerError)
	})
}

func TestDeactivate(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		err        error
		wantStatus int
	}{
		{"caso feliz", "/products/" + testProductID, nil, http.StatusNoContent},
		{"id inválido", "/products/x", nil, http.StatusBadRequest},
		{"inexistente o ya dado de baja", "/products/" + testProductID, ErrNotFound, http.StatusNotFound},
		{"error interno", "/products/" + testProductID, errBoom, http.StatusInternalServerError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec, _ := serve(t, &fakeStore{writeErr: tc.err}, http.MethodDelete, tc.path, "")
			requireStatus(t, rec, tc.wantStatus)
			if tc.wantStatus == http.StatusNoContent && rec.Body.Len() != 0 {
				t.Fatalf("204 sin body, recibí %q", rec.Body.String())
			}
		})
	}
}

// ------------------------------------------------- rutas heredadas de PR #30

func TestListCategoriesYReviews(t *testing.T) {
	rec, _ := serve(t, &fakeStore{}, http.MethodGet, "/product-categories", "")
	requireStatus(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"name":"Refrigeracion"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}

	rec, _ = serve(t, &fakeStore{listErr: errBoom}, http.MethodGet, "/product-categories", "")
	requireStatus(t, rec, http.StatusInternalServerError)

	rec, _ = serve(t, &fakeStore{}, http.MethodGet, "/products/"+testProductID+"/reviews", "")
	requireStatus(t, rec, http.StatusOK)
	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("body = %s", rec.Body.String())
	}

	rec, _ = serve(t, &fakeStore{detailErr: ErrNotFound}, http.MethodGet, "/products/"+testProductID+"/reviews", "")
	requireStatus(t, rec, http.StatusNotFound)
}
