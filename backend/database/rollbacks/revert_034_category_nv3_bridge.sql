-- =====================================================================
-- Reversa de 034: quita el puente NV3 -> product_category.
--
-- No toca el árbol NV, product_category, categories ni
-- product.category_nv3_id (eso es de la reversa de 032/033).
-- Elimina la fila de schema_migrations para que el runner pueda
-- reaplicar 034.
--
-- Uso (con la base detenida para escrituras):
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f revert_034_category_nv3_bridge.sql
-- =====================================================================
BEGIN;

DROP INDEX IF EXISTS uq_categories_nv3_legacy_product_category_id;
ALTER TABLE categories_nv3
    DROP CONSTRAINT IF EXISTS categories_nv3_legacy_product_category_fk;
ALTER TABLE categories_nv3
    DROP COLUMN IF EXISTS legacy_product_category_id;

DO $$
BEGIN
    IF to_regclass('public.schema_migrations') IS NOT NULL THEN
        DELETE FROM schema_migrations
        WHERE version = '034_category_nv3_bridge.sql';
    END IF;
END;
$$;

COMMIT;
