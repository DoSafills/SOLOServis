package ingestion

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/DoSafills/SOLOServis/backend/internal/categories"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// rowQuerier son las operaciones que comparten el pool y una transacción.
type rowQuerier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Repository accede a las tablas de scraping y a lo que Ingestion necesita
// del catálogo (marcas, búsqueda de productos). Consultas parametrizadas.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository crea el repositorio de ingestion sobre el pool de PostgreSQL.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// OpenRun crea (o reutiliza) la fuente y su configuración y abre una
// ejecución en estado running. Recibe el nombre de la fuente, su URL base, la
// tienda (0 en corridas de servicio), el tipo de scraper y el objetivo
// ("product" o "service").
//
// Una corrida de producto exige una tienda activa y enlaza la fuente a ella.
// Una corrida de servicio deja `store_id` NULL y marca la fuente como
// `provider`: el proveedor viaja en cada ítem, no en la fuente (issue #191).
func (r *Repository) OpenRun(ctx context.Context, name string, baseURL *string, storeID int32, scraperType, targetType string) (int32, error) {
	if targetType == "" {
		targetType = TargetProduct
	}
	sourceType := "store"
	var storeArg *int32
	if targetType == TargetService {
		sourceType = "provider"
	} else if storeID > 0 {
		storeArg = &storeID
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	if storeArg != nil {
		var storeExists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM store WHERE id = $1 AND active)`, *storeArg).Scan(&storeExists); err != nil {
			return 0, err
		}
		if !storeExists {
			return 0, ErrStoreNotFound
		}
	}

	var (
		sourceID      int32
		sourceStoreID *int32
		active        bool
	)
	if err := tx.QueryRow(ctx, `
		INSERT INTO data_source (name, base_url, source_type, store_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (name) DO UPDATE
		    SET base_url = COALESCE(EXCLUDED.base_url, data_source.base_url), updated_at = NOW()
		RETURNING id, store_id, active`, name, baseURL, sourceType, storeArg).Scan(&sourceID, &sourceStoreID, &active); err != nil {
		return 0, err
	}
	switch {
	case !active:
		return 0, ErrSourceInactive
	case sourceStoreID == nil && storeArg != nil:
		// Fuente existente sin tienda (p. ej. creada antes de TAL-77): se enlaza.
		if _, err := tx.Exec(ctx, `UPDATE data_source SET store_id = $2 WHERE id = $1`, sourceID, *storeArg); err != nil {
			return 0, err
		}
	case storeArg != nil && *sourceStoreID != *storeArg:
		return 0, ErrSourceStoreMismatch
	}

	var configID int32
	err = tx.QueryRow(ctx, `
		SELECT id FROM scraper_config
		WHERE data_source_id = $1 AND scraper_type = $2 AND target_type = $3 AND active
		ORDER BY id LIMIT 1`, sourceID, scraperType, targetType).Scan(&configID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `
			INSERT INTO scraper_config (data_source_id, scraper_type, target_type)
			VALUES ($1, $2, $3) RETURNING id`, sourceID, scraperType, targetType).Scan(&configID)
	}
	if err != nil {
		return 0, err
	}

	var runID int32
	if err := tx.QueryRow(ctx, `
		INSERT INTO scrape_run (scraper_config_id, status) VALUES ($1, 'running') RETURNING id`, configID).Scan(&runID); err != nil {
		return 0, err
	}
	return runID, tx.Commit(ctx)
}

const runColumnsSQL = `
SELECT r.id, ds.name, ds.base_url, ds.store_id, sc.scraper_type, COALESCE(sc.target_type, 'product'),
       r.status, r.started_at, r.finished_at,
       COALESCE(r.records_found, 0), COALESCE(r.records_processed, 0), COALESCE(r.records_failed, 0),
       r.error_message
FROM scrape_run r
JOIN scraper_config sc ON sc.id = r.scraper_config_id
JOIN data_source ds ON ds.id = sc.data_source_id`

// scanRun convierte una fila de la consulta de ejecuciones en un Run.
func scanRun(row pgx.Row) (Run, error) {
	var run Run
	err := row.Scan(&run.ID, &run.Source, &run.SourceURL, &run.StoreID, &run.ScraperType, &run.TargetType,
		&run.Status, &run.StartedAt, &run.FinishedAt, &run.RecordsFound, &run.RecordsProcessed,
		&run.RecordsFailed, &run.ErrorMessage)
	return run, err
}

// GetRun devuelve una ejecución, o ErrRunNotFound.
func (r *Repository) GetRun(ctx context.Context, id int32) (Run, error) {
	run, err := scanRun(r.db.QueryRow(ctx, runColumnsSQL+` WHERE r.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Run{}, ErrRunNotFound
	}
	return run, err
}

// ListRuns devuelve las ejecuciones, la más reciente primero, y el total.
func (r *Repository) ListRuns(ctx context.Context, limit, offset int) ([]Run, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM scrape_run`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(ctx, runColumnsSQL+` ORDER BY r.started_at DESC, r.id DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	runs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Run, error) { return scanRun(row) })
	return runs, total, err
}

