package ingestion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/DoSafills/SOLOServis/backend/internal/categories"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/DoSafills/SOLOServis/backend/internal/pricing"
	"github.com/DoSafills/SOLOServis/backend/internal/products"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// RunStore es lo que Ingestion necesita de sus tablas; *Repository la implementa.
// Las consultas del ítem van por la transacción del ítem (TAL-191): producto,
// imagen y oferta se aplican juntos o no se aplica nada.
type RunStore interface {
	OpenRun(ctx context.Context, name string, baseURL *string, storeID int32, scraperType, targetType string) (int32, error)
	GetRun(ctx context.Context, id int32) (Run, error)
	ListRuns(ctx context.Context, limit, offset int) ([]Run, int64, error)
	FinishRun(ctx context.Context, id int32, status string, message *string) error
	ItemsTarget(ctx context.Context, id int32) (int32, error)
	ServiceTarget(ctx context.Context, id int32) (Run, error)
	AddCounters(ctx context.Context, id int32, found, processed, failed int) error
	RecentFound(ctx context.Context, storeID, excludeRunID int32, limit int) ([]int32, error)
	SaveScraped(ctx context.Context, runID int32, externalID, externalURL *string, entityType string, raw json.RawMessage) error
	SaveScrapedBatch(ctx context.Context, runID int32, items []ScrapedItem) error
	BeginTx(ctx context.Context) (pgx.Tx, error)
	FindProductTx(ctx context.Context, tx pgx.Tx, sku, brand, model, name, categorySlug string, storeID int32, productURL string) (ExistingProduct, bool, error)
	AddImageIfMissingTx(ctx context.Context, tx pgx.Tx, productID int32, imageURL string) (bool, error)
	AddDescriptionIfMissingTx(ctx context.Context, tx pgx.Tx, productID int32, description string) (bool, error)
	EnsureBrandTx(ctx context.Context, tx pgx.Tx, name string) (int32, error)
	ProductIDTx(ctx context.Context, tx pgx.Tx, publicID pgtype.UUID) (int32, error)
	ResolveServiceCategory(ctx context.Context, slug string) (ServiceCategory, error)
	EnsureProviderTx(ctx context.Context, tx pgx.Tx, slug, name string) (int32, error)
	FindServiceOfferTx(ctx context.Context, tx pgx.Tx, providerID int32, externalID, serviceURL string) (int32, bool, error)
	CreateServiceTx(ctx context.Context, tx pgx.Tx, write ServiceWrite) (int32, string, error)
	ServicePublicIDTx(ctx context.Context, tx pgx.Tx, id int32) (string, error)
	UpsertServiceOfferTx(ctx context.Context, tx pgx.Tx, write ServiceOfferWrite) (int32, error)
}

// Catalog crea productos con las reglas de Product API (TAL-70);
// *products.Repository la implementa.
type Catalog interface {
	ResolveCategory(ctx context.Context, ref products.CategoryRef) (products.WriteCategory, error)
	CreateTx(ctx context.Context, tx pgx.Tx, product products.ProductWrite) (pgtype.UUID, error)
}

// Offers registra precios con las reglas de Pricing API (TAL-74);
// *pricing.Repository la implementa.
type Offers interface {
	UpsertOfferTx(ctx context.Context, tx pgx.Tx, w pricing.OfferWrite) (int32, bool, *string, error)
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
// detiene el lote. Al final guarda todos los ítems en scraped_data de una vez
// (TAL-205) y suma los contadores. Recibe la ejecución, la tienda y los ítems
// del lote; devuelve el resultado de cada ítem y los totales.
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
	}
	p.save(ctx, runID, items)
	if err := p.runs.AddCounters(ctx, runID, result.Received, result.Processed, result.Failed); err != nil {
		p.logger.ErrorContext(ctx, "ingestion_counters_failed", slog.Int("run_id", int(runID)), slog.String("error", err.Error()))
	}
	return result
}

