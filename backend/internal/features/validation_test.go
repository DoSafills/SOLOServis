package features

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
)

// Validación de entradas (TAL-159): cada caso es una entrada que antes
// llegaba a PostgreSQL y terminaba en 500 (22P02, 22001, 22003) o que la
// base aceptaba sin cumplir el contrato. Ahora responde 400 en la ruta y 422
// con errors[] en el body, sin tocar la base (los casos no declaran reglas).

// badUUID tiene la forma de un UUID (36 caracteres, 4 guiones) pero no es
// hexadecimal: PostgreSQL lo rechaza con 22P02.
const badUUID = "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz"

// misplacedUUID tiene 36 caracteres y 4 guiones, pero el primero está corrido.
const misplacedUUID = "111111112-222-3333-4444-555555555555"

// TestValidacionFavoritos fija que los favoritos rechazan UUID mal formados en la ruta.
func TestValidacionFavoritos(t *testing.T) {
	runCases(t, "favorites-api", []apiCase{
		{name: "add/uuid no hexadecimal", method: http.MethodPost, target: "/favorites/" + badUUID, body: `{}`,
			rules:      []rule{sessionUser},
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
		{name: "add/guion corrido", method: http.MethodPost, target: "/favorites/" + misplacedUUID, body: `{}`,
			rules:      []rule{sessionUser},
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
		{name: "get/uuid no hexadecimal", method: http.MethodGet, target: "/favorites/" + badUUID,
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
		{name: "delete/uuid no hexadecimal", method: http.MethodDelete, target: "/favorites/" + badUUID,
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
		{name: "add service/uuid no hexadecimal", method: http.MethodPost, target: "/favorites/services/" + badUUID, body: `{}`,
			rules:      []rule{sessionUser},
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
	})
}

// TestValidacionResenas fija las reglas de UUID, largo del título y rango de rating de las reseñas.
func TestValidacionResenas(t *testing.T) {
	longTitle := `{"rating":4,"title":"` + strings.Repeat("a", maxReviewTitle+1) + `"}`
	// El largo se cuenta en caracteres: 200 "ñ" ocupan 400 bytes y son válidos.
	limitTitle := `{"rating":4,"title":"` + strings.Repeat("ñ", maxReviewTitle) + `"}`

	runCases(t, "review-api", []apiCase{
		{name: "create product/uuid no hexadecimal", method: http.MethodPost, target: "/reviews/product/" + badUUID,
			body: `{"rating":4}`, wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
		{name: "create product/title largo", method: http.MethodPost, target: "/reviews/product/" + productUUID,
			body: longTitle, wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"title/max_length"}},
		{name: "create product/title en el límite", method: http.MethodPost, target: "/reviews/product/" + productUUID,
			body: limitTitle, remote: remotes("/products/"+productUUID, `{}`),
			rules:      []rule{productRow, qRow("INSERT INTO product_review", `{"id":"product:1","rating":4}`)},
			wantStatus: http.StatusCreated, wantBody: `"rating":4`},
		{name: "create store/title largo", method: http.MethodPost, target: "/reviews/store/3",
			body: longTitle, wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"title/max_length"}},
		{name: "create service/uuid no hexadecimal", method: http.MethodPost, target: "/reviews/service/" + badUUID,
			body: `{"rating":4}`, wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
		{name: "update/title largo y rating fuera de rango", method: http.MethodPut, target: "/reviews/product:1",
			body:       `{"rating":9,"title":"` + strings.Repeat("a", maxReviewTitle+1) + `"}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"rating/max", "title/max_length"}},
		{name: "list product/uuid no hexadecimal", method: http.MethodGet, target: "/reviews/product/" + badUUID, token: "-",
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
	})
}

// TestValidacionWatchlist fija las reglas de productId y del precio objetivo de la watchlist.
func TestValidacionWatchlist(t *testing.T) {
	runCases(t, "watchlist-api", []apiCase{
		{name: "add/productId no es uuid", method: http.MethodPost, target: "/watchlist",
			body:       `{"productId":"` + badUUID + `","targetPrice":1000}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"productId/format"}},
		{name: "add/targetPrice sobre el máximo", method: http.MethodPost, target: "/watchlist",
			body:       `{"productId":"` + productUUID + `","targetPrice":10000000000}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"targetPrice/max"}},
		{name: "add/targetPrice negativo", method: http.MethodPost, target: "/watchlist",
			body:       `{"productId":"` + productUUID + `","targetPrice":-1}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"targetPrice/min"}},
		{name: "update/targetPrice sobre el máximo", method: http.MethodPut, target: "/watchlist/" + productUUID,
			body:       `{"targetPrice":1e12}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"targetPrice/max"}},
		{name: "update/uuid no hexadecimal", method: http.MethodPut, target: "/watchlist/" + badUUID,
			body: `{"targetPrice":1000}`, wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
		{name: "delete/uuid no hexadecimal", method: http.MethodDelete, target: "/watchlist/" + badUUID,
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
	})
}

// TestValidacionNotificaciones fija las reglas de largo de type y title y de forma de data al crear notificaciones.
func TestValidacionNotificaciones(t *testing.T) {
	notification := func(typ, title, data string) string {
		return `{"userId":7,"type":"` + typ + `","title":"` + title + `","message":"m","data":` + data + `}`
	}
	insert := qRows("INSERT INTO notification", `{"id":1,"type":"price_drop"}`)

	runCases(t, "notification-api", []apiCase{
		{name: "create/type y title largos", method: http.MethodPost, target: "/notifications", token: serviceToken,
			body:       notification(strings.Repeat("t", maxNotificationType+1), strings.Repeat("a", maxNotificationTitle+1), `{}`),
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"type/max_length", "title/max_length"}},
		{name: "create/data arreglo", method: http.MethodPost, target: "/notifications", token: serviceToken,
			body:       notification("price_drop", "Bajó", `[1,2]`),
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"data/invalid_type"}},
		{name: "create/data texto", method: http.MethodPost, target: "/notifications", token: serviceToken,
			body:       notification("price_drop", "Bajó", `"x"`),
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"data/invalid_type"}},
		{name: "create/data null", method: http.MethodPost, target: "/notifications", token: serviceToken,
			body: notification("price_drop", "Bajó", `null`), rules: []rule{insert},
			wantStatus: http.StatusCreated, wantBody: `"price_drop"`},
	})
}

// TestValidacionCarrito fija las reglas de quantity y productId del carrito.
func TestValidacionCarrito(t *testing.T) {
	item := func(quantity string) string {
		return `{"productId":"` + productUUID + `","offerId":5,"quantity":` + quantity + `}`
	}
	offer := qRow("FROM product_offer o JOIN product p", int32(10), productUUID, 990.0, 0.0, true)

	runCases(t, "cart-api", []apiCase{
		{name: "add/quantity sobre el máximo", method: http.MethodPost, target: "/cart/items", body: item("100"),
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"quantity/max"}},
		{name: "add/quantity que desborda int32", method: http.MethodPost, target: "/cart/items", body: item("3000000000"),
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidType},
		{name: "add/productId no es uuid", method: http.MethodPost, target: "/cart/items",
			body:       `{"productId":"` + badUUID + `","offerId":5,"quantity":1}`,
			rules:      []rule{sessionUser, offer},
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"productId/format"}},
		{name: "update/quantity sobre el máximo", method: http.MethodPut, target: "/cart/items/1", body: `{"quantity":100}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"quantity/max"}},
		{name: "update/quantity en el límite", method: http.MethodPut, target: "/cart/items/1", body: `{"quantity":99}`,
			rules:      []rule{qExec("UPDATE cart_item SET quantity", 1)},
			wantStatus: http.StatusOK, wantBody: `"quantity":99`},
	})
}

// TestValidacionBusquedasGuardadas fija las reglas de type en el historial y de name y filters en las búsquedas guardadas.
func TestValidacionBusquedasGuardadas(t *testing.T) {
	runCases(t, "search-api", []apiCase{
		{name: "history/type desconocido", method: http.MethodPost, target: "/search/history",
			body:       `{"query":"tv","type":"otro"}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"type/one_of"}},
		{name: "history/type mixed", method: http.MethodPost, target: "/search/history",
			body:       `{"query":"tv","type":"mixed"}`,
			rules:      []rule{trimHistory, qRows("INSERT INTO search_query", `{"id":1,"type":"mixed"}`)},
			wantStatus: http.StatusCreated, wantBody: `"mixed"`},
		{name: "saved/name largo", method: http.MethodPost, target: "/search/saved",
			body:       `{"name":"` + strings.Repeat("a", maxSavedSearchName+1) + `"}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"name/max_length"}},
		{name: "saved/filters arreglo", method: http.MethodPost, target: "/search/saved",
			body:       `{"name":"TVs","filters":["LG"]}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"filters/invalid_type"}},
		{name: "saved/filters número", method: http.MethodPost, target: "/search/saved",
			body:       `{"name":"TVs","filters":3}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"filters/invalid_type"}},
		{name: "saved/filters null", method: http.MethodPost, target: "/search/saved",
			body:       `{"name":"TVs","filters":null}`,
			rules:      []rule{savedCount, qRows("INSERT INTO saved_search", `{"id":1,"name":"TVs","filters":{}}`)},
			wantStatus: http.StatusCreated, wantBody: `"TVs"`},
	})
}

