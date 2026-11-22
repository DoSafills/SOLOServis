package features

import (
	"encoding/json"
	"fmt"
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

// validate aplica las reglas de una reseña: rating entre 1 y 5 y title de
// hasta maxReviewTitle caracteres. Devuelve el Validator con los errores por
// campo (vacío si la reseña es válida).
func (input reviewInput) validate() *httpx.Validator {
	var v httpx.Validator
	v.Check(input.Rating >= 1, "rating", "min", "rating debe ser al menos 1")
	v.Check(input.Rating <= 5, "rating", "max", "rating no puede superar 5")
	v.MaxLength("title", input.Title, maxReviewTitle)
	return &v
}

// reviewAuthor son los campos del autor que muestra una reseña: su nombre y si
// confirmó el email, igual que GET /products/{id}/reviews. El userId interno no
// se publica (TAL-200, hallazgo H12 de TAL-195). Espera la reseña con alias r
// y user_account con alias u.
const reviewAuthor = `'author',u.name,'authorVerified',u.email_verified`

// ListProductReviews atiende GET /reviews/product/{productId}: responde 200
// con las reseñas del producto, de la más nueva a la más antigua, con el
// autor por nombre (reviewAuthor). Responde 404 si el producto no existe y
// 500 si falla la base.
func (h *Handler) ListProductReviews(w http.ResponseWriter, r *http.Request) {
	productID, err := h.service.ProductID(r.Context(), chi.URLParam(r, "productId"))
	if err != nil {
		h.dbError(w, r, err, http.StatusNotFound, "Producto no encontrado.")
		return
	}
	h.rows(w, r, `SELECT jsonb_build_object('id','product:'||r.id,`+reviewAuthor+`,'rating',r.rating,'title',r.title,'content',r.content,'verified',r.verified,'createdAt',r.created_at)
		FROM product_review r JOIN user_account u ON u.id=r.user_id WHERE r.product_id=$1 ORDER BY r.created_at DESC`, productID)
}

// CreateProductReview atiende POST /reviews/product/{productId}: crea o
// reemplaza la reseña del usuario de la sesión. Responde 400 si productId no
// es un id numérico ni un UUID, 422 si la reseña no cumple las reglas y 201
// con la reseña guardada.
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
	if isUUID(productKey) {
		if err := verifyProductService(r.Context(), productKey); err != nil {
			h.remoteError(w, r, err, "Product API no está disponible.")
			return
		}
	}
	h.createReview(w, r, `WITH r AS (INSERT INTO product_review (user_id,product_id,rating,title,content) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (user_id,product_id) DO UPDATE SET rating=EXCLUDED.rating,title=EXCLUDED.title,content=EXCLUDED.content,updated_at=NOW()
		RETURNING id,user_id,product_id,rating,title,content,verified,created_at)
		SELECT jsonb_build_object('id','product:'||r.id,`+reviewAuthor+`,'productId',r.product_id,'rating',r.rating,'title',r.title,'content',r.content,'verified',r.verified,'createdAt',r.created_at)
		FROM r JOIN user_account u ON u.id=r.user_id`, input, productID)
}

// ListStoreReviews atiende GET /reviews/store/{storeId}: responde 200 con las
// reseñas de la tienda, de la más nueva a la más antigua, con el autor por
// nombre (reviewAuthor). Responde 400 si storeId no es un número positivo y
// 500 si falla la base.
func (h *Handler) ListStoreReviews(w http.ResponseWriter, r *http.Request) {
	storeID, ok := intParam(r, "storeId")
	if !ok {
		badRequest(w, r, "El id de la tienda no es válido.")
		return
	}
	h.rows(w, r, `SELECT jsonb_build_object('id','store:'||r.id,`+reviewAuthor+`,'rating',r.rating,'title',r.title,'content',r.content,'createdAt',r.created_at)
		FROM store_review r JOIN user_account u ON u.id=r.user_id WHERE r.store_id=$1 ORDER BY r.created_at DESC`, storeID)
}

