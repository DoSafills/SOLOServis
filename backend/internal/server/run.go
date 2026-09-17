package server

import (
	"context"
	"log"
	"net/http"
	"time"

	appconfig "github.com/DoSafills/SOLOServis/backend/internal/config"
	"github.com/DoSafills/SOLOServis/backend/internal/database"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RouterFactory func(*pgxpool.Pool) *chi.Mux

func Run(serviceName string, defaultPort string, factory RouterFactory) {
	cfg := appconfig.LoadForPort(defaultPort)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := database.NewPostgresPool(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := database.Ping(ctx, db); err != nil {
		log.Fatal(err)
	}

	router := factory(db)
	server := &http.Server{Addr: ":" + cfg.Port, Handler: router}
	log.Printf("%s API running on http://localhost:%s", serviceName, cfg.Port)

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}