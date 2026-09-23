package dto

import (
	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/jackc/pgx/v5/pgtype"
)

type ProductListItem struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Brand       string  `json:"brand"`
	Model       string  `json:"model"`
	CategoryID  int32   `json:"categoryId"`
	Category    string  `json:"category"`
	Subcategory string  `json:"subcategory"`
	Description string  `json:"description"`
	Rating      float64 `json:"rating"`
	ReviewCount int64   `json:"reviewCount"`
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
	}
}

// FromListProductByCategory mapea la fila del listado filtrado por categoría,
// que tiene las mismas columnas que ListProducts.
func FromListProductByCategory(row generated.ListProductsByCategoryRow) ProductListItem {
	return FromListProduct(generated.ListProductsRow{
		PublicID:           row.PublicID,
		Name:               row.Name,
		Model:              row.Model,
		Description:        row.Description,
		BrandName:          row.BrandName,
		CategoryID:         row.CategoryID,
		CategoryName:       row.CategoryName,
		ParentCategoryName: row.ParentCategoryName,
		Rating:             row.Rating,
		ReviewCount:        row.ReviewCount,
	})
}
