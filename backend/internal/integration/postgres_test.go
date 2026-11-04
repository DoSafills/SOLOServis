package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/allapis"
	"github.com/jackc/pgx/v5/pgxpool"
)

// env levanta los 3 dominios como servidores HTTP reales sobre la misma base,
// igual que los pods de Kubernetes: las llamadas entre APIs (cart → product,
// cart → pricing, ingestion → notification) viajan por HTTP.
type env struct {
	t    *testing.T
	pool *pgxpool.Pool
	urls map[string]string
	tag  string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL no definida: se omite el test de integración")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	t.Setenv("INTERNAL_API_TOKEN", internalToken)

	// Servidores sin arrancar: sus URLs se conocen antes de armar los routers,
	// porque pricing e ingestion leen NOTIFICATION_API_URL al construirse.
	servers := map[string]*httptest.Server{}
	e := &env{t: t, pool: pool, urls: map[string]string{}, tag: "tal133-" + strconv.FormatInt(time.Now().UnixNano(), 10)}
	for domain := range allapis.Domains {
		servers[domain] = httptest.NewUnstartedServer(nil)
		e.urls[domain] = "http://" + servers[domain].Listener.Addr().String()
	}
	// Las APIs que otras llaman (product, store, service, pricing, notification)
	// están todas en comercial: como en el clúster, todas las URLs van a él.
	urls, err := allapis.InternalURLs(allapis.Domains["comercial"], e.urls["comercial"], func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	for name, url := range urls {
		t.Setenv(name, url)
	}
	for domain, server := range servers {
		if _, err := allapis.InternalURLs(allapis.Domains[domain], e.urls[domain], os.Getenv); err != nil {
			t.Fatalf("%s: %v", domain, err)
		}
		server.Config.Handler = allapis.NewDomainRouter(allapis.Domains[domain])(pool, "http://localhost:5173")
		server.Start()
		t.Cleanup(server.Close)
	}
	return e
}

// do llama a un dominio por HTTP y devuelve el código y el cuerpo.
func (e *env) do(domain, method, path, token string, body any) (int, []byte) {
	e.t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, _ := json.Marshal(body)
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, e.urls[domain]+path, reader)
	if err != nil {
		e.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return res.StatusCode, data
}

// expect falla si el código no es el esperado y devuelve el cuerpo.
func (e *env) expect(want int, domain, method, path, token string, body any) []byte {
	e.t.Helper()
	status, data := e.do(domain, method, path, token, body)
	if status != want {
		e.t.Fatalf("%s %s %s = %d %s, quiero %d", domain, method, path, status, data, want)
	}
	return data
}

// count devuelve el resultado de un SELECT count(*).
func (e *env) count(query string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		e.t.Fatalf("%s: %v", query, err)
	}
	return n
}

