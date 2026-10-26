package features

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(name string, db *pgxpool.Pool, frontendURL string) *chi.Mux {
	h := NewHandler(name, db)
	r := chi.NewRouter()
	r.Use(cors(frontendURL))
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
		r.Get("/reviews/product/{productId}", h.ListProductReviews)
		r.Post("/reviews/product/{productId}", h.CreateProductReview)
		r.Get("/reviews/store/{storeId}", h.ListStoreReviews)
		r.Post("/reviews/store/{storeId}", h.CreateStoreReview)
		r.Get("/reviews/service/{serviceId}", h.ListServiceReviews)
		r.Post("/reviews/service/{serviceId}", h.CreateServiceReview)
		r.Put("/reviews/{id}", h.UpdateReview)
		r.Delete("/reviews/{id}", h.DeleteReview)
	case "watchlist-api":
		r.Get("/watchlist", h.GetWatchlist)
		r.Post("/watchlist", h.AddWatchlist)
		r.Put("/watchlist/{productId}", h.UpdateWatchlist)
		r.Delete("/watchlist/{productId}", h.DeleteWatchlist)
	case "notification-api":
		r.Post("/notifications", h.CreateNotification)
		r.Get("/notifications", h.GetNotifications)
		r.Get("/notifications/unread", h.GetUnreadNotifications)
		r.Put("/notifications/{id}/read", h.ReadNotification)
		r.Put("/notifications/read-all", h.ReadAllNotifications)
	case "ingestion-api":
		r.Post("/ingestion/products", h.IngestProduct)
		r.Post("/ingestion/offers", h.IngestOffer)
		r.Post("/ingestion/sync", h.SyncIngestion)
	}
	return r
}

// RegisterSearchSessionRoutes monta el historial y las búsquedas guardadas
// (requieren sesión) en el router de Search API (TAL-75).
func RegisterSearchSessionRoutes(r chi.Router, db *pgxpool.Pool) {
	registerSearchSessionRoutes(r, NewHandler("search-api", db))
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
func RegisterServiceComparisonRoutes(r chi.Router, db *pgxpool.Pool) {
	registerServiceComparisonRoutes(r, NewHandler("comparison-api", db))
}

func registerServiceComparisonRoutes(r chi.Router, h *Handler) {
	r.Post("/comparisons/services", h.CreateServiceComparison)
	r.Get("/comparisons/services/{id}", h.GetServiceComparison)
	r.Delete("/comparisons/services/{id}", h.DeleteServiceComparison)
	r.Post("/comparisons/services/{id}/items", h.AddServiceComparisonItem)
	r.Delete("/comparisons/services/{id}/items/{serviceId}", h.DeleteServiceComparisonItem)
}

func cors(frontendURL string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", frontendURL)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
