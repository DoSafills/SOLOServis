-- Local operational schema. Transcribed from AGENTS.md and
-- backend/database/migrations/001_schema.sql: four tables, no additions.
-- The scraper stores one SCRAPED_DATA row per page (entity_type='webpage')
-- and one per extracted item (entity_type='product', raw_data = item JSON).

CREATE TABLE IF NOT EXISTS DATA_SOURCE (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    base_url TEXT NOT NULL,
    source_type TEXT NOT NULL,
    store_id INTEGER,
    active BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS SCRAPER_CONFIG (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    data_source_id INTEGER NOT NULL,
    scraper_type TEXT NOT NULL,
    target_type TEXT NOT NULL,
    schedule TEXT,
    rate_limit TEXT,
    parser_config TEXT,
    active BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (data_source_id) REFERENCES DATA_SOURCE(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS SCRAPE_RUN (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    scraper_config_id INTEGER NOT NULL,
    started_at DATETIME NOT NULL,
    finished_at DATETIME,
    records_found INTEGER DEFAULT 0,
    records_processed INTEGER DEFAULT 0,
    records_failed INTEGER DEFAULT 0,
    status TEXT NOT NULL
        CHECK (status IN ('pending', 'running', 'success', 'failed')),
    error_message TEXT,
    ingestion_run_id INTEGER,
    items_found INTEGER DEFAULT 0,
    items_sent INTEGER DEFAULT 0,
    items_failed INTEGER DEFAULT 0,
    items_rejected INTEGER DEFAULT 0,
    ingestion_error TEXT,
    FOREIGN KEY (scraper_config_id) REFERENCES SCRAPER_CONFIG(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS SCRAPED_DATA (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    scrape_run_id INTEGER NOT NULL,
    external_id TEXT,
    external_url TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    raw_data TEXT,
    extracted_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (scrape_run_id) REFERENCES SCRAPE_RUN(id) ON DELETE CASCADE
);

-- Indexes are implementation details, not new entities: they keep the
-- run-vs-run diff and the catalog export linear as history grows.
CREATE INDEX IF NOT EXISTS idx_scraped_run_entity
    ON SCRAPED_DATA (scrape_run_id, entity_type);
CREATE INDEX IF NOT EXISTS idx_scraped_external
    ON SCRAPED_DATA (external_id);
