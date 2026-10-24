package main

import (
	"github.com/DoSafills/SOLOServis/backend/internal/comparisons"
	"github.com/DoSafills/SOLOServis/backend/internal/runner"
)

func main() { runner.Run("Comparison API", comparisons.NewRouter) }
