-- Rollback manual de 047_source_change_event.sql (TAL-209).
--
-- El runner de migraciones no tiene "down": este script se ejecuta a mano
-- contra una base de desarrollo, nunca en producción ni desde
-- migration_order.txt. No vive en migrations/ porque el runner exige que todo
-- .sql de esa carpeta esté listado y lo aplicaría como una migración más.
--
-- Uso: psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f backend/database/rollback/047_source_change_event.down.sql

BEGIN;

DROP TABLE IF EXISTS source_change_event;
ALTER TABLE scrape_run DROP COLUMN IF EXISTS fingerprint;
ALTER TABLE scrape_run DROP COLUMN IF EXISTS spec_hash;

COMMIT;
