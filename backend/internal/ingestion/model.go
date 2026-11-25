// Package ingestion implementa Ingestion API (TAL-77): recibe del scraper
// productos y ofertas ya normalizados, los incorpora al catálogo con las
// mismas reglas que Product (TAL-70/72) y Pricing (TAL-74), y registra cada
// ejecución en las tablas de 001_schema.sql (data_source, scraper_config,
// scrape_run, scraped_data), que el scraper (PR #25) ya refleja.
package ingestion

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/DoSafills/SOLOServis/backend/internal/pricing"
)

// Errores de dominio que el handler traduce a HTTP (TAL-78 §3.5).
var (
	ErrRunNotFound         = errors.New("run not found")
	ErrRunClosed           = errors.New("run is not running")
	ErrRunWithoutStore     = errors.New("run source has no store")
	ErrStoreNotFound       = errors.New("store not found")
	ErrSourceStoreMismatch = errors.New("source belongs to another store")
	ErrSourceInactive      = errors.New("source is inactive")
	// ErrRunTargetMismatch separa las corridas de producto de las de servicio:
	// cada endpoint de ítems acepta solo su tipo (issue #191).
	ErrRunTargetMismatch = errors.New("run target type does not match the endpoint")
)

// Tipos de objetivo de una corrida (scraper_config.target_type).
const (
	TargetProduct = "product"
	TargetService = "service"
)

// Límites.
const (
	MaxBatchItems   = 100
	MaxSourceName   = 150 // data_source.name
	MaxScraperType  = 50  // scraper_config.scraper_type
	MaxBrandName    = 150 // brand.name
	MaxExternalID   = 255 // scraped_data.external_id
	MaxErrorMessage = 2000
	MaxSpecHash     = 128
	MaxSpecVersion  = 32
	// MaxFingerprintValues acota la huella: un mapa gigante es un error del
	// cliente, no una corrida real.
	MaxFingerprintValues = 500
	MaxFingerprintText   = 128
	DefaultScraper       = "http"
	DefaultListLimit     = 20
	MaxListLimit         = 100
	MaxServiceName       = 255 // service.name
	MaxProviderName      = 150 // provider.name
	MaxProviderSlug      = 64  // provider.slug
	MaxServiceURL        = 2048
	MaxServiceSummary    = 2000
	StatusRunning        = "running"
	StatusSuccess        = "success"
	StatusFailed         = "failed"
	ItemCreated          = "created" // producto nuevo + oferta
	ItemUpdated          = "updated" // producto existente, oferta registrada
	ItemFailed           = "failed"
	codeInternalError    = "internal_error"
	// MaxCounterValue acota los contadores del scraper: más que esto es un
	// error de programación del cliente, no una corrida real.
	MaxCounterValue = 1_000_000
)

// Umbrales por defecto de la alerta de extracción (TAL-211).
const (
	// DefaultRejectRatio: proporción de tarjetas rechazadas sobre las
	// encontradas a partir de la cual la corrida se considera degradada.
	DefaultRejectRatio = 0.30
	// DefaultDropRatio: caída de ítems enviados contra la mediana histórica.
	DefaultDropRatio = 0.50
	// DefaultBaselineRuns: corridas exitosas previas que forman la mediana.
	DefaultBaselineRuns = 5
	// DefaultPriceRatio: proporción de ítems con precio nulo o en 0 sobre los
	// recibidos a partir de la cual la corrida se considera degradada.
	DefaultPriceRatio = 0.20
	// NotificationTypeDegraded marca el aviso de una tienda que dejó de
	// entregar datos; el frontend decide cómo mostrarlo por el `type`.
	NotificationTypeDegraded = "scraper_degraded"
	// NotificationTypeSourceChanged marca un cambio detectado en la estructura,
	// la respuesta o el endpoint de una fuente (TAL-209).
	NotificationTypeSourceChanged = "source_changed"

	// Umbrales por defecto de la huella estructural (TAL-209).
	DefaultRootDropRatio        = 0.50
	DefaultSelectorMinRatio     = 0.10
	DefaultHTTPFailRatio        = 0.20
	DefaultResponseMissingRatio = 0.50
	DefaultMinCards             = 10
	DefaultMinPages             = 5
)

