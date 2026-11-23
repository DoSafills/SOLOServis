package assistant

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/DoSafills/SOLOServis/backend/internal/comparison"
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

// TestModelContextDropsUnusedFields comprueba que el contexto ya no lleve el
// UUID del producto ni la clave y el tipo de los atributos (TAL-181): el
// modelo no los usa y la allowlist los declara prohibidos.
func TestModelContextDropsUnusedFields(t *testing.T) {
	data, err := json.Marshal(buildModelContext(newFakeStore().matrix))
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	for _, banned := range []string{`"id"`, `"key"`, `"dataType"`, uuidA, "capacidad_litros"} {
		if strings.Contains(string(data), banned) {
			t.Errorf("el contexto incluye %s: %s", banned, data)
		}
	}
	if !strings.Contains(string(data), `"label":"Capacidad"`) {
		t.Errorf("el contexto perdió la etiqueta del atributo: %s", data)
	}
}

// TestModelContextDeduplicatesSpecifications recorre los casos del filtro de
// especificaciones repetidas: se descarta solo si coincide por nombre con un
// atributo y el atributo tiene el dato en cada producto donde la
// especificación lo tiene. Así no se pierde ningún valor.
func TestModelContextDeduplicatesSpecifications(t *testing.T) {
	capacity := comparison.AttributeRow{
		Key: "capacidad_litros", Label: "Capacidad", DataType: "integer", Unit: ptr("L"),
		Values: []json.RawMessage{json.RawMessage("380"), json.RawMessage("550")}, Differs: true,
	}
	partial := capacity
	partial.Values = []json.RawMessage{json.RawMessage("380"), json.RawMessage("null")}

	tests := []struct {
		name          string
		attribute     comparison.AttributeRow
		specification comparison.SpecificationRow
		wantKept      bool
	}{
		{
			name:          "mismo nombre que la etiqueta y el atributo cubre todo",
			attribute:     capacity,
			specification: comparison.SpecificationRow{Name: "Capacidad", Unit: ptr("litros"), Values: []*string{ptr("380"), ptr("550")}},
			wantKept:      false,
		},
		{
			name:          "coincide con la clave sin importar mayúsculas ni guiones bajos",
			attribute:     capacity,
			specification: comparison.SpecificationRow{Name: "Capacidad  Litros", Values: []*string{ptr("380"), nil}},
			wantKept:      false,
		},
		{
			name:          "el atributo no tiene el dato de un producto",
			attribute:     partial,
			specification: comparison.SpecificationRow{Name: "Capacidad", Values: []*string{ptr("380"), ptr("550")}},
			wantKept:      true,
		},
		{
			name:          "nombre distinto",
			attribute:     capacity,
			specification: comparison.SpecificationRow{Name: "Eficiencia energética", Values: []*string{ptr("A++"), ptr("A+")}},
			wantKept:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matrix := newFakeStore().matrix
			matrix.Attributes = []comparison.AttributeRow{tt.attribute}
			matrix.Specifications = []comparison.SpecificationRow{tt.specification}

			ctx := buildModelContext(matrix)
			if kept := len(ctx.Specifications) == 1; kept != tt.wantKept {
				t.Fatalf("especificación conservada = %v, quiero %v", kept, tt.wantKept)
			}
			if len(ctx.Attributes) != 1 {
				t.Fatalf("el atributo nunca se descarta; hay %d", len(ctx.Attributes))
			}
		})
	}
}

