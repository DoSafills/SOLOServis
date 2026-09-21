package dto

import (
	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
)

type ProductDetail struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Brand       string         `json:"brand"`
	Model       string         `json:"model"`
	Category    string         `json:"category"`
	Description string         `json:"description"`
	Rating      float64        `json:"rating"`
	ReviewCount int64          `json:"reviewCount"`
	Images         []ProductImage         `json:"images"`
	Offers         []ProductOffer         `json:"offers"`
	Specifications []ProductSpecification `json:"specifications"`
}

type ProductSpecification struct {
	Name       string `json:"name"`
	Value      string `json:"value"`
	Unit       string `json:"unit"`
	DataType   string `json:"dataType"`
	Comparable bool   `json:"comparable"`
}

type ProductImage struct {
	URL       string `json:"url"`
	AltText   string `json:"altText"`
	SortOrder int32  `json:"sortOrder"`
}

type ProductOffer struct {
	StoreID      int32  `json:"storeId"`
	StoreName    string `json:"storeName"`
	Price        string `json:"price"`
	ListPrice    string `json:"listPrice"`
	Currency     string `json:"currency"`
	ShippingCost string `json:"shippingCost"`
	ShippingFree bool   `json:"shippingFree"`
	Available    bool   `json:"available"`
	Stock        *int32 `json:"stock"`
	Condition    string `json:"condition"`
	ProductURL   string `json:"productUrl"`
}

func FromProduct(product generated.GetProductDetailByPublicIDRow) ProductDetail {
	var model string
	if product.Model.Valid {
		model = product.Model.String
	}

	var description string
	if product.Description.Valid {
		description = product.Description.String
	}

	var brand string
	if product.BrandName.Valid {
		brand = product.BrandName.String
	}

	var rating float64
	if product.Rating.Valid {
		value, err := product.Rating.Float64Value()
		if err == nil && value.Valid {
			rating = value.Float64
		}
	}

	return ProductDetail{
		ID:          product.PublicID.String(),
		Name:        product.Name,
		Brand:       brand,
		Model:       model,
		Category:    product.CategoryName,
		Description: description,
		Rating:      rating,
		ReviewCount: product.ReviewCount,
		Images:         []ProductImage{},
		Offers:         []ProductOffer{},
		Specifications: []ProductSpecification{},
	}
}

func FromProductSpecification(spec generated.ListProductSpecificationsRow) ProductSpecification {
	var unit string
	if spec.Unit.Valid {
		unit = spec.Unit.String
	}

	return ProductSpecification{
		Name:       spec.Name,
		Value:      spec.Value,
		Unit:       unit,
		DataType:   spec.DataType,
		Comparable: spec.Comparable,
	}
}

func FromProductImage(image generated.ProductImage) ProductImage {
	var altText string
	if image.AltText.Valid {
		altText = image.AltText.String
	}

	return ProductImage{
		URL:       image.ImageUrl,
		AltText:   altText,
		SortOrder: image.SortOrder,
	}
}
