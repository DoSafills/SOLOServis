// all-apis sirve las 14 APIs en un solo proceso (ver internal/allapis).
// Las llamadas entre APIs (PRODUCT_API_URL, PRICING_API_URL, ...) apuntan al
// mismo proceso salvo que se definan explícitamente.
package main

import (
	"os"

	"github.com/DoSafills/SOLOServis/backend/internal/allapis"
	"github.com/DoSafills/SOLOServis/backend/internal/runner"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	self := "http://127.0.0.1:" + port
	for _, name := range []string{"PRODUCT_API_URL", "STORE_API_URL", "SERVICE_API_URL", "PRICING_API_URL", "NOTIFICATION_API_URL"} {
		if os.Getenv(name) == "" {
			_ = os.Setenv(name, self)
		}
	}
	runner.Run("All APIs", allapis.NewRouter)
}
