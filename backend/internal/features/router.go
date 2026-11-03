package features

import (
	"log/slog"
	"os"

	"github.com/DoSafills/SOLOServis/backend/internal/authz"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(name string, db *pgxpool.Pool, frontendURL string) *chi.Mux {
	// Estándar TAL-78, en el mismo orden que las demás APIs
	// (internal/http.useStandardMiddleware): RequestID → AccessLog → Recover.
	logger := httpx.NewLogger(os.Stdout, httpx.ParseLevel(os.Getenv("LOG_LEVEL")))
	h := NewHandler(name, db, logger)
	r := chi.NewRouter()
	r.Use(httpx.RequestID, httpx.AccessLog(logger), httpx.Recover(logger))
	r.Use(httpx.CORS(frontendURL))
	r.Get("/health", h.Health)

	switch name {
	// pricing-api se movió a internal/pricing (TAL-74).
	case "search-api":
		// La búsqueda pública se movió a internal/search (TAL-75).
		registerSearchSessionRoutes(r, h)
	case "comparison-api":
		// Las comparaciones de productos se movieron a internal/comparison (TAL-76).
		registerServiceComparisonRoutes(r, h)
	case "auth-api":
		r.Post("/auth/register", h.Register)
		r.Post("/auth/login", h.Login)
		r.Post("/auth/refresh", h.Refresh)
		r.Post("/auth/logout", h.Logout)
		r.Get("/auth/validate", h.ValidateToken)
	case "favorites-api":
		r.Group(func(r chi.Router) {
			r.Use(h.requireSession)
			r.Get("/favorites", h.ListFavorites)
			r.Post("/favorites/{productId}", h.AddFavorite)
			r.Delete("/favorites/{productId}", h.DeleteFavorite)
			r.Get("/favorites/{productId}", h.GetFavorite)
			r.Get("/favorites/services", h.ListServiceFavorites)
			r.Post("/favorites/services/{serviceId}", h.AddServiceFavorite)
			r.Delete("/favorites/services/{serviceId}", h.DeleteServiceFavorite)
		})
	case "cart-api":
		r.Group(func(r chi.Router) {
			r.Use(h.requireSession)
			r.Get("/cart", h.GetCart)
			r.Post("/cart/items", h.AddCartItem)
			r.Put("/cart/items/{id}", h.UpdateCartItem)
			r.Delete("/cart/items/{id}", h.DeleteCartItem)
			r.Delete("/cart", h.ClearCart)
			r.Get("/cart/count", h.CartCount)
		})
	case "review-api":
		// Leer es público; escribir exige sesión y el autor sale del token.
		r.Get("/reviews/product/{productId}", h.ListProductReviews)
		r.Get("/reviews/store/{storeId}", h.ListStoreReviews)
		r.Get("/reviews/service/{serviceId}", h.ListServiceReviews)
		r.Group(func(r chi.Router) {
			r.Use(h.requireSession)
			r.Post("/reviews/product/{productId}", h.CreateProductReview)
			r.Post("/reviews/store/{storeId}", h.CreateStoreReview)
			r.Post("/reviews/service/{serviceId}", h.CreateServiceReview)
			r.Put("/reviews/{id}", h.UpdateReview)
			r.Delete("/reviews/{id}", h.DeleteReview)
		})
	case "watchlist-api":
		r.Group(func(r chi.Router) {
			r.Use(h.requireSession)
			r.Get("/watchlist", h.GetWatchlist)
			r.Post("/watchlist", h.AddWatchlist)
			r.Put("/watchlist/{productId}", h.UpdateWatchlist)
			r.Delete("/watchlist/{productId}", h.DeleteWatchlist)
		})
	case "notification-api":
		// Crear es interno: lo llaman pricing e ingestion con INTERNAL_API_TOKEN.
		admin := authz.New(authz.NewPostgresSessions(db), os.Getenv("INTERNAL_API_TOKEN"), logger).RequireAdmin
		r.With(admin).Post("/notifications", h.CreateNotification)
		r.Group(func(r chi.Router) {
			r.Use(h.requireSession)
			r.Get("/notifications", h.GetNotifications)
			r.Get("/notifications/unread", h.GetUnreadNotifications)
			r.Put("/notifications/{id}/read", h.ReadNotification)
			r.Put("/notifications/read-all", h.ReadAllNotifications)
		})
		// ingestion-api se movió a internal/ingestion (TAL-77).
	}
	return r
}

// RegisterSearchSessionRoutes monta el historial y las búsquedas guardadas
// (requieren sesión) en el router de Search API (TAL-75).
func RegisterSearchSessionRoutes(r chi.Router, db *pgxpool.Pool, logger *slog.Logger) {
	registerSearchSessionRoutes(r, NewHandler("search-api", db, logger))
}

func registerSearchSessionRoutes(r chi.Router, h *Handler) {
	r.Group(func(r chi.Router) {
		r.Use(h.requireSession)
		r.Post("/search/history", h.CreateSearchHistory)
		r.Get("/search/history", h.GetSearchHistory)
		r.Post("/search/saved", h.SaveSearch)
		r.Get("/search/saved", h.GetSavedSearches)
		r.Delete("/search/saved/{id}", h.DeleteSavedSearch)
	})
}

// RegisterServiceComparisonRoutes monta las comparaciones de servicios en el
// router de Comparison API (TAL-76).
func RegisterServiceComparisonRoutes(r chi.Router, db *pgxpool.Pool, logger *slog.Logger) {
	registerServiceComparisonRoutes(r, NewHandler("comparison-api", db, logger))
}

// Cada comparación es de un usuario: solo su dueño la ve o la modifica; la de
// otro responde 404.
func registerServiceComparisonRoutes(r chi.Router, h *Handler) {
	r.Group(func(r chi.Router) {
		r.Use(h.requireSession)
		r.Post("/comparisons/services", h.CreateServiceComparison)
		r.Get("/comparisons/services/{id}", h.GetServiceComparison)
		r.Delete("/comparisons/services/{id}", h.DeleteServiceComparison)
		r.Post("/comparisons/services/{id}/items", h.AddServiceComparisonItem)
		r.Delete("/comparisons/services/{id}/items/{serviceId}", h.DeleteServiceComparisonItem)
	})
}
