package assistant

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/comparison"
)

const (
	geminiBaseURL            = "https://generativelanguage.googleapis.com/v1beta"
	pollinationsBaseURL      = "https://text.pollinations.ai/openai"
	pollinationsDefaultModel = "openai"
	maxProviderBody          = 1 << 20 // 1 MB
)

// ProviderRequest es lo que el servicio envía al proveedor.
type ProviderRequest struct {
	SystemPrompt    string
	UserContent     string
	Question        string
	Matrix          comparison.Matrix
	Temperature     float64
	TopP            float64
	MaxOutputTokens int
}

// ProviderResult es la respuesta del proveedor.
type ProviderResult struct {
	Text         string
	PromptTokens int
	OutputTokens int
}

// Provider abstrae el proveedor de IA para poder simularlo en los tests.
type Provider interface {
	Generate(ctx context.Context, req ProviderRequest) (ProviderResult, error)
}

// identified lo implementan los proveedores reales para informar qué
// proveedor y modelo respondieron cuando la configuración es "auto".
type identified interface {
	ProviderName() string
	ProviderModel() string
}

// NewProviderFromConfig devuelve el proveedor configurado o nil si no hay
// ninguno disponible. Con AI_PROVIDER=auto se usa Gemini si hay AI_API_KEY.
// Sin clave no hay respuesta de respaldo: la ruta devuelve 503 ai_unavailable
// y jamás arma texto por su cuenta.
func NewProviderFromConfig(cfg Config, logger *slog.Logger) Provider {
	apiKey := strings.TrimSpace(os.Getenv("AI_API_KEY"))
	baseOverride := strings.TrimSpace(os.Getenv("AI_BASE_URL"))
	timeout := time.Duration(cfg.Limits.Provider.TimeoutMs) * time.Millisecond

	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" || provider == "auto" {
		provider = "gemini"
	}

	switch provider {
	case "gemini":
		if apiKey == "" {
			if logger != nil {
				logger.Warn("assistant_provider_without_key", "provider", provider)
			}
			return nil
		}
		baseURL := geminiBaseURL
		if baseOverride != "" {
			baseURL = baseOverride
		}
		return newGeminiProvider(apiKey, cfg.Model, baseURL, timeout,
			cfg.Limits.Provider.MaxRetries, cfg.Limits.Provider.RetryOnStatus)
	case "local":
		if logger != nil {
			logger.Warn("assistant_provider_removed", "provider", provider,
				"hint", "no hay respuestas de plantilla; configura AI_API_KEY o AI_BASE_URL")
		}
		return nil
	case "pollinations":
		baseURL := pollinationsBaseURL
		if baseOverride != "" {
			baseURL = baseOverride
		}
		return newOpenAIProvider("pollinations", apiKey, pollinationsModel(cfg.Model), baseURL,
			timeout, cfg.Limits.Provider.MaxRetries, cfg.Limits.Provider.RetryOnStatus, true)
	case "compatible", "openai":
		baseURL := baseOverride
		if baseURL == "" {
			if provider == "openai" {
				baseURL = "https://api.openai.com/v1"
			} else {
				if logger != nil {
					logger.Warn("assistant_provider_without_base_url", "provider", provider)
				}
				return nil
			}
		}
		// El modelo por defecto es de Gemini. Si se cambia el proveedor y no el
		// modelo, el proveedor responde 400 y el usuario sólo ve un 503. El aviso
		// deja la causa en el primer log del arranque.
		if strings.HasPrefix(strings.ToLower(cfg.Model), "gemini") && logger != nil {
			logger.Warn("assistant_provider_model_mismatch",
				"provider", provider, "model", cfg.Model,
				"hint", "el modelo por defecto es de Gemini; configura AI_MODEL para este proveedor")
		}
		return newOpenAIProvider(provider, apiKey, cfg.Model, baseURL, timeout,
			cfg.Limits.Provider.MaxRetries, cfg.Limits.Provider.RetryOnStatus, false)
	case "off", "none", "disabled":
		return nil
	default:
		if logger != nil {
			logger.Warn("assistant_provider_unsupported", "provider", cfg.Provider)
		}
		return nil
	}
}

// pollinationsModel evita enviar el nombre de un modelo de Gemini al
// proveedor gratuito: si no se configuró uno propio, usa el genérico.
func pollinationsModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" || strings.HasPrefix(strings.ToLower(model), "gemini") {
		return pollinationsDefaultModel
	}
	return model
}

