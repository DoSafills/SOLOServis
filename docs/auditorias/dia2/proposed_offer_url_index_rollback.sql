-- PROPUESTA - no aprobada - rollback del indice no unico opcional
BEGIN;

DROP INDEX IF EXISTS idx_product_offer_store_url_nonempty;

COMMIT;
