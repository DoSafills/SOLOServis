package http

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/apiutil"
	appconfig "github.com/DoSafills/SOLOServis/backend/internal/config"
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

func corsMiddleware(frontendURL string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", frontendURL)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if req.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, req)
		})
	}
}

func NewProductsRouter(db *pgxpool.Pool) *chi.Mux {
	return NewProductRouter(db, appconfig.LoadForPort("8081").FrontendURL)
}

func NewServicesRouter(db *pgxpool.Pool) *chi.Mux {
	return NewServiceRouter(db, appconfig.LoadForPort("8083").FrontendURL)
}

func NewStoresRouter(db *pgxpool.Pool) *chi.Mux {
	return NewStoreRouter(db, appconfig.LoadForPort("8082").FrontendURL)
}

func NewProductRouter(db *pgxpool.Pool, frontendURL string) *chi.Mux {
	r := chi.NewRouter()
	r.Use(corsMiddleware(frontendURL))
	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		healthHandler(w, db)
	})

	productRepository := products.NewRepository(db)
	productHandler := products.NewHandler(productRepository)

	r.Get("/products", productHandler.List)
	r.Get("/products/{publicID}", productHandler.GetByPublicID)
	r.Post("/products", productHandler.Create)
	r.Put("/products/{publicID}", productHandler.Update)
	r.Delete("/products/{publicID}", productHandler.Deactivate)

	return r
}

func NewServiceRouter(db *pgxpool.Pool, frontendURL string) *chi.Mux {
	r := chi.NewRouter()
	r.Use(corsMiddleware(frontendURL))
	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		healthHandler(w, db)
	})

	serviceRepository := services.NewRepository(db)
	serviceHandler := services.NewHandler(serviceRepository)

	r.Get("/services", serviceHandler.List)
	r.Get("/services/{publicID}", serviceHandler.GetByPublicID)

	return r
}

func NewStoreRouter(db *pgxpool.Pool, frontendURL string) *chi.Mux {
	r := chi.NewRouter()
	r.Use(corsMiddleware(frontendURL))
	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		healthHandler(w, db)
	})

	storeRepository := stores.NewRepository(db)
	storeHandler := stores.NewHandler(storeRepository)

	r.Get("/stores", storeHandler.List)
	r.Get("/stores/{id}", storeHandler.GetByID)

	return r
}

func NewUserRouter(db *pgxpool.Pool, frontendURL string) *chi.Mux {
	r := chi.NewRouter()
	r.Use(corsMiddleware(frontendURL))
	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		healthHandler(w, db)
	})
	r.Get("/users", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "user-api-ready",
			"note":   "User profile endpoints are available.",
		})
	})
	r.Get("/users/{id}", func(w http.ResponseWriter, req *http.Request) {
		token := userBearerToken(req)
		if token == "" {
			apiutil.WriteError(w, http.StatusUnauthorized, "bearer token is required")
			return
		}
		tokenHash := sha256.Sum256([]byte(token))
		var user struct {
			ID    int32  `json:"id"`
			Name  string `json:"name"`
			Email string `json:"email"`
		}
		if err := db.QueryRow(req.Context(), `SELECT u.id,u.name,u.email FROM user_account u
			JOIN user_session s ON s.user_id=u.id
			WHERE u.id=$1 AND u.active AND s.access_token_hash=$2 AND s.access_expires_at>NOW()`,
			chi.URLParam(req, "id"), tokenHash[:]).Scan(&user.ID, &user.Name, &user.Email); err != nil {
			apiutil.WriteError(w, http.StatusNotFound, "user not found")
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, user)
	})
	return r
}

func userBearerToken(req *http.Request) string {
	value := req.Header.Get("Authorization")
	if len(value) > 7 && strings.EqualFold(value[:7], "Bearer ") {
		return strings.TrimSpace(value[7:])
	}
	return ""
}

func NewRouter(db *pgxpool.Pool, frontendURL string) *chi.Mux {
	r := chi.NewRouter()
	r.Use(corsMiddleware(frontendURL))

	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		healthHandler(w, db)
	})

	productRepository := products.NewRepository(db)
	productHandler := products.NewHandler(productRepository)

	r.Get("/products", productHandler.List)
	r.Get("/products/{publicID}", productHandler.GetByPublicID)
	r.Post("/products", productHandler.Create)
	r.Put("/products/{publicID}", productHandler.Update)
	r.Delete("/products/{publicID}", productHandler.Deactivate)

	serviceRepository := services.NewRepository(db)
	serviceHandler := services.NewHandler(serviceRepository)

	r.Get("/services", serviceHandler.List)
	r.Get("/services/{publicID}", serviceHandler.GetByPublicID)

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
			Database: "disconnected",
		})
		return
	}

	_ = json.NewEncoder(w).Encode(HealthResponse{
		Status:   "ok",
		Service:  "soloservis-api",
		Database: "connected",
	})
}
