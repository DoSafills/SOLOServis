-- TAL-231: consolida categorías de producto y aplica el árbol canónico.
BEGIN;

CREATE TEMP TABLE tal231_retired_categories (
    id INTEGER PRIMARY KEY
) ON COMMIT DROP;

CREATE TEMP TABLE tal231_final_category_plan (
    id INTEGER PRIMARY KEY,
    name VARCHAR(150) NOT NULL,
    parent_category_id INTEGER
) ON COMMIT DROP;

CREATE OR REPLACE FUNCTION pg_temp.tal231_normalize_label(label_value TEXT)
RETURNS TEXT
LANGUAGE SQL
IMMUTABLE
AS $$
    SELECT translate(lower(btrim(label_value)), 'áéíóúüñ', 'aeiouun')
$$;

CREATE OR REPLACE FUNCTION pg_temp.tal231_merge_category(source_id INTEGER, target_id INTEGER)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    child RECORD;
    target_child_id INTEGER;
    source_spec RECORD;
    target_spec_id INTEGER;
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

    UPDATE product_category
    SET active = TRUE
    WHERE id = target_id;

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
          AND pg_temp.tal231_normalize_label(name) =
              pg_temp.tal231_normalize_label(child.name)
        ORDER BY active DESC, id
        LIMIT 1;

        IF target_child_id IS NULL THEN
            UPDATE product_category
            SET parent_category_id = target_id,
                updated_at = NOW()
            WHERE id = child.id;
        ELSE
            PERFORM pg_temp.tal231_merge_category(child.id, target_child_id);
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
          AND pg_temp.tal231_normalize_label(name) =
              pg_temp.tal231_normalize_label(source_spec.name)
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
        RAISE EXCEPTION 'La categoría duplicada % aún tiene referencias', source_id;
    END IF;

    UPDATE product_category
    SET active = FALSE,
        updated_at = NOW()
    WHERE id = source_id;

    INSERT INTO tal231_retired_categories (id)
    VALUES (source_id)
    ON CONFLICT (id) DO NOTHING;
END;
$$;

DO $$
DECLARE
    appliance_id INTEGER;
    technology_id INTEGER;
    computing_id INTEGER;
    cellular_id INTEGER;
    gaming_id INTEGER;
    home_id INTEGER;
    refrigerator_id INTEGER;
    washer_id INTEGER;
    television_id INTEGER;
    audio_id INTEGER;
    notebook_id INTEGER;
    computer_id INTEGER;
    merge_row RECORD;
