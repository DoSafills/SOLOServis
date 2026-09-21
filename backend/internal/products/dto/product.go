package dto

import (
	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
)

type ProductListItem struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Brand       string  `json:"brand"`
	Model       string  `json:"model"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Rating      float64 `json:"rating"`
	ReviewCount int64   `json:"reviewCount"`
	Offer       *ProductOffer `json:"offer,omitempty"`
}

func FromListProduct(row generated.ListProductsRow) ProductListItem {
	var model string
	if row.Model.Valid {
		model = row.Model.String
	}

	var description string
	if row.Description.Valid {
		description = row.Description.String
	}

	var brand string
	if row.BrandName.Valid {
		brand = row.BrandName.String
	}

	var rating float64
	if row.Rating.Valid {
		value, err := row.Rating.Float64Value()
		if err == nil && value.Valid {
			rating = value.Float64
		}
	}

	var offer *ProductOffer
	if row.OfferStoreID.Valid && row.OfferStoreName.Valid && row.OfferPrice.Valid {
		price, err := row.OfferPrice.MarshalJSON()
		if err == nil {
			shippingCost := ""
			if row.OfferShippingCost.Valid {
				value, shippingErr := row.OfferShippingCost.MarshalJSON()
				if shippingErr == nil {
					shippingCost = string(value)
				}
			}
			productURL := ""
			if row.OfferProductUrl.Valid {
				productURL = row.OfferProductUrl.String
			}
			offer = &ProductOffer{
				StoreID: row.OfferStoreID.Int32,
				StoreName: row.OfferStoreName.String,
				Price: string(price),
				ShippingCost: shippingCost,
				ShippingFree: row.OfferShippingFree.Valid && row.OfferShippingFree.Bool,
				Available: row.OfferAvailable.Valid && row.OfferAvailable.Bool,
				ProductURL: productURL,
			}
		}
	}

	return ProductListItem{
		ID:          row.PublicID.String(),
		Name:        row.Name,
		Brand:       brand,
		Model:       model,
		Category:    row.CategoryName,
		Description: description,
		Rating:      rating,
		ReviewCount: row.ReviewCount,
		Offer: offer,
	}
}
