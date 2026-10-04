package watchlist

import (
	"github.com/DoSafills/SOLOServis/backend/internal/features"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(db *pgxpool.Pool, frontendURL string) *chi.Mux {
	return features.NewRouter("watchlist-api", db, frontendURL)
}
