package dto

import (
	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
)

// CreatedProduct is the body returned when a product is created or
// deactivated.
type CreatedProduct struct {
	ID       string `json:"id" format:"uuid" doc:"Public identifier of the product." example:"6f1c1f7e-0a1a-4a0a-9a0a-1a2b3c4d5e6f"`
	Name     string `json:"name" doc:"Commercial name of the product." example:"Refrigerador Samsung No Frost"`
	ImageURL string `json:"imageUrl" doc:"Absolute URL of the main product image." example:"https://cdn.example.com/products/1/main.jpg"`
}

// ProductListItem is one element of the GET /products response.
type ProductListItem struct {
	ID          string         `json:"id" format:"uuid" doc:"Public identifier of the product." example:"6f1c1f7e-0a1a-4a0a-9a0a-1a2b3c4d5e6f"`
	Name        string         `json:"name" doc:"Commercial name of the product." example:"Refrigerador Samsung No Frost"`
	Brand       string         `json:"brand" doc:"Brand name, or an empty string when the product has no brand." example:"Samsung"`
	Model       string         `json:"model" doc:"Manufacturer model reference, or an empty string." example:"RT38K"`
	Category    string         `json:"category" doc:"Name of the category the product belongs to." example:"Refrigeradores"`
	Description string         `json:"description" doc:"Free-form description, or an empty string." example:"Refrigerador no frost de 380 litros"`
	Rating      float64        `json:"rating" minimum:"0" maximum:"5" doc:"Average rating, 0 when the product has no reviews." example:"4.5"`
	ReviewCount int64          `json:"reviewCount" minimum:"0" doc:"Number of published reviews." example:"12"`
	Images      []ProductImage `json:"images" doc:"Active images ordered by sortOrder."`
	Offers      []ProductOffer `json:"offers" doc:"Offers ordered by ascending price."`
}

// ProductRelations holds the child rows of several products, fetched with one
// query per relation instead of one query per product.
type ProductRelations struct {
	images map[int32][]ProductImage
	offers map[int32][]ProductOffer
}

// NewProductRelations groups images and offers by their product identifier.
// Rows for products that are not in the listing are ignored.
func NewProductRelations(
	images []generated.ProductImage,
	offers []generated.ListProductOffersByProductIDsRow,
) ProductRelations {
	relations := ProductRelations{
		images: make(map[int32][]ProductImage),
		offers: make(map[int32][]ProductOffer),
	}

	for _, image := range images {
		relations.images[image.ProductID] = append(relations.images[image.ProductID], FromProductImage(image))
	}

	for _, offer := range offers {
		relations.offers[offer.ProductID] = append(relations.offers[offer.ProductID], FromProductOffer(offer))
	}

	return relations
}

// Images returns the images of a product, never nil so that the JSON contains
// an empty array instead of null.
func (r ProductRelations) Images(productID int32) []ProductImage {
	images := r.images[productID]
	if images == nil {
		return []ProductImage{}
	}

	return images
}

// Offers returns the offers of a product, never nil so that the JSON contains
// an empty array instead of null.
func (r ProductRelations) Offers(productID int32) []ProductOffer {
	offers := r.offers[productID]
	if offers == nil {
		return []ProductOffer{}
	}

	return offers
}

// ListProducts maps listing rows into the API representation, attaching the
// already-fetched relations. The result is never nil.
func ListProducts(rows []generated.ListProductsFilteredRow, relations ProductRelations) []ProductListItem {
	items := make([]ProductListItem, 0, len(rows))

	for _, row := range rows {
		item := FromListProduct(row)
		item.Images = relations.Images(row.ID)
		item.Offers = relations.Offers(row.ID)

		items = append(items, item)
	}

	return items
}

// FromListProduct maps a single listing row.
func FromListProduct(row generated.ListProductsFilteredRow) ProductListItem {
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
		if value, err := row.Rating.Float64Value(); err == nil && value.Valid {
			rating = value.Float64
		}
	}

	return ProductListItem{
		ID:          row.PublicID.String(),
		Name:        row.Name,
		Brand:       brand,
		Model:       model,
		Category:    row.CategoryName,
		Description: description,
		Rating:      rating,
		ReviewCount: row.ReviewCount,
		Images:      []ProductImage{},
		Offers:      []ProductOffer{},
	}
}
