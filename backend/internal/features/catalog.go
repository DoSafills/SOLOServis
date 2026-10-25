package features

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/DoSafills/SOLOServis/backend/internal/apiutil"
)

type offerInput struct {
	ProductID    string   `json:"productId"`
	StoreID      int32    `json:"storeId"`
	Price        float64  `json:"price"`
	ListPrice    *float64 `json:"listPrice"`
	Currency     string   `json:"currency"`
	ShippingCost float64  `json:"shippingCost"`
	ShippingFree bool     `json:"shippingFree"`
	Available    bool     `json:"available"`
	Stock        *int32   `json:"stock"`
	Condition    string   `json:"condition"`
	ProductURL   string   `json:"productUrl"`
}

func (h *Handler) SearchProducts(w http.ResponseWriter, r *http.Request) {
	var catalog []json.RawMessage
	if err := remoteJSON(r.Context(), "PRODUCT_API_URL", "http://localhost:8081", http.MethodGet, "/products", nil, &catalog); err != nil {
		apiutil.WriteError(w, http.StatusServiceUnavailable, "product catalog service is unavailable")
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	category := r.URL.Query().Get("categoryId")
	brand := r.URL.Query().Get("brandId")
	minPrice, _ := strconv.ParseFloat(r.URL.Query().Get("minPrice"), 64)
	maxPrice, _ := strconv.ParseFloat(r.URL.Query().Get("maxPrice"), 64)
	sort := strings.ToLower(r.URL.Query().Get("sort"))
	order := "p.name ASC"
	if sort == "price_asc" {
		order = "best.price ASC NULLS LAST"
	} else if sort == "price_desc" {
		order = "best.price DESC NULLS LAST"
	} else if sort == "newest" {
		order = "p.created_at DESC"
	}
	sql := `SELECT jsonb_build_object('id', p.public_id, 'name', p.name, 'brand', COALESCE(b.name,''),
		'category', c.name, 'description', COALESCE(p.description,''), 'price', best.price,
		'imageUrl', (SELECT image_url FROM product_image WHERE product_id=p.id AND active ORDER BY sort_order LIMIT 1))
		FROM product p JOIN product_category c ON c.id=p.category_id
		LEFT JOIN brand b ON b.id=p.brand_id
		LEFT JOIN LATERAL (SELECT MIN(price + COALESCE(shipping_cost,0)) AS price FROM product_offer WHERE product_id=p.id AND available) best ON TRUE
		WHERE p.active AND ($1='' OR p.name ILIKE '%' || $1 || '%' OR COALESCE(b.name,'') ILIKE '%' || $1 || '%')
		AND ($2='' OR c.id::text=$2 OR c.name ILIKE '%' || $2 || '%')
		AND ($3='' OR b.id::text=$3 OR b.name ILIKE '%' || $3 || '%')
		AND ($4=0 OR best.price >= $4) AND ($5=0 OR best.price <= $5) ORDER BY ` + order + ` LIMIT 100`
	h.rows(w, r, sql, query, category, brand, minPrice, maxPrice)
}

func (h *Handler) SearchCategories(w http.ResponseWriter, r *http.Request) {
	h.rows(w, r, `SELECT jsonb_build_object('id',id,'name',name,'description',description)
		FROM product_category WHERE active AND ($1='' OR name ILIKE '%' || $1 || '%') ORDER BY name`, r.URL.Query().Get("q"))
}

func (h *Handler) SearchBrands(w http.ResponseWriter, r *http.Request) {
	h.rows(w, r, `SELECT jsonb_build_object('id',id,'name',name,'logoUrl',logo_url)
		FROM brand WHERE active AND ($1='' OR name ILIKE '%' || $1 || '%') ORDER BY name`, r.URL.Query().Get("q"))
}

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
