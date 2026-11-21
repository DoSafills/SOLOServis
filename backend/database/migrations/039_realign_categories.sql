-- =====================================================================
-- 039: realinea la tabla de escritura categories con el árbol de 3 niveles
-- que dejaron 036/038.
--
-- Por qué: categories.slug='computacion' apuntaba a la hoja Accesorios de
-- computación (56), pero classifyComputacion ya convierte "computacion" en
-- una hoja concreta (notebooks, computadores-de-escritorio o accesorios)
-- antes de resolver contra el catálogo. La fila queda como alias sin hoja.
--
-- Resultado:
--   a) categories.computacion conserva la fila y queda con legacy NULL;
--   b) categories.accesorios-de-computacion (product) apunta a la hoja
--      Accesorios de computación;
--   c) los productos de la hoja Notebooks con category_ref NULL o que
--      apunta a otra hoja se repuntan a la fila de categories de su hoja.
--      Un producto con valores de atributo no se toca: el valor referencia
--      (product_id, category_ref) y moverlo rompería la FK. Se avisa por
--      NOTICE con id y claves.
--   d) cualquier otro producto con category_ref incoherente se informa por
--      NOTICE (no se cambia: no es de esta entrega).
--
-- Idempotente: los UPDATE solo mueven lo que difiere y la fila nueva se
-- inserta solo si falta.
-- Reversa: backend/database/rollbacks/revert_039_realign_categories.sql
-- BEGIN/COMMIT propio: el runner no envuelve cada archivo en una transacción.
-- =====================================================================
BEGIN;

DO $$
DECLARE
    accesorios_leaf  INTEGER;
    notebooks_leaf   INTEGER;
    accesorios_row   INTEGER;
    notebooks_row    INTEGER;
    computacion_row  INTEGER;
    skipped_total    INTEGER;
    skipped_list     TEXT;
    moved            INTEGER;
    otros_total      INTEGER;
    otros_list       TEXT;