type session struct {
	UserID       int32  `json:"userId"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

// register crea un usuario por usuarios (POST /auth/register) y lo borra al
// terminar; el borrado en cascada se lleva sesiones, favoritos, carrito,
// reseñas, watchlist y notificaciones.
func (e *env) register(name string) (session, string) {
	e.t.Helper()
	email := name + "-" + e.tag + "@integracion.test"
	data := e.expect(http.StatusCreated, "usuarios", "POST", "/auth/register", "",
		map[string]any{"name": "Test " + name, "email": email, "password": "Clave12345!"})
	var s session
	if err := json.Unmarshal(data, &s); err != nil || s.UserID == 0 || s.AccessToken == "" {
		e.t.Fatalf("register: %s", data)
	}
	e.t.Cleanup(func() {
		_, _ = e.pool.Exec(context.Background(), `DELETE FROM user_account WHERE id = $1`, s.UserID)
	})
	return s, email
}

// seedOffer devuelve un producto activo con una oferta disponible del seed.
func (e *env) seedOffer() (productID string, offerID, storeID int32) {
	e.t.Helper()
	err := e.pool.QueryRow(context.Background(), `
		SELECT p.public_id::text, o.id, o.store_id FROM product p JOIN product_offer o ON o.product_id = p.id
		WHERE p.active AND o.available ORDER BY p.id, o.id LIMIT 1`).Scan(&productID, &offerID, &storeID)
	if err != nil {
		e.t.Fatalf("seed sin ofertas (¿faltan migraciones?): %v", err)
	}
	return productID, offerID, storeID
}

func (e *env) newStore() int32 {
	e.t.Helper()
	var id int32
	if err := e.pool.QueryRow(context.Background(), `INSERT INTO store (name) VALUES ($1) RETURNING id`, "Tienda "+e.tag).Scan(&id); err != nil {
		e.t.Fatal(err)
	}
	// ON DELETE CASCADE: se lleva sus ofertas y el historial de precios.
	e.t.Cleanup(func() { _, _ = e.pool.Exec(context.Background(), `DELETE FROM store WHERE id = $1`, id) })
	return id
}

func hash(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// Dominio usuarios: cuentas y sesiones en user_account y user_session.
func TestIntegracion_Usuarios(t *testing.T) {
	e := newEnv(t)
	s, email := e.register("ana")

	// La contraseña se guarda con PBKDF2 y la sesión solo como hash SHA-256.
	var passwordHash string
	if err := e.pool.QueryRow(context.Background(), `SELECT password_hash FROM user_account WHERE id = $1`, s.UserID).Scan(&passwordHash); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(passwordHash, "pbkdf2-sha256$") || strings.Contains(passwordHash, "Clave12345!") {
		t.Errorf("password_hash = %q", passwordHash)
	}
	if n := e.count(`SELECT count(*) FROM user_session WHERE user_id = $1 AND access_token_hash = $2 AND refresh_token_hash = $3`,
		s.UserID, hash(s.AccessToken), hash(s.RefreshToken)); n != 1 {
		t.Errorf("sesiones con el hash de los tokens = %d, quiero 1", n)
	}

	e.expect(http.StatusOK, "usuarios", "POST", "/auth/login", "", map[string]any{"email": "  " + strings.ToUpper(email) + "  ", "password": "Clave12345!"})
	e.expect(http.StatusUnauthorized, "usuarios", "POST", "/auth/login", "", map[string]any{"email": email, "password": "otra"})
	e.expect(http.StatusConflict, "usuarios", "POST", "/auth/register", "", map[string]any{"name": "Otra", "email": "  " + strings.ToUpper(email) + "  ", "password": "Clave12345!"})
	e.expect(http.StatusOK, "usuarios", "GET", "/auth/validate", s.AccessToken, nil)

	// user lee el perfil solo con el token del mismo usuario.
	path := fmt.Sprintf("/users/%d", s.UserID)
	if data := e.expect(http.StatusOK, "usuarios", "GET", path, s.AccessToken, nil); !strings.Contains(string(data), email) {
		t.Errorf("GET %s = %s", path, data)
	}
	e.expect(http.StatusUnauthorized, "usuarios", "GET", path, "", nil)

	// refresh rota la sesión: el refresh token viejo deja de servir.
	var renewed session
	_ = json.Unmarshal(e.expect(http.StatusOK, "usuarios", "POST", "/auth/refresh", "", map[string]any{"refreshToken": s.RefreshToken}), &renewed)
	e.expect(http.StatusUnauthorized, "usuarios", "POST", "/auth/refresh", "", map[string]any{"refreshToken": s.RefreshToken})

	// logout borra la sesión.
	e.expect(http.StatusNoContent, "usuarios", "POST", "/auth/logout", renewed.AccessToken, nil)
	e.expect(http.StatusUnauthorized, "usuarios", "GET", "/auth/validate", renewed.AccessToken, nil)
	if n := e.count(`SELECT count(*) FROM user_session WHERE user_id = $1`, s.UserID); n != 1 {
		t.Errorf("sesiones tras refresh y logout = %d, quiero 1 (la del registro)", n)
	}
}

// Dominio comercial: cada API escribe y lee sus tablas, y las que llaman a
// otra API (favorites, cart, review, watchlist) lo hacen por HTTP al mismo
// proceso.
func TestIntegracion_Comercial(t *testing.T) {
	e := newEnv(t)
	s, _ := e.register("beto")
	user := s.UserID
	productID, offerID, storeID := e.seedOffer()

	t.Run("lecturas product, store, service, search y comparison", func(t *testing.T) {
		e.expect(http.StatusOK, "comercial", "GET", "/products/"+productID, "", nil)
		e.expect(http.StatusOK, "comercial", "GET", fmt.Sprintf("/stores/%d", storeID), "", nil)
		if data := e.expect(http.StatusOK, "comercial", "GET", "/services", "", nil); !strings.HasPrefix(string(data), "[{") {
			t.Errorf("GET /services sin servicios: %.80s", data)
		}
		e.expect(http.StatusOK, "comercial", "GET", "/search/products?q=a&limit=1", "", nil)
		e.expect(http.StatusOK, "comercial", "GET", "/comparisons/compatible/"+productID, "", nil)
	})

	t.Run("favorites escribe product_user_interest", func(t *testing.T) {
		e.expect(http.StatusCreated, "comercial", "POST", "/favorites/"+productID, s.AccessToken, map[string]any{})
		if n := e.count(`SELECT count(*) FROM product_user_interest i JOIN product p ON p.id = i.product_id
			WHERE i.user_id = $1 AND p.public_id = $2::uuid AND i.is_favorite`, user, productID); n != 1 {
			t.Errorf("favorito en la base = %d", n)
		}
		if data := e.expect(http.StatusOK, "comercial", "GET", "/favorites", s.AccessToken, nil); !strings.Contains(string(data), productID) {
			t.Errorf("GET /favorites = %s", data)
		}
		e.expect(http.StatusNoContent, "comercial", "DELETE", "/favorites/"+productID, s.AccessToken, nil)
		e.expect(http.StatusUnauthorized, "comercial", "GET", "/favorites", "", nil)
	})

	t.Run("cart llama a product y pricing y escribe cart_item", func(t *testing.T) {
		e.expect(http.StatusCreated, "comercial", "POST", "/cart/items", s.AccessToken, map[string]any{"productId": productID, "offerId": offerID, "quantity": 2})
		if data := e.expect(http.StatusOK, "comercial", "GET", "/cart/count", s.AccessToken, nil); !strings.Contains(string(data), `"count":2`) {
			t.Errorf("GET /cart/count = %s", data)
		}
		if n := e.count(`SELECT count(*) FROM cart_item WHERE user_id = $1 AND offer_id = $2 AND quantity = 2`, user, offerID); n != 1 {
			t.Errorf("cart_item en la base = %d", n)
		}
		e.expect(http.StatusNoContent, "comercial", "DELETE", "/cart", s.AccessToken, nil)
		if n := e.count(`SELECT count(*) FROM cart_item WHERE user_id = $1`, user); n != 0 {
			t.Errorf("carrito vaciado = %d ítems", n)
		}
	})

	t.Run("review llama a product y escribe product_review", func(t *testing.T) {
		var review struct {
			ID string `json:"id"`
		}
		e.expect(http.StatusUnauthorized, "comercial", "POST", "/reviews/product/"+productID, "",
			map[string]any{"userId": user, "rating": 4})
		data := e.expect(http.StatusCreated, "comercial", "POST", "/reviews/product/"+productID, s.AccessToken,
			map[string]any{"rating": 4, "title": e.tag, "content": "integración"})
		_ = json.Unmarshal(data, &review)
		if data := e.expect(http.StatusOK, "comercial", "GET", "/reviews/product/"+productID, "", nil); !strings.Contains(string(data), e.tag) {
			t.Errorf("la reseña no aparece en el listado")
		}
		if n := e.count(`SELECT count(*) FROM product_review WHERE user_id = $1 AND title = $2`, user, e.tag); n != 1 {
			t.Errorf("product_review en la base = %d", n)
		}
		e.expect(http.StatusNoContent, "comercial", "DELETE", "/reviews/"+review.ID, s.AccessToken, nil)
	})

	t.Run("watchlist llama a pricing y guarda el precio objetivo", func(t *testing.T) {
		e.expect(http.StatusUnauthorized, "comercial", "GET", fmt.Sprintf("/watchlist?userId=%d", user), "", nil)
		e.expect(http.StatusCreated, "comercial", "POST", "/watchlist", s.AccessToken, map[string]any{"productId": productID, "targetPrice": 1})
		if data := e.expect(http.StatusOK, "comercial", "GET", "/watchlist", s.AccessToken, nil); !strings.Contains(string(data), productID) {
			t.Errorf("GET /watchlist = %s", data)
		}
		if n := e.count(`SELECT count(*) FROM product_user_interest WHERE user_id = $1 AND target_price = 1 AND notify_price_drop`, user); n != 1 {
			t.Errorf("watchlist en la base = %d", n)
		}
	})

	t.Run("notification escribe notification", func(t *testing.T) {
		// Crear es solo interno (pricing e ingestion), ni siquiera el dueño.
		notification := map[string]any{"userId": user, "type": "test", "title": e.tag, "message": "integración"}
		e.expect(http.StatusUnauthorized, "comercial", "POST", "/notifications", "", notification)
		e.expect(http.StatusForbidden, "comercial", "POST", "/notifications", s.AccessToken, notification)
		e.expect(http.StatusCreated, "comercial", "POST", "/notifications", internalToken, notification)
		e.expect(http.StatusUnauthorized, "comercial", "GET", fmt.Sprintf("/notifications?userId=%d", user), "", nil)
		if data := e.expect(http.StatusOK, "comercial", "GET", "/notifications", s.AccessToken, nil); !strings.Contains(string(data), e.tag) {
			t.Errorf("GET /notifications = %s", data)
		}
		e.expect(http.StatusOK, "comercial", "PUT", "/notifications/read-all", s.AccessToken, nil)
		if n := e.count(`SELECT count(*) FROM notification WHERE user_id = $1 AND read_at IS NOT NULL`, user); n != 1 {
			t.Errorf("notificaciones leídas = %d", n)
		}
	})

	t.Run("search guarda el historial con la sesión", func(t *testing.T) {
		t.Cleanup(func() { _, _ = e.pool.Exec(context.Background(), `DELETE FROM search_query WHERE raw_query = $1`, e.tag) })
		e.expect(http.StatusCreated, "comercial", "POST", "/search/history", s.AccessToken, map[string]any{"query": e.tag})
		if n := e.count(`SELECT count(*) FROM search_query WHERE user_id = $1 AND raw_query = $2`, user, e.tag); n != 1 {
			t.Errorf("search_query en la base = %d", n)
		}
	})

	t.Run("pricing escribe product_offer y el trigger el historial", func(t *testing.T) {
		store := e.newStore()
		var offer struct {
			ID int32 `json:"id"`
		}
		data := e.expect(http.StatusCreated, "comercial", "POST", "/offers", internalToken, map[string]any{
			"productId": productID, "storeId": store, "price": 1000, "productUrl": "https://integracion.test/" + e.tag})
		_ = json.Unmarshal(data, &offer)
		e.expect(http.StatusOK, "comercial", "PUT", fmt.Sprintf("/offers/%d", offer.ID), internalToken, map[string]any{"price": 900})
		if n := e.count(`SELECT count(*) FROM product_price_history WHERE product_offer_id = $1`, offer.ID); n != 2 {
			t.Errorf("historial de precios = %d filas, quiero 2 (alta y cambio de precio)", n)
		}
		e.expect(http.StatusUnauthorized, "comercial", "POST", "/offers", "", map[string]any{})
	})
}

// Ingestion guarda las ofertas del scraper y, si el precio baja del objetivo
// de alguien, avisa por HTTP a notification, que corre en comercial. Es la
// única llamada entre dominios.
func TestIntegracion_IngestionNotificaAComercial(t *testing.T) {
	e := newEnv(t)
	s, _ := e.register("caro")
	store := e.newStore()
	source := "Scraper " + e.tag
	t.Cleanup(func() { _, _ = e.pool.Exec(context.Background(), `DELETE FROM data_source WHERE name = $1`, source) })

	// Producto del seed (LG GM53): ingestion lo encuentra por marca y modelo.
	var productID string
	if err := e.pool.QueryRow(context.Background(), `SELECT p.public_id::text FROM product p JOIN brand b ON b.id = p.brand_id
		WHERE b.name = 'LG' AND p.model = 'GM53'`).Scan(&productID); err != nil {
		t.Fatalf("producto del seed: %v", err)
	}
	e.expect(http.StatusCreated, "comercial", "POST", "/watchlist", s.AccessToken, map[string]any{"productId": productID, "targetPrice": 500000})

	var run struct {
		ID int32 `json:"id"`
	}
	data := e.expect(http.StatusCreated, "ingestion", "POST", "/ingestion/runs", internalToken,
		map[string]any{"source": map[string]any{"name": source, "baseUrl": "https://integracion.test"}, "storeId": store})
	_ = json.Unmarshal(data, &run)
	item := func(price int) {
		t.Helper()
		e.expect(http.StatusOK, "ingestion", "POST", fmt.Sprintf("/ingestion/runs/%d/items", run.ID), internalToken, map[string]any{"items": []any{
			map[string]any{"externalId": "lg-gm53", "product": map[string]any{"name": "Refrigerador LG", "brand": "LG", "model": "GM53"},
				"offer": map[string]any{"price": price, "productUrl": "https://integracion.test/" + e.tag}},
		}})
	}

	item(600000) // oferta nueva: no hay precio anterior, no se avisa
	if n := e.count(`SELECT count(*) FROM notification WHERE user_id = $1`, s.UserID); n != 0 {
		t.Fatalf("notificaciones antes de la baja = %d", n)
	}
	item(450000) // baja de 600.000 a 450.000: cruza el objetivo de 500.000
	if n := e.count(`SELECT count(*) FROM notification WHERE user_id = $1 AND type = 'price_drop'`, s.UserID); n != 1 {
		t.Errorf("notificaciones price_drop = %d, quiero 1", n)
	}
	if n := e.count(`SELECT count(*) FROM product_offer o JOIN product p ON p.id = o.product_id
		WHERE o.store_id = $1 AND p.public_id = $2::uuid AND o.price = 450000`, store, productID); n != 1 {
		t.Errorf("oferta actualizada = %d", n)
	}
	if n := e.count(`SELECT count(*) FROM product_price_history h JOIN product_offer o ON o.id = h.product_offer_id
		WHERE o.store_id = $1`, store); n != 2 {
		t.Errorf("historial de la oferta = %d, quiero 2", n)
	}
	e.expect(http.StatusOK, "ingestion", "POST", fmt.Sprintf("/ingestion/runs/%d/finish", run.ID), internalToken, map[string]any{"status": "success"})
	e.expect(http.StatusUnauthorized, "ingestion", "GET", "/ingestion/runs", "", nil)
}

// Cada usuario solo lee y modifica lo suyo: el usuario sale del token, nunca
// del userId que manda el cliente (TAL-125, H1). Antes, cambiar el userId del
// query o del body bastaba para leer o borrar los datos de otro.
func TestIntegracion_AccesoCruzado(t *testing.T) {
	e := newEnv(t)
	ana, _ := e.register("ana-idor")
	beto, _ := e.register("beto-idor")
	productID, _, _ := e.seedOffer()
	var serviceID string
	if err := e.pool.QueryRow(context.Background(), `SELECT public_id::text FROM service WHERE active ORDER BY id LIMIT 1`).Scan(&serviceID); err != nil {
		t.Fatalf("seed sin servicios: %v", err)
	}
	asAna := fmt.Sprintf("?userId=%d", ana.UserID)

	t.Run("review", func(t *testing.T) {
		var review struct {
			ID     string `json:"id"`
			UserID int32  `json:"userId"`
		}
		// El userId del body se ignora: la reseña queda a nombre del token.
		data := e.expect(http.StatusCreated, "comercial", "POST", "/reviews/product/"+productID, ana.AccessToken,
			map[string]any{"userId": beto.UserID, "rating": 5, "title": e.tag})
		_ = json.Unmarshal(data, &review)
		if review.UserID != ana.UserID {
			t.Errorf("autor de la reseña = %d, quiero %d (el del token)", review.UserID, ana.UserID)
		}
		e.expect(http.StatusNotFound, "comercial", "PUT", "/reviews/"+review.ID, beto.AccessToken,
			map[string]any{"userId": ana.UserID, "rating": 1})
		e.expect(http.StatusNotFound, "comercial", "DELETE", "/reviews/"+review.ID+asAna, beto.AccessToken, nil)
		if n := e.count(`SELECT count(*) FROM product_review WHERE user_id = $1 AND rating = 5`, ana.UserID); n != 1 {
			t.Errorf("la reseña de ana cambió: %d filas con rating 5", n)
		}
	})

	t.Run("watchlist", func(t *testing.T) {
		e.expect(http.StatusCreated, "comercial", "POST", "/watchlist", ana.AccessToken, map[string]any{"productId": productID, "targetPrice": 1})
		if data := e.expect(http.StatusOK, "comercial", "GET", "/watchlist"+asAna, beto.AccessToken, nil); string(data) != "[]\n" {
			t.Errorf("beto ve la watchlist de ana: %s", data)
		}
		e.expect(http.StatusNotFound, "comercial", "PUT", "/watchlist/"+productID, beto.AccessToken, map[string]any{"userId": ana.UserID, "targetPrice": 2})
		e.expect(http.StatusNotFound, "comercial", "DELETE", "/watchlist/"+productID+asAna, beto.AccessToken, nil)
		if n := e.count(`SELECT count(*) FROM product_user_interest WHERE user_id = $1 AND target_price = 1 AND notify_price_drop`, ana.UserID); n != 1 {
			t.Errorf("la watchlist de ana cambió")
		}
	})

	t.Run("notification", func(t *testing.T) {
		var notification struct {
			ID int32 `json:"id"`
		}
		data := e.expect(http.StatusCreated, "comercial", "POST", "/notifications", internalToken,
			map[string]any{"userId": ana.UserID, "type": "test", "title": e.tag, "message": "solo para ana"})
		_ = json.Unmarshal(data, &notification)
		if data := e.expect(http.StatusOK, "comercial", "GET", "/notifications"+asAna, beto.AccessToken, nil); strings.Contains(string(data), e.tag) {
			t.Errorf("beto ve las notificaciones de ana: %s", data)
		}
		e.expect(http.StatusNotFound, "comercial", "PUT", fmt.Sprintf("/notifications/%d/read%s", notification.ID, asAna), beto.AccessToken, nil)
		e.expect(http.StatusOK, "comercial", "PUT", "/notifications/read-all"+asAna, beto.AccessToken, map[string]any{"userId": ana.UserID})
		if n := e.count(`SELECT count(*) FROM notification WHERE user_id = $1 AND read_at IS NULL`, ana.UserID); n != 1 {
			t.Errorf("beto marcó como leídas las notificaciones de ana")
		}
	})

	t.Run("comparación de servicios", func(t *testing.T) {
		e.expect(http.StatusUnauthorized, "comercial", "POST", "/comparisons/services", "", map[string]any{"userId": ana.UserID, "serviceIds": []string{serviceID}})
		var comparison struct {
			ID     int32 `json:"id"`
			UserID int32 `json:"userId"`
		}
		data := e.expect(http.StatusCreated, "comercial", "POST", "/comparisons/services", ana.AccessToken,
			map[string]any{"userId": beto.UserID, "serviceIds": []string{serviceID}})
		_ = json.Unmarshal(data, &comparison)
		if comparison.UserID != ana.UserID {
			t.Errorf("dueño de la comparación = %d, quiero %d", comparison.UserID, ana.UserID)
		}
		path := fmt.Sprintf("/comparisons/services/%d", comparison.ID)
		e.expect(http.StatusNotFound, "comercial", "GET", path, beto.AccessToken, nil)
		e.expect(http.StatusNotFound, "comercial", "DELETE", path+"/items/"+serviceID, beto.AccessToken, nil)
		e.expect(http.StatusNotFound, "comercial", "DELETE", path, beto.AccessToken, nil)
		e.expect(http.StatusOK, "comercial", "GET", path, ana.AccessToken, nil)
		if n := e.count(`SELECT count(*) FROM service_comparison_item WHERE comparison_id = $1`, comparison.ID); n != 1 {
			t.Errorf("ítems de la comparación de ana = %d, quiero 1", n)
		}
		e.expect(http.StatusNoContent, "comercial", "DELETE", path, ana.AccessToken, nil)
	})
}

// /health de cada pod dice su dominio, para saber cuál contestó (TAL-125, H6).
func TestIntegracion_HealthPorDominio(t *testing.T) {
	e := newEnv(t)
	for domain := range allapis.Domains {
		data := e.expect(http.StatusOK, domain, "GET", "/health", "", nil)
		if want := `"service":"` + domain + `"`; !strings.Contains(string(data), want) {
			t.Errorf("GET /health de %s = %s, quiero %s", domain, data, want)
		}
	}
}
