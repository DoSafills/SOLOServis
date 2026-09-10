package dto

import (
	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
)

type Store struct {
	ID                  int32   `json:"id"`
	Name                string  `json:"name"`
	WebsiteURL          string  `json:"websiteUrl"`
	LogoURL             string  `json:"logoUrl"`
	Rating              float64 `json:"rating"`
	Reputation          string  `json:"reputation"`
	ShippingInformation string  `json:"shippingInformation"`
	GeneralConditions   string  `json:"generalConditions"`
	Active              bool    `json:"active"`
}

func FromStore(store generated.Store) Store {
	var websiteURL string
	if store.WebsiteUrl.Valid {
		websiteURL = store.WebsiteUrl.String
	}

	var logoURL string
	if store.LogoUrl.Valid {
		logoURL = store.LogoUrl.String
	}

	var rating float64
	if store.Rating.Valid {
		value, err := store.Rating.Float64Value()
		if err == nil && value.Valid {
			rating = value.Float64
		}
	}

	var reputation string
	if store.Reputation.Valid {
		reputation = store.Reputation.String
	}

	var shippingInformation string
	if store.ShippingInformation.Valid {
		shippingInformation = store.ShippingInformation.String
	}

	var generalConditions string
	if store.GeneralConditions.Valid {
		generalConditions = store.GeneralConditions.String
	}

	return Store{
		ID:                  store.ID,
		Name:                store.Name,
		WebsiteURL:          websiteURL,
		LogoURL:             logoURL,
		Rating:              rating,
		Reputation:          reputation,
		ShippingInformation: shippingInformation,
		GeneralConditions:   generalConditions,
		Active:              store.Active,
	}
}
