package features

import (
	"net/http"
	"strconv"

	"github.com/DoSafills/SOLOServis/backend/internal/apiutil"
	"github.com/go-chi/chi/v5"
)

type favoriteInput struct {
	UserID int32 `json:"userId"`
}

func (h *Handler) ListFavorites(w http.ResponseWriter, r *http.Request) {
	userID, ok := requestUserID(r)
	if !ok {
		badRequest(w, "userId is required")
		return
	}
	h.rows(w, r, `SELECT jsonb_build_object('productId',p.public_id,'name',p.name,'addedAt',i.updated_at)
		FROM product_user_interest i JOIN product p ON p.id=i.product_id
		WHERE i.user_id=$1 AND i.is_favorite ORDER BY i.updated_at DESC`, userID)
}

func (h *Handler) AddFavorite(w http.ResponseWriter, r *http.Request) {
	var input favoriteInput
	if !decode(r, &input) {
		badRequest(w, "invalid request body")
		return
	}
	input.UserID = normalizeUserID(r, input.UserID)
	userID, err := h.service.UserID(r.Context(), input.UserID)
	productID, productErr := h.service.ProductID(r.Context(), chi.URLParam(r, "productId"))
	if err != nil || productErr != nil {
		apiutil.WriteError(w, http.StatusNotFound, "user or product not found")
		return
	}
	productKey := chi.URLParam(r, "productId")
	if len(productKey) == 36 {
		if err := verifyProductService(r.Context(), productKey); err != nil {
			apiutil.WriteError(w, http.StatusServiceUnavailable, "product service is unavailable")
			return
		}
	}
	_, err = h.db.Exec(r.Context(), `INSERT INTO product_user_interest (user_id,product_id,is_favorite)
		VALUES ($1,$2,TRUE) ON CONFLICT (user_id,product_id) DO UPDATE SET is_favorite=TRUE,updated_at=NOW()`, userID, productID)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "could not save favorite")
		return
	}
	apiutil.WriteJSON(w, http.StatusCreated, map[string]any{"productId": chi.URLParam(r, "productId"), "favorite": true})
}

func (h *Handler) GetFavorite(w http.ResponseWriter, r *http.Request) {
	userID, ok := requestUserID(r)
	productID, err := h.service.ProductID(r.Context(), chi.URLParam(r, "productId"))
	if !ok || err != nil {
		badRequest(w, "valid userId and productId are required")
		return
	}
	h.one(w, r, `SELECT jsonb_build_object('productId',p.public_id,'favorite',i.is_favorite)
		FROM product p LEFT JOIN product_user_interest i ON i.product_id=p.id AND i.user_id=$2
		WHERE p.id=$1 AND COALESCE(i.is_favorite,FALSE)`, productID, userID)
}

