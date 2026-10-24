package main

import (
	"github.com/DoSafills/SOLOServis/backend/internal/runner"
	"github.com/DoSafills/SOLOServis/backend/internal/search"
)

func main() { runner.Run("Search API", search.NewRouter) }
