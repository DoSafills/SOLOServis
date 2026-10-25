package main

import (
	"github.com/DoSafills/SOLOServis/backend/internal/runner"
	apphttp "github.com/DoSafills/SOLOServis/backend/internal/http"
)

func main() { runner.Run("Search API", apphttp.NewSearchRouter) }
