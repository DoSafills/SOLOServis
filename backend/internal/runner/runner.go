package runner

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/config"
	"github.com/DoSafills/SOLOServis/backend/internal/database"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RouterFactory func(*pgxpool.Pool, string) *chi.Mux

func Run(name string, factory RouterFactory) {
	cfg := config.Load()
	db, err := database.NewPostgresPool(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := database.Ping(ctx, db); err != nil {
		log.Fatal(err)
	}
	server := &http.Server{Addr: ":" + cfg.Port, Handler: factory(db, cfg.FrontendURL)}
	log.Printf("%s running on http://localhost:%s", name, cfg.Port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
