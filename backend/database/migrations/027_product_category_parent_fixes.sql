-- =====================================================================
-- TAL-190: corrige el árbol de product_category sobre datos con deriva.
--
-- Árbol canónico (solo filas active = TRUE):
--   Electrodomésticos, Tecnología, Computación, Celulares, Gaming,
--   Hogar y mueble (raíces).
--   Refrigeradores y Lavadoras bajo Electrodomésticos.
--   Televisores y Audio bajo Tecnología.
--
-- Reglas:
--   - nunca toca filas inactivas ni borra filas;
--   - una categoría activa con el padre equivocado o inactivo se reasigna;
--   - dos activas con el mismo nombre normalizado se fusionan: productos,
--     especificaciones, comparaciones y vínculos de categories pasan a la
--     conservada y la otra queda inactiva como 'fusionada-<id>-<nombre>';
--   - respeta la unicidad (padre, nombre normalizado) de 011 y la de
--     especificaciones de 017 (si el destino ya tiene el nombre, la
--     definición se queda con la categoría fusionada);
--   - idempotente: sin deriva, no cambia nada.
--
-- Sin BEGIN/COMMIT propio (migrate.sh usa --single-transaction).
-- =====================================================================

CREATE OR REPLACE FUNCTION pg_temp.tal027_normalize_label(label_value TEXT)
RETURNS TEXT
LANGUAGE SQL
IMMUTABLE
AS $$
    SELECT translate(lower(btrim(label_value)), 'áéíóúüñ', 'aeiouun')
$$;

DO $$
DECLARE
    canonical RECORD;
    duplicate RECORD;
    source_id INTEGER;
    target_id INTEGER;
    parent_id INTEGER;
    spec RECORD;
    merged INTEGER := 0;
