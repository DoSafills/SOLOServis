-- =====================================================================
-- 038: hojas de nivel 3 bajo Muebles.
--
-- Por qué: Muebles (NV2) tenía una sola hoja, "Muebles y decoración", que
-- mezclaba muebles y decoración. Se reemplaza por hojas por tipo de mueble
-- y la fila existente pasa a ser "Decoración" (mismo id, nuevo slug).
--
-- Resultado bajo Muebles (nivel 2):
--   Sofás | Mesas de comedor | Mesitas de noche | Camas |
--   Closets y armarios | Decoración
--
-- Reglas:
--   - el padre se resuelve por slug (muebles, nivel 2 activo), nunca por id;
--   - el renombre conserva el id y solo ocurre si el slug nuevo está libre;
--   - las hojas se crean solo si no existe el slug ni otra fila con el
--     mismo nombre normalizado bajo el padre (índice de 030);
--   - no se tocan products ni category_ref: las hojas nacen sin productos.
--
-- Idempotente: renombre y altas comprueban el estado actual.
-- Reversa: backend/database/rollbacks/revert_038_muebles_subcategories.sql
-- BEGIN/COMMIT propio: el runner no envuelve cada archivo en una transacción.
-- =====================================================================
BEGIN;

CREATE OR REPLACE FUNCTION pg_temp.tal038_normalize_label(label_value TEXT)
RETURNS TEXT
LANGUAGE SQL
IMMUTABLE
AS $$
    SELECT translate(lower(btrim(label_value)),
                     U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                     'aeiouun')
$$;

DO $$
DECLARE
    muebles_id INTEGER;
    affected   INTEGER;
BEGIN
    SELECT id
    INTO muebles_id
    FROM product_category
    WHERE slug = 'muebles' AND level = 2 AND active;

    IF muebles_id IS NULL THEN
        RAISE EXCEPTION '038: no existe la categoría muebles de nivel 2 activa';
    END IF;

    -- La hoja única anterior pasa a Decoración conservando su id.
    IF NOT EXISTS (SELECT 1 FROM product_category WHERE slug = 'decoracion') THEN
        IF EXISTS (
            SELECT 1
            FROM product_category
            WHERE slug = 'muebles-y-decoracion'
              AND parent_category_id = muebles_id
              AND level = 3
        ) THEN
            UPDATE product_category
            SET slug = 'decoracion',
                name = 'Decoración',
                description = 'Adornos, alfombras y textiles para el hogar',
                updated_at = NOW()
            WHERE slug = 'muebles-y-decoracion'
              AND parent_category_id = muebles_id
              AND level = 3;
            GET DIAGNOSTICS affected = ROW_COUNT;
            RAISE NOTICE '038: muebles-y-decoracion renombrada a decoracion (% fila)', affected;
        ELSE
            INSERT INTO product_category (parent_category_id, name, description, active, slug, level)
            VALUES (muebles_id, 'Decoración', 'Adornos, alfombras y textiles para el hogar', TRUE, 'decoracion', 3);
            RAISE NOTICE '038: hoja decoracion creada';
        END IF;
    END IF;

    INSERT INTO product_category (parent_category_id, name, description, active, slug, level)
    SELECT muebles_id, seed.name, seed.description, TRUE, seed.slug, 3
    FROM (VALUES
        ('sofas', 'Sofás', 'Sofás, sillones y reposeras'),
        ('mesas-de-comedor', 'Mesas de comedor', 'Mesas y juegos de comedor'),
        ('mesitas-de-noche', 'Mesitas de noche', 'Veladores y mesitas de noche'),
        ('camas', 'Camas', 'Camas, catres y respaldos'),
        ('closets-y-armarios', 'Closets y armarios', 'Closets, armarios y cómodas')
    ) AS seed(slug, name, description)
    WHERE NOT EXISTS (
        SELECT 1 FROM product_category existing WHERE existing.slug = seed.slug
    )
      AND NOT EXISTS (
        SELECT 1
        FROM product_category existing
        WHERE existing.parent_category_id = muebles_id
          AND pg_temp.tal038_normalize_label(existing.name) =
              pg_temp.tal038_normalize_label(seed.name)
    );
    GET DIAGNOSTICS affected = ROW_COUNT;
    RAISE NOTICE '038: % hoja(s) de nivel 3 creadas bajo muebles', affected;
END;
$$;

COMMIT;
