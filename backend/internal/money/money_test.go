package money

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func numeric(t *testing.T, intValue int64, exp int32) pgtype.Numeric {
	t.Helper()

	return pgtype.Numeric{Int: big.NewInt(intValue), Exp: exp, Valid: true}
}

func TestMarshalJSONEmitsBareNumber(t *testing.T) {
	tests := []struct {
		name   string
		amount Money
		want   string
	}{
		{"integer amount", Must("549990"), "549990"},
		{"two decimals", Must("549990.00"), "549990.00"},
		{"zero", Must("0"), "0"},
		{"fractional", Must("0.5"), "0.5"},
		{"absent", Money{}, "null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.amount)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			if string(got) != tt.want {
				t.Errorf("Marshal() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestMoneyIsNotAJSONString(t *testing.T) {
	// Regression guard: the previous implementation embedded the decimal in a
	// Go string, producing "price": "549990.00" and breaking clients that
	// expect a number.
	type offer struct {
		Price Money `json:"price"`
	}

	encoded, err := json.Marshal(offer{Price: Must("549990.00")})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	if want := `{"price":549990.00}`; string(encoded) != want {
		t.Fatalf("Marshal() = %s, want %s", encoded, want)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	price, ok := decoded["price"].(float64)
	if !ok {
		t.Fatalf("price decoded as %T, want float64", decoded["price"])
	}

	if price != 549990 {
		t.Errorf("price = %v, want 549990", price)
	}
}

func TestMarshalInsideNullablePointer(t *testing.T) {
	type offer struct {
		Price     Money  `json:"price"`
		ListPrice *Money `json:"listPrice"`
	}

	encoded, err := json.Marshal(offer{Price: Must("10.50")})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	if want := `{"price":10.50,"listPrice":null}`; string(encoded) != want {
		t.Fatalf("Marshal() = %s, want %s", encoded, want)
	}
}

func TestUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"number", "549990.00", "549990.00", false},
		{"integer", `549990`, "549990", false},
		{"quoted decimal is tolerated", `"549990.00"`, "549990.00", false},
		{"null", "null", "", false},
		{"not a number", `"abc"`, "", true},
		{"object", "{}", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Money
			err := json.Unmarshal([]byte(tt.input), &got)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Unmarshal(%s) expected error, got %v", tt.input, got.String())
				}

				return
			}

			if err != nil {
				t.Fatalf("Unmarshal(%s) error = %v", tt.input, err)
			}

			if got.String() != tt.want {
				t.Errorf("Unmarshal(%s) = %q, want %q", tt.input, got.String(), tt.want)
			}
		})
	}
}

func TestRoundTripPreservesExactDigits(t *testing.T) {
	// 0.1 + 0.2 style drift must not appear for a 12,2 numeric column.
	const exact = "1234567890.99"

	encoded, err := json.Marshal(Must(exact))
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	if string(encoded) != exact {
		t.Fatalf("Marshal() = %s, want %s", encoded, exact)
	}

	var decoded Money
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.String() != exact {
		t.Fatalf("round trip = %s, want %s", decoded.String(), exact)
	}
}

func TestFromNumeric(t *testing.T) {
	tests := []struct {
		name    string
		column  pgtype.Numeric
		want    string
		wantAny bool
	}{
		{"valid 549990.00", numeric(t, 54999000, -2), "549990.00", false},
		{"valid 0", numeric(t, 0, -2), "0.00", false},
		{"null column", pgtype.Numeric{}, "", false},
		{"NaN", pgtype.Numeric{NaN: true, Valid: true}, "", false},
		{"infinity", pgtype.Numeric{InfinityModifier: pgtype.Infinity, Valid: true}, "", false},
		{"negative infinity", pgtype.Numeric{InfinityModifier: pgtype.NegativeInfinity, Valid: true}, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromNumeric(tt.column)
			if got.String() != tt.want {
				t.Errorf("FromNumeric() = %q, want %q", got.String(), tt.want)
			}

			if got.Valid() != (tt.want != "") {
				t.Errorf("Valid() = %v, want %v", got.Valid(), tt.want != "")
			}
		})
	}
}

func TestNewRejectsInvalidAmounts(t *testing.T) {
	invalid := []string{
		"",
		" ",
		"abc",
		"1.2.3",
		"1e5",
		"$10",
		"1,000",
		"-",
		".5",
		"10.",
		"+10",
	}

	for _, amount := range invalid {
		t.Run(amount, func(t *testing.T) {
			if _, err := New(amount); err == nil {
				t.Errorf("New(%q) expected error, got none", amount)
			}
		})
	}
}

func TestValidAndIsZero(t *testing.T) {
	tests := []struct {
		amount   Money
		valid    bool
		isZero   bool
		testName string
	}{
		{amount: Must("0"), valid: true, isZero: true, testName: "zero"},
		{amount: Must("0.00"), valid: true, isZero: true, testName: "zero with decimals"},
		{amount: Must("0.01"), valid: true, isZero: false, testName: "smallest cent"},
		{amount: Must("10"), valid: true, isZero: false, testName: "non zero"},
		{amount: Money{}, valid: false, isZero: true, testName: "absent"},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			if got := tt.amount.Valid(); got != tt.valid {
				t.Errorf("Valid() = %v, want %v", got, tt.valid)
			}

			if got := tt.amount.IsZero(); got != tt.isZero {
				t.Errorf("IsZero() = %v, want %v", got, tt.isZero)
			}
		})
	}
}

func TestFloat64(t *testing.T) {
	if got := Must("549990.50").Float64(); got != 549990.5 {
		t.Errorf("Float64() = %v, want 549990.5", got)
	}

	if got := (Money{}).Float64(); got != 0 {
		t.Errorf("Float64() on absent = %v, want 0", got)
	}
}

func TestInt64(t *testing.T) {
	tests := []struct {
		name   string
		amount Money
		want   int64
		wantOK bool
	}{
		{name: "integer", amount: Must("549990"), want: 549990, wantOK: true},
		{name: "trailing zeros", amount: Must("549990.00"), want: 549990, wantOK: true},
		{name: "fractional", amount: Must("10.50"), wantOK: false},
		{name: "absent", amount: Money{}, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.amount.Int64()
			if ok != tt.wantOK {
				t.Fatalf("Int64() ok = %v, want %v", ok, tt.wantOK)
			}

			if ok && got != tt.want {
				t.Errorf("Int64() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestValueAndScan(t *testing.T) {
	value, err := Must("549990.00").Value()
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}

	if value != "549990.00" {
		t.Errorf("Value() = %v, want 549990.00", value)
	}

	absent, err := Money{}.Value()
	if err != nil {
		t.Fatalf("Value() on absent error = %v", err)
	}

	if absent != nil {
		t.Errorf("Value() on absent = %v, want nil", absent)
	}

	var scanned Money
	if err := scanned.Scan("1234.56"); err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if scanned.String() != "1234.56" {
		t.Errorf("Scan() = %s, want 1234.56", scanned.String())
	}

	if err := scanned.Scan(nil); err != nil {
		t.Fatalf("Scan(nil) error = %v", err)
	}

	if scanned.Valid() {
		t.Error("Scan(nil) should clear the amount")
	}

	if err := scanned.Scan(struct{}{}); err == nil {
		t.Error("Scan(unsupported) expected error, got none")
	}
}
