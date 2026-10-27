// Package allapis sirve las 14 APIs en un solo proceso (cmd/all-apis).
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

// Routers construye el router de cada API, indexado por nombre.
func Routers(db *pgxpool.Pool, frontendURL string) map[string]*chi.Mux {
	return map[string]*chi.Mux{
		"product":      apphttp.NewProductRouter(db, frontendURL),
		"store":        apphttp.NewStoreRouter(db, frontendURL),
		"service":      apphttp.NewServiceRouter(db, frontendURL),
		"user":         apphttp.NewUserRouter(db, frontendURL),
		"pricing":      apphttp.NewPricingRouter(db, frontendURL),
		"search":       apphttp.NewSearchRouter(db, frontendURL),
		"comparison":   apphttp.NewComparisonRouter(db, frontendURL),
		"ingestion":    apphttp.NewIngestionRouter(db, frontendURL),
		"auth":         auth.NewRouter(db, frontendURL),
		"cart":         cart.NewRouter(db, frontendURL),
		"favorites":    favorites.NewRouter(db, frontendURL),
		"notification": notifications.NewRouter(db, frontendURL),
		"review":       reviews.NewRouter(db, frontendURL),
		"watchlist":    watchlist.NewRouter(db, frontendURL),
	}
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

// NewRouter entrega cada request al router de su API. /health lo responde
// Product API (todas hacen el mismo ping a PostgreSQL).
func NewRouter(db *pgxpool.Pool, frontendURL string) *chi.Mux {
	routers := Routers(db, frontendURL)
	r := chi.NewRouter()
	r.HandleFunc("/*", func(w http.ResponseWriter, req *http.Request) {
		api := APIFor(req.URL.Path)
		if req.URL.Path == "/health" {
			api = "product"
		}
		if api == "" {
			httpx.WriteProblem(w, req, http.StatusNotFound, httpx.CodeNotFound, "La ruta no existe.")
			return
		}
		routers[api].ServeHTTP(w, req)
	})
	return r
}