// TestValidacionComparacionServicios fija las reglas de nombre, servicios y tope de las comparaciones de servicios.
func TestValidacionComparacionServicios(t *testing.T) {
	other := "aaaaaaaa-bbbb-cccc-dddd-000000000001"
	ids := func(values ...string) string {
		encoded, _ := json.Marshal(values)
		return `{"serviceIds":` + string(encoded) + `}`
	}

	runCases(t, "comparison-api", []apiCase{
		{name: "create/más de 3 servicios", method: http.MethodPost, target: "/comparisons/services",
			body:       ids(serviceUUID, other, "aaaaaaaa-bbbb-cccc-dddd-000000000002", "aaaaaaaa-bbbb-cccc-dddd-000000000003"),
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"serviceIds/max"}},
		{name: "create/servicio repetido", method: http.MethodPost, target: "/comparisons/services",
			body:       ids(serviceUUID, strings.ToUpper(serviceUUID)),
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"serviceIds/unique"}},
		{name: "create/serviceId repite serviceIds", method: http.MethodPost, target: "/comparisons/services",
			body:       `{"serviceIds":["` + serviceUUID + `"],"serviceId":"` + serviceUUID + `"}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"serviceIds/unique"}},
		{name: "create/name largo", method: http.MethodPost, target: "/comparisons/services",
			body:       `{"name":"` + strings.Repeat("a", maxServiceComparison+1) + `","serviceIds":["` + serviceUUID + `"]}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"name/max_length"}},
		{name: "create/uuid no hexadecimal", method: http.MethodPost, target: "/comparisons/services",
			body: ids(badUUID), rules: []rule{sessionUser},
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"serviceIds.0/format"}},
		{name: "delete item/uuid no hexadecimal", method: http.MethodDelete, target: "/comparisons/services/9/items/" + badUUID,
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
	})
}

