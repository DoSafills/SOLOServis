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
	case "pricing-api":
		r.Get("/offers/product/{productId}", h.OffersByProduct)
		r.Get("/offers/{id}/price-history", h.PriceHistory)
		r.Get("/offers/{id}", h.GetOffer)
		r.Post("/offers", h.CreateOffer)
		r.Put("/offers/{id}", h.UpdateOffer)
	case "search-api":
		r.Get("/search/products", h.SearchProducts)
		r.Get("/search/categories", h.SearchCategories)
		r.Get("/search/brands", h.SearchBrands)
		r.Group(func(r chi.Router) {
			r.Use(h.requireSession)
			r.Post("/search/history", h.CreateSearchHistory)
			r.Get("/search/history", h.GetSearchHistory)
			r.Post("/search/saved", h.SaveSearch)
			r.Get("/search/saved", h.GetSavedSearches)
			r.Delete("/search/saved/{id}", h.DeleteSavedSearch)
		})
	case "comparison-api":
		r.Post("/comparisons/products", h.CreateComparison)
		r.Get("/comparisons/products/{id}", h.GetComparison)
		r.Put("/comparisons/products/{id}", h.UpdateComparison)
		r.Delete("/comparisons/products/{id}", h.DeleteComparison)
		r.Post("/comparisons/products/{id}/items", h.AddComparisonItem)
		r.Delete("/comparisons/products/{id}/items/{productId}", h.DeleteComparisonItem)
		r.Get("/comparisons/compatible/{productId}", h.CompatibleProducts)
		r.Post("/comparisons/services", h.CreateServiceComparison)
		r.Get("/comparisons/services/{id}", h.GetServiceComparison)
		r.Delete("/comparisons/services/{id}", h.DeleteServiceComparison)
		r.Post("/comparisons/services/{id}/items", h.AddServiceComparisonItem)
		r.Delete("/comparisons/services/{id}/items/{serviceId}", h.DeleteServiceComparisonItem)
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
