// Package assistant conecta el asistente de comparación con un proveedor de IA
// (Gemini). El contexto se arma en el servidor desde la matriz de la
// comparación: el cliente solo envía los IDs de los productos y la pregunta.
//
// La configuración y el prompt viven en assistant/ (raíz del repo) y se copian
// a assets/ con scripts/sync-assistant-assets.sh para poder embeberlos con
// go:embed.
package assistant

import (
	"context"
	"errors"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/comparison"
)

// Errores de dominio que el handler traduce a HTTP.
var (
	// ErrUnavailable indica que el asistente no está configurado o que el
	// proveedor no respondió. Nunca se expone el detalle al cliente.
	ErrUnavailable = errors.New("assistant unavailable")
	// ErrRateLimited indica que se agotó la cuota por minuto o por día.
	ErrRateLimited = errors.New("assistant rate limited")
)

// Límites por defecto si la configuración no los trae.
const (
	DefaultMinProducts     = 2
	DefaultMaxProducts     = 5
	DefaultMaxQuestion     = 300
	DefaultMaxAnswerChars  = 900
	DefaultPerMinute       = 10
	DefaultPerDay          = 100
	DefaultTimeout         = 10 * time.Second
	DefaultMaxContextBytes = 8192
	DefaultRejectionPhrase = "Solo puedo responder sobre los productos que estás comparando."
)

// Config es la unión de config/ai.config.json y config/limits.json, con las
// variables de entorno AI_PROVIDER y AI_MODEL como override.
type Config struct {
	Version         int     `json:"version"`
	Enabled         bool    `json:"enabled"`
	Scope           string  `json:"scope"`
	Provider        string  `json:"provider"`
	Model           string  `json:"model"`
	Language        string  `json:"language"`
	Temperature     float64 `json:"temperature"`
	TopP            float64 `json:"topP"`
	MaxOutputTokens int     `json:"maxOutputTokens"`
	Limits          Limits  `json:"-"`
}

// Limits es config/limits.json.
type Limits struct {
	Products struct {
		Min          int  `json:"min"`
		Max          int  `json:"max"`
		SameCategory bool `json:"sameCategory"`
	} `json:"products"`
	Question struct {
		MinLength int  `json:"minLength"`
		MaxLength int  `json:"maxLength"`
		AllowHTML bool `json:"allowHtml"`
	} `json:"question"`
	Answer struct {
		MaxWords                   int  `json:"maxWords"`
		MaxChars                   int  `json:"maxChars"`
		MustMentionComparedProduct bool `json:"mustMentionComparedProduct"`
	} `json:"answer"`
	RateLimit struct {
		Scope     string `json:"scope"`
		PerMinute int    `json:"perMinute"`
		PerDay    int    `json:"perDay"`
	} `json:"rateLimit"`
	Provider struct {
		TimeoutMs     int   `json:"timeoutMs"`
		MaxRetries    int   `json:"maxRetries"`
		RetryOnStatus []int `json:"retryOnStatus"`
	} `json:"provider"`
	Cache struct {
		Enabled    bool     `json:"enabled"`
		TTLSeconds int      `json:"ttlSeconds"`
		KeyFields  []string `json:"keyFields"`
	} `json:"cache"`
	// Context.MaxBytes es el tope del JSON de productos que recibe el modelo
	// (TAL-181). Si la matriz lo pasa, encodeModelContext quita filas.
	Context struct {
		MaxBytes int `json:"maxBytes"`
	} `json:"context"`
}

// Request es el body de POST /comparisons/assistant. Solo IDs y pregunta.
type Request struct {
	ProductIDs []string `json:"productIds"`
	Question   string   `json:"question"`
}

// Response es la respuesta pública; coincide con
// fronted/src/services/api/assistant.ts.
type Response struct {
	Answer   string   `json:"answer"`
	Sources  []string `json:"sources"`
	Provider string   `json:"provider"`
	Model    string   `json:"model"`
	Cached   bool     `json:"cached"`
}

// AskInput es lo que el handler entrega al servicio.
type AskInput struct {
	ProductIDs []string
	Question   string
	Matrix     comparison.Matrix
	RateKey    string
	UserID     *int32
}

// Answer es el resultado del servicio.
type Answer struct {
	Answer   string
	Sources  []string
	Provider string
	Model    string
	Cached   bool
}

// Audit es una fila de ai_comparison_query.
type Audit struct {
	UserID       *int32
	ProductIDs   []string
	Question     string
	Answer       string
	Provider     string
	Model        string
	PromptTokens int
	OutputTokens int
	LatencyMS    int
	Cached       bool
	Success      bool
	ErrorMessage string
}

// Recorder guarda la auditoría. PostgresRecorder es la implementación real y
// los tests usan un fake o nil.
type Recorder interface {
	Record(ctx context.Context, audit Audit) error
}
