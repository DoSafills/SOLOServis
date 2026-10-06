// Package dbutil agrupa las conversiones entre los tipos nullables de pgx y
// los tipos planos que exponen los DTOs.
package dbutil

import (
	"github.com/jackc/pgx/v5/pgtype"
)

// Text devuelve el texto o "" si es NULL.
func Text(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}

	return value.String
}

// Float devuelve el número o 0 si es NULL o no representable.
func Float(value pgtype.Numeric) float64 {
	if pointer := FloatPtr(value); pointer != nil {
		return *pointer
	}

	return 0
}

// FloatPtr devuelve nil si el número es NULL o no representable.
func FloatPtr(value pgtype.Numeric) *float64 {
	if !value.Valid {
		return nil
	}

	number, err := value.Float64Value()
	if err != nil || !number.Valid {
		return nil
	}

	return &number.Float64
}

// NumericString conserva la precisión exacta del NUMERIC (p. ej. precios)
// serializándolo como texto. Devuelve "" si es NULL.
func NumericString(value pgtype.Numeric) string {
	if !value.Valid {
		return ""
	}

	encoded, err := value.MarshalJSON()
	if err != nil {
		return ""
	}

	return string(encoded)
}

// Int4Ptr devuelve nil si el entero es NULL.
func Int4Ptr(value pgtype.Int4) *int32 {
	if !value.Valid {
		return nil
	}

	number := value.Int32
	return &number
}

// Date formatea la fecha como YYYY-MM-DD, o "" si es NULL.
func Date(value pgtype.Timestamp) string {
	if !value.Valid {
		return ""
	}

	return value.Time.Format("2006-01-02")
}

// WithUnit une el valor de una especificación con su unidad ("5000 mAh").
// Si la especificación no tiene unidad, devuelve solo el valor.
func WithUnit(value string, unit pgtype.Text) string {
	if !unit.Valid || unit.String == "" {
		return value
	}

	return value + " " + unit.String
}

// NullableText guarda "" como NULL.
func NullableText(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

// NullableInt4 guarda nil como NULL.
func NullableInt4(value *int32) pgtype.Int4 {
	if value == nil {
		return pgtype.Int4{}
	}

	return pgtype.Int4{Int32: *value, Valid: true}
}

// GroupBy agrupa filas por id, para repartir entre varias entidades el
// resultado de una sola consulta con `= ANY(ids)`.
func GroupBy[T any](rows []T, id func(T) int32) map[int32][]T {
	groups := make(map[int32][]T)
	for _, row := range rows {
		key := id(row)
		groups[key] = append(groups[key], row)
	}

	return groups
}
