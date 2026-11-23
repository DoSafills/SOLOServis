package assistant

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/DoSafills/SOLOServis/backend/internal/comparison"
)

const (
	auditTimeout  = 2 * time.Second
	maxCacheItems = 1000
)

// Service arma el contexto, aplica límites y llama al proveedor.
type Service struct {
	provider     Provider
	cfg          Config
	prompt       string
	rejection    string
	providerName string
	modelName    string
	cache        *answerCache
	limiter      *rateLimiter
	recorder     Recorder
	logger       *slog.Logger
}

// NewService construye el servicio. provider puede ser nil (asistente
// desactivado): en ese caso Ask devuelve ErrUnavailable.
func NewService(provider Provider, cfg Config, prompt string, recorder Recorder, logger *slog.Logger) *Service {
	providerName, modelName := cfg.Provider, cfg.Model
	if id, ok := provider.(identified); ok {
		providerName, modelName = id.ProviderName(), id.ProviderModel()
	}
	return &Service{
		provider:     provider,
		cfg:          cfg,
		prompt:       prompt,
		rejection:    RejectionPhrase(prompt),
		providerName: providerName,
		modelName:    modelName,
		cache:        newAnswerCache(cfg.Limits.Cache.Enabled, time.Duration(cfg.Limits.Cache.TTLSeconds)*time.Second),
		limiter:      newRateLimiter(cfg.Limits.RateLimit.PerMinute, cfg.Limits.RateLimit.PerDay),
		recorder:     recorder,
		logger:       logger,
	}
}

// Ask responde la pregunta usando solo la matriz recibida.
func (s *Service) Ask(ctx context.Context, in AskInput) (Answer, error) {
	if s == nil || s.provider == nil || !s.cfg.Enabled {
		return Answer{}, ErrUnavailable
	}
	if !s.limiter.Allow(in.RateKey) {
		return Answer{}, ErrRateLimited
	}

	key := cacheKey(in.ProductIDs, in.Question)
	if cached, ok := s.cache.Get(key); ok {
		cached.Cached = true
		s.record(ctx, in, cached, 0, 0, 0, true, nil)
		return cached, nil
	}

	maxBytes := s.cfg.Limits.Context.MaxBytes
	contextJSON, dropped, err := encodeModelContext(in.Matrix, maxBytes)
	if err != nil {
		return Answer{}, fmt.Errorf("armar contexto del asistente: %w", err)
	}
	if s.logger != nil && dropped > 0 {
		// TAL-181: el recorte se avisa porque el modelo responde con menos
		// filas de las que ve el usuario en la comparación.
		s.logger.Warn("assistant_context_trimmed",
			"products", len(in.Matrix.Products),
			"rows_dropped", dropped,
			"context_bytes", len(contextJSON),
			"max_bytes", maxBytes,
		)
	}
	if s.logger != nil {
		s.logger.Debug("assistant_context_built",
			"provider", s.providerName,
			"products", len(in.Matrix.Products),
			"attributes", len(in.Matrix.Attributes),
			"specifications", len(in.Matrix.Specifications),
			"context_bytes", len(contextJSON),
		)
	}

	start := time.Now()
	result, err := s.provider.Generate(ctx, ProviderRequest{
		SystemPrompt:    s.prompt,
		UserContent:     fmt.Sprintf("<productos>%s</productos>\n<pregunta>%s</pregunta>", contextJSON, in.Question),
		Question:        in.Question,
		Matrix:          in.Matrix,
		Temperature:     s.cfg.Temperature,
		TopP:            s.cfg.TopP,
		MaxOutputTokens: s.cfg.MaxOutputTokens,
	})
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		s.record(ctx, in, Answer{}, 0, 0, latency, false, ErrUnavailable)
		return Answer{}, ErrUnavailable
	}

	answer := Answer{
		Answer:   s.normalize(result.Text, in.Matrix),
		Sources:  append([]string(nil), in.ProductIDs...),
		Provider: s.providerName,
		Model:    s.modelName,
	}
	s.cache.Set(key, answer)
	s.record(ctx, in, answer, result.PromptTokens, result.OutputTokens, latency, false, nil)
	return answer, nil
}

