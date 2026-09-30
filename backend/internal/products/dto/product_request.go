package dto

import (
	"strings"
)

// Field limits mirror the column definitions in
// database/migrations/001_schema.sql so that an oversized payload is rejected
// with a 422 before it reaches PostgreSQL, which would otherwise surface as an
// opaque 500 from a value-too-long error.
const (
	// product.name is VARCHAR(255) NOT NULL.
	MaxNameLength = 255
	// product.model is VARCHAR(150).
	MaxModelLength = 150
	// product.sku is VARCHAR(100) UNIQUE.
	MaxSKULength = 100
	// product.description is TEXT; bounded to keep payloads reviewable.
	MaxDescriptionLength = 5000
	// product_image.image_url is TEXT NOT NULL.
	MaxImageURLLength = 2048
)

// CreateProductRequest is the payload of POST /products.
type CreateProductRequest struct {
	CategoryID  int32  `json:"categoryId" minimum:"1" doc:"Identifier of the category the product belongs to." example:"1"`
	BrandID     *int32 `json:"brandId,omitempty" minimum:"1" doc:"Identifier of the brand. Omit when the product has no brand." example:"1"`
	Name        string `json:"name" minLength:"1" maxLength:"255" doc:"Commercial name of the product." example:"Refrigerador Samsung No Frost"`
	Model       string `json:"model,omitempty" maxLength:"150" doc:"Manufacturer model reference." example:"RT38K"`
	SKU         string `json:"sku,omitempty" maxLength:"100" doc:"Stock keeping unit. Must be unique when present." example:"SKU-SAMS-001"`
	Description string `json:"description,omitempty" maxLength:"5000" doc:"Free-form description of the product." example:"Refrigerador no frost de 380 litros"`
	ImageURL    string `json:"imageUrl" minLength:"1" maxLength:"2048" format:"uri" doc:"Absolute http(s) URL of the main product image." example:"https://cdn.example.com/products/1/main.jpg"`
}

// UpdateProductRequest is the payload of PUT /products/{publicID}. The product
// is identified by the path, so the body carries only editable fields.
type UpdateProductRequest struct {
	CategoryID  int32  `json:"categoryId" minimum:"1" doc:"Identifier of the category the product belongs to." example:"1"`
	BrandID     *int32 `json:"brandId,omitempty" minimum:"1" doc:"Identifier of the brand. Omit to clear it." example:"1"`
	Name        string `json:"name" minLength:"1" maxLength:"255" doc:"Commercial name of the product." example:"Refrigerador Samsung No Frost"`
	Model       string `json:"model,omitempty" maxLength:"150" doc:"Manufacturer model reference." example:"RT38K"`
	SKU         string `json:"sku,omitempty" maxLength:"100" doc:"Stock keeping unit. Must be unique when present." example:"SKU-SAMS-001"`
	Description string `json:"description,omitempty" maxLength:"5000" doc:"Free-form description of the product." example:"Refrigerador no frost de 380 litros"`
}

// Normalize trims the free-text fields so that a name of only whitespace is
// treated as absent rather than stored as a blank label.
func (r *CreateProductRequest) Normalize() {
	r.Name = strings.TrimSpace(r.Name)
	r.Model = strings.TrimSpace(r.Model)
	r.SKU = strings.TrimSpace(r.SKU)
	r.Description = strings.TrimSpace(r.Description)
	r.ImageURL = strings.TrimSpace(r.ImageURL)
}

// Normalize trims the free-text fields of an update payload.
func (r *UpdateProductRequest) Normalize() {
	r.Name = strings.TrimSpace(r.Name)
	r.Model = strings.TrimSpace(r.Model)
	r.SKU = strings.TrimSpace(r.SKU)
	r.Description = strings.TrimSpace(r.Description)
}
