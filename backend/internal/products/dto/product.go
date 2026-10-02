package dto

import (
	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/jackc/pgx/v5/pgtype"
)

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

// splitCategory devuelve (categoría, subcategoría). Si la categoría del producto
// tiene padre, el padre es la categoría y la del producto es la subcategoría.
func splitCategory(categoryName string, parentName pgtype.Text) (string, string) {
	if parentName.Valid && parentName.String != "" {
		return parentName.String, categoryName
	}
	return categoryName, ""
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

	category, subcategory := splitCategory(row.CategoryName, row.ParentCategoryName)

	return ProductListItem{
		ID:          row.PublicID.String(),
		Name:        row.Name,
		Brand:       brand,
		Model:       model,
		CategoryID:  row.CategoryID,
		Category:    category,
		Subcategory: subcategory,
		Description: description,
		Rating:      rating,
		ReviewCount: row.ReviewCount,
		Images:      []ProductImage{},
		Offers:      []ProductOffer{},
	}
}
