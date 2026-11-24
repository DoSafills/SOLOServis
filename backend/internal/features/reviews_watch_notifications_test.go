package features

import (
	"net/http"
	"testing"

	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
)

func TestReviewAPI(t *testing.T) {
	review := `{"userId":7,"rating":5,"title":"Bueno","content":"Llegó rápido"}`
	productReviews := "/reviews/product/" + productUUID
	serviceReviews := "/reviews/service/" + serviceUUID
	saved := `{"id":"product:1","rating":5}`
	insertProductReview := "INSERT INTO product_review"
	userFK := pgError("23503", `Key (user_id)=(7) is not present in table "user_account".`)

	runCases(t, "review-api", []apiCase{
		// GET /reviews/product/{productId}
		{name: "list product/feliz", method: http.MethodGet, target: productReviews, token: "-",
			rules:      []rule{productRow, qRows("FROM product_review", saved)},
			wantStatus: http.StatusOK, wantBody: `"product:1"`},
		{name: "list product/id inválido", method: http.MethodGet, target: "/reviews/product/xx", token: "-",
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
		{name: "list product/producto inexistente", method: http.MethodGet, target: productReviews, token: "-",
			rules:      []rule{qNoRows("FROM product WHERE public_id")},
			wantStatus: http.StatusNotFound, wantCode: httpx.CodeNotFound},
		{name: "list product/error interno", method: http.MethodGet, target: productReviews, token: "-",
			rules:      []rule{productRow, qFail("FROM product_review", errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// POST /reviews/product/{productId}
		{name: "create product/feliz", method: http.MethodPost, target: productReviews, body: review,
			remote:     remotes("/products/"+productUUID, `{}`),
			rules:      []rule{productRow, qRow(insertProductReview, saved)},
			wantStatus: http.StatusCreated, wantBody: `"rating":5`},
		{name: "create product/sin sesión", method: http.MethodPost, target: productReviews, body: review, token: "-",
			wantStatus: http.StatusUnauthorized, wantCode: httpx.CodeUnauthorized},
		{name: "create product/json malformado", method: http.MethodPost, target: productReviews, body: `{"rating":`,
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeMalformedJSON},
		{name: "create product/rating mayor que 5", method: http.MethodPost, target: productReviews, body: `{"rating":9}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed, wantFields: []string{"rating/max"}},
		{name: "create product/rating 0", method: http.MethodPost, target: productReviews, body: `{"rating":0}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed, wantFields: []string{"rating/min"}},
		{name: "create product/producto inexistente", method: http.MethodPost, target: productReviews, body: review,
			rules:      []rule{qNoRows("FROM product WHERE public_id")},
			wantStatus: http.StatusNotFound, wantCode: httpx.CodeNotFound},
		{name: "create product/product api caída", method: http.MethodPost, target: productReviews, body: review,
			rules:      []rule{productRow},
			wantStatus: http.StatusServiceUnavailable, wantCode: httpx.CodeServiceUnavailable},
		{name: "create product/usuario inexistente (FK)", method: http.MethodPost, target: productReviews, body: review,
			remote:     remotes("/products/"+productUUID, `{}`),
			rules:      []rule{productRow, qFail(insertProductReview, userFK)},
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed, wantFields: []string{"userId/not_found"}},
		{name: "create product/error interno", method: http.MethodPost, target: productReviews, body: review,
			remote:     remotes("/products/"+productUUID, `{}`),
			rules:      []rule{productRow, qFail(insertProductReview, errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// GET /reviews/store/{storeId}
		{name: "list store/feliz", method: http.MethodGet, target: "/reviews/store/3", token: "-",
			rules:      []rule{qRows("FROM store_review", `{"id":"store:1"}`)},
			wantStatus: http.StatusOK, wantBody: `"store:1"`},
		{name: "list store/id inválido", method: http.MethodGet, target: "/reviews/store/x", token: "-",
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
		{name: "list store/error interno", method: http.MethodGet, target: "/reviews/store/3", token: "-",
			rules:      []rule{qFail("FROM store_review", errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// POST /reviews/store/{storeId}
		{name: "create store/feliz", method: http.MethodPost, target: "/reviews/store/3", body: review,
			remote:     remotes("/stores/3", `{}`),
			rules:      []rule{qRow("INSERT INTO store_review", `{"id":"store:1"}`)},
			wantStatus: http.StatusCreated, wantBody: `"store:1"`},
		{name: "create store/id inválido", method: http.MethodPost, target: "/reviews/store/x", body: review,
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
		{name: "create store/reglas", method: http.MethodPost, target: "/reviews/store/3", body: `{"rating":6}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed, wantFields: []string{"rating/max"}},
		{name: "create store/tienda inexistente o store api caída", method: http.MethodPost, target: "/reviews/store/3", body: review,
			wantStatus: http.StatusServiceUnavailable, wantCode: httpx.CodeServiceUnavailable},
		{name: "create store/error interno", method: http.MethodPost, target: "/reviews/store/3", body: review,
			remote:     remotes("/stores/3", `{}`),
			rules:      []rule{qFail("INSERT INTO store_review", errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// GET /reviews/service/{serviceId}
		{name: "list service/feliz", method: http.MethodGet, target: serviceReviews, token: "-",
			rules:      []rule{serviceRow, qRows("FROM service_review", `{"id":"service:1"}`)},
			wantStatus: http.StatusOK, wantBody: `"service:1"`},
		{name: "list service/id inválido", method: http.MethodGet, target: "/reviews/service/xx", token: "-",
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
		{name: "list service/servicio inexistente", method: http.MethodGet, target: serviceReviews, token: "-",
			rules:      []rule{qNoRows("SELECT id FROM service WHERE public_id")},
			wantStatus: http.StatusNotFound, wantCode: httpx.CodeNotFound},
		{name: "list service/error interno", method: http.MethodGet, target: serviceReviews, token: "-",
			rules:      []rule{serviceRow, qFail("FROM service_review", errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// POST /reviews/service/{serviceId}
		{name: "create service/feliz", method: http.MethodPost, target: serviceReviews, body: review,
			remote:     remotes("/services/"+serviceUUID, `{}`),
			rules:      []rule{serviceRow, qRow("INSERT INTO service_review", `{"id":"service:1"}`)},
			wantStatus: http.StatusCreated, wantBody: `"service:1"`},
		{name: "create service/reglas", method: http.MethodPost, target: serviceReviews, body: `{}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed, wantFields: []string{"rating/min"}},
		{name: "create service/servicio inexistente", method: http.MethodPost, target: serviceReviews, body: review,
			rules:      []rule{qNoRows("SELECT id FROM service WHERE public_id")},
			wantStatus: http.StatusNotFound, wantCode: httpx.CodeNotFound},
		{name: "create service/service api caída", method: http.MethodPost, target: serviceReviews, body: review,
			rules:      []rule{serviceRow},
			wantStatus: http.StatusServiceUnavailable, wantCode: httpx.CodeServiceUnavailable},
		{name: "create service/error interno", method: http.MethodPost, target: serviceReviews, body: review,
			remote:     remotes("/services/"+serviceUUID, `{}`),
			rules:      []rule{serviceRow, qFail("INSERT INTO service_review", errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// PUT /reviews/{id}
		{name: "update/feliz", method: http.MethodPut, target: "/reviews/product:4", body: review,
			rules:      []rule{qExec("UPDATE product_review", 1)},
			wantStatus: http.StatusOK, wantBody: `"updated":true`},
		{name: "update/id inválido", method: http.MethodPut, target: "/reviews/otro:4", body: review,
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
		{name: "update/reglas", method: http.MethodPut, target: "/reviews/product:4", body: `{"rating":-1}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed, wantFields: []string{"rating/min"}},
		{name: "update/reseña ajena o inexistente", method: http.MethodPut, target: "/reviews/product:4", body: review,
			rules:      []rule{qExec("UPDATE product_review", 0)},
			wantStatus: http.StatusNotFound, wantCode: httpx.CodeNotFound},
		{name: "update/error interno", method: http.MethodPut, target: "/reviews/product:4", body: review,
			rules:      []rule{qFail("UPDATE product_review", errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// DELETE /reviews/{id}
		{name: "delete/feliz", method: http.MethodDelete, target: "/reviews/store:4",
			rules:      []rule{qExec("DELETE FROM store_review", 1)},
			wantStatus: http.StatusNoContent},
		{name: "delete/id inválido", method: http.MethodDelete, target: "/reviews/store:0",
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
		{name: "delete/inexistente", method: http.MethodDelete, target: "/reviews/store:4",
			rules:      []rule{qExec("DELETE FROM store_review", 0)},
			wantStatus: http.StatusNotFound, wantCode: httpx.CodeNotFound},
		{name: "delete/error interno", method: http.MethodDelete, target: "/reviews/store:4",
			rules:      []rule{qFail("DELETE FROM store_review", errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},
	})
}

func TestWatchlistAPI(t *testing.T) {
	item := `{"userId":7,"productId":"` + productUUID + `","targetPrice":500}`
	pricing := remotes("/offers/product/"+productUUID, `[]`)
	watched := "/watchlist/" + productUUID
	insertWatch := "INSERT INTO product_user_interest"
	updateTarget := "UPDATE product_user_interest SET target_price"
	unwatch := "UPDATE product_user_interest SET notify_price_drop=FALSE"

	runCases(t, "watchlist-api", []apiCase{
		// GET /watchlist
		{name: "get/feliz", method: http.MethodGet, target: "/watchlist",
			rules:      []rule{qRows("FROM product_user_interest i JOIN product p", `{"targetPrice":500}`)},
			wantStatus: http.StatusOK, wantBody: `"targetPrice":500`},
		{name: "get/error interno", method: http.MethodGet, target: "/watchlist",
			rules:      []rule{qFail("FROM product_user_interest", errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// POST /watchlist
		{name: "add/feliz", method: http.MethodPost, target: "/watchlist", body: item, remote: pricing,
			rules:      []rule{productRow, qExec(insertWatch, 1)},
			wantStatus: http.StatusCreated, wantBody: `"targetPrice":500`},
		{name: "add/json malformado", method: http.MethodPost, target: "/watchlist", body: `[`,
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeMalformedJSON},
		{name: "add/reglas", method: http.MethodPost, target: "/watchlist", body: `{"productId":"","targetPrice":-1}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"productId/required", "targetPrice/min"}},
		{name: "add/productId mal formado", method: http.MethodPost, target: "/watchlist", body: `{"productId":"abc"}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed, wantFields: []string{"productId/format"}},
		{name: "add/producto inexistente", method: http.MethodPost, target: "/watchlist", body: item,
			rules:      []rule{qNoRows("FROM product WHERE public_id")},
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed, wantFields: []string{"productId/not_found"}},
		{name: "add/pricing api caída", method: http.MethodPost, target: "/watchlist", body: item,
			rules:      []rule{productRow},
			wantStatus: http.StatusServiceUnavailable, wantCode: httpx.CodeServiceUnavailable},
		{name: "add/error interno", method: http.MethodPost, target: "/watchlist", body: item, remote: pricing,
			rules:      []rule{productRow, qFail(insertWatch, errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// PUT /watchlist/{productId}
		{name: "update/feliz", method: http.MethodPut, target: watched, body: `{"targetPrice":400}`, remote: pricing,
			rules:      []rule{productRow, qExec(updateTarget, 1)},
			wantStatus: http.StatusOK, wantBody: `"targetPrice":400`},
		{name: "update/reglas", method: http.MethodPut, target: watched, body: `{"targetPrice":-1}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed, wantFields: []string{"targetPrice/min"}},
		{name: "update/producto inexistente", method: http.MethodPut, target: watched, body: `{"targetPrice":400}`,
			rules:      []rule{qNoRows("FROM product WHERE public_id")},
			wantStatus: http.StatusNotFound, wantCode: httpx.CodeNotFound},
		{name: "update/no está en la watchlist", method: http.MethodPut, target: watched, body: `{"targetPrice":400}`, remote: pricing,
			rules:      []rule{productRow, qExec(updateTarget, 0)},
			wantStatus: http.StatusNotFound, wantCode: httpx.CodeNotFound},
		{name: "update/error interno", method: http.MethodPut, target: watched, body: `{"targetPrice":400}`, remote: pricing,
			rules:      []rule{productRow, qFail(updateTarget, errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// DELETE /watchlist/{productId}
		{name: "delete/feliz", method: http.MethodDelete, target: watched,
			rules:      []rule{productRow, qExec(unwatch, 1)},
			wantStatus: http.StatusNoContent},
		{name: "delete/id inválido", method: http.MethodDelete, target: "/watchlist/abc",
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
		{name: "delete/no está en la watchlist", method: http.MethodDelete, target: watched,
			rules:      []rule{productRow, qExec(unwatch, 0)},
			wantStatus: http.StatusNotFound, wantCode: httpx.CodeNotFound},
		{name: "delete/error interno", method: http.MethodDelete, target: watched,
			rules:      []rule{productRow, qFail(unwatch, errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},
	})
}

func TestNotificationAPI(t *testing.T) {
	notification := `{"userId":7,"type":"price_drop","title":"Bajó","message":"El TV bajó","data":{"offerId":5}}`
	insert := "INSERT INTO notification"
	readOne := "UPDATE notification SET read_at=NOW() WHERE id"
	readAll := "UPDATE notification SET read_at=NOW() WHERE user_id"

	runCases(t, "notification-api", []apiCase{
		// POST /notifications (interno)
		{name: "create/feliz", method: http.MethodPost, target: "/notifications", body: notification, token: serviceToken,
			rules:      []rule{qRows(insert, `{"id":1,"type":"price_drop"}`)},
			wantStatus: http.StatusCreated, wantBody: `"price_drop"`},
		{name: "create/sin token", method: http.MethodPost, target: "/notifications", body: notification, token: "-",
			wantStatus: http.StatusUnauthorized, wantCode: httpx.CodeUnauthorized},
		{name: "create/usuario sin rol admin", method: http.MethodPost, target: "/notifications", body: notification,
			wantStatus: http.StatusForbidden, wantCode: httpx.CodeForbidden},
		{name: "create/json malformado", method: http.MethodPost, target: "/notifications", body: `{`, token: serviceToken,
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeMalformedJSON},
		{name: "create/reglas", method: http.MethodPost, target: "/notifications", body: `{}`, token: serviceToken,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed,
			wantFields: []string{"userId/min", "type/required", "title/required", "message/required"}},
		{name: "create/usuario inexistente (FK)", method: http.MethodPost, target: "/notifications", body: notification, token: serviceToken,
			rules:      []rule{qFail(insert, pgError("23503", `Key (user_id)=(7) is not present in table "user_account".`))},
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed, wantFields: []string{"userId/not_found"}},
		{name: "create/error interno", method: http.MethodPost, target: "/notifications", body: notification, token: serviceToken,
			rules:      []rule{qFail(insert, errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// GET /notifications y /notifications/unread
		{name: "list/feliz", method: http.MethodGet, target: "/notifications",
			rules:      []rule{qRows("FROM notification WHERE user_id=$1 ORDER BY", `{"id":1,"read":true}`)},
			wantStatus: http.StatusOK, wantBody: `"read":true`},
		{name: "list/error interno", method: http.MethodGet, target: "/notifications",
			rules:      []rule{qFail("FROM notification", errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},
		{name: "unread/feliz", method: http.MethodGet, target: "/notifications/unread",
			rules:      []rule{qRows("AND read_at IS NULL ORDER BY", `{"id":2,"read":false}`)},
			wantStatus: http.StatusOK, wantBody: `"read":false`},
		{name: "unread/error interno", method: http.MethodGet, target: "/notifications/unread",
			rules:      []rule{qFail("FROM notification", errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// PUT /notifications/{id}/read
		{name: "read/feliz", method: http.MethodPut, target: "/notifications/5/read",
			rules:      []rule{qExec(readOne, 1)},
			wantStatus: http.StatusOK, wantBody: `"read":true`},
		{name: "read/id inválido", method: http.MethodPut, target: "/notifications/x/read",
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
		{name: "read/inexistente o ya leída", method: http.MethodPut, target: "/notifications/5/read",
			rules:      []rule{qExec(readOne, 0)},
			wantStatus: http.StatusNotFound, wantCode: httpx.CodeNotFound},
		{name: "read/error interno", method: http.MethodPut, target: "/notifications/5/read",
			rules:      []rule{qFail(readOne, errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// PUT /notifications/read-all
		{name: "read all/feliz sin body", method: http.MethodPut, target: "/notifications/read-all",
			rules:      []rule{qExec(readAll, 3)},
			wantStatus: http.StatusOK, wantBody: `"readAll":true`},
		{name: "read all/feliz con userId en el body", method: http.MethodPut, target: "/notifications/read-all", body: `{"userId":7}`,
			rules:      []rule{qExec(readAll, 3)},
			wantStatus: http.StatusOK, wantBody: `"readAll":true`},
		{name: "read all/json malformado", method: http.MethodPut, target: "/notifications/read-all", body: `{"userId"`,
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeMalformedJSON},
		{name: "read all/error interno", method: http.MethodPut, target: "/notifications/read-all",
			rules:      []rule{qFail(readAll, errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},
	})
}
