package features

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/DoSafills/SOLOServis/backend/internal/apiutil"
	"github.com/go-chi/chi/v5"
)

type reviewInput struct {
	UserID  int32  `json:"userId"`
	Rating  int32  `json:"rating"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

func (h *Handler) ListProductReviews(w http.ResponseWriter, r *http.Request) {
	productID, err := h.service.ProductID(r.Context(), chi.URLParam(r, "productId"))
	if err != nil {
		apiutil.WriteError(w, http.StatusNotFound, "product not found")
		return
	}
	h.rows(w, r, `SELECT jsonb_build_object('id','product:'||id,'userId',user_id,'rating',rating,'title',title,'content',content,'verified',verified,'createdAt',created_at)
		FROM product_review WHERE product_id=$1 ORDER BY created_at DESC`, productID)
}

func (h *Handler) CreateProductReview(w http.ResponseWriter, r *http.Request) {
	productKey := chi.URLParam(r, "productId")
	productID, err := h.service.ProductID(r.Context(), productKey)
	var input reviewInput
	if err != nil || !decode(r, &input) || input.UserID <= 0 || input.Rating < 1 || input.Rating > 5 {
		badRequest(w, "valid productId, userId and rating from 1 to 5 are required")
		return
	}
	if len(productKey) == 36 {
		if err := verifyProductService(r.Context(), productKey); err != nil {
			apiutil.WriteError(w, http.StatusServiceUnavailable, "product service is unavailable")
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
		badRequest(w, "invalid store id")
		return
	}
	h.rows(w, r, `SELECT jsonb_build_object('id','store:'||id,'userId',user_id,'rating',rating,'title',title,'content',content,'createdAt',created_at)
		FROM store_review WHERE store_id=$1 ORDER BY created_at DESC`, storeID)
}

func (h *Handler) CreateStoreReview(w http.ResponseWriter, r *http.Request) {
	storeID, ok := intParam(r, "storeId")
	var input reviewInput
	if !ok || !decode(r, &input) || input.UserID <= 0 || input.Rating < 1 || input.Rating > 5 {
		badRequest(w, "valid storeId, userId and rating from 1 to 5 are required")
		return
	}
	var store json.RawMessage
	if err := remoteJSON(r.Context(), "STORE_API_URL", "http://localhost:8082", http.MethodGet, "/stores/"+strconv.Itoa(int(storeID)), nil, &store); err != nil {
		apiutil.WriteError(w, http.StatusServiceUnavailable, "store service is unavailable")
		return
	}
	h.createReview(w, r, `INSERT INTO store_review (user_id,store_id,rating,title,content) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (user_id,store_id) DO UPDATE SET rating=EXCLUDED.rating,title=EXCLUDED.title,content=EXCLUDED.content,updated_at=NOW()
		RETURNING jsonb_build_object('id','store:'||id,'userId',user_id,'storeId',store_id,'rating',rating,'title',title,'content',content,'createdAt',created_at)`, input, storeID)
}

func (h *Handler) ListServiceReviews(w http.ResponseWriter, r *http.Request) {
	serviceID, err := h.resolveServiceID(r.Context(), chi.URLParam(r, "serviceId"))
	if err != nil {
		apiutil.WriteError(w, http.StatusNotFound, "service not found")
		return
	}
	h.rows(w, r, `SELECT jsonb_build_object('id','service:'||id,'userId',user_id,'rating',rating,'title',title,'content',content,'verified',verified,'createdAt',created_at)
		FROM service_review WHERE service_id=$1 ORDER BY created_at DESC`, serviceID)
}

func (h *Handler) CreateServiceReview(w http.ResponseWriter, r *http.Request) {
	serviceKey := chi.URLParam(r, "serviceId")
	serviceID, err := h.resolveServiceID(r.Context(), serviceKey)
	var input reviewInput
	if err != nil || !decode(r, &input) || input.UserID <= 0 || input.Rating < 1 || input.Rating > 5 {
		badRequest(w, "valid serviceId, userId and rating from 1 to 5 are required")
		return
	}
	if len(serviceKey) == 36 {
		var service json.RawMessage
		if err := remoteJSON(r.Context(), "SERVICE_API_URL", "http://localhost:8083", http.MethodGet, "/services/"+serviceKey, nil, &service); err != nil {
			apiutil.WriteError(w, http.StatusServiceUnavailable, "service catalog API is unavailable")
			return
		}
	}
	h.createReview(w, r, `INSERT INTO service_review (user_id,service_id,rating,title,content) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (user_id,service_id) DO UPDATE SET rating=EXCLUDED.rating,title=EXCLUDED.title,content=EXCLUDED.content,updated_at=NOW()
		RETURNING jsonb_build_object('id','service:'||id,'userId',user_id,'serviceId',service_id,'rating',rating,'title',title,'content',content,'createdAt',created_at)`, input, serviceID)
}

func (h *Handler) createReview(w http.ResponseWriter, r *http.Request, query string, input reviewInput, subjectID int32) {
	var result json.RawMessage
	err := h.db.QueryRow(r.Context(), query, input.UserID, subjectID, input.Rating, input.Title, input.Content).Scan(&result)
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "could not save review")
		return
	}
	apiutil.WriteJSON(w, http.StatusCreated, result)
}

func (h *Handler) resolveServiceID(ctx context.Context, value string) (int32, error) {
	var id int32
	if _, err := strconv.Atoi(value); err == nil {
		err = h.db.QueryRow(ctx, `SELECT id FROM service WHERE id=$1 AND active`, value).Scan(&id)
		return id, err
	}
	if len(value) != 36 || strings.Count(value, "-") != 4 {
		return 0, errors.New("invalid service id")
	}
	err := h.db.QueryRow(ctx, `SELECT id FROM service WHERE public_id=$1::uuid AND active`, value).Scan(&id)
	return id, err
}

func (h *Handler) UpdateReview(w http.ResponseWriter, r *http.Request) {
	table, id, ok := parseReviewID(chi.URLParam(r, "id"))
	var input reviewInput
	if !ok || !decode(r, &input) || input.UserID <= 0 || input.Rating < 1 || input.Rating > 5 {
		badRequest(w, "valid userId and rating from 1 to 5 are required")
		return
	}
	result, err := h.db.Exec(r.Context(), `UPDATE `+table+` SET rating=$3,title=$4,content=$5,updated_at=NOW() WHERE id=$1 AND user_id=$2`, id, input.UserID, input.Rating, input.Title, input.Content)
	if err == nil && result.RowsAffected() > 0 {
		apiutil.WriteJSON(w, http.StatusOK, map[string]any{"id": chi.URLParam(r, "id"), "rating": input.Rating, "updated": true})
		return
	}
	apiutil.WriteError(w, http.StatusNotFound, "review not found")
}

func (h *Handler) DeleteReview(w http.ResponseWriter, r *http.Request) {
	table, id, ok := parseReviewID(chi.URLParam(r, "id"))
	userID, userOK := queryInt(r, "userId")
	if !ok || !userOK {
		badRequest(w, "valid id and userId are required")
		return
	}
	result, err := h.db.Exec(r.Context(), `DELETE FROM `+table+` WHERE id=$1 AND user_id=$2`, id, userID)
	if err == nil && result.RowsAffected() > 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	apiutil.WriteError(w, http.StatusNotFound, "review not found")
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
	userID, ok := queryInt(r, "userId")
	if !ok {
		badRequest(w, "userId is required")
		return
	}
	h.rows(w, r, `SELECT jsonb_build_object('productId',p.public_id,'name',p.name,'targetPrice',i.target_price,'createdAt',i.created_at)
		FROM product_user_interest i JOIN product p ON p.id=i.product_id WHERE i.user_id=$1 AND i.notify_price_drop
		ORDER BY i.updated_at DESC`, userID)
}

func (h *Handler) AddWatchlist(w http.ResponseWriter, r *http.Request) {
	var input watchlistInput
	if !decode(r, &input) || input.UserID <= 0 || input.ProductID == "" || input.TargetPrice < 0 {
		badRequest(w, "userId, productId and non-negative targetPrice are required")
		return
	}
	var offers []json.RawMessage
	if err := remoteJSON(r.Context(), "PRICING_API_URL", "http://localhost:8085", http.MethodGet, "/offers/product/"+input.ProductID, nil, &offers); err != nil {
		apiutil.WriteError(w, http.StatusServiceUnavailable, "pricing service is unavailable")
		return
	}
	productID, err := h.service.ProductID(r.Context(), input.ProductID)
	if err != nil {
		apiutil.WriteError(w, http.StatusNotFound, "product not found")
		return
	}
	_, err = h.db.Exec(r.Context(), `INSERT INTO product_user_interest(user_id,product_id,target_price,notify_price_drop)
		VALUES($1,$2,$3,TRUE) ON CONFLICT(user_id,product_id) DO UPDATE SET target_price=EXCLUDED.target_price,notify_price_drop=TRUE,updated_at=NOW()`, input.UserID, productID, input.TargetPrice)
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "could not add watchlist item")
		return
	}
	apiutil.WriteJSON(w, http.StatusCreated, map[string]any{"productId": input.ProductID, "targetPrice": input.TargetPrice})
}

func (h *Handler) UpdateWatchlist(w http.ResponseWriter, r *http.Request) {
	var input watchlistInput
	if !decode(r, &input) || input.UserID <= 0 || input.TargetPrice < 0 {
		badRequest(w, "userId and non-negative targetPrice are required")
		return
	}
	productKey := chi.URLParam(r, "productId")
	var offers []json.RawMessage
	if err := remoteJSON(r.Context(), "PRICING_API_URL", "http://localhost:8085", http.MethodGet, "/offers/product/"+productKey, nil, &offers); err != nil {
		apiutil.WriteError(w, http.StatusServiceUnavailable, "pricing service is unavailable")
		return
	}
	productID, err := h.service.ProductID(r.Context(), productKey)
	if err != nil {
		apiutil.WriteError(w, http.StatusNotFound, "product not found")
		return
	}
	result, err := h.db.Exec(r.Context(), `UPDATE product_user_interest SET target_price=$3,notify_price_drop=TRUE,updated_at=NOW()
		WHERE user_id=$1 AND product_id=$2 AND notify_price_drop`, input.UserID, productID, input.TargetPrice)
	if err != nil || result.RowsAffected() == 0 {
		apiutil.WriteError(w, http.StatusNotFound, "watchlist item not found")
		return
	}
	apiutil.WriteJSON(w, http.StatusOK, map[string]any{"productId": chi.URLParam(r, "productId"), "targetPrice": input.TargetPrice})
}

func (h *Handler) DeleteWatchlist(w http.ResponseWriter, r *http.Request) {
	userID, ok := queryInt(r, "userId")
	productID, err := h.service.ProductID(r.Context(), chi.URLParam(r, "productId"))
	if !ok || err != nil {
		badRequest(w, "valid userId and productId are required")
		return
	}
	result, err := h.db.Exec(r.Context(), `UPDATE product_user_interest SET notify_price_drop=FALSE,updated_at=NOW() WHERE user_id=$1 AND product_id=$2 AND notify_price_drop`, userID, productID)
	if err != nil || result.RowsAffected() == 0 {
		apiutil.WriteError(w, http.StatusNotFound, "watchlist item not found")
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
	if !decode(r, &input) || input.UserID <= 0 || strings.TrimSpace(input.Type) == "" || strings.TrimSpace(input.Title) == "" || strings.TrimSpace(input.Message) == "" {
		badRequest(w, "userId, type, title and message are required")
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
	userID, ok := queryInt(r, "userId")
	if !ok {
		badRequest(w, "userId is required")
		return
	}
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
	userID, userOK := queryInt(r, "userId")
	if !ok || !userOK {
		badRequest(w, "valid id and userId are required")
		return
	}
	result, err := h.db.Exec(r.Context(), `UPDATE notification SET read_at=NOW() WHERE id=$1 AND user_id=$2 AND read_at IS NULL`, id, userID)
	if err != nil || result.RowsAffected() == 0 {
		apiutil.WriteError(w, http.StatusNotFound, "unread notification not found")
		return
	}
	apiutil.WriteJSON(w, http.StatusOK, map[string]any{"id": id, "read": true})
}

func (h *Handler) ReadAllNotifications(w http.ResponseWriter, r *http.Request) {
	var input notificationInput
	if r.ContentLength != 0 {
		if !decode(r, &input) {
			badRequest(w, "invalid request body")
			return
		}
	}
	input.UserID = normalizeUserID(r, input.UserID)
	if input.UserID <= 0 {
		badRequest(w, "userId is required")
		return
	}
	_, err := h.db.Exec(r.Context(), `UPDATE notification SET read_at=NOW() WHERE user_id=$1 AND read_at IS NULL`, input.UserID)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "could not update notifications")
		return
	}
	apiutil.WriteJSON(w, http.StatusOK, map[string]any{"readAll": true})
}
