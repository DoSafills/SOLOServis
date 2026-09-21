package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DatabaseURL string
}

// Load es para el servidor combinado (cmd/server): respeta PORT del .env,
// con 8080 como default si no está seteada.
func Load() Config {
	_ = godotenv.Load(".env")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return Config{
		Port:        port,
		DatabaseURL: resolveDatabaseURL(),
	}
}

// LoadForPort es para los binarios de microservicio (products-api, stores-api,
// services-api): cada uno usa siempre su propio puerto por defecto, sin que
// la variable PORT compartida del .env se lo pise — si no, los tres terminan
// compitiendo por el mismo puerto.
func LoadForPort(defaultPort string) Config {
	_ = godotenv.Load(".env")

	return Config{
		Port:        defaultPort,
		DatabaseURL: resolveDatabaseURL(),
	}
}

func resolveDatabaseURL() string {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://postgres:postgres@localhost:5432/soloservis"
	}
	return databaseURL
}
