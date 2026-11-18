package categories

import "testing"

func TestNormalizeSlug(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "nombre de categoría", in: "Refrigeradores", want: "refrigeradores"},
		{name: "recorta espacios y baja mayúsculas", in: "  REFRIGERADORES ", want: "refrigeradores"},
		{name: "trim final", in: "refrigeradores ", want: "refrigeradores"},
		{name: "notebooks", in: "Notebooks", want: "notebooks"},
		{name: "guiones bajos", in: "internet_fibra_optica", want: "internet-fibra-optica"},
		{name: "quita tilde sin crear alias", in: "refrigeración", want: "refrigeracion"},
		{name: "vacío", in: "", want: ""},
		{name: "solo espacios", in: "   ", want: ""},
		{name: "no repara guiones dobles", in: "internet__fibra_optica", want: "internet--fibra-optica"},
		{name: "no repara guion inicial", in: "-refrigeradores", want: "-refrigeradores"},
		{name: "no repara guion final", in: "refrigeradores-", want: "refrigeradores-"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeSlug(tc.in); got != tc.want {
				t.Fatalf("NormalizeSlug(%q) = %q, quiero %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSeedCategorySlugsRemainValid(t *testing.T) {
	for _, slug := range []string{
		"refrigeradores",
		"notebooks",
		"televisores",
		"internet-fibra-optica",
	} {
		if got := NormalizeSlug(slug); got != slug {
			t.Errorf("NormalizeSlug(%q) = %q", slug, got)
		}
		if !ValidSlug(slug) || len(slug) == 0 || slug[0] == '-' || slug[len(slug)-1] == '-' {
			t.Errorf("slug de semilla %q no cumple el formato", slug)
		}
		for i := 1; i < len(slug); i++ {
			if slug[i] == '-' && slug[i-1] == '-' {
				t.Errorf("slug de semilla %q contiene guiones dobles", slug)
				break
			}
		}
	}
}

func TestCanonicalCategorySlugAliases(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "refrigeración", want: "refrigeradores"},
		{input: "Refrigerador", want: "refrigeradores"},
		{input: "neveras", want: "refrigeradores"},
		{input: "frigorífico", want: "refrigeradores"},
		{input: "heladera", want: "refrigeradores"},
		{input: "freezer", want: "refrigeradores"},
		{input: "congeladores", want: "refrigeradores"},
		{input: "Refrigeradores y equipos de frío", want: "refrigeradores"},
		{input: "Laptop", want: "notebooks"},
		{input: "computadores portátiles", want: "notebooks"},
		{input: "televisión", want: "televisores"},
		{input: "TV", want: "televisores"},
		{input: "electrodomésticos", want: "electrodomesticos"},
		{input: "categoria-desconocida", want: "categoria-desconocida"},
		{input: "internet_fibra_optica", want: "internet-fibra-optica"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			if got := CanonicalCategorySlug(tc.input); got != tc.want {
				t.Fatalf("CanonicalCategorySlug(%q) = %q, quiero %q", tc.input, got, tc.want)
			}
		})
	}
}
