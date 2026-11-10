package pricing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
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
	baseURL string
	token   string
	client  *http.Client
}

// NewHTTPNotifier lee NOTIFICATION_API_URL (por defecto localhost:8093) e
// INTERNAL_API_TOKEN, que se reenvía como Bearer.
func NewHTTPNotifier() *HTTPNotifier {
	baseURL := strings.TrimRight(os.Getenv("NOTIFICATION_API_URL"), "/")
	if baseURL == "" {
		baseURL = "http://localhost:8093"
	}
	return &HTTPNotifier{
		baseURL: baseURL,
		token:   os.Getenv("INTERNAL_API_TOKEN"),
		client:  &http.Client{Timeout: 3 * time.Second},
	}
}

func (n *HTTPNotifier) Notify(ctx context.Context, notification Notification) error {
	body, err := json.Marshal(notification)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, n.baseURL+"/notifications", bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if n.token != "" {
		request.Header.Set("Authorization", "Bearer "+n.token)
	}
	response, err := n.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("notification API respondió HTTP %d", response.StatusCode)
	}
	return nil
}
