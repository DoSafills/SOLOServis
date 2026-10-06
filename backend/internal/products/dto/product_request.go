package dto

import (
	"github.com/DoSafills/SOLOServis/backend/internal/database/dbutil"
	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/jackc/pgx/v5/pgtype"
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

type UpdateProductRequest struct {
	CategoryID  int32  `json:"categoryId"`
	BrandID     *int32 `json:"brandId"`
	Name        string `json:"name"`
	Model       string `json:"model"`
	SKU         string `json:"sku"`
	Description string `json:"description"`
}

func (r CreateProductRequest) ToParams() generated.CreateProductParams {
	return generated.CreateProductParams{
		CategoryID:  r.CategoryID,
		BrandID:     dbutil.NullableInt4(r.BrandID),
		Name:        r.Name,
		Model:       dbutil.NullableText(r.Model),
		Sku:         dbutil.NullableText(r.SKU),
		Description: dbutil.NullableText(r.Description),
	}
}

func (r UpdateProductRequest) ToParams(publicID pgtype.UUID) generated.UpdateProductByPublicIDParams {
	return generated.UpdateProductByPublicIDParams{
		PublicID:    publicID,
		CategoryID:  r.CategoryID,
		BrandID:     dbutil.NullableInt4(r.BrandID),
		Name:        r.Name,
		Model:       dbutil.NullableText(r.Model),
		Sku:         dbutil.NullableText(r.SKU),
		Description: dbutil.NullableText(r.Description),
	}
}
