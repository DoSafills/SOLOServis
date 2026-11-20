-- Fusiona filas duplicadas de Refrigeradores/Refrigeracion en la categoría
-- canónica: la enlazada por categories.slug='refrigeradores' (bajo
-- Electrodomésticos después de 019/022). No importa el padre del duplicado.
--
-- Para cada fila activa con nombre normalizado equivalente se reasignan
-- productos, comparaciones, definiciones y valores de especificación, hijas y
-- vínculos de categories; recién entonces se renombra con prefijo
-- 'fusionada-<id>-' y se desactiva. Es idempotente: sin duplicados no hace nada.

BEGIN;

CREATE OR REPLACE FUNCTION pg_temp.tal025_normalize(label TEXT)
RETURNS TEXT
LANGUAGE SQL
IMMUTABLE
AS $$
    SELECT translate(lower(btrim(label)), 'áéíóúüñ', 'aeiouun')
$$;

CREATE OR REPLACE FUNCTION pg_temp.tal025_is_ancestor(ancestor_id INTEGER, node_id INTEGER)
RETURNS BOOLEAN
LANGUAGE SQL
STABLE
AS $$
    WITH RECURSIVE up AS (
        SELECT id, parent_category_id
        FROM product_category
        WHERE id = node_id
        UNION
        SELECT parent.id, parent.parent_category_id
        FROM product_category parent
        JOIN up ON parent.id = up.parent_category_id
    )
    SELECT EXISTS (SELECT 1 FROM up WHERE id = ancestor_id)
$$;

CREATE OR REPLACE FUNCTION pg_temp.tal025_merge_category(source_id INTEGER, target_id INTEGER)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    child RECORD;
    target_child_id INTEGER;
    source_spec RECORD;
    target_spec_id INTEGER;
BEGIN
    IF source_id = target_id
       OR NOT EXISTS (SELECT 1 FROM product_category WHERE id = source_id) THEN
        RETURN;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM product_category WHERE id = target_id) THEN
        RAISE EXCEPTION '025: no existe la categoría canónica %', target_id;
    END IF;
    IF pg_temp.tal025_is_ancestor(source_id, target_id) THEN
        RAISE EXCEPTION '025: la categoría canónica % está dentro de la duplicada %', target_id, source_id;
    END IF;

    FOR child IN
        SELECT id, name
        FROM product_category
        WHERE parent_category_id = source_id
        ORDER BY id
    LOOP
        SELECT id
        INTO target_child_id
        FROM product_category
        WHERE parent_category_id = target_id
          AND pg_temp.tal025_normalize(name) = pg_temp.tal025_normalize(child.name)
        ORDER BY active DESC, id
        LIMIT 1;

        IF target_child_id IS NULL THEN
            UPDATE product_category
            SET parent_category_id = target_id,
                updated_at = NOW()
            WHERE id = child.id;
        ELSE
            PERFORM pg_temp.tal025_merge_category(child.id, target_child_id);
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
          AND pg_temp.tal025_normalize(name) = pg_temp.tal025_normalize(source_spec.name)
        ORDER BY id
        LIMIT 1;

        IF target_spec_id IS NULL THEN
            UPDATE product_category_specification
            SET category_id = target_id
            WHERE id = source_spec.id;
        ELSE
            INSERT INTO product_specification_value (product_id, specification_id, value)
            SELECT value.product_id, target_spec_id, value.value
            FROM product_specification_value value
            WHERE value.specification_id = source_spec.id
              AND NOT EXISTS (
                  SELECT 1
                  FROM product_specification_value existing
                  WHERE existing.product_id = value.product_id
                    AND existing.specification_id = target_spec_id)
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

    UPDATE categories
    SET legacy_product_category_id = NULL,
        updated_at = NOW()
    WHERE legacy_product_category_id = source_id;

    IF EXISTS (SELECT 1 FROM product_category WHERE parent_category_id = source_id)
       OR EXISTS (SELECT 1 FROM product WHERE category_id = source_id)
       OR EXISTS (SELECT 1 FROM product_comparison WHERE category_id = source_id)
       OR EXISTS (SELECT 1 FROM product_category_specification WHERE category_id = source_id)
       OR EXISTS (SELECT 1 FROM categories WHERE legacy_product_category_id = source_id) THEN
        RAISE EXCEPTION '025: la categoría duplicada % aún tiene referencias', source_id;
    END IF;

    UPDATE product_category
    SET name = left('fusionada-' || id::TEXT || '-' || name, 150),
        active = FALSE,
        updated_at = NOW()
    WHERE id = source_id;
END;
$$;

DO $$
DECLARE
    canonical_id INTEGER;
    duplicate RECORD;
    merged INTEGER := 0;
BEGIN
    SELECT category.legacy_product_category_id
      INTO canonical_id
    FROM categories category
    WHERE category.slug = 'refrigeradores'
      AND category.item_type = 'product'
      AND category.active
      AND category.legacy_product_category_id IS NOT NULL
    ORDER BY category.id
    LIMIT 1;

    IF canonical_id IS NULL THEN
        RAISE NOTICE '025 no-op: categories.refrigeradores sin vínculo legacy';
        RETURN;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM product_category
        WHERE id = canonical_id AND active
    ) THEN
        RAISE EXCEPTION '025: la categoría canónica % no existe o está inactiva', canonical_id;
    END IF;

    FOR duplicate IN
        SELECT id
        FROM product_category
        WHERE active
          AND id <> canonical_id
          AND pg_temp.tal025_normalize(name) IN ('refrigeradores', 'refrigeracion')
        ORDER BY id
    LOOP
        PERFORM pg_temp.tal025_merge_category(duplicate.id, canonical_id);
        merged := merged + 1;
    END LOOP;

    RAISE NOTICE '025: % fila(s) duplicada(s) fusionadas en la categoría %', merged, canonical_id;
END;
$$;

COMMIT;
