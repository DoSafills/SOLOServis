package pricing

import (
	"context"
	"net/http"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/apiclient"
)

// Notification es el aviso que se envía a Notification API.
type Notification struct {
	UserID  int32          `json:"userId"`
	Type    string         `json:"type"`
	Title   string         `json:"title"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
}

// Notifier envía avisos a los usuarios. La implementación HTTP llama a
// Notification API; los tests usan un fake.
type Notifier interface {
	Notify(ctx context.Context, n Notification) error
}

// HTTPNotifier publica en POST {NOTIFICATION_API_URL}/notifications.
type HTTPNotifier struct {
	client *apiclient.Client
}

// NewHTTPNotifier usa NOTIFICATION_API_URL o GATEWAY_URL (por defecto
// localhost:8093) y reenvía INTERNAL_API_TOKEN como Bearer.
func NewHTTPNotifier() *HTTPNotifier {
	return &HTTPNotifier{
		client: apiclient.NewClient(3 * time.Second),
	}
}

func (n *HTTPNotifier) Notify(ctx context.Context, notification Notification) error {
	return n.client.DoJSON(ctx, "NOTIFICATION_API_URL", "http://localhost:8093",
		http.MethodPost, "/notifications", notification, nil)
}
