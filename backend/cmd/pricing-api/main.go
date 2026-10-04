package main

import (
	"github.com/DoSafills/SOLOServis/backend/internal/pricing"
	"github.com/DoSafills/SOLOServis/backend/internal/runner"
)

func main() { runner.Run("Pricing API", pricing.NewRouter) }
