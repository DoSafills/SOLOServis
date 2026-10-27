package categories

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
)

// maxSafeInteger es el mayor entero que float64 representa sin ambigüedad
// (2^53 - 1, igual que Number.MAX_SAFE_INTEGER en JavaScript).
const maxSafeInteger = 1<<53 - 1

// maxTextLength es el largo máximo de un valor de texto cuando la definición
// no fija max_length: coincide con *_specification_value.value VARCHAR(255).
const maxTextLength = 255

// Value es un atributo validado y normalizado, listo para guardar.
type Value struct {
	AttributeID int32  // FK a category_attributes.id
	Key         string // key de la definición
	DataType    string // data_type de la definición
	Text        string // forma canónica: "350", "65.5", "true", "A+", "Gris"
}

// ValidateAttributes valida los atributos recibidos de un producto o servicio
// contra las definiciones de su categoría (TAL-72). No consulta la base: recibe
// las definiciones ya cargadas (p. ej. con Repository.GetBySlug).
//
// input es el objeto "attributes" del request, decodificado como
// map[string]json.RawMessage. No hay conversiones implícitas: "350" no es un
// entero válido. Los errores usan el campo "attributes.<key>" y salen en orden
// determinista (definiciones por displayOrder, luego keys desconocidas por
// orden alfabético). Si hay errores, el handler responde 422 con
// httpx.WriteValidation.
func ValidateAttributes(defs []Attribute, input map[string]json.RawMessage) ([]Value, []httpx.FieldError) {
	ordered := slices.Clone(defs)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].DisplayOrder < ordered[j].DisplayOrder })

	var v httpx.Validator
	values := make([]Value, 0, len(input))
	known := make(map[string]bool, len(ordered))

	for _, def := range ordered {
		known[def.Key] = true
		field := "attributes." + def.Key

		raw, present := input[def.Key]
		if !present || isJSONNull(raw) {
			if def.Required {
				v.Add(field, "required", field+" es obligatorio")
			}
			continue
		}

		text, ok := normalizeValue(&v, field, def, raw)
		if !ok {
			continue
		}
		if text == "" { // texto vacío tras TrimSpace: equivale a no enviarlo
			if def.Required {
				v.Add(field, "required", field+" es obligatorio")
			}
			continue
		}

		values = append(values, Value{AttributeID: def.ID, Key: def.Key, DataType: def.DataType, Text: text})
	}

	unknown := make([]string, 0)
	for key := range input {
		if !known[key] {
			unknown = append(unknown, key)
		}
	}
	sort.Strings(unknown)
	for _, key := range unknown {
		field := "attributes." + key
		v.Add(field, "unknown_field", field+" no está definido para esta categoría")
	}

	return values, v.Errors()
}

// normalizeValue valida raw según el data_type y devuelve su forma canónica.
// Si el valor no es válido registra el error en v y devuelve ok = false.
func normalizeValue(v *httpx.Validator, field string, def Attribute, raw json.RawMessage) (string, bool) {
	switch def.DataType {
	case "integer":
		n, ok := parseNumber(raw)
		if !ok || n != math.Trunc(n) || math.Abs(n) > maxSafeInteger {
			v.Add(field, "invalid_type", field+" debe ser un número entero")
			return "", false
		}
		if !checkRange(v, field, def, n) {
			return "", false
		}
		return strconv.FormatInt(int64(n), 10), true

	case "decimal":
		n, ok := parseNumber(raw)
		if !ok {
			v.Add(field, "invalid_type", field+" debe ser un número")
			return "", false
		}
		if !checkRange(v, field, def, n) {
			return "", false
		}
		return formatNumber(n), true

	case "boolean":
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			v.Add(field, "invalid_type", field+" debe ser true o false")
			return "", false
		}
		return strconv.FormatBool(b), true

	case "enum":
		s, ok := parseString(raw)
		if !ok {
			v.Add(field, "invalid_type", field+" debe ser un texto")
			return "", false
		}
		if s != "" && !slices.Contains(def.AllowedValues, s) {
			v.Add(field, "one_of", fmt.Sprintf("%s debe ser uno de: %s", field, strings.Join(def.AllowedValues, ", ")))
			return "", false
		}
		return s, true

	case "string":
		s, ok := parseString(raw)
		if !ok {
			v.Add(field, "invalid_type", field+" debe ser un texto")
			return "", false
		}
		limit := maxTextLength
		if def.MaxLength != nil {
			limit = int(*def.MaxLength)
		}
		if utf8.RuneCountInString(s) > limit {
			v.Add(field, "max_length", fmt.Sprintf("%s no puede superar %d caracteres", field, limit))
			return "", false
		}
		return s, true
	}

	// Un data_type fuera del catálogo no debería existir (CHECK en BD).
	v.Add(field, "invalid_type", field+" tiene un tipo de dato no soportado")
	return "", false
}

// checkRange aplica min/max de la definición (inclusivos).
func checkRange(v *httpx.Validator, field string, def Attribute, n float64) bool {
	if def.Min != nil && n < *def.Min {
		v.Add(field, "min", fmt.Sprintf("%s debe ser mayor o igual a %s", field, formatNumber(*def.Min)))
		return false
	}
	if def.Max != nil && n > *def.Max {
		v.Add(field, "max", fmt.Sprintf("%s debe ser menor o igual a %s", field, formatNumber(*def.Max)))
		return false
	}
	return true
}

// parseNumber acepta solo números JSON (no strings) finitos en float64.
// 350, 350.0 y 3.5e2 representan el mismo valor. ParseFloat tiene costo
// acotado aunque el exponente sea enorme (1e999999 → fuera de rango).
func parseNumber(raw json.RawMessage) (float64, bool) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return 0, false
	}
	number, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseFloat(number.String(), 64)
	if err != nil || math.IsInf(n, 0) || math.IsNaN(n) {
		return 0, false
	}
	return n, true
}

// parseString acepta solo strings JSON y les quita espacios en los extremos.
func parseString(raw json.RawMessage) (string, bool) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	return strings.TrimSpace(s), true
}

func isJSONNull(raw json.RawMessage) bool {
	return string(bytes.TrimSpace(raw)) == "null"
}

func formatNumber(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}
