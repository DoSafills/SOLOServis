package ingestion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"unicode"

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
	FindProduct(ctx context.Context, sku, brand, model string, storeID int32, productURL string) (ExistingProduct, bool, error)
	AddImageIfMissing(ctx context.Context, productID int32, imageURL string) (bool, error)
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

// NewProcessor arma el procesador de ítems con sus dependencias: ejecuciones,
// catálogo de productos, ofertas, notificaciones y logger.
func NewProcessor(runs RunStore, catalog Catalog, offers Offers, notifier pricing.Notifier, logger *slog.Logger) *Processor {
	return &Processor{runs: runs, catalog: catalog, offers: offers, notifier: notifier, logger: logger}
}

// Process aplica cada ítem por separado: un ítem inválido o con error no
// detiene el lote. Guarda cada ítem en scraped_data y suma los contadores.
func (p *Processor) Process(ctx context.Context, runID, storeID int32, storeName string, items []Item) BatchResult {
	result := BatchResult{RunID: runID, Received: len(items), Results: make([]ItemResult, 0, len(items))}
	for i, item := range items {
		r := p.processItem(ctx, runID, storeID, storeName, i, item)
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

// processItem procesa el ítem número index del lote runID para la tienda
// storeID: lo valida, busca si el producto ya existe (o lo crea), completa la
// imagen de un producto existente que no tiene (TAL-243), registra la oferta y
// avisa si el precio bajó. Devuelve el resultado del ítem; nunca detiene el
// lote: un ítem con problemas vuelve como fallido con sus errores, y un error
// interno, con el código internal_error y el detalle solo en el log.
func (p *Processor) processItem(ctx context.Context, runID, storeID int32, storeName string, index int, item Item) ItemResult {
	result := ItemResult{Index: index, ExternalID: strings.TrimSpace(item.ExternalID)}
	originalCategory := item.Product.CategorySlug
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

	productURL := ""
	if offer.ProductURL != nil {
		productURL = strings.TrimSpace(*offer.ProductURL)
	}
	existing, found, err := p.runs.FindProduct(ctx, product.SKU, product.Brand, product.Model, storeID, productURL)
	if err != nil {
		return internal("find_product", err)
	}
	var productID int32
	switch {
	case found && !existing.Active:
		return fail(httpx.FieldError{Field: "product", Code: "conflict",
			Message: "el producto que coincide (por sku, marca y modelo o URL de la oferta) está dado de baja"})
	case found:
		// El catálogo existente lo cura el admin: no se modifica, salvo para
		// completar la imagen si el producto no tiene ninguna activa (TAL-243).
		productID, result.ProductID, result.Status = existing.ID, existing.PublicID, ItemUpdated
		if product.ImageURL != "" {
			if _, err := p.runs.AddImageIfMissing(ctx, productID, product.ImageURL); err != nil {
				return internal("add_image", err)
			}
		}
	default:
		var fieldErrs []httpx.FieldError
		productID, result.ProductID, fieldErrs, err = p.createProduct(
			ctx, runID, storeID, storeName, index, originalCategory, product)
		if err != nil {
			return internal("create_product", err)
		}
		if len(fieldErrs) > 0 {
			return fail(fieldErrs...)
		}
		result.Status = ItemCreated
	}

	offer.ProductID = productID
	if found && existing.OfferURL != "" {
		// Se actualiza la oferta que ya existe, aunque la tienda haya cambiado
		// los parámetros de la URL: una sola oferta y un solo historial de precios.
		offer.ProductURL = &existing.OfferURL
	}
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
func (p *Processor) createProduct(
	ctx context.Context,
	runID, storeID int32,
	storeName string,
	index int,
	originalCategory string,
	item ItemProduct,
) (int32, string, []httpx.FieldError, error) {
	if item.CategorySlug == "" {
		return 0, "", []httpx.FieldError{p.categoryMappingError(
			ctx, runID, storeID, storeName, index, originalCategory, "required")}, nil
	}
	item.CategorySlug = classifyComputacion(item.CategorySlug, item)
	category, err := p.catalog.ResolveCategory(ctx, products.CategoryRef{Slug: item.CategorySlug})
	if errors.Is(err, products.ErrCategoryNotFound) {
		return 0, "", []httpx.FieldError{p.categoryMappingError(
			ctx, runID, storeID, storeName, index, originalCategory, "not_found")}, nil
	}
	if err != nil {
		return 0, "", nil, err
	}
	if category.ItemType != categories.ItemTypeProduct || category.CategoryID == nil {
		return 0, "", []httpx.FieldError{{Field: "product.categorySlug", Code: "not_enabled",
			Message: "product.categorySlug no está habilitada para productos"}}, nil
	}

	// Sin fila en categories no hay atributos definidos: los que lleguen
	// responden unknown_field y un producto sin atributos se acepta.
	values, attrErrs := categories.ValidateAttributes(category.Attributes, item.Attributes)
	if len(attrErrs) > 0 {
		return 0, "", prefixErrors("product.", attrErrs), nil
	}

	write := products.ProductWrite{
		CategoryID: *category.CategoryID, CategoryRef: category.ID, Name: item.Name,
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

func (p *Processor) categoryMappingError(
	ctx context.Context,
	runID, storeID int32,
	storeName string,
	index int,
	original string,
	code string,
) httpx.FieldError {
	if strings.TrimSpace(storeName) == "" {
		storeName = fmt.Sprintf("ID %d", storeID)
	}
	p.logger.WarnContext(ctx, "ingestion_category_unmapped",
		slog.Int("run_id", int(runID)),
		slog.Int("index", index),
		slog.String("store", storeName),
		slog.Int("store_id", int(storeID)),
		slog.String("category_original", original))
	return httpx.FieldError{
		Field: "product.categorySlug",
		Code:  code,
		Message: fmt.Sprintf(
			"la categoría de origen %q de la tienda %q (ID %d) no tiene correspondencia con el catálogo",
			original, storeName, storeID),
	}
}

// validateItem normaliza el ítem y aplica las reglas de campos de Product y
// Pricing, con los errores prefijados por product. y offer.
func validateItem(item Item) (ItemProduct, pricing.OfferWrite, []httpx.FieldError) {
	p := item.Product
	p.SKU = strings.TrimSpace(p.SKU)
	p.Name = strings.TrimSpace(p.Name)
	p.Brand = strings.TrimSpace(p.Brand)
	p.Model = strings.TrimSpace(p.Model)
	p.CategorySlug = categories.CanonicalCategorySlug(p.CategorySlug)
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
	v.Check(p.CategorySlug == "" || (categories.ValidSlug(p.CategorySlug) &&
		!strings.HasPrefix(p.CategorySlug, "-") && !strings.HasSuffix(p.CategorySlug, "-") &&
		!strings.Contains(p.CategorySlug, "--")), "product.categorySlug", "format",
		"product.categorySlug debe tener entre 1 y 64 caracteres: minúsculas, dígitos o guiones")
	v.Check(p.ImageURL == "" || isHTTPURL(p.ImageURL), "product.imageUrl", "format",
		"product.imageUrl debe ser una URL http(s) válida")

	var offerV httpx.Validator
	offer := pricing.ValidateOfferFields(&offerV, item.Offer)

	errs := append(v.Errors(), prefixErrors("offer.", offerV.Errors())...)
	return p, offer, errs
}

// prefixErrors antepone prefix al campo y al mensaje de cada error, para que
// apunten a la ruta completa dentro del ítem.
func prefixErrors(prefix string, errs []httpx.FieldError) []httpx.FieldError {
	out := make([]httpx.FieldError, 0, len(errs))
	for _, e := range errs {
		e.Field = prefix + e.Field
		e.Message = prefix + e.Message
		out = append(out, e)
	}
	return out
}

// classifyComputacion elige la categoría final de un producto cuyo slug
// canónico es computacion, según las palabras de su nombre. Recibe el slug ya
// canónico y el ítem con el nombre original; devuelve el slug que se resuelve
// contra el catálogo y deja los demás slugs intactos.
//
// El orden importa: un accesorio (mochila, monitor, tablet…) que mencione
// "notebook" sigue en Accesorios de computación; un portátil (notebook,
// laptop, macbook…) va a la hija Notebooks; el resto se asume Computadores
// de escritorio.
func classifyComputacion(slug string, item ItemProduct) string {
	if slug != "computacion" {
		return slug
	}
	words := strings.Split(categories.NormalizeSlug(item.Name), "-")
	for i, word := range words {
		words[i] = strings.TrimFunc(word, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		})
	}
	for _, word := range words {
		switch word {
		case "mochila", "funda", "maletin", "soporte", "base", "cargador",
			"adaptador", "mouse", "teclado", "monitor", "tablet", "ipad":
			return "accesorios-de-computacion"
		}
	}
	for _, word := range words {
		switch word {
		case "macbook", "notebook", "laptop", "ultrabook", "chromebook", "portatil":
			return "notebooks"
		}
	}
	return "computadores-de-escritorio"
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

// isHTTPURL indica si value es una URL http o https válida.
func isHTTPURL(value string) bool {
	parsed, err := url.ParseRequestURI(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}
