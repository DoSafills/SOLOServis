-- PROPUESTA - no aprobada - no aplicar a bases del proyecto
-- Indice no unico para consultas por tienda y URL no vacia.
-- Columnas verificadas en backend/database/migrations/001_schema.sql:121,132.
-- En una tabla grande de produccion, evaluar CREATE INDEX CONCURRENTLY
-- (que debe ejecutarse fuera de una transaccion).
BEGIN;

CREATE INDEX IF NOT EXISTS idx_product_offer_store_url_nonempty
    ON product_offer (store_id, product_url)
    WHERE product_url IS NOT NULL
      AND BTRIM(product_url) <> '';

COMMIT;
