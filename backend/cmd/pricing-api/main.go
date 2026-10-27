package main

import (
	apphttp "github.com/DoSafills/SOLOServis/backend/internal/http"
	"github.com/DoSafills/SOLOServis/backend/internal/runner"
)

func main() { runner.Run("Pricing API", apphttp.NewPricingRouter) }
