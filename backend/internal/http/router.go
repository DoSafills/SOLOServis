package http

import (
	"reflect"

	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/products"
	"github.com/DoSafills/SOLOServis/backend/internal/services"
	"github.com/DoSafills/SOLOServis/backend/internal/stores"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// APITitle, APIVersion and APIDescription populate the OpenAPI info object.
const (
	APITitle   = "SOLOServis API"
	APIVersion = "1.0.0"
	APIDescription = `Comparison API for products, services and stores.

Every endpoint validates its input, documents itself in the OpenAPI 3.1
specification served at /openapi.json, and reports failures as RFC 9457
problem details. Monetary amounts are JSON numbers that preserve the decimal
digits stored in PostgreSQL.`
)

// Documentation endpoints exposed by the API.
const (
	// DocsPath serves the interactive Stoplight Elements UI.
	DocsPath = "/docs"
	// OpenAPIPath is the base of the specification routes.
	OpenAPIPath = "/openapi"
	// DocsJSONPath is the canonical 3.1 JSON specification route.
	DocsJSONPath = OpenAPIPath + ".json"
)

// ServiceName is reported by the health endpoint.
const ServiceName = "soloservis-api"

// HealthPath is the health endpoint route.
const HealthPath = "/health"

// healthProbeTimeout bounds how long the health check waits for the database.
const healthProbeTimeout = 2 * time.Second

// HealthResponse is the body of GET /health.
//
// The payload shape is identical whether the database is reachable or not, so a
// monitor can read the fields without branching on the status code. The code
// still differs: 200 when the database answered the ping, 503 otherwise.
type HealthResponse struct {
	Status   string `json:"status" enum:"ok,error" doc:"Overall service status." example:"ok"`
	Service  string `json:"service" doc:"Name of the service." example:"soloservis-api"`
	Database string `json:"database" enum:"connected,unavailable" doc:"Database connectivity as observed by the health probe." example:"connected"`
}

// Pinger is the subset of *pgxpool.Pool the health check needs. Declaring it as
// an interface lets the health endpoint be tested without a database.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Repositories groups the persistence ports the API depends on.
type Repositories struct {
	Products products.Repository
	Services services.Repository
	Stores   stores.Repository
}

// NewRouter builds the HTTP router backed by the given connection pool.
func NewRouter(db *pgxpool.Pool, frontendURL string) *chi.Mux {
	_, router := NewRouterWithAPI(db, Repositories{
		Products: products.NewRepository(db),
		Services: services.NewRepository(db),
		Stores:   stores.NewRepository(db),
	}, frontendURL)

	return router
}

// NewRouterWithAPI builds the API and its router from explicit ports. Tests use
// it to inject fake repositories and a fake Pinger.
func NewRouterWithAPI(pinger Pinger, repositories Repositories, frontendURL string) (huma.API, *chi.Mux) {
	router := chi.NewRouter()
	router.Use(cors(frontendURL))

	api := humachi.New(router, newConfig())

	registerHealth(router, api, pinger)
	products.NewHandler(repositories.Products).Register(api, "/products")
	services.NewHandler(repositories.Services).Register(api, "/services")
	stores.NewHandler(repositories.Stores).Register(api, "/stores")

	return api, router
}

func newConfig() huma.Config {
	config := huma.DefaultConfig(APITitle, APIVersion)

	config.OpenAPI.Info.Description = APIDescription
	config.DocsPath = DocsPath
	config.OpenAPIPath = OpenAPIPath

	// Stoplight Elements renders the 3.1 document together with a working
	// "try it" console.
	config.DocsRenderer = "stoplight"

	return config
}

// registerHealth attaches the health endpoint and documents it by hand.
//
// The endpoint is served by chi rather than by huma because its status code is
// decided at runtime. Huma can only vary the status by returning a
// huma.StatusError, which would replace the payload with a problem detail and
// break the monitors that read the database field. Keeping the route on chi
// preserves the existing contract, and the operation below keeps it present in
// the generated specification.
func registerHealth(router *chi.Mux, api huma.API, pinger Pinger) {
	router.Get(HealthPath, func(w http.ResponseWriter, r *http.Request) {
		body, status := probe(pinger, r.Context())

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)

		_ = json.NewEncoder(w).Encode(body)
	})

	if api.OpenAPI().Paths == nil {
		api.OpenAPI().Paths = map[string]*huma.PathItem{}
	}
	api.OpenAPI().Paths[HealthPath] = &huma.PathItem{
		Get: &huma.Operation{
			OperationID: "health-check",
			Summary:     "Health check",
			Description: `Reports whether the service is running and whether the database is reachable.

Returns 200 when the database answers a ping within two seconds, and 503
otherwise. The response body has the same shape in both cases.`,
			Tags: []string{"Health"},
			Responses: map[string]*huma.Response{
				"200": {
					Description: "The service is running and the database is reachable.",
					Content: map[string]*huma.MediaType{
						"application/json": {Schema: healthSchema(api)},
					},
				},
				"503": {
					Description: "The database is unreachable.",
					Content: map[string]*huma.MediaType{
						"application/json": {Schema: healthSchema(api)},
					},
				},
			},
		},
	}
}

// probe runs the database ping and derives both the payload and the status
// code.
func probe(pinger Pinger, ctx context.Context) (HealthResponse, int) {
	probeCtx, cancel := context.WithTimeout(ctx, healthProbeTimeout)
	defer cancel()

	if err := pinger.Ping(probeCtx); err != nil {
		return HealthResponse{
			Status:   "error",
			Service:  ServiceName,
			Database: "unavailable",
		}, http.StatusServiceUnavailable
	}

	return HealthResponse{
		Status:   "ok",
		Service:  ServiceName,
		Database: "connected",
	}, http.StatusOK
}

// healthSchema registers the health payload in the components section and
// returns the reference used by both documented responses.
func healthSchema(api huma.API) *huma.Schema {
	return api.OpenAPI().Components.Schemas.Schema(
		reflect.TypeOf(HealthResponse{}),
		true,
		"",
	)
}

// cors answers preflight requests and allows the configured frontend origin.
// A single origin is reflected because the API is not credentialed; answering
// with a wildcard would imply otherwise.
func cors(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type")
			w.Header().Set("Access-Control-Max-Age", "600")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
