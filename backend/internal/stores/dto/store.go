package dto

import (
	"strconv"

	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
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

func FromStore(row generated.ListStoresRow) Store {
	result := Store{
		ID:           strconv.Itoa(int(row.ID)),
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
