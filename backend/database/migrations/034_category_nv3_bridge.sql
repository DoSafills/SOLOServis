-- =====================================================================
-- 034: puente entre cada NV3 y su fila legacy de product_category.
--
-- Por qué: product.category_id sigue siendo NOT NULL y apunta a
-- product_category, y el filtro por categoría legacy recorre ese árbol.
-- Para que una NV3 sin fila legacy propia pueda recibir productos y para
-- traducir un id legacy a NV3 hace falta este vínculo.
--
-- Columnas y restricciones:
--   - categories_nv3.legacy_product_category_id INTEGER, NULL;
--   - único (índice único): una fila legacy no puede apuntar a dos NV3;
--     el mismo índice cubre la FK;
--   - FK a product_category ON DELETE SET NULL: retirar una fila legacy
--     deja la NV3 sin puente, no bloquea la operación.
--
-- Backfill: tabla de pares explícita (slug NV3 -> nombre legacy + nombre
-- del padre legacy), resuelta por nombre normalizado y por padre; nunca
-- por coincidencia entre slug y nombre. Las NV3 que no resuelven quedan
-- NULL y se avisan por NOTICE con su slug. 'muebles-y-decoracion' se
-- enlaza a la raíz legacy 'Hogar y mueble' (mismo criterio que 033).
--
-- Al final se llama backfill_category_nv3() (033) para llenar
-- product.category_nv3_id de los productos que aún estén en NULL.
--
-- Idempotente: columna/índice/constraint con IF NOT EXISTS; el backfill
-- solo toca NV3 con el puente NULL y nunca pisa un valor existente.
-- BEGIN/COMMIT propio: el runner no envuelve cada archivo.
-- Reversa: backend/database/rollbacks/revert_034_category_nv3_bridge.sql
-- =====================================================================
BEGIN;

CREATE OR REPLACE FUNCTION pg_temp.tal034_normalize_label(label_value TEXT)
RETURNS TEXT
LANGUAGE SQL
IMMUTABLE
AS $$
    SELECT translate(lower(btrim(label_value)),
                     U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                     'aeiouun')
$$;

ALTER TABLE categories_nv3
    ADD COLUMN IF NOT EXISTS legacy_product_category_id INTEGER;

-- Unicidad del puente; el índice único también sirve de índice de la FK.
CREATE UNIQUE INDEX IF NOT EXISTS uq_categories_nv3_legacy_product_category_id
    ON categories_nv3 (legacy_product_category_id);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'categories_nv3_legacy_product_category_fk'
          AND conrelid = 'categories_nv3'::regclass
    ) THEN
        ALTER TABLE categories_nv3
            ADD CONSTRAINT categories_nv3_legacy_product_category_fk
            FOREIGN KEY (legacy_product_category_id)
            REFERENCES product_category(id) ON DELETE SET NULL;
    END IF;
END;
$$;

-- Puente explícito. El padre NULL exige una raíz legacy.
WITH pairs(nv3_slug, legacy_name, legacy_parent_name) AS (
    VALUES
        ('refrigeradores', 'refrigeradores', 'electrodomesticos'),
        ('lavadoras', 'lavadoras', 'electrodomesticos'),
        ('cocina', 'cocina', 'electrodomesticos'),
        ('climatizacion', 'climatizacion', 'electrodomesticos'),
        ('limpieza-del-hogar', 'limpieza del hogar', 'electrodomesticos'),
        ('cuidado-personal', 'cuidado personal', 'electrodomesticos'),
        ('muebles-y-decoracion', 'hogar y mueble', NULL),
        ('notebooks', 'notebooks', 'computacion'),
        ('computadores-de-escritorio', 'computadores de escritorio', 'computacion'),
        ('accesorios-de-computacion', 'accesorios de computacion', 'computacion'),
        ('televisores', 'televisores', 'tecnologia'),
        ('audio', 'audio', 'tecnologia'),
        ('celulares', 'celulares', 'tecnologia'),
        ('gaming', 'gaming', 'tecnologia')
)
UPDATE categories_nv3 nv3
SET legacy_product_category_id = legacy.id,
    updated_at = NOW()
FROM pairs
JOIN LATERAL (
    SELECT pc.id
    FROM product_category pc
    LEFT JOIN product_category parent ON parent.id = pc.parent_category_id
    WHERE pg_temp.tal034_normalize_label(pc.name) =
          pg_temp.tal034_normalize_label(pairs.legacy_name)
      AND (
          (pairs.legacy_parent_name IS NULL AND pc.parent_category_id IS NULL)
          OR pg_temp.tal034_normalize_label(parent.name) =
             pg_temp.tal034_normalize_label(pairs.legacy_parent_name)
      )
    ORDER BY pc.active DESC, pc.id
    LIMIT 1
) AS legacy ON TRUE
WHERE nv3.slug = pairs.nv3_slug
  AND nv3.legacy_product_category_id IS NULL
  AND NOT EXISTS (
      SELECT 1
      FROM categories_nv3 other
      WHERE other.legacy_product_category_id = legacy.id
  );

-- Aviso por cada NV3 que quedó sin puente.
DO $$
DECLARE
    pendiente RECORD;
    pendientes INTEGER := 0;
BEGIN
    FOR pendiente IN
        SELECT slug
        FROM categories_nv3
        WHERE legacy_product_category_id IS NULL
        ORDER BY slug
    LOOP
        pendientes := pendientes + 1;
        RAISE NOTICE '034: NV3 sin puente legacy: %', pendiente.slug;
    END LOOP;

    IF pendientes = 0 THEN
        RAISE NOTICE '034: todas las NV3 tienen puente legacy';
    ELSE
        RAISE NOTICE '034: % NV3 quedaron sin puente legacy', pendientes;
    END IF;
END;
$$;

-- Productos que sigan sin NV3 (altas posteriores a 033).
SELECT backfill_category_nv3();

COMMIT;
