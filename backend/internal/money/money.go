// Package money provides a JSON-safe representation of monetary amounts.
//
// PostgreSQL stores prices as NUMERIC(12,2). Using float64 for money is a
// well-known source of rounding errors, and encoding the decimal as a JSON
// string (for example "549990.00") breaks clients that expect a number.
//
// Money keeps the exact decimal digits that PostgreSQL returned and marshals
// them as a bare JSON number, so the wire format is lossless and numeric.
package money

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"
)

// Money is a monetary amount backed by its exact decimal representation.
//
// The zero value is an absent amount and marshals to JSON null. Use New or
// FromNumeric to build a populated value.
type Money struct {
	amount string
}

// New returns a Money holding the given decimal string, for example
// "549990.00". The string is validated to be a plain decimal number; no
// exponent notation, currency symbols or thousands separators are accepted.
func New(amount string) (Money, error) {
	trimmed := strings.TrimSpace(amount)

	if !isDecimal(trimmed) {
		return Money{}, fmt.Errorf("money: %q is not a valid decimal amount", amount)
	}

	return Money{amount: trimmed}, nil
}

// Must is like New but panics on invalid input. It is intended for constants
// in tests and package-level declarations.
func Must(amount string) Money {
	value, err := New(amount)
	if err != nil {
		panic(err)
	}

	return value
}

// FromNumeric converts a PostgreSQL NUMERIC into a Money. A NULL, NaN or
// infinite column produces the zero (absent) value.
func FromNumeric(n pgtype.Numeric) Money {
	if !n.Valid || n.NaN || n.InfinityModifier != pgtype.Finite {
		return Money{}
	}

	// pgtype.Numeric marshals to the bare decimal digits, which is exactly the
	// representation Money keeps.
	encoded, err := n.MarshalJSON()
	if err != nil {
		return Money{}
	}

	value, err := New(string(encoded))
	if err != nil {
		return Money{}
	}

	return value
}

// FromFloat64 builds a Money from a float using the given number of decimal
// places. It is intended for tests and for values that did not originate from
// an exact decimal column.
func FromFloat64(amount float64, decimals int) Money {
	return Money{amount: strings.TrimSpace(fmt.Sprintf("%.*f", decimals, amount))}
}

// Valid reports whether the amount is present.
func (m Money) Valid() bool {
	return m.amount != ""
}

// IsZero reports whether the amount is present and equal to zero.
func (m Money) IsZero() bool {
	if !m.Valid() {
		return true
	}

	return strings.Trim(m.amount, "0.") == ""
}

// String returns the exact decimal representation, or an empty string when the
// amount is absent.
func (m Money) String() string {
	return m.amount
}

// Float64 returns the amount as a float64. Precision beyond 15 significant
// digits is lost, so prefer String for display and serialization.
func (m Money) Float64() float64 {
	if !m.Valid() {
		return 0
	}

	var value float64
	if _, err := fmt.Sscanf(m.amount, "%g", &value); err != nil {
		return 0
	}

	return value
}

// Int64 returns the amount truncated to an integer. It reports false when the
// amount is absent or has a fractional part.
func (m Money) Int64() (int64, bool) {
	if !m.Valid() {
		return 0, false
	}

	if strings.Contains(m.amount, ".") {
		trimmed := strings.TrimRight(m.amount, "0")
		trimmed = strings.TrimSuffix(trimmed, ".")

		if strings.Contains(trimmed, ".") {
			return 0, false
		}
	}

	var value int64
	if _, err := fmt.Sscanf(m.amount, "%d", &value); err != nil {
		return 0, false
	}

	return value, true
}

// MarshalJSON writes the amount as a bare JSON number, or null when absent.
func (m Money) MarshalJSON() ([]byte, error) {
	if !m.Valid() {
		return []byte("null"), nil
	}

	return []byte(m.amount), nil
}

// UnmarshalJSON accepts a JSON number or a decimal string and stores the exact
// digits. Strings are tolerated so that clients sending "549990.00" keep
// working, but they are re-emitted as numbers.
func (m *Money) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)

	if bytes.Equal(trimmed, []byte("null")) {
		m.amount = ""

		return nil
	}

	candidate := string(trimmed)
	if len(trimmed) > 1 && trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"' {
		var unquoted string
		if err := json.Unmarshal(trimmed, &unquoted); err != nil {
			return fmt.Errorf("money: cannot parse %s: %w", data, err)
		}

		candidate = unquoted
	}

	value, err := New(candidate)
	if err != nil {
		return err
	}

	m.amount = value.amount

	return nil
}

// Value implements driver.Valuer so Money can be used as a query parameter.
func (m Money) Value() (driver.Value, error) {
	if !m.Valid() {
		return nil, nil
	}

	return m.amount, nil
}

// Scan implements sql.Scanner so Money can be read from a numeric column.
func (m *Money) Scan(src any) error {
	switch value := src.(type) {
	case nil:
		m.amount = ""

		return nil
	case string:
		parsed, err := New(value)
		if err != nil {
			return err
		}

		m.amount = parsed.amount

		return nil
	case []byte:
		parsed, err := New(string(value))
		if err != nil {
			return err
		}

		m.amount = parsed.amount

		return nil
	case int64:
		m.amount = fmt.Sprintf("%d", value)

		return nil
	case float64:
		m.amount = strings.TrimSpace(fmt.Sprintf("%f", value))

		return nil
	default:
		return fmt.Errorf("money: cannot scan %T", src)
	}
}

// Schema makes Money describe itself to Huma as a JSON number rather than an
// opaque object, so the generated OpenAPI document matches the wire format.
//
// No format is declared on purpose. "double" would tell clients to parse the
// amount as an IEEE 754 float, which is exactly the precision loss this type
// exists to prevent, so the amount is documented as a plain JSON number with
// the value's decimal digits preserved. No minimum is declared either, since a
// generic monetary amount may legitimately be a negative credit.
func (Money) Schema(_ huma.Registry) *huma.Schema {
	return &huma.Schema{
		Type:        huma.TypeNumber,
		Description: "Monetary amount, serialized as a JSON number with its decimal digits preserved.",
		Examples:    []any{549990},
	}
}

// isDecimal reports whether value is a plain decimal number such as "0",
// "549990" or "549990.00". Scientific notation is rejected so that the value
// can be written to the JSON stream verbatim, and a trailing separator such as
// "10." is rejected because it is not a valid JSON number.
func isDecimal(value string) bool {
	if value == "" {
		return false
	}

	digits := 0
	dots := 0

	for i, r := range value {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == '.':
			// A separator may not lead the value or follow another separator.
			if i == 0 || i == len(value)-1 || dots > 0 {
				return false
			}

			dots++
		default:
			return false
		}
	}

	return digits > 0 && dots <= 1
}
