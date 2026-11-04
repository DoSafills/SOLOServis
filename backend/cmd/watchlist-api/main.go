package main

import (
	"github.com/DoSafills/SOLOServis/backend/internal/runner"
	"github.com/DoSafills/SOLOServis/backend/internal/watchlist"
)

func main() { runner.Run("Watchlist API", watchlist.NewRouter) }
