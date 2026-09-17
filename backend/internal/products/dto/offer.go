package dto

import (
	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
)

func FromProductOffer(offer generated.ListProductOffersRow) ProductOffer {
	price := ""
	if offer.Price.Valid {
		value, err := offer.Price.MarshalJSON()
		if err == nil {
			price = string(value)
		}
	}

	listPrice := ""
	if offer.ListPrice.Valid {
		value, err := offer.ListPrice.MarshalJSON()
		if err == nil {
			listPrice = string(value)
		}
	}

	shippingCost := ""
	if offer.ShippingCost.Valid {
		value, err := offer.ShippingCost.MarshalJSON()
		if err == nil {
			shippingCost = string(value)
		}
	}

	var stock *int32
	if offer.Stock.Valid {
		value := offer.Stock.Int32
		stock = &value
	}

	productURL := ""
	if offer.ProductUrl.Valid {
		productURL = offer.ProductUrl.String
	}

	return ProductOffer{
		StoreID:      offer.StoreID,
		StoreName:    offer.StoreName,
		Price:        price,
		ListPrice:    listPrice,
		Currency:     offer.Currency,
		ShippingCost: shippingCost,
		ShippingFree: offer.ShippingFree,
		Available:    offer.Available,
		Stock:        stock,
		Condition:    offer.Condition,
		ProductURL:   productURL,
	}
}
