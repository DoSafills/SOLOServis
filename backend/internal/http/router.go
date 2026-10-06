package http

import (
	"context"
	"net/http"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/http/httpx"
	"github.com/DoSafills/SOLOServis/backend/internal/products"
	"github.com/DoSafills/SOLOServis/backend/internal/services"
	"github.com/DoSafills/SOLOServis/backend/internal/stores"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type HealthResponse struct {
	Status   string `json:"status"`
	Service  string `json:"service"`
	Database string `json:"database"`
}

type routeRegistrar func(*chi.Mux, *pgxpool.Pool)

// NewRouter arma el servidor combinado (cmd/server): expone productos,
// tiendas y servicios en un mismo puerto.
func NewRouter(db *pgxpool.Pool, frontendURL string) *chi.Mux {
	return newRouter(db, frontendURL, registerProductRoutes, registerStoreRoutes, registerServiceRoutes)
}

// Routers de los microservicios (cmd/products-api, cmd/stores-api,
// cmd/services-api): cada uno expone solo su dominio.
func NewProductsRouter(db *pgxpool.Pool, frontendURL string) *chi.Mux {
	return newRouter(db, frontendURL, registerProductRoutes)
}

func NewStoresRouter(db *pgxpool.Pool, frontendURL string) *chi.Mux {
	return newRouter(db, frontendURL, registerStoreRoutes)
}

func NewServicesRouter(db *pgxpool.Pool, frontendURL string) *chi.Mux {
	return newRouter(db, frontendURL, registerServiceRoutes)
}

func newRouter(db *pgxpool.Pool, frontendURL string, registrars ...routeRegistrar) *chi.Mux {
	r := chi.NewRouter()
	addCORS(r, frontendURL)

	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		healthHandler(w, db)
	})

	for _, register := range registrars {
		register(r, db)
	}

	return r
}

func addCORS(r *chi.Mux, frontendURL string) {
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", frontendURL)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if req.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, req)
		})
	})
}

func registerProductRoutes(r *chi.Mux, db *pgxpool.Pool) {
	productHandler := products.NewHandler(products.NewRepository(db))

	r.Get("/categories", productHandler.ListCategories)
	r.Get("/products", productHandler.List)
	r.Post("/products", productHandler.Create)
	r.Get("/products/{publicID}", productHandler.GetByPublicID)
	r.Put("/products/{publicID}", productHandler.Update)
	r.Delete("/products/{publicID}", productHandler.Deactivate)
	r.Get("/products/{publicID}/reviews", productHandler.ListReviews)
}

func registerStoreRoutes(r *chi.Mux, db *pgxpool.Pool) {
	storeHandler := stores.NewHandler(stores.NewRepository(db))

	r.Get("/stores", storeHandler.List)
	r.Get("/stores/{id}", storeHandler.GetByID)
}

func registerServiceRoutes(r *chi.Mux, db *pgxpool.Pool) {
	serviceHandler := services.NewHandler(services.NewRepository(db))

	r.Get("/services", serviceHandler.List)
	r.Get("/services/{publicID}", serviceHandler.GetByPublicID)
}

func healthHandler(w http.ResponseWriter, db *pgxpool.Pool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := db.Ping(ctx); err != nil {
		httpx.WriteJSON(w, http.StatusServiceUnavailable, HealthResponse{
			Status:   "error",
			Service:  "soloservis-api",
			Database: "unavailable",
		})
		return
	}

	httpx.WriteJSON(w, http.StatusOK, HealthResponse{
		Status:   "ok",
		Service:  "soloservis-api",
		Database: "connected",
	})
}
