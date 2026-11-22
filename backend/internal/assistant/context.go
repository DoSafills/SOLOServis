package assistant

import (
	"encoding/json"

	"github.com/DoSafills/SOLOServis/backend/internal/comparison"
)

// modelContext es la forma compacta de la matriz que recibe el modelo. Solo
// incluye los campos de assistant/context/allowed-fields.json: nada de
// imágenes, descripciones, URLs de tienda ni datos de usuario.
type modelContext struct {
	Category       modelCategory        `json:"category"`
	Products       []modelProduct       `json:"products"`
	Attributes     []modelAttribute     `json:"attributes"`
	Specifications []modelSpecification `json:"specifications"`
}

type modelCategory struct {
	Name string `json:"name"`
}

type modelProduct struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Brand      string  `json:"brand"`
	Model      string  `json:"model"`
	BestPrice  *string `json:"bestPrice"`
	BestStore  *string `json:"bestStore"`
	OfferCount int64   `json:"offerCount"`
	// LastUpdated permite responder «¿este precio es de hoy?» con el dato real
	// de la oferta, no con una suposición (TAL-223).
	LastUpdated *string `json:"lastUpdated"`
}

type modelAttribute struct {
	Key      string            `json:"key"`
	Label    string            `json:"label"`
	DataType string            `json:"dataType"`
	Unit     *string           `json:"unit"`
	Values   []json.RawMessage `json:"values"`
	Differs  bool              `json:"differs"`
}

type modelSpecification struct {
	Name    string    `json:"name"`
	Unit    *string   `json:"unit"`
	Values  []*string `json:"values"`
	Differs bool      `json:"differs"`
}

// buildModelContext filtra la matriz a la allowlist. El índice de cada valor
// corresponde al producto en la misma posición.
func buildModelContext(matrix comparison.Matrix) modelContext {
	ctx := modelContext{
		Products:       make([]modelProduct, 0, len(matrix.Products)),
		Attributes:     make([]modelAttribute, 0, len(matrix.Attributes)),
		Specifications: make([]modelSpecification, 0, len(matrix.Specifications)),
	}
	if matrix.Category != nil {
		ctx.Category.Name = matrix.Category.Name
	}
	for _, product := range matrix.Products {
		ctx.Products = append(ctx.Products, modelProduct{
			ID:          product.ID,
			Name:        product.Name,
			Brand:       product.Brand,
			Model:       product.Model,
			BestPrice:   product.BestPrice,
			BestStore:   product.BestStore,
			OfferCount:  product.OfferCount,
			LastUpdated: product.LastUpdated,
		})
	}
	for _, attribute := range matrix.Attributes {
		ctx.Attributes = append(ctx.Attributes, modelAttribute{
			Key:      attribute.Key,
			Label:    attribute.Label,
			DataType: attribute.DataType,
			Unit:     attribute.Unit,
			Values:   attribute.Values,
			Differs:  attribute.Differs,
		})
	}
	for _, specification := range matrix.Specifications {
		ctx.Specifications = append(ctx.Specifications, modelSpecification{
			Name:    specification.Name,
			Unit:    specification.Unit,
			Values:  specification.Values,
			Differs: specification.Differs,
		})
	}
	return ctx
}
