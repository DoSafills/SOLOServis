package pricing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Integración TAL-74 contra PostgreSQL con todas las migraciones (incluido el
// trigger de historial de la 010). Reemplaza a los tests de caracterización de
// Pricing que vivían en internal/features. Se omite sin TEST_DATABASE_URL.

type pricingEnv struct {
	pool      *pgxpool.Pool
	router    http.Handler
	notifier  *fakeNotifier
	productID string // UUID público
	internal  int32  // id interno del producto
	storeA    int32
	storeB    int32
	tag       string
}

func newPricingEnv(t *testing.T) *pricingEnv {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL no definida: se omite el test de integración")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	env := &pricingEnv{pool: pool, notifier: &fakeNotifier{}, tag: strconv.FormatInt(time.Now().UnixNano(), 10)}
	scan := func(dest []any, sql string, args ...any) {
		t.Helper()
		if err := pool.QueryRow(ctx, sql, args...).Scan(dest...); err != nil {
			t.Fatalf("fixture: %v\n%s", err, sql)
		}
	}
	scan([]any{&env.internal, &env.productID}, `INSERT INTO product (category_id, name)
		SELECT id, $1 FROM product_category WHERE active ORDER BY id LIMIT 1 RETURNING id, public_id::text`,
		"TAL74 Producto "+env.tag)
	scan([]any{&env.storeA}, `INSERT INTO store (name) VALUES ($1) RETURNING id`, "TAL74 Tienda A "+env.tag)
	scan([]any{&env.storeB}, `INSERT INTO store (name) VALUES ($1) RETURNING id`, "TAL74 Tienda B "+env.tag)
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM product WHERE id = $1`, env.internal)
		_, _ = pool.Exec(ctx, `DELETE FROM store WHERE id = ANY($1)`, []int32{env.storeA, env.storeB})
		_, _ = pool.Exec(ctx, `DELETE FROM user_account WHERE email LIKE $1`, "tal74-%-"+env.tag+"@example.test")
	})

	h := NewHandler(NewRepository(pool), env.notifier, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Get("/offers/product/{productId}", h.ListProductOffers)
	r.Get("/offers/product/{productId}/best", h.BestOffer)
	r.Get("/offers/product/{productId}/price-history", h.ProductHistory)
	r.Get("/offers/{id}", h.GetOffer)
	r.Get("/offers/{id}/price-history", h.OfferHistory)
	r.Post("/offers", h.CreateOffer)
	r.Put("/offers/{id}", h.UpdateOffer)
	env.router = r
	return env
}

func (e *pricingEnv) do(t *testing.T, method, path string, body any) (*httptest.ResponseRecorder, Offer) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, _ := json.Marshal(body)
		reader = bytes.NewReader(encoded)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	var offer Offer
	_ = json.Unmarshal(rec.Body.Bytes(), &offer)
	return rec, offer
}

func (e *pricingEnv) history(t *testing.T, offerID int32) []string {
	t.Helper()
	rows, err := e.pool.Query(context.Background(),
		`SELECT price::text FROM product_price_history WHERE product_offer_id = $1 ORDER BY recorded_at, id`, offerID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	prices := []string{}
	for rows.Next() {
		var price string
		_ = rows.Scan(&price)
		prices = append(prices, price)
	}
	return prices
}

func (e *pricingEnv) offer(storeID int32, price any, extra map[string]any) map[string]any {
	body := map[string]any{"productId": e.productID, "storeId": storeID, "price": price,
		"productUrl": fmt.Sprintf("https://example.test/tal74-%d-%s", storeID, e.tag)}
	for k, v := range extra {
		body[k] = v
	}
	return body
}

func TestPricingIntegration_RegistroDePrecioActualEHistorial(t *testing.T) {
	e := newPricingEnv(t)

	// Alta: 201 y el trigger registra el precio inicial.
	rec, created := e.do(t, http.MethodPost, "/offers", e.offer(e.storeA, 123.45, map[string]any{"currency": "USD"}))
	if rec.Code != http.StatusCreated || created.ID == 0 || rec.Header().Get("Location") != "/offers/"+strconv.Itoa(int(created.ID)) {
		t.Fatalf("alta = %d %s", rec.Code, rec.Body.String())
	}
	// Se mantiene de la caracterización: el trigger de la 008 fuerza CLP.
	if created.Price != "123.45" || created.Currency != "CLP" || !created.Available || created.TotalPrice != "123.45" {
		t.Fatalf("oferta creada = %+v", created)
	}
	if got := e.history(t, created.ID); fmt.Sprint(got) != "[123.45]" {
		t.Fatalf("historial inicial = %v", got)
	}

	// Mismo producto, tienda y URL (esquema y host en mayúsculas, como en
	// TAL-63/64): se actualiza, no se duplica.
	sameURL := fmt.Sprintf("HTTPS://EXAMPLE.TEST/tal74-%d-%s", e.storeA, e.tag)
	rec, again := e.do(t, http.MethodPost, "/offers", e.offer(e.storeA, 123.45, map[string]any{"productUrl": sameURL}))
	if rec.Code != http.StatusOK || again.ID != created.ID {
		t.Fatalf("reenvío = %d id %d (quiero 200 id %d): %s", rec.Code, again.ID, created.ID, rec.Body.String())
	}
	if got := e.history(t, created.ID); fmt.Sprint(got) != "[123.45]" {
		t.Fatalf("sin cambio de precio no debe haber historial nuevo: %v", got)
	}

	// Nuevo precio por POST: historial +1.
	rec, _ = e.do(t, http.MethodPost, "/offers", e.offer(e.storeA, 99.9, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("actualización por POST = %d", rec.Code)
	}
	// PUT con solo el precio: ya no apaga la oferta ni borra datos por omisión.
	rec, updated := e.do(t, http.MethodPut, "/offers/"+strconv.Itoa(int(created.ID)), map[string]any{"price": 234.56})
	if rec.Code != http.StatusOK || updated.Price != "234.56" || !updated.Available || updated.Condition != "new" {
		t.Fatalf("PUT = %d %+v", rec.Code, updated)
	}
	// PUT sin cambio de precio: actualiza campos, no agrega historial.
	rec, updated = e.do(t, http.MethodPut, "/offers/"+strconv.Itoa(int(created.ID)), map[string]any{
		"price": 234.56, "stock": 4, "productUrl": "https://example.test/offer-" + e.tag, "condition": "used",
		"shippingCost": 1000, "available": false})
	if rec.Code != http.StatusOK || *updated.Stock != 4 || updated.Condition != "used" || updated.Available ||
		*updated.ProductURL != "https://example.test/offer-"+e.tag || updated.TotalPrice != "1234.56" {
		t.Fatalf("PUT de campos = %d %+v", rec.Code, updated)
	}
	if got := e.history(t, created.ID); fmt.Sprint(got) != "[123.45 99.90 234.56]" {
		t.Fatalf("historial = %v, quiero [123.45 99.90 234.56]", got)
	}

	// Historial por API, más reciente primero, con total.
	req := httptest.NewRequest(http.MethodGet, "/offers/"+strconv.Itoa(int(created.ID))+"/price-history?limit=2", nil)
	hrec := httptest.NewRecorder()
	e.router.ServeHTTP(hrec, req)
	var points []PricePoint
	_ = json.Unmarshal(hrec.Body.Bytes(), &points)
	if hrec.Code != 200 || hrec.Header().Get("X-Total-Count") != "3" || len(points) != 2 ||
		points[0].Price != "234.56" || points[1].Price != "99.90" {
		t.Fatalf("historial API = %d %s %s", hrec.Code, hrec.Header().Get("X-Total-Count"), hrec.Body.String())
	}
	// Filtro por fecha futura: vacío, total 0.
	req = httptest.NewRequest(http.MethodGet, "/offers/"+strconv.Itoa(int(created.ID))+"/price-history?from=2999-01-01", nil)
	hrec = httptest.NewRecorder()
	e.router.ServeHTTP(hrec, req)
	if hrec.Header().Get("X-Total-Count") != "0" || strings.TrimSpace(hrec.Body.String()) != "[]" {
		t.Fatalf("filtro futuro = %s %s", hrec.Header().Get("X-Total-Count"), hrec.Body.String())
	}
}

func TestPricingIntegration_DatosInvalidosNoLleganALaBD(t *testing.T) {
	e := newPricingEnv(t)

	for name, body := range map[string]map[string]any{
		"precio ausente":     {"productId": e.productID, "storeId": e.storeA},
		"condición inválida": e.offer(e.storeA, 99, map[string]any{"condition": "damaged"}),
		"stock negativo":     e.offer(e.storeA, 99, map[string]any{"stock": -1}),
		"tienda inexistente": e.offer(2147483647, 99, nil),
		"producto inexistente": {"productId": "00000000-0000-0000-0000-000000000000",
			"storeId": e.storeA, "price": 1},
	} {
		rec, _ := e.do(t, http.MethodPost, "/offers", body)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s: status = %d, quiero 422 (%s)", name, rec.Code, rec.Body.String())
		}
	}
	var count int
	if err := e.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM product_offer WHERE product_id = $1`, e.internal).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("se crearon %d ofertas con datos inválidos", count)
	}

	if rec, _ := e.do(t, http.MethodGet, "/offers/2147483647", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("oferta inexistente = %d", rec.Code)
	}
	if rec, _ := e.do(t, http.MethodGet, "/offers/2147483647/price-history", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("historial de oferta inexistente = %d", rec.Code)
	}
	if rec, _ := e.do(t, http.MethodPut, "/offers/2147483647", map[string]any{"price": 1}); rec.Code != http.StatusNotFound {
		t.Fatalf("PUT de oferta inexistente = %d", rec.Code)
	}
}

