package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/config"
	"github.com/DoSafills/SOLOServis/backend/internal/database"
	apphttp "github.com/DoSafills/SOLOServis/backend/internal/http"
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

	router := apphttp.NewUserRouter(db, cfg.FrontendURL)
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	log.Printf("User API running on http://localhost:%s", cfg.Port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
