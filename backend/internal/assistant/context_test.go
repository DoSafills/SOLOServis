package assistant

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestModelContextCarriesLastUpdated comprueba que la fecha de verificación de
// la mejor oferta llegue al contexto del modelo (TAL-223). Sin ella, el
// asistente no puede decir si un precio es de hoy; con ella, sí, y sin salirse
// de la allowlist.
func TestModelContextCarriesLastUpdated(t *testing.T) {
	fresh := "2026-11-22T15:04:05Z"
	matrix := newFakeStore().matrix
	matrix.Products[0].LastUpdated = &fresh

	data, err := json.Marshal(buildModelContext(matrix))
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	if !strings.Contains(string(data), `"lastUpdated":"`+fresh+`"`) {
		t.Fatalf("el contexto no lleva lastUpdated: %s", data)
	}
}

// TestModelContextOmitsMissingLastUpdated verifica el caso contrario: si el
// producto no tiene ofertas, el campo viaja en null y el modelo no puede
// inventar una fecha. Es la mitad que evita un «actualizado hoy» falso.
func TestModelContextOmitsMissingLastUpdated(t *testing.T) {
	matrix := newFakeStore().matrix
	for i := range matrix.Products {
		matrix.Products[i].LastUpdated = nil
	}

	data, err := json.Marshal(buildModelContext(matrix))
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	if !strings.Contains(string(data), `"lastUpdated":null`) {
		t.Fatalf("sin oferta, lastUpdated debe viajar en null: %s", data)
	}
}
