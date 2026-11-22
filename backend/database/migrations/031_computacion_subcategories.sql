-- TAL-231 (ajuste): separa Computación en sus dos hijas del catálogo.
--
-- Deja la fila "Computación" (la vinculada por categories.slug='notebooks')
-- con las hijas Notebooks y Computadores de escritorio, y crea las filas de
-- categories correspondientes: computacion (raíz, con el legacy de
-- Computación), notebooks (ahora con el legacy de la hija Notebooks) y
-- computadores-de-escritorio (con el legacy de su hija).
--
-- Los productos guardados con category_ref=notebooks y category_id=Computación
-- pasan su category_ref a computacion. Si alguno tiene valores de atributos,
-- no se toca nada y la migración falla: esos valores cuelgan de la FK
-- compuesta (product_id, category_ref) y no se pueden repuntar a ciegas.
--
-- No hace backfill de productos por nombre: eso lo decide la ingestion.
-- Idempotente: una segunda aplicación no cambia datos ni falla.
-- BEGIN/COMMIT propio: el runner no envuelve cada archivo en una transacción.
BEGIN;

CREATE OR REPLACE FUNCTION pg_temp.tal_normalize_label(label_value TEXT)
RETURNS TEXT
LANGUAGE SQL
IMMUTABLE
AS $$
    SELECT translate(lower(btrim(label_value)), 'áéíóúüñ', 'aeiouun')
$$;

DO $$
DECLARE
    computacion_id INTEGER;
    parent_id INTEGER;
    notebooks_category_id INTEGER;
    notebooks_child_id INTEGER;
    escritorio_child_id INTEGER;
    computacion_category_id INTEGER;
    productos_con_atributos INTEGER;
BEGIN
    -- Locator: el product_category que apunta categories.notebooks.
    -- Primera aplicación: esa fila es Computación (raíz).
    -- Segunda aplicación: ya es la hija Notebooks y Computación es su padre.
    SELECT c.id, c.legacy_product_category_id
    INTO notebooks_category_id, computacion_id
    FROM categories c
    WHERE c.slug = 'notebooks'
      AND c.item_type = 'product'
      AND c.legacy_product_category_id IS NOT NULL
    ORDER BY c.id
    LIMIT 1;

    IF computacion_id IS NULL THEN
        RAISE EXCEPTION 'No existe el vínculo legacy de categories.slug=notebooks';
    END IF;

    -- Segunda aplicación: notebooks ya apunta a la hija Notebooks y Computación
    -- es su padre. Se sube un nivel solo si el nombre y el padre coinciden.
    SELECT pc.parent_category_id
    INTO parent_id
    FROM product_category pc
    JOIN product_category parent ON parent.id = pc.parent_category_id
    WHERE pc.id = computacion_id
      AND pg_temp.tal_normalize_label(pc.name) = 'notebooks'
      AND pg_temp.tal_normalize_label(parent.name) = 'computacion';
    IF parent_id IS NOT NULL THEN
        computacion_id := parent_id;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM product_category WHERE id = computacion_id) THEN
        RAISE EXCEPTION 'No se pudo localizar la categoría Computación';
    END IF;

    -- Hijas bajo Computación: se buscan por padre + nombre normalizado.
    SELECT id
    INTO notebooks_child_id
    FROM product_category
    WHERE parent_category_id = computacion_id
      AND pg_temp.tal_normalize_label(name) = 'notebooks'
    ORDER BY active DESC, id
    LIMIT 1;
    IF notebooks_child_id IS NULL THEN
        INSERT INTO product_category (parent_category_id, name)
        VALUES (computacion_id, 'Notebooks')
        RETURNING id INTO notebooks_child_id;
    END IF;

    SELECT id
    INTO escritorio_child_id
    FROM product_category
    WHERE parent_category_id = computacion_id
      AND pg_temp.tal_normalize_label(name) = 'computadores de escritorio'
    ORDER BY active DESC, id
    LIMIT 1;
    IF escritorio_child_id IS NULL THEN
        INSERT INTO product_category (parent_category_id, name)
        VALUES (computacion_id, 'Computadores de escritorio')
        RETURNING id INTO escritorio_child_id;
    END IF;

    -- (1) notebooks apunta a la hija Notebooks (libera el legacy de Computación).
    UPDATE categories
    SET legacy_product_category_id = notebooks_child_id,
        updated_at = NOW()
    WHERE id = notebooks_category_id
      AND legacy_product_category_id IS DISTINCT FROM notebooks_child_id;

    -- (2) computacion, raíz, con el legacy de Computación.
    INSERT INTO categories (slug, name, item_type, legacy_product_category_id)
    SELECT 'computacion', 'Computación', 'product', computacion_id
    WHERE NOT EXISTS (
        SELECT 1 FROM categories WHERE slug = 'computacion'
    ) AND NOT EXISTS (
        SELECT 1 FROM categories WHERE legacy_product_category_id = computacion_id
    );

    SELECT id
    INTO computacion_category_id
    FROM categories
    WHERE slug = 'computacion'
      AND item_type = 'product'
    ORDER BY id
    LIMIT 1;
    IF computacion_category_id IS NULL THEN
        RAISE EXCEPTION 'No se pudo crear categories.slug=computacion';
    END IF;

    -- (3) computadores-de-escritorio con el legacy de su hija.
    INSERT INTO categories (slug, name, item_type, legacy_product_category_id)
    SELECT 'computadores-de-escritorio', 'Computadores de escritorio', 'product', escritorio_child_id
    WHERE NOT EXISTS (
        SELECT 1 FROM categories WHERE slug = 'computadores-de-escritorio'
    ) AND NOT EXISTS (
        SELECT 1 FROM categories WHERE legacy_product_category_id = escritorio_child_id
    );

    -- Productos que quedaron con el category_ref de notebooks y el category_id
    -- de Computación: su category_ref pasa a computacion. Con valores de
    -- atributos no se puede mover (FK compuesta), así que se aborta entero.
    SELECT count(*)
    INTO productos_con_atributos
    FROM product p
    WHERE p.category_ref = notebooks_category_id
      AND p.category_id = computacion_id
      AND EXISTS (
          SELECT 1 FROM product_attribute_value a WHERE a.product_id = p.id
      );
    IF productos_con_atributos > 0 THEN
        RAISE EXCEPTION '031: % producto(s) de Computación tienen valores de atributos; no se cambia su category_ref',
            productos_con_atributos;
    END IF;

    UPDATE product
    SET category_ref = computacion_category_id,
        updated_at = NOW()
    WHERE category_ref = notebooks_category_id
      AND category_id = computacion_id;
END;
$$;

COMMIT;
