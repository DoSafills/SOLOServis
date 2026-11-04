package ingestion

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"strings"

	"github.com/DoSafills/SOLOServis/backend/internal/categories"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/DoSafills/SOLOServis/backend/internal/pricing"
	"github.com/DoSafills/SOLOServis/backend/internal/products"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// RunStore es lo que Ingestion necesita de sus tablas; *Repository la implementa.
type RunStore interface {
	OpenRun(ctx context.Context, name string, baseURL *string, storeID int32, scraperType string) (int32, error)
	GetRun(ctx context.Context, id int32) (Run, error)
	ListRuns(ctx context.Context, limit, offset int) ([]Run, int64, error)
	FinishRun(ctx context.Context, id int32, status string, message *string) error
	ItemsTarget(ctx context.Context, id int32) (int32, error)
	AddCounters(ctx context.Context, id int32, found, processed, failed int) error
	SaveScraped(ctx context.Context, runID int32, externalID, externalURL *string, raw json.RawMessage) error
	FindProduct(ctx context.Context, sku, brand, model string) (ExistingProduct, bool, error)
	EnsureBrand(ctx context.Context, name string) (int32, error)
	ProductID(ctx context.Context, publicID pgtype.UUID) (int32, error)
}

// Catalog crea productos con las reglas de Product API (TAL-70);
// *products.Repository la implementa.
type Catalog interface {
	ResolveCategory(ctx context.Context, ref products.CategoryRef) (products.WriteCategory, error)
	Create(ctx context.Context, product products.ProductWrite) (pgtype.UUID, error)
}

// Offers registra precios con las reglas de Pricing API (TAL-74);
// *pricing.Repository la implementa.
type Offers interface {
	UpsertOffer(ctx context.Context, w pricing.OfferWrite) (int32, bool, *string, error)
	GetOffer(ctx context.Context, offerID int32) (pricing.Offer, error)
	pricing.WatcherSource
}

// Processor incorpora los ítems de un lote al catálogo.
type Processor struct {
	runs     RunStore
	catalog  Catalog
	offers   Offers
	notifier pricing.Notifier
	logger   *slog.Logger
}

func NewProcessor(runs RunStore, catalog Catalog, offers Offers, notifier pricing.Notifier, logger *slog.Logger) *Processor {
	return &Processor{runs: runs, catalog: catalog, offers: offers, notifier: notifier, logger: logger}
}

// Process aplica cada ítem por separado: un ítem inválido o con error no
// detiene el lote. Guarda cada ítem en scraped_data y suma los contadores.
func (p *Processor) Process(ctx context.Context, runID, storeID int32, items []Item) BatchResult {
	result := BatchResult{RunID: runID, Received: len(items), Results: make([]ItemResult, 0, len(items))}
	for i, item := range items {
		r := p.processItem(ctx, runID, storeID, i, item)
		if r.Status == ItemFailed {
			result.Failed++
		} else {
			result.Processed++
		}
		result.Results = append(result.Results, r)
		p.save(ctx, runID, item)
	}
	if err := p.runs.AddCounters(ctx, runID, result.Received, result.Processed, result.Failed); err != nil {
		p.logger.ErrorContext(ctx, "ingestion_counters_failed", slog.Int("run_id", int(runID)), slog.String("error", err.Error()))
	}
	return result
}

func (p *Processor) processItem(ctx context.Context, runID, storeID int32, index int, item Item) ItemResult {
	result := ItemResult{Index: index, ExternalID: strings.TrimSpace(item.ExternalID)}
	fail := func(errs ...httpx.FieldError) ItemResult {
		result.Status, result.Errors = ItemFailed, errs
		return result
	}
	internal := func(stage string, err error) ItemResult {
		// El detalle va al log; el resultado solo indica un error interno.
		p.logger.ErrorContext(ctx, "ingestion_item_failed", slog.Int("run_id", int(runID)),
			slog.Int("index", index), slog.String("stage", stage), slog.String("error", err.Error()))
		return fail(httpx.FieldError{Field: "", Code: codeInternalError,
			Message: "error interno al procesar el ítem; ver el log del servicio"})
	}

	product, offer, errs := validateItem(item)
	if len(errs) > 0 {
		return fail(errs...)
	}
	offer.StoreID = storeID

	existing, found, err := p.runs.FindProduct(ctx, product.SKU, product.Brand, product.Model)
	if err != nil {
		return internal("find_product", err)
	}
	var productID int32
	switch {
	case found && !existing.Active:
		return fail(httpx.FieldError{Field: "product", Code: "conflict",
			Message: "el producto que coincide (por sku o marca y modelo) está dado de baja"})
	case found:
		// El catálogo existente no se modifica: lo cura el admin. Solo la oferta.
		productID, result.ProductID, result.Status = existing.ID, existing.PublicID, ItemUpdated
	default:
		var fieldErrs []httpx.FieldError
		productID, result.ProductID, fieldErrs, err = p.createProduct(ctx, product)
		if err != nil {
			return internal("create_product", err)
		}
		if len(fieldErrs) > 0 {
			return fail(fieldErrs...)
		}
		result.Status = ItemCreated
	}

	offer.ProductID = productID
	offerID, _, previous, err := p.offers.UpsertOffer(ctx, offer)
	if err != nil {
		return internal("upsert_offer", err)
	}
	result.OfferID = offerID
	if previous != nil {
		saved, err := p.offers.GetOffer(ctx, offerID)
		if err != nil {
			return internal("get_offer", err)
		}
		pricing.NotifyPriceDrop(ctx, p.offers, p.notifier, p.logger, saved, *previous)
	}
	return result
}

