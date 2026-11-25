package httpx

import (
	"net/http"
	"strings"
)

// corsAllowedMethods son los métodos que acepta el CORS del estándar TAL-78.
// No incluye PATCH: ninguna ruta lo admite y el 405 lo informa en Allow.
const corsAllowedMethods = "GET, POST, PUT, DELETE, OPTIONS"

// NewCORS devuelve el middleware CORS común a las 14 APIs. Recibe el origen
// del frontend (FRONTEND_URL) y devuelve el middleware que agrega las
// cabeceras CORS y responde 204 a los preflight OPTIONS.
//
// Existe para que todas las APIs respondan lo mismo: antes cada router tenía
// su copia y solo una exponía X-Total-Count, así que el navegador no podía
// leer el total de un listado paginado en las APIs de features. Con el origen
// vacío (mismo origen, como en Kubernetes) no se manda
// Access-Control-Allow-Origin: una cabecera con valor vacío es inválida.
func NewCORS(frontendURL string) func(http.Handler) http.Handler {
	origin := strings.TrimRight(strings.TrimSpace(frontendURL), "/")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
			}
			w.Header().Set("Access-Control-Allow-Methods", corsAllowedMethods)
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Expose-Headers", "X-Total-Count")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
