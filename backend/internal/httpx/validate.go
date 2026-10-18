package httpx

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Validator acumula errores por campo. El valor cero está listo para usarse.
type Validator struct {
	errors []FieldError
}

// Add registra un error para field.
func (v *Validator) Add(field, code, message string) {
	v.errors = append(v.errors, FieldError{Field: field, Code: code, Message: message})
}

// Check registra el error si ok es false.
func (v *Validator) Check(ok bool, field, code, message string) {
	if !ok {
		v.Add(field, code, message)
	}
}

// Required exige un texto no vacío (ignorando espacios).
func (v *Validator) Required(field, value string) {
	v.Check(strings.TrimSpace(value) != "", field, "required", field+" es obligatorio")
}

// MaxLength limita el largo en caracteres (no bytes).
func (v *Validator) MaxLength(field, value string, max int) {
	v.Check(utf8.RuneCountInString(value) <= max, field, "max_length",
		fmt.Sprintf("%s no puede superar %d caracteres", field, max))
}

// Valid indica si no hubo errores.
func (v *Validator) Valid() bool {
	return len(v.errors) == 0
}

// Errors devuelve una copia de los errores acumulados.
func (v *Validator) Errors() []FieldError {
	return append([]FieldError(nil), v.errors...)
}
