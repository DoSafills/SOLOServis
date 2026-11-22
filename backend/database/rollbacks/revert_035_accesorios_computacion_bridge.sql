-- =====================================================================
-- Reversa de 035: suelta el puente de accesorios-de-computacion.
--
-- Solo limpia el puente si apunta a la fila legacy "Computación" (la que
-- tiene como hija a Notebooks), que es la que tomó 035. No toca ninguna
-- otra NV3, ni el árbol, ni product.category_nv3_id.
-- Elimina la fila de schema_migrations para que el runner pueda
-- reaplicar 035.
--
-- Uso (con la base detenida para escrituras):
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f revert_035_accesorios_computacion_bridge.sql
-- =====================================================================
BEGIN;

UPDATE categories_nv3 nv3
SET legacy_product_category_id = NULL, updated_at = NOW()
WHERE nv3.slug = 'accesorios-de-computacion'
  AND nv3.legacy_product_category_id IN (
      SELECT pc.id
      FROM product_category pc
      WHERE translate(lower(btrim(pc.name)),
                      U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                      'aeiouun') = 'computacion'
        AND EXISTS (
            SELECT 1
            FROM product_category child
            WHERE child.parent_category_id = pc.id
              AND translate(lower(btrim(child.name)),
                            U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                            'aeiouun') = 'notebooks'
        )
  );

DO $$
BEGIN
    IF to_regclass('public.schema_migrations') IS NOT NULL THEN
        DELETE FROM schema_migrations
        WHERE version = '035_accesorios_computacion_bridge.sql';
    END IF;
END;
$$;

COMMIT;
