package dto

import (
	"strconv"

	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
)

// Store is one element of the GET /stores response.
type Store struct {
	ID           string  `json:"id" doc:"Internal identifier of the store, rendered as a string." example:"1"`
	Name         string  `json:"name" doc:"Name of the store." example:"Falabella"`
	Logo         string  `json:"logo" format:"uri" doc:"Absolute URL of the store logo, or an empty string." example:"https://cdn.example.com/stores/1.png"`
	Rating       float64 `json:"rating" minimum:"0" maximum:"5" doc:"Average rating, 0 when the store has no reviews." example:"4.1"`
	ReviewCount  int64   `json:"reviewCount" minimum:"0" doc:"Number of published reviews." example:"230"`
	ProductCount int64   `json:"productCount" minimum:"0" doc:"Number of products the store currently offers." example:"58"`
	Reputation   string  `json:"reputation" doc:"Reputation label, or an empty string." example:"Destacado"`
	DispatchTime string  `json:"dispatchTime" doc:"Shipping information, or an empty string." example:"Despacha en 24 horas"`
	Conditions   string  `json:"conditions" doc:"General purchase conditions, or an empty string." example:"Compra con tarjeta"`
	Website      string  `json:"website" format:"uri" doc:"Absolute URL of the store website, or an empty string." example:"https://www.falabella.com"`
}

// FromStore maps a store row.
func FromStore(row generated.ListStoresRow) Store {
	result := Store{
		// The public contract renders the internal identifier as a string, so
		// it is formatted through int64 rather than int to stay correct on
		// 32-bit platforms.
		ID:           strconv.FormatInt(int64(row.ID), 10),
		Name:         row.Name,
		ReviewCount:  0,
		ProductCount: row.ProductCount,
	}

	if row.LogoUrl.Valid {
		result.Logo = row.LogoUrl.String
	}

	if row.Rating.Valid {
		if value, err := row.Rating.Float64Value(); err == nil && value.Valid {
			result.Rating = value.Float64
		}
	}

	if row.Reputation.Valid {
		result.Reputation = row.Reputation.String
	}

	if row.ShippingInformation.Valid {
		result.DispatchTime = row.ShippingInformation.String
	}

	if row.GeneralConditions.Valid {
		result.Conditions = row.GeneralConditions.String
	}

	if row.WebsiteUrl.Valid {
		result.Website = row.WebsiteUrl.String
	}

	return result
}

// ListStores maps store rows into the API representation. The result is never
// nil so the JSON contains an empty array instead of null.
func ListStores(rows []generated.ListStoresRow) []Store {
	stores := make([]Store, 0, len(rows))
	for _, row := range rows {
		stores = append(stores, FromStore(row))
	}

	return stores
}
