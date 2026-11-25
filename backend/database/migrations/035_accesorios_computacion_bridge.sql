-- =====================================================================
-- 035: puente de accesorios-de-computacion y control de slugs repetidos.
--
-- Por qué: 034 no pudo enlazar accesorios-de-computacion porque no existe
-- una fila legacy "Accesorios de computación". La fila legacy "Computación"
-- (la que tiene como hija a Notebooks) queda libre: 031 enlazó las hijas
-- Notebooks y Computadores de escritorio, no el padre. Se usa esa fila para
-- que la NV3 pueda guardar productos (product.category_id es NOT NULL).
--
-- Regla de seguridad: solo se enlaza si la fila legacy "Computación" no
-- está ya enlazada a otra NV3. Si lo está, no se fuerza nada y se avisa
-- por NOTICE con el slug que la ocupa.
--
-- También se listan (sin modificarlos) los slugs repetidos entre
-- categories_nv1, categories_nv2 y categories_nv3: las tres tablas son
-- independientes y la unicidad de slug solo se exige dentro de NV3 (032).
--
-- Idempotente: el UPDATE solo toca filas con el puente NULL y comprueba
-- que la fila legacy siga libre; el reporte y el control de slugs solo
-- leen. Al final se llama backfill_category_nv3() (033).
-- BEGIN/COMMIT propio: el runner no envuelve cada archivo en una transacción.
-- Reversa: backend/database/rollbacks/revert_035_accesorios_computacion_bridge.sql
-- =====================================================================
BEGIN;

CREATE OR REPLACE FUNCTION pg_temp.tal035_normalize_label(label_value TEXT)
RETURNS TEXT
LANGUAGE SQL
IMMUTABLE
AS $$
    SELECT translate(lower(btrim(label_value)),
                     U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                     'aeiouun')
$$;

-- Enlaza accesorios-de-computacion con la fila legacy "Computación" (la que
-- tiene a Notebooks como hija) solo si esa fila no está ocupada por otra NV3.
WITH legacy AS (
    SELECT pc.id
    FROM product_category pc
    WHERE pg_temp.tal035_normalize_label(pc.name) = 'computacion'
      AND EXISTS (
          SELECT 1
          FROM product_category child
          WHERE child.parent_category_id = pc.id
            AND pg_temp.tal035_normalize_label(child.name) = 'notebooks'
      )
      AND NOT EXISTS (
          SELECT 1
          FROM categories_nv3 bound
          WHERE bound.legacy_product_category_id = pc.id
      )
    ORDER BY pc.active DESC, pc.id
    LIMIT 1
)
UPDATE categories_nv3 nv3
SET legacy_product_category_id = legacy.id, updated_at = NOW()
FROM legacy
WHERE nv3.slug = 'accesorios-de-computacion'
  AND nv3.legacy_product_category_id IS NULL
  AND NOT EXISTS (
      SELECT 1
      FROM categories_nv3 other
      WHERE other.legacy_product_category_id = legacy.id
  );

-- Reporte del puente: creado, ocupado por otra NV3 o sin fila legacy.
DO $$
DECLARE
    legacy_id INTEGER;
    legacy_linked_slug TEXT;
    accesorios_bridge INTEGER;
BEGIN
    SELECT pc.id
    INTO legacy_id
    FROM product_category pc
    WHERE pg_temp.tal035_normalize_label(pc.name) = 'computacion'
      AND EXISTS (
          SELECT 1
          FROM product_category child
          WHERE child.parent_category_id = pc.id
            AND pg_temp.tal035_normalize_label(child.name) = 'notebooks'
      )
    ORDER BY pc.active DESC, pc.id
    LIMIT 1;

    SELECT nv3.legacy_product_category_id
    INTO accesorios_bridge
    FROM categories_nv3 nv3
    WHERE nv3.slug = 'accesorios-de-computacion';

    IF accesorios_bridge IS NOT NULL THEN
        RAISE NOTICE '035: accesorios-de-computacion enlazada a la fila legacy %', accesorios_bridge;
        RETURN;
    END IF;

    IF legacy_id IS NULL THEN
        RAISE NOTICE '035: no existe la fila legacy Computación (con hija Notebooks); accesorios-de-computacion queda sin puente';
        RETURN;
    END IF;

    SELECT nv3.slug
    INTO legacy_linked_slug
    FROM categories_nv3 nv3
    WHERE nv3.legacy_product_category_id = legacy_id;

    IF legacy_linked_slug IS NOT NULL THEN
        RAISE NOTICE '035: la fila legacy Computación % ya está enlazada a %; accesorios-de-computacion queda sin puente',
            legacy_id, legacy_linked_slug;
    ELSE
        RAISE NOTICE '035: accesorios-de-computacion quedó sin puente pese a que la fila legacy Computación % está libre',
            legacy_id;
    END IF;
END;
$$;

-- Productos que aún estén sin category_nv3_id (backfill de 033, idempotente).
SELECT backfill_category_nv3();

-- Control de unicidad de slug entre niveles. No cambia nada: solo informa
-- las colisiones para que se resuelvan a mano (las tres tablas son
-- independientes y la unicidad global no está impuesta en la base).
DO $$
DECLARE
    collisions TEXT;
BEGIN
    WITH all_slugs AS (
        SELECT 'categories_nv1' AS table_name, slug FROM categories_nv1
        UNION ALL
        SELECT 'categories_nv2' AS table_name, slug FROM categories_nv2
        UNION ALL
        SELECT 'categories_nv3' AS table_name, slug FROM categories_nv3
    )
    SELECT string_agg(format('%s en %s', slug, tables), '; ' ORDER BY slug)
    INTO collisions
    FROM (
        SELECT slug,
               string_agg(table_name, '+') AS tables
        FROM all_slugs
        GROUP BY slug
        HAVING count(*) > 1
    ) duplicated;

    IF collisions IS NULL THEN
        RAISE NOTICE '035: ningún slug se repite entre categories_nv1, categories_nv2 y categories_nv3';
    ELSE
        RAISE NOTICE '035: slugs repetidos entre niveles (sin cambios): %', collisions;
    END IF;
END;
$$;

COMMIT;