// createProduct valida categoría y atributos con TAL-70/72 y crea el producto.
func (p *Processor) createProduct(ctx context.Context, item ItemProduct) (int32, string, []httpx.FieldError, error) {
	if item.CategorySlug == "" {
		return 0, "", []httpx.FieldError{{Field: "product.categorySlug", Code: "required",
			Message: "product.categorySlug es obligatorio para crear un producto nuevo"}}, nil
	}
	category, err := p.catalog.ResolveCategory(ctx, products.CategoryRef{Slug: item.CategorySlug})
	if errors.Is(err, products.ErrCategoryNotFound) {
		return 0, "", []httpx.FieldError{{Field: "product.categorySlug", Code: "not_found", Message: "product.categorySlug no existe"}}, nil
	}
	if err != nil {
		return 0, "", nil, err
	}
	if category.ItemType != categories.ItemTypeProduct || category.LegacyID == nil {
		return 0, "", []httpx.FieldError{{Field: "product.categorySlug", Code: "not_found",
			Message: "product.categorySlug no está habilitada para productos"}}, nil
	}

	values, attrErrs := categories.ValidateAttributes(category.Attributes, item.Attributes)
	if len(attrErrs) > 0 {
		return 0, "", prefixErrors("product.", attrErrs), nil
	}

	write := products.ProductWrite{
		LegacyCategoryID: *category.LegacyID, CategoryRef: category.ID, Name: item.Name,
		Model: item.Model, SKU: item.SKU, Description: item.Description, ImageURL: item.ImageURL,
		Attributes: values,
	}
	if item.Brand != "" {
		brandID, err := p.runs.EnsureBrand(ctx, item.Brand)
		if err != nil {
			return 0, "", nil, err
		}
		write.BrandID = &brandID
	}

	publicID, err := p.catalog.Create(ctx, write)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return 0, "", []httpx.FieldError{{Field: "product.sku", Code: "conflict", Message: "ya existe un producto con ese sku"}}, nil
	}
	if err != nil {
		return 0, "", nil, err
	}
	id, err := p.runs.ProductID(ctx, publicID)
	if err != nil {
		return 0, "", nil, err
	}
	return id, publicID.String(), nil, nil
}

// validateItem normaliza el ítem y aplica las reglas de campos de Product y
// Pricing, con los errores prefijados por product. y offer.
func validateItem(item Item) (ItemProduct, pricing.OfferWrite, []httpx.FieldError) {
	p := item.Product
	p.SKU = strings.TrimSpace(p.SKU)
	p.Name = strings.TrimSpace(p.Name)
	p.Brand = strings.TrimSpace(p.Brand)
	p.Model = strings.TrimSpace(p.Model)
	p.CategorySlug = strings.TrimSpace(p.CategorySlug)
	p.Description = strings.TrimSpace(p.Description)
	p.ImageURL = strings.TrimSpace(p.ImageURL)

	var v httpx.Validator
	v.MaxLength("externalId", strings.TrimSpace(item.ExternalID), MaxExternalID)
	v.Required("product.name", p.Name)
	v.MaxLength("product.name", p.Name, products.MaxNameLength)
	v.MaxLength("product.model", p.Model, products.MaxModelLength)
	v.MaxLength("product.sku", p.SKU, products.MaxSKULength)
	v.MaxLength("product.description", p.Description, products.MaxDescriptionLength)
	v.MaxLength("product.brand", p.Brand, MaxBrandName)
	v.Check(p.CategorySlug == "" || categories.ValidSlug(p.CategorySlug), "product.categorySlug", "format",
		"product.categorySlug debe tener entre 1 y 64 caracteres: minúsculas, dígitos o guiones")
	v.Check(p.ImageURL == "" || isHTTPURL(p.ImageURL), "product.imageUrl", "format",
		"product.imageUrl debe ser una URL http(s) válida")

	var offerV httpx.Validator
	offer := pricing.ValidateOfferFields(&offerV, item.Offer)

	errs := append(v.Errors(), prefixErrors("offer.", offerV.Errors())...)
	return p, offer, errs
}

func prefixErrors(prefix string, errs []httpx.FieldError) []httpx.FieldError {
	out := make([]httpx.FieldError, 0, len(errs))
	for _, e := range errs {
		e.Field = prefix + e.Field
		e.Message = prefix + e.Message
		out = append(out, e)
	}
	return out
}

// save guarda el ítem tal como llegó (normalizado por el scraper) en
// scraped_data; un fallo aquí solo se registra en el log.
func (p *Processor) save(ctx context.Context, runID int32, item Item) {
	raw, err := json.Marshal(item)
	if err == nil {
		var externalID, externalURL *string
		if id := strings.TrimSpace(item.ExternalID); id != "" && len(id) <= MaxExternalID {
			externalID = &id
		}
		if u := strings.TrimSpace(item.Offer.ProductURL); u != "" {
			externalURL = &u
		}
		err = p.runs.SaveScraped(ctx, runID, externalID, externalURL, raw)
	}
	if err != nil {
		p.logger.WarnContext(ctx, "ingestion_save_scraped_failed", slog.Int("run_id", int(runID)), slog.String("error", err.Error()))
	}
}

func isHTTPURL(value string) bool {
	parsed, err := url.ParseRequestURI(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}