BEGIN
    SELECT id INTO appliance_id
    FROM product_category
    WHERE pg_temp.tal231_normalize_label(name) = 'electrodomesticos'
    ORDER BY (parent_category_id IS NULL) DESC, active DESC, id
    LIMIT 1;
    IF appliance_id IS NULL THEN
        INSERT INTO product_category (parent_category_id, name, description)
        VALUES (NULL, 'Electrodomésticos', 'Categoría principal de electrodomésticos')
        RETURNING id INTO appliance_id;
    END IF;

    SELECT id INTO technology_id
    FROM product_category
    WHERE pg_temp.tal231_normalize_label(name) = 'tecnologia'
    ORDER BY (parent_category_id IS NULL) DESC, active DESC, id
    LIMIT 1;
    IF technology_id IS NULL THEN
        INSERT INTO product_category (parent_category_id, name, description)
        VALUES (NULL, 'Tecnología', 'Categoría principal de tecnología')
        RETURNING id INTO technology_id;
    END IF;

    SELECT id INTO computing_id
    FROM product_category
    WHERE pg_temp.tal231_normalize_label(name) = 'computacion'
    ORDER BY (name = 'Computación') DESC, active DESC, (parent_category_id IS NULL) DESC, id
    LIMIT 1;
    IF computing_id IS NULL THEN
        INSERT INTO product_category (parent_category_id, name, description)
        VALUES (NULL, 'Computación', 'Categoría principal de computación')
        RETURNING id INTO computing_id;
    END IF;

    SELECT id INTO cellular_id
    FROM product_category
    WHERE pg_temp.tal231_normalize_label(name) = 'celulares'
    ORDER BY active DESC, id
    LIMIT 1;
    IF cellular_id IS NULL THEN
        INSERT INTO product_category (parent_category_id, name, description)
        VALUES (NULL, 'Celulares', 'Teléfonos móviles')
        RETURNING id INTO cellular_id;
    END IF;

    SELECT id INTO gaming_id
    FROM product_category
    WHERE pg_temp.tal231_normalize_label(name) = 'gaming'
    ORDER BY active DESC, id
    LIMIT 1;
    IF gaming_id IS NULL THEN
        INSERT INTO product_category (parent_category_id, name, description)
        VALUES (NULL, 'Gaming', 'Consolas y videojuegos')
        RETURNING id INTO gaming_id;
    END IF;

    SELECT id INTO home_id
    FROM product_category
    WHERE pg_temp.tal231_normalize_label(name) = 'hogar y mueble'
    ORDER BY active DESC, id
    LIMIT 1;
    IF home_id IS NULL THEN
        INSERT INTO product_category (parent_category_id, name, description)
        VALUES (NULL, 'Hogar y mueble', 'Productos para el hogar y muebles')
        RETURNING id INTO home_id;
    END IF;

    SELECT category.legacy_product_category_id INTO refrigerator_id
    FROM categories category
    WHERE category.slug = 'refrigeradores'
      AND category.item_type = 'product'
      AND category.legacy_product_category_id IS NOT NULL
    ORDER BY category.id
    LIMIT 1;
    IF refrigerator_id IS NULL THEN
        RAISE EXCEPTION 'No existe el vínculo legacy de categories.slug=refrigeradores';
    END IF;

    SELECT category.legacy_product_category_id INTO computer_id
    FROM categories category
    WHERE category.slug = 'notebooks'
      AND category.item_type = 'product'
      AND category.legacy_product_category_id IS NOT NULL
    ORDER BY category.id
    LIMIT 1;
    IF computer_id IS NULL THEN
        RAISE EXCEPTION 'No existe el vínculo legacy de categories.slug=notebooks';
    END IF;

    SELECT category.legacy_product_category_id INTO television_id
    FROM categories category
    WHERE category.slug = 'televisores'
      AND category.item_type = 'product'
      AND category.legacy_product_category_id IS NOT NULL
    ORDER BY category.id
    LIMIT 1;
    IF television_id IS NULL THEN
        RAISE EXCEPTION 'No existe el vínculo legacy de categories.slug=televisores';
    END IF;

    SELECT id INTO washer_id
    FROM product_category
    WHERE pg_temp.tal231_normalize_label(name) = 'lavadoras'
    ORDER BY (parent_category_id = appliance_id) DESC, active DESC, id
    LIMIT 1;
    IF washer_id IS NULL THEN
        INSERT INTO product_category (parent_category_id, name, description)
        VALUES (appliance_id, 'Lavadoras', 'Lavadoras y secadoras')
        RETURNING id INTO washer_id;
    END IF;

    SELECT id INTO audio_id
    FROM product_category
    WHERE pg_temp.tal231_normalize_label(name) = 'audio'
    ORDER BY (parent_category_id = technology_id) DESC, active DESC, id
    LIMIT 1;
    IF audio_id IS NULL THEN
        INSERT INTO product_category (parent_category_id, name, description)
        VALUES (technology_id, 'Audio', 'Audífonos y parlantes')
        RETURNING id INTO audio_id;
    END IF;

    SELECT id INTO notebook_id
    FROM product_category
    WHERE pg_temp.tal231_normalize_label(name) = 'notebooks'
    ORDER BY (parent_category_id = computing_id) DESC, active DESC, id
    LIMIT 1;
    IF notebook_id IS NULL THEN
        INSERT INTO product_category (parent_category_id, name, description)
        VALUES (computing_id, 'Notebooks', 'Computadores portátiles')
        RETURNING id INTO notebook_id;
    END IF;

    CREATE TEMP TABLE tal231_canonical_categories (
        name_key TEXT PRIMARY KEY,
        category_id INTEGER NOT NULL
    ) ON COMMIT DROP;

    INSERT INTO tal231_canonical_categories (name_key, category_id)
    VALUES
        ('electrodomesticos', appliance_id),
        ('refrigeradores', refrigerator_id),
        ('refrigeracion', refrigerator_id),
        ('lavadoras', washer_id),
        ('lavado', washer_id),
        ('tecnologia', technology_id),
        ('televisores', television_id),
        ('audio', audio_id),
        ('computacion', computing_id),
        ('computadores', computer_id),
        ('notebooks', notebook_id),
        ('celulares', cellular_id),
        ('telefonia', cellular_id),
        ('gaming', gaming_id),
        ('videojuegos', gaming_id),
        ('hogar y mueble', home_id),
        ('hogar', home_id);

    FOR merge_row IN
        SELECT category.id AS source_id, canonical.category_id AS target_id
        FROM product_category category
        JOIN tal231_canonical_categories canonical
          ON pg_temp.tal231_normalize_label(category.name) = canonical.name_key
        WHERE category.id <> canonical.category_id
        ORDER BY category.id
    LOOP
        PERFORM pg_temp.tal231_merge_category(merge_row.source_id, merge_row.target_id);
    END LOOP;

    IF notebook_id = computer_id THEN
        INSERT INTO tal231_final_category_plan (id, name, parent_category_id)
        VALUES
            (appliance_id, 'Electrodomésticos', NULL),
            (refrigerator_id, 'Refrigeradores', appliance_id),
            (washer_id, 'Lavadoras', appliance_id),
            (technology_id, 'Tecnología', NULL),
            (television_id, 'Televisores', technology_id),
            (audio_id, 'Audio', technology_id),
            (computing_id, 'Computación', NULL),
            (notebook_id, 'Notebooks', computing_id),
            (cellular_id, 'Celulares', NULL),
            (gaming_id, 'Gaming', NULL),
            (home_id, 'Hogar y mueble', NULL);
    ELSE
        INSERT INTO tal231_final_category_plan (id, name, parent_category_id)
        VALUES
            (appliance_id, 'Electrodomésticos', NULL),
            (refrigerator_id, 'Refrigeradores', appliance_id),
            (washer_id, 'Lavadoras', appliance_id),
            (technology_id, 'Tecnología', NULL),
            (television_id, 'Televisores', technology_id),
            (audio_id, 'Audio', technology_id),
            (computing_id, 'Computación', NULL),
            (notebook_id, 'Notebooks', computing_id),
            (computer_id, 'Computadores', computing_id),
            (cellular_id, 'Celulares', NULL),
            (gaming_id, 'Gaming', NULL),
            (home_id, 'Hogar y mueble', NULL);
    END IF;

    UPDATE product_category retired
    SET name = left('fusionada-' || retired.id::TEXT || '-' || retired.name, 150)
    FROM tal231_retired_categories archived
    JOIN tal231_final_category_plan planned ON TRUE
    WHERE retired.id = archived.id
      AND retired.id <> planned.id
      AND retired.parent_category_id IS NOT DISTINCT FROM planned.parent_category_id
      AND pg_temp.tal231_normalize_label(retired.name) =
          pg_temp.tal231_normalize_label(planned.name);

    UPDATE product_category category
    SET name = planned.name,
        parent_category_id = planned.parent_category_id,
        active = TRUE,
        updated_at = NOW()
    FROM tal231_final_category_plan planned
    WHERE category.id = planned.id;
END;
$$;

COMMIT;
