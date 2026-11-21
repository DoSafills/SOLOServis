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
)

// Límites.
const (
	MaxBatchItems     = 100
	MaxSourceName     = 150 // data_source.name
	MaxScraperType    = 50  // scraper_config.scraper_type
	MaxBrandName      = 150 // brand.name
	MaxExternalID     = 255 // scraped_data.external_id
	MaxErrorMessage   = 2000
	DefaultScraper    = "http"
	DefaultListLimit  = 20
	MaxListLimit      = 100
	StatusRunning     = "running"
	StatusSuccess     = "success"
	StatusFailed      = "failed"
	ItemCreated       = "created" // producto nuevo + oferta
	ItemUpdated       = "updated" // producto existente, oferta registrada
	ItemFailed        = "failed"
	codeInternalError = "internal_error"
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
	// NotificationTypeDegraded marca el aviso de una tienda que dejó de
	// entregar datos; el frontend decide cómo mostrarlo por el `type`.
	NotificationTypeDegraded = "scraper_degraded"
)

// Run es una ejecución del scraper (scrape_run) con su fuente.
type Run struct {
	ID               int32      `json:"id"`
	Source           string     `json:"source"`
	SourceURL        *string    `json:"sourceUrl"`
	StoreID          *int32     `json:"storeId"`
	ScraperType      *string    `json:"scraperType"`
	Status           string     `json:"status"`
	StartedAt        time.Time  `json:"startedAt"`
	FinishedAt       *time.Time `json:"finishedAt"`
	RecordsFound     int32      `json:"recordsFound"`
	RecordsProcessed int32      `json:"recordsProcessed"`
	RecordsFailed    int32      `json:"recordsFailed"`
	ErrorMessage     *string    `json:"errorMessage"`
}

// OpenRunRequest es el body de POST /ingestion/runs.
type OpenRunRequest struct {
	Source      SourceInput `json:"source"`
	StoreID     int32       `json:"storeId"`
	ScraperType string      `json:"scraperType"`
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
}

// ItemsRequest es el body de POST /ingestion/runs/{id}/items.
type ItemsRequest struct {
	Items []Item `json:"items"`
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
