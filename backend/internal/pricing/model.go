// Package pricing implementa Pricing API (TAL-74): registra el precio actual
// de las ofertas (product_offer) y expone su historial
// (product_price_history, que llena el trigger de la migración 010).
package pricing

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
)

// Errores de dominio que el handler traduce a HTTP (TAL-78 §3.5).
var (
	ErrOfferNotFound   = errors.New("offer not found")
	ErrProductNotFound = errors.New("product not found")
	ErrStoreNotFound   = errors.New("store not found")
	ErrNoAvailable     = errors.New("no available offer")
)

// Offer es una oferta con su precio actual. Los montos son strings con dos
// decimales (NUMERIC(12,2)) para no perder precisión. TotalPrice = precio +
// envío (0 si el envío es gratis).
type Offer struct {
	ID           int32   `json:"id"`
	ProductID    string  `json:"productId"`
	StoreID      int32   `json:"storeId"`
	StoreName    string  `json:"storeName"`
	Price        string  `json:"price"`
	ListPrice    *string `json:"listPrice"`
	Currency     string  `json:"currency"`
	ShippingCost string  `json:"shippingCost"`
	ShippingFree bool    `json:"shippingFree"`
	TotalPrice   string  `json:"totalPrice"`
	Available    bool    `json:"available"`
	Stock        *int32  `json:"stock"`
	Condition    string  `json:"condition"`
	ProductURL   *string `json:"productUrl"`
	LastUpdated  string  `json:"lastUpdated"`
}

// PricePoint es un registro del historial de precios de una oferta.
type PricePoint struct {
	ID            int32  `json:"id"`
	Price         string `json:"price"`
	IsPromotional bool   `json:"isPromotional"`
	RecordedAt    string `json:"recordedAt"`
}

// ProductPricePoint es un registro del historial de un producto en todas sus
// tiendas.
type ProductPricePoint struct {
	PricePoint
	OfferID   int32  `json:"offerId"`
	StoreID   int32  `json:"storeId"`
	StoreName string `json:"storeName"`
}

// HistoryFilter acota el historial por fecha (YYYY-MM-DD, inclusivas) y pagina.
type HistoryFilter struct {
	From   *string
	To     *string
	Limit  int
	Offset int
}

// Límites de paginación del historial.
const (
	DefaultHistoryLimit = 100
	MaxHistoryLimit     = 500
)

// CreateOfferRequest es el body de POST /offers: registra el precio actual de
// un producto en una tienda. Si ya existe la oferta (mismo producto, tienda y
// URL normalizada) se actualiza.
type CreateOfferRequest struct {
	ProductID string `json:"productId"`
	StoreID   int32  `json:"storeId"`
	OfferFields
}

// UpdateOfferRequest es el body de PUT /offers/{id}: reemplaza los datos de
// la oferta. Producto y tienda no cambian.
type UpdateOfferRequest struct {
	OfferFields
}

// OfferFields son los datos comunes de POST y PUT. Los montos aceptan número
// JSON (o string numérico) con hasta 2 decimales.
type OfferFields struct {
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

// OfferWrite son los datos validados que se guardan. Los montos van como texto
// decimal y PostgreSQL los convierte a NUMERIC sin pasar por float64.
type OfferWrite struct {
	ProductID    int32
	StoreID      int32
	Price        string
	ListPrice    *string
	Currency     string
	ShippingCost string
	ShippingFree bool
	Available    bool
	Stock        *int32
	Condition    string
	ProductURL   *string
}

// moneyPattern admite lo que cabe en NUMERIC(12,2): 10 enteros y 2 decimales.
var moneyPattern = regexp.MustCompile(`^\d{1,10}(\.\d{1,2})?$`)

// parseMoney valida un monto. Devuelve el texto normalizado ("012.5" no se
// reescribe: PostgreSQL lo guarda como 12.50) o el código de error por campo.
func parseMoney(value *json.Number) (string, string) {
	text := strings.TrimSpace(value.String())
	if strings.HasPrefix(text, "-") {
		return "", "min"
	}
	if !moneyPattern.MatchString(text) {
		return "", "format"
	}
	return text, ""
}

// Condiciones permitidas (CHECK de product_offer.condition).
var conditions = map[string]bool{"new": true, "used": true, "refurbished": true}
