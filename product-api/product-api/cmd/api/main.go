package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/product-api/internal/api"
	"github.com/example/product-api/internal/config"
)

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Conexión a PostgreSQL: pendiente de credenciales reales.
	// Se arma desde variables de entorno (ver .env.example / config.go).
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL())
	if err != nil {
		log.Fatalf("no se pudo crear el pool de conexión: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Printf("advertencia: no fue posible conectar a la base de datos todavía: %v", err)
		log.Printf("la API arrancará igual; revisa DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME en el .env")
	} else {
		log.Println("conexión a PostgreSQL establecida correctamente")
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.Timeout(15 * time.Second))

	h := api.NewHandler(pool)

	r.Route("/api/v1", func(r chi.Router) {
		h.Routes(r)
	})

	addr := ":" + cfg.Port
	log.Printf("product-api escuchando en %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("error del servidor HTTP: %v", err)
	}
}
