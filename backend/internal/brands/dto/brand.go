package dto

import (
	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
)

type Brand struct {
	ID         int32  `json:"id"`
	Name       string `json:"name"`
	LogoURL    string `json:"logoUrl"`
	WebsiteURL string `json:"websiteUrl"`
	Active     bool   `json:"active"`
}

func FromBrand(brand generated.Brand) Brand {
	var logoURL string
	if brand.LogoUrl.Valid {
		logoURL = brand.LogoUrl.String
	}

	var websiteURL string
	if brand.WebsiteUrl.Valid {
		websiteURL = brand.WebsiteUrl.String
	}

	return Brand{
		ID:         brand.ID,
		Name:       brand.Name,
		LogoURL:    logoURL,
		WebsiteURL: websiteURL,
		Active:     brand.Active,
	}
}
