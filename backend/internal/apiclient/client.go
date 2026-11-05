package apiclient

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

const (
	GatewayURLEnv = "GATEWAY_URL"
	internalToken = "INTERNAL_API_TOKEN"
)

// Client centraliza las solicitudes JSON entre APIs.
type Client struct {
	httpClient *http.Client
}

func NewClient(timeout time.Duration) *Client {
	return &Client{httpClient: &http.Client{Timeout: timeout}}
}

// DoJSON envía una solicitud JSON usando primero la URL específica del servicio,
// después GATEWAY_URL y, finalmente, la URL local predeterminada.
func (c *Client) DoJSON(ctx context.Context, serviceURLEnv, defaultURL, method, path string, input, output any) error {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv(serviceURLEnv)), "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(strings.TrimSpace(os.Getenv(GatewayURLEnv)), "/")
	}
	if baseURL == "" {
		baseURL = strings.TrimRight(defaultURL, "/")
	}

	var body *bytes.Reader
	if input == nil {
		body = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, baseURL+"/"+strings.TrimLeft(path, "/"), body)
	if err != nil {
		return err
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token := os.Getenv(internalToken); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("service returned HTTP %d", response.StatusCode)
	}
	if output == nil || response.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(response.Body).Decode(output)
}
