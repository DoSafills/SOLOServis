package main

import (
	"github.com/DoSafills/SOLOServis/backend/internal/config"
	apphttp "github.com/DoSafills/SOLOServis/backend/internal/http"
	"github.com/DoSafills/SOLOServis/backend/internal/server"
)

func main() {
	server.Run("Products", config.LoadForPort("8081"), apphttp.NewProductsRouter)
}
