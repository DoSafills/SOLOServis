package ingestion

import (
	"context"
	"encoding/json"
	"errors"

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
// ejecución en estado running. La fuente queda enlazada a storeID. El specHash
// y la versión del spec son opcionales: se guardan para poder correlacionar un
// cambio de la fuente con el spec que lo produjo (TAL-209).
func (r *Repository) OpenRun(ctx context.Context, name string, baseURL *string, storeID int32, scraperType, specHash, specVersion string) (int32, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var storeExists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM store WHERE id = $1 AND active)`, storeID).Scan(&storeExists); err != nil {
		return 0, err
	}
	if !storeExists {
		return 0, ErrStoreNotFound
	}

	var (
		sourceID      int32
		sourceStoreID *int32
		active        bool
	)
	if err := tx.QueryRow(ctx, `
		INSERT INTO data_source (name, base_url, source_type, store_id)
		VALUES ($1, $2, 'store', $3)
		ON CONFLICT (name) DO UPDATE
		    SET base_url = COALESCE(EXCLUDED.base_url, data_source.base_url), updated_at = NOW()
		RETURNING id, store_id, active`, name, baseURL, storeID).Scan(&sourceID, &sourceStoreID, &active); err != nil {
		return 0, err
	}
	switch {
	case !active:
		return 0, ErrSourceInactive
	case sourceStoreID == nil:
		// Fuente existente sin tienda (p. ej. creada antes de TAL-77): se enlaza.
		if _, err := tx.Exec(ctx, `UPDATE data_source SET store_id = $2 WHERE id = $1`, sourceID, storeID); err != nil {
			return 0, err
		}
	case *sourceStoreID != storeID:
		return 0, ErrSourceStoreMismatch
	}

	var configID int32
	err = tx.QueryRow(ctx, `
		SELECT id FROM scraper_config
		WHERE data_source_id = $1 AND scraper_type = $2 AND target_type = 'product' AND active
		ORDER BY id LIMIT 1`, sourceID, scraperType).Scan(&configID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `
			INSERT INTO scraper_config (data_source_id, scraper_type, target_type)
			VALUES ($1, $2, 'product') RETURNING id`, sourceID, scraperType).Scan(&configID)
	}
	if err != nil {
		return 0, err
	}

	if specHash != "" || specVersion != "" {
		// parser_config queda con la identidad del spec; no pisa otras claves.
		if _, err := tx.Exec(ctx, `
			UPDATE scraper_config
			SET parser_config = COALESCE(parser_config, '{}'::jsonb)
			    || jsonb_strip_nulls(jsonb_build_object('specHash', NULLIF($2, ''), 'specVersion', NULLIF($3, ''))),
			    updated_at = NOW()
			WHERE id = $1`, configID, specHash, specVersion); err != nil {
			return 0, err
		}
	}

	var runID int32
	if err := tx.QueryRow(ctx, `
		INSERT INTO scrape_run (scraper_config_id, status, spec_hash)
		VALUES ($1, 'running', NULLIF($2, '')) RETURNING id`, configID, specHash).Scan(&runID); err != nil {
		return 0, err
	}
	return runID, tx.Commit(ctx)
}

const runColumnsSQL = `
SELECT r.id, ds.name, ds.base_url, ds.store_id, sc.scraper_type, r.status, r.started_at, r.finished_at,
       COALESCE(r.records_found, 0), COALESCE(r.records_processed, 0), COALESCE(r.records_failed, 0),
       r.error_message
FROM scrape_run r
JOIN scraper_config sc ON sc.id = r.scraper_config_id
JOIN data_source ds ON ds.id = sc.data_source_id`

// scanRun convierte una fila de la consulta de ejecuciones en un Run.
func scanRun(row pgx.Row) (Run, error) {
	var run Run
	err := row.Scan(&run.ID, &run.Source, &run.SourceURL, &run.StoreID, &run.ScraperType, &run.Status,
		&run.StartedAt, &run.FinishedAt, &run.RecordsFound, &run.RecordsProcessed, &run.RecordsFailed,
		&run.ErrorMessage)
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

// SourceChangeEvent es un cambio detectado en una fuente (TAL-209). Se guarda
// en source_change_event para que la detección no viva solo en el log.
type SourceChangeEvent struct {
	StoreID             int32
	RunID               int32
	Tipo                string
	Detalle             string
	Fingerprint         json.RawMessage
	PreviousFingerprint json.RawMessage
	Verdict             string
	Reasons             []string
}

// SaveFingerprint guarda la huella estructural de la corrida. Recibe la
// ejecución y la huella; devuelve el error de la base, si lo hay.
func (r *Repository) SaveFingerprint(ctx context.Context, runID int32, fingerprint Fingerprint) error {
	raw, err := json.Marshal(fingerprint)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `
		UPDATE scrape_run SET fingerprint = $2::jsonb WHERE id = $1`, runID, string(raw))
	return err
}

// RecentFingerprints devuelve las huellas de las últimas corridas exitosas de
// una tienda, sin contar la actual y sin las corridas sin huella. Es la
// referencia contra la que se mide la corrida de hoy; una lista corta o vacía
// significa «sin historial suficiente» para el evaluador.
func (r *Repository) RecentFingerprints(ctx context.Context, storeID, excludeRunID int32, limit int) ([]Fingerprint, error) {
	rows, err := r.db.Query(ctx, `
		SELECT r.fingerprint
		FROM scrape_run r
		JOIN scraper_config sc ON sc.id = r.scraper_config_id
		JOIN data_source ds ON ds.id = sc.data_source_id
		WHERE ds.store_id = $1 AND r.id <> $2 AND r.status = 'success'
		  AND r.fingerprint IS NOT NULL
		ORDER BY r.started_at DESC, r.id DESC
		LIMIT $3`, storeID, excludeRunID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	fingerprints := make([]Fingerprint, 0, limit)
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var fingerprint Fingerprint
		if err := json.Unmarshal(raw, &fingerprint); err != nil {
			continue // una huella ilegible no invalida el historial
		}
		fingerprints = append(fingerprints, fingerprint)
	}
	return fingerprints, rows.Err()
}

// SaveSourceChangeEvent registra un cambio detectado. El UNIQUE (run_id, tipo)
// y el ON CONFLICT DO NOTHING hacen que un reintento de finish no duplique el
// evento; recibe el evento y devuelve el error de la base, si lo hay.
func (r *Repository) SaveSourceChangeEvent(ctx context.Context, event SourceChangeEvent) error {
	reasons, err := json.Marshal(event.Reasons)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO source_change_event
		    (store_id, run_id, tipo, detalle, fingerprint, previous_fingerprint, verdict, reasons)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7, $8::jsonb)
		ON CONFLICT (run_id, tipo) DO NOTHING`,
		event.StoreID, event.RunID, event.Tipo, event.Detalle,
		nullableJSON(event.Fingerprint), nullableJSON(event.PreviousFingerprint),
		event.Verdict, string(reasons))
	return err
}

