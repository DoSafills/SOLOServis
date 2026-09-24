package dto

import (
	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
)

type CreateProductRequest struct {
	CategoryID  int32  `json:"categoryId"`
	BrandID     *int32 `json:"brandId"`
	Name        string `json:"name"`
	Model       string `json:"model"`
	SKU         string `json:"sku"`
	Description string `json:"description"`
	ImageURL    string `json:"imageUrl"`
}

type CreatedProduct struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ImageURL string `json:"imageUrl"`
}

type ProductListItem struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Brand       string         `json:"brand"`
	Model       string         `json:"model"`
	Category    string         `json:"category"`
	Description string         `json:"description"`
	Rating      float64        `json:"rating"`
	ReviewCount int64          `json:"reviewCount"`
	Images      []ProductImage `json:"images"`
	Offers      []ProductOffer `json:"offers"`
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

	return ProductListItem{
		ID:          row.PublicID.String(),
		Name:        row.Name,
		Brand:       brand,
		Model:       model,
		Category:    row.CategoryName,
		Description: description,
		Rating:      rating,
		ReviewCount: row.ReviewCount,
		Images:      []ProductImage{},
		Offers:      []ProductOffer{},
	}
}
