package dto

import (
	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
)

// ProductCategory es un nodo del árbol de categorías.
// ParentID en null identifica una categoría raíz; ProductCount solo viene
// cuando el cliente pide ?counts=true y suma las hojas de la rama.
type ProductCategory struct {
	ID           int32  `json:"id"`
	ParentID     *int32 `json:"parentId"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Slug         string `json:"slug"`
	Level        int32  `json:"level"`
	ProductCount *int64 `json:"productCount,omitempty"`
}

// FromProductCategory convierte una fila del árbol al DTO; counts puede ser
// nil y entonces no se incluye productCount.
func FromProductCategory(category generated.ListProductCategoriesRow, counts map[int32]int64) ProductCategory {
	var parentID *int32
	if category.ParentCategoryID.Valid {
		value := category.ParentCategoryID.Int32
		parentID = &value
	}

	var description string
	if category.Description.Valid {
		description = category.Description.String
	}

	var slug string
	if category.Slug.Valid {
		slug = category.Slug.String
	}

	var level int32
	if category.Level.Valid {
		level = int32(category.Level.Int16)
	}

	var productCount *int64
	if counts != nil {
		value := counts[category.ID]
		productCount = &value
	}

	return ProductCategory{
		ID:           category.ID,
		ParentID:     parentID,
		Name:         category.Name,
		Description:  description,
		Slug:         slug,
		Level:        level,
		ProductCount: productCount,
	}
}
