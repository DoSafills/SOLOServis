package products

import "testing"

func TestNormalizeProductOfferURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "normalize scheme host path and fragment",
			input: "  HTTPS://Shop.Example.COM/Path/?utm_source=x#section  ",
			want:  "https://shop.example.com/Path?utm_source=x",
		},
		{
			name:  "preserve query string",
			input: "https://shop.example/path/?return=/home",
			want:  "https://shop.example/path?return=/home",
		},
		{
			name:  "retain unparsable URL after trimming",
			input: " not a URL#fragment ",
			want:  "not a URL#fragment",
		},
		{
			name:  "retain malformed escape after trimming",
			input: " https://shop.example/%zz#fragment ",
			want:  "https://shop.example/%zz#fragment",
		},
		{
			name:  "normalize valid IPv6 host",
			input: "HTTPS://[2001:DB8::1]/Path/",
			want:  "https://[2001:db8::1]/Path",
		},
		{
			name:  "normalize valid IPv6 host zone",
			input: "HTTPS://[FE80::1%25en0]/Path/",
			want:  "https://[fe80::1%25en0]/Path",
		},
		{
			name:  "retain invalid IPv6 host",
			input: "https://[invalid]/Path/#fragment",
			want:  "https://[invalid]/Path/#fragment",
		},
		{
			name:  "retain malformed escape in fragment",
			input: "https://shop.example/path#frag%zz",
			want:  "https://shop.example/path#frag%zz",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := NormalizeProductOfferURL(test.input); got != test.want {
				t.Fatalf("NormalizeProductOfferURL(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestValidateCurrencyCode(t *testing.T) {
	for _, currency := range []string{"CLP", "USD", "EUR"} {
		if err := ValidateCurrencyCode(currency); err != nil {
			t.Errorf("ValidateCurrencyCode(%q) returned an error: %v", currency, err)
		}
	}

	for _, currency := range []string{"", "US", "usd", "U1D", "ÜSD"} {
		if err := ValidateCurrencyCode(currency); err == nil {
			t.Errorf("ValidateCurrencyCode(%q) returned no error", currency)
		}
	}
}
