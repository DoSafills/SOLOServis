package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/DoSafills/SOLOServis/backend/internal/authz"
	"github.com/DoSafills/SOLOServis/backend/internal/comparison"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/jackc/pgx/v5/pgtype"
)

// ComparisonStore es lo que el handler necesita de Comparison API; el
// repositorio real y los tests lo implementan.
type ComparisonStore interface {
	ResolveProducts(ctx context.Context, publicIDs []string) ([]comparison.Product, error)
	BuildMatrix(ctx context.Context, ids []int32) (comparison.Matrix, error)
}

// Handler atiende POST /comparisons/assistant.
type Handler struct {
	store   ComparisonStore
	service *Service
	logger  *slog.Logger
}

func NewHandler(store ComparisonStore, service *Service, logger *slog.Logger) *Handler {
	return &Handler{store: store, service: service, logger: logger}
}

// Ask valida el body, resuelve los productos y delega la respuesta al servicio.
func (h *Handler) Ask(w http.ResponseWriter, r *http.Request) {
	var request Request
	if err := httpx.DecodeJSON(w, r, &request, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteDecodeError(w, r, err)
		return
	}

	var validator httpx.Validator
	productIDs := h.checkProductIDs(&validator, request.ProductIDs)
	question := h.normalizeQuestion(&validator, request.Question)
	if !validator.Valid() {
		httpx.WriteValidation(w, r, validator.Errors())
		return
	}

	products, err := h.store.ResolveProducts(r.Context(), productIDs)
	if errors.Is(err, comparison.ErrProductNotFound) {
		httpx.WriteProblem(w, r, http.StatusNotFound, httpx.CodeNotFound, "Uno de los productos no existe.")
		return
	}
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	if !sameCategory(products) {
		httpx.WriteValidation(w, r, []httpx.FieldError{{
			Field:   "productIds",
			Code:    "incompatible_category",
			Message: "productIds debe contener productos de la misma categoría",
		}})
		return
	}

	matrix, err := h.store.BuildMatrix(r.Context(), internalIDs(products))
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}

	answer, err := h.service.Ask(r.Context(), AskInput{
		ProductIDs: productIDs,
		Question:   question,
		Matrix:     matrix,
		RateKey:    rateKey(r),
		UserID:     requestUserID(r),
	})
	switch {
	case errors.Is(err, ErrRateLimited):
		httpx.WriteProblem(w, r, http.StatusTooManyRequests, httpx.CodeTooManyRequests,
			"Hiciste demasiadas preguntas seguidas. Espera un momento y vuelve a intentarlo.")
		return
	case errors.Is(err, ErrUnavailable):
		httpx.WriteProblem(w, r, http.StatusServiceUnavailable, httpx.CodeAIUnavailable,
			"El asistente no está disponible en este momento.")
		return
	case err != nil:
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}

	writeJSON(w, http.StatusOK, Response{
		Answer:   answer.Answer,
		Sources:  answer.Sources,
		Provider: answer.Provider,
		Model:    answer.Model,
		Cached:   answer.Cached,
	})
}

func (h *Handler) checkProductIDs(validator *httpx.Validator, raw []string) []string {
	limits := h.service.cfg.Limits.Products
	switch {
	case len(raw) < limits.Min:
		validator.Add("productIds", "min", fmt.Sprintf("productIds debe tener al menos %d productos", limits.Min))
		return nil
	case len(raw) > limits.Max:
		validator.Add("productIds", "max", fmt.Sprintf("productIds admite hasta %d productos", limits.Max))
		return nil
	}

	seen := make(map[string]bool, len(raw))
	ids := make([]string, 0, len(raw))
	for _, value := range raw {
		id := strings.ToLower(strings.TrimSpace(value))
		if !validUUID(id) {
			validator.Add("productIds", "format", "productIds debe contener UUIDs")
			return nil
		}
		if seen[id] {
			validator.Add("productIds", "unique", "productIds no puede repetir productos")
			return nil
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

func (h *Handler) normalizeQuestion(validator *httpx.Validator, raw string) string {
	limits := h.service.cfg.Limits.Question
	question := strings.TrimSpace(raw)
	validator.Required("question", question)
	if question == "" {
		return question
	}
	if limits.MinLength > 1 {
		validator.Check(utf8.RuneCountInString(question) >= limits.MinLength, "question", "min",
			fmt.Sprintf("question debe tener al menos %d caracteres", limits.MinLength))
	}
	validator.MaxLength("question", question, limits.MaxLength)
	if !limits.AllowHTML {
		validator.Check(!strings.ContainsAny(question, "<>"), "question", "plain_text",
			"question debe ser texto plano, sin HTML")
	}
	return question
}

func sameCategory(products []comparison.Product) bool {
	for _, product := range products[1:] {
		if product.CategoryID != products[0].CategoryID {
			return false
		}
	}
	return true
}

func internalIDs(products []comparison.Product) []int32 {
	ids := make([]int32, len(products))
	for i, product := range products {
		ids[i] = product.ID
	}
	return ids
}

func validUUID(value string) bool {
	var uuid pgtype.UUID
	return len(value) == 36 && uuid.Scan(value) == nil
}

// rateKey usa la sesión si existe; la ruta es pública, así que normalmente es
// la IP. Detrás de nginx se toma el primer valor de X-Forwarded-For.
func rateKey(r *http.Request) string {
	if session, ok := authz.SessionFrom(r.Context()); ok {
		return fmt.Sprintf("user:%d", session.UserID)
	}
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		if index := strings.IndexByte(forwarded, ','); index >= 0 {
			forwarded = forwarded[:index]
		}
		if ip := strings.TrimSpace(forwarded); ip != "" {
			return "ip:" + ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if host == "" {
		host = "unknown"
	}
	return "ip:" + host
}

func requestUserID(r *http.Request) *int32 {
	session, ok := authz.SessionFrom(r.Context())
	if !ok {
		return nil
	}
	id := session.UserID
	return &id
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
