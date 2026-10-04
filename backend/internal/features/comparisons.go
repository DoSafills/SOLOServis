package features

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/DoSafills/SOLOServis/backend/internal/apiutil"
	"github.com/go-chi/chi/v5"
)

type comparisonInput struct {
	UserID    int32    `json:"userId"`
	Name      string   `json:"name"`
	ProductID string   `json:"productId"`
	Products  []string `json:"productIds"`
}

func (h *Handler) CreateComparison(w http.ResponseWriter, r *http.Request) {
	var input comparisonInput
	if !decode(r, &input) || input.UserID <= 0 {
		badRequest(w, "userId is required")
		return
	}
	if input.Name == "" {
		input.Name = "Comparación de productos"
	}
	if input.ProductID != "" {
		input.Products = append(input.Products, input.ProductID)
	}
	if len(input.Products) == 0 {
		badRequest(w, "at least one productId is required")
		return
	}
	userID, err := h.service.UserID(r.Context(), input.UserID)
	if err != nil {
		apiutil.WriteError(w, http.StatusNotFound, "user not found")
		return
	}
	productIDs := make([]int32, 0, len(input.Products))
	var categoryID int32
	for _, publicID := range input.Products {
		if len(publicID) == 36 {
			if err := verifyProductService(r.Context(), publicID); err != nil {
				apiutil.WriteError(w, http.StatusServiceUnavailable, "product service is unavailable")
				return
			}
		}
		id, err := h.service.ProductID(r.Context(), publicID)
		if err != nil {
			apiutil.WriteError(w, http.StatusNotFound, "product not found")
			return
		}
		var category int32
		if err := h.db.QueryRow(r.Context(), `SELECT category_id FROM product WHERE id=$1`, id).Scan(&category); err != nil {
			apiutil.WriteError(w, http.StatusNotFound, "product not found")
			return
		}
		if categoryID != 0 && categoryID != category {
			badRequest(w, "all products must belong to the same category")
			return
		}
		categoryID = category
		productIDs = append(productIDs, id)
	}
	tx, err := h.db.Begin(r.Context())
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "could not start comparison")
		return
	}
	defer tx.Rollback(r.Context())
	var comparisonID int32
	err = tx.QueryRow(r.Context(), `INSERT INTO product_comparison (user_id,category_id,name)
		VALUES ($1,$2,$3) RETURNING id`, userID, categoryID, input.Name).Scan(&comparisonID)
	for position, productID := range productIDs {
		if err == nil {
			_, err = tx.Exec(r.Context(), `INSERT INTO product_comparison_item (comparison_id,product_id,position) VALUES ($1,$2,$3)`, comparisonID, productID, position)
		}
	}
	if err != nil || tx.Commit(r.Context()) != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "could not create comparison")
		return
	}
	apiutil.WriteJSON(w, http.StatusCreated, map[string]any{"id": comparisonID, "userId": userID, "name": input.Name, "categoryId": categoryID, "products": input.Products})
}

func (h *Handler) GetComparison(w http.ResponseWriter, r *http.Request) {
	id, ok := intParam(r, "id")
	if !ok {
		badRequest(w, "invalid comparison id")
		return
	}
	result, err := h.repo.QueryJSON(r.Context(), `SELECT jsonb_build_object('id',c.id,'userId',c.user_id,'name',c.name,
		'categoryId',c.category_id,'createdAt',c.created_at,'updatedAt',c.updated_at,
		'products',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',p.public_id,'name',p.name,
			'brand',COALESCE(b.name,''),'specifications',COALESCE((SELECT jsonb_object_agg(cs.name,sv.value)
				FROM product_specification_value sv JOIN product_category_specification cs ON cs.id=sv.specification_id
				WHERE sv.product_id=p.id AND cs.comparable), '{}'::jsonb),
			'offers',COALESCE((SELECT jsonb_agg(jsonb_build_object('offerId',o.id,'storeId',o.store_id,'price',o.price,'shippingCost',o.shipping_cost,'productUrl',o.product_url))
				FROM product_offer o WHERE o.product_id=p.id AND o.available), '[]'::jsonb)))
			FROM product_comparison_item ci JOIN product p ON p.id=ci.product_id
			LEFT JOIN brand b ON b.id=p.brand_id WHERE ci.comparison_id=c.id), '[]'::jsonb))
		FROM product_comparison c WHERE c.id=$1`, id)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "could not load comparison")
		return
	}
	if len(result) == 0 {
		apiutil.WriteError(w, http.StatusNotFound, "comparison not found")
		return
	}
	var comparison map[string]any
	if err := json.Unmarshal(result[0], &comparison); err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "invalid comparison data")
		return
	}
	products, _ := comparison["products"].([]any)
	for _, productValue := range products {
		product, ok := productValue.(map[string]any)
		if !ok {
			continue
		}
		productID, _ := product["id"].(string)
		var offers []json.RawMessage
		if err := remoteJSON(r.Context(), "PRICING_API_URL", "http://localhost:8085", http.MethodGet,
			"/offers/product/"+productID, nil, &offers); err != nil {
			apiutil.WriteError(w, http.StatusServiceUnavailable, "pricing service is unavailable")
			return
		}
		product["offers"] = offers
	}
	apiutil.WriteJSON(w, http.StatusOK, comparison)
}

