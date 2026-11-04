package main

import (
	"github.com/DoSafills/SOLOServis/backend/internal/notifications"
	"github.com/DoSafills/SOLOServis/backend/internal/runner"
)

func main() { runner.Run("Notification API", notifications.NewRouter) }
