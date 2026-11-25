-- Indices para las dos FK que 016 y 026 no cubrieron: la ejecucion de
-- scraping por configuracion (JOIN de ingestion/repository.go) y la tienda
-- de la fuente. Aditiva e idempotente.
CREATE INDEX IF NOT EXISTS idx_scrape_run_scraper_config
    ON scrape_run(scraper_config_id);

CREATE INDEX IF NOT EXISTS idx_data_source_store
    ON data_source(store_id);
