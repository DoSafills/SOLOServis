package main

import (
	"github.com/DoSafills/SOLOServis/backend/internal/ingestion"
	"github.com/DoSafills/SOLOServis/backend/internal/runner"
)

func main() { runner.Run("Ingestion API", ingestion.NewRouter) }
