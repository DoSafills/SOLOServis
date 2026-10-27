package main

import (
	apphttp "github.com/DoSafills/SOLOServis/backend/internal/http"
	"github.com/DoSafills/SOLOServis/backend/internal/server"
)

func main() {
	server.Run("Products", "8081", apphttp.NewProductsRouter)
}