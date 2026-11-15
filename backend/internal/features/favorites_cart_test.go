package features

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
)

func TestFavoritesAPI(t *testing.T) {
	favorite := "/favorites/" + productUUID
	serviceFavorite := "/favorites/services/" + serviceUUID
	productAPI := remotes("/products/"+productUUID, `{}`)
	saveFavorite := "INSERT INTO product_user_interest"
	unfavorite := "UPDATE product_user_interest SET is_favorite=FALSE"
	saveServiceFavorite := "INSERT INTO service_user_interest"
	unfavoriteService := "UPDATE service_user_interest"

	runCases(t, "favorites-api", []apiCase{
		// GET /favorites
		{name: "list/feliz", method: http.MethodGet, target: "/favorites",
			rules:      []rule{qRows("FROM product_user_interest i JOIN product p", `{"productId":"`+productUUID+`"}`)},
			wantStatus: http.StatusOK, wantBody: productUUID},
		{name: "list/sin sesión", method: http.MethodGet, target: "/favorites", token: "-",
			wantStatus: http.StatusUnauthorized, wantCode: httpx.CodeUnauthorized},
		{name: "list/error interno", method: http.MethodGet, target: "/favorites",
			rules:      []rule{qFail("FROM product_user_interest", errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// POST /favorites/{productId}
		{name: "add/feliz", method: http.MethodPost, target: favorite, body: `{"userId":7}`, remote: productAPI,
			rules:      []rule{sessionUser, productRow, qExec(saveFavorite, 1)},
			wantStatus: http.StatusCreated, wantBody: `"favorite":true`},
		{name: "add/json malformado", method: http.MethodPost, target: favorite, body: `{"userId":`,
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeMalformedJSON},
		{name: "add/id inválido", method: http.MethodPost, target: "/favorites/no-es-id", body: `{}`,
			rules:      []rule{sessionUser},
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
		{name: "add/producto inexistente", method: http.MethodPost, target: favorite, body: `{}`,
			rules:      []rule{sessionUser, qNoRows("FROM product WHERE public_id")},
			wantStatus: http.StatusNotFound, wantCode: httpx.CodeNotFound},
		{name: "add/product api caída", method: http.MethodPost, target: favorite, body: `{}`,
			rules:      []rule{sessionUser, productRow},
			wantStatus: http.StatusServiceUnavailable, wantCode: httpx.CodeServiceUnavailable},
		{name: "add/error interno", method: http.MethodPost, target: favorite, body: `{}`, remote: productAPI,
			rules:      []rule{sessionUser, productRow, qFail(saveFavorite, errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// GET /favorites/{productId}
		{name: "get/feliz", method: http.MethodGet, target: favorite,
			rules:      []rule{productRow, qRows("FROM product p LEFT JOIN", `{"favorite":true}`)},
			wantStatus: http.StatusOK, wantBody: `"favorite":true`},
		{name: "get/id inválido", method: http.MethodGet, target: "/favorites/no-es-id",
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
		{name: "get/no es favorito", method: http.MethodGet, target: favorite,
			rules:      []rule{productRow, qRows("FROM product p LEFT JOIN")},
			wantStatus: http.StatusNotFound, wantCode: httpx.CodeNotFound},
		{name: "get/error interno", method: http.MethodGet, target: favorite,
			rules:      []rule{productRow, qFail("FROM product p LEFT JOIN", errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// DELETE /favorites/{productId}
		{name: "delete/feliz", method: http.MethodDelete, target: favorite,
			rules:      []rule{productRow, qExec(unfavorite, 1)},
			wantStatus: http.StatusNoContent},
		{name: "delete/id inválido", method: http.MethodDelete, target: "/favorites/no-es-id",
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
		{name: "delete/no es favorito", method: http.MethodDelete, target: favorite,
			rules:      []rule{productRow, qExec(unfavorite, 0)},
			wantStatus: http.StatusNotFound, wantCode: httpx.CodeNotFound},
		{name: "delete/error interno", method: http.MethodDelete, target: favorite,
			rules:      []rule{productRow, qFail(unfavorite, errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// GET /favorites/services
		{name: "list services/feliz", method: http.MethodGet, target: "/favorites/services",
			rules:      []rule{qRows("FROM service_user_interest", `{"serviceId":"`+serviceUUID+`"}`)},
			wantStatus: http.StatusOK, wantBody: serviceUUID},
		{name: "list services/error interno", method: http.MethodGet, target: "/favorites/services",
			rules:      []rule{qFail("FROM service_user_interest", errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// POST /favorites/services/{serviceId}
		{name: "add service/feliz", method: http.MethodPost, target: serviceFavorite, body: `{"userId":7}`,
			rules:      []rule{sessionUser, serviceRow, qExec(saveServiceFavorite, 1)},
			wantStatus: http.StatusCreated, wantBody: `"favorite":true`},
		{name: "add service/campo desconocido", method: http.MethodPost, target: serviceFavorite, body: `{"usuario":7}`,
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeUnknownField, wantFields: []string{"usuario/unknown_field"}},
		{name: "add service/servicio inexistente", method: http.MethodPost, target: serviceFavorite, body: `{}`,
			rules:      []rule{sessionUser, qNoRows("SELECT id FROM service WHERE public_id")},
			wantStatus: http.StatusNotFound, wantCode: httpx.CodeNotFound},
		{name: "add service/error interno", method: http.MethodPost, target: serviceFavorite, body: `{}`,
			rules:      []rule{sessionUser, serviceRow, qFail(saveServiceFavorite, errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// DELETE /favorites/services/{serviceId}
		{name: "delete service/feliz", method: http.MethodDelete, target: serviceFavorite,
			rules:      []rule{serviceRow, qExec(unfavoriteService, 1)},
			wantStatus: http.StatusNoContent},
		{name: "delete service/id inválido", method: http.MethodDelete, target: "/favorites/services/no-es-id",
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
		{name: "delete service/no es favorito", method: http.MethodDelete, target: serviceFavorite,
			rules:      []rule{serviceRow, qExec(unfavoriteService, 0)},
			wantStatus: http.StatusNotFound, wantCode: httpx.CodeNotFound},
		{name: "delete service/error interno", method: http.MethodDelete, target: serviceFavorite,
			rules:      []rule{serviceRow, qFail(unfavoriteService, errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},
	})
}

func TestCartAPI(t *testing.T) {
	productItem := `{"userId":7,"productId":"` + productUUID + `","offerId":5,"quantity":2}`
	serviceItem := `{"serviceId":"` + serviceUUID + `","serviceOfferId":4,"quantity":1}`
	offer := "FROM product_offer o JOIN product p"
	availableOffer := qRow(offer, int32(10), productUUID, 990.0, 0.0, true)
	offerAPIs := remotes("/products/"+productUUID, `{}`, "/offers/5", `{"productId":"`+productUUID+`"}`)
	serviceOffer := "FROM service_offer so JOIN service s"
	insertItem := "INSERT INTO cart_item"
	updateItem := "UPDATE cart_item SET quantity"
	deleteItem := "DELETE FROM cart_item WHERE id"
	tooLarge := `{"productId":"` + strings.Repeat("x", int(httpx.DefaultMaxBodyBytes)) + `"}`

	runCases(t, "cart-api", []apiCase{
		// GET /cart
		{name: "get/feliz", method: http.MethodGet, target: "/cart",
			rules:      []rule{qRows("FROM cart_item ci", `{"id":1,"quantity":2}`)},
			wantStatus: http.StatusOK, wantBody: `"quantity":2`},
		{name: "get/sin sesión", method: http.MethodGet, target: "/cart", token: "-",
			wantStatus: http.StatusUnauthorized, wantCode: httpx.CodeUnauthorized},
		{name: "get/error interno", method: http.MethodGet, target: "/cart",
			rules:      []rule{qFail("FROM cart_item ci", errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// POST /cart/items (producto)
		{name: "add product/feliz", method: http.MethodPost, target: "/cart/items", body: productItem, remote: offerAPIs,
			rules:      []rule{sessionUser, availableOffer, productRow, qExec(insertItem, 1)},
			wantStatus: http.StatusCreated, wantBody: `"savedPrice":990`},
		{name: "add product/json malformado", method: http.MethodPost, target: "/cart/items", body: `{"quantity":`,
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeMalformedJSON},
		{name: "add product/tipo inválido", method: http.MethodPost, target: "/cart/items", body: `{"quantity":"dos"}`,
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidType, wantFields: []string{"quantity/invalid_type"}},
		{name: "add product/content-type incorrecto", method: http.MethodPost, target: "/cart/items", body: productItem, contentType: "text/plain",
			wantStatus: http.StatusUnsupportedMediaType, wantCode: httpx.CodeUnsupportedMediaType},
		{name: "add product/body excedido", method: http.MethodPost, target: "/cart/items", body: tooLarge,
			wantStatus: http.StatusRequestEntityTooLarge, wantCode: httpx.CodeBodyTooLarge},
		{name: "add product/reglas", method: http.MethodPost, target: "/cart/items", body: `{"quantity":0}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"quantity/min", "productId/one_of"}},
		{name: "add product/oferta inexistente", method: http.MethodPost, target: "/cart/items", body: productItem,
			rules:      []rule{sessionUser, qNoRows(offer)},
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed, wantFields: []string{"offerId/not_found"}},
		{name: "add product/oferta no disponible", method: http.MethodPost, target: "/cart/items", body: productItem,
			rules:      []rule{sessionUser, qRow(offer, int32(10), productUUID, 990.0, 0.0, false)},
			wantStatus: http.StatusConflict, wantCode: httpx.CodeConflict},
		{name: "add product/oferta de otro producto", method: http.MethodPost, target: "/cart/items", body: productItem,
			rules:      []rule{sessionUser, availableOffer, qRow("FROM product WHERE public_id", int32(99))},
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed, wantFields: []string{"offerId/mismatch"}},
		{name: "add product/pricing api caída", method: http.MethodPost, target: "/cart/items", body: productItem,
			remote:     remotes("/products/"+productUUID, `{}`),
			rules:      []rule{sessionUser, availableOffer, productRow},
			wantStatus: http.StatusServiceUnavailable, wantCode: httpx.CodeServiceUnavailable},
		{name: "add product/pricing devuelve otro producto", method: http.MethodPost, target: "/cart/items", body: productItem,
			remote:     remotes("/products/"+productUUID, `{}`, "/offers/5", `{"productId":"otro"}`),
			rules:      []rule{sessionUser, availableOffer, productRow},
			wantStatus: http.StatusConflict, wantCode: httpx.CodeConflict},
		{name: "add product/error interno", method: http.MethodPost, target: "/cart/items", body: productItem, remote: offerAPIs,
			rules:      []rule{sessionUser, availableOffer, productRow, qFail(insertItem, errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},
		{name: "add product/falla el commit", method: http.MethodPost, target: "/cart/items", body: productItem, remote: offerAPIs,
			rules:      []rule{sessionUser, availableOffer, productRow, qExec(insertItem, 1)},
			commitErr:  errors.New("commit failed"),
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// POST /cart/items (servicio)
		{name: "add service/feliz", method: http.MethodPost, target: "/cart/items", body: serviceItem,
			rules:      []rule{sessionUser, serviceRow, qRow(serviceOffer, serviceUUID, int32(20), 19990.0), qExec(insertItem, 1)},
			wantStatus: http.StatusCreated, wantBody: `"serviceOfferId":4`},
		{name: "add service/servicio inexistente", method: http.MethodPost, target: "/cart/items", body: serviceItem,
			rules:      []rule{sessionUser, qNoRows("SELECT id FROM service WHERE public_id")},
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed, wantFields: []string{"serviceId/not_found"}},
		{name: "add service/oferta de otro servicio", method: http.MethodPost, target: "/cart/items", body: serviceItem,
			rules:      []rule{sessionUser, serviceRow, qRow(serviceOffer, serviceUUID, int32(99), 19990.0)},
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed, wantFields: []string{"serviceOfferId/mismatch"}},
		{name: "add service/error interno", method: http.MethodPost, target: "/cart/items", body: serviceItem,
			rules:      []rule{sessionUser, serviceRow, qFail(serviceOffer, errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// PUT /cart/items/{id}
		{name: "update/feliz", method: http.MethodPut, target: "/cart/items/3", body: `{"quantity":4}`,
			rules:      []rule{qExec(updateItem, 1)},
			wantStatus: http.StatusOK, wantBody: `"quantity":4`},
		{name: "update/id inválido", method: http.MethodPut, target: "/cart/items/abc", body: `{"quantity":4}`,
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
		{name: "update/reglas", method: http.MethodPut, target: "/cart/items/3", body: `{"quantity":0}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed, wantFields: []string{"quantity/min"}},
		{name: "update/inexistente", method: http.MethodPut, target: "/cart/items/3", body: `{"quantity":4}`,
			rules:      []rule{qExec(updateItem, 0)},
			wantStatus: http.StatusNotFound, wantCode: httpx.CodeNotFound},
		{name: "update/error interno", method: http.MethodPut, target: "/cart/items/3", body: `{"quantity":4}`,
			rules:      []rule{qFail(updateItem, errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// DELETE /cart/items/{id}
		{name: "delete item/feliz", method: http.MethodDelete, target: "/cart/items/3",
			rules:      []rule{qExec(deleteItem, 1)},
			wantStatus: http.StatusNoContent},
		{name: "delete item/id inválido", method: http.MethodDelete, target: "/cart/items/0",
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
		{name: "delete item/inexistente", method: http.MethodDelete, target: "/cart/items/3",
			rules:      []rule{qExec(deleteItem, 0)},
			wantStatus: http.StatusNotFound, wantCode: httpx.CodeNotFound},
		{name: "delete item/error interno", method: http.MethodDelete, target: "/cart/items/3",
			rules:      []rule{qFail(deleteItem, errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// DELETE /cart
		{name: "clear/feliz", method: http.MethodDelete, target: "/cart",
			rules:      []rule{qExec("DELETE FROM cart_item WHERE user_id", 2)},
			wantStatus: http.StatusNoContent},
		{name: "clear/error interno", method: http.MethodDelete, target: "/cart",
			rules:      []rule{qFail("DELETE FROM cart_item WHERE user_id", errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// GET /cart/count
		{name: "count/feliz", method: http.MethodGet, target: "/cart/count",
			rules:      []rule{qRow("SUM(quantity)", int32(3))},
			wantStatus: http.StatusOK, wantBody: `"count":3`},
		{name: "count/error interno", method: http.MethodGet, target: "/cart/count",
			rules:      []rule{qFail("SUM(quantity)", errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},
	})
}
