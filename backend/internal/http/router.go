package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/brands"
	"github.com/DoSafills/SOLOServis/backend/internal/categories"
	"github.com/DoSafills/SOLOServis/backend/internal/offers"
	"github.com/DoSafills/SOLOServis/backend/internal/pricehistory"
	"github.com/DoSafills/SOLOServis/backend/internal/products"
	"github.com/DoSafills/SOLOServis/backend/internal/stores"
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

	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		healthHandler(w, db)
	})

	productRepository := products.NewRepository(db)
	productHandler := products.NewHandler(productRepository)

	r.Get("/products", productHandler.List)
	r.Get("/products/{publicID}", productHandler.GetByPublicID)

	brandRepository := brands.NewRepository(db)
	brandHandler := brands.NewHandler(brandRepository)

	r.Get("/brands", brandHandler.List)
	r.Get("/brands/{id}", brandHandler.GetByID)

	categoryRepository := categories.NewRepository(db)
	categoryHandler := categories.NewHandler(categoryRepository)

	r.Get("/categories", categoryHandler.List)
	r.Get("/categories/{id}", categoryHandler.GetByID)

	offerRepository := offers.NewRepository(db)
	offerHandler := offers.NewHandler(offerRepository)

	r.Get("/product-offers", offerHandler.List)
	r.Get("/product-offers/{id}", offerHandler.GetByID)

	priceHistoryRepository := pricehistory.NewRepository(db)
	priceHistoryHandler := pricehistory.NewHandler(priceHistoryRepository)

	r.Get("/price-history/{offerId}", priceHistoryHandler.ListByOfferID)

	storeRepository := stores.NewRepository(db)
	storeHandler := stores.NewHandler(storeRepository)

	r.Get("/stores", storeHandler.List)
	r.Get("/stores/{id}", storeHandler.GetByID)

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