func TestPricingIntegration_MejorPrecioYOfertasDelProducto(t *testing.T) {
	e := newPricingEnv(t)

	// A: 100 + 50 de envío = 150. B: 120 con envío gratis = 120. B2: 80 no disponible.
	e.do(t, http.MethodPost, "/offers", e.offer(e.storeA, 100, map[string]any{"shippingCost": 50}))
	_, offerB := e.do(t, http.MethodPost, "/offers", e.offer(e.storeB, 120, map[string]any{"shippingFree": true, "shippingCost": 990}))
	e.do(t, http.MethodPost, "/offers", e.offer(e.storeB, 80, map[string]any{"available": false,
		"productUrl": "https://example.test/tal74-b2-" + e.tag}))

	rec, best := e.do(t, http.MethodGet, "/offers/product/"+e.productID+"/best", nil)
	if rec.Code != http.StatusOK || best.ID != offerB.ID || best.TotalPrice != "120.00" || best.ShippingCost != "990.00" {
		t.Fatalf("mejor oferta = %d %+v (quiero la de B con total 120.00)", rec.Code, best)
	}

	// Lista: disponibles primero y por precio; acepta también el id numérico.
	for _, key := range []string{e.productID, strconv.Itoa(int(e.internal))} {
		req := httptest.NewRequest(http.MethodGet, "/offers/product/"+key, nil)
		lrec := httptest.NewRecorder()
		e.router.ServeHTTP(lrec, req)
		var offers []Offer
		_ = json.Unmarshal(lrec.Body.Bytes(), &offers)
		var got []string
		for _, o := range offers {
			got = append(got, fmt.Sprintf("%s/%v", o.Price, o.Available))
		}
		if fmt.Sprint(got) != "[100.00/true 120.00/true 80.00/false]" {
			t.Fatalf("ofertas (%s) = %v", key, got)
		}
	}

	// Historial del producto en ambas tiendas, con su nombre.
	req := httptest.NewRequest(http.MethodGet, "/offers/product/"+e.productID+"/price-history", nil)
	hrec := httptest.NewRecorder()
	e.router.ServeHTTP(hrec, req)
	var points []ProductPricePoint
	_ = json.Unmarshal(hrec.Body.Bytes(), &points)
	stores := map[string]bool{}
	for _, p := range points {
		stores[p.StoreName] = true
	}
	if hrec.Header().Get("X-Total-Count") != "3" || len(points) != 3 || len(stores) != 2 {
		t.Fatalf("historial del producto = %s %s", hrec.Header().Get("X-Total-Count"), hrec.Body.String())
	}

	// Sin ofertas disponibles: 404 explícito.
	_, err := e.pool.Exec(context.Background(), `UPDATE product_offer SET available = false WHERE product_id = $1`, e.internal)
	if err != nil {
		t.Fatal(err)
	}
	if rec, _ := e.do(t, http.MethodGet, "/offers/product/"+e.productID+"/best", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("best sin disponibles = %d", rec.Code)
	}
}

