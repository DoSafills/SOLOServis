package dto

import (
	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
)

type Category struct {
	ID               int32  `json:"id"`
	ParentCategoryID *int32 `json:"parentCategoryId"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	Active           bool   `json:"active"`
}

func FromCategory(category generated.ProductCategory) Category {
	var parentCategoryID *int32
	if category.ParentCategoryID.Valid {
		value := category.ParentCategoryID.Int32
		parentCategoryID = &value
	}

	var description string
	if category.Description.Valid {
		description = category.Description.String
	}

	return Category{
		ID:               category.ID,
		ParentCategoryID: parentCategoryID,
		Name:             category.Name,
		Description:      description,
		Active:           category.Active,
	}
}
