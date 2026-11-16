BEGIN;

DO $$
DECLARE
    appliance_id INTEGER;
    refrigerator_id INTEGER;
    refrigeration_id INTEGER;
    unified_id INTEGER;
BEGIN
    SELECT id INTO appliance_id
    FROM product_category
    WHERE name = 'Electrodomésticos'
      AND parent_category_id IS NULL
      AND active
    ORDER BY id
    LIMIT 1;

    SELECT id INTO refrigerator_id
    FROM product_category
    WHERE name = 'Refrigeradores'
      AND parent_category_id = appliance_id
      AND active
    ORDER BY id
    LIMIT 1;

    SELECT id INTO refrigeration_id
    FROM product_category
    WHERE name = 'Refrigeracion'
      AND parent_category_id IS NULL
    ORDER BY id
    LIMIT 1;

    SELECT id INTO unified_id
    FROM categories
    WHERE slug = 'refrigeradores'
      AND item_type = 'product'
      AND active;

    IF appliance_id IS NULL OR refrigerator_id IS NULL
       OR refrigeration_id IS NULL OR unified_id IS NULL THEN
        RAISE EXCEPTION 'No se encontró la jerarquía esperada de categorías de refrigeradores';
    END IF;

    IF EXISTS (
        SELECT 1 FROM product_category
        WHERE parent_category_id = refrigeration_id
    ) THEN
        RAISE EXCEPTION 'Refrigeracion tiene subcategorías; no se puede retirar automáticamente';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM categories
        WHERE id = unified_id
          AND legacy_product_category_id IN (refrigeration_id, refrigerator_id)
    ) THEN
        RAISE EXCEPTION 'La categoría unificada refrigeradores tiene un vínculo legado inesperado';
    END IF;

    IF EXISTS (
        SELECT 1 FROM categories
        WHERE legacy_product_category_id = refrigerator_id
          AND id <> unified_id
    ) THEN
        RAISE EXCEPTION 'Refrigeradores ya está vinculado a otra categoría unificada';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM product
        WHERE category_id IN (refrigeration_id, refrigerator_id)
          AND category_ref IS NOT NULL
          AND category_ref <> unified_id
    ) THEN
        RAISE EXCEPTION 'Hay productos de refrigeradores vinculados a otra categoría unificada';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM product_category_specification old_spec
        WHERE old_spec.category_id = refrigeration_id
          AND NOT EXISTS (
              SELECT 1
              FROM product_category_specification new_spec
              WHERE new_spec.category_id = refrigerator_id
                AND new_spec.data_type = old_spec.data_type
                AND translate(lower(btrim(new_spec.name)), 'áéíóú', 'aeiou') =
                    translate(lower(btrim(old_spec.name)), 'áéíóú', 'aeiou')
          )
    ) THEN
        RAISE EXCEPTION 'Hay especificaciones de Refrigeracion sin equivalente en Refrigeradores';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM product_specification_value old_value
        JOIN product_category_specification old_spec
          ON old_spec.id = old_value.specification_id
        JOIN product_category_specification new_spec
          ON new_spec.category_id = refrigerator_id
         AND new_spec.data_type = old_spec.data_type
         AND translate(lower(btrim(new_spec.name)), 'áéíóú', 'aeiou') =
             translate(lower(btrim(old_spec.name)), 'áéíóú', 'aeiou')
        JOIN product_specification_value new_value
          ON new_value.product_id = old_value.product_id
         AND new_value.specification_id = new_spec.id
        WHERE old_spec.category_id = refrigeration_id
          AND old_value.value <> new_value.value
    ) THEN
        RAISE EXCEPTION 'Hay valores de especificación distintos que colisionarían al consolidar';
    END IF;

    UPDATE categories
    SET legacy_product_category_id = refrigerator_id,
        updated_at = NOW()
    WHERE id = unified_id;

    INSERT INTO product_specification_value (product_id, specification_id, value)
    SELECT old_value.product_id, new_spec.id, old_value.value
    FROM product_specification_value old_value
    JOIN product_category_specification old_spec
      ON old_spec.id = old_value.specification_id
     AND old_spec.category_id = refrigeration_id
    JOIN product_category_specification new_spec
      ON new_spec.category_id = refrigerator_id
     AND new_spec.data_type = old_spec.data_type
     AND translate(lower(btrim(new_spec.name)), 'áéíóú', 'aeiou') =
         translate(lower(btrim(old_spec.name)), 'áéíóú', 'aeiou')
    ON CONFLICT (product_id, specification_id) DO NOTHING;

    DELETE FROM product_specification_value old_value
    USING product_category_specification old_spec
    WHERE old_value.specification_id = old_spec.id
      AND old_spec.category_id = refrigeration_id;

    UPDATE product_comparison
    SET category_id = refrigerator_id
    WHERE category_id = refrigeration_id;

    UPDATE product
    SET category_id = refrigerator_id,
        category_ref = unified_id,
        updated_at = NOW()
    WHERE category_id IN (refrigeration_id, refrigerator_id);

    UPDATE product_category
    SET active = FALSE,
        updated_at = NOW()
    WHERE id = refrigeration_id;
END;
$$;

COMMIT;