func TestPricingIntegration_AvisoDeBajadaDePrecio(t *testing.T) {
	e := newPricingEnv(t)
	ctx := context.Background()

	var watcher, other int32
	for _, u := range []struct {
		id     *int32
		name   string
		target string
	}{{&watcher, "w", "90"}, {&other, "o", "50"}} {
		if err := e.pool.QueryRow(ctx, `INSERT INTO user_account (name, email, password_hash)
			VALUES ($1, $2, 'x') RETURNING id`, "TAL74 "+u.name, "tal74-"+u.name+"-"+e.tag+"@example.test").Scan(u.id); err != nil {
			t.Fatal(err)
		}
		if _, err := e.pool.Exec(ctx, `INSERT INTO product_user_interest (user_id, product_id, target_price, notify_price_drop)
			VALUES ($1, $2, $3::numeric, TRUE)`, *u.id, e.internal, u.target); err != nil {
			t.Fatal(err)
		}
	}

	_, created := e.do(t, http.MethodPost, "/offers", e.offer(e.storeA, 100, nil))
	if len(e.notifier.sent) != 0 {
		t.Fatal("el alta no debe avisar")
	}

	// 100 → 85 cruza el objetivo 90 (no el de 50): un aviso.
	rec, _ := e.do(t, http.MethodPut, "/offers/"+strconv.Itoa(int(created.ID)), map[string]any{"price": 85})
	if rec.Code != http.StatusOK || len(e.notifier.sent) != 1 {
		t.Fatalf("avisos = %+v", e.notifier.sent)
	}
	n := e.notifier.sent[0]
	if n.UserID != watcher || n.Data["oldPrice"] != "100.00" || n.Data["price"] != "85.00" || n.Data["targetPrice"] != "90.00" {
		t.Fatalf("aviso = %+v", n)
	}

	// Una subida no avisa.
	e.do(t, http.MethodPost, "/offers", e.offer(e.storeA, 95, nil))
	if len(e.notifier.sent) != 1 {
		t.Fatalf("una subida no debe avisar: %+v", e.notifier.sent)
	}
}
