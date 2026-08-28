package config

import (
	"fmt"
	"os"
)

// Config agrupa la configuración de la aplicación, leída desde variables
// de entorno. La conexión real a la base de datos se establece en main.go
// usando DatabaseURL(); mientras no se provea, la app no arranca.
type Config struct {
	Port       string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string
}

func Load() Config {
	return Config{
		Port:       getEnv("PORT", "8080"),
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "postgres"),
		DBName:     getEnv("DB_NAME", "product_api"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),
	}
}

// DatabaseURL arma el DSN de conexión a PostgreSQL a partir de las
// variables de entorno cargadas.
func (c Config) DatabaseURL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName, c.DBSSLMode,
	)
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