// normalize aplica los guardrails de salida: texto plano, mención de algún
// producto comparado y largo máximo. Si la respuesta no sirve, usa la frase
// fija del prompt. Una negativa explícita a inventar un dato ausente es válida
// aunque no repita el nombre del producto.
func (s *Service) normalize(text string, matrix comparison.Matrix) string {
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return s.rejection
	}
	if s.cfg.Limits.Answer.MustMentionComparedProduct &&
		!mentionsProduct(text, matrix.Products) && !saysDataUnavailable(text) {
		return s.rejection
	}
	return truncateAnswer(text, s.cfg.Limits.Answer.MaxChars)
}

// saysDataUnavailable reconoce una negativa a inventar: el modelo dice que el
// dato pedido no está en la comparación, que es exactamente lo que se espera.
func saysDataUnavailable(answer string) bool {
	lower := strings.ToLower(answer)
	for _, phrase := range []string{
		"no está disponible", "no están disponibles", "no estan disponibles",
		"no tengo", "no incluye", "no incluyen", "no aparece", "no aparecen",
		"no hay información", "no hay informacion", "no hay datos",
		"no se indica", "no se especifica", "no cuenta con", "no dispongo",
		"no puedo saber", "no figura", "no figuran", "no se menciona",
	} {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

// record guarda la auditoría sin bloquear la respuesta: si falla, solo se
// registra una advertencia.
func (s *Service) record(ctx context.Context, in AskInput, answer Answer, promptTokens, outputTokens, latencyMS int, cached bool, cause error) {
	if s.recorder == nil {
		return
	}
	audit := Audit{
		UserID:       in.UserID,
		ProductIDs:   append([]string(nil), in.ProductIDs...),
		Question:     in.Question,
		Answer:       answer.Answer,
		Provider:     answer.Provider,
		Model:        answer.Model,
		PromptTokens: promptTokens,
		OutputTokens: outputTokens,
		LatencyMS:    latencyMS,
		Cached:       cached,
		Success:      cause == nil,
	}
	if cause != nil {
		audit.ErrorMessage = cause.Error()
	}

	auditCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), auditTimeout)
	defer cancel()
	if err := s.recorder.Record(auditCtx, audit); err != nil && s.logger != nil {
		s.logger.Warn("assistant_audit_failed", "error", err.Error())
	}
}

func mentionsProduct(answer string, products []comparison.ProductSummary) bool {
	lower := strings.ToLower(answer)
	for _, product := range products {
		name := strings.ToLower(strings.TrimSpace(product.Name))
		if name != "" && strings.Contains(lower, name) {
			return true
		}
		for _, word := range strings.Fields(name) {
			if utf8.RuneCountInString(word) >= 4 && strings.Contains(lower, word) {
				return true
			}
		}
		brand := strings.ToLower(strings.TrimSpace(product.Brand))
		if brand != "" && strings.Contains(lower, brand) {
			return true
		}
	}
	return false
}

func truncateAnswer(text string, max int) string {
	if max <= 0 || utf8.RuneCountInString(text) <= max {
		return text
	}
	runes := []rune(text)
	if max == 1 {
		return "…"
	}
	cut := runes[:max-1]
	if index := lastSpace(cut); index > len(cut)/2 {
		cut = cut[:index]
	}
	return strings.TrimSpace(string(cut)) + "…"
}

func lastSpace(runes []rune) int {
	for i := len(runes) - 1; i >= 0; i-- {
		if runes[i] == ' ' {
			return i
		}
	}
	return -1
}

// cacheKey usa IDs ordenados y pregunta normalizada: el orden de la comparación
// no cambia la respuesta.
func cacheKey(productIDs []string, question string) string {
	ids := append([]string(nil), productIDs...)
	sort.Strings(ids)
	normalized := strings.ToLower(strings.Join(strings.Fields(question), " "))
	sum := sha256.Sum256([]byte(strings.Join(ids, ",") + "|" + normalized))
	return hex.EncodeToString(sum[:])
}

type cacheEntry struct {
	answer  Answer
	expires time.Time
}

type answerCache struct {
	mu      sync.Mutex
	enabled bool
	ttl     time.Duration
	items   map[string]cacheEntry
	now     func() time.Time
}

