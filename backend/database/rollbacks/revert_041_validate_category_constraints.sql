-- =====================================================================
-- Reversa de 041: devuelve las restricciones de product_category al estado
-- NOT VALID que dejaron 022/026.
--
-- PostgreSQL no permite "desvalidar" una restricción: se elimina y se vuelve
-- a crear con la misma expresión y NOT VALID. Las filas nuevas siguen
-- validándose; las históricas dejan de escanearse. No cambia datos.
--
-- Uso:
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f revert_041_validate_category_constraints.sql
-- BEGIN/COMMIT propio: el runner no envuelve cada archivo en una transacción.
-- =====================================================================
BEGIN;

ALTER TABLE product_category DROP CONSTRAINT IF EXISTS product_category_level_parent;
ALTER TABLE product_category DROP CONSTRAINT IF EXISTS product_category_level_range;
ALTER TABLE product_category DROP CONSTRAINT IF EXISTS product_category_parent_not_self;
ALTER TABLE product_category DROP CONSTRAINT IF EXISTS product_category_slug_format;

ALTER TABLE product_category
    ADD CONSTRAINT product_category_level_parent
    CHECK (
        level IS NULL
        OR (level = 1 AND parent_category_id IS NULL)
        OR (level > 1 AND parent_category_id IS NOT NULL)
    ) NOT VALID,
    ADD CONSTRAINT product_category_level_range
    CHECK (level IS NULL OR level BETWEEN 1 AND 3) NOT VALID,
    ADD CONSTRAINT product_category_parent_not_self
    CHECK (parent_category_id IS NULL OR parent_category_id <> id) NOT VALID,
    ADD CONSTRAINT product_category_slug_format
    CHECK (slug IS NULL OR slug ~ '^[a-z0-9-]{1,64}$') NOT VALID;

DO $$
BEGIN
    IF to_regclass('public.schema_migrations') IS NOT NULL THEN
        DELETE FROM schema_migrations
        WHERE version = '041_validate_category_constraints.sql';
    END IF;
END;
$$;

COMMIT;