type geminiProvider struct {
	apiKey     string
	model      string
	baseURL    string
	client     *http.Client
	maxRetries int
	retryOn    map[int]bool
}

func newGeminiProvider(apiKey, model, baseURL string, timeout time.Duration, maxRetries int, retryOn []int) *geminiProvider {
	return &geminiProvider{
		apiKey:     apiKey,
		model:      model,
		baseURL:    baseURL,
		client:     &http.Client{Timeout: timeout},
		maxRetries: maxRetries,
		retryOn:    retrySet(retryOn),
	}
}

func (g *geminiProvider) ProviderName() string  { return "gemini" }
func (g *geminiProvider) ProviderModel() string { return g.model }

func (g *geminiProvider) Generate(ctx context.Context, req ProviderRequest) (ProviderResult, error) {
	payload, err := json.Marshal(geminiRequest{
		SystemInstruction: geminiContent{Parts: []geminiPart{{Text: req.SystemPrompt}}},
		Contents: []geminiContent{{
			Role:  "user",
			Parts: []geminiPart{{Text: req.UserContent}},
		}},
		GenerationConfig: geminiGenerationConfig{
			Temperature:     req.Temperature,
			TopP:            req.TopP,
			MaxOutputTokens: req.MaxOutputTokens,
		},
	})
	if err != nil {
		return ProviderResult{}, fmt.Errorf("%w: no se pudo armar la solicitud", ErrUnavailable)
	}

	endpoint := strings.TrimRight(g.baseURL, "/") + "/models/" + url.PathEscape(g.model) + ":generateContent"
	return withRetries(ctx, g.client, g.maxRetries, g.retryOn, endpoint, payload,
		func(req *http.Request) { req.Header.Set("x-goog-api-key", g.apiKey) },
		parseGeminiResponse)
}

type geminiRequest struct {
	SystemInstruction geminiContent          `json:"systemInstruction"`
	Contents          []geminiContent        `json:"contents"`
	GenerationConfig  geminiGenerationConfig `json:"generationConfig"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerationConfig struct {
	Temperature     float64 `json:"temperature"`
	TopP            float64 `json:"topP"`
	MaxOutputTokens int     `json:"maxOutputTokens"`
}

type geminiResponse struct {
	Candidates []struct {
		Content      geminiContent `json:"content"`
		FinishReason string        `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
	} `json:"usageMetadata"`
}

func parseGeminiResponse(body []byte) (ProviderResult, error) {
	var response geminiResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return ProviderResult{}, fmt.Errorf("%w: respuesta ilegible del proveedor", ErrUnavailable)
	}
	if len(response.Candidates) == 0 {
		return ProviderResult{}, fmt.Errorf("%w: el proveedor no devolvió candidatos", ErrUnavailable)
	}

	var parts []string
	for _, part := range response.Candidates[0].Content.Parts {
		if text := strings.TrimSpace(part.Text); text != "" {
			parts = append(parts, text)
		}
	}
	if len(parts) == 0 {
		return ProviderResult{}, fmt.Errorf("%w: el proveedor devolvió una respuesta vacía", ErrUnavailable)
	}

	return ProviderResult{
		Text:         strings.Join(parts, "\n"),
		PromptTokens: response.UsageMetadata.PromptTokenCount,
		OutputTokens: response.UsageMetadata.CandidatesTokenCount,
	}, nil
}

// openAIProvider habla el formato chat/completions de OpenAI. Sirve para
// Pollinations (sin clave) y para cualquier proveedor compatible que se
// configure con AI_BASE_URL (Groq, OpenAI, un proxy propio).
type openAIProvider struct {
	name       string
	apiKey     string
	model      string
	baseURL    string
	client     *http.Client
	maxRetries int
	retryOn    map[int]bool
	private    bool
}

func newOpenAIProvider(name, apiKey, model, baseURL string, timeout time.Duration, maxRetries int, retryOn []int, private bool) *openAIProvider {
	return &openAIProvider{
		name:       name,
		apiKey:     apiKey,
		model:      model,
		baseURL:    baseURL,
		client:     &http.Client{Timeout: timeout},
		maxRetries: maxRetries,
		retryOn:    retrySet(retryOn),
		private:    private,
	}
}

func (o *openAIProvider) ProviderName() string  { return o.name }
func (o *openAIProvider) ProviderModel() string { return o.model }

