package http

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/apiutil"
	"github.com/DoSafills/SOLOServis/backend/internal/authz"
	"github.com/DoSafills/SOLOServis/backend/internal/categories"
	"github.com/DoSafills/SOLOServis/backend/internal/comparison"
	"github.com/DoSafills/SOLOServis/backend/internal/features"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/DoSafills/SOLOServis/backend/internal/ingestion"
	"github.com/DoSafills/SOLOServis/backend/internal/pricing"
	"github.com/DoSafills/SOLOServis/backend/internal/products"
	"github.com/DoSafills/SOLOServis/backend/internal/search"
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

// routeRegistrar monta las rutas de un servicio. Recibe el logger del
// estándar TAL-78 para que los handlers registren sus errores 5xx.
type routeRegistrar func(chi.Router, *pgxpool.Pool, *slog.Logger)

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

// NewPricingRouter es el router de Pricing API (TAL-74); cmd/pricing-api lo usa.
func NewPricingRouter(db *pgxpool.Pool, frontendURL string) *chi.Mux {
	return newRouter(db, frontendURL, registerPricingRoutes)
}

// NewSearchRouter es el router de Search API (TAL-75); cmd/search-api lo usa.
func NewSearchRouter(db *pgxpool.Pool, frontendURL string) *chi.Mux {
	return newRouter(db, frontendURL, registerSearchRoutes)
}

// NewComparisonRouter es el router de Comparison API (TAL-76); cmd/comparison-api lo usa.
func NewComparisonRouter(db *pgxpool.Pool, frontendURL string) *chi.Mux {
	return newRouter(db, frontendURL, registerComparisonRoutes)
}

// NewIngestionRouter es el router de Ingestion API (TAL-77); cmd/ingestion-api lo usa.
func NewIngestionRouter(db *pgxpool.Pool, frontendURL string) *chi.Mux {
	return newRouter(db, frontendURL, registerIngestionRoutes)
}

// adminOnly protege escrituras: sesión con rol admin o INTERNAL_API_TOKEN (TAL-70).
func adminOnly(db *pgxpool.Pool, logger *slog.Logger) func(http.Handler) http.Handler {
	return authz.New(authz.NewPostgresSessions(db), os.Getenv("INTERNAL_API_TOKEN"), logger).RequireAdmin
}

func newRouter(db *pgxpool.Pool, frontendURL string, registrars ...routeRegistrar) *chi.Mux {
	r := chi.NewRouter()
	logger := useStandardMiddleware(r)
	r.Use(corsMiddleware(frontendURL))
	registerHealth(r, db)
	for _, register := range registrars {
		register(r, db, logger)
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

// useStandardMiddleware aplica el estándar TAL-78: RequestID → AccessLog → Recover.
// Debe registrarse antes que cualquier otro middleware. Devuelve el logger
// para que los handlers registren sus errores 5xx (httpx.WriteInternal).
func useStandardMiddleware(r chi.Router) *slog.Logger {
	logger := httpx.NewLogger(os.Stdout, httpx.ParseLevel(os.Getenv("LOG_LEVEL")))
	r.Use(httpx.RequestID, httpx.AccessLog(logger), httpx.Recover(logger))
	return logger
}

func registerHealth(r chi.Router, db *pgxpool.Pool) {
	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		healthHandler(w, db)
	})
}

func registerProductRoutes(r chi.Router, db *pgxpool.Pool, logger *slog.Logger) {
	handler := products.NewHandler(products.NewRepository(db), logger)
	// Escrituras (TAL-70): solo admin o servicios internos con INTERNAL_API_TOKEN.
	admin := adminOnly(db, logger)

	// Árbol de product_category (modelo anterior). Transitorio: el frontend
	// debe migrar a /categories (TAL-51) y luego se elimina.
	r.Get("/product-categories", handler.ListCategories)
	r.Get("/products", handler.List)
	r.Get("/products/{publicID}", handler.GetByPublicID)
	r.With(admin).Post("/products", handler.Create)
	r.With(admin).Put("/products/{publicID}", handler.Update)
	r.With(admin).Delete("/products/{publicID}", handler.Deactivate)
	r.Get("/products/{publicID}/reviews", handler.ListReviews)

	// Categorías y definiciones de atributos (TAL-71): solo lectura.
	categoryHandler := categories.NewHandler(categories.NewRepository(db), logger)
	r.Get("/categories", categoryHandler.List)
	r.Get("/categories/{slug}", categoryHandler.GetBySlug)
}