// FinishRun cierra una ejecución abierta: ErrRunNotFound o ErrRunClosed.
func (r *Repository) FinishRun(ctx context.Context, id int32, status string, message *string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE scrape_run SET status = $2, error_message = $3, finished_at = NOW()
		WHERE id = $1 AND status IN ('pending', 'running')`, id, status, message)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	if _, err := r.GetRun(ctx, id); err != nil {
		return err
	}
	return ErrRunClosed
}

// ItemsTarget valida que la ejecución acepte productos y devuelve la tienda de
// su fuente: ErrRunNotFound, ErrRunClosed, ErrRunWithoutStore o
// ErrRunTargetMismatch si la corrida es de servicios.
func (r *Repository) ItemsTarget(ctx context.Context, id int32) (int32, error) {
	run, err := r.GetRun(ctx, id)
	if err != nil {
		return 0, err
	}
	if run.Status != StatusRunning && run.Status != "pending" {
		return 0, ErrRunClosed
	}
	if run.TargetType == TargetService {
		return 0, ErrRunTargetMismatch
	}
	if run.StoreID == nil {
		return 0, ErrRunWithoutStore
	}
	return *run.StoreID, nil
}

// ServiceTarget valida que la ejecución acepte servicios y la devuelve.
// Las corridas de servicio no tienen tienda, así que no se exige store_id.
// Devuelve ErrRunNotFound, ErrRunClosed o ErrRunTargetMismatch.
func (r *Repository) ServiceTarget(ctx context.Context, id int32) (Run, error) {
	run, err := r.GetRun(ctx, id)
	if err != nil {
		return Run{}, err
	}
	if run.Status != StatusRunning && run.Status != "pending" {
		return Run{}, ErrRunClosed
	}
	if run.TargetType != TargetService {
		return Run{}, ErrRunTargetMismatch
	}
	return run, nil
}

// AddCounters suma los resultados de un lote a la ejecución.
func (r *Repository) AddCounters(ctx context.Context, id int32, found, processed, failed int) error {
	_, err := r.db.Exec(ctx, `
		UPDATE scrape_run
		SET records_found = COALESCE(records_found, 0) + $2,
		    records_processed = COALESCE(records_processed, 0) + $3,
		    records_failed = COALESCE(records_failed, 0) + $4
		WHERE id = $1`, id, found, processed, failed)
	return err
}

// RecentFound devuelve los `records_found` de las últimas corridas exitosas de
// una tienda, sin contar la corrida actual.
//
// Es la referencia con la que se mide si la corrida de hoy entregó mucho menos
// que lo habitual (TAL-211). Una corrida exitosa que no encontró nada no sirve
// de referencia: se excluye para que un vacío puntual no baje la mediana y
// esconda la próxima caída (TAL-172). Devuelve una lista vacía cuando la tienda
// no tiene historial, que el evaluador interpreta como «sin base de
// comparación».
func (r *Repository) RecentFound(ctx context.Context, storeID, excludeRunID int32, limit int) ([]int32, error) {
	rows, err := r.db.Query(ctx, `
		SELECT COALESCE(r.records_found, 0)
		FROM scrape_run r
		JOIN scraper_config sc ON sc.id = r.scraper_config_id
		JOIN data_source ds ON ds.id = sc.data_source_id
		WHERE ds.store_id = $1 AND r.id <> $2 AND r.status = 'success'
		  AND COALESCE(r.records_found, 0) > 0
		ORDER BY r.started_at DESC, r.id DESC
		LIMIT $3`, storeID, excludeRunID, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[int32])
}

// SaveScraped guarda el ítem recibido (JSON normalizado) para auditoría.
// Recibe la ejecución, los identificadores en el origen, el tipo de entidad
// ("product" o "service") y el JSON del ítem.
func (r *Repository) SaveScraped(ctx context.Context, runID int32, externalID, externalURL *string, entityType string, raw json.RawMessage) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO scraped_data (scrape_run_id, external_id, external_url, entity_type, raw_data)
		VALUES ($1, $2, $3, $4, $5)`, runID, externalID, externalURL, entityType, raw)
	return err
}

