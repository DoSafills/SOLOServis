// Package chiroute conecta los errores de enrutamiento de chi con el estándar
// TAL-78 (TAL-245). Está fuera de httpx para que httpx siga dependiendo solo
// de la biblioteca estándar.
package chiroute

import (
	"net/http"

	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
)

// candidateMethods son los métodos que se prueban para armar el encabezado
// Allow, en el orden en que aparecen en él.
var candidateMethods = []string{
	http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch,
	http.MethodDelete, http.MethodOptions, http.MethodConnect, http.MethodTrace,
}

// UseProblemFallbacks hace que r responda problem+json en vez de las
// respuestas por defecto de chi: 404 not_found si ninguna ruta coincide y 405
// method_not_allowed, con el encabezado Allow, si la ruta existe con otros
// métodos. Recibe el router base de la API; los r.Group lo comparten, pero un
// router agregado con r.Mount necesita su propia llamada. Los handlers corren
// detrás de los middlewares de r, así que llevan request_id. No devuelve nada.
func UseProblemFallbacks(r *chi.Mux) {
	r.NotFound(httpx.WriteRouteNotFound)
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		httpx.WriteMethodNotAllowed(w, req, AllowedMethods(r, req))
	})
}

// AllowedMethods devuelve los métodos con los que r atiende la ruta de req,
// en el orden de candidateMethods; una lista vacía si ninguno. Recibe el
// router y el request. Usa la misma ruta que chi al enrutar: RoutePath del
// contexto de chi (con Mount) o, si no hay, RawPath o Path de la URL.
func AllowedMethods(r *chi.Mux, req *http.Request) []string {
	path := routePath(req)
	var allowed []string
	for _, method := range candidateMethods {
		if r.Match(chi.NewRouteContext(), method, path) {
			allowed = append(allowed, method)
		}
	}
	return allowed
}

// routePath devuelve la ruta que chi usa para enrutar req (ver
// chi.Mux.routeHTTP). Recibe el request y devuelve la ruta, "/" si está vacía.
func routePath(req *http.Request) string {
	if rctx := chi.RouteContext(req.Context()); rctx != nil && rctx.RoutePath != "" {
		return rctx.RoutePath
	}
	if req.URL.RawPath != "" {
		return req.URL.RawPath
	}
	if req.URL.Path != "" {
		return req.URL.Path
	}
	return "/"
}
