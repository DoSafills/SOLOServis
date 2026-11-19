package httpx

import (
	"net/http"
	"strings"
)

// Mensajes de los errores de enrutamiento (TAL-245). RouteNotFoundDetail es
// distinto del 404 de internal/allapis ("La ruta no existe."), que significa
// que el proceso no sirve esa API: así se distingue qué proceso respondió.
const (
	RouteNotFoundDetail    = "La ruta no existe en esta API."
	MethodNotAllowedDetail = "El método no está permitido en esta ruta."
)

// WriteRouteNotFound responde 404 not_found para una ruta que ninguna ruta
// registrada de la API atiende. Recibe el ResponseWriter y el request; no
// devuelve nada: escribe el Problem completo.
func WriteRouteNotFound(w http.ResponseWriter, r *http.Request) {
	WriteProblem(w, r, http.StatusNotFound, CodeNotFound, RouteNotFoundDetail)
}

// WriteMethodNotAllowed responde 405 method_not_allowed cuando la ruta existe
// pero no con el método pedido. Recibe el ResponseWriter, el request y los
// métodos que la ruta sí acepta, que van en el encabezado Allow separados por
// coma (RFC 9110, §10.2.1). No devuelve nada: escribe el Problem completo.
func WriteMethodNotAllowed(w http.ResponseWriter, r *http.Request, allowed []string) {
	if len(allowed) > 0 {
		w.Header().Set("Allow", strings.Join(allowed, ", "))
	}
	WriteProblem(w, r, http.StatusMethodNotAllowed, CodeMethodNotAllowed, MethodNotAllowedDetail)
}
