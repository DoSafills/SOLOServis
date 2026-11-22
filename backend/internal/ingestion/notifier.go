package ingestion

import (
	"context"
	"net/http"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/apiclient"
)

// AdminNotification es un aviso dirigido a todos los administradores.
//
// A diferencia de pricing.Notification, no lleva `userId`: Notification API
// resuelve los destinatarios y crea una fila por cada usuario admin. Así el
// scraper no necesita conocer la tabla de usuarios ni sus ids.
type AdminNotification struct {
	Type    string
	Title   string
	Message string
	Data    map[string]any
}

// Notifier avisa a los administradores que una fuente dejó de entregar datos.
// Los tests usan un fake; en producción lo implementa HTTPNotifier.
type Notifier interface {
	NotifyAdmins(ctx context.Context, notification AdminNotification) error
}

// HTTPNotifier publica en POST {NOTIFICATION_API_URL}/notifications/admins con
// el token de servicio, que es el único camino entre dominios declarado para
// ingestion (AGENTS.md §5).
type HTTPNotifier struct {
	client *apiclient.Client
}

// NewHTTPNotifier arma el notificador con un timeout corto: una alerta que no
// se puede entregar rápido no debe retrasar el cierre de la corrida.
func NewHTTPNotifier() *HTTPNotifier {
	return &HTTPNotifier{client: apiclient.NewClient(3 * time.Second)}
}

// NotifyAdmins envía el aviso a Notification API y devuelve su error, si lo
// hay. Usa NOTIFICATION_API_URL o GATEWAY_URL y reenvía INTERNAL_API_TOKEN.
func (n *HTTPNotifier) NotifyAdmins(ctx context.Context, notification AdminNotification) error {
	data := notification.Data
	if data == nil {
		data = map[string]any{}
	}
	payload := map[string]any{
		"type":    notification.Type,
		"title":   notification.Title,
		"message": notification.Message,
		"data":    data,
	}
	return n.client.DoJSON(ctx, "NOTIFICATION_API_URL", "http://localhost:8093",
		http.MethodPost, "/notifications/admins", payload, nil)
}