// Tipos de cambio de una fuente (TAL-209). Cada evento guardado en
// source_change_event lleva uno de estos valores.
const (
	ChangeTypeStructure = "estructura" // selectores clave dejaron de coincidir
	ChangeTypeResponse  = "respuesta"  // claves o tipos del JSON-LD cambiaron
	ChangeTypeEndpoint  = "endpoint"   // HTTP, content-type o redirecciones
	ChangeTypeRecords   = "registros"  // caída de tarjetas contra la mediana
)

// Run es una ejecución del scraper (scrape_run) con su fuente.
type Run struct {
	ID          int32   `json:"id"`
	Source      string  `json:"source"`
	SourceURL   *string `json:"sourceUrl"`
	StoreID     *int32  `json:"storeId"`
	ScraperType *string `json:"scraperType"`
	// TargetType distingue producto de servicio (scraper_config.target_type).
	TargetType       string     `json:"targetType"`
	Status           string     `json:"status"`
	StartedAt        time.Time  `json:"startedAt"`
	FinishedAt       *time.Time `json:"finishedAt"`
	RecordsFound     int32      `json:"recordsFound"`
	RecordsProcessed int32      `json:"recordsProcessed"`
	RecordsFailed    int32      `json:"recordsFailed"`
	ErrorMessage     *string    `json:"errorMessage"`
}

// OpenRunRequest es el body de POST /ingestion/runs.
//
// `targetType` es opcional y vale "product" por defecto, para no romper a los
// scraper que ya corren. Las corridas de servicio (issue #191) no exigen
// `storeId`: la fuente es un proveedor y no una tienda.
type OpenRunRequest struct {
	Source      SourceInput `json:"source"`
	StoreID     int32       `json:"storeId"`
	ScraperType string      `json:"scraperType"`
	TargetType  string      `json:"targetType"`
	// SpecHash y SpecVersion identifican el spec con el que el scraper extrajo.
	// Son opcionales: un scraper sin ellos abre la corrida como antes.
	SpecHash    string `json:"specHash"`
	SpecVersion string `json:"specVersion"`
}

type SourceInput struct {
	Name    string `json:"name"`
	BaseURL string `json:"baseUrl"`
}

// FinishRequest es el body de POST /ingestion/runs/{id}/finish.
//
// Los contadores son opcionales: un scraper que no los envíe sigue cerrando la
// corrida como antes. Cuando llegan, ingestion evalúa si la corrida quedó
// degradada (TAL-211) y avisa a los administradores. `itemsDuplicated` (TAL-172)
// explica una caída que en realidad fue un listado con tarjetas repetidas.
type FinishRequest struct {
	Status          string `json:"status"`
	ErrorMessage    string `json:"errorMessage"`
	ItemsFound      *int   `json:"itemsFound"`
	ItemsSent       *int   `json:"itemsSent"`
	ItemsRejected   *int   `json:"itemsRejected"`
	ItemsDuplicated *int   `json:"itemsDuplicated"`
	PagesFailed     *int   `json:"pagesFailed"`
	// Fingerprint es la huella estructural de las páginas leídas (TAL-209).
	// Opcional: sin ella la corrida se cierra igual que antes.
	Fingerprint *Fingerprint `json:"fingerprint"`
}

// Fingerprint es la huella estructural que el scraper reporta al cerrar una
// corrida: cuántas tarjetas vio, cuántas veces coincidió cada selector
// declarado, el esquema del JSON-LD de la página y el estado de las páginas.
// Es lo que permite detectar un cambio de HTML, de respuesta o de endpoint sin
// guardar el contenido completo (que haría ruido por anuncios y contenido
// dinámico). Los ceros son valores reales: por eso el bloque es opcional y no
// se evalúa si falta.
type Fingerprint struct {
	RootMatches       int            `json:"rootMatches"`
	SelectorHits      map[string]int `json:"selectorHits,omitempty"`
	RequiredSelectors []string       `json:"requiredSelectors,omitempty"`
	JSONLDKeys        []string       `json:"jsonldKeys,omitempty"`
	JSONLDTypes       []string       `json:"jsonldTypes,omitempty"`
	PagesChecked      int            `json:"pagesChecked"`
	PagesHTTPFailed   int            `json:"pagesHttpFailed"`
	PagesNonHTML      int            `json:"pagesNonHtml"`
}

