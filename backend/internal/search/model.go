// Package search implementa la búsqueda pública de Search API (TAL-75):
// productos por texto, categoría, marca, precio y atributos filtrables,
// facetas para los filtros del frontend y búsqueda de categorías y marcas.
// El historial y las búsquedas guardadas (con sesión) siguen en features.
package search

import (
	"encoding/json"
	"errors"
)

// ErrCategoryNotFound indica que el slug de categoría no existe o está inactivo.
var ErrCategoryNotFound = errors.New("category not found")

// Límites de la búsqueda.
const (
	DefaultLimit   = 20
	MaxLimit       = 100
	MaxWords       = 5
	MaxQueryLength = 200
	MaxBrands      = 20
)

// Operadores de los filtros por atributo (ver attributeFilterSQL).
const (
	OpEqual            = "eq"  // texto exacto (enum, boolean)
	OpEqualInsensitive = "eqi" // texto sin distinguir mayúsculas (string)
	OpEqualNumeric     = "eqn" // igualdad numérica (integer, decimal)
	OpMin              = "min" // value_numeric >= valor
	OpMax              = "max" // value_numeric <= valor
)

// AttributeFilter es un filtro ya validado contra la definición del atributo.
type AttributeFilter struct {
	AttributeID int32
	Op          string
	Value       string
}

// Query son los filtros ya validados de /search/products y /search/facets.
type Query struct {
	Words            []string
	LegacyCategoryID *int32
	CategorySlug     *string
	BrandIDs         []int32
	MinPrice         *string
	MaxPrice         *string
	Available        bool
	Attributes       []AttributeFilter
	Sort             string
	Limit            int
	Offset           int
}

// ProductResult es un producto encontrado. BestPrice es el menor precio
// total (precio + envío) entre sus ofertas disponibles; null si no tiene.
type ProductResult struct {
	ID           string                     `json:"id"`
	Name         string                     `json:"name"`
	Brand        string                     `json:"brand"`
	Model        string                     `json:"model"`
	Category     string                     `json:"category"`
	CategorySlug *string                    `json:"categorySlug"`
	ImageURL     *string                    `json:"imageUrl"`
	BestPrice    *string                    `json:"bestPrice"`
	OfferCount   int64                      `json:"offerCount"`
	Attributes   map[string]json.RawMessage `json:"attributes"`
}

// Facets resume los productos que cumplen los filtros (TAL-57).
type Facets struct {
	Total      int64            `json:"total"`
	Price      PriceRange       `json:"price"`
	Brands     []BrandFacet     `json:"brands"`
	Attributes []AttributeFacet `json:"attributes"`
}

type PriceRange struct {
	Min *string `json:"min"`
	Max *string `json:"max"`
}

type BrandFacet struct {
	ID    int32  `json:"id"`
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// AttributeFacet es un atributo filtrable de la categoría: valores con conteo
// (enum, boolean, string) o mínimo y máximo (integer, decimal).
type AttributeFacet struct {
	Key      string       `json:"key"`
	Label    string       `json:"label"`
	DataType string       `json:"dataType"`
	Unit     *string      `json:"unit"`
	Values   []ValueCount `json:"values,omitempty"`
	Min      *float64     `json:"min,omitempty"`
	Max      *float64     `json:"max,omitempty"`
}

type ValueCount struct {
	Value json.RawMessage `json:"value"`
	Count int64           `json:"count"`
}

type CategoryResult struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	ItemType string `json:"itemType"`
}

type BrandResult struct {
	ID      int32   `json:"id"`
	Name    string  `json:"name"`
	LogoURL *string `json:"logoUrl"`
}
