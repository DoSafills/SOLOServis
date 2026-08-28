package api

import (
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/example/product-api/internal/db"
)

// ProductSummary es la representación JSON de un producto en el listado,
// con el rango de precios agregado entre todas las tiendas que lo venden.
type ProductSummary struct {
	ID          int64    `json:"id"`
	PublicID    string   `json:"public_id"`
	Name        string   `json:"name"`
	Model       *string  `json:"model,omitempty"`
	SKU         *string  `json:"sku,omitempty"`
	Description *string  `json:"description,omitempty"`
	Brand       *string  `json:"brand,omitempty"`
	Category    *string  `json:"category,omitempty"`
	MinPrice    *float64 `json:"min_price,omitempty"`
	MaxPrice    *float64 `json:"max_price,omitempty"`
	OfferCount  int64    `json:"offer_count"`
}

// ProductDetail es un producto con el detalle de todas sus ofertas
// (mismo producto, distintas tiendas, distintos precios).
type ProductDetail struct {
	ID          int64      `json:"id"`
	PublicID    string     `json:"public_id"`
	Name        string     `json:"name"`
	Model       *string    `json:"model,omitempty"`
	SKU         *string    `json:"sku,omitempty"`
	Description *string    `json:"description,omitempty"`
	Brand       *string    `json:"brand,omitempty"`
	Category    *string    `json:"category,omitempty"`
	Offers      []OfferDTO `json:"offers"`
}

// OfferDTO representa el precio de un producto en una tienda específica.
type OfferDTO struct {
	ID           int64    `json:"id"`
	StoreID      int64    `json:"store_id"`
	StoreName    string   `json:"store_name"`
	StoreLogoURL *string  `json:"store_logo_url,omitempty"`
	Price        float64  `json:"price"`
	ListPrice    *float64 `json:"list_price,omitempty"`
	Currency     string   `json:"currency"`
	ShippingCost *float64 `json:"shipping_cost,omitempty"`
	ShippingFree bool     `json:"shipping_free"`
	Available    bool     `json:"available"`
	Stock        *int32   `json:"stock,omitempty"`
	Condition    string   `json:"condition"`
	ProductURL   *string  `json:"product_url,omitempty"`
	LastUpdated  string   `json:"last_updated"`
}

// PaginatedProducts es la envoltura de respuesta para el listado paginado.
type PaginatedProducts struct {
	Data       []ProductSummary `json:"data"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	TotalItems int64            `json:"total_items"`
	TotalPages int              `json:"total_pages"`
}

// ---- helpers de conversión pgtype -> tipos Go / JSON ----

func textPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

func int4Ptr(i pgtype.Int4) *int32 {
	if !i.Valid {
		return nil
	}
	return &i.Int32
}

func numericPtr(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	return &f.Float64
}

func numericVal(n pgtype.Numeric) float64 {
	if v := numericPtr(n); v != nil {
		return *v
	}
	return 0
}

func uuidString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	b := u.Bytes
	return formatUUID(b)
}

func formatUUID(b [16]byte) string {
	const hexDigits = "0123456789abcdef"
	buf := make([]byte, 36)
	i := 0
	writeHex := func(bs []byte) {
		for _, x := range bs {
			buf[i] = hexDigits[x>>4]
			buf[i+1] = hexDigits[x&0xF]
			i += 2
		}
	}
	writeHex(b[0:4])
	buf[i] = '-'
	i++
	writeHex(b[4:6])
	buf[i] = '-'
	i++
	writeHex(b[6:8])
	buf[i] = '-'
	i++
	writeHex(b[8:10])
	buf[i] = '-'
	i++
	writeHex(b[10:16])
	return string(buf)
}

func toProductSummary(r db.ListProductsRow) ProductSummary {
	return ProductSummary{
		ID:          r.ID,
		PublicID:    uuidString(r.PublicID),
		Name:        r.Name,
		Model:       textPtr(r.Model),
		SKU:         textPtr(r.Sku),
		Description: textPtr(r.Description),
		Brand:       textPtr(r.BrandName),
		Category:    textPtr(r.CategoryName),
		MinPrice:    numericPtr(r.MinPrice),
		MaxPrice:    numericPtr(r.MaxPrice),
		OfferCount:  r.OfferCount,
	}
}

func toOfferDTO(r db.ListOffersByProductIDRow) OfferDTO {
	return OfferDTO{
		ID:           r.ID,
		StoreID:      r.StoreID,
		StoreName:    r.StoreName,
		StoreLogoURL: textPtr(r.StoreLogoUrl),
		Price:        numericVal(r.Price),
		ListPrice:    numericPtr(r.ListPrice),
		Currency:     r.Currency,
		ShippingCost: numericPtr(r.ShippingCost),
		ShippingFree: r.ShippingFree,
		Available:    r.Available,
		Stock:        int4Ptr(r.Stock),
		Condition:    r.Condition,
		ProductURL:   textPtr(r.ProductUrl),
		LastUpdated:  r.LastUpdated.Time.Format("2006-01-02T15:04:05Z07:00"),
	}
}