// ItemsRequest es el body de POST /ingestion/runs/{id}/items.
type ItemsRequest struct {
	Items []Item `json:"items"`
}

// ServicesRequest es el body de POST /ingestion/runs/{id}/services.
type ServicesRequest struct {
	Items []ServiceItem `json:"items"`
}

// ServiceItem es un servicio (plan) con su oferta en el proveedor de la
// corrida, ya normalizado por el scraper (issue #191).
//
// `externalUrl` es la identidad dentro del proveedor: un plan nuevo del mismo
// proveedor es una URL nueva. `externalId` es opcional y solo se guarda para
// auditoría.
type ServiceItem struct {
	ExternalID  string             `json:"externalId"`
	ExternalURL string             `json:"externalUrl"`
	Provider    ProviderInput      `json:"provider"`
	Service     ServiceFields      `json:"service"`
	Offer       ServiceOfferFields `json:"offer"`
}

// ProviderInput identifica al proveedor del servicio. `slug` es la identidad
// estable; `name` es el nombre visible.
type ProviderInput struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// ServiceFields son los datos del plan. La categoría es un slug de la tabla
// unificada `categories` y sus atributos se validan contra ella.
type ServiceFields struct {
	Name         string                     `json:"name"`
	CategorySlug string                     `json:"categorySlug"`
	Description  string                     `json:"description"`
	ImageURL     string                     `json:"imageUrl"`
	Attributes   map[string]json.RawMessage `json:"attributes"`
}

// ServiceOfferFields es el precio y las condiciones del plan en un proveedor.
type ServiceOfferFields struct {
	Price            *json.Number `json:"price"`
	Currency         string       `json:"currency"`
	BillingPeriod    string       `json:"billingPeriod"`
	InstallationCost *json.Number `json:"installationCost"`
	ContractPeriod   string       `json:"contractPeriod"`
	Available        *bool        `json:"available"`
	CoverageSummary  string       `json:"coverageSummary"`
	AdditionalCosts  string       `json:"additionalCosts"`
}

// Item es un producto con su oferta en la tienda de la ejecución, ya
// normalizado por el scraper (contrato de TAL-79/90).
type Item struct {
	ExternalID string              `json:"externalId"`
	Product    ItemProduct         `json:"product"`
	Offer      pricing.OfferFields `json:"offer"`
}

// ItemProduct identifica el producto (sku, o marca + modelo) y trae lo
// necesario para crearlo si no existe.
type ItemProduct struct {
	SKU          string                     `json:"sku"`
	Name         string                     `json:"name"`
	Brand        string                     `json:"brand"`
	Model        string                     `json:"model"`
	CategorySlug string                     `json:"categorySlug"`
	Description  string                     `json:"description"`
	ImageURL     string                     `json:"imageUrl"`
	Attributes   map[string]json.RawMessage `json:"attributes"`
}

// ItemResult es el resultado de un ítem del lote.
type ItemResult struct {
	Index      int                `json:"index"`
	ExternalID string             `json:"externalId,omitempty"`
	Status     string             `json:"status"`
	ProductID  string             `json:"productId,omitempty"`
	ServiceID  string             `json:"serviceId,omitempty"`
	OfferID    int32              `json:"offerId,omitempty"`
	Errors     []httpx.FieldError `json:"errors,omitempty"`
}

// BatchResult es la respuesta de POST /ingestion/runs/{id}/items.
type BatchResult struct {
	RunID     int32        `json:"runId"`
	Received  int          `json:"received"`
	Processed int          `json:"processed"`
	Failed    int          `json:"failed"`
	Results   []ItemResult `json:"results"`
}

// ExistingProduct es un producto ya catalogado que coincide con el ítem.
type ExistingProduct struct {
	ID       int32
	PublicID string
	Active   bool
	// OfferURL es la URL guardada de la oferta que identificó al producto,
	// cuando se encontró por tienda + URL. Vacía en los otros casos.
	OfferURL string
}
