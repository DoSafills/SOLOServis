package features

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/DoSafills/SOLOServis/backend/internal/apiutil"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
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
		badRequest(w, r, "categoryId and name are required")
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
	apiutil.WriteJSON(w, http.StatusCreated, map[string]any{"productId": publicID, "name": input.Name, "source": defaultSource(input.Source)})
}

type offerInput struct {
	ProductID    string       `json:"productId"`
	StoreID      int32        `json:"storeId"`
	Price        *json.Number `json:"price"`
	ListPrice    *json.Number `json:"listPrice"`
	Currency     string       `json:"currency"`
	ShippingCost *json.Number `json:"shippingCost"`
	ShippingFree bool         `json:"shippingFree"`
	Available    *bool        `json:"available"`
	Stock        *int32       `json:"stock"`
	Condition    string       `json:"condition"`
	ProductURL   string       `json:"productUrl"`
}

var ingestionMoneyPattern = regexp.MustCompile(`^\d{1,10}(\.\d{1,2})?$`)

func (h *Handler) IngestOffer(w http.ResponseWriter, r *http.Request) {
	var input offerInput
	if !decode(r, &input) {
		badRequest(w, r, "productId, storeId and non-negative price are required")
		return
	}
	input.ProductID = strings.TrimSpace(input.ProductID)
	var validation httpx.Validator
	validation.Required("productId", input.ProductID)
	validation.Check(input.StoreID > 0, "storeId", "min", "storeId debe ser mayor que 0")
	if input.Price == nil {
		validation.Required("price", "")
	} else {
		validation.Check(ingestionMoneyPattern.MatchString(input.Price.String()),
			"price", "format", "price debe ser un monto no negativo con hasta dos decimales")
	}
	for _, field := range []struct {
		name   string
		amount *json.Number
	}{
		{name: "listPrice", amount: input.ListPrice},
		{name: "shippingCost", amount: input.ShippingCost},
	} {
		validation.Check(field.amount == nil || ingestionMoneyPattern.MatchString(field.amount.String()),
			field.name, "format", field.name+" debe ser un monto no negativo con hasta dos decimales")
	}
	validation.Check(input.Stock == nil || *input.Stock >= 0,
		"stock", "min", "stock debe ser mayor o igual a 0")
	if !validation.Valid() {
		httpx.WriteValidation(w, r, validation.Errors())
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
		badRequest(w, r, "invalid sync payload")
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