// TestEncodeModelContextTrims comprueba el tope de bytes: sin tope o con
// espacio de sobra no se quita nada; si no cabe, se van primero las filas que
// no distinguen productos y, después, las últimas. Productos y categoría se
// conservan siempre.
func TestEncodeModelContextTrims(t *testing.T) {
	matrix := newFakeStore().matrix
	matrix.Specifications = []comparison.SpecificationRow{
		{Name: "Eficiencia energética", Values: []*string{ptr("A++"), ptr("A+")}, Differs: true},
		{Name: "Dispensador de agua", Values: []*string{ptr("Sí"), ptr("Sí")}, Differs: false},
		{Name: "Color", Values: []*string{ptr("Gris"), ptr("Negro")}, Differs: true},
	}
	full, _, err := encodeModelContext(matrix, 0)
	if err != nil {
		t.Fatalf("encodeModelContext: %v", err)
	}
	withoutEqual := matrix
	withoutEqual.Specifications = []comparison.SpecificationRow{matrix.Specifications[0], matrix.Specifications[2]}
	noEqual, _, _ := encodeModelContext(withoutEqual, 0)

	tests := []struct {
		name        string
		maxBytes    int
		wantDropped int
		impossible  bool
		wantAbsent  []string
		wantPresent []string
	}{
		{name: "sin tope", maxBytes: 0, wantDropped: 0, wantPresent: []string{"Dispensador", "Color"}},
		{name: "cabe justo", maxBytes: len(full), wantDropped: 0, wantPresent: []string{"Dispensador", "Color"}},
		{
			name: "un byte de más quita la fila igual", maxBytes: len(full) - 1, wantDropped: 1,
			wantAbsent: []string{"Dispensador"}, wantPresent: []string{"Color", "Eficiencia", "Capacidad"},
		},
		{
			name: "después quita las últimas filas", maxBytes: len(noEqual) - 1, wantDropped: 2,
			wantAbsent: []string{"Dispensador", "Color"}, wantPresent: []string{"Eficiencia", "Capacidad"},
		},
		{
			name: "tope imposible deja productos y categoría", maxBytes: 10, wantDropped: 4, impossible: true,
			wantAbsent: []string{"Capacidad", "Eficiencia"}, wantPresent: []string{"Refrigeradores", "Samsung", "LG"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, dropped, err := encodeModelContext(matrix, tt.maxBytes)
			if err != nil {
				t.Fatalf("encodeModelContext: %v", err)
			}
			if dropped != tt.wantDropped {
				t.Errorf("filas quitadas = %d, quiero %d", dropped, tt.wantDropped)
			}
			if tt.maxBytes > 0 && !tt.impossible && len(data) > tt.maxBytes {
				t.Errorf("el contexto mide %d bytes, tope %d", len(data), tt.maxBytes)
			}
			for _, text := range tt.wantAbsent {
				if strings.Contains(string(data), text) {
					t.Errorf("debía quitar %q: %s", text, data)
				}
			}
			for _, text := range tt.wantPresent {
				if !strings.Contains(string(data), text) {
					t.Errorf("debía conservar %q: %s", text, data)
				}
			}
		})
	}
}

// TestModelContextFixtureSize fija el tamaño del contexto del fixture de las
// evaluaciones (TAL-181): sin duplicados, sin campos de más y bajo el tope de
// limits.json. Registra los bytes con t.Logf para comparar antes y después de
// un cambio sin llamar al proveedor.
func TestModelContextFixtureSize(t *testing.T) {
	matrix, err := EvalFixture()
	if err != nil {
		t.Fatalf("EvalFixture() error = %v", err)
	}
	cfg := testConfig(t)
	data, dropped, err := encodeModelContext(matrix, cfg.Limits.Context.MaxBytes)
	if err != nil {
		t.Fatalf("encodeModelContext: %v", err)
	}
	t.Logf("contexto del fixture: %d bytes, tope %d", len(data), cfg.Limits.Context.MaxBytes)
	if dropped != 0 {
		t.Fatalf("el fixture no debería recortarse; se quitaron %d filas", dropped)
	}
	// El fixture trae Capacidad y Eficiencia energética como atributo y como
	// especificación: deben llegar una sola vez.
	for _, name := range []string{"Capacidad", "Eficiencia energética"} {
		if count := strings.Count(string(data), `"`+name+`"`); count != 1 {
			t.Errorf("%q aparece %d veces en el contexto, quiero 1", name, count)
		}
	}
}

// TestAskLogsTrimmedContext comprueba que el servicio aplique
// limits.json context.maxBytes y avise con assistant_context_trimmed cuando
// recorta, y que no avise cuando el contexto cabe. El aviso importa porque el
// modelo responde con menos filas de las que ve el usuario.
func TestAskLogsTrimmedContext(t *testing.T) {
	tests := []struct {
		name     string
		maxBytes int
		wantLog  bool
	}{
		{name: "cabe en el tope", maxBytes: 8192, wantLog: false},
		{name: "se pasa del tope", maxBytes: 300, wantLog: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig(t)
			cfg.Limits.Context.MaxBytes = tt.maxBytes
			var logs bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logs, nil))
			provider := &fakeProvider{result: ProviderResult{Text: "El Refrigerador Samsung No Frost cuesta menos."}}
			service := NewService(provider, cfg, "prompt", nil, logger)

			matrix := newFakeStore().matrix
			if _, err := service.Ask(context.Background(), AskInput{
				ProductIDs: []string{uuidA, uuidB}, Question: "¿Cuál conviene?", Matrix: matrix, RateKey: tt.name,
			}); err != nil {
				t.Fatalf("Ask() error = %v", err)
			}
			if got := strings.Contains(logs.String(), "assistant_context_trimmed"); got != tt.wantLog {
				t.Fatalf("aviso de recorte = %v, quiero %v; logs: %s", got, tt.wantLog, logs.String())
			}
			if tt.wantLog && strings.Contains(provider.lastReq.UserContent, "Capacidad") {
				t.Fatalf("el contexto recortado todavía trae filas: %s", provider.lastReq.UserContent)
			}
		})
	}
}
