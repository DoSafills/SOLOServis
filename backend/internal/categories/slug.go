package categories

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var categorySlugAliases = map[string]string{
	"refrigeracion":                    "refrigeradores",
	"refrigerador":                     "refrigeradores",
	"nevera":                           "refrigeradores",
	"neveras":                          "refrigeradores",
	"frigorifico":                      "refrigeradores",
	"frigorificos":                     "refrigeradores",
	"heladera":                         "refrigeradores",
	"heladeras":                        "refrigeradores",
	"freezer":                          "refrigeradores",
	"freezers":                         "refrigeradores",
	"congelador":                       "refrigeradores",
	"congeladores":                     "refrigeradores",
	"refrigeradores-y-equipos-de-frio": "refrigeradores",
	"linea-blanca-refrigeracion":       "refrigeradores",
	"notebook":                         "notebooks",
	"laptop":                           "notebooks",
	"laptops":                          "notebooks",
	"computador-portatil":              "notebooks",
	"computadores-portatiles":          "notebooks",
	"computadora-portatil":             "notebooks",
	"computadoras-portatiles":          "notebooks",
	"televisor":                        "televisores",
	"television":                       "televisores",
	"televisiones":                     "televisores",
	"tv":                               "televisores",
	"tvs":                              "televisores",
	"telefonia":                        "celulares",
	"computadores":                     "computacion",
}

// NormalizeSlug normaliza el slug recibido sin reparar separadores inválidos.
func NormalizeSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = norm.NFD.String(value)

	var normalized strings.Builder
	for _, r := range value {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if unicode.IsSpace(r) || r == '_' {
			normalized.WriteByte('-')
			continue
		}
		normalized.WriteRune(r)
	}

	return normalized.String()
}

// CanonicalCategorySlug aplica aliases explícitos después de normalizar.
func CanonicalCategorySlug(value string) string {
	slug := NormalizeSlug(value)
	if canonical, ok := categorySlugAliases[slug]; ok {
		return canonical
	}
	return slug
}
