package assistant

import (
	"encoding/json"
	"strings"

	"github.com/DoSafills/SOLOServis/backend/internal/comparison"
)

// modelContext es la forma compacta de la matriz que recibe el modelo. Solo
// incluye los campos de assistant/context/allowed-fields.json: nada de
// imágenes, descripciones, URLs de tienda ni datos de usuario. Tampoco viajan
// el UUID del producto ni la clave y el tipo de cada atributo (TAL-181): el
// modelo no los usa y solo gastan tokens; las fuentes de la respuesta las pone
// el backend con los productIds de la petición.
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
	Label   string            `json:"label"`
	Unit    *string           `json:"unit"`
	Values  []json.RawMessage `json:"values"`
	Differs bool              `json:"differs"`
}

type modelSpecification struct {
	Name    string    `json:"name"`
	Unit    *string   `json:"unit"`
	Values  []*string `json:"values"`
	Differs bool      `json:"differs"`
}

// buildModelContext filtra la matriz a la allowlist y descarta las
// especificaciones que repiten un atributo (ver duplicatesAttribute). Recibe la
// matriz que arma la Comparison API y devuelve el contexto listo para
// serializar. El índice de cada valor corresponde al producto en la misma
// posición.
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
			Label:   attribute.Label,
			Unit:    attribute.Unit,
			Values:  attribute.Values,
			Differs: attribute.Differs,
		})
	}
	for _, specification := range matrix.Specifications {
		if duplicatesAttribute(specification, matrix.Attributes) {
			continue
		}
		ctx.Specifications = append(ctx.Specifications, modelSpecification{
			Name:    specification.Name,
			Unit:    specification.Unit,
			Values:  specification.Values,
			Differs: specification.Differs,
		})
	}
	return ctx
}

// duplicatesAttribute indica si una especificación anterior repite un atributo
// de TAL-71 que ya va en el contexto: mismo nombre (contra la etiqueta o la
// clave del atributo) y el atributo tiene valor en cada producto donde la
// especificación lo tiene. Recibe la especificación y los atributos de la
// matriz y devuelve true solo si descartarla no hace perder ningún dato. Existe
// porque la matriz trae las dos fuentes y, sin este filtro, el modelo recibe
// «Capacidad» dos veces y con unidades distintas («L» y «litros»).
func duplicatesAttribute(specification comparison.SpecificationRow, attributes []comparison.AttributeRow) bool {
	name := normalizeRowName(specification.Name)
	for _, attribute := range attributes {
		if normalizeRowName(attribute.Label) != name && normalizeRowName(attribute.Key) != name {
			continue
		}
		if coversSpecification(attribute, specification) {
			return true
		}
	}
	return false
}

// coversSpecification indica si el atributo tiene valor en todas las posiciones
// donde la especificación lo tiene. Recibe ambas filas, alineadas por producto,
// y devuelve false si algún producto quedaría sin el dato al descartar la
// especificación.
func coversSpecification(attribute comparison.AttributeRow, specification comparison.SpecificationRow) bool {
	for i, value := range specification.Values {
		if value == nil {
			continue
		}
		if i >= len(attribute.Values) || isJSONNull(attribute.Values[i]) {
			return false
		}
	}
	return true
}

// isJSONNull indica si un valor tipado de atributo está vacío. Recibe el JSON
// crudo y devuelve true para null o un valor ausente.
func isJSONNull(value json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(value))
	return trimmed == "" || trimmed == "null"
}

// normalizeRowName deja comparable el nombre de una fila: minúsculas, guiones
// bajos como espacios y espacios simples. Recibe la etiqueta, la clave o el
// nombre y devuelve su forma normalizada; así «capacidad_litros» y
// «Capacidad litros» comparan igual.
func normalizeRowName(name string) string {
	name = strings.ReplaceAll(strings.ToLower(name), "_", " ")
	return strings.Join(strings.Fields(name), " ")
}

// encodeModelContext arma el contexto y lo serializa respetando maxBytes.
// Recibe la matriz y el tope en bytes (0 o menos desactiva el tope). Devuelve
// el JSON, cuántas filas se quitaron para que cupiera y el error de
// serialización si lo hubo.
//
// Si el JSON se pasa del tope, quita primero las filas con differs en false
// (no distinguen a ningún producto) y después las demás, siempre desde el
// final: las especificaciones antes que los atributos y, dentro de cada grupo,
// las de menor display_order. Los productos y la categoría nunca se quitan,
// porque sin ellos no hay comparación; si aun así no cabe, se devuelve sin
// filas.
func encodeModelContext(matrix comparison.Matrix, maxBytes int) ([]byte, int, error) {
	ctx := buildModelContext(matrix)
	data, err := json.Marshal(ctx)
	if err != nil || maxBytes <= 0 {
		return data, 0, err
	}

	dropped := 0
	for _, onlyEqual := range []bool{true, false} {
		for len(data) > maxBytes && dropLastRow(&ctx, onlyEqual) {
			dropped++
			if data, err = json.Marshal(ctx); err != nil {
				return nil, dropped, err
			}
		}
	}
	return data, dropped, nil
}

// dropLastRow quita la última fila del contexto: primero busca entre las
// especificaciones y después entre los atributos. Recibe el contexto y, si
// onlyEqual es true, considera solo las filas con differs en false. Devuelve
// false si no quedaba ninguna fila que quitar.
func dropLastRow(ctx *modelContext, onlyEqual bool) bool {
	for i := len(ctx.Specifications) - 1; i >= 0; i-- {
		if !onlyEqual || !ctx.Specifications[i].Differs {
			ctx.Specifications = append(ctx.Specifications[:i], ctx.Specifications[i+1:]...)
			return true
		}
	}
	for i := len(ctx.Attributes) - 1; i >= 0; i-- {
		if !onlyEqual || !ctx.Attributes[i].Differs {
			ctx.Attributes = append(ctx.Attributes[:i], ctx.Attributes[i+1:]...)
			return true
		}
	}
	return false
}