func (o *openAIProvider) Generate(ctx context.Context, req ProviderRequest) (ProviderResult, error) {
	request := openAIChatRequest{
		Model: o.model,
		Messages: []openAIMessage{
			{Role: "system", Content: req.SystemPrompt},
			{Role: "user", Content: req.UserContent},
		},
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxOutputTokens,
		Private:     o.private,
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return ProviderResult{}, fmt.Errorf("%w: no se pudo armar la solicitud", ErrUnavailable)
	}

	endpoint := strings.TrimRight(o.baseURL, "/") + "/chat/completions"
	return withRetries(ctx, o.client, o.maxRetries, o.retryOn, endpoint, payload,
		func(req *http.Request) {
			if o.apiKey != "" {
				req.Header.Set("Authorization", "Bearer "+o.apiKey)
			}
		},
		parseOpenAIResponse)
}

type openAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature float64         `json:"temperature"`
	TopP        float64         `json:"top_p"`
	MaxTokens   int             `json:"max_tokens"`
	Private     bool            `json:"private,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

func parseOpenAIResponse(body []byte) (ProviderResult, error) {
	var response openAIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return ProviderResult{}, fmt.Errorf("%w: respuesta ilegible del proveedor", ErrUnavailable)
	}
	if len(response.Choices) == 0 {
		return ProviderResult{}, fmt.Errorf("%w: el proveedor no devolvió opciones", ErrUnavailable)
	}
	text := strings.TrimSpace(response.Choices[0].Message.Content)
	if text == "" {
		return ProviderResult{}, fmt.Errorf("%w: el proveedor devolvió una respuesta vacía", ErrUnavailable)
	}
	return ProviderResult{
		Text:         text,
		PromptTokens: response.Usage.PromptTokens,
		OutputTokens: response.Usage.CompletionTokens,
	}, nil
}

// withRetries envía el payload y reintenta solo ante errores transitorios.
func withRetries(
	ctx context.Context,
	client *http.Client,
	maxRetries int,
	retryOn map[int]bool,
	endpoint string,
	payload []byte,
	authorize func(*http.Request),
	parse func([]byte) (ProviderResult, error),
) (ProviderResult, error) {
	attempts := maxRetries + 1
	for attempt := 0; attempt < attempts; attempt++ {
		result, err := doRequest(ctx, client, endpoint, payload, authorize, parse)
		if err == nil {
			return result, nil
		}
		if attempt == attempts-1 || !retryable(err, retryOn) {
			return ProviderResult{}, err
		}
	}
	return ProviderResult{}, fmt.Errorf("%w: el proveedor no respondió", ErrUnavailable)
}

func doRequest(
	ctx context.Context,
	client *http.Client,
	endpoint string,
	payload []byte,
	authorize func(*http.Request),
	parse func([]byte) (ProviderResult, error),
) (ProviderResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return ProviderResult{}, fmt.Errorf("%w: solicitud inválida", ErrUnavailable)
	}
	req.Header.Set("Content-Type", "application/json")
	if authorize != nil {
		authorize(req)
	}

	res, err := client.Do(req)
	if err != nil {
		return ProviderResult{}, &providerHTTPError{status: 0}
	}
	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
		return ProviderResult{}, &providerHTTPError{status: res.StatusCode}
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, maxProviderBody))
	if err != nil {
		return ProviderResult{}, fmt.Errorf("%w: respuesta ilegible del proveedor", ErrUnavailable)
	}
	return parse(body)
}

func retrySet(retryOn []int) map[int]bool {
	if len(retryOn) == 0 {
		return map[int]bool{429: true, 500: true, 502: true, 503: true, 504: true}
	}
	retry := make(map[int]bool, len(retryOn))
	for _, status := range retryOn {
		retry[status] = true
	}
	return retry
}

func retryable(err error, retryOn map[int]bool) bool {
	var httpErr *providerHTTPError
	if !errors.As(err, &httpErr) {
		return false
	}
	if httpErr.status == 0 {
		return true
	}
	return retryOn[httpErr.status]
}

// providerHTTPError lleva solo el estado: el cuerpo del proveedor puede
// contener detalles internos y nunca se registra ni se devuelve.
type providerHTTPError struct {
	status int
}

func (e *providerHTTPError) Error() string {
	return fmt.Sprintf("%s: proveedor status %d", ErrUnavailable.Error(), e.status)
}

func (e *providerHTTPError) Unwrap() error {
	return ErrUnavailable
}
