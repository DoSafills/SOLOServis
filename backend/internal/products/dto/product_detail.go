package dto

import (
	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/jackc/pgx/v5/pgtype"
)

type ProductDetail struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Brand             string            `json:"brand"`
	Model             string            `json:"model"`
	CategoryID        int32             `json:"categoryId"`
	Category          string            `json:"category"`
	Subcategory       string            `json:"subcategory"`
	Description       string            `json:"description"`
	Rating            float64           `json:"rating"`
	ReviewCount       int64             `json:"reviewCount"`
	Specs             map[string]string `json:"specs"`
	Images            []ProductImage    `json:"images"`
	Offers            []ProductOffer    `json:"offers"`
	PriceHistory      []PricePoint      `json:"priceHistory"`
	OfferPriceHistory []PricePoint      `json:"offerPriceHistory"`
}

type PricePoint struct {
	Date  string `json:"date"`
	Price string `json:"price"`
}

type ProductImage struct {
	URL       string `json:"url"`
	AltText   string `json:"altText"`
	SortOrder int32  `json:"sortOrder"`
}

type ProductOffer struct {
	OfferID      int32  `json:"offerId"`
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

func splitCategory(categoryName string, parentCategoryName pgtype.Text) (string, string) {
	if parentCategoryName.Valid && parentCategoryName.String != "" {
		return parentCategoryName.String, categoryName
	}
	return categoryName, ""
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

	category, subcategory := splitCategory(product.CategoryName, product.ParentCategoryName)

	return ProductDetail{
		ID:                product.PublicID.String(),
		Name:              product.Name,
		Brand:             brand,
		Model:             model,
		CategoryID:        product.CategoryID,
		Category:          category,
		Subcategory:       subcategory,
		Description:       description,
		Rating:            rating,
		ReviewCount:       product.ReviewCount,
		Specs:             make(map[string]string),
		Images:            []ProductImage{},
		Offers:            []ProductOffer{},
		PriceHistory:      []PricePoint{},
		OfferPriceHistory: []PricePoint{},
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
