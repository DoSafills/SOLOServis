package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/products"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type HealthResponse struct {
	Status   string `json:"status"`
	Service  string `json:"service"`
	Database string `json:"database"`
}

func NewRouter(db *pgxpool.Pool) *chi.Mux {
	r := chi.NewRouter()
	addCORS(r)

	registerHealth(r, db)
	registerProductRoutes(r, db)

	return r
}

func NewProductsRouter(db *pgxpool.Pool) *chi.Mux {
	r := chi.NewRouter()
	addCORS(r)
	registerHealth(r, db)
	registerProductRoutes(r, db)
	return r
}

func NewStoresRouter(db *pgxpool.Pool) *chi.Mux {
	r := chi.NewRouter()
	addCORS(r)
	registerHealth(r, db)
	return r
}

func NewServicesRouter(db *pgxpool.Pool) *chi.Mux {
	r := chi.NewRouter()
	addCORS(r)
	registerHealth(r, db)
	return r
}

func addCORS(r *chi.Mux) {

	// CORS
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "http://localhost:8443")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
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

	r.Get("/products", productHandler.List)
	r.Get("/products/{publicID}", productHandler.GetByPublicID)
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
