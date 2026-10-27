package features

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/DoSafills/SOLOServis/backend/internal/apiutil"
)

type searchInput struct {
	UserID  int32           `json:"userId"`
	Query   string          `json:"query"`
	Name    string          `json:"name"`
	Type    string          `json:"type"`
	Filters json.RawMessage `json:"filters"`
}

func (h *Handler) CreateSearchHistory(w http.ResponseWriter, r *http.Request) {
	var input searchInput
	if !decode(r, &input) || strings.TrimSpace(input.Query) == "" {
		badRequest(w, "query is required")
		return
	}
	input.UserID = normalizeUserID(r, input.UserID)
	if input.UserID <= 0 {
		badRequest(w, "userId is required")
		return
	}
	typeValue := input.Type
	if typeValue == "" {
		typeValue = "product"
	}
	h.created(w, r, `WITH inserted AS (INSERT INTO search_query (user_id,raw_query,search_type)
		VALUES ($1,$2,$3) RETURNING id,user_id,raw_query,search_type,created_at)
		SELECT jsonb_build_object('id',id,'userId',user_id,'query',raw_query,'type',search_type,'createdAt',created_at) FROM inserted`, input.UserID, input.Query, typeValue)
}

func (h *Handler) GetSearchHistory(w http.ResponseWriter, r *http.Request) {
	userID, ok := requestUserID(r)
	if !ok {
		badRequest(w, "userId is required")
		return
	}
	h.rows(w, r, `SELECT jsonb_build_object('id',id,'query',raw_query,'type',search_type,'createdAt',created_at)
		FROM search_query WHERE user_id=$1 ORDER BY created_at DESC LIMIT 100`, userID)
}

func (h *Handler) SaveSearch(w http.ResponseWriter, r *http.Request) {
	var input searchInput
	if !decode(r, &input) || strings.TrimSpace(input.Name) == "" {
		badRequest(w, "userId and name are required")
		return
	}
	input.UserID = normalizeUserID(r, input.UserID)
	if input.UserID <= 0 {
		badRequest(w, "userId and name are required")
		return
	}
	h.created(w, r, `WITH inserted AS (INSERT INTO saved_search (user_id,name,query,filters)
		VALUES ($1,$2,$3,$4::jsonb) RETURNING id,user_id,name,query,filters,created_at)
		SELECT jsonb_build_object('id',id,'userId',user_id,'name',name,'query',query,'filters',filters,'createdAt',created_at) FROM inserted`,
		input.UserID, input.Name, input.Query, rawJSON(input.Filters))
}

func (h *Handler) GetSavedSearches(w http.ResponseWriter, r *http.Request) {
	userID, ok := requestUserID(r)
	if !ok {
		badRequest(w, "userId is required")
		return
	}
	h.rows(w, r, `SELECT jsonb_build_object('id',id,'name',name,'query',query,'filters',filters,'createdAt',created_at)
		FROM saved_search WHERE user_id=$1 ORDER BY created_at DESC`, userID)
}

func (h *Handler) DeleteSavedSearch(w http.ResponseWriter, r *http.Request) {
	id, ok := intParam(r, "id")
	userID, userOK := requestUserID(r)
	if !ok || !userOK {
		badRequest(w, "valid id and userId are required")
		return
	}
	result, err := h.db.Exec(r.Context(), `DELETE FROM saved_search WHERE id=$1 AND user_id=$2`, id, userID)
	if err != nil || result.RowsAffected() == 0 {
		apiutil.WriteError(w, http.StatusNotFound, "saved search not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
