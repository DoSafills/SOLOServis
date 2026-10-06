package dto

import (
	"github.com/DoSafills/SOLOServis/backend/internal/database/dbutil"
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
	return ProductCategory{
		ID:          category.ID,
		ParentID:    dbutil.Int4Ptr(category.ParentCategoryID),
		Name:        category.Name,
		Description: dbutil.Text(category.Description),
	}
}
