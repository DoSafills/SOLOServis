package main

import (
	"github.com/DoSafills/SOLOServis/backend/internal/auth"
	"github.com/DoSafills/SOLOServis/backend/internal/runner"
)

func main() { runner.Run("Auth API", auth.NewRouter) }
