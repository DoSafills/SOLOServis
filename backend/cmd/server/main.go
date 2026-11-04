package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/allapis"
	"github.com/DoSafills/SOLOServis/backend/internal/config"
	"github.com/DoSafills/SOLOServis/backend/internal/database"
	"github.com/DoSafills/SOLOServis/backend/internal/server"
)

func main() {
	cfg := config.Load()

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

	router := allapis.NewRouter(db, cfg.FrontendURL)

	httpServer := server.NewHTTPServer(":"+cfg.Port, router)

	log.Printf("SoloServis API running on http://localhost:%s", cfg.Port)

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
