package dto

import (
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
)

type PricePoint struct {
	ID             int32  `json:"id"`
	ProductOfferID int32  `json:"productOfferId"`
	Price          string `json:"price"`
	IsPromotional  bool   `json:"isPromotional"`
	RecordedAt     string `json:"recordedAt"`
}

func FromProductPriceHistory(history generated.ProductPriceHistory) PricePoint {
	price := ""
	if history.Price.Valid {
		value, err := history.Price.MarshalJSON()
		if err == nil {
			price = string(value)
		}
	}

	var recordedAt string
	if history.RecordedAt.Valid {
		recordedAt = history.RecordedAt.Time.Format(time.RFC3339)
	}

	return PricePoint{
		ID:             history.ID,
		ProductOfferID: history.ProductOfferID,
		Price:          price,
		IsPromotional:  history.IsPromotional,
		RecordedAt:     recordedAt,
	}
}
