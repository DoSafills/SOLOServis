package features

import (
	"net/http"
	"strconv"

	"github.com/DoSafills/SOLOServis/backend/internal/apiutil"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
)

type favoriteInput struct {
	UserID int32 `json:"userId"`
}

func (h *Handler) ListFavorites(w http.ResponseWriter, r *http.Request) {
	userID := sessionUserID(r)
	h.rows(w, r, `SELECT jsonb_build_object('productId',p.public_id,'name',p.name,'addedAt',i.updated_at)
		FROM product_user_interest i JOIN product p ON p.id=i.product_id
		WHERE i.user_id=$1 AND i.is_favorite ORDER BY i.updated_at DESC`, userID)
}

func (h *Handler) AddFavorite(w http.ResponseWriter, r *http.Request) {
	var input favoriteInput
	if !decode(w, r, &input) {
		return
	}
	userID, err := h.service.UserID(r.Context(), sessionUserID(r))
	if err != nil {
		h.dbError(w, r, err, http.StatusNotFound, "Usuario no encontrado.")
		return
	}
	productKey := chi.URLParam(r, "productId")
	productID, err := h.service.ProductID(r.Context(), productKey)
	if err != nil {
		h.dbError(w, r, err, http.StatusNotFound, "Producto no encontrado.")
		return
	}
	if looksLikeUUID(productKey) {
		if err := verifyProductService(r.Context(), productKey); err != nil {
			h.remoteError(w, r, err, "Product API no está disponible.")
			return
		}
	}
	_, err = h.db.Exec(r.Context(), `INSERT INTO product_user_interest (user_id,product_id,is_favorite)
		VALUES ($1,$2,TRUE) ON CONFLICT (user_id,product_id) DO UPDATE SET is_favorite=TRUE,updated_at=NOW()`, userID, productID)
	if err != nil {
		h.dbError(w, r, err, http.StatusInternalServerError, "No se pudo guardar el favorito.")
		return
	}
	apiutil.WriteJSON(w, http.StatusCreated, map[string]any{"productId": productKey, "favorite": true})
}

func (h *Handler) GetFavorite(w http.ResponseWriter, r *http.Request) {
	userID := sessionUserID(r)
	productID, err := h.service.ProductID(r.Context(), chi.URLParam(r, "productId"))
	if err != nil {
		h.dbError(w, r, err, http.StatusNotFound, "Producto no encontrado.")
		return
	}
	h.one(w, r, "El producto no está en favoritos.", `SELECT jsonb_build_object('productId',p.public_id,'favorite',i.is_favorite)
		FROM product p LEFT JOIN product_user_interest i ON i.product_id=p.id AND i.user_id=$2
		WHERE p.id=$1 AND COALESCE(i.is_favorite,FALSE)`, productID, userID)
}

