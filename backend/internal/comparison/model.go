// Package comparison implementa la comparación de productos de Comparison API
// (TAL-76): compara alternativas de una misma categoría con sus atributos
// comparables (TAL-71), sus especificaciones anteriores y su mejor precio, y
// guarda comparaciones por usuario (con sesión). Las comparaciones de
// servicios siguen en internal/features.
package comparison

import (
	"encoding/json"
	"errors"
	"time"
)

// Errores de dominio que el handler traduce a HTTP (TAL-78 §3.5).
var (
	ErrProductNotFound      = errors.New("product not found")
	ErrComparisonNotFound   = errors.New("comparison not found")
	ErrItemNotFound         = errors.New("comparison item not found")
	ErrIncompatibleCategory = errors.New("product category differs from the comparison")
	ErrComparisonFull       = errors.New("comparison is full")
	ErrAlreadyIncluded      = errors.New("product already in comparison")
)

// Límites.
const (
	MinCompare    = 2   // productos en /compare
	MaxProducts   = 5   // productos por comparación (UI de columnas)
	MaxNameLength = 150 // product_comparison.name VARCHAR(150)
	DefaultName   = "Comparación de productos"
	DefaultLimit  = 20
	MaxLimit      = 100
)

// Product es un producto resuelto por su UUID público.
type Product struct {
	ID         int32
	PublicID   string
	CategoryID int32 // product.category_id: la misma categoría implica la misma category_ref
}

// ProductSummary es la columna de un producto en la comparación. BestPrice es
// el menor precio total (precio + envío) entre sus ofertas disponibles.
type ProductSummary struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Brand      string  `json:"brand"`
	Model      string  `json:"model"`
	ImageURL   *string `json:"imageUrl"`
	BestPrice  *string `json:"bestPrice"`
	BestStore  *string `json:"bestStore"`
	OfferCount int64   `json:"offerCount"`
}

// CategoryInfo identifica la categoría común. Slug es null si la categoría
// anterior no tiene equivalente en categories.
type CategoryInfo struct {
	Slug *string `json:"slug"`
	Name string  `json:"name"`
}

// AttributeRow es una fila de atributo comparable (TAL-71): un valor tipado
// por producto, en el mismo orden que Matrix.Products (null si no lo tiene).
type AttributeRow struct {
	Key      string            `json:"key"`
	Label    string            `json:"label"`
	DataType string            `json:"dataType"`
	Unit     *string           `json:"unit"`
	Values   []json.RawMessage `json:"values"`
	Differs  bool              `json:"differs"`
}

// SpecificationRow es una especificación anterior comparable
// (product_category_specification.comparable), como texto.
type SpecificationRow struct {
	Name    string    `json:"name"`
	Unit    *string   `json:"unit"`
	Values  []*string `json:"values"`
	Differs bool      `json:"differs"`
}

// Matrix es la comparación armada: productos en columnas y una fila por
// atributo o especificación con al menos un valor.
type Matrix struct {
	Category       *CategoryInfo      `json:"category"`
	Products       []ProductSummary   `json:"products"`
	Attributes     []AttributeRow     `json:"attributes"`
	Specifications []SpecificationRow `json:"specifications"`
}

// SavedComparison es una comparación guardada con su matriz.
type SavedComparison struct {
	ID        int32     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Matrix
}

// Summary es una comparación guardada en el listado del usuario.
type Summary struct {
	ID           int32     `json:"id"`
	Name         string    `json:"name"`
	Category     string    `json:"category"`
	ProductCount int64     `json:"productCount"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// Header son los datos propios de una comparación guardada.
type Header struct {
	ID         int32
	Name       string
	CategoryID int32
	CreatedAt  time.Time
	UpdatedAt  time.Time
	ProductIDs []int32 // en orden de posición
}

// Request bodies.
type CreateRequest struct {
	Name       string   `json:"name"`
	ProductIDs []string `json:"productIds"`
}

type RenameRequest struct {
	Name string `json:"name"`
}

type AddItemRequest struct {
	ProductID string `json:"productId"`
}
