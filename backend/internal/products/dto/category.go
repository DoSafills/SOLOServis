package dto

import (
	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
)

// ProductCategory es un nodo del árbol de categorías.
// ParentID en null identifica una categoría raíz.
type ProductCategory struct {
	ID          int32  `json:"id"`
	ParentID    *int32 `json:"parentId"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func FromProductCategory(category generated.ListProductCategoriesRow) ProductCategory {
	var parentID *int32
	if category.ParentCategoryID.Valid {
		value := category.ParentCategoryID.Int32
		parentID = &value
	}

	var description string
	if category.Description.Valid {
		description = category.Description.String
	}

	return ProductCategory{
		ID:          category.ID,
		ParentID:    parentID,
		Name:        category.Name,
		Description: description,
	}
}