BEGIN
    SELECT id
    INTO accesorios_leaf
    FROM product_category
    WHERE slug = 'accesorios-de-computacion' AND level = 3 AND active;

    IF accesorios_leaf IS NULL THEN
        RAISE EXCEPTION '039: no existe la hoja accesorios-de-computacion';
    END IF;

    -- a) La fila computacion no necesita hoja: classifyComputacion resuelve
    --    la hoja antes de tocar el catálogo. Se libera el enlace sin borrarla.
    UPDATE categories
    SET legacy_product_category_id = NULL,
        updated_at = NOW()
    WHERE slug = 'computacion'
      AND item_type = 'product'
      AND legacy_product_category_id IS NOT NULL;
    GET DIAGNOSTICS moved = ROW_COUNT;
    IF moved > 0 THEN
        RAISE NOTICE '039: computacion liberada de su hoja (% fila)', moved;
    END IF;

    SELECT id INTO computacion_row
    FROM categories
    WHERE slug = 'computacion' AND item_type = 'product';

    IF computacion_row IS NULL THEN
        RAISE NOTICE '039: no existe la fila categories computacion; nada que liberar';
    END IF;

    -- b) La fila de escritura de Accesorios de computación apunta a su hoja.
    SELECT id INTO accesorios_row
    FROM categories
    WHERE slug = 'accesorios-de-computacion';

    IF accesorios_row IS NOT NULL THEN
        IF EXISTS (
            SELECT 1 FROM categories
            WHERE id = accesorios_row AND item_type <> 'product'
        ) THEN
            RAISE NOTICE '039: accesorios-de-computacion existe como servicio; no se toca';
        ELSIF EXISTS (
            SELECT 1 FROM categories
            WHERE legacy_product_category_id = accesorios_leaf
              AND id <> accesorios_row
        ) THEN
            -- El enlace es UNIQUE: otra fila ya representa la hoja.
            RAISE NOTICE '039: la hoja accesorios-de-computacion ya tiene otra fila de categories';
        ELSE
            UPDATE categories
            SET legacy_product_category_id = accesorios_leaf,
                updated_at = NOW()
            WHERE id = accesorios_row
              AND legacy_product_category_id IS DISTINCT FROM accesorios_leaf;
            GET DIAGNOSTICS moved = ROW_COUNT;
            IF moved > 0 THEN
                RAISE NOTICE '039: accesorios-de-computacion enlazada a su hoja';
            END IF;
        END IF;
    ELSIF EXISTS (
        SELECT 1 FROM categories
        WHERE legacy_product_category_id = accesorios_leaf
    ) THEN
        RAISE NOTICE '039: la hoja accesorios-de-computacion ya tiene fila con otro slug';
    ELSE
        INSERT INTO categories (slug, name, item_type, description, legacy_product_category_id)
        SELECT 'accesorios-de-computacion',
               'Accesorios de computación',
               'product',
               pc.description,
               pc.id
        FROM product_category pc
        WHERE pc.id = accesorios_leaf;
        RAISE NOTICE '039: fila categories accesorios-de-computacion creada';
    END IF;

    -- c) Repunte de los productos de Notebooks con category_ref incoherente.
    SELECT id
    INTO notebooks_leaf
    FROM product_category
    WHERE slug = 'notebooks' AND level = 3 AND active;

    IF notebooks_leaf IS NULL THEN
        RAISE NOTICE '039: no existe la hoja notebooks; no se repunta category_ref';
        RETURN;
    END IF;

    SELECT id
    INTO notebooks_row
    FROM categories
    WHERE item_type = 'product'
      AND legacy_product_category_id = notebooks_leaf;

    IF notebooks_row IS NULL THEN
        RAISE NOTICE '039: la hoja notebooks no tiene fila de categories; no se repunta category_ref';
        RETURN;
    END IF;

    SELECT count(*),
           string_agg(format('%s (%s)', p.id, keys), '; ' ORDER BY p.id)
    INTO skipped_total, skipped_list
    FROM product p
    JOIN LATERAL (
        SELECT string_agg(ca.key, ', ' ORDER BY ca.key) AS keys
        FROM product_attribute_value pav
        JOIN category_attributes ca ON ca.id = pav.attribute_id
        WHERE pav.product_id = p.id
    ) attr ON attr.keys IS NOT NULL
    WHERE p.category_id = notebooks_leaf
      AND (
          p.category_ref IS NULL
          OR NOT EXISTS (
              SELECT 1
              FROM categories c
              WHERE c.id = p.category_ref
                AND c.item_type = 'product'
                AND c.legacy_product_category_id = notebooks_leaf
          )
      );

    IF skipped_total > 0 THEN
        RAISE NOTICE '039: % producto(s) con valores de atributo no se repuntan: %',
            skipped_total, skipped_list;
    END IF;

    UPDATE product p
    SET category_ref = notebooks_row,
        updated_at = NOW()
    WHERE p.category_id = notebooks_leaf
      AND p.category_ref IS DISTINCT FROM notebooks_row
      AND (
          p.category_ref IS NULL
          OR NOT EXISTS (
              SELECT 1
              FROM categories c
              WHERE c.id = p.category_ref
                AND c.item_type = 'product'
                AND c.legacy_product_category_id = notebooks_leaf
          )
      )
      AND NOT EXISTS (
          SELECT 1 FROM product_attribute_value pav WHERE pav.product_id = p.id
      );
    GET DIAGNOSTICS moved = ROW_COUNT;
    IF moved > 0 THEN
        RAISE NOTICE '039: % producto(s) de notebooks con category_ref realineado', moved;
    END IF;

    -- d) Fuera de Notebooks no se cambia nada; solo se informa.
    SELECT count(*),
           string_agg(format('%s (hoja %s)', p.id, pc.slug), '; ' ORDER BY p.id)
    INTO otros_total, otros_list
    FROM product p
    JOIN product_category pc ON pc.id = p.category_id
    WHERE p.category_ref IS NOT NULL
      AND p.category_id IS DISTINCT FROM notebooks_leaf
      AND NOT EXISTS (
          SELECT 1
          FROM categories c
          WHERE c.id = p.category_ref
            AND c.item_type = 'product'
            AND c.legacy_product_category_id = p.category_id
      );

    IF otros_total > 0 THEN
        RAISE NOTICE '039: % producto(s) fuera de notebooks con category_ref incoherente quedan sin cambiar: %',
            otros_total, otros_list;
    END IF;
END;
$$;

COMMIT;