// ScrapedItem es una fila de scraped_data: el ítem recibido como JSON y sus
// identificadores en el origen (nil si no vinieron). EntityType vacío se
// guarda como 'product' para conservar el comportamiento anterior.
type ScrapedItem struct {
	ExternalID  *string
	ExternalURL *string
	EntityType  string
	Raw         json.RawMessage
}

// SaveScrapedBatch guarda los ítems de un lote en scraped_data con un solo
// INSERT (TAL-205), en el orden recibido. Recibe la ejecución y las filas;
// devuelve el error de la base si el INSERT falla, y entonces no guarda
// ninguna. El JSON viaja como texto y se convierte a jsonb en la consulta.
func (r *Repository) SaveScrapedBatch(ctx context.Context, runID int32, items []ScrapedItem) error {
	externalIDs := make([]*string, len(items))
	externalURLs := make([]*string, len(items))
	entityTypes := make([]string, len(items))
	raws := make([]string, len(items))
	for i, item := range items {
		entityType := item.EntityType
		if entityType == "" {
			entityType = TargetProduct
		}
		externalIDs[i], externalURLs[i], entityTypes[i], raws[i] = item.ExternalID, item.ExternalURL, entityType, string(item.Raw)
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO scraped_data (scrape_run_id, external_id, external_url, entity_type, raw_data)
		SELECT $1, t.external_id, t.external_url, t.entity_type, t.raw::jsonb
		FROM unnest($2::text[], $3::text[], $4::text[], $5::text[]) WITH ORDINALITY
		     AS t(external_id, external_url, entity_type, raw, position)
		ORDER BY t.position`, runID, externalIDs, externalURLs, entityTypes, raws)
	return err
}

// BeginTx abre la transacción con la que Processor aplica un ítem completo
// (producto, imagen, oferta e historial) de forma atómica (TAL-191).
func (r *Repository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.db.Begin(ctx)
}

// FindProduct busca un producto ya catalogado: primero por SKU y, si no lo
// encuentra (o no hay SKU), por marca + modelo sin distinguir mayúsculas.
// Luego usa la identidad que el scraper siempre envía: una oferta de la misma
// tienda con la misma URL (TAL-169), comparada sin parámetros ni fragmento
// porque las tiendas agregan parámetros de seguimiento que cambian en cada
// corrida (Ripley: pos, catPos, ps). Por último, cuando el ítem no trae SKU ni
// modelo (caso normal del scraper), reconoce el mismo producto por marca,
// nombre normalizado y categoría, siempre dentro de la misma tienda: es el
// caso de las variantes de color, que la tienda publica con otra URL (TAL-191).
// Prefiere los activos.
func (r *Repository) FindProduct(ctx context.Context, sku, brand, model, name, categorySlug string, storeID int32, productURL string) (ExistingProduct, bool, error) {
	return findProduct(ctx, r.db, sku, brand, model, name, categorySlug, storeID, productURL)
}

// FindProductTx es FindProduct dentro de la transacción del ítem (TAL-191):
// la búsqueda y el alta ven el mismo estado de la base.
func (r *Repository) FindProductTx(ctx context.Context, tx pgx.Tx, sku, brand, model, name, categorySlug string, storeID int32, productURL string) (ExistingProduct, bool, error) {
	return findProduct(ctx, tx, sku, brand, model, name, categorySlug, storeID, productURL)
}

// findProduct ejecuta la búsqueda por identidad sobre q (pool o transacción).
func findProduct(ctx context.Context, q rowQuerier, sku, brand, model, name, categorySlug string, storeID int32, productURL string) (ExistingProduct, bool, error) {
	var p ExistingProduct
	if sku != "" {
		err := q.QueryRow(ctx, `SELECT id, public_id::text, active FROM product WHERE sku = $1`, sku).
			Scan(&p.ID, &p.PublicID, &p.Active)
		if err == nil {
			return p, true, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return ExistingProduct{}, false, err
		}
	}
	if brand != "" && model != "" {
		err := q.QueryRow(ctx, `
			SELECT p.id, p.public_id::text, p.active
			FROM product p JOIN brand b ON b.id = p.brand_id
			WHERE lower(b.name) = lower($1) AND lower(p.model) = lower($2)
			ORDER BY p.active DESC, p.id LIMIT 1`, brand, model).Scan(&p.ID, &p.PublicID, &p.Active)
		if err == nil {
			return p, true, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return ExistingProduct{}, false, err
		}
	}
	if productURL != "" {
		// Misma normalización que el índice único de product_offer (migración 009),
		// cortada en el "?". Una URL que no se puede normalizar da NULL y no
		// coincide con nada. Se devuelve la URL guardada para reutilizar esa oferta.
		err := q.QueryRow(ctx, `
			SELECT p.id, p.public_id::text, p.active, o.product_url
			FROM product_offer o JOIN product p ON p.id = o.product_id
			WHERE o.store_id = $1
			  AND split_part(normalize_product_offer_url(o.product_url), '?', 1) =
			      split_part(normalize_product_offer_url($2), '?', 1)
			ORDER BY p.active DESC, p.id, o.id LIMIT 1`, storeID, productURL).Scan(&p.ID, &p.PublicID, &p.Active, &p.OfferURL)
		if err == nil {
			return p, true, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return ExistingProduct{}, false, err
		}
	}
	if brand == "" || name == "" || categorySlug == "" {
		return ExistingProduct{}, false, nil
	}
	// Último recurso (TAL-191): misma tienda, misma marca, misma categoría y
	// mismo nombre (sin distinguir mayúsculas ni espacios repetidos). Las
	// variantes de color de un mismo producto comparten nombre y cambian la URL,
	// así que sus ofertas quedan bajo un solo producto. Dos productos distintos
	// de la misma marca y categoría tienen nombres distintos y no se unen.
	var storeHasProduct bool
	err := q.QueryRow(ctx, `
		SELECT p.id, p.public_id::text, p.active, EXISTS (
			SELECT 1 FROM product_offer o WHERE o.product_id = p.id AND o.store_id = $4)
		FROM product p
		JOIN brand b ON b.id = p.brand_id
		JOIN categories c ON c.id = p.category_ref
		WHERE lower(b.name) = lower($1)
		  AND c.slug = $2
		  AND lower(trim(regexp_replace(p.name, '[[:space:]]+', ' ', 'g'))) =
		      lower(trim(regexp_replace($3, '[[:space:]]+', ' ', 'g')))
		ORDER BY p.active DESC, p.id LIMIT 1`, brand, categorySlug, name, storeID).
		Scan(&p.ID, &p.PublicID, &p.Active, &storeHasProduct)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !storeHasProduct) {
		return ExistingProduct{}, false, nil
	}
	if err != nil {
		return ExistingProduct{}, false, err
	}
	return p, true, nil
}

// AddImageIfMissing completa la imagen de un producto ya catalogado (TAL-243).
// Recibe el id interno del producto y la URL de imagen que envió la tienda, ya
// validada como http(s). La guarda como imagen principal (sort_order 0, con el
// nombre del producto como texto alternativo; ambos son VARCHAR(255)) solo si
// el producto no tiene ninguna imagen activa y si esa URL no fue desactivada
// antes por un admin: nunca reemplaza ni reactiva lo curado. Devuelve true si
// la guardó.
//
// Bloquea la fila del producto (FOR NO KEY UPDATE, que no choca con las
// ofertas que lo referencian) para que dos lotes simultáneos no inserten la
// imagen dos veces: product_image no tiene una restricción única que lo evite.
func (r *Repository) AddImageIfMissing(ctx context.Context, productID int32, imageURL string) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	saved, err := addImageIfMissing(ctx, tx, productID, imageURL)
	if err != nil {
		return false, err
	}
	return saved, tx.Commit(ctx)
}

// AddImageIfMissingTx es AddImageIfMissing dentro de la transacción del ítem
// (TAL-191) y no hace commit: si la oferta falla, la imagen se revierte con el
// resto del ítem. La fila del producto se bloquea igual que en la versión
// independiente.
func (r *Repository) AddImageIfMissingTx(ctx context.Context, tx pgx.Tx, productID int32, imageURL string) (bool, error) {
	return addImageIfMissing(ctx, tx, productID, imageURL)
}

// addImageIfMissing ejecuta la inserción condicional sobre q (pool o transacción).
func addImageIfMissing(ctx context.Context, q rowQuerier, productID int32, imageURL string) (bool, error) {
	var name string
	err := q.QueryRow(ctx, `SELECT name FROM product WHERE id = $1 FOR NO KEY UPDATE`, productID).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	tag, err := q.Exec(ctx, `
		INSERT INTO product_image (product_id, image_url, alt_text, sort_order)
		SELECT $1, $2, $3, 0
		WHERE NOT EXISTS (SELECT 1 FROM product_image WHERE product_id = $1 AND active)
		  AND NOT EXISTS (SELECT 1 FROM product_image WHERE product_id = $1 AND image_url = $2)`,
		productID, imageURL, name)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// AddDescriptionIfMissingTx completa la descripción de un producto ya
// catalogado solo si está vacía o nula (issue #191). No reemplaza una
// descripción curada por un admin y no reactiva productos dados de baja.
// Recibe el id interno y el texto del ítem; devuelve true si lo guardó.
func (r *Repository) AddDescriptionIfMissingTx(ctx context.Context, tx pgx.Tx, productID int32, description string) (bool, error) {
	tag, err := tx.Exec(ctx, `
		UPDATE product
		SET description = $2, updated_at = NOW()
		WHERE id = $1
		  AND active
		  AND (description IS NULL OR btrim(description) = '')`, productID, description)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// EnsureBrand devuelve la marca con la misma clave normalizada o la crea.
func (r *Repository) EnsureBrand(ctx context.Context, name string) (int32, error) {
	return ensureBrand(ctx, r.db, name)
}

// EnsureBrandTx es EnsureBrand dentro de la transacción del ítem (TAL-191): si
// el ítem falla, una marca creada para él no queda en la base.
func (r *Repository) EnsureBrandTx(ctx context.Context, tx pgx.Tx, name string) (int32, error) {
	return ensureBrand(ctx, tx, name)
}

// ensureBrand busca o crea la marca sobre q (pool o transacción).
func ensureBrand(ctx context.Context, q rowQuerier, name string) (int32, error) {
	var id int32
	err := q.QueryRow(ctx, `
		SELECT id FROM brand
		WHERE lower(trim(regexp_replace(replace(name, '.', ''), '[[:space:]]+', ' ', 'g'))) =
		      lower(trim(regexp_replace(replace($1, '.', ''), '[[:space:]]+', ' ', 'g')))
		ORDER BY id LIMIT 1`, name).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = q.QueryRow(ctx, `
			INSERT INTO brand (name) VALUES ($1)
			ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
			RETURNING id`, name).Scan(&id)
	}
	return id, err
}

// ProductID devuelve el id interno de un producto por su UUID público.
func (r *Repository) ProductID(ctx context.Context, publicID pgtype.UUID) (int32, error) {
	return productID(ctx, r.db, publicID)
}

// ProductIDTx es ProductID dentro de la transacción del ítem (TAL-191), para
// leer el id recién insertado antes del commit.
func (r *Repository) ProductIDTx(ctx context.Context, tx pgx.Tx, publicID pgtype.UUID) (int32, error) {
	return productID(ctx, tx, publicID)
}

// productID resuelve el UUID público a id interno sobre q (pool o transacción).
func productID(ctx context.Context, q rowQuerier, publicID pgtype.UUID) (int32, error) {
	var id int32
	err := q.QueryRow(ctx, `SELECT id FROM product WHERE public_id = $1`, publicID).Scan(&id)
	return id, err
}

// ---------------------------------------------------------------------
// Servicios (issue #191)
// ---------------------------------------------------------------------

// Errores de categoría de servicio que el procesador traduce a 422.
var (
	ErrServiceCategoryNotFound = errors.New("service category not found")
)

// ServiceCategory es la categoría unificada de un servicio (tabla categories)
// con su equivalencia legada y sus definiciones de atributos.
type ServiceCategory struct {
	ID         int32                  // categories.id → service.category_ref
	Name       string                 // categories.name
	ItemType   string                 // categories.item_type ("service" esperado)
	LegacyID   *int32                 // service_category.id → service.category_id
	Attributes []categories.Attribute // definiciones para validar los valores
}

// ResolveServiceCategory busca por slug una categoría activa de tipo servicio y
// devuelve su fila unificada y sus definiciones de atributos ordenadas por
// display_order. Devuelve ErrServiceCategoryNotFound si el slug no existe.
// No verifica el item_type: eso lo decide el procesador para responder con un
// mensaje que distinga "no existe" de "no está habilitada para servicios".
func (r *Repository) ResolveServiceCategory(ctx context.Context, slug string) (ServiceCategory, error) {
	var category ServiceCategory
	err := r.db.QueryRow(ctx, `
		SELECT id, name, item_type, legacy_service_category_id
		FROM categories
		WHERE slug = $1 AND active`, slug).Scan(&category.ID, &category.Name, &category.ItemType, &category.LegacyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ServiceCategory{}, ErrServiceCategoryNotFound
	}
	if err != nil {
		return ServiceCategory{}, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, key, label, data_type, unit, required, comparable, filterable,
		       min_value::float8, max_value::float8, max_length, allowed_values, display_order
		FROM category_attributes
		WHERE category_id = $1
		ORDER BY display_order, id`, category.ID)
	if err != nil {
		return ServiceCategory{}, err
	}
	defer rows.Close()

	category.Attributes = make([]categories.Attribute, 0)
	for rows.Next() {
		var attribute categories.Attribute
		if err := rows.Scan(&attribute.ID, &attribute.Key, &attribute.Label, &attribute.DataType,
			&attribute.Unit, &attribute.Required, &attribute.Comparable, &attribute.Filterable,
			&attribute.Min, &attribute.Max, &attribute.MaxLength, &attribute.AllowedValues,
			&attribute.DisplayOrder); err != nil {
			return ServiceCategory{}, err
		}
		category.Attributes = append(category.Attributes, attribute)
	}
	return category, rows.Err()
}

