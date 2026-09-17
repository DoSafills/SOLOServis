package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DatabaseURL string
}

func Load() Config {
	_ = godotenv.Load(".env")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return Config{
		Port:        port,
		DatabaseURL: databaseURL(),
	}
}

func LoadForPort(defaultPort string) Config {
	_ = godotenv.Load(".env")

	return Config{
		Port:        defaultPort,
		DatabaseURL: databaseURL(),
	}
}

func databaseURL() string {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://postgres:postgres@localhost:5432/soloservis"
	}

	return url
}
