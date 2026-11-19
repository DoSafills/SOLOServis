-- TAL-231 (ajuste): deja una sola categoría raíz "Computación" en
-- product_category. La canónica es la fila vinculada por
-- categories.slug='notebooks' (computer_id); absorbe Computacion, Notebooks
-- y Computadores. No toca el resto del árbol.
--
-- Idempotente: si ya está consolidada no cambia datos ni falla.
-- BEGIN/COMMIT propio: el runner no envuelve cada archivo en una transacción.
BEGIN;

CREATE OR REPLACE FUNCTION pg_temp.tal232_normalize_label(label_value TEXT)
RETURNS TEXT
LANGUAGE SQL
IMMUTABLE
AS $$
    SELECT translate(lower(btrim(label_value)), 'áéíóúüñ', 'aeiouun')
$$;

CREATE OR REPLACE FUNCTION pg_temp.tal232_merge_category(source_id INTEGER, target_id INTEGER)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    child RECORD;
    target_child_id INTEGER;
    source_spec RECORD;
    target_spec_id INTEGER;
    category_slug TEXT;
    source_name TEXT;
    target_name TEXT;
BEGIN
    IF source_id = target_id THEN
        RETURN;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM product_category WHERE id = source_id) THEN
        RETURN;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM product_category WHERE id = target_id) THEN
        RAISE EXCEPTION 'No existe la categoría canónica %', target_id;
    END IF;

    SELECT name INTO source_name FROM product_category WHERE id = source_id;
    SELECT name INTO target_name FROM product_category WHERE id = target_id;

    UPDATE product_category
    SET active = TRUE,
        updated_at = NOW()
    WHERE id = target_id
      AND NOT active;

    FOR child IN
        SELECT id, name
        FROM product_category
        WHERE parent_category_id = source_id
        ORDER BY id
    LOOP
        -- Si el destino cuelga del origen, al retirar el origen el destino
        -- hereda su posición para no quedar huérfano ni apuntarse a sí mismo.
        IF child.id = target_id THEN
            UPDATE product_category
            SET parent_category_id = (
                    SELECT parent_category_id FROM product_category WHERE id = source_id),
                updated_at = NOW()
            WHERE id = target_id;
            CONTINUE;
        END IF;

        SELECT id
        INTO target_child_id
        FROM product_category
        WHERE parent_category_id = target_id
          AND pg_temp.tal232_normalize_label(name) =
              pg_temp.tal232_normalize_label(child.name)
        ORDER BY active DESC, id
        LIMIT 1;

        IF target_child_id IS NULL THEN
            UPDATE product_category
            SET parent_category_id = target_id,
                updated_at = NOW()
            WHERE id = child.id;
        ELSE
            PERFORM pg_temp.tal232_merge_category(child.id, target_child_id);
        END IF;
    END LOOP;

    FOR source_spec IN
        SELECT id, name
        FROM product_category_specification
        WHERE category_id = source_id
        ORDER BY id
    LOOP
        SELECT id
        INTO target_spec_id
        FROM product_category_specification
        WHERE category_id = target_id
          AND pg_temp.tal232_normalize_label(name) =
              pg_temp.tal232_normalize_label(source_spec.name)
        ORDER BY id
        LIMIT 1;

        IF target_spec_id IS NULL THEN
            UPDATE product_category_specification
            SET category_id = target_id
            WHERE id = source_spec.id;
        ELSE
            INSERT INTO product_specification_value (product_id, specification_id, value)
            SELECT source_value.product_id, target_spec_id, source_value.value
            FROM product_specification_value source_value
            WHERE source_value.specification_id = source_spec.id
              AND NOT EXISTS (
                  SELECT 1
                  FROM product_specification_value canonical_value
                  WHERE canonical_value.product_id = source_value.product_id
                    AND canonical_value.specification_id = target_spec_id
              )
            ON CONFLICT (product_id, specification_id) DO NOTHING;

            DELETE FROM product_specification_value
            WHERE specification_id = source_spec.id;
            DELETE FROM product_category_specification
            WHERE id = source_spec.id;
        END IF;
    END LOOP;

    UPDATE product
    SET category_id = target_id,
        updated_at = NOW()
    WHERE category_id = source_id;

    UPDATE product_comparison
    SET category_id = target_id,
        updated_at = NOW()
    WHERE category_id = source_id;

    UPDATE categories category
    SET legacy_product_category_id = target_id,
        updated_at = NOW()
    WHERE category.legacy_product_category_id = source_id
      AND NOT EXISTS (
          SELECT 1
          FROM categories existing
          WHERE existing.legacy_product_category_id = target_id
            AND existing.id <> category.id
      );

    FOR category_slug IN
        SELECT slug
        FROM categories
        WHERE legacy_product_category_id = source_id
        ORDER BY slug
    LOOP
        RAISE NOTICE '023: categories.% queda con legacy NULL: % ya está vinculada a otra categoría',
            category_slug, target_id;
    END LOOP;

    UPDATE categories
    SET legacy_product_category_id = NULL,
        updated_at = NOW()
    WHERE legacy_product_category_id = source_id;

    IF EXISTS (
        SELECT 1 FROM product_category
        WHERE parent_category_id = source_id
    ) OR EXISTS (
        SELECT 1 FROM product
        WHERE category_id = source_id
    ) OR EXISTS (
        SELECT 1 FROM product_comparison
        WHERE category_id = source_id
    ) OR EXISTS (
        SELECT 1 FROM product_category_specification
        WHERE category_id = source_id
    ) OR EXISTS (
        SELECT 1 FROM categories
        WHERE legacy_product_category_id = source_id
    ) THEN
        RAISE EXCEPTION 'La categoría fusionada % aún tiene referencias', source_id;
    END IF;

    UPDATE product_category
    SET active = FALSE,
        updated_at = NOW()
    WHERE id = source_id;

    IF pg_temp.tal232_normalize_label(source_name) =
       pg_temp.tal232_normalize_label(target_name) THEN
        UPDATE product_category
        SET name = left('fusionada-' || id::TEXT || '-' || name, 150),
            updated_at = NOW()
        WHERE id = source_id;
    END IF;
