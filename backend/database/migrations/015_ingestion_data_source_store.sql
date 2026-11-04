-- =====================================================================
-- TAL-77: Ingestion API registra las ejecuciones del scraper en las tablas
-- de 001_schema.sql (data_source, scraper_config, scrape_run, scraped_data),
-- que el scraper (PR #25) ya refleja. Una fuente de tipo tienda debe saber a
-- qué store pertenecen las ofertas que trae.
--
-- Sin BEGIN/COMMIT propio (migrate.sh usa --single-transaction).
-- =====================================================================

ALTER TABLE data_source
    ADD COLUMN store_id INTEGER REFERENCES store(id) ON DELETE SET NULL;

CREATE INDEX idx_data_source_store ON data_source(store_id);

-- Historial de ejecuciones por fuente, más reciente primero.
CREATE INDEX idx_scrape_run_config_started ON scrape_run(scraper_config_id, started_at DESC);
