package http

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/apiutil"
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

type routeRegistrar func(chi.Router, *pgxpool.Pool)

func NewRouter(db *pgxpool.Pool, frontendURL string) *chi.Mux {
	return newRouter(db, frontendURL, registerProductRoutes, registerStoreRoutes, registerServiceRoutes, registerUserRoutes)
}

func NewProductsRouter(db *pgxpool.Pool) *chi.Mux {
	return newRouter(db, configuredFrontendURL(), registerProductRoutes)
}

func NewStoresRouter(db *pgxpool.Pool) *chi.Mux {
	return newRouter(db, configuredFrontendURL(), registerStoreRoutes)
}

func NewServicesRouter(db *pgxpool.Pool) *chi.Mux {
	return newRouter(db, configuredFrontendURL(), registerServiceRoutes)
}

func NewProductRouter(db *pgxpool.Pool, frontendURL string) *chi.Mux {
	return newRouter(db, frontendURL, registerProductRoutes)
}

func NewServiceRouter(db *pgxpool.Pool, frontendURL string) *chi.Mux {
	return newRouter(db, frontendURL, registerServiceRoutes)
}

func NewStoreRouter(db *pgxpool.Pool, frontendURL string) *chi.Mux {
	return newRouter(db, frontendURL, registerStoreRoutes)
}

func NewUserRouter(db *pgxpool.Pool, frontendURL string) *chi.Mux {
	return newRouter(db, frontendURL, registerUserRoutes)
}

func newRouter(db *pgxpool.Pool, frontendURL string, registrars ...routeRegistrar) *chi.Mux {
	r := chi.NewRouter()
	r.Use(corsMiddleware(frontendURL))
	registerHealth(r, db)
	for _, register := range registrars {
		register(r, db)
	}
	return r
}

func configuredFrontendURL() string {
	if value := os.Getenv("FRONTEND_URL"); value != "" {
		return value
	}
	return "http://localhost:5173"
}

func corsMiddleware(frontendURL string) func(http.Handler) http.Handler {
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

func registerHealth(r chi.Router, db *pgxpool.Pool) {
	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		healthHandler(w, db)
	})
}

func registerProductRoutes(r chi.Router, db *pgxpool.Pool) {
	handler := products.NewHandler(products.NewRepository(db))
	r.Get("/categories", handler.ListCategories)
	r.Get("/products", handler.List)
	r.Get("/products/{publicID}", handler.GetByPublicID)
	r.Post("/products", handler.Create)
	r.Put("/products/{publicID}", handler.Update)
	r.Delete("/products/{publicID}", handler.Deactivate)
	r.Get("/products/{publicID}/reviews", handler.ListReviews)
}

func registerStoreRoutes(r chi.Router, db *pgxpool.Pool) {
	handler := stores.NewHandler(stores.NewRepository(db))
	r.Get("/stores", handler.List)
	r.Get("/stores/{id}", handler.GetByID)
}

func registerServiceRoutes(r chi.Router, db *pgxpool.Pool) {
	handler := services.NewHandler(services.NewRepository(db))
	r.Get("/services", handler.List)
	r.Get("/services/{publicID}", handler.GetByPublicID)
}

func registerUserRoutes(r chi.Router, db *pgxpool.Pool) {
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
		err := db.QueryRow(req.Context(), `SELECT u.id,u.name,u.email FROM user_account u
			JOIN user_session s ON s.user_id=u.id
			WHERE u.id=$1 AND u.active AND s.access_token_hash=$2 AND s.access_expires_at>NOW()`,
			chi.URLParam(req, "id"), tokenHash[:]).Scan(&user.ID, &user.Name, &user.Email)
		if err != nil {
			apiutil.WriteError(w, http.StatusNotFound, "user not found")
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, user)
	})
}

func userBearerToken(req *http.Request) string {
	value := req.Header.Get("Authorization")
	if len(value) > 7 && strings.EqualFold(value[:7], "Bearer ") {
		return strings.TrimSpace(value[7:])
	}
	return ""
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
