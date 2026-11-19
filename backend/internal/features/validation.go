package features

import (
	"bytes"
	"encoding/json"
)

// Límites de los campos de features (TAL-159). Los largos son los de las
// columnas VARCHAR: un valor más largo hacía fallar el INSERT con 22001 y el
// cliente recibía 500 en vez de 422.
const (
	maxReviewTitle        = 200 // product_review.title, store_review.title, service_review.title
	maxNotificationType   = 50  // notification.type
	maxNotificationTitle  = 200 // notification.title
	maxSavedSearchName    = 150 // saved_search.name
	maxServiceComparison  = 150 // service_comparison.name
	maxCartQuantity       = 99  // unidades por ítem del carrito
	maxTargetPrice        = 9999999999.99
	defaultSearchType     = "product"
	defaultComparisonName = "Comparación de servicios"
)

// searchTypes son los valores aceptados en el type del historial de búsqueda
// (search_query.search_type).
var searchTypes = map[string]bool{"product": true, "service": true, "mixed": true}

// isUUID informa si value es un UUID en su forma canónica: 36 caracteres, con
// guiones en las posiciones 8, 13, 18 y 23 y dígitos hexadecimales en el
// resto. Recibe el texto tal como llegó en la ruta o en el body y devuelve
// true solo si PostgreSQL puede convertirlo a uuid sin error (22P02).
func isUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i := 0; i < len(value); i++ {
		char := value[i]
		switch i {
		case 8, 13, 18, 23:
			if char != '-' {
				return false
			}
		default:
			if !isHexDigit(char) {
				return false
			}
		}
	}
	return true
}

// isHexDigit informa si char es un dígito hexadecimal (0-9, a-f, A-F).
func isHexDigit(char byte) bool {
	return (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')
}

// isJSONObject informa si raw, ya validado como JSON por httpx.DecodeJSON, es
// un objeto. Un campo omitido o null también vale: se guarda como {}.
// Recibe el valor crudo del body y devuelve false para arreglos, textos,
// números o booleanos.
func isJSONObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) == 0 || string(trimmed) == "null" || trimmed[0] == '{'
}
