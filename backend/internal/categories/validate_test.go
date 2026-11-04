package categories

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
)

func float(f float64) *float64 { return &f }
func int32p(i int32) *int32    { return &i }

// testDefs replica la forma de los seeds: un atributo de cada data_type.
// Van desordenadas a propósito: el validador ordena por DisplayOrder.
var testDefs = []Attribute{
	{ID: 13, Key: "color", DataType: "string", MaxLength: int32p(10), DisplayOrder: 5},
	{ID: 10, Key: "capacidad_litros", DataType: "integer", Unit: strPtr("L"), Required: true,
		Min: float(50), Max: float(1500), DisplayOrder: 1},
	{ID: 11, Key: "eficiencia_energetica", DataType: "enum",
		AllowedValues: []string{"A+++", "A++", "A+", "A"}, DisplayOrder: 2},
	{ID: 12, Key: "no_frost", DataType: "boolean", DisplayOrder: 3},
	{ID: 14, Key: "peso_kg", DataType: "decimal", Unit: strPtr("kg"), Min: float(0), Max: float(300), DisplayOrder: 4},
	{ID: 15, Key: "modelo", DataType: "string", DisplayOrder: 6},
}

func decodeInput(t *testing.T, body string) map[string]json.RawMessage {
	t.Helper()
	var input map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &input); err != nil {
		t.Fatalf("input de test inválido: %v", err)
	}
	return input
}

func TestValidateAttributes_ValoresValidos(t *testing.T) {
	input := decodeInput(t, `{
		"capacidad_litros": 350,
		"eficiencia_energetica": " A+ ",
		"no_frost": false,
		"peso_kg": 65.50,
		"color": "  Gris  "
	}`)

	values, errs := ValidateAttributes(testDefs, input)

	if len(errs) != 0 {
		t.Fatalf("errores inesperados: %+v", errs)
	}
	want := []Value{
		{AttributeID: 10, Key: "capacidad_litros", DataType: "integer", Text: "350"},
		{AttributeID: 11, Key: "eficiencia_energetica", DataType: "enum", Text: "A+"},
		{AttributeID: 12, Key: "no_frost", DataType: "boolean", Text: "false"},
		{AttributeID: 14, Key: "peso_kg", DataType: "decimal", Text: "65.5"},
		{AttributeID: 13, Key: "color", DataType: "string", Text: "Gris"},
	}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("values =\n%+v\nquiero\n%+v", values, want)
	}
}

func TestValidateAttributes_FormaCanonica(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		want  string
	}{
		{"entero simple", "capacidad_litros", `350`, "350"},
		{"entero con .0", "capacidad_litros", `350.0`, "350"},
		{"entero con exponente", "capacidad_litros", `3.5e2`, "350"},
		{"límite mínimo inclusivo", "capacidad_litros", `50`, "50"},
		{"límite máximo inclusivo", "capacidad_litros", `1500`, "1500"},
		{"decimal sin ceros finales", "peso_kg", `65.50`, "65.5"},
		{"decimal entero", "peso_kg", `65`, "65"},
		{"decimal cero", "peso_kg", `0`, "0"},
		{"decimal con exponente", "peso_kg", `1.25e2`, "125"},
		{"booleano true", "no_frost", `true`, "true"},
		{"enum exacto", "eficiencia_energetica", `"A+++"`, "A+++"},
		{"texto con acentos en el límite", "color", `"ñandú azul"`, "ñandú azul"},
		{"texto sin max_length usa 255", "modelo", `"` + strings.Repeat("x", 255) + `"`, strings.Repeat("x", 255)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := map[string]json.RawMessage{
				"capacidad_litros": json.RawMessage(`100`), // requerido
				tc.key:             json.RawMessage(tc.value),
			}
			values, errs := ValidateAttributes(testDefs, input)
			if len(errs) != 0 {
				t.Fatalf("errores inesperados: %+v", errs)
			}
			for _, v := range values {
				if v.Key == tc.key {
					if v.Text != tc.want {
						t.Fatalf("Text = %q, quiero %q", v.Text, tc.want)
					}
					return
				}
			}
			t.Fatalf("no se devolvió el valor de %s: %+v", tc.key, values)
		})
	}
}

func TestValidateAttributes_ValoresInvalidos(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		wantCode string
	}{
		{"entero como string", "capacidad_litros", `"350"`, "invalid_type"},
		{"entero con decimales", "capacidad_litros", `350.5`, "invalid_type"},
		{"entero booleano", "capacidad_litros", `true`, "invalid_type"},
		{"entero fuera de float64", "capacidad_litros", `1e999999999`, "invalid_type"},
		{"entero sobre 2^53", "capacidad_litros", `9007199254740993`, "invalid_type"},
		{"entero bajo el mínimo", "capacidad_litros", `49`, "min"},
		{"entero sobre el máximo", "capacidad_litros", `1501`, "max"},
		// Costo acotado: ParseFloat lo lleva a 0 sin calcular 10^999999999.
		{"exponente negativo enorme no se cuelga", "capacidad_litros", `1e-999999999`, "min"},
		{"decimal como string", "peso_kg", `"65.5"`, "invalid_type"},
		{"decimal objeto", "peso_kg", `{"valor":1}`, "invalid_type"},
		{"decimal negativo", "peso_kg", `-0.1`, "min"},
		{"decimal sobre el máximo", "peso_kg", `300.01`, "max"},
		{"booleano como string", "no_frost", `"true"`, "invalid_type"},
		{"booleano como número", "no_frost", `1`, "invalid_type"},
		{"enum fuera de la lista", "eficiencia_energetica", `"B"`, "one_of"},
		{"enum distingue mayúsculas", "eficiencia_energetica", `"a+"`, "one_of"},
		{"enum como número", "eficiencia_energetica", `1`, "invalid_type"},
		{"enum como arreglo", "eficiencia_energetica", `["A+"]`, "invalid_type"},
		{"texto como número", "color", `5`, "invalid_type"},
		{"texto sobre max_length", "color", `"ñandú azul!"`, "max_length"},
		{"texto sobre 255 sin max_length", "modelo", `"` + strings.Repeat("x", 256) + `"`, "max_length"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := map[string]json.RawMessage{"capacidad_litros": json.RawMessage(`100`)}
			input[tc.key] = json.RawMessage(tc.value)

			values, errs := ValidateAttributes(testDefs, input)

			want := []httpx.FieldError{{Field: "attributes." + tc.key, Code: tc.wantCode}}
			if len(errs) != 1 || errs[0].Field != want[0].Field || errs[0].Code != want[0].Code {
				t.Fatalf("errores = %+v, quiero %s/%s", errs, want[0].Field, want[0].Code)
			}
			if errs[0].Message == "" {
				t.Fatal("el error debe tener mensaje")
			}
			for _, v := range values {
				if v.Key == tc.key {
					t.Fatalf("un valor inválido no debe devolverse: %+v", v)
				}
			}
		})
	}
}