// TestIsUUID fija qué textos acepta isUUID como UUID.
func TestIsUUID(t *testing.T) {
	cases := []struct {
		value string
		want  bool
	}{
		{productUUID, true},
		{strings.ToUpper(serviceUUID), true},
		{badUUID, false},
		{misplacedUUID, false},
		{"11111111222233334444555555555555", false},
		{productUUID + "0", false},
		{"", false},
		{"12", false},
	}
	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			if got := isUUID(tc.value); got != tc.want {
				t.Fatalf("isUUID(%q) = %v, quiero %v", tc.value, got, tc.want)
			}
		})
	}
}

// TestIsJSONObjectYRawJSON fija qué acepta isJSONObject y qué guarda rawJSON para filters.
func TestIsJSONObjectYRawJSON(t *testing.T) {
	cases := []struct {
		name       string
		raw        string
		wantObject bool
		wantStored string
	}{
		{name: "omitido", raw: "", wantObject: true, wantStored: "{}"},
		{name: "null", raw: "null", wantObject: true, wantStored: "{}"},
		{name: "objeto", raw: ` {"a":1}`, wantObject: true, wantStored: ` {"a":1}`},
		{name: "arreglo", raw: "[1]", wantObject: false, wantStored: "[1]"},
		{name: "texto", raw: `"x"`, wantObject: false, wantStored: `"x"`},
		{name: "número", raw: "3", wantObject: false, wantStored: "3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isJSONObject(json.RawMessage(tc.raw)); got != tc.wantObject {
				t.Fatalf("isJSONObject(%q) = %v, quiero %v", tc.raw, got, tc.wantObject)
			}
			if got := rawJSON(json.RawMessage(tc.raw)); got != tc.wantStored {
				t.Fatalf("rawJSON(%q) = %q, quiero %q", tc.raw, got, tc.wantStored)
			}
		})
	}
}