func (h *Handler) DeleteFavorite(w http.ResponseWriter, r *http.Request) {
	userID := sessionUserID(r)
	productID, err := h.service.ProductID(r.Context(), chi.URLParam(r, "productId"))
	if err != nil {
		h.dbError(w, r, err, http.StatusNotFound, "Producto no encontrado.")
		return
	}
	result, err := h.db.Exec(r.Context(), `UPDATE product_user_interest SET is_favorite=FALSE,updated_at=NOW() WHERE user_id=$1 AND product_id=$2 AND is_favorite`, userID, productID)
	if err != nil || result.RowsAffected() == 0 {
		h.dbError(w, r, err, http.StatusNotFound, "Favorito no encontrado.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type cartInput struct {
	UserID         int32  `json:"userId"`
	ProductID      string `json:"productId"`
	OfferID        int32  `json:"offerId"`
	ServiceID      string `json:"serviceId"`
	ServiceOfferID int32  `json:"serviceOfferId"`
	Quantity       int32  `json:"quantity"`
}

func (h *Handler) GetCart(w http.ResponseWriter, r *http.Request) {
	userID := sessionUserID(r)
	h.rows(w, r, `SELECT jsonb_build_object('id',ci.id,'userId',ci.user_id,
		'itemType',CASE WHEN ci.service_offer_id IS NULL THEN 'product' ELSE 'service' END,
		'productId',p.public_id,'productName',p.name,'offerId',o.id,'storeId',o.store_id,
		'storeName',s.name,'productUrl',o.product_url,'serviceId',sv.public_id,
		'serviceName',sv.name,'serviceOfferId',so.id,'serviceProvider',sp.name,
		'serviceMonthlyPrice',so.price,'serviceInstallationCost',so.installation_cost,
		'serviceBillingPeriod',so.billing_period,'serviceContractPeriod',so.contract_period,'quantity',ci.quantity,
		'savedPrice',ci.saved_price,'savedShippingCost',ci.saved_shipping_cost,
		'currentPrice',COALESCE(o.price,so.price),'updatedAt',ci.updated_at)
		FROM cart_item ci
		LEFT JOIN product p ON p.id=ci.product_id
		LEFT JOIN product_offer o ON o.id=ci.offer_id
		LEFT JOIN store s ON s.id=o.store_id
		LEFT JOIN service_offer so ON so.id=ci.service_offer_id
		LEFT JOIN service sv ON sv.id=so.service_id
		LEFT JOIN provider sp ON sp.id=so.provider_id
		WHERE ci.user_id=$1 ORDER BY ci.created_at DESC`, userID)
}

func (h *Handler) AddCartItem(w http.ResponseWriter, r *http.Request) {
	var input cartInput
	if !decode(w, r, &input) {
		return
	}
	productItem := input.ProductID != "" && input.OfferID > 0 &&
		input.ServiceID == "" && input.ServiceOfferID == 0
	serviceItem := input.ServiceID != "" && input.ServiceOfferID > 0 &&
		input.ProductID == "" && input.OfferID == 0

	var v httpx.Validator
	v.Check(input.Quantity > 0, "quantity", "min", "quantity debe ser mayor que 0")
	v.Check(productItem || serviceItem, "productId", "one_of",
		"Indica productId y offerId, o serviceId y serviceOfferId")
	if !valid(w, r, &v) {
		return
	}
	userID, err := h.service.UserID(r.Context(), sessionUserID(r))
	if err != nil {
		h.dbError(w, r, err, http.StatusNotFound, "Usuario no encontrado.")
		return
	}
	if serviceItem {
		h.addServiceCartItem(w, r, input, userID)
		return
	}
	tx, err := h.db.Begin(r.Context())
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
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
	if err != nil {
		h.referenceError(w, r, err, "offerId")
		return
	}
	if !available {
		writeError(w, r, http.StatusConflict, "La oferta no está disponible.")
		return
	}
	requestedID, err := h.service.ProductID(r.Context(), input.ProductID)
	if err != nil {
		h.referenceError(w, r, err, "productId")
		return
	}
	if requestedID != productID {
		httpx.WriteValidation(w, r, []httpx.FieldError{{Field: "offerId", Code: "mismatch",
			Message: "offerId no es una oferta de productId"}})
		return
	}
	if err := verifyProductService(r.Context(), productPublicID); err != nil {
		h.remoteError(w, r, err, "Product API no está disponible.")
		return
	}
	var priceOffer struct {
		ProductID string `json:"productId"`
	}
	if err := remoteJSON(r.Context(), "PRICING_API_URL", "http://localhost:8085", http.MethodGet, "/offers/"+strconv.Itoa(int(input.OfferID)), nil, &priceOffer); err != nil {
		h.remoteError(w, r, err, "Pricing API no está disponible.")
		return
	}
	if priceOffer.ProductID != productPublicID {
		writeError(w, r, http.StatusConflict, "Pricing API devolvió otro producto para esta oferta.")
		return
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO cart_item (user_id,product_id,offer_id,quantity,saved_price,saved_shipping_cost)
		VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (user_id,offer_id) DO UPDATE SET
		quantity=cart_item.quantity+EXCLUDED.quantity,saved_price=EXCLUDED.saved_price,
		saved_shipping_cost=EXCLUDED.saved_shipping_cost,updated_at=NOW()`, userID, productID, input.OfferID, input.Quantity, price, shipping)
	if err != nil {
		h.dbError(w, r, err, http.StatusInternalServerError, "No se pudo agregar la oferta al carrito.")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		h.dbError(w, r, err, http.StatusInternalServerError, "No se pudo agregar la oferta al carrito.")
		return
	}
	apiutil.WriteJSON(w, http.StatusCreated, map[string]any{"userId": userID, "productId": productPublicID, "offerId": input.OfferID, "quantity": input.Quantity, "savedPrice": price, "savedShippingCost": shipping})
}

func (h *Handler) addServiceCartItem(w http.ResponseWriter, r *http.Request, input cartInput, userID int32) {
	serviceID, err := h.service.ServiceID(r.Context(), input.ServiceID)
	if err != nil {
		h.referenceError(w, r, err, "serviceId")
		return
	}

	var publicID string
	var price float64
	var offerServiceID int32
	err = h.db.QueryRow(r.Context(), `SELECT s.public_id::text,so.service_id,so.price
		FROM service_offer so JOIN service s ON s.id=so.service_id
		JOIN provider p ON p.id=so.provider_id
		WHERE so.id=$1 AND so.available AND p.active`, input.ServiceOfferID).
		Scan(&publicID, &offerServiceID, &price)
	if err != nil {
		h.referenceError(w, r, err, "serviceOfferId")
		return
	}
	if offerServiceID != serviceID {
		httpx.WriteValidation(w, r, []httpx.FieldError{{Field: "serviceOfferId", Code: "mismatch",
			Message: "serviceOfferId no es una oferta de serviceId"}})
		return
	}

	_, err = h.db.Exec(r.Context(), `INSERT INTO cart_item
		(user_id,product_id,offer_id,service_offer_id,quantity,saved_price,saved_shipping_cost)
		VALUES ($1,NULL,NULL,$2,$3,$4,0)
		ON CONFLICT (user_id,service_offer_id) WHERE service_offer_id IS NOT NULL DO NOTHING`,
		userID, input.ServiceOfferID, input.Quantity, price)
	if err != nil {
		h.dbError(w, r, err, http.StatusInternalServerError, "No se pudo agregar el servicio al carrito.")
		return
	}
	apiutil.WriteJSON(w, http.StatusCreated, map[string]any{
		"userId": userID, "serviceId": publicID, "serviceOfferId": input.ServiceOfferID,
		"quantity": input.Quantity, "savedPrice": price,
	})
}

func (h *Handler) UpdateCartItem(w http.ResponseWriter, r *http.Request) {
	id, ok := intParam(r, "id")
	if !ok {
		badRequest(w, r, "El id del ítem del carrito no es válido.")
		return
	}
	var input cartInput
	if !decode(w, r, &input) {
		return
	}
	var v httpx.Validator
	v.Check(input.Quantity > 0, "quantity", "min", "quantity debe ser mayor que 0")
	if !valid(w, r, &v) {
		return
	}
	result, err := h.db.Exec(r.Context(), `UPDATE cart_item SET quantity=$3,updated_at=NOW() WHERE id=$1 AND user_id=$2`, id, sessionUserID(r), input.Quantity)
	if err != nil || result.RowsAffected() == 0 {
		h.dbError(w, r, err, http.StatusNotFound, "Ítem del carrito no encontrado.")
		return
	}
	apiutil.WriteJSON(w, http.StatusOK, map[string]any{"id": id, "quantity": input.Quantity})
}

func (h *Handler) DeleteCartItem(w http.ResponseWriter, r *http.Request) {
	id, ok := intParam(r, "id")
	if !ok {
		badRequest(w, r, "El id del ítem del carrito no es válido.")
		return
	}
	result, err := h.db.Exec(r.Context(), `DELETE FROM cart_item WHERE id=$1 AND user_id=$2`, id, sessionUserID(r))
	if err != nil || result.RowsAffected() == 0 {
		h.dbError(w, r, err, http.StatusNotFound, "Ítem del carrito no encontrado.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ClearCart(w http.ResponseWriter, r *http.Request) {
	userID := sessionUserID(r)
	_, err := h.db.Exec(r.Context(), `DELETE FROM cart_item WHERE user_id=$1`, userID)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CartCount(w http.ResponseWriter, r *http.Request) {
	userID := sessionUserID(r)
	var count int32
	if err := h.db.QueryRow(r.Context(), `SELECT COALESCE(SUM(quantity),0)::int FROM cart_item WHERE user_id=$1`, userID).Scan(&count); err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	apiutil.WriteJSON(w, http.StatusOK, map[string]int32{"count": count})
}
