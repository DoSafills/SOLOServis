package features

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/DoSafills/SOLOServis/backend/internal/apiutil"
)

type ingestionProductInput struct {
	CategoryID  int32  `json:"categoryId"`
	BrandID     *int32 `json:"brandId"`
	Name        string `json:"name"`
	Model       string `json:"model"`
	SKU         string `json:"sku"`
	Description string `json:"description"`
	ImageURL    string `json:"imageUrl"`
	Source      string `json:"source"`
}

func (h *Handler) IngestProduct(w http.ResponseWriter, r *http.Request) {
	var input ingestionProductInput
	if !decode(r, &input) || input.CategoryID <= 0 || strings.TrimSpace(input.Name) == "" {
		badRequest(w, "categoryId and name are required")
		return
	}
	if input.ImageURL != "" {
		productRequest := map[string]any{
			"categoryId":  input.CategoryID,
			"brandId":     input.BrandID,
			"name":        input.Name,
			"model":       input.Model,
			"sku":         input.SKU,
			"description": input.Description,
			"imageUrl":    input.ImageURL,
		}
		var product json.RawMessage
		if err := remoteJSON(r.Context(), "PRODUCT_API_URL", "http://localhost:8081", http.MethodPost, "/products", productRequest, &product); err != nil {
			apiutil.WriteError(w, http.StatusBadGateway, "product API rejected the ingested product")
			return
		}
		apiutil.WriteJSON(w, http.StatusCreated, product)
		return
	}
	var publicID string
	err := h.db.QueryRow(r.Context(), `INSERT INTO product (category_id,brand_id,name,model,sku,description)
		VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,''),NULLIF($6,'')) RETURNING public_id::text`,
		input.CategoryID, input.BrandID, strings.TrimSpace(input.Name), input.Model, input.SKU, input.Description).Scan(&publicID)
	if err != nil {
		apiutil.WriteError(w, http.StatusBadRequest, "could not ingest product; check category, brand, and SKU")
		return
	}
	if input.ImageURL != "" {
		if _, err := h.db.Exec(r.Context(), `INSERT INTO product_image(product_id,image_url,alt_text) SELECT id,$2,$3 FROM product WHERE public_id=$1::uuid`, publicID, input.ImageURL, input.Name); err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, "product was saved but image could not be saved")
			return
		}
	}
	apiutil.WriteJSON(w, http.StatusCreated, map[string]any{"productId": publicID, "name": input.Name, "source": defaultSource(input.Source)})
}

func (h *Handler) IngestOffer(w http.ResponseWriter, r *http.Request) {
	var input offerInput
	if !decode(r, &input) || input.ProductID == "" || input.StoreID <= 0 || input.Price < 0 {
		badRequest(w, "productId, storeId and non-negative price are required")
		return
	}
	var store json.RawMessage
	if err := remoteJSON(r.Context(), "STORE_API_URL", "http://localhost:8082", http.MethodGet,
		"/stores/"+strconv.Itoa(int(input.StoreID)), nil, &store); err != nil {
		apiutil.WriteError(w, http.StatusBadGateway, "store API could not validate the offer store")
		return
	}
	var offer json.RawMessage
	if err := remoteJSON(r.Context(), "PRICING_API_URL", "http://localhost:8085", http.MethodPost, "/offers", input, &offer); err != nil {
		apiutil.WriteError(w, http.StatusBadGateway, "pricing API rejected the ingested offer")
		return
	}
	apiutil.WriteJSON(w, http.StatusCreated, offer)
}

type syncInput struct {
	Source  string          `json:"source"`
	Details json.RawMessage `json:"details"`
}

func (h *Handler) SyncIngestion(w http.ResponseWriter, r *http.Request) {
	var input syncInput
	if !decode(r, &input) {
		badRequest(w, "invalid sync payload")
		return
	}
	input.Source = defaultSource(input.Source)
	var result json.RawMessage
	err := h.db.QueryRow(r.Context(), `INSERT INTO ingestion_run(source,status,details)
		VALUES($1,'accepted',$2::jsonb) RETURNING jsonb_build_object('id',id,'source',source,'status',status,'details',details,'createdAt',created_at)`,
		input.Source, rawJSON(input.Details)).Scan(&result)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "could not create sync run")
		return
	}
	apiutil.WriteJSON(w, http.StatusAccepted, result)
}

func defaultSource(source string) string {
	if strings.TrimSpace(source) == "" {
		return "manual"
	}
	return strings.TrimSpace(source)
}