BEGIN
    -- 1. Raíces canónicas: se crean solo si no existe ninguna activa con ese
    -- nombre y ninguna fila (activa o no) ocupa la clave de 011.
    FOR canonical IN
        SELECT * FROM (VALUES
            ('Electrodomésticos'),
            ('Tecnología'),
            ('Computación'),
            ('Celulares'),
            ('Gaming'),
            ('Hogar y mueble')
        ) AS roots(name)
    LOOP
        IF NOT EXISTS (
            SELECT 1 FROM product_category
            WHERE active
              AND pg_temp.tal027_normalize_label(name) = pg_temp.tal027_normalize_label(canonical.name)
        ) THEN
            INSERT INTO product_category (parent_category_id, name)
            SELECT NULL, canonical.name
            WHERE NOT EXISTS (
                SELECT 1 FROM product_category
                WHERE parent_category_id IS NULL
                  AND lower(btrim(name)) = lower(btrim(canonical.name))
            );
        END IF;
    END LOOP;

    -- 2. Hijas canónicas: bajo la raíz activa correspondiente.
    FOR canonical IN
        SELECT * FROM (VALUES
            ('Refrigeradores', 'Electrodomésticos'),
            ('Lavadoras', 'Electrodomésticos'),
            ('Televisores', 'Tecnología'),
            ('Audio', 'Tecnología')
        ) AS children(name, parent_name)
    LOOP
        SELECT id INTO parent_id
        FROM product_category
        WHERE active
          AND pg_temp.tal027_normalize_label(name) = pg_temp.tal027_normalize_label(canonical.parent_name)
        ORDER BY id
        LIMIT 1;

        IF parent_id IS NOT NULL AND NOT EXISTS (
            SELECT 1 FROM product_category
            WHERE active
              AND pg_temp.tal027_normalize_label(name) = pg_temp.tal027_normalize_label(canonical.name)
        ) THEN
            INSERT INTO product_category (parent_category_id, name)
            SELECT parent_id, canonical.name
            WHERE NOT EXISTS (
                SELECT 1 FROM product_category
                WHERE parent_category_id = parent_id
                  AND lower(btrim(name)) = lower(btrim(canonical.name))
            );
        END IF;
    END LOOP;

    -- 3. Fusiona duplicados activos por nombre normalizado. Se conserva la
    -- fila que ya está en una posición razonable (raíz o padre activo); si no,
    -- la más antigua.
    FOR duplicate IN
        SELECT pg_temp.tal027_normalize_label(name) AS normalized, array_agg(id ORDER BY
                   CASE WHEN parent_category_id IS NULL OR parent_category_id IN (
                       SELECT id FROM product_category WHERE active
                   ) THEN 0 ELSE 1 END,
                   id) AS ids
        FROM product_category
        WHERE active
        GROUP BY 1
        HAVING count(*) > 1
    LOOP
        target_id := duplicate.ids[1];
        FOR i IN 2 .. array_length(duplicate.ids, 1) LOOP
            source_id := duplicate.ids[i];
            IF source_id = target_id THEN
                CONTINUE;
            END IF;

            UPDATE product
            SET category_id = target_id, updated_at = NOW()
            WHERE category_id = source_id;

            UPDATE product_comparison
            SET category_id = target_id, updated_at = NOW()
            WHERE category_id = source_id;

            UPDATE product_category child
            SET parent_category_id = target_id, updated_at = NOW()
            WHERE child.parent_category_id = source_id
              AND NOT EXISTS (
                  SELECT 1 FROM product_category existing
                  WHERE existing.id <> child.id
                    AND existing.parent_category_id = target_id
                    AND pg_temp.tal027_normalize_label(existing.name) = pg_temp.tal027_normalize_label(child.name)
              );

            -- Las definiciones con nombre repetido en el destino se quedan en
            -- la categoría que se fusiona: 017 no permite duplicarlas.
            FOR spec IN
                SELECT id, name FROM product_category_specification WHERE category_id = source_id
            LOOP
                IF NOT EXISTS (
                    SELECT 1 FROM product_category_specification
                    WHERE category_id = target_id
                      AND pg_temp.tal027_normalize_label(name) = pg_temp.tal027_normalize_label(spec.name)
                ) THEN
                    UPDATE product_category_specification
                    SET category_id = target_id
                    WHERE id = spec.id;
                END IF;
            END LOOP;

            -- categories.legacy_product_category_id es UNIQUE: solo se mueve
            -- si el destino no está enlazado ya.
            UPDATE categories
            SET legacy_product_category_id = target_id, updated_at = NOW()
            WHERE legacy_product_category_id = source_id
              AND NOT EXISTS (
                  SELECT 1 FROM categories linked WHERE linked.legacy_product_category_id = target_id
              );

            UPDATE product_category
            SET active = FALSE,
                name = left('fusionada-' || source_id || '-' || name, 150),
                updated_at = NOW()
            WHERE id = source_id;

            merged := merged + 1;
            RAISE NOTICE 'tal027: categoría % fusionada en %', source_id, target_id;
        END LOOP;
    END LOOP;

    -- 4. Raíces: parent_category_id NULL.
    FOR canonical IN
        SELECT * FROM (VALUES
            ('Electrodomésticos'),
            ('Tecnología'),
            ('Computación'),
            ('Celulares'),
            ('Gaming'),
            ('Hogar y mueble')
        ) AS roots(name)
    LOOP
        UPDATE product_category root
        SET parent_category_id = NULL, updated_at = NOW()
        WHERE root.active
          AND root.parent_category_id IS NOT NULL
          AND pg_temp.tal027_normalize_label(root.name) = pg_temp.tal027_normalize_label(canonical.name)
          AND NOT EXISTS (
              SELECT 1 FROM product_category other
              WHERE other.id <> root.id
                AND other.parent_category_id IS NULL
                AND lower(btrim(other.name)) = lower(btrim(root.name))
          );
    END LOOP;

    -- 5. Hijas: bajo su raíz activa.
    FOR canonical IN
        SELECT * FROM (VALUES
            ('Refrigeradores', 'Electrodomésticos'),
            ('Lavadoras', 'Electrodomésticos'),
            ('Televisores', 'Tecnología'),
            ('Audio', 'Tecnología')
        ) AS children(name, parent_name)
    LOOP
        SELECT id INTO parent_id
        FROM product_category
        WHERE active
          AND pg_temp.tal027_normalize_label(name) = pg_temp.tal027_normalize_label(canonical.parent_name)
        ORDER BY id
        LIMIT 1;

        IF parent_id IS NOT NULL THEN
            UPDATE product_category child
            SET parent_category_id = parent_id, updated_at = NOW()
            WHERE child.active
              AND pg_temp.tal027_normalize_label(child.name) = pg_temp.tal027_normalize_label(canonical.name)
              AND child.parent_category_id IS DISTINCT FROM parent_id
              AND NOT EXISTS (
                  SELECT 1 FROM product_category other
                  WHERE other.id <> child.id
                    AND other.parent_category_id = parent_id
                    AND lower(btrim(other.name)) = lower(btrim(child.name))
              );
        END IF;
    END LOOP;

    RAISE NOTICE 'tal027: % categorías duplicadas fusionadas', merged;
END;
$$;
