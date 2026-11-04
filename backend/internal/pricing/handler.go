package pricing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/DoSafills/SOLOServis/backend/internal/products"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// maxURLLength acota productUrl (TEXT) a un largo razonable.
const maxURLLength = 2048

// OfferStore es lo que el handler necesita de la capa de datos; *Repository
// la implementa y los tests usan un fake en memoria.
type OfferStore interface {
	ResolveProduct(ctx context.Context, key string) (int32, error)
	StoreExists(ctx context.Context, storeID int32) (bool, error)
	GetOffer(ctx context.Context, offerID int32) (Offer, error)
	ListProductOffers(ctx context.Context, productID int32) ([]Offer, error)
	BestOffer(ctx context.Context, productID int32) (Offer, error)
	UpsertOffer(ctx context.Context, w OfferWrite) (int32, bool, *string, error)
	UpdateOffer(ctx context.Context, offerID int32, w OfferWrite) (string, error)
	PriceDropWatchers(ctx context.Context, offerID int32, oldPrice, newPrice string) ([]Watcher, error)
	OfferHistory(ctx context.Context, offerID int32, f HistoryFilter) ([]PricePoint, int64, error)
	ProductHistory(ctx context.Context, productID int32, f HistoryFilter) ([]ProductPricePoint, int64, error)
}

type Handler struct {
	store    OfferStore
	notifier Notifier
	logger   *slog.Logger
}

func NewHandler(store OfferStore, notifier Notifier, logger *slog.Logger) *Handler {
	return &Handler{store: store, notifier: notifier, logger: logger}
}

// ------------------------------------------------------------------ escritura

// CreateOffer atiende POST /offers: registra el precio actual. 201 + Location
// si la oferta es nueva; 200 si ya existía y se actualizó.
func (h *Handler) CreateOffer(w http.ResponseWriter, r *http.Request) {
	var request CreateOfferRequest
	if err := httpx.DecodeJSON(w, r, &request, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteDecodeError(w, r, err)
		return
	}

	var v httpx.Validator
	request.ProductID = strings.TrimSpace(request.ProductID)
	v.Required("productId", request.ProductID)
	v.Check(request.ProductID == "" || validProductKey(request.ProductID), "productId", "format",
		"productId debe ser un UUID o un id numérico positivo")
	v.Check(request.StoreID > 0, "storeId", "min", "storeId debe ser mayor que 0")
	offer := ValidateOfferFields(&v, request.OfferFields)
	if !v.Valid() {
		httpx.WriteValidation(w, r, v.Errors())
		return
	}

	productID, err := h.store.ResolveProduct(r.Context(), request.ProductID)
	if errors.Is(err, ErrProductNotFound) {
		httpx.WriteValidation(w, r, []httpx.FieldError{{Field: "productId", Code: "not_found", Message: "productId no existe"}})
		return
	}
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	exists, err := h.store.StoreExists(r.Context(), request.StoreID)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	if !exists {
		httpx.WriteValidation(w, r, []httpx.FieldError{{Field: "storeId", Code: "not_found", Message: "storeId no existe"}})
		return
	}
	offer.ProductID = productID
	offer.StoreID = request.StoreID

	offerID, inserted, previous, err := h.store.UpsertOffer(r.Context(), offer)
	if err != nil {
		h.writeWriteError(w, r, err)
		return
	}

	saved, err := h.store.GetOffer(r.Context(), offerID)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	if previous != nil {
		NotifyPriceDrop(r.Context(), h.store, h.notifier, h.logger, saved, *previous)
	}

	status := http.StatusOK
	if inserted {
		status = http.StatusCreated
		w.Header().Set("Location", "/offers/"+strconv.Itoa(int(saved.ID)))
	}
	writeJSON(w, status, saved)
}

