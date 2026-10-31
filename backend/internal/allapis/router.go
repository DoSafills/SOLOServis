// Package allapis sirve varias APIs en un solo proceso (cmd/all-apis): las de
// un dominio (API_DOMAIN, ver domains.go) o las 14 (modo consolidado).
//
// Existe por la cuota del clúster UCT (12 pods y 10 Services por estudiante):
// ahí no caben 14 Deployments. Cada request se entrega sin cambios al router
// de la API dueña de su prefijo, con sus mismos middlewares, así que el
// contrato HTTP es idéntico al de los microservicios.
package allapis

import (
	"net/http"
	"sort"
	"strings"

	"github.com/DoSafills/SOLOServis/backend/internal/auth"
	"github.com/DoSafills/SOLOServis/backend/internal/cart"
	"github.com/DoSafills/SOLOServis/backend/internal/favorites"
	apphttp "github.com/DoSafills/SOLOServis/backend/internal/http"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/DoSafills/SOLOServis/backend/internal/notifications"
	"github.com/DoSafills/SOLOServis/backend/internal/reviews"
	"github.com/DoSafills/SOLOServis/backend/internal/watchlist"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Prefixes asigna cada prefijo de ruta a la API que lo atiende.
var Prefixes = map[string]string{
	"/products":           "product",
	"/product-categories": "product",
	"/categories":         "product",
	"/stores":             "store",
	"/services":           "service",
	"/users":              "user",
	"/offers":             "pricing",
	"/search":             "search",
	"/comparisons":        "comparison",
	"/ingestion":          "ingestion",
	"/auth":               "auth",
	"/cart":               "cart",
	"/favorites":          "favorites",
	"/notifications":      "notification",
	"/reviews":            "review",
	"/watchlist":          "watchlist",
}

// RouterFactory construye el router de una API.
type RouterFactory = func(db *pgxpool.Pool, frontendURL string) *chi.Mux

// factories tiene el constructor del router de cada API.
var factories = map[string]RouterFactory{
	"product":      apphttp.NewProductRouter,
	"store":        apphttp.NewStoreRouter,
	"service":      apphttp.NewServiceRouter,
	"user":         apphttp.NewUserRouter,
	"pricing":      apphttp.NewPricingRouter,
	"search":       apphttp.NewSearchRouter,
	"comparison":   apphttp.NewComparisonRouter,
	"ingestion":    apphttp.NewIngestionRouter,
	"auth":         auth.NewRouter,
	"cart":         cart.NewRouter,
	"favorites":    favorites.NewRouter,
	"notification": notifications.NewRouter,
	"review":       reviews.NewRouter,
	"watchlist":    watchlist.NewRouter,
}

// allAPIs son las 14 APIs del modo consolidado; product responde /health.
var allAPIs = []string{"product", "store", "service", "user", "pricing", "search", "comparison",
	"ingestion", "auth", "cart", "favorites", "notification", "review", "watchlist"}

// Routers construye el router de cada API, indexado por nombre.
func Routers(db *pgxpool.Pool, frontendURL string) map[string]*chi.Mux {
	return build(allAPIs, db, frontendURL)
}

func build(apis []string, db *pgxpool.Pool, frontendURL string) map[string]*chi.Mux {
	routers := make(map[string]*chi.Mux, len(apis))
	for _, api := range apis {
		routers[api] = factories[api](db, frontendURL)
	}
	return routers
}

// APIFor devuelve la API dueña de path, o "" si ningún prefijo coincide.
// Gana el prefijo más largo y solo coincide por segmento completo
// (/products y /products/x, pero no /productsx).
func APIFor(path string) string {
	for _, prefix := range sortedPrefixes {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return Prefixes[prefix]
		}
	}
	return ""
}

var sortedPrefixes = func() []string {
	list := make([]string, 0, len(Prefixes))
	for prefix := range Prefixes {
		list = append(list, prefix)
	}
	sort.Slice(list, func(i, j int) bool { return len(list[i]) > len(list[j]) })
	return list
}()

// NewRouter sirve las 14 APIs (modo consolidado).
func NewRouter(db *pgxpool.Pool, frontendURL string) *chi.Mux {
	return NewDomainRouter(allAPIs)(db, frontendURL)
}

// NewDomainRouter sirve solo las APIs dadas: cada request va al router de su
// API, y las rutas de APIs que no están en la lista responden 404. /health lo
// responde la primera API (todas hacen el mismo ping a PostgreSQL).
func NewDomainRouter(apis []string) RouterFactory {
	return func(db *pgxpool.Pool, frontendURL string) *chi.Mux {
		routers := build(apis, db, frontendURL)
		r := chi.NewRouter()
		r.HandleFunc("/*", func(w http.ResponseWriter, req *http.Request) {
			api := APIFor(req.URL.Path)
			if req.URL.Path == "/health" {
				api = apis[0]
			}
			router, ok := routers[api]
			if !ok {
				httpx.WriteProblem(w, req, http.StatusNotFound, httpx.CodeNotFound, "La ruta no existe.")
				return
			}
			router.ServeHTTP(w, req)
		})
		return r
	}
}
