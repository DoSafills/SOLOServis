package features

import (
	"errors"
	"net/http"
	"testing"

	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
)

func TestServiceComparisonAPI(t *testing.T) {
	create := `{"userId":7,"name":"Internet","serviceIds":["` + serviceUUID + `"]}`
	category := qRow("SELECT category_id FROM service", int32(2))
	insertComparison := "INSERT INTO service_comparison (user_id"
	insertItem := "INSERT INTO service_comparison_item"
	countItems := "SELECT COUNT(ci.id)"
	comparison := "/comparisons/services/9"
	loadComparison := "FROM service_comparison c"
	deleteComparison := "DELETE FROM service_comparison WHERE"
	deleteItem := "DELETE FROM service_comparison_item"
	duplicate := pgError("23505", "")

	runCases(t, "comparison-api", []apiCase{
		// POST /comparisons/services
		{name: "create/feliz", method: http.MethodPost, target: "/comparisons/services", body: create,
			rules:      []rule{sessionUser, serviceRow, category, qRow(insertComparison, int32(9)), qExec(insertItem, 1)},
			wantStatus: http.StatusCreated, wantBody: `"id":9`},
		{name: "create/sin sesión", method: http.MethodPost, target: "/comparisons/services", body: create, token: "-",
			wantStatus: http.StatusUnauthorized, wantCode: httpx.CodeUnauthorized},
		{name: "create/json malformado", method: http.MethodPost, target: "/comparisons/services", body: `{"serviceIds":[`,
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeMalformedJSON},
		{name: "create/reglas", method: http.MethodPost, target: "/comparisons/services", body: `{}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed, wantFields: []string{"serviceIds/required"}},
		{name: "create/servicio inexistente", method: http.MethodPost, target: "/comparisons/services", body: create,
			rules:      []rule{sessionUser, qNoRows("SELECT id FROM service WHERE public_id")},
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed, wantFields: []string{"serviceIds.0/not_found"}},
		{name: "create/servicio repetido", method: http.MethodPost, target: "/comparisons/services", body: create,
			rules:      []rule{sessionUser, serviceRow, category, qRow(insertComparison, int32(9)), qFail(insertItem, duplicate)},
			wantStatus: http.StatusConflict, wantCode: httpx.CodeConflict},
		{name: "create/error interno", method: http.MethodPost, target: "/comparisons/services", body: create,
			rules:      []rule{sessionUser, serviceRow, category, qFail(insertComparison, errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},
		{name: "create/falla el commit", method: http.MethodPost, target: "/comparisons/services", body: create,
			rules:      []rule{sessionUser, serviceRow, category, qRow(insertComparison, int32(9)), qExec(insertItem, 1)},
			commitErr:  errors.New("commit failed"),
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// GET /comparisons/services/{id}
		{name: "get/feliz", method: http.MethodGet, target: comparison,
			rules:      []rule{qRows(loadComparison, `{"id":9,"services":[]}`)},
			wantStatus: http.StatusOK, wantBody: `"id":9`},
		{name: "get/id inválido", method: http.MethodGet, target: "/comparisons/services/x",
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
		{name: "get/ajena o inexistente", method: http.MethodGet, target: comparison,
			rules:      []rule{qRows(loadComparison)},
			wantStatus: http.StatusNotFound, wantCode: httpx.CodeNotFound},
		{name: "get/error interno", method: http.MethodGet, target: comparison,
			rules:      []rule{qFail(loadComparison, errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// DELETE /comparisons/services/{id}
		{name: "delete/feliz", method: http.MethodDelete, target: comparison,
			rules:      []rule{qExec(deleteComparison, 1)},
			wantStatus: http.StatusNoContent},
		{name: "delete/id inválido", method: http.MethodDelete, target: "/comparisons/services/0",
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
		{name: "delete/inexistente", method: http.MethodDelete, target: comparison,
			rules:      []rule{qExec(deleteComparison, 0)},
			wantStatus: http.StatusNotFound, wantCode: httpx.CodeNotFound},
		{name: "delete/error interno", method: http.MethodDelete, target: comparison,
			rules:      []rule{qFail(deleteComparison, errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// POST /comparisons/services/{id}/items
		{name: "add item/feliz", method: http.MethodPost, target: comparison + "/items", body: `{"serviceId":"` + serviceUUID + `"}`,
			rules:      []rule{serviceRow, qRow(countItems, int32(1)), qExec(insertItem, 1)},
			wantStatus: http.StatusCreated, wantBody: `"comparisonId":9`},
		{name: "add item/id inválido", method: http.MethodPost, target: "/comparisons/services/x/items", body: `{}`,
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
		{name: "add item/reglas", method: http.MethodPost, target: comparison + "/items", body: `{}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed, wantFields: []string{"serviceId/required"}},
		{name: "add item/servicio inexistente", method: http.MethodPost, target: comparison + "/items", body: `{"serviceId":"` + serviceUUID + `"}`,
			rules:      []rule{qNoRows("SELECT id FROM service WHERE public_id")},
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed, wantFields: []string{"serviceId/not_found"}},
		{name: "add item/comparación ajena o inexistente", method: http.MethodPost, target: comparison + "/items", body: `{"serviceId":"` + serviceUUID + `"}`,
			rules:      []rule{serviceRow, qNoRows(countItems)},
			wantStatus: http.StatusNotFound, wantCode: httpx.CodeNotFound},
		{name: "add item/comparación llena", method: http.MethodPost, target: comparison + "/items", body: `{"serviceId":"` + serviceUUID + `"}`,
			rules:      []rule{serviceRow, qRow(countItems, int32(maxComparedServices))},
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed, wantFields: []string{"serviceId/max"}},
		{name: "add item/servicio repetido", method: http.MethodPost, target: comparison + "/items", body: `{"serviceId":"` + serviceUUID + `"}`,
			rules:      []rule{serviceRow, qRow(countItems, int32(1)), qFail(insertItem, duplicate)},
			wantStatus: http.StatusConflict, wantCode: httpx.CodeConflict},
		{name: "add item/error interno", method: http.MethodPost, target: comparison + "/items", body: `{"serviceId":"` + serviceUUID + `"}`,
			rules:      []rule{serviceRow, qFail(countItems, errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// DELETE /comparisons/services/{id}/items/{serviceId}
		{name: "delete item/feliz", method: http.MethodDelete, target: comparison + "/items/" + serviceUUID,
			rules:      []rule{serviceRow, qExec(deleteItem, 1)},
			wantStatus: http.StatusNoContent},
		{name: "delete item/id inválido", method: http.MethodDelete, target: "/comparisons/services/x/items/" + serviceUUID,
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
		{name: "delete item/servicio inexistente", method: http.MethodDelete, target: comparison + "/items/" + serviceUUID,
			rules:      []rule{qNoRows("SELECT id FROM service WHERE public_id")},
			wantStatus: http.StatusNotFound, wantCode: httpx.CodeNotFound},
		{name: "delete item/no está en la comparación", method: http.MethodDelete, target: comparison + "/items/" + serviceUUID,
			rules:      []rule{serviceRow, qExec(deleteItem, 0)},
			wantStatus: http.StatusNotFound, wantCode: httpx.CodeNotFound},
		{name: "delete item/error interno", method: http.MethodDelete, target: comparison + "/items/" + serviceUUID,
			rules:      []rule{serviceRow, qFail(deleteItem, errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},
	})
}