END;
$$;

DO $$
DECLARE
    target_id INTEGER;
    source_row RECORD;
BEGIN
    SELECT category.legacy_product_category_id
    INTO target_id
    FROM categories category
    WHERE category.slug = 'notebooks'
      AND category.item_type = 'product'
      AND category.legacy_product_category_id IS NOT NULL
    ORDER BY category.id
    LIMIT 1;

    IF target_id IS NULL THEN
        RAISE EXCEPTION 'No existe el vínculo legacy de categories.slug=notebooks';
    END IF;

    CREATE TEMP TABLE tal232_sources (
        id INTEGER PRIMARY KEY
    ) ON COMMIT DROP;

    INSERT INTO tal232_sources (id)
    SELECT id
    FROM product_category
    WHERE active
      AND id <> target_id
      AND pg_temp.tal232_normalize_label(name) IN ('computacion', 'notebooks', 'computadores')
    ORDER BY id;

    FOR source_row IN SELECT id FROM tal232_sources ORDER BY id LOOP
        PERFORM pg_temp.tal232_merge_category(source_row.id, target_id);
    END LOOP;

    -- Computación no tiene hijas: cualquier hija restante sube a raíz.
    UPDATE product_category
    SET parent_category_id = NULL,
        updated_at = NOW()
    WHERE parent_category_id = target_id;

    -- Archiva los nombres "Computación" restantes antes de renombrar la raíz:
    -- la unicidad (parent, lower(name)) de 017 también abarca filas inactivas.
    UPDATE product_category retired
    SET name = left('fusionada-' || retired.id::TEXT || '-' || retired.name, 150),
        updated_at = NOW()
    WHERE retired.id <> target_id
      AND retired.name NOT LIKE 'fusionada-%'
      AND pg_temp.tal232_normalize_label(retired.name) = 'computacion';

    UPDATE product_category
    SET name = 'Computación',
        parent_category_id = NULL,
        active = TRUE,
        updated_at = NOW()
    WHERE id = target_id
      AND (name IS DISTINCT FROM 'Computación'
           OR parent_category_id IS NOT NULL
           OR NOT active);
END;
$$;

COMMIT;
