package dto

import (
	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
)

type ProductOffer struct {
	ID              int32  `json:"id"`
	ProductID       int32  `json:"productId"`
	ProductPublicID string `json:"productPublicId"`
	ProductName     string `json:"productName"`
	StoreID         int32  `json:"storeId"`
	StoreName       string `json:"storeName"`
	Price           string `json:"price"`
	ListPrice       string `json:"listPrice"`
	Currency        string `json:"currency"`
	ShippingCost    string `json:"shippingCost"`
	ShippingFree    bool   `json:"shippingFree"`
	Available       bool   `json:"available"`
	Stock           *int32 `json:"stock"`
	Condition       string `json:"condition"`
	ProductURL      string `json:"productUrl"`
}

func FromListOffersRow(offer generated.ListOffersRow) ProductOffer {
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
		ID:              offer.ID,
		ProductID:       offer.ProductID,
		ProductPublicID: offer.ProductPublicID.String(),
		ProductName:     offer.ProductName,
		StoreID:         offer.StoreID,
		StoreName:       offer.StoreName,
		Price:           price,
		ListPrice:       listPrice,
		Currency:        offer.Currency,
		ShippingCost:    shippingCost,
		ShippingFree:    offer.ShippingFree,
		Available:       offer.Available,
		Stock:           stock,
		Condition:       offer.Condition,
		ProductURL:      productURL,
	}
}

func FromGetOfferByIDRow(offer generated.GetOfferByIDRow) ProductOffer {
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
		ID:              offer.ID,
		ProductID:       offer.ProductID,
		ProductPublicID: offer.ProductPublicID.String(),
		ProductName:     offer.ProductName,
		StoreID:         offer.StoreID,
		StoreName:       offer.StoreName,
		Price:           price,
		ListPrice:       listPrice,
		Currency:        offer.Currency,
		ShippingCost:    shippingCost,
		ShippingFree:    offer.ShippingFree,
		Available:       offer.Available,
		Stock:           stock,
		Condition:       offer.Condition,
		ProductURL:      productURL,
	}
}
