package features

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

var serviceHTTPClient = &http.Client{Timeout: 4 * time.Second}

func remoteJSON(ctx context.Context, envName, defaultURL, method, path string, input, output any) error {
	baseURL := strings.TrimRight(os.Getenv(envName), "/")
	if baseURL == "" {
		baseURL = defaultURL
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
	request, err := http.NewRequestWithContext(ctx, method, baseURL+path, body)
	if err != nil {
		return err
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := serviceHTTPClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("service returned HTTP %d", response.StatusCode)
	}
	if output == nil || response.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(response.Body).Decode(output)
}

func verifyProductService(ctx context.Context, productID string) error {
	var result json.RawMessage
	return remoteJSON(ctx, "PRODUCT_API_URL", "http://localhost:8081", http.MethodGet, "/products/"+productID, nil, &result)
}
