-- 047: huella estructural por corrida y eventos de cambio de fuente (TAL-209).
--
-- 1. scrape_run.spec_hash y scrape_run.fingerprint guardan con qué spec y con
--    qué estructura de página se extrajo cada corrida, para poder comparar una
--    corrida con la anterior de la misma fuente sin hashear el HTML completo.
-- 2. source_change_event registra cada cambio detectado (estructura, respuesta,
--    endpoint o caída de registros) para que la detección no viva solo en el log.
--    UNIQUE (run_id, tipo) permite como máximo un evento de cada tipo por
--    corrida: un reintento de finish no duplica la fila.
--
-- Aditiva e idempotente. El script inverso, para una base de desarrollo, está en
-- backend/database/rollback/047_source_change_event.down.sql.

BEGIN;

ALTER TABLE scrape_run ADD COLUMN IF NOT EXISTS spec_hash TEXT;
ALTER TABLE scrape_run ADD COLUMN IF NOT EXISTS fingerprint JSONB;

CREATE TABLE IF NOT EXISTS source_change_event (
    id                   SERIAL PRIMARY KEY,
    store_id             INTEGER NOT NULL REFERENCES store(id) ON DELETE CASCADE,
    run_id               INTEGER NOT NULL REFERENCES scrape_run(id) ON DELETE CASCADE,
    tipo                 VARCHAR(30) NOT NULL,
    detalle              TEXT,
    fingerprint          JSONB,
    previous_fingerprint JSONB,
    verdict              VARCHAR(20) NOT NULL DEFAULT 'degraded',
    reasons              JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at           TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (run_id, tipo)
);

CREATE INDEX IF NOT EXISTS idx_source_change_event_store
    ON source_change_event(store_id, created_at DESC);

COMMIT;