func TestValidateAttributes_Requeridos(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"ausente", `{}`},
		{"null", `{"capacidad_litros": null}`},
		{"input nil", ``},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var input map[string]json.RawMessage
			if tc.input != "" {
				input = decodeInput(t, tc.input)
			}
			values, errs := ValidateAttributes(testDefs, input)
			if len(errs) != 1 || errs[0].Field != "attributes.capacidad_litros" || errs[0].Code != "required" {
				t.Fatalf("errores = %+v", errs)
			}
			if len(values) != 0 {
				t.Fatalf("values = %+v, quiero ninguno", values)
			}
		})
	}
}

func TestValidateAttributes_TextoVacioEnRequerido(t *testing.T) {
	defs := []Attribute{{ID: 1, Key: "procesador", DataType: "string", Required: true}}

	_, errs := ValidateAttributes(defs, decodeInput(t, `{"procesador": "   "}`))

	if len(errs) != 1 || errs[0].Code != "required" {
		t.Fatalf("errores = %+v, quiero required", errs)
	}
}

func TestValidateAttributes_OpcionalesNullOVaciosSeOmiten(t *testing.T) {
	input := decodeInput(t, `{"capacidad_litros": 100, "no_frost": null, "color": "  ", "eficiencia_energetica": ""}`)

	values, errs := ValidateAttributes(testDefs, input)

	if len(errs) != 0 {
		t.Fatalf("errores inesperados: %+v", errs)
	}
	if len(values) != 1 || values[0].Key != "capacidad_litros" {
		t.Fatalf("values = %+v, quiero solo capacidad_litros", values)
	}
}

func TestValidateAttributes_KeysDesconocidas(t *testing.T) {
	input := decodeInput(t, `{"capacidad_litros": 100, "zeta": 1, "Capacidad": 2, "alfa": "x"}`)

	_, errs := ValidateAttributes(testDefs, input)

	var fields []string
	for _, e := range errs {
		if e.Code != "unknown_field" {
			t.Fatalf("code = %q, quiero unknown_field", e.Code)
		}
		fields = append(fields, e.Field)
	}
	want := []string{"attributes.Capacidad", "attributes.alfa", "attributes.zeta"}
	if !reflect.DeepEqual(fields, want) {
		t.Fatalf("fields = %v, quiero %v (orden alfabético)", fields, want)
	}
}

func TestValidateAttributes_AcumulaErroresEnOrdenDeterminista(t *testing.T) {
	input := decodeInput(t, `{
		"desconocida": 1,
		"color": 7,
		"peso_kg": 999,
		"eficiencia_energetica": "Z"
	}`)

	for i := 0; i < 20; i++ { // el orden de un map en Go es aleatorio
		_, errs := ValidateAttributes(testDefs, input)
		var got []string
		for _, e := range errs {
			got = append(got, e.Field+"/"+e.Code)
		}
		want := []string{
			"attributes.capacidad_litros/required",
			"attributes.eficiencia_energetica/one_of",
			"attributes.peso_kg/max",
			"attributes.color/invalid_type",
			"attributes.desconocida/unknown_field",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("errores = %v, quiero %v", got, want)
		}
	}
}

func TestValidateAttributes_NoModificaLasDefiniciones(t *testing.T) {
	defs := slicesCloneAttributes(testDefs)

	ValidateAttributes(defs, decodeInput(t, `{"capacidad_litros": 100}`))

	if !reflect.DeepEqual(defs, testDefs) {
		t.Fatal("ValidateAttributes no debe reordenar el slice recibido")
	}
}

func TestValidateAttributes_MensajesLegibles(t *testing.T) {
	input := decodeInput(t, `{"capacidad_litros": 10, "eficiencia_energetica": "Z"}`)

	_, errs := ValidateAttributes(testDefs, input)

	want := []string{
		"attributes.capacidad_litros debe ser mayor o igual a 50",
		"attributes.eficiencia_energetica debe ser uno de: A+++, A++, A+, A",
	}
	if len(errs) != 2 || errs[0].Message != want[0] || errs[1].Message != want[1] {
		t.Fatalf("mensajes = %+v", errs)
	}
}

func slicesCloneAttributes(in []Attribute) []Attribute {
	return append([]Attribute(nil), in...)
}
