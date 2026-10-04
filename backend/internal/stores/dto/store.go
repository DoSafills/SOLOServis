package dto

import (
	"strconv"

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

func FromStore(row generated.ListStoresRow) Store {
	return Store{
		ID:           strconv.Itoa(int(row.ID)),
		Name:         row.Name,
		Logo:         textValue(row.LogoUrl),
		Rating:       numericValue(row.Rating),
		ProductCount: row.ProductCount,
		Reputation:   textValue(row.Reputation),
		DispatchTime: textValue(row.ShippingInformation),
		Conditions:   textValue(row.GeneralConditions),
		Website:      textValue(row.WebsiteUrl),
	}
}

func FromStoreDetail(row generated.GetStoreByIDRow) Store {
	return Store{
		ID:           strconv.Itoa(int(row.ID)),
		Name:         row.Name,
		Logo:         textValue(row.LogoUrl),
		Rating:       numericValue(row.Rating),
		ProductCount: row.ProductCount,
		Reputation:   textValue(row.Reputation),
		DispatchTime: textValue(row.ShippingInformation),
		Conditions:   textValue(row.GeneralConditions),
		Website:      textValue(row.WebsiteUrl),
	}
}

func FromStoreProduct(row generated.ListStoreProductsRow) StoreProduct {
	return StoreProduct{
		ID:           row.PublicID.String(),
		Name:         row.Name,
		Brand:        textValue(row.BrandName),
		Model:        textValue(row.Model),
		Category:     row.CategoryName,
		Description:  textValue(row.Description),
		Price:        numericString(row.Price),
		ListPrice:    numericString(row.ListPrice),
		Currency:     row.Currency,
		ShippingCost: numericString(row.ShippingCost),
		ShippingFree: row.ShippingFree,
		Available:    row.Available,
		Stock:        intValue(row.Stock),
		Condition:    row.Condition,
		ProductURL:   textValue(row.ProductUrl),
		Image:        row.ImageUrl,
	}
}

func FromStoreLocation(row generated.ListStoreLocationsRow) StoreLocation {
	return StoreLocation{
		ID:         row.ID,
		LocationID: row.LocationID,
		Address:    textValue(row.Address),
		PostalCode: textValue(row.PostalCode),
		Country:    row.Country,
		Region:     textValue(row.Region),
		City:       textValue(row.City),
		Commune:    textValue(row.Commune),
	}
}

func textValue(value pgtype.Text) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func numericValue(value pgtype.Numeric) float64 {
	if !value.Valid {
		return 0
	}

	result, err := value.Float64Value()
	if err != nil || !result.Valid {
		return 0
	}

	return result.Float64
}

func numericString(value pgtype.Numeric) string {
	if !value.Valid {
		return ""
	}

	data, err := value.MarshalJSON()
	if err != nil {
		return ""
	}

	return string(data)
}

func intValue(value pgtype.Int4) *int32 {
	if !value.Valid {
		return nil
	}

	result := value.Int32
	return &result
}
