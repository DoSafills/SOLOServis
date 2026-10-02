package http

import (
	"context"
	"encoding/json"
	"net/http"
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

func NewRouter(db *pgxpool.Pool, frontendURL string) *chi.Mux {
	r := chi.NewRouter()

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
	r.Get("/products/{publicID}", productHandler.GetByPublicID)
}

func registerStoreRoutes(r *chi.Mux, db *pgxpool.Pool) {
	storeRepository := stores.NewRepository(db)
	storeHandler := stores.NewHandler(storeRepository)
	r.Get("/stores", storeHandler.List)
	r.Get("/stores/{id}", storeHandler.Get)
}

func registerServiceRoutes(r *chi.Mux, db *pgxpool.Pool) {
	serviceRepository := services.NewRepository(db)
	serviceHandler := services.NewHandler(serviceRepository)

	r.Get("/services", serviceHandler.List)
	r.Get("/services/{publicID}", serviceHandler.GetByPublicID)

	storeRepository := stores.NewRepository(db)
	storeHandler := stores.NewHandler(storeRepository)

	r.Get("/stores", storeHandler.List)

	return r
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
