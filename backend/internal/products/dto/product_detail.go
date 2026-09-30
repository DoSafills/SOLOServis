package dto

import (
	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/DoSafills/SOLOServis/backend/internal/money"
)

// ProductDetail is the body of GET /products/{publicID}.
type ProductDetail struct {
	ID                string            `json:"id" format:"uuid" doc:"Public identifier of the product." example:"6f1c1f7e-0a1a-4a0a-9a0a-1a2b3c4d5e6f"`
	Name              string            `json:"name" doc:"Commercial name of the product." example:"Refrigerador Samsung No Frost"`
	Brand             string            `json:"brand" doc:"Brand name, or an empty string when the product has no brand." example:"Samsung"`
	Model             string            `json:"model" doc:"Manufacturer model reference, or an empty string." example:"RT38K"`
	Category          string            `json:"category" doc:"Name of the category the product belongs to." example:"Refrigeradores"`
	Description       string            `json:"description" doc:"Free-form description, or an empty string." example:"Refrigerador no frost de 380 litros"`
	Rating            float64           `json:"rating" minimum:"0" maximum:"5" doc:"Average rating, 0 when the product has no reviews." example:"4.5"`
	ReviewCount       int64             `json:"reviewCount" minimum:"0" doc:"Number of published reviews." example:"12"`
	Specs             map[string]string `json:"specs" doc:"Category specifications keyed by specification name."`
	Images            []ProductImage    `json:"images" doc:"Active images ordered by sortOrder."`
	Offers            []ProductOffer    `json:"offers" doc:"Offers ordered by ascending price."`
	PriceHistory      []PricePoint      `json:"priceHistory" doc:"Every recorded price, oldest first."`
	OfferPriceHistory []PricePoint      `json:"offerPriceHistory" doc:"Subset of priceHistory flagged as promotional."`
}

// PricePoint is one historical price of a product.
type PricePoint struct {
	Date  string      `json:"date" format:"date" doc:"Day the price was recorded, in ISO 8601." example:"2026-08-16"`
	Price money.Money `json:"price" doc:"Recorded price, serialized as a JSON number." example:"549990"`
}

// ProductImage is one image of a product.
type ProductImage struct {
	URL       string `json:"url" format:"uri" doc:"Absolute URL of the image." example:"https://cdn.example.com/products/1/main.jpg"`
	AltText   string `json:"altText" doc:"Alternative text, or an empty string." example:"Refrigerador Samsung frontal"`
	SortOrder int32  `json:"sortOrder" doc:"Position of the image within the product." example:"0"`
}

// ProductOffer is one store offer of a product. Monetary fields are JSON
// numbers, not strings, so clients can compare them numerically.
type ProductOffer struct {
	StoreID      int32        `json:"storeId" doc:"Internal identifier of the store." example:"1"`
	StoreName    string       `json:"storeName" doc:"Name of the store." example:"Falabella"`
	Price        money.Money  `json:"price" doc:"Current price, serialized as a JSON number." example:"549990"`
	ListPrice    *money.Money `json:"listPrice" nullable:"true" doc:"Reference price before discount, or null when absent."`
	Currency     string       `json:"currency" minLength:"3" maxLength:"3" doc:"ISO 4217 currency code." example:"CLP"`
	ShippingCost *money.Money `json:"shippingCost" nullable:"true" doc:"Shipping cost, or null when absent."`
	ShippingFree bool         `json:"shippingFree" doc:"Whether shipping is free of charge." example:"true"`
	Available    bool         `json:"available" doc:"Whether the offer can currently be purchased." example:"true"`
	Stock        *int32       `json:"stock" nullable:"true" doc:"Units in stock, or null when unknown." example:"15"`
	Condition    string       `json:"condition" enum:"new,used,refurbished" doc:"Condition of the offered unit." example:"new"`
	ProductURL   string       `json:"productUrl" doc:"Absolute URL of the offer, or an empty string." example:"https://falabella.com/producto/1"`
}

// BuildProductDetail assembles the detail body from the product row and its
// relations. The specification and price-history slices may be nil.
func BuildProductDetail(
	product generated.GetProductDetailByPublicIDRow,
	images []generated.ProductImage,
	offers []generated.ListProductOffersByProductIDsRow,
	specifications []generated.ListProductSpecificationsRow,
	priceHistory []generated.ListProductPriceHistoryRow,
) ProductDetail {
	result := FromProduct(product)

	relations := NewProductRelations(images, offers)
	result.Images = relations.Images(product.ID)
	result.Offers = relations.Offers(product.ID)

	result.Specs = make(map[string]string, len(specifications))
	for _, specification := range specifications {
		result.Specs[specification.Name] = specification.Value
	}

	result.PriceHistory = make([]PricePoint, 0, len(priceHistory))
	result.OfferPriceHistory = make([]PricePoint, 0)

	for _, point := range priceHistory {
		price := money.FromNumeric(point.Price)
		if !price.Valid() {
			continue
		}

		var date string
		if point.RecordedAt.Valid {
			date = point.RecordedAt.Time.Format("2006-01-02")
		}

		pricePoint := PricePoint{Date: date, Price: price}

		result.PriceHistory = append(result.PriceHistory, pricePoint)

		if point.IsPromotional {
			result.OfferPriceHistory = append(result.OfferPriceHistory, pricePoint)
		}
	}

	return result
}

// FromProduct maps a product row without its relations.
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
		if value, err := product.Rating.Float64Value(); err == nil && value.Valid {
			rating = value.Float64
		}
	}

	return ProductDetail{
		ID:                product.PublicID.String(),
		Name:              product.Name,
		Brand:             brand,
		Model:             model,
		Category:          product.CategoryName,
		Description:       description,
		Rating:            rating,
		ReviewCount:       product.ReviewCount,
		Specs:             map[string]string{},
		Images:            []ProductImage{},
		Offers:            []ProductOffer{},
		PriceHistory:      []PricePoint{},
		OfferPriceHistory: []PricePoint{},
	}
}

// FromProductImage maps an image row.
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
