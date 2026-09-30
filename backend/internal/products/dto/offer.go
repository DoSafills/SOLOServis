package dto

import (
	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/DoSafills/SOLOServis/backend/internal/money"
)

// FromProductOffer maps an offer row. Monetary values keep the exact decimal
// digits stored in NUMERIC(12,2) and are emitted as JSON numbers.
func FromProductOffer(offer generated.ListProductOffersByProductIDsRow) ProductOffer {
	var listPrice *money.Money
	if value := money.FromNumeric(offer.ListPrice); value.Valid() {
		listPrice = &value
	}

	var shippingCost *money.Money
	if value := money.FromNumeric(offer.ShippingCost); value.Valid() {
		shippingCost = &value
	}

	var stock *int32
	if offer.Stock.Valid {
		value := offer.Stock.Int32
		stock = &value
	}

	var productURL string
	if offer.ProductUrl.Valid {
		productURL = offer.ProductUrl.String
	}

	return ProductOffer{
		StoreID:      offer.StoreID,
		StoreName:    offer.StoreName,
		Price:        money.FromNumeric(offer.Price),
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
