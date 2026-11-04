package main

import (
	"github.com/DoSafills/SOLOServis/backend/internal/favorites"
	"github.com/DoSafills/SOLOServis/backend/internal/runner"
)

func main() { runner.Run("Favorites API", favorites.NewRouter) }
