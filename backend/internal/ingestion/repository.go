package ingestion

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository accede a las tablas de scraping y a lo que Ingestion necesita
// del catálogo (marcas, búsqueda de productos). Consultas parametrizadas.
type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// OpenRun crea (o reutiliza) la fuente y su configuración y abre una
// ejecución en estado running. La fuente queda enlazada a storeID.
func (r *Repository) OpenRun(ctx context.Context, name string, baseURL *string, storeID int32, scraperType string) (int32, error) {
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

	var runID int32
	if err := tx.QueryRow(ctx, `
		INSERT INTO scrape_run (scraper_config_id, status) VALUES ($1, 'running') RETURNING id`, configID).Scan(&runID); err != nil {
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

// SaveScraped guarda el ítem recibido (JSON normalizado) para auditoría.
func (r *Repository) SaveScraped(ctx context.Context, runID int32, externalID, externalURL *string, raw json.RawMessage) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO scraped_data (scrape_run_id, external_id, external_url, entity_type, raw_data)
		VALUES ($1, $2, $3, 'product', $4)`, runID, externalID, externalURL, raw)
	return err
}

// FindProduct busca un producto ya catalogado: primero por SKU y, si no hay
// SKU, por marca + modelo (sin distinguir mayúsculas). Prefiere los activos.
func (r *Repository) FindProduct(ctx context.Context, sku, brand, model string) (ExistingProduct, bool, error) {
	var (
		p   ExistingProduct
		err error
	)
	switch {
	case sku != "":
		err = r.db.QueryRow(ctx, `
			SELECT id, public_id::text, active FROM product WHERE sku = $1`, sku).Scan(&p.ID, &p.PublicID, &p.Active)
	case brand != "" && model != "":
		err = r.db.QueryRow(ctx, `
			SELECT p.id, p.public_id::text, p.active
			FROM product p JOIN brand b ON b.id = p.brand_id
			WHERE lower(b.name) = lower($1) AND lower(p.model) = lower($2)
			ORDER BY p.active DESC, p.id LIMIT 1`, brand, model).Scan(&p.ID, &p.PublicID, &p.Active)
	default:
		return ExistingProduct{}, false, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ExistingProduct{}, false, nil
	}
	return p, err == nil, err
}

// EnsureBrand devuelve la marca con ese nombre (sin distinguir mayúsculas)
// o la crea.
func (r *Repository) EnsureBrand(ctx context.Context, name string) (int32, error) {
	var id int32
	err := r.db.QueryRow(ctx, `SELECT id FROM brand WHERE lower(name) = lower($1) ORDER BY id LIMIT 1`, name).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = r.db.QueryRow(ctx, `
			INSERT INTO brand (name) VALUES ($1)
			ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
			RETURNING id`, name).Scan(&id)
	}
	return id, err
}

// ProductID devuelve el id interno de un producto por su UUID público.
func (r *Repository) ProductID(ctx context.Context, publicID pgtype.UUID) (int32, error) {
	var id int32
	err := r.db.QueryRow(ctx, `SELECT id FROM product WHERE public_id = $1`, publicID).Scan(&id)
	return id, err
}
