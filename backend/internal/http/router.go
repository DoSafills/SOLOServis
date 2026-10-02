package http

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

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

// NewRouter arma el servidor combinado (cmd/server): expone productos,
// tiendas y servicios en un mismo puerto.
func NewRouter(db *pgxpool.Pool, frontendURL string) *chi.Mux {
	r := chi.NewRouter()
	addCORS(r, frontendURL)

	registerHealth(r, db)
	registerProductRoutes(r, db)
	registerStoreRoutes(r, db)
	registerServiceRoutes(r, db)

	return r
}

// Routers de los microservicios (cmd/products-api, cmd/stores-api,
// cmd/services-api): cada uno expone solo su dominio.
func NewProductsRouter(db *pgxpool.Pool) *chi.Mux {
	return newDomainRouter(db, registerProductRoutes)
}

func NewStoresRouter(db *pgxpool.Pool) *chi.Mux {
	return newDomainRouter(db, registerStoreRoutes)
}

func NewServicesRouter(db *pgxpool.Pool) *chi.Mux {
	return newDomainRouter(db, registerServiceRoutes)
}

func newDomainRouter(db *pgxpool.Pool, register func(*chi.Mux, *pgxpool.Pool)) *chi.Mux {
	r := chi.NewRouter()
	addCORS(r, frontendURL())

	registerHealth(r, db)
	register(r, db)

	return r
}

// frontendURL se resuelve desde el entorno porque los microservicios
// construyen su router sin recibir la configuración.
func frontendURL() string {
	if value := os.Getenv("FRONTEND_URL"); value != "" {
		return value
	}

	return "http://localhost:5173"
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

func registerHealth(r *chi.Mux, db *pgxpool.Pool) {
	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		healthHandler(w, db)
	})
}

func registerProductRoutes(r *chi.Mux, db *pgxpool.Pool) {
	productRepository := products.NewRepository(db)
	productHandler := products.NewHandler(productRepository)

	r.Get("/categories", productHandler.ListCategories)
	r.Get("/products", productHandler.List)
	r.Post("/products", productHandler.Create)
	r.Get("/products/{publicID}", productHandler.GetByPublicID)
	r.Put("/products/{publicID}", productHandler.Update)
	r.Delete("/products/{publicID}", productHandler.Deactivate)
	r.Get("/products/{publicID}/reviews", productHandler.ListReviews)
}

func registerStoreRoutes(r *chi.Mux, db *pgxpool.Pool) {
	storeRepository := stores.NewRepository(db)
	storeHandler := stores.NewHandler(storeRepository)

	r.Get("/stores", storeHandler.List)
}

func registerServiceRoutes(r *chi.Mux, db *pgxpool.Pool) {
	serviceRepository := services.NewRepository(db)
	serviceHandler := services.NewHandler(serviceRepository)

	r.Get("/services", serviceHandler.List)
	r.Get("/services/{publicID}", serviceHandler.GetByPublicID)
}

func healthHandler(w http.ResponseWriter, db *pgxpool.Pool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	w.Header().Set("Content-Type", "application/json")

	if err := db.Ping(ctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(HealthResponse{
			Status:   "error",
			Service:  "soloservis-api",
			Database: "unavailable",
		})
		return
	}

	_ = json.NewEncoder(w).Encode(HealthResponse{
		Status:   "ok",
		Service:  "soloservis-api",
		Database: "connected",
	})
}
