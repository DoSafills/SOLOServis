-- PROPUESTA OPCIONAL - no aprobada - rollback del indice unico
BEGIN;

DROP INDEX IF EXISTS idx_product_offer_store_url_nonempty_unique;

COMMIT;
