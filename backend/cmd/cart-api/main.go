package main

import (
	"github.com/DoSafills/SOLOServis/backend/internal/cart"
	"github.com/DoSafills/SOLOServis/backend/internal/runner"
)

func main() { runner.Run("Cart API", cart.NewRouter) }