func registerStoreRoutes(r chi.Router, db *pgxpool.Pool, logger *slog.Logger) {
	handler := stores.NewHandler(stores.NewRepository(db), logger)
	r.Get("/stores", handler.List)
	// Ofertas de un producto en todas las tiendas (TAL-73). Ruta estática:
	// chi la prioriza sobre /stores/{id}.
	r.Get("/stores/offers", handler.ListProductOffers)
	r.Get("/stores/{id}", handler.GetByID)
	r.Get("/stores/{id}/offers", handler.ListStoreOffers)
}

func registerPricingRoutes(r chi.Router, db *pgxpool.Pool, logger *slog.Logger) {
	handler := pricing.NewHandler(pricing.NewRepository(db), pricing.NewHTTPNotifier(), logger)
	admin := adminOnly(db, logger)

	r.Get("/offers/product/{productId}", handler.ListProductOffers)
	r.Get("/offers/product/{productId}/best", handler.BestOffer)
	r.Get("/offers/product/{productId}/price-history", handler.ProductHistory)
	r.Get("/offers/{id}", handler.GetOffer)
	r.Get("/offers/{id}/price-history", handler.OfferHistory)
	r.With(admin).Post("/offers", handler.CreateOffer)
	r.With(admin).Put("/offers/{id}", handler.UpdateOffer)
}

func registerSearchRoutes(r chi.Router, db *pgxpool.Pool, logger *slog.Logger) {
	handler := search.NewHandler(search.NewRepository(db), logger)
	r.Get("/search/products", handler.Products)
	r.Get("/search/facets", handler.Facets)
	r.Get("/search/categories", handler.Categories)
	r.Get("/search/brands", handler.Brands)
	// Historial y búsquedas guardadas (requieren sesión): siguen en features.
	features.RegisterSearchSessionRoutes(r, db)
}

func registerComparisonRoutes(r chi.Router, db *pgxpool.Pool, logger *slog.Logger) {
	handler := comparison.NewHandler(comparison.NewRepository(db), logger)
	session := authz.New(authz.NewPostgresSessions(db), "", logger).RequireSession

	// Públicos: comparar sin guardar y buscar alternativas compatibles.
	r.Get("/comparisons/products/compare", handler.Compare)
	r.Get("/comparisons/compatible/{productId}", handler.Compatible)

	// Comparaciones guardadas: el usuario sale de la sesión (corrige S3).
	r.Group(func(r chi.Router) {
		r.Use(session)
		r.Post("/comparisons/products", handler.Create)
		r.Get("/comparisons/products", handler.List)
		r.Get("/comparisons/products/{id}", handler.Get)
		r.Put("/comparisons/products/{id}", handler.Rename)
		r.Delete("/comparisons/products/{id}", handler.Delete)
		r.Post("/comparisons/products/{id}/items", handler.AddItem)
		r.Delete("/comparisons/products/{id}/items/{productId}", handler.RemoveItem)
	})

	// Comparaciones de servicios: siguen en features (fuera de TAL-76).
	features.RegisterServiceComparisonRoutes(r, db)
}

func registerIngestionRoutes(r chi.Router, db *pgxpool.Pool, logger *slog.Logger) {
	runs := ingestion.NewRepository(db)
	processor := ingestion.NewProcessor(runs, products.NewRepository(db), pricing.NewRepository(db),
		pricing.NewHTTPNotifier(), logger)
	handler := ingestion.NewHandler(runs, processor, logger)

	// Solo el scraper (INTERNAL_API_TOKEN) o un admin.
	r.Group(func(r chi.Router) {
		r.Use(adminOnly(db, logger))
		r.Post("/ingestion/runs", handler.OpenRun)
		r.Get("/ingestion/runs", handler.ListRuns)
		r.Get("/ingestion/runs/{id}", handler.GetRun)
		r.Post("/ingestion/runs/{id}/items", handler.AddItems)
		r.Post("/ingestion/runs/{id}/finish", handler.FinishRun)
	})
}

func registerServiceRoutes(r chi.Router, db *pgxpool.Pool, _ *slog.Logger) {
	handler := services.NewHandler(services.NewRepository(db))
	r.Get("/services", handler.List)
	r.Get("/services/{publicID}", handler.GetByPublicID)
}

func registerUserRoutes(r chi.Router, db *pgxpool.Pool, _ *slog.Logger) {
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
