package dto

import "encoding/json"

// CreateProductRequest es el body de POST /products (TAL-70).
//
// La categoría se indica con categorySlug (modelo nuevo, TAL-71) o con
// categoryId (id de product_category, se mantiene para Ingestion API hasta
// TAL-77); exactamente uno de los dos. attributes se valida contra las
// definiciones de la categoría (categories.ValidateAttributes).
type CreateProductRequest struct {
	CategorySlug string                     `json:"categorySlug"`
	CategoryID   *int32                     `json:"categoryId"`
	BrandID      *int32                     `json:"brandId"`
	Name         string                     `json:"name"`
	Model        string                     `json:"model"`
	SKU          string                     `json:"sku"`
	Description  string                     `json:"description"`
	ImageURL     string                     `json:"imageUrl"`
	Attributes   map[string]json.RawMessage `json:"attributes"`
}

// UpdateProductRequest es el body de PUT /products/{publicID}: reemplaza el
// producto completo, atributos incluidos. Las imágenes no se modifican.
type UpdateProductRequest struct {
	CategorySlug string                     `json:"categorySlug"`
	CategoryID   *int32                     `json:"categoryId"`
	BrandID      *int32                     `json:"brandId"`
	Name         string                     `json:"name"`
	Model        string                     `json:"model"`
	SKU          string                     `json:"sku"`
	Description  string                     `json:"description"`
	Attributes   map[string]json.RawMessage `json:"attributes"`
}
