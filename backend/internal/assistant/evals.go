package assistant

import (
	"encoding/json"
	"fmt"

	"github.com/DoSafills/SOLOServis/backend/internal/comparison"
)

// EvalCase es un caso de assistant/evals/cases.json. El archivo es la fuente de
// verdad: se escribe a mano, se copia a assets/ con sync-assistant-assets.sh y
// los tests lo recorren desde aquí.
type EvalCase struct {
	// ID identifica el caso en los mensajes de fallo.
	ID string `json:"id"`
	// Question es lo que se le pregunta al asistente.
	Question string `json:"question"`
	// Expect es "answer" (debe responder sobre la comparación) o "refusal"
	// (debe usar la frase fija de rechazo).
	Expect string `json:"expect"`
	// MustIncludeAny exige al menos uno de estos fragmentos en la respuesta.
	MustIncludeAny []string `json:"mustIncludeAny"`
	// MustNotIncludeAny prohíbe todos estos fragmentos en la respuesta.
	MustNotIncludeAny []string `json:"mustNotIncludeAny"`
	// Rationale explica por qué el caso es correcto, para quien lo lea después.
	Rationale string `json:"rationale"`
}

// EvalSuite es el contenido completo de assistant/evals/cases.json.
type EvalSuite struct {
	Version int `json:"version"`
	// ContextFixture es la ruta, relativa a assistant/, del contexto de ejemplo
	// contra el que se evalúan los casos.
	ContextFixture string `json:"contextFixture"`
	// RefusalExpected debe coincidir con la frase de prompts/system.es.md.
	RefusalExpected string     `json:"refusalExpected"`
	Cases           []EvalCase `json:"cases"`
}

// EvalCases lee la suite de evaluación embebida. Devuelve el error de lectura o
// de decodificación si el archivo no está o no es JSON válido.
func EvalCases() (EvalSuite, error) {
	var suite EvalSuite
	if err := readJSON(evalsPath, &suite); err != nil {
		return EvalSuite{}, err
	}
	return suite, nil
}

// EvalFixture carga el contexto de ejemplo como matriz de comparación. Es el
// mismo tipo que arma la Comparison API desde PostgreSQL, así que los casos se
// evalúan contra la estructura real y no contra una copia.
func EvalFixture() (comparison.Matrix, error) {
	data, err := embedded.ReadFile(fixturePath)
	if err != nil {
		return comparison.Matrix{}, fmt.Errorf("leer %s: %w", fixturePath, err)
	}
	var matrix comparison.Matrix
	if err := json.Unmarshal(data, &matrix); err != nil {
		return comparison.Matrix{}, fmt.Errorf("decodificar %s: %w", fixturePath, err)
	}
	return matrix, nil
}