// CreateStoreReview atiende POST /reviews/store/{storeId}: crea o reemplaza
// la reseña del usuario de la sesión. Responde 400 si storeId no es un número
// positivo, 422 si la reseña no cumple las reglas, 503 si Store API no
// confirma la tienda y 201 con la reseña guardada.
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
	h.createReview(w, r, `WITH r AS (INSERT INTO store_review (user_id,store_id,rating,title,content) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (user_id,store_id) DO UPDATE SET rating=EXCLUDED.rating,title=EXCLUDED.title,content=EXCLUDED.content,updated_at=NOW()
		RETURNING id,user_id,store_id,rating,title,content,created_at)
		SELECT jsonb_build_object('id','store:'||r.id,`+reviewAuthor+`,'storeId',r.store_id,'rating',r.rating,'title',r.title,'content',r.content,'createdAt',r.created_at)
		FROM r JOIN user_account u ON u.id=r.user_id`, input, storeID)
}

// ListServiceReviews atiende GET /reviews/service/{serviceId}: responde 200
// con las reseñas del servicio, de la más nueva a la más antigua, con el
// autor por nombre (reviewAuthor). Responde 404 si el servicio no existe y
// 500 si falla la base.
func (h *Handler) ListServiceReviews(w http.ResponseWriter, r *http.Request) {
	serviceID, err := h.service.ServiceID(r.Context(), chi.URLParam(r, "serviceId"))
	if err != nil {
		h.dbError(w, r, err, http.StatusNotFound, "Servicio no encontrado.")
		return
	}
	h.rows(w, r, `SELECT jsonb_build_object('id','service:'||r.id,`+reviewAuthor+`,'rating',r.rating,'title',r.title,'content',r.content,'verified',r.verified,'createdAt',r.created_at)
		FROM service_review r JOIN user_account u ON u.id=r.user_id WHERE r.service_id=$1 ORDER BY r.created_at DESC`, serviceID)
}

// CreateServiceReview atiende POST /reviews/service/{serviceId}: crea o
// reemplaza la reseña del usuario de la sesión. Responde 400 si serviceId no
// es un id numérico ni un UUID, 422 si la reseña no cumple las reglas y 201
// con la reseña guardada.
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
	if isUUID(serviceKey) {
		var service json.RawMessage
		if err := remoteJSON(r.Context(), "SERVICE_API_URL", "http://localhost:8083", http.MethodGet, "/services/"+serviceKey, nil, &service); err != nil {
			h.remoteError(w, r, err, "Service API no está disponible.")
			return
		}
	}
	h.createReview(w, r, `WITH r AS (INSERT INTO service_review (user_id,service_id,rating,title,content) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (user_id,service_id) DO UPDATE SET rating=EXCLUDED.rating,title=EXCLUDED.title,content=EXCLUDED.content,updated_at=NOW()
		RETURNING id,user_id,service_id,rating,title,content,verified,created_at)
		SELECT jsonb_build_object('id','service:'||r.id,`+reviewAuthor+`,'serviceId',r.service_id,'rating',r.rating,'title',r.title,'content',r.content,'verified',r.verified,'createdAt',r.created_at)
		FROM r JOIN user_account u ON u.id=r.user_id`, input, serviceID)
}

// createReview ejecuta query (un INSERT ... ON CONFLICT que devuelve la reseña
// como JSON con la forma del GET, sin el userId interno) con el usuario de la
// sesión, subjectID y los campos de input, y responde 201 con la reseña. Los
// errores de base se mapean con dbError (por ejemplo 23503 → 422).
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