// UpdateOffer atiende PUT /offers/{id}: reemplaza los datos de la oferta.
func (h *Handler) UpdateOffer(w http.ResponseWriter, r *http.Request) {
	offerID, ok := parseOfferID(w, r)
	if !ok {
		return
	}

	var request UpdateOfferRequest
	if err := httpx.DecodeJSON(w, r, &request, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteDecodeError(w, r, err)
		return
	}
	var v httpx.Validator
	offer := ValidateOfferFields(&v, request.OfferFields)
	if !v.Valid() {
		httpx.WriteValidation(w, r, v.Errors())
		return
	}

	previous, err := h.store.UpdateOffer(r.Context(), offerID, offer)
	if err != nil {
		h.writeWriteError(w, r, err)
		return
	}

	saved, err := h.store.GetOffer(r.Context(), offerID)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	if previous != "" {
		NotifyPriceDrop(r.Context(), h.store, h.notifier, h.logger, saved, previous)
	}
	writeJSON(w, http.StatusOK, saved)
}

// ValidateOfferFields aplica las reglas comunes de POST y PUT (422) y devuelve
// los datos normalizados. Los defaults son explícitos: un campo omitido no
// apaga la oferta ni borra su precio. También la usa Ingestion API (TAL-77).
func ValidateOfferFields(v *httpx.Validator, f OfferFields) OfferWrite {
	offer := OfferWrite{ShippingCost: "0", ShippingFree: f.ShippingFree, Available: true, Stock: f.Stock}

	if f.Price == nil {
		v.Add("price", "required", "price es obligatorio")
	} else {
		offer.Price = checkMoney(v, "price", f.Price)
	}
	if f.ListPrice != nil {
		listPrice := checkMoney(v, "listPrice", f.ListPrice)
		offer.ListPrice = &listPrice
	}
	if f.ShippingCost != nil {
		offer.ShippingCost = checkMoney(v, "shippingCost", f.ShippingCost)
	}
	if f.Available != nil {
		offer.Available = *f.Available
	}

	offer.Currency = strings.TrimSpace(f.Currency)
	if offer.Currency == "" {
		offer.Currency = "CLP"
	}
	v.Check(products.ValidateCurrencyCode(offer.Currency) == nil, "currency", "format",
		"currency debe tener exactamente tres letras mayúsculas (ISO 4217)")

	offer.Condition = strings.TrimSpace(f.Condition)
	if offer.Condition == "" {
		offer.Condition = "new"
	}
	v.Check(conditions[offer.Condition], "condition", "one_of", "condition debe ser new, used o refurbished")

	v.Check(f.Stock == nil || *f.Stock >= 0, "stock", "min", "stock debe ser mayor o igual a 0")

	if productURL := strings.TrimSpace(f.ProductURL); productURL != "" {
		v.MaxLength("productUrl", productURL, maxURLLength)
		v.Check(isHTTPURL(productURL), "productUrl", "format", "productUrl debe ser una URL http(s) válida")
		offer.ProductURL = &productURL
	}
	return offer
}

func checkMoney(v *httpx.Validator, field string, value *json.Number) string {
	text, code := parseMoney(value)
	switch code {
	case "min":
		v.Add(field, "min", field+" debe ser mayor o igual a 0")
	case "format":
		v.Add(field, "format", field+" debe ser un monto con hasta 10 enteros y 2 decimales")
	}
	return text
}

// writeWriteError mapea errores de escritura según TAL-78 §3.5.
func (h *Handler) writeWriteError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, ErrOfferNotFound) {
		httpx.WriteProblem(w, r, http.StatusNotFound, httpx.CodeNotFound, "La oferta no existe.")
		return
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		httpx.WriteProblem(w, r, http.StatusConflict, httpx.CodeConflict,
			"Ya existe otra oferta de ese producto en la tienda con la misma URL.")
		return
	}
	httpx.WriteInternal(w, r, h.logger, err)
}

// WatcherSource entrega los usuarios a avisar ante una bajada de precio.
type WatcherSource interface {
	PriceDropWatchers(ctx context.Context, offerID int32, oldPrice, newPrice string) ([]Watcher, error)
}