func (h *Handler) DeleteFavorite(w http.ResponseWriter, r *http.Request) {
	userID, ok := requestUserID(r)
	productID, err := h.service.ProductID(r.Context(), chi.URLParam(r, "productId"))
	if !ok || err != nil {
		badRequest(w, "valid userId and productId are required")
		return
	}
	result, err := h.db.Exec(r.Context(), `UPDATE product_user_interest SET is_favorite=FALSE,updated_at=NOW() WHERE user_id=$1 AND product_id=$2 AND is_favorite`, userID, productID)
	if err != nil || result.RowsAffected() == 0 {
		apiutil.WriteError(w, http.StatusNotFound, "favorite not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type cartInput struct {
	UserID    int32  `json:"userId"`
	ProductID string `json:"productId"`
	OfferID   int32  `json:"offerId"`
	Quantity  int32  `json:"quantity"`
}

func (h *Handler) GetCart(w http.ResponseWriter, r *http.Request) {
	userID, ok := requestUserID(r)
	if !ok {
		badRequest(w, "userId is required")
		return
	}
	h.rows(w, r, `SELECT jsonb_build_object('id',ci.id,'userId',ci.user_id,'productId',p.public_id,
		'productName',p.name,'offerId',o.id,'storeId',o.store_id,'storeName',s.name,
		'quantity',ci.quantity,'savedPrice',ci.saved_price,'savedShippingCost',ci.saved_shipping_cost,
		'currentPrice',o.price,'productUrl',o.product_url,'updatedAt',ci.updated_at)
		FROM cart_item ci JOIN product p ON p.id=ci.product_id JOIN product_offer o ON o.id=ci.offer_id
		JOIN store s ON s.id=o.store_id WHERE ci.user_id=$1 ORDER BY ci.created_at DESC`, userID)
}

func (h *Handler) AddCartItem(w http.ResponseWriter, r *http.Request) {
	var input cartInput
	if !decode(r, &input) || input.OfferID <= 0 || input.Quantity <= 0 {
		badRequest(w, "userId, offerId and a positive quantity are required")
		return
	}
	input.UserID = normalizeUserID(r, input.UserID)
	userID, err := h.service.UserID(r.Context(), input.UserID)
	if err != nil {
		apiutil.WriteError(w, http.StatusNotFound, "user not found")
		return
	}
	tx, err := h.db.Begin(r.Context())
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "could not start cart update")
		return
	}
	defer tx.Rollback(r.Context())
	var productID int32
	var productPublicID string
	var price, shipping float64
	var available bool
	err = tx.QueryRow(r.Context(), `SELECT p.id,p.public_id::text,o.price,o.shipping_cost,o.available
		FROM product_offer o JOIN product p ON p.id=o.product_id WHERE o.id=$1`, input.OfferID).
		Scan(&productID, &productPublicID, &price, &shipping, &available)
	if err != nil || !available {
		apiutil.WriteError(w, http.StatusNotFound, "available offer not found")
		return
	}
	if err := verifyProductService(r.Context(), productPublicID); err != nil {
		apiutil.WriteError(w, http.StatusServiceUnavailable, "product service is unavailable")
		return
	}
	var priceOffer struct {
		ProductID string `json:"productId"`
	}
	if err := remoteJSON(r.Context(), "PRICING_API_URL", "http://localhost:8085", http.MethodGet, "/offers/"+strconv.Itoa(int(input.OfferID)), nil, &priceOffer); err != nil {
		apiutil.WriteError(w, http.StatusServiceUnavailable, "pricing service is unavailable")
		return
	}
	if priceOffer.ProductID != productPublicID {
		apiutil.WriteError(w, http.StatusConflict, "pricing service returned a different product for this offer")
		return
	}
	if input.ProductID != "" {
		requestedID, resolveErr := h.service.ProductID(r.Context(), input.ProductID)
		if resolveErr != nil || requestedID != productID {
			badRequest(w, "offer does not belong to productId")
			return
		}
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO cart_item (user_id,product_id,offer_id,quantity,saved_price,saved_shipping_cost)
		VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (user_id,offer_id) DO UPDATE SET
		quantity=cart_item.quantity+EXCLUDED.quantity,saved_price=EXCLUDED.saved_price,
		saved_shipping_cost=EXCLUDED.saved_shipping_cost,updated_at=NOW()`, userID, productID, input.OfferID, input.Quantity, price, shipping)
	if err != nil || tx.Commit(r.Context()) != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "could not add offer to cart")
		return
	}
	apiutil.WriteJSON(w, http.StatusCreated, map[string]any{"userId": userID, "productId": productPublicID, "offerId": input.OfferID, "quantity": input.Quantity, "savedPrice": price, "savedShippingCost": shipping})
}

func (h *Handler) UpdateCartItem(w http.ResponseWriter, r *http.Request) {
	id, ok := intParam(r, "id")
	var input cartInput
	if !ok || !decode(r, &input) || input.Quantity <= 0 {
		badRequest(w, "valid userId and positive quantity are required")
		return
	}
	input.UserID = normalizeUserID(r, input.UserID)
	if input.UserID <= 0 {
		badRequest(w, "valid userId and positive quantity are required")
		return
	}
	result, err := h.db.Exec(r.Context(), `UPDATE cart_item SET quantity=$3,updated_at=NOW() WHERE id=$1 AND user_id=$2`, id, input.UserID, input.Quantity)
	if err != nil || result.RowsAffected() == 0 {
		apiutil.WriteError(w, http.StatusNotFound, "cart item not found")
		return
	}
	apiutil.WriteJSON(w, http.StatusOK, map[string]any{"id": id, "quantity": input.Quantity})
}

func (h *Handler) DeleteCartItem(w http.ResponseWriter, r *http.Request) {
	id, ok := intParam(r, "id")
	userID, userOK := requestUserID(r)
	if !ok || !userOK {
		badRequest(w, "valid id and userId are required")
		return
	}
	result, err := h.db.Exec(r.Context(), `DELETE FROM cart_item WHERE id=$1 AND user_id=$2`, id, userID)
	if err != nil || result.RowsAffected() == 0 {
		apiutil.WriteError(w, http.StatusNotFound, "cart item not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ClearCart(w http.ResponseWriter, r *http.Request) {
	userID, ok := requestUserID(r)
	if !ok {
		badRequest(w, "userId is required")
		return
	}
	_, err := h.db.Exec(r.Context(), `DELETE FROM cart_item WHERE user_id=$1`, userID)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "could not clear cart")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CartCount(w http.ResponseWriter, r *http.Request) {
	userID, ok := requestUserID(r)
	if !ok {
		badRequest(w, "userId is required")
		return
	}
	var count int32
	if err := h.db.QueryRow(r.Context(), `SELECT COALESCE(SUM(quantity),0)::int FROM cart_item WHERE user_id=$1`, userID).Scan(&count); err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "could not count cart items")
		return
	}
	apiutil.WriteJSON(w, http.StatusOK, map[string]int32{"count": count})
}
