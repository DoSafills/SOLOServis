-- =====================================================================
-- Reversa de 033 y 032: quita el backfill, la columna y el árbol NV.
--
-- Se ejecuta en orden inverso a las migraciones. Borra datos de las tres
-- tablas nuevas y la columna product.category_nv3_id; no toca
-- product_category, categories, atributos ni product.category_ref.
-- También elimina las versiones de schema_migrations para que el runner
-- pueda reaplicar 032 y 033 si se decide volver adelante.
--
-- Uso (con la base detenida para escrituras):
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f revert_032_033_category_tree_nv.sql
-- =====================================================================
BEGIN;

DROP FUNCTION IF EXISTS public.backfill_category_nv3();

DROP INDEX IF EXISTS idx_product_category_nv3_id;
ALTER TABLE product DROP CONSTRAINT IF EXISTS product_category_nv3_fk;
ALTER TABLE product DROP COLUMN IF EXISTS category_nv3_id;

DROP TABLE IF EXISTS categories_nv3;
DROP TABLE IF EXISTS categories_nv2;
DROP TABLE IF EXISTS categories_nv1;

DO $$
BEGIN
    IF to_regclass('public.schema_migrations') IS NOT NULL THEN
        DELETE FROM schema_migrations
        WHERE version IN ('032_category_tree_nv.sql',
                          '033_backfill_category_tree_nv.sql');
    END IF;
END;
$$;

COMMIT;