func newAnswerCache(enabled bool, ttl time.Duration) *answerCache {
	if ttl <= 0 {
		enabled = false
	}
	return &answerCache{enabled: enabled, ttl: ttl, items: map[string]cacheEntry{}, now: time.Now}
}

func (c *answerCache) Get(key string) (Answer, bool) {
	if c == nil || !c.enabled {
		return Answer{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.items[key]
	if !ok {
		return Answer{}, false
	}
	if !entry.expires.After(c.now()) {
		delete(c.items, key)
		return Answer{}, false
	}
	return entry.answer, true
}

func (c *answerCache) Set(key string, answer Answer) {
	if c == nil || !c.enabled {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if len(c.items) >= maxCacheItems {
		for itemKey, entry := range c.items {
			if !entry.expires.After(now) {
				delete(c.items, itemKey)
			}
		}
		if len(c.items) >= maxCacheItems {
			return
		}
	}
	c.items[key] = cacheEntry{answer: answer, expires: now.Add(c.ttl)}
}

// RetryAfter expone la espera del limitador para la clave dada, para que el
// handler arme el header `Retry-After` del 429 sin conocer sus internos.
// Devuelve un minuto si el servicio o el limitador no existen.
func (s *Service) RetryAfter(key string) time.Duration {
	if s == nil {
		return time.Minute
	}
	return s.limiter.RetryAfter(key)
}

type rateLimiter struct {
	mu        sync.Mutex
	hits      map[string][]time.Time
	perMinute int
	perDay    int
	now       func() time.Time
}

// RetryAfter devuelve cuánto falta para que key pueda volver a preguntar.
// Existe para que el handler cumpla TAL-78: un 429 lleva `Retry-After` en
// segundos. Si la ventana que se agotó es la del día, espera hasta que el
// primer acierto de las últimas 24 h salga de la ventana; si es la del
// minuto, hasta que el más antiguo del último minuto expire. Sin historial
// devuelve un minuto, que es el mínimo razonable para reintentar.
func (l *rateLimiter) RetryAfter(key string) time.Duration {
	const fallback = time.Minute
	if l == nil {
		return fallback
	}
	if key == "" {
		key = "unknown"
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	hits := l.hits[key]
	if len(hits) == 0 {
		return fallback
	}
	now := l.now()
	minuteCutoff := now.Add(-time.Minute)

	inMinute := 0
	for _, hit := range hits {
		if hit.After(minuteCutoff) {
			inMinute++
		}
	}
	// Si se agotó el minuto, basta con esperar al acierto más antiguo de esa
	// ventana; si se agotó el día, al más antiguo de las últimas 24 h.
	wait := hits[0].Add(24 * time.Hour).Sub(now)
	if inMinute >= l.perMinute {
		wait = hits[len(hits)-inMinute].Add(time.Minute).Sub(now)
	}
	if wait < time.Second {
		return time.Second
	}
	return wait
}

func newRateLimiter(perMinute, perDay int) *rateLimiter {
	return &rateLimiter{hits: map[string][]time.Time{}, perMinute: perMinute, perDay: perDay, now: time.Now}
}

// Allow aplica una ventana deslizante por minuto y por día.
func (l *rateLimiter) Allow(key string) bool {
	if l == nil {
		return true
	}
	if key == "" {
		key = "unknown"
	}
	now := l.now()
	dayCutoff := now.Add(-24 * time.Hour)
	minuteCutoff := now.Add(-time.Minute)

	l.mu.Lock()
	defer l.mu.Unlock()

	kept := l.hits[key][:0]
	inMinute := 0
	for _, hit := range l.hits[key] {
		if hit.After(dayCutoff) {
			kept = append(kept, hit)
			if hit.After(minuteCutoff) {
				inMinute++
			}
		}
	}
	if inMinute >= l.perMinute || len(kept) >= l.perDay {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	if len(l.hits) > 5000 {
		l.cleanup(dayCutoff)
	}
	return true
}

func (l *rateLimiter) cleanup(dayCutoff time.Time) {
	for key, hits := range l.hits {
		if len(hits) == 0 || !hits[len(hits)-1].After(dayCutoff) {
			delete(l.hits, key)
		}
	}
}
