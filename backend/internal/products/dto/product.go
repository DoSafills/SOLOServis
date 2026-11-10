package dto

import (
	"encoding/json"

	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/jackc/pgx/v5/pgtype"
)

type ProductListItem struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Brand       string            `json:"brand"`
	Model       string            `json:"model"`
	CategoryID  int32             `json:"categoryId"`
	Category    string            `json:"category"`
	Subcategory string            `json:"subcategory"`
	Description string            `json:"description"`
	Rating      float64           `json:"rating"`
	ReviewCount int64             `json:"reviewCount"`
	Specs       map[string]string `json:"specs"`
	Images      []ProductImage    `json:"images"`
	Offers      []ProductOffer    `json:"offers"`
	// TAL-70: categoría del modelo nuevo (null si el producto no tiene
	// equivalente) y valores de atributos tipados por key.
	CategorySlug *string                    `json:"categorySlug"`
	Attributes   map[string]json.RawMessage `json:"attributes"`
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
		Specs:       make(map[string]string),
		Images:      []ProductImage{},
		Offers:      []ProductOffer{},
		Attributes:  make(map[string]json.RawMessage),
	}
}

// AttributeJSON convierte un valor guardado (forma canónica de
// categories.ValidateAttributes) en JSON tipado según su data_type:
// 350 → 350, "true" → true, "A+" → "A+".
func AttributeJSON(dataType, value string) json.RawMessage {
	switch dataType {
	case "integer", "decimal", "boolean":
		if json.Valid([]byte(value)) {
			return json.RawMessage(value)
		}
	}
	encoded, _ := json.Marshal(value)
	return encoded
}
