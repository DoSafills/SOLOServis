package main

import (
	"github.com/DoSafills/SOLOServis/backend/internal/config"
	apphttp "github.com/DoSafills/SOLOServis/backend/internal/http"
	"github.com/DoSafills/SOLOServis/backend/internal/server"
)

func main() {
	server.Run("SoloServis", config.Load(), apphttp.NewRouter)
}
