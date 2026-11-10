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

// Load lee PORT (8080 si no está), DATABASE_URL y FRONTEND_URL del entorno o
// del .env.
func Load() Config {
	_ = godotenv.Load(".env")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return Config{
		Port:        port,
		DatabaseURL: resolveDatabaseURL(),
		FrontendURL: resolveFrontendURL(),
	}
}

func resolveDatabaseURL() string {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://postgres:postgres@localhost:5432/soloservis"
	}

	return databaseURL
}

func resolveFrontendURL() string {
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:8443"
	}

	return frontendURL
}