// NotifyPriceDrop avisa a los usuarios cuyo precio objetivo quedó cruzado por
// la bajada de previousPrice al precio actual de offer. Un fallo se registra
// en el log y no afecta la respuesta. La usan Pricing e Ingestion (TAL-77).
func NotifyPriceDrop(ctx context.Context, store WatcherSource, notifier Notifier, logger *slog.Logger, offer Offer, previousPrice string) {
	watchers, err := store.PriceDropWatchers(ctx, offer.ID, previousPrice, offer.Price)
	if err != nil {
		logger.WarnContext(ctx, "price_drop_watchers_failed", slog.Int("offer_id", int(offer.ID)), slog.String("error", err.Error()))
		return
	}
	for _, watcher := range watchers {
		err := notifier.Notify(ctx, Notification{
			UserID:  watcher.UserID,
			Type:    "price_drop",
			Title:   "Bajó el precio de " + watcher.ProductName,
			Message: "El precio llegó al objetivo configurado.",
			Data: map[string]any{
				"productId":   watcher.ProductID,
				"offerId":     offer.ID,
				"oldPrice":    previousPrice,
				"price":       offer.Price,
				"targetPrice": watcher.TargetPrice,
			},
		})
		if err != nil {
			logger.WarnContext(ctx, "price_drop_notification_failed",
				slog.Int("offer_id", int(offer.ID)), slog.Int("user_id", int(watcher.UserID)), slog.String("error", err.Error()))
		}
	}
}

// ------------------------------------------------------------------- lectura

// GetOffer atiende GET /offers/{id}.
func (h *Handler) GetOffer(w http.ResponseWriter, r *http.Request) {
	offerID, ok := parseOfferID(w, r)
	if !ok {
		return
	}
	offer, err := h.store.GetOffer(r.Context(), offerID)
	if errors.Is(err, ErrOfferNotFound) {
		httpx.WriteProblem(w, r, http.StatusNotFound, httpx.CodeNotFound, "La oferta no existe.")
		return
	}
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, offer)
}

// ListProductOffers atiende GET /offers/product/{productId}.
func (h *Handler) ListProductOffers(w http.ResponseWriter, r *http.Request) {
	productID, ok := h.resolveProductParam(w, r)
	if !ok {
		return
	}
	offers, err := h.store.ListProductOffers(r.Context(), productID)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	if offers == nil {
		offers = []Offer{}
	}
	writeJSON(w, http.StatusOK, offers)
}

// BestOffer atiende GET /offers/product/{productId}/best: la oferta disponible
// con menor precio total.
func (h *Handler) BestOffer(w http.ResponseWriter, r *http.Request) {
	productID, ok := h.resolveProductParam(w, r)
	if !ok {
		return
	}
	offer, err := h.store.BestOffer(r.Context(), productID)
	if errors.Is(err, ErrNoAvailable) {
		httpx.WriteProblem(w, r, http.StatusNotFound, httpx.CodeNotFound, "El producto no tiene ofertas disponibles.")
		return
	}
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, offer)
}

// OfferHistory atiende GET /offers/{id}/price-history?from=&to=&limit=&offset=.
func (h *Handler) OfferHistory(w http.ResponseWriter, r *http.Request) {
	offerID, ok := parseOfferID(w, r)
	if !ok {
		return
	}
	filter, ok := parseHistoryFilter(w, r)
	if !ok {
		return
	}
	points, total, err := h.store.OfferHistory(r.Context(), offerID, filter)
	if errors.Is(err, ErrOfferNotFound) {
		httpx.WriteProblem(w, r, http.StatusNotFound, httpx.CodeNotFound, "La oferta no existe.")
		return
	}
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	w.Header().Set("X-Total-Count", strconv.FormatInt(total, 10))
	writeJSON(w, http.StatusOK, points)
}

