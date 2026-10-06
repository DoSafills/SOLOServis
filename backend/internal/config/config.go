package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DatabaseURL string
	FrontendURL string
}

// Load es para el servidor combinado (cmd/server): respeta PORT del .env,
// con 8080 como default si no está seteada.
func Load() Config {
	_ = godotenv.Load(".env")

	return LoadForPort(envOr("PORT", "8080"))
}

// LoadForPort es para los binarios de microservicio (products-api, stores-api,
// services-api): cada uno usa siempre su propio puerto por defecto, sin que
// la variable PORT compartida del .env se lo pise — si no, los tres terminan
// compitiendo por el mismo puerto.
func LoadForPort(port string) Config {
	_ = godotenv.Load(".env")

	return Config{
		Port:        port,
		DatabaseURL: envOr("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/soloservis"),
		FrontendURL: envOr("FRONTEND_URL", "http://localhost:5173"),
	}
}

func envOr(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