// AddWatchlist atiende POST /watchlist: sigue el precio de productId para el
// usuario de la sesión. Responde 422 si productId falta o no existe, o si
// targetPrice está fuera de rango, y 201 con el producto y el precio objetivo.
func (h *Handler) AddWatchlist(w http.ResponseWriter, r *http.Request) {
	var input watchlistInput
	if !decode(w, r, &input) {
		return
	}
	var v httpx.Validator
	v.Required("productId", input.ProductID)
	checkTargetPrice(&v, input.TargetPrice)
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

// UpdateWatchlist atiende PUT /watchlist/{productId}: cambia el precio
// objetivo. Responde 422 si targetPrice está fuera de rango, 404 si el
// producto no está en la watchlist y 200 con el nuevo precio objetivo.
func (h *Handler) UpdateWatchlist(w http.ResponseWriter, r *http.Request) {
	productKey := chi.URLParam(r, "productId")
	var input watchlistInput
	if !decode(w, r, &input) {
		return
	}
	var v httpx.Validator
	checkTargetPrice(&v, input.TargetPrice)
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

// checkTargetPrice exige un targetPrice entre 0 y maxTargetPrice, el máximo
// de target_price NUMERIC(12,2). Recibe el Validator donde registra el error.
func checkTargetPrice(v *httpx.Validator, price float64) {
	v.Check(price >= 0, "targetPrice", "min", "targetPrice no puede ser negativo")
	v.Check(price <= maxTargetPrice, "targetPrice", "max",
		fmt.Sprintf("targetPrice no puede superar %.2f", maxTargetPrice))
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

// CreateNotification atiende POST /notifications (interno, con token de
// servicio). Responde 422 si falta un campo, si type o title superan el largo
// de su columna o si data no es un objeto JSON, y 201 con la notificación.
func (h *Handler) CreateNotification(w http.ResponseWriter, r *http.Request) {
	var input notificationInput
	if !decode(w, r, &input) {
		return
	}
	var v httpx.Validator
	v.Check(input.UserID > 0, "userId", "min", "userId debe ser mayor que 0")
	v.Required("type", input.Type)
	v.MaxLength("type", input.Type, maxNotificationType)
	v.Required("title", input.Title)
	v.MaxLength("title", input.Title, maxNotificationTitle)
	v.Required("message", input.Message)
	v.Check(isJSONObject(input.Data), "data", "invalid_type", "data debe ser un objeto JSON")
	if !valid(w, r, &v) {
		return
	}
	h.created(w, r, `WITH inserted AS (INSERT INTO notification(user_id,type,title,message,data)
		VALUES($1,$2,$3,$4,$5::jsonb) RETURNING id,user_id,type,title,message,data,created_at)
		SELECT jsonb_build_object('id',id,'userId',user_id,'type',type,'title',title,'message',message,'data',data,'createdAt',created_at) FROM inserted`,
		input.UserID, input.Type, input.Title, input.Message, rawJSON(input.Data))
}

// CreateAdminNotification atiende POST /notifications/admins (interno, con
// token de servicio o admin). Copia el aviso a cada usuario admin y responde
// 201 con cuántos se crearon.
//
// Existe para que Ingestion pueda avisar de una fuente degradada (TAL-211) sin
// conocer los ids de los usuarios ni consultar la tabla de cuentas: el
// destinatario lo resuelve Notification API, que es la dueña del dato.
func (h *Handler) CreateAdminNotification(w http.ResponseWriter, r *http.Request) {
	var input notificationInput
	if !decode(w, r, &input) {
		return
	}
	var v httpx.Validator
	v.Required("type", input.Type)
	v.MaxLength("type", input.Type, maxNotificationType)
	v.Required("title", input.Title)
	v.MaxLength("title", input.Title, maxNotificationTitle)
	v.Required("message", input.Message)
	v.Check(isJSONObject(input.Data), "data", "invalid_type", "data debe ser un objeto JSON")
	if !valid(w, r, &v) {
		return
	}
	h.created(w, r, `WITH inserted AS (
			INSERT INTO notification(user_id,type,title,message,data)
			SELECT u.id,$1,$2,$3,$4::jsonb FROM user_account u WHERE u.role = 'admin'
			RETURNING id)
		SELECT jsonb_build_object('created', (SELECT COUNT(*) FROM inserted))`,
		input.Type, input.Title, input.Message, rawJSON(input.Data))
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
