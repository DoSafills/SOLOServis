package assistant

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Los archivos se copian desde assistant/ con scripts/sync-assistant-assets.sh.
// Solo se embebe lo que el backend usa en tiempo de ejecución.
//
//go:embed assets/config/ai.config.json assets/config/limits.json assets/prompts/system.es.md assets/context/allowed-fields.json
var embedded embed.FS

const (
	configPath    = "assets/config/ai.config.json"
	limitsPath    = "assets/config/limits.json"
	promptPath    = "assets/prompts/system.es.md"
	allowlistPath = "assets/context/allowed-fields.json"
)

// LoadConfig lee la configuración embebida y aplica AI_PROVIDER / AI_MODEL.
func LoadConfig() (Config, error) {
	var cfg Config
	if err := readJSON(configPath, &cfg); err != nil {
		return Config{}, err
	}
	if err := readJSON(limitsPath, &cfg.Limits); err != nil {
		return Config{}, err
	}
	if provider := strings.TrimSpace(os.Getenv("AI_PROVIDER")); provider != "" {
		cfg.Provider = strings.ToLower(provider)
	}
	if model := strings.TrimSpace(os.Getenv("AI_MODEL")); model != "" {
		cfg.Model = model
	}
	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// DefaultConfig deja el asistente deshabilitado. Se usa si la configuración
// embebida no se puede leer: la ruta responde 503 y el resto del sitio sigue.
func DefaultConfig() Config {
	cfg := Config{Enabled: false, Provider: "gemini", Model: "gemini-3.5-flash"}
	cfg.applyDefaults()
	return cfg
}

// SystemPrompt devuelve el pre-contexto que se envía al proveedor.
func SystemPrompt() (string, error) {
	data, err := embedded.ReadFile(promptPath)
	if err != nil {
		return "", fmt.Errorf("leer %s: %w", promptPath, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// RejectionPhrase extrae del prompt la frase exacta que el modelo debe usar
// cuando la pregunta queda fuera de alcance. La fuente de verdad es
// assistant/prompts/system.es.md.
func RejectionPhrase(prompt string) string {
	const marker = "responde exactamente:"
	index := strings.Index(prompt, marker)
	if index < 0 {
		return DefaultRejectionPhrase
	}
	for _, line := range strings.Split(prompt[index+len(marker):], "\n") {
		line = strings.TrimSpace(line)
		line = strings.Trim(line, "`\"")
		if line == "" {
			continue
		}
		return line
	}
	return DefaultRejectionPhrase
}

// Allowlist devuelve el JSON de context/allowed-fields.json. Los tests lo usan
// para comprobar que el contexto armado no incluye campos prohibidos.
func Allowlist() ([]byte, error) {
	return embedded.ReadFile(allowlistPath)
}

func readJSON(name string, dst any) error {
	data, err := embedded.ReadFile(name)
	if err != nil {
		return fmt.Errorf("leer %s: %w", name, err)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("decodificar %s: %w", name, err)
	}
	return nil
}

func (c *Config) applyDefaults() {
	if c.Provider == "" {
		c.Provider = "gemini"
	}
	if c.Model == "" {
		c.Model = "gemini-3.5-flash"
	}
	if c.Language == "" {
		c.Language = "es-CL"
	}
	if c.MaxOutputTokens <= 0 {
		c.MaxOutputTokens = 400
	}
	if c.Limits.Products.Min < 2 {
		c.Limits.Products.Min = DefaultMinProducts
	}
	if c.Limits.Products.Max < c.Limits.Products.Min {
		c.Limits.Products.Max = DefaultMaxProducts
	}
	if c.Limits.Question.MinLength < 1 {
		c.Limits.Question.MinLength = 1
	}
	if c.Limits.Question.MaxLength <= 0 {
		c.Limits.Question.MaxLength = DefaultMaxQuestion
	}
	if c.Limits.Answer.MaxChars <= 0 {
		c.Limits.Answer.MaxChars = DefaultMaxAnswerChars
	}
	if c.Limits.RateLimit.PerMinute <= 0 {
		c.Limits.RateLimit.PerMinute = DefaultPerMinute
	}
	if c.Limits.RateLimit.PerDay <= 0 {
		c.Limits.RateLimit.PerDay = DefaultPerDay
	}
	if c.Limits.Provider.TimeoutMs <= 0 {
		c.Limits.Provider.TimeoutMs = int(DefaultTimeout / 1e6)
	}
	if c.Limits.Provider.MaxRetries < 0 {
		c.Limits.Provider.MaxRetries = 0
	}
	if c.Limits.Cache.TTLSeconds <= 0 {
		c.Limits.Cache.TTLSeconds = 86400
	}
}

func (c Config) validate() error {
	if strings.TrimSpace(c.Provider) == "" {
		return fmt.Errorf("configuración del asistente: provider vacío")
	}
	if strings.TrimSpace(c.Model) == "" {
		return fmt.Errorf("configuración del asistente: model vacío")
	}
	if c.Limits.Products.Min < 2 {
		return fmt.Errorf("configuración del asistente: products.min debe ser al menos 2")
	}
	if c.Limits.Products.Max < c.Limits.Products.Min {
		return fmt.Errorf("configuración del asistente: products.max debe ser mayor o igual a products.min")
	}
	if c.Limits.Question.MaxLength < 1 {
		return fmt.Errorf("configuración del asistente: question.maxLength debe ser positivo")
	}
	return nil
}
