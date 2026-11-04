package features

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/DoSafills/SOLOServis/backend/internal/apiutil"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
)

type reviewInput struct {
	UserID  int32  `json:"userId"`
	Rating  int32  `json:"rating"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// validate aplica las reglas de una reseña: rating entre 1 y 5.
func (input reviewInput) validate() *httpx.Validator {
	var v httpx.Validator
	v.Check(input.Rating >= 1, "rating", "min", "rating debe ser al menos 1")
	v.Check(input.Rating <= 5, "rating", "max", "rating no puede superar 5")
	return &v
}

func (h *Handler) ListProductReviews(w http.ResponseWriter, r *http.Request) {
	productID, err := h.service.ProductID(r.Context(), chi.URLParam(r, "productId"))
	if err != nil {
		h.dbError(w, r, err, http.StatusNotFound, "Producto no encontrado.")
		return
	}
	h.rows(w, r, `SELECT jsonb_build_object('id','product:'||id,'userId',user_id,'rating',rating,'title',title,'content',content,'verified',verified,'createdAt',created_at)
		FROM product_review WHERE product_id=$1 ORDER BY created_at DESC`, productID)
}

func (h *Handler) CreateProductReview(w http.ResponseWriter, r *http.Request) {
	productKey := chi.URLParam(r, "productId")
	var input reviewInput
	if !decode(w, r, &input) || !valid(w, r, input.validate()) {
		return
	}
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
	h.createReview(w, r, `INSERT INTO product_review (user_id,product_id,rating,title,content) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (user_id,product_id) DO UPDATE SET rating=EXCLUDED.rating,title=EXCLUDED.title,content=EXCLUDED.content,updated_at=NOW()
		RETURNING jsonb_build_object('id','product:'||id,'userId',user_id,'productId',product_id,'rating',rating,'title',title,'content',content,'createdAt',created_at)`, input, productID)
}

func (h *Handler) ListStoreReviews(w http.ResponseWriter, r *http.Request) {
	storeID, ok := intParam(r, "storeId")
	if !ok {
		badRequest(w, r, "El id de la tienda no es válido.")
		return
	}
	h.rows(w, r, `SELECT jsonb_build_object('id','store:'||id,'userId',user_id,'rating',rating,'title',title,'content',content,'createdAt',created_at)
		FROM store_review WHERE store_id=$1 ORDER BY created_at DESC`, storeID)
}

func (h *Handler) CreateStoreReview(w http.ResponseWriter, r *http.Request) {
	storeID, ok := intParam(r, "storeId")
	if !ok {
		badRequest(w, r, "El id de la tienda no es válido.")
		return
	}
	var input reviewInput
	if !decode(w, r, &input) || !valid(w, r, input.validate()) {
		return
	}
	var store json.RawMessage
	if err := remoteJSON(r.Context(), "STORE_API_URL", "http://localhost:8082", http.MethodGet, "/stores/"+strconv.Itoa(int(storeID)), nil, &store); err != nil {
		h.remoteError(w, r, err, "Store API no está disponible.")
		return
	}
	h.createReview(w, r, `INSERT INTO store_review (user_id,store_id,rating,title,content) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (user_id,store_id) DO UPDATE SET rating=EXCLUDED.rating,title=EXCLUDED.title,content=EXCLUDED.content,updated_at=NOW()
		RETURNING jsonb_build_object('id','store:'||id,'userId',user_id,'storeId',store_id,'rating',rating,'title',title,'content',content,'createdAt',created_at)`, input, storeID)
}

func (h *Handler) ListServiceReviews(w http.ResponseWriter, r *http.Request) {
	serviceID, err := h.service.ServiceID(r.Context(), chi.URLParam(r, "serviceId"))
	if err != nil {
		h.dbError(w, r, err, http.StatusNotFound, "Servicio no encontrado.")
		return
	}
	h.rows(w, r, `SELECT jsonb_build_object('id','service:'||id,'userId',user_id,'rating',rating,'title',title,'content',content,'verified',verified,'createdAt',created_at)
		FROM service_review WHERE service_id=$1 ORDER BY created_at DESC`, serviceID)
}

func (h *Handler) CreateServiceReview(w http.ResponseWriter, r *http.Request) {
	serviceKey := chi.URLParam(r, "serviceId")
	var input reviewInput
	if !decode(w, r, &input) || !valid(w, r, input.validate()) {
		return
	}
	serviceID, err := h.service.ServiceID(r.Context(), serviceKey)
	if err != nil {
		h.dbError(w, r, err, http.StatusNotFound, "Servicio no encontrado.")
		return
	}
	if looksLikeUUID(serviceKey) {
		var service json.RawMessage
		if err := remoteJSON(r.Context(), "SERVICE_API_URL", "http://localhost:8083", http.MethodGet, "/services/"+serviceKey, nil, &service); err != nil {
			h.remoteError(w, r, err, "Service API no está disponible.")
			return
		}
	}
	h.createReview(w, r, `INSERT INTO service_review (user_id,service_id,rating,title,content) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (user_id,service_id) DO UPDATE SET rating=EXCLUDED.rating,title=EXCLUDED.title,content=EXCLUDED.content,updated_at=NOW()
		RETURNING jsonb_build_object('id','service:'||id,'userId',user_id,'serviceId',service_id,'rating',rating,'title',title,'content',content,'createdAt',created_at)`, input, serviceID)
}

func (h *Handler) createReview(w http.ResponseWriter, r *http.Request, query string, input reviewInput, subjectID int32) {
	var result json.RawMessage
	err := h.db.QueryRow(r.Context(), query, sessionUserID(r), subjectID, input.Rating, input.Title, input.Content).Scan(&result)
	if err != nil {
		h.dbError(w, r, err, http.StatusInternalServerError, "No se pudo guardar la reseña.")
		return
	}
	apiutil.WriteJSON(w, http.StatusCreated, result)
}

func (h *Handler) UpdateReview(w http.ResponseWriter, r *http.Request) {
	table, id, ok := parseReviewID(chi.URLParam(r, "id"))
	if !ok {
		badRequest(w, r, "El id de la reseña no es válido.")
		return
	}
	var input reviewInput
	if !decode(w, r, &input) || !valid(w, r, input.validate()) {
		return
	}
	result, err := h.db.Exec(r.Context(), `UPDATE `+table+` SET rating=$3,title=$4,content=$5,updated_at=NOW() WHERE id=$1 AND user_id=$2`, id, sessionUserID(r), input.Rating, input.Title, input.Content)
	if err != nil || result.RowsAffected() == 0 {
		h.dbError(w, r, err, http.StatusNotFound, "Reseña no encontrada.")
		return
	}
	apiutil.WriteJSON(w, http.StatusOK, map[string]any{"id": chi.URLParam(r, "id"), "rating": input.Rating, "updated": true})
}

func (h *Handler) DeleteReview(w http.ResponseWriter, r *http.Request) {
	table, id, ok := parseReviewID(chi.URLParam(r, "id"))
	if !ok {
		badRequest(w, r, "El id de la reseña no es válido.")
		return
	}
	result, err := h.db.Exec(r.Context(), `DELETE FROM `+table+` WHERE id=$1 AND user_id=$2`, id, sessionUserID(r))
	if err != nil || result.RowsAffected() == 0 {
		h.dbError(w, r, err, http.StatusNotFound, "Reseña no encontrada.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseReviewID(value string) (string, int64, bool) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return "", 0, false
	}
	tables := map[string]string{"product": "product_review", "store": "store_review", "service": "service_review"}
	table, ok := tables[parts[0]]
	if !ok {
		return "", 0, false
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	return table, id, err == nil && id > 0
}

type watchlistInput struct {
	UserID      int32   `json:"userId"`
	ProductID   string  `json:"productId"`
	TargetPrice float64 `json:"targetPrice"`
}

func (h *Handler) GetWatchlist(w http.ResponseWriter, r *http.Request) {
	userID := sessionUserID(r)
	h.rows(w, r, `SELECT jsonb_build_object('productId',p.public_id,'name',p.name,'targetPrice',i.target_price,'createdAt',i.created_at)
		FROM product_user_interest i JOIN product p ON p.id=i.product_id WHERE i.user_id=$1 AND i.notify_price_drop
		ORDER BY i.updated_at DESC`, userID)
}

func (h *Handler) AddWatchlist(w http.ResponseWriter, r *http.Request) {
	var input watchlistInput
	if !decode(w, r, &input) {
		return
	}
	var v httpx.Validator
	v.Required("productId", input.ProductID)
	v.Check(input.TargetPrice >= 0, "targetPrice", "min", "targetPrice no puede ser negativo")
	if !valid(w, r, &v) {
		return
	}
	productID, err := h.service.ProductID(r.Context(), input.ProductID)
	if err != nil {
		h.referenceError(w, r, err, "productId")
		return
	}
	var offers []json.RawMessage
	if err := remoteJSON(r.Context(), "PRICING_API_URL", "http://localhost:8085", http.MethodGet, "/offers/product/"+input.ProductID, nil, &offers); err != nil {
		h.remoteError(w, r, err, "Pricing API no está disponible.")
		return
	}
	_, err = h.db.Exec(r.Context(), `INSERT INTO product_user_interest(user_id,product_id,target_price,notify_price_drop)
		VALUES($1,$2,$3,TRUE) ON CONFLICT(user_id,product_id) DO UPDATE SET target_price=EXCLUDED.target_price,notify_price_drop=TRUE,updated_at=NOW()`, sessionUserID(r), productID, input.TargetPrice)
	if err != nil {
		h.dbError(w, r, err, http.StatusInternalServerError, "No se pudo agregar el producto a la watchlist.")
		return
	}
	apiutil.WriteJSON(w, http.StatusCreated, map[string]any{"productId": input.ProductID, "targetPrice": input.TargetPrice})
}

func (h *Handler) UpdateWatchlist(w http.ResponseWriter, r *http.Request) {
	productKey := chi.URLParam(r, "productId")
	var input watchlistInput
	if !decode(w, r, &input) {
		return
	}
	var v httpx.Validator
	v.Check(input.TargetPrice >= 0, "targetPrice", "min", "targetPrice no puede ser negativo")
	if !valid(w, r, &v) {
		return
	}
	productID, err := h.service.ProductID(r.Context(), productKey)
	if err != nil {
		h.dbError(w, r, err, http.StatusNotFound, "Producto no encontrado.")
		return
	}
	var offers []json.RawMessage
	if err := remoteJSON(r.Context(), "PRICING_API_URL", "http://localhost:8085", http.MethodGet, "/offers/product/"+productKey, nil, &offers); err != nil {
		h.remoteError(w, r, err, "Pricing API no está disponible.")
		return
	}
	result, err := h.db.Exec(r.Context(), `UPDATE product_user_interest SET target_price=$3,notify_price_drop=TRUE,updated_at=NOW()
		WHERE user_id=$1 AND product_id=$2 AND notify_price_drop`, sessionUserID(r), productID, input.TargetPrice)
	if err != nil || result.RowsAffected() == 0 {
		h.dbError(w, r, err, http.StatusNotFound, "El producto no está en la watchlist.")
		return
	}
	apiutil.WriteJSON(w, http.StatusOK, map[string]any{"productId": productKey, "targetPrice": input.TargetPrice})
}

func (h *Handler) DeleteWatchlist(w http.ResponseWriter, r *http.Request) {
	userID := sessionUserID(r)
	productID, err := h.service.ProductID(r.Context(), chi.URLParam(r, "productId"))
	if err != nil {
		h.dbError(w, r, err, http.StatusNotFound, "Producto no encontrado.")
		return
	}
	result, err := h.db.Exec(r.Context(), `UPDATE product_user_interest SET notify_price_drop=FALSE,updated_at=NOW() WHERE user_id=$1 AND product_id=$2 AND notify_price_drop`, userID, productID)
	if err != nil || result.RowsAffected() == 0 {
		h.dbError(w, r, err, http.StatusNotFound, "El producto no está en la watchlist.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type notificationInput struct {
	UserID  int32           `json:"userId"`
	Type    string          `json:"type"`
	Title   string          `json:"title"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (h *Handler) CreateNotification(w http.ResponseWriter, r *http.Request) {
	var input notificationInput
	if !decode(w, r, &input) {
		return
	}
	var v httpx.Validator
	v.Check(input.UserID > 0, "userId", "min", "userId debe ser mayor que 0")
	v.Required("type", input.Type)
	v.Required("title", input.Title)
	v.Required("message", input.Message)
	if !valid(w, r, &v) {
		return
	}
	h.created(w, r, `WITH inserted AS (INSERT INTO notification(user_id,type,title,message,data)
		VALUES($1,$2,$3,$4,$5::jsonb) RETURNING id,user_id,type,title,message,data,created_at)
		SELECT jsonb_build_object('id',id,'userId',user_id,'type',type,'title',title,'message',message,'data',data,'createdAt',created_at) FROM inserted`,
		input.UserID, input.Type, input.Title, input.Message, rawJSON(input.Data))
}

func (h *Handler) GetNotifications(w http.ResponseWriter, r *http.Request) {
	h.listNotifications(w, r, false)
}

func (h *Handler) GetUnreadNotifications(w http.ResponseWriter, r *http.Request) {
	h.listNotifications(w, r, true)
}

func (h *Handler) listNotifications(w http.ResponseWriter, r *http.Request, unread bool) {
	userID := sessionUserID(r)
	query := `SELECT jsonb_build_object('id',id,'type',type,'title',title,'message',message,'data',data,'read',read_at IS NOT NULL,'createdAt',created_at)
		FROM notification WHERE user_id=$1 ORDER BY created_at DESC LIMIT 100`
	if unread {
		query = `SELECT jsonb_build_object('id',id,'type',type,'title',title,'message',message,'data',data,'read',FALSE,'createdAt',created_at)
			FROM notification WHERE user_id=$1 AND read_at IS NULL ORDER BY created_at DESC LIMIT 100`
	}
	h.rows(w, r, query, userID)
}

func (h *Handler) ReadNotification(w http.ResponseWriter, r *http.Request) {
	id, ok := intParam(r, "id")
	if !ok {
		badRequest(w, r, "El id de la notificación no es válido.")
		return
	}
	result, err := h.db.Exec(r.Context(), `UPDATE notification SET read_at=NOW() WHERE id=$1 AND user_id=$2 AND read_at IS NULL`, id, sessionUserID(r))
	if err != nil || result.RowsAffected() == 0 {
		h.dbError(w, r, err, http.StatusNotFound, "Notificación sin leer no encontrada.")
		return
	}
	apiutil.WriteJSON(w, http.StatusOK, map[string]any{"id": id, "read": true})
}

// ReadAllNotifications acepta un body opcional (el frontend envía userId,
// que se ignora: manda la sesión).
func (h *Handler) ReadAllNotifications(w http.ResponseWriter, r *http.Request) {
	if r.ContentLength != 0 {
		var input notificationInput
		if !decode(w, r, &input) {
			return
		}
	}
	_, err := h.db.Exec(r.Context(), `UPDATE notification SET read_at=NOW() WHERE user_id=$1 AND read_at IS NULL`, sessionUserID(r))
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	apiutil.WriteJSON(w, http.StatusOK, map[string]any{"readAll": true})
}
