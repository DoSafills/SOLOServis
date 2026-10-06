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

type RouterFactory func(db *pgxpool.Pool, frontendURL string) *chi.Mux

// Run conecta a PostgreSQL y levanta el servidor HTTP. Lo usan tanto el
// servidor combinado (cmd/server) como los microservicios.
func Run(serviceName string, cfg appconfig.Config, factory RouterFactory) {
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

	log.Println("PostgreSQL connection established")

	server := &http.Server{Addr: ":" + cfg.Port, Handler: factory(db, cfg.FrontendURL)}
	log.Printf("%s API running on http://localhost:%s", serviceName, cfg.Port)

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