// ProductHistory atiende GET /offers/product/{productId}/price-history: el
// historial del producto en todas sus tiendas.
func (h *Handler) ProductHistory(w http.ResponseWriter, r *http.Request) {
	productID, ok := h.resolveProductParam(w, r)
	if !ok {
		return
	}
	filter, ok := parseHistoryFilter(w, r)
	if !ok {
		return
	}
	points, total, err := h.store.ProductHistory(r.Context(), productID, filter)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	w.Header().Set("X-Total-Count", strconv.FormatInt(total, 10))
	writeJSON(w, http.StatusOK, points)
}

// resolveProductParam valida {productId} (UUID o id numérico) y lo resuelve.
// Responde 400 o 404 y devuelve ok = false si no corresponde.
func (h *Handler) resolveProductParam(w http.ResponseWriter, r *http.Request) (int32, bool) {
	key := chi.URLParam(r, "productId")
	if !validProductKey(key) {
		httpx.WriteProblem(w, r, http.StatusBadRequest, httpx.CodeInvalidParameter,
			"El id del producto no es válido.",
			httpx.FieldError{Field: "productId", Code: "format", Message: "productId debe ser un UUID o un id numérico positivo"})
		return 0, false
	}
	productID, err := h.store.ResolveProduct(r.Context(), key)
	if errors.Is(err, ErrProductNotFound) {
		httpx.WriteProblem(w, r, http.StatusNotFound, httpx.CodeNotFound, "El producto no existe.")
		return 0, false
	}
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return 0, false
	}
	return productID, true
}

func parseOfferID(w http.ResponseWriter, r *http.Request) (int32, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
	if err != nil || id <= 0 {
		httpx.WriteProblem(w, r, http.StatusBadRequest, httpx.CodeInvalidParameter,
			"El id de la oferta no es válido.",
			httpx.FieldError{Field: "id", Code: "format", Message: "id debe ser un entero positivo"})
		return 0, false
	}
	return int32(id), true
}

func parseHistoryFilter(w http.ResponseWriter, r *http.Request) (HistoryFilter, bool) {
	query := r.URL.Query()
	filter := HistoryFilter{Limit: DefaultHistoryLimit}
	var v httpx.Validator

	parseDate := func(field string) *string {
		if !query.Has(field) {
			return nil
		}
		value := query.Get(field)
		if _, err := time.Parse(time.DateOnly, value); err != nil {
			v.Add(field, "format", field+" debe tener el formato YYYY-MM-DD")
			return nil
		}
		return &value
	}
	filter.From = parseDate("from")
	filter.To = parseDate("to")
	if filter.From != nil && filter.To != nil && *filter.To < *filter.From {
		v.Add("to", "min", "to debe ser igual o posterior a from")
	}

	if query.Has("limit") {
		limit, err := strconv.Atoi(query.Get("limit"))
		switch {
		case err != nil:
			v.Add("limit", "invalid_type", "limit debe ser un número entero")
		case limit < 1:
			v.Add("limit", "min", "limit debe ser mayor o igual a 1")
		case limit > MaxHistoryLimit:
			v.Add("limit", "max", fmt.Sprintf("limit no puede superar %d", MaxHistoryLimit))
		default:
			filter.Limit = limit
		}
	}
	if query.Has("offset") {
		offset, err := strconv.Atoi(query.Get("offset"))
		switch {
		case err != nil:
			v.Add("offset", "invalid_type", "offset debe ser un número entero")
		case offset < 0:
			v.Add("offset", "min", "offset debe ser mayor o igual a 0")
		default:
			filter.Offset = offset
		}
	}

	if !v.Valid() {
		httpx.WriteProblem(w, r, http.StatusBadRequest, httpx.CodeInvalidParameter,
			"Los parámetros de la consulta no son válidos.", v.Errors()...)
		return HistoryFilter{}, false
	}
	return filter, true
}

// validProductKey acepta el UUID público o un id numérico positivo.
func validProductKey(key string) bool {
	if id, err := strconv.ParseInt(key, 10, 32); err == nil {
		return id > 0
	}
	var uuid pgtype.UUID
	return uuid.Scan(key) == nil
}

func isHTTPURL(value string) bool {
	parsed, err := url.ParseRequestURI(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
