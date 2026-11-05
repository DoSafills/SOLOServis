package features

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/apiclient"
)

var serviceHTTPClient = apiclient.NewClient(4 * time.Second)

func remoteJSON(ctx context.Context, envName, defaultURL, method, path string, input, output any) error {
	return serviceHTTPClient.DoJSON(ctx, envName, defaultURL, method, path, input, output)
}

func verifyProductService(ctx context.Context, productID string) error {
	var result json.RawMessage
	return remoteJSON(ctx, "PRODUCT_API_URL", "http://localhost:8081", http.MethodGet, "/products/"+productID, nil, &result)
}
