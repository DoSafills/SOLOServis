package dto

import (
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
		BrandID:     nullableInt4(r.BrandID),
		Name:        r.Name,
		Model:       nullableText(r.Model),
		Sku:         nullableText(r.SKU),
		Description: nullableText(r.Description),
	}
}

func (r UpdateProductRequest) ToParams(publicID pgtype.UUID) generated.UpdateProductByPublicIDParams {
	return generated.UpdateProductByPublicIDParams{
		PublicID:    publicID,
		CategoryID:  r.CategoryID,
		BrandID:     nullableInt4(r.BrandID),
		Name:        r.Name,
		Model:       nullableText(r.Model),
		Sku:         nullableText(r.SKU),
		Description: nullableText(r.Description),
	}
}

func nullableInt4(value *int32) pgtype.Int4 {
	if value == nil {
		return pgtype.Int4{}
	}

	return pgtype.Int4{
		Int32: *value,
		Valid: true,
	}
}

func nullableText(value string) pgtype.Text {
	if value == "" {
		return pgtype.Text{}
	}

	return pgtype.Text{
		String: value,
		Valid:  true,
	}
}