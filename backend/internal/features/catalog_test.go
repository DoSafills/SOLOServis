package features

import (
	"net/http"
	"testing"

	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
)

// Reglas de los topes por usuario (TAL-201): el recorte del historial y la
// cuenta de búsquedas guardadas que corren antes de cada INSERT.
var (
	trimHistory = qExec("DELETE FROM search_query WHERE user_id", 0)
	savedCount  = qRow("SELECT COUNT(*)::int FROM saved_search", int32(3))
)

// TestSearchSessionAPI cubre historial y búsquedas guardadas (requieren
// sesión): casos feliz, 400, 422, 404 y 500 de cada endpoint, más los topes
// por usuario de TAL-201.
func TestSearchSessionAPI(t *testing.T) {
	insertHistory := "INSERT INTO search_query"
	insertSaved := "INSERT INTO saved_search"
	deleteSaved := "DELETE FROM saved_search"
	saved := `{"name":"TVs","query":"tv","filters":{"brand":"LG"}}`

	runCases(t, "search-api", []apiCase{
		// POST /search/history
		{name: "create history/feliz", method: http.MethodPost, target: "/search/history", body: `{"query":"tv"}`,
			rules:      []rule{trimHistory, qRows(insertHistory, `{"id":1,"query":"tv","type":"product"}`)},
			wantStatus: http.StatusCreated, wantBody: `"query":"tv"`},
		{name: "create history/recorte falla", method: http.MethodPost, target: "/search/history", body: `{"query":"tv"}`,
			rules:      []rule{qFail("DELETE FROM search_query WHERE user_id", errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},
		{name: "create history/sin sesión", method: http.MethodPost, target: "/search/history", body: `{"query":"tv"}`, token: "-",
			wantStatus: http.StatusUnauthorized, wantCode: httpx.CodeUnauthorized},
		{name: "create history/json malformado", method: http.MethodPost, target: "/search/history", body: `{"query":}`,
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeMalformedJSON},
		{name: "create history/reglas", method: http.MethodPost, target: "/search/history", body: `{"query":"  "}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed, wantFields: []string{"query/required"}},
		{name: "create history/error interno", method: http.MethodPost, target: "/search/history", body: `{"query":"tv"}`,
			rules:      []rule{trimHistory, qFail(insertHistory, errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},
		{name: "create history/insert sin filas", method: http.MethodPost, target: "/search/history", body: `{"query":"tv"}`,
			rules:      []rule{trimHistory, qRows(insertHistory)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// GET /search/history
		{name: "get history/feliz", method: http.MethodGet, target: "/search/history",
			rules:      []rule{qRows("FROM search_query", `{"id":1,"query":"tv"}`)},
			wantStatus: http.StatusOK, wantBody: `"query":"tv"`},
		{name: "get history/error interno", method: http.MethodGet, target: "/search/history",
			rules:      []rule{qFail("FROM search_query", errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// POST /search/saved
		{name: "save/feliz", method: http.MethodPost, target: "/search/saved", body: saved,
			rules:      []rule{savedCount, qRows(insertSaved, `{"id":2,"name":"TVs"}`)},
			wantStatus: http.StatusCreated, wantBody: `"name":"TVs"`},
		{name: "save/tope alcanzado", method: http.MethodPost, target: "/search/saved", body: saved,
			rules:      []rule{qRow("SELECT COUNT(*)::int FROM saved_search", int32(maxSavedSearches))},
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed, wantFields: []string{"name/max"}},
		{name: "save/cuenta falla", method: http.MethodPost, target: "/search/saved", body: saved,
			rules:      []rule{qFail("SELECT COUNT(*)::int FROM saved_search", errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},
		{name: "save/campo desconocido", method: http.MethodPost, target: "/search/saved", body: `{"nombre":"TVs"}`,
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeUnknownField, wantFields: []string{"nombre/unknown_field"}},
		{name: "save/reglas", method: http.MethodPost, target: "/search/saved", body: `{"query":"tv"}`,
			wantStatus: http.StatusUnprocessableEntity, wantCode: httpx.CodeValidationFailed, wantFields: []string{"name/required"}},
		{name: "save/error interno", method: http.MethodPost, target: "/search/saved", body: saved,
			rules:      []rule{savedCount, qFail(insertSaved, errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// GET /search/saved
		{name: "get saved/feliz", method: http.MethodGet, target: "/search/saved",
			rules:      []rule{qRows("FROM saved_search", `{"id":2,"name":"TVs"}`)},
			wantStatus: http.StatusOK, wantBody: `"name":"TVs"`},
		{name: "get saved/error interno", method: http.MethodGet, target: "/search/saved",
			rules:      []rule{qFail("FROM saved_search", errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},

		// DELETE /search/saved/{id}
		{name: "delete saved/feliz", method: http.MethodDelete, target: "/search/saved/2",
			rules:      []rule{qExec(deleteSaved, 1)},
			wantStatus: http.StatusNoContent},
		{name: "delete saved/id inválido", method: http.MethodDelete, target: "/search/saved/dos",
			wantStatus: http.StatusBadRequest, wantCode: httpx.CodeInvalidParameter},
		{name: "delete saved/inexistente", method: http.MethodDelete, target: "/search/saved/2",
			rules:      []rule{qExec(deleteSaved, 0)},
			wantStatus: http.StatusNotFound, wantCode: httpx.CodeNotFound},
		{name: "delete saved/error interno", method: http.MethodDelete, target: "/search/saved/2",
			rules:      []rule{qFail(deleteSaved, errBoom)},
			wantStatus: http.StatusInternalServerError, wantCode: httpx.CodeInternalError},
	})
}