// processItem procesa el ítem número index del lote runID para la tienda
// storeID dentro de una sola transacción (TAL-191): busca si el producto ya
// existe (o lo crea), completa la imagen de un producto existente que no tiene
// (TAL-243) y registra la oferta con su historial de precios. Si algo falla,
// la transacción se revierte completa: no quedan productos sin oferta ni
// ofertas sin producto. El aviso de bajada de precio se envía después del
// commit. Devuelve el resultado del ítem; nunca detiene el lote: un ítem con
// problemas vuelve como fallido con sus errores, y un error interno, con el
// código internal_error y el detalle solo en el log.
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
	// La búsqueda por nombre usa la misma categoría con la que se crearía el
	// producto, incluida la clasificación de Computación por nombre (TAL-191).
	categorySlug := product.CategorySlug
	if categorySlug != "" {
		categorySlug = classifyComputacion(categorySlug, product)
	}

	// Un solo ítem, una sola transacción: producto, imagen, oferta e historial.
	tx, err := p.runs.BeginTx(ctx)
	if err != nil {
		return internal("begin_tx", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	existing, found, err := p.runs.FindProductTx(ctx, tx, product.SKU, product.Brand, product.Model,
		product.Name, categorySlug, storeID, productURL)
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
			if _, err := p.runs.AddImageIfMissingTx(ctx, tx, productID, product.ImageURL); err != nil {
				return internal("add_image", err)
			}
		}
		// Igual que la imagen (TAL-243), la descripción se completa solo si el
		// producto no tiene una: nunca reemplaza el texto curado por un admin.
		if product.Description != "" {
			if _, err := p.runs.AddDescriptionIfMissingTx(ctx, tx, productID, product.Description); err != nil {
				return internal("add_description", err)
			}
		}
	default:
		var fieldErrs []httpx.FieldError
		productID, result.ProductID, fieldErrs, err = p.createProduct(
			ctx, tx, runID, storeID, storeName, index, originalCategory, product)
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
	offerID, _, previous, err := p.offers.UpsertOfferTx(ctx, tx, offer)
	if err != nil {
		return internal("upsert_offer", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return internal("commit", err)
	}
	committed = true

	result.OfferID = offerID
	// Solo una bajada de precio puede cruzar un precio objetivo: si el precio
	// subió o quedó igual (lo habitual en cada corrida), no se consulta nada más.
	if previous != nil && priceDropped(*previous, offer.Price) {
		// El aviso se envía solo con la oferta ya guardada.
		saved, err := p.offers.GetOffer(ctx, offerID)
		if err != nil {
			return internal("get_offer", err)
		}
		pricing.NotifyPriceDrop(ctx, p.offers, p.notifier, p.logger, saved, *previous)
	}
	return result
}

// ProcessServices aplica cada servicio del lote por separado, igual que
// Process con los productos: un ítem inválido no detiene a los demás. Al final
// guarda los ítems en scraped_data como entity_type 'service' y suma los
// contadores de la corrida. Recibe la ejecución y los ítems; devuelve el
// resultado de cada uno y los totales (issue #191).
func (p *Processor) ProcessServices(ctx context.Context, runID int32, items []ServiceItem) BatchResult {
	result := BatchResult{RunID: runID, Received: len(items), Results: make([]ItemResult, 0, len(items))}
	for i, item := range items {
		r := p.processServiceItem(ctx, runID, i, item)
		if r.Status == ItemFailed {
			result.Failed++
		} else {
			result.Processed++
		}
		result.Results = append(result.Results, r)
	}
	p.saveServices(ctx, runID, items)
	if err := p.runs.AddCounters(ctx, runID, result.Received, result.Processed, result.Failed); err != nil {
		p.logger.ErrorContext(ctx, "ingestion_counters_failed", slog.Int("run_id", int(runID)), slog.String("error", err.Error()))
	}
	return result
}

// processServiceItem procesa un servicio del lote dentro de una sola
// transacción: resuelve la categoría y valida sus atributos, asegura el
// proveedor, reutiliza el servicio si la URL de la oferta ya existe o lo crea,
// y registra la oferta con su historial si el precio cambió. Si algo falla, la
// transacción se revierte completa. Devuelve el resultado del ítem; nunca
// detiene el lote.
func (p *Processor) processServiceItem(ctx context.Context, runID int32, index int, item ServiceItem) ItemResult {
	result := ItemResult{Index: index, ExternalID: strings.TrimSpace(item.ExternalID)}
	fail := func(errs ...httpx.FieldError) ItemResult {
		result.Status, result.Errors = ItemFailed, errs
		return result
	}
	internal := func(stage string, err error) ItemResult {
		p.logger.ErrorContext(ctx, "ingestion_service_failed", slog.Int("run_id", int(runID)),
			slog.Int("index", index), slog.String("stage", stage), slog.String("error", err.Error()))
		return fail(httpx.FieldError{Field: "", Code: codeInternalError,
			Message: "error interno al procesar el servicio; ver el log del servicio"})
	}

	item, offer, errs := validateServiceItem(item)
	if len(errs) > 0 {
		return fail(errs...)
	}

	category, err := p.runs.ResolveServiceCategory(ctx, item.Service.CategorySlug)
	if errors.Is(err, ErrServiceCategoryNotFound) {
		return fail(httpx.FieldError{Field: "service.categorySlug", Code: "not_found",
			Message: "service.categorySlug no existe en el catálogo de categorías"})
	}
	if err != nil {
		return internal("resolve_category", err)
	}
	if category.ItemType != "service" {
		return fail(httpx.FieldError{Field: "service.categorySlug", Code: "not_enabled",
			Message: "service.categorySlug no está habilitada para servicios"})
	}
	if category.LegacyID == nil {
		return fail(httpx.FieldError{Field: "service.categorySlug", Code: "not_enabled",
			Message: "service.categorySlug no tiene equivalencia en el catálogo de servicios"})
	}

	values, attrErrs := categories.ValidateAttributes(category.Attributes, item.Service.Attributes)
	if len(attrErrs) > 0 {
		return fail(prefixErrors("service.", attrErrs)...)
	}

	tx, err := p.runs.BeginTx(ctx)
	if err != nil {
		return internal("begin_tx", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	providerID, err := p.runs.EnsureProviderTx(ctx, tx, item.Provider.Slug, item.Provider.Name)
	if err != nil {
		return internal("ensure_provider", err)
	}

	serviceID, found, err := p.runs.FindServiceOfferTx(ctx, tx, providerID, item.ExternalID, item.ExternalURL)
	if err != nil {
		return internal("find_service", err)
	}
	if found {
		result.ServiceID, err = p.runs.ServicePublicIDTx(ctx, tx, serviceID)
		if err != nil {
			return internal("service_public_id", err)
		}
		result.Status = ItemUpdated
	} else {
		serviceID, result.ServiceID, err = p.runs.CreateServiceTx(ctx, tx, ServiceWrite{
			CategoryID: *category.LegacyID, CategoryRef: category.ID,
			Name: item.Service.Name, Description: item.Service.Description,
			ImageURL: item.Service.ImageURL, Attributes: values,
		})
		if err != nil {
			return internal("create_service", err)
		}
		result.Status = ItemCreated
	}

	offer.ServiceID, offer.ProviderID = serviceID, providerID
	offer.ServiceURL = item.ExternalURL
	if item.ExternalID != "" {
		offer.ExternalID = &item.ExternalID
	}
	offerID, err := p.runs.UpsertServiceOfferTx(ctx, tx, offer)
	if err != nil {
		return internal("upsert_service_offer", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return internal("commit", err)
	}
	committed = true
	result.OfferID = offerID
	return result
}

// validateServiceItem normaliza el ítem de servicio y valida sus campos con
// las mismas reglas que el resto de la API. Devuelve el ítem normalizado, la
// oferta lista para guardar (sin ids) y los errores por campo, prefijados por
// provider., service. u offer.
func validateServiceItem(item ServiceItem) (ServiceItem, ServiceOfferWrite, []httpx.FieldError) {
	item.ExternalID = strings.TrimSpace(item.ExternalID)
	item.ExternalURL = strings.TrimSpace(item.ExternalURL)
	item.Provider.Slug = strings.TrimSpace(item.Provider.Slug)
	item.Provider.Name = strings.TrimSpace(item.Provider.Name)
	item.Service.Name = strings.TrimSpace(item.Service.Name)
	item.Service.CategorySlug = categories.CanonicalCategorySlug(item.Service.CategorySlug)
	item.Service.Description = strings.TrimSpace(item.Service.Description)
	item.Service.ImageURL = strings.TrimSpace(item.Service.ImageURL)

	var v httpx.Validator
	v.MaxLength("externalId", item.ExternalID, MaxExternalID)
	v.Required("externalUrl", item.ExternalURL)
	v.MaxLength("externalUrl", item.ExternalURL, MaxServiceURL)
	v.Check(item.ExternalURL == "" || isHTTPURL(item.ExternalURL), "externalUrl", "format",
		"externalUrl debe ser una URL http(s) válida")

	v.Required("provider.slug", item.Provider.Slug)
	v.MaxLength("provider.slug", item.Provider.Slug, MaxProviderSlug)
	v.Check(item.Provider.Slug == "" || validServiceSlug(item.Provider.Slug), "provider.slug", "format",
		"provider.slug debe tener entre 1 y 64 caracteres: minúsculas, dígitos o guiones")
	v.Required("provider.name", item.Provider.Name)
	v.MaxLength("provider.name", item.Provider.Name, MaxProviderName)

	v.Required("service.name", item.Service.Name)
	v.MaxLength("service.name", item.Service.Name, MaxServiceName)
	v.Required("service.categorySlug", item.Service.CategorySlug)
	v.MaxLength("service.categorySlug", item.Service.CategorySlug, 64)
	v.Check(item.Service.CategorySlug == "" || validServiceSlug(item.Service.CategorySlug),
		"service.categorySlug", "format",
		"service.categorySlug debe tener entre 1 y 64 caracteres: minúsculas, dígitos o guiones")
	v.MaxLength("service.description", item.Service.Description, products.MaxDescriptionLength)
	v.Check(item.Service.ImageURL == "" || isHTTPURL(item.Service.ImageURL), "service.imageUrl", "format",
		"service.imageUrl debe ser una URL http(s) válida")

	var offerV httpx.Validator
	offer := ServiceOfferWrite{Currency: "CLP", Available: true}
	if item.Offer.Price == nil {
		offerV.Add("price", "required", "price es obligatorio")
	} else {
		offer.Price = serviceMoney(&offerV, "price", item.Offer.Price)
	}

	offer.Currency = strings.TrimSpace(item.Offer.Currency)
	if offer.Currency == "" {
		offer.Currency = "CLP"
	}
	offerV.Check(products.ValidateCurrencyCode(offer.Currency) == nil, "currency", "format",
		"currency debe tener exactamente tres letras mayúsculas (ISO 4217)")

	offer.BillingPeriod = optionalText(item.Offer.BillingPeriod, "monthly")
	if offer.BillingPeriod != nil {
		offerV.MaxLength("billingPeriod", *offer.BillingPeriod, 30)
		offerV.Check(serviceBillingPeriods[*offer.BillingPeriod],
			"billingPeriod", "one_of", "billingPeriod debe ser monthly, yearly o one_time")
	}

	if item.Offer.InstallationCost != nil {
		cost := serviceMoney(&offerV, "installationCost", item.Offer.InstallationCost)
		offer.InstallationCost = &cost
	}
	offer.ContractPeriod = optionalText(item.Offer.ContractPeriod, "")
	checkOptionalLength(&offerV, "contractPeriod", offer.ContractPeriod, 50)
	offer.CoverageSummary = optionalText(item.Offer.CoverageSummary, "")
	checkOptionalLength(&offerV, "coverageSummary", offer.CoverageSummary, MaxServiceSummary)
	offer.AdditionalCosts = optionalText(item.Offer.AdditionalCosts, "")
	checkOptionalLength(&offerV, "additionalCosts", offer.AdditionalCosts, MaxServiceSummary)
	if item.Offer.Available != nil {
		offer.Available = *item.Offer.Available
	}

	errs := append(v.Errors(), prefixErrors("offer.", offerV.Errors())...)
	return item, offer, errs
}

// serviceBillingPeriods son los períodos de cobro que entiende el catálogo.
var serviceBillingPeriods = map[string]bool{"monthly": true, "yearly": true, "one_time": true}

// serviceMoneyPattern acota lo que cabe en service_offer.price NUMERIC(12,2).
var serviceMoneyPattern = regexp.MustCompile(`^\d{1,10}(\.\d{1,2})?$`)

// serviceMoney valida un monto de la oferta de servicio y devuelve su texto
// decimal. Registra el error por campo cuando no es válido.
func serviceMoney(v *httpx.Validator, field string, value *json.Number) string {
	text := strings.TrimSpace(value.String())
	switch {
	case strings.HasPrefix(text, "-"):
		v.Add(field, "min", field+" debe ser mayor o igual a 0")
		return ""
	case !serviceMoneyPattern.MatchString(text):
		v.Add(field, "format", field+" debe ser un monto con hasta 10 enteros y 2 decimales")
		return ""
	}
	return text
}

// optionalText recorta el texto y devuelve un puntero; con el texto vacío (y
// sin fallback) devuelve nil para que la base guarde NULL, no "".
func optionalText(value, fallback string) *string {
	text := strings.TrimSpace(value)
	if text == "" {
		text = fallback
	}
	if text == "" {
		return nil
	}
	return &text
}

// checkOptionalLength valida el largo de un texto opcional ya normalizado.
func checkOptionalLength(v *httpx.Validator, field string, value *string, max int) {
	if value != nil {
		v.MaxLength(field, *value, max)
	}
}

// validServiceSlug comprueba el formato de slug de proveedor y categoría.
func validServiceSlug(value string) bool {
	if value == "" || len(value) > MaxProviderSlug {
		return false
	}
	if strings.HasPrefix(value, "-") || strings.HasSuffix(value, "-") || strings.Contains(value, "--") {
		return false
	}
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}

// saveServices guarda los ítems de servicio tal como llegaron en scraped_data
// con entity_type 'service'. Un fallo del lote se reintenta ítem por ítem y
// solo se registra en el log, como en el camino de producto.
func (p *Processor) saveServices(ctx context.Context, runID int32, items []ServiceItem) {
	rows := make([]ScrapedItem, 0, len(items))
	for _, item := range items {
		raw, err := json.Marshal(item)
		if err != nil {
			p.logger.WarnContext(ctx, "ingestion_save_scraped_failed", slog.Int("run_id", int(runID)), slog.String("error", err.Error()))
			continue
		}
		row := ScrapedItem{EntityType: TargetService, Raw: raw}
		if id := strings.TrimSpace(item.ExternalID); id != "" && len(id) <= MaxExternalID {
			row.ExternalID = &id
		}
		if u := strings.TrimSpace(item.ExternalURL); u != "" {
			row.ExternalURL = &u
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return
	}
	err := p.runs.SaveScrapedBatch(ctx, runID, rows)
	if err == nil {
		return
	}
	p.logger.WarnContext(ctx, "ingestion_save_scraped_batch_failed", slog.Int("run_id", int(runID)), slog.String("error", err.Error()))
	for _, row := range rows {
		if err := p.runs.SaveScraped(ctx, runID, row.ExternalID, row.ExternalURL, row.EntityType, row.Raw); err != nil {
			p.logger.WarnContext(ctx, "ingestion_save_scraped_failed", slog.Int("run_id", int(runID)), slog.String("error", err.Error()))
		}
	}
}

// createProduct valida categoría y atributos con TAL-70/72 y crea el producto
// dentro de la transacción del ítem (tx): si la oferta falla, el producto y su
// marca no quedan en la base.
func (p *Processor) createProduct(
	ctx context.Context,
	tx pgx.Tx,
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
		brandID, err := p.runs.EnsureBrandTx(ctx, tx, item.Brand)
		if err != nil {
			return 0, "", nil, err
		}
		write.BrandID = &brandID
	}

	publicID, err := p.catalog.CreateTx(ctx, tx, write)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return 0, "", []httpx.FieldError{{Field: "product.sku", Code: "conflict", Message: "ya existe un producto con ese sku"}}, nil
	}
	if err != nil {
		return 0, "", nil, err
	}
	id, err := p.runs.ProductIDTx(ctx, tx, publicID)
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

// save guarda los ítems tal como llegaron (normalizados por el scraper) en
// scraped_data, en el mismo orden y con un solo INSERT por lote (TAL-205).
// Recibe la ejecución y los ítems del lote. Un ítem que no se puede convertir
// a JSON se omite; un fallo al guardar solo se registra en el log, como antes.
func (p *Processor) save(ctx context.Context, runID int32, items []Item) {
	rows := make([]ScrapedItem, 0, len(items))
	for _, item := range items {
		raw, err := json.Marshal(item)
		if err != nil {
			p.logger.WarnContext(ctx, "ingestion_save_scraped_failed", slog.Int("run_id", int(runID)), slog.String("error", err.Error()))
			continue
		}
		row := ScrapedItem{EntityType: TargetProduct, Raw: raw}
		if id := strings.TrimSpace(item.ExternalID); id != "" && len(id) <= MaxExternalID {
			row.ExternalID = &id
		}
		if u := strings.TrimSpace(item.Offer.ProductURL); u != "" {
			row.ExternalURL = &u
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return
	}
	err := p.runs.SaveScrapedBatch(ctx, runID, rows)
	if err == nil {
		return
	}
	p.logger.WarnContext(ctx, "ingestion_save_scraped_batch_failed", slog.Int("run_id", int(runID)), slog.String("error", err.Error()))
	// Si el INSERT del lote falla (por ejemplo, un ítem que jsonb rechaza),
	// se guarda ítem por ítem para no perder los demás, como antes de TAL-205.
	for _, row := range rows {
		if err := p.runs.SaveScraped(ctx, runID, row.ExternalID, row.ExternalURL, TargetProduct, row.Raw); err != nil {
			p.logger.WarnContext(ctx, "ingestion_save_scraped_failed", slog.Int("run_id", int(runID)), slog.String("error", err.Error()))
		}
	}
}

// priceDropped indica si el precio nuevo es menor que el anterior. Recibe los
// dos precios como texto decimal (el anterior viene de NUMERIC(12,2) y el
// nuevo ya validado) y los compara como números exactos, sin redondeo. Si
// alguno no se puede leer devuelve true, para que la consulta de avisos decida
// como antes de TAL-205 en vez de perder un aviso.
func priceDropped(previous, current string) bool {
	old, okOld := new(big.Rat).SetString(strings.TrimSpace(previous))
	now, okNow := new(big.Rat).SetString(strings.TrimSpace(current))
	if !okOld || !okNow {
		return true
	}
	return now.Cmp(old) < 0
}

// isHTTPURL indica si value es una URL http o https válida.
func isHTTPURL(value string) bool {
	parsed, err := url.ParseRequestURI(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}
