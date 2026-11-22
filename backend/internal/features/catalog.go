package features

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
)

type searchInput struct {
	UserID  int32           `json:"userId"`
	Query   string          `json:"query"`
	Name    string          `json:"name"`
	Type    string          `json:"type"`
	Filters json.RawMessage `json:"filters"`
}

// CreateSearchHistory atiende POST /search/history: registra una búsqueda
// del usuario de la sesión. Responde 422 si query viene vacío o type no es
// product, service o mixed (por defecto product), y 201 con la búsqueda.
// Antes de insertar recorta el historial del usuario a las maxSearchHistory-1
// más recientes, así nunca pasa de maxSearchHistory filas (TAL-201). El
// recorte borra en cascada los filtros y procesos de IA de esas búsquedas.
func (h *Handler) CreateSearchHistory(w http.ResponseWriter, r *http.Request) {
	var input searchInput
	if !decode(w, r, &input) {
		return
	}
	typeValue := strings.TrimSpace(input.Type)
	if typeValue == "" {
		typeValue = defaultSearchType
	}
	var v httpx.Validator
	v.Required("query", input.Query)
	v.Check(searchTypes[typeValue], "type", "one_of", "type debe ser product, service o mixed")
	if !valid(w, r, &v) {
		return
	}
	userID := sessionUserID(r)
	if _, err := h.db.Exec(r.Context(), `DELETE FROM search_query WHERE user_id=$1 AND id NOT IN (
		SELECT id FROM search_query WHERE user_id=$1 ORDER BY created_at DESC, id DESC LIMIT $2)`,
		userID, maxSearchHistory-1); err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	h.created(w, r, `WITH inserted AS (INSERT INTO search_query (user_id,raw_query,search_type)
		VALUES ($1,$2,$3) RETURNING id,user_id,raw_query,search_type,created_at)
		SELECT jsonb_build_object('id',id,'userId',user_id,'query',raw_query,'type',search_type,'createdAt',created_at) FROM inserted`, userID, input.Query, typeValue)
}

// GetSearchHistory atiende GET /search/history: responde 200 con las 100
// búsquedas más recientes del usuario de la sesión.
func (h *Handler) GetSearchHistory(w http.ResponseWriter, r *http.Request) {
	userID := sessionUserID(r)
	h.rows(w, r, `SELECT jsonb_build_object('id',id,'query',raw_query,'type',search_type,'createdAt',created_at)
		FROM search_query WHERE user_id=$1 ORDER BY created_at DESC LIMIT 100`, userID)
}

// SaveSearch atiende POST /search/saved: guarda una búsqueda con nombre para
// el usuario de la sesión. Responde 422 si name falta o supera
// maxSavedSearchName caracteres, si filters no es un objeto JSON o si el
// usuario ya tiene maxSavedSearches búsquedas guardadas (TAL-201), y 201 con
// la búsqueda guardada.
func (h *Handler) SaveSearch(w http.ResponseWriter, r *http.Request) {
	var input searchInput
	if !decode(w, r, &input) {
		return
	}
	var v httpx.Validator
	v.Required("name", input.Name)
	v.MaxLength("name", input.Name, maxSavedSearchName)
	v.Check(isJSONObject(input.Filters), "filters", "invalid_type", "filters debe ser un objeto JSON")
	if !valid(w, r, &v) {
		return
	}
	var count int32
	if err := h.db.QueryRow(r.Context(), `SELECT COUNT(*)::int FROM saved_search WHERE user_id=$1`,
		sessionUserID(r)).Scan(&count); err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	if count >= maxSavedSearches {
		httpx.WriteValidation(w, r, []httpx.FieldError{{Field: "name", Code: "max",
			Message: fmt.Sprintf("No puedes tener más de %d búsquedas guardadas; borra alguna antes de guardar otra", maxSavedSearches)}})
		return
	}
	h.created(w, r, `WITH inserted AS (INSERT INTO saved_search (user_id,name,query,filters)
		VALUES ($1,$2,$3,$4::jsonb) RETURNING id,user_id,name,query,filters,created_at)
		SELECT jsonb_build_object('id',id,'userId',user_id,'name',name,'query',query,'filters',filters,'createdAt',created_at) FROM inserted`,
		sessionUserID(r), input.Name, input.Query, rawJSON(input.Filters))
}

// GetSavedSearches atiende GET /search/saved: responde 200 con las búsquedas
// guardadas del usuario de la sesión, de la más nueva a la más antigua.
func (h *Handler) GetSavedSearches(w http.ResponseWriter, r *http.Request) {
	userID := sessionUserID(r)
	h.rows(w, r, `SELECT jsonb_build_object('id',id,'name',name,'query',query,'filters',filters,'createdAt',created_at)
		FROM saved_search WHERE user_id=$1 ORDER BY created_at DESC`, userID)
}

// DeleteSavedSearch atiende DELETE /search/saved/{id}: borra la búsqueda
// guardada si es del usuario de la sesión. Responde 204, 400 si el id no es
// válido y 404 si no existe o es de otro usuario.
func (h *Handler) DeleteSavedSearch(w http.ResponseWriter, r *http.Request) {
	id, ok := intParam(r, "id")
	if !ok {
		badRequest(w, r, "El id de la búsqueda guardada no es válido.")
		return
	}
	result, err := h.db.Exec(r.Context(), `DELETE FROM saved_search WHERE id=$1 AND user_id=$2`, id, sessionUserID(r))
	if err != nil || result.RowsAffected() == 0 {
		h.dbError(w, r, err, http.StatusNotFound, "Búsqueda guardada no encontrada.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
