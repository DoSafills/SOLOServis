package main

import (
	"github.com/DoSafills/SOLOServis/backend/internal/reviews"
	"github.com/DoSafills/SOLOServis/backend/internal/runner"
)

func main() { runner.Run("Review API", reviews.NewRouter) }