func (h *Handler) UpdateComparison(w http.ResponseWriter, r *http.Request) {
	id, ok := intParam(r, "id")
	var input comparisonInput
	if !ok || !decode(r, &input) || strings.TrimSpace(input.Name) == "" {
		badRequest(w, "valid id and name are required")
		return
	}
	result, err := h.db.Exec(r.Context(), `UPDATE product_comparison SET name=$2,updated_at=NOW() WHERE id=$1`, id, input.Name)
	if err != nil || result.RowsAffected() == 0 {
		apiutil.WriteError(w, http.StatusNotFound, "comparison not found")
		return
	}
	apiutil.WriteJSON(w, http.StatusOK, map[string]any{"id": id, "name": input.Name})
}

func (h *Handler) DeleteComparison(w http.ResponseWriter, r *http.Request) {
	id, ok := intParam(r, "id")
	if !ok {
		badRequest(w, "invalid comparison id")
		return
	}
	result, err := h.db.Exec(r.Context(), `DELETE FROM product_comparison WHERE id=$1`, id)
	if err != nil || result.RowsAffected() == 0 {
		apiutil.WriteError(w, http.StatusNotFound, "comparison not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) AddComparisonItem(w http.ResponseWriter, r *http.Request) {
	comparisonID, ok := intParam(r, "id")
	var input comparisonInput
	if !ok || !decode(r, &input) || input.ProductID == "" {
		badRequest(w, "valid comparison id and productId are required")
		return
	}
	productID, err := h.service.ProductID(r.Context(), input.ProductID)
	if err != nil {
		apiutil.WriteError(w, http.StatusNotFound, "product not found")
		return
	}
	if len(input.ProductID) == 36 {
		if err := verifyProductService(r.Context(), input.ProductID); err != nil {
			apiutil.WriteError(w, http.StatusServiceUnavailable, "product service is unavailable")
			return
		}
	}
	var comparisonCategory, productCategory int32
	err = h.db.QueryRow(r.Context(), `SELECT c.category_id,p.category_id FROM product_comparison c CROSS JOIN product p WHERE c.id=$1 AND p.id=$2`, comparisonID, productID).Scan(&comparisonCategory, &productCategory)
	if err != nil {
		apiutil.WriteError(w, http.StatusNotFound, "comparison or product not found")
		return
	}
	if comparisonCategory != productCategory {
		badRequest(w, "product category is incompatible with this comparison")
		return
	}
	_, err = h.db.Exec(r.Context(), `INSERT INTO product_comparison_item (comparison_id,product_id) VALUES ($1,$2)`, comparisonID, productID)
	if err != nil {
		apiutil.WriteError(w, http.StatusConflict, "product is already compared or cannot be added")
		return
	}
	apiutil.WriteJSON(w, http.StatusCreated, map[string]any{"comparisonId": comparisonID, "productId": input.ProductID})
}

func (h *Handler) DeleteComparisonItem(w http.ResponseWriter, r *http.Request) {
	comparisonID, ok := intParam(r, "id")
	productID, err := h.service.ProductID(r.Context(), chi.URLParam(r, "productId"))
	if !ok || err != nil {
		badRequest(w, "invalid comparison or product id")
		return
	}
	result, err := h.db.Exec(r.Context(), `DELETE FROM product_comparison_item WHERE comparison_id=$1 AND product_id=$2`, comparisonID, productID)
	if err != nil || result.RowsAffected() == 0 {
		apiutil.WriteError(w, http.StatusNotFound, "comparison item not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CompatibleProducts(w http.ResponseWriter, r *http.Request) {
	productID, err := h.service.ProductID(r.Context(), chi.URLParam(r, "productId"))
	if err != nil {
		apiutil.WriteError(w, http.StatusNotFound, "product not found")
		return
	}
	if len(chi.URLParam(r, "productId")) == 36 {
		if err := verifyProductService(r.Context(), chi.URLParam(r, "productId")); err != nil {
			apiutil.WriteError(w, http.StatusServiceUnavailable, "product service is unavailable")
			return
		}
	}
	h.rows(w, r, `SELECT jsonb_build_object('id',p.public_id,'name',p.name,'brand',COALESCE(b.name,''),'category',c.name)
		FROM product selected JOIN product p ON p.category_id=selected.category_id AND p.id<>selected.id
		JOIN product_category c ON c.id=p.category_id LEFT JOIN brand b ON b.id=p.brand_id
		WHERE selected.id=$1 AND p.active ORDER BY p.name LIMIT 100`, productID)
}