// nullableJSON convierte una huella ausente en NULL, para que la columna no
// guarde un jsonb vacío cuando no hay corrida anterior que comparar.
func nullableJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return string(raw)
}

// ItemsTarget valida que la ejecución acepte ítems y devuelve la tienda de
// su fuente: ErrRunNotFound, ErrRunClosed o ErrRunWithoutStore.
func (r *Repository) ItemsTarget(ctx context.Context, id int32) (int32, error) {
	run, err := r.GetRun(ctx, id)
	if err != nil {
		return 0, err
	}
	if run.Status != StatusRunning && run.Status != "pending" {
		return 0, ErrRunClosed
	}
	if run.StoreID == nil {
		return 0, ErrRunWithoutStore
	}
	return *run.StoreID, nil
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
func (r *Repository) SaveScraped(ctx context.Context, runID int32, externalID, externalURL *string, raw json.RawMessage) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO scraped_data (scrape_run_id, external_id, external_url, entity_type, raw_data)
		VALUES ($1, $2, $3, 'product', $4)`, runID, externalID, externalURL, raw)
	return err
}

// ScrapedItem es una fila de scraped_data: el ítem recibido como JSON y sus
// identificadores en la tienda (nil si no vinieron).
type ScrapedItem struct {
	ExternalID  *string
	ExternalURL *string
	Raw         json.RawMessage
}

// SaveScrapedBatch guarda los ítems de un lote en scraped_data con un solo
// INSERT (TAL-205), en el orden recibido. Recibe la ejecución y las filas;
// devuelve el error de la base si el INSERT falla, y entonces no guarda
// ninguna. El JSON viaja como texto y se convierte a jsonb en la consulta.
func (r *Repository) SaveScrapedBatch(ctx context.Context, runID int32, items []ScrapedItem) error {
	externalIDs := make([]*string, len(items))
	externalURLs := make([]*string, len(items))
	raws := make([]string, len(items))
	for i, item := range items {
		externalIDs[i], externalURLs[i], raws[i] = item.ExternalID, item.ExternalURL, string(item.Raw)
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO scraped_data (scrape_run_id, external_id, external_url, entity_type, raw_data)
		SELECT $1, t.external_id, t.external_url, 'product', t.raw::jsonb
		FROM unnest($2::text[], $3::text[], $4::text[]) WITH ORDINALITY
		     AS t(external_id, external_url, raw, position)
		ORDER BY t.position`, runID, externalIDs, externalURLs, raws)
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