// ServiceWrite son los datos validados de un servicio nuevo. CategoryID es la
// fila legada de service_category (NOT NULL en el esquema) y CategoryRef la
// categoría unificada que usan los atributos.
type ServiceWrite struct {
	CategoryID  int32
	CategoryRef int32
	Name        string
	Description string
	ImageURL    string
	Attributes  []categories.Value
}

// CreateServiceTx inserta un servicio y sus atributos dentro de la
// transacción del ítem. Devuelve el id interno y el public_id como texto.
func (r *Repository) CreateServiceTx(ctx context.Context, tx pgx.Tx, write ServiceWrite) (int32, string, error) {
	var (
		id       int32
		publicID string
	)
	err := tx.QueryRow(ctx, `
		INSERT INTO service (category_id, category_ref, name, description, image_url)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''))
		RETURNING id, public_id::text`,
		write.CategoryID, write.CategoryRef, write.Name, write.Description, write.ImageURL).
		Scan(&id, &publicID)
	if err != nil {
		return 0, "", err
	}
	if err := insertServiceAttributeValues(ctx, tx, id, write); err != nil {
		return 0, "", err
	}
	return id, publicID, nil
}

// insertServiceAttributeValues guarda los atributos validados del servicio en
// service_attribute_value, con value_numeric para los tipos numéricos. Es el
// espejo de products.insertAttributeValues.
func insertServiceAttributeValues(ctx context.Context, tx pgx.Tx, serviceID int32, write ServiceWrite) error {
	if len(write.Attributes) == 0 {
		return nil
	}
	attributeIDs := make([]int32, 0, len(write.Attributes))
	values := make([]string, 0, len(write.Attributes))
	numerics := make([]string, 0, len(write.Attributes))
	for _, value := range write.Attributes {
		attributeIDs = append(attributeIDs, value.AttributeID)
		values = append(values, value.Text)
		if value.DataType == "integer" || value.DataType == "decimal" {
			numerics = append(numerics, value.Text)
		} else {
			numerics = append(numerics, "")
		}
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO service_attribute_value (service_id, category_id, attribute_id, value, value_numeric)
		SELECT $1, $2, v.attribute_id, v.value, NULLIF(v.numeric_text, '')::numeric
		FROM unnest($3::int[], $4::text[], $5::text[]) AS v(attribute_id, value, numeric_text)`,
		serviceID, write.CategoryRef, attributeIDs, values, numerics)
	return err
}

// ServicePublicIDTx devuelve el public_id (UUID como texto) del servicio
// indicado, leído dentro de la transacción del ítem.
func (r *Repository) ServicePublicIDTx(ctx context.Context, tx pgx.Tx, id int32) (string, error) {
	var publicID string
	err := tx.QueryRow(ctx, `SELECT public_id::text FROM service WHERE id = $1`, id).Scan(&publicID)
	return publicID, err
}

// EnsureProviderTx devuelve el proveedor con ese slug o lo crea. El slug es la
// identidad; el nombre solo se usa al crearlo y no sobrescribe lo curado.
func (r *Repository) EnsureProviderTx(ctx context.Context, tx pgx.Tx, slug, name string) (int32, error) {
	var id int32
	err := tx.QueryRow(ctx, `
		INSERT INTO provider (slug, name) VALUES ($1, $2)
		ON CONFLICT (slug) DO UPDATE SET slug = EXCLUDED.slug
		RETURNING id`, slug, name).Scan(&id)
	return id, err
}

// FindServiceOfferTx busca un plan ya catalogado. Si el ítem trae
// “externalId“, esa es la identidad (proveedor + id del plan), porque hay
// proveedores que publican varios planes en una sola URL. Sin id, compara la
// URL de la oferta sin query string (los proveedores agregan parámetros de
// seguimiento). Devuelve el service_id y true si existe.
func (r *Repository) FindServiceOfferTx(ctx context.Context, tx pgx.Tx, providerID int32, externalID, serviceURL string) (int32, bool, error) {
	var serviceID int32
	var err error
	if externalID != "" {
		err = tx.QueryRow(ctx, `
			SELECT service_id
			FROM service_offer
			WHERE provider_id = $1 AND external_id = $2
			ORDER BY id LIMIT 1`, providerID, externalID).Scan(&serviceID)
	} else {
		err = tx.QueryRow(ctx, `
			SELECT service_id
			FROM service_offer
			WHERE provider_id = $1
			  AND service_url IS NOT NULL
			  AND split_part(btrim(service_url), '?', 1) = split_part(btrim($2), '?', 1)
			ORDER BY id LIMIT 1`, providerID, serviceURL).Scan(&serviceID)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return serviceID, true, nil
}

// ServiceOfferWrite son los datos validados de la oferta de un plan.
type ServiceOfferWrite struct {
	ServiceID        int32
	ProviderID       int32
	ExternalID       *string
	Price            string
	Currency         string
	BillingPeriod    *string
	InstallationCost *string
	ContractPeriod   *string
	Available        bool
	CoverageSummary  *string
	AdditionalCosts  *string
	ServiceURL       string
}

// UpsertServiceOfferTx registra o actualiza la oferta del plan en el proveedor
// y agrega una fila al historial solo cuando el precio cambió. Devuelve el id
// de la oferta.
func (r *Repository) UpsertServiceOfferTx(ctx context.Context, tx pgx.Tx, write ServiceOfferWrite) (int32, error) {
	var priceChanged bool
	err := tx.QueryRow(ctx, `
		SELECT NOT EXISTS (
			SELECT 1 FROM service_offer
			WHERE service_id = $1 AND provider_id = $2 AND price = $3::numeric)`,
		write.ServiceID, write.ProviderID, write.Price).Scan(&priceChanged)
	if err != nil {
		return 0, err
	}

	var offerID int32
	err = tx.QueryRow(ctx, `
		INSERT INTO service_offer (
			service_id, provider_id, external_id, price, currency, billing_period,
			installation_cost, contract_period, available, coverage_summary,
			additional_costs_summary, service_url, last_updated)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW())
		ON CONFLICT (service_id, provider_id) DO UPDATE SET
			external_id = EXCLUDED.external_id,
			price = EXCLUDED.price,
			currency = EXCLUDED.currency,
			billing_period = EXCLUDED.billing_period,
			installation_cost = EXCLUDED.installation_cost,
			contract_period = EXCLUDED.contract_period,
			available = EXCLUDED.available,
			coverage_summary = EXCLUDED.coverage_summary,
			additional_costs_summary = EXCLUDED.additional_costs_summary,
			service_url = EXCLUDED.service_url,
			last_updated = NOW()
		RETURNING id`,
		write.ServiceID, write.ProviderID, write.ExternalID, write.Price, write.Currency,
		write.BillingPeriod, write.InstallationCost, write.ContractPeriod, write.Available,
		write.CoverageSummary, write.AdditionalCosts, write.ServiceURL).Scan(&offerID)
	if err != nil {
		return 0, err
	}
	if !priceChanged {
		return offerID, nil
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO service_price_history (service_offer_id, price) VALUES ($1, $2)`,
		offerID, write.Price)
	return offerID, err
}
