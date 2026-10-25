// Package categories expone las categorías unificadas (producto/servicio)
// y sus definiciones de atributos dinámicos (TAL-71). Solo lectura.
package categories

// Tipos de ítem permitidos (CHECK de categories.item_type).
const (
	ItemTypeProduct = "product"
	ItemTypeService = "service"
)

// CategorySummary es un elemento del listado de categorías.
type CategorySummary struct {
	Slug           string  `json:"slug"`
	Name           string  `json:"name"`
	ItemType       string  `json:"itemType"`
	Description    *string `json:"description"`
	AttributeCount int64   `json:"attributeCount"`
}

// CategoryPage es la respuesta paginada de GET /categories.
type CategoryPage struct {
	Items  []CategorySummary `json:"items"`
	Total  int64             `json:"total"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}

// CategoryDetail es la respuesta de GET /categories/{slug}.
type CategoryDetail struct {
	Slug        string      `json:"slug"`
	Name        string      `json:"name"`
	ItemType    string      `json:"itemType"`
	Description *string     `json:"description"`
	Attributes  []Attribute `json:"attributes"`
}

// Attribute es la definición de un atributo dinámico. Las restricciones que
// no aplican al data_type se serializan como null para un contrato estable.
type Attribute struct {
	Key           string   `json:"key"`
	Label         string   `json:"label"`
	DataType      string   `json:"dataType"`
	Unit          *string  `json:"unit"`
	Required      bool     `json:"required"`
	Comparable    bool     `json:"comparable"`
	Filterable    bool     `json:"filterable"`
	Min           *float64 `json:"min"`
	Max           *float64 `json:"max"`
	MaxLength     *int32   `json:"maxLength"`
	AllowedValues []string `json:"allowedValues"`
	DisplayOrder  int32    `json:"displayOrder"`
}

// ListParams son los parámetros ya validados del listado.
type ListParams struct {
	ItemType *string
	Limit    int
	Offset   int
}
