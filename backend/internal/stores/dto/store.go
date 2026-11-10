package dto

import (
	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/jackc/pgx/v5/pgtype"
)

type Store struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Logo         string  `json:"logo"`
	Rating       float64 `json:"rating"`
	ReviewCount  int64   `json:"reviewCount"`
	ProductCount int64   `json:"productCount"`
	Reputation   string  `json:"reputation"`
	DispatchTime string  `json:"dispatchTime"`
	Conditions   string  `json:"conditions"`
	Website      string  `json:"website"`
}

type StoreDetail struct {
	Store
	Products  []StoreProduct  `json:"products"`
	Locations []StoreLocation `json:"locations"`
}

type StoreProduct struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Brand        string `json:"brand"`
	Model        string `json:"model"`
	Category     string `json:"category"`
	Description  string `json:"description"`
	Price        string `json:"price"`
	ListPrice    string `json:"listPrice"`
	Currency     string `json:"currency"`
	ShippingCost string `json:"shippingCost"`
	ShippingFree bool   `json:"shippingFree"`
	Available    bool   `json:"available"`
	Stock        *int32 `json:"stock"`
	Condition    string `json:"condition"`
	ProductURL   string `json:"productUrl"`
	Image        string `json:"image"`
}

type StoreLocation struct {
	ID         int32  `json:"id"`
	LocationID int32  `json:"locationId"`
	Address    string `json:"address"`
	PostalCode string `json:"postalCode"`
	Country    string `json:"country"`
	Region     string `json:"region"`
	City       string `json:"city"`
	Commune    string `json:"commune"`
}

// StoreSummary es la tienda resumida que acompaña a cada oferta (TAL-73).
type StoreSummary struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Logo        string  `json:"logo"`
	Rating      float64 `json:"rating"`
	ReviewCount int64   `json:"reviewCount"`
	Website     string  `json:"website"`
}

// Offer es una oferta de un producto en una tienda (TAL-73). Los montos van
// como string para no perder precisión (igual que en Product API). Store solo
// viene en GET /stores/offers (por producto); en /stores/{id}/offers la tienda
// ya es la de la ruta.
type Offer struct {
	OfferID      int32         `json:"offerId"`
	ProductID    string        `json:"productId"`
	ProductName  string        `json:"productName"`
	Price        string        `json:"price"`
	ListPrice    string        `json:"listPrice"`
	Currency     string        `json:"currency"`
	ShippingCost string        `json:"shippingCost"`
	ShippingFree bool          `json:"shippingFree"`
	Available    bool          `json:"available"`
	Stock        *int32        `json:"stock"`
	Condition    string        `json:"condition"`
	ProductURL   string        `json:"productUrl"`
	LastUpdated  string        `json:"lastUpdated"`
	Store        *StoreSummary `json:"store,omitempty"`
}

func FromStoreProduct(row generated.ListStoreProductsRow) StoreProduct {
	return StoreProduct{
		ID:           row.PublicID.String(),
		Name:         row.Name,
		Brand:        TextValue(row.BrandName),
		Model:        TextValue(row.Model),
		Category:     row.CategoryName,
		Description:  TextValue(row.Description),
		Price:        NumericString(row.Price),
		ListPrice:    NumericString(row.ListPrice),
		Currency:     row.Currency,
		ShippingCost: NumericString(row.ShippingCost),
		ShippingFree: row.ShippingFree,
		Available:    row.Available,
		Stock:        IntValue(row.Stock),
		Condition:    row.Condition,
		ProductURL:   TextValue(row.ProductUrl),
		Image:        row.ImageUrl,
	}
}

func FromStoreLocation(row generated.ListStoreLocationsRow) StoreLocation {
	return StoreLocation{
		ID:         row.ID,
		LocationID: row.LocationID,
		Address:    TextValue(row.Address),
		PostalCode: TextValue(row.PostalCode),
		Country:    row.Country,
		Region:     TextValue(row.Region),
		City:       TextValue(row.City),
		Commune:    TextValue(row.Commune),
	}
}

func TextValue(value pgtype.Text) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func NumericValue(value pgtype.Numeric) float64 {
	if !value.Valid {
		return 0
	}

	result, err := value.Float64Value()
	if err != nil || !result.Valid {
		return 0
	}

	return result.Float64
}

func NumericString(value pgtype.Numeric) string {
	if !value.Valid {
		return ""
	}

	data, err := value.MarshalJSON()
	if err != nil {
		return ""
	}

	return string(data)
}

func IntValue(value pgtype.Int4) *int32 {
	if !value.Valid {
		return nil
	}

	result := value.Int32
	return &result
}
