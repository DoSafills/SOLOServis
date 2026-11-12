BEGIN;

DO $$
DECLARE
    legacy_category_id INTEGER;
    existing_item_type VARCHAR(10);
    existing_legacy_category_id INTEGER;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM product_category
        WHERE parent_category_id IS NULL
          AND LOWER(BTRIM(name)) = LOWER(BTRIM('Sin clasificar'))
    ) THEN
        INSERT INTO product_category (parent_category_id, name)
        VALUES (NULL, 'Sin clasificar')
        ON CONFLICT ((COALESCE(parent_category_id, -1)), (LOWER(BTRIM(name))))
        DO NOTHING;
    END IF;

    SELECT id INTO legacy_category_id
    FROM product_category
    WHERE parent_category_id IS NULL
      AND LOWER(BTRIM(name)) = LOWER(BTRIM('Sin clasificar'));

    IF legacy_category_id IS NULL THEN
        RAISE EXCEPTION 'could not resolve root product_category Sin clasificar';
    END IF;

    SELECT item_type, legacy_product_category_id
    INTO existing_item_type, existing_legacy_category_id
    FROM categories
    WHERE slug = 'sin-clasificar';

    IF FOUND THEN
        IF existing_item_type IS DISTINCT FROM 'product'
           OR existing_legacy_category_id IS DISTINCT FROM legacy_category_id THEN
            RAISE EXCEPTION
                'categories.slug=sin-clasificar exists with conflicting item_type or legacy category';
        END IF;
        RETURN;
    END IF;

    IF EXISTS (
        SELECT 1 FROM categories
        WHERE legacy_product_category_id = legacy_category_id
          AND slug <> 'sin-clasificar'
    ) THEN
        RAISE EXCEPTION
            'legacy product category Sin clasificar is linked to another categories row';
    END IF;

    INSERT INTO categories
        (slug, name, item_type, legacy_product_category_id)
    VALUES
        ('sin-clasificar', 'Sin clasificar', 'product', legacy_category_id)
    ON CONFLICT (slug) DO NOTHING;

    SELECT item_type, legacy_product_category_id
    INTO existing_item_type, existing_legacy_category_id
    FROM categories
    WHERE slug = 'sin-clasificar';

    IF existing_item_type IS DISTINCT FROM 'product'
       OR existing_legacy_category_id IS DISTINCT FROM legacy_category_id THEN
        RAISE EXCEPTION
            'categories.slug=sin-clasificar exists with conflicting item_type or legacy category';
    END IF;
END;
$$;

COMMIT;

-- Rollback conservador (manual): el seed puede haber reutilizado filas
-- preexistentes y ambas categorías pueden adquirir referencias. No borrar
-- automáticamente. Solo retirar las filas si se confirma que fueron creadas
-- por esta migración y que ninguna FK o dato dependiente las referencia.
