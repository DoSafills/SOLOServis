-- TAL-140: habilita el slug canónico 'celulares' para la ingestion.
--
-- La categoría legada ya existe (022 la deja como raíz activa y 023 no la
-- toca); aquí solo se crea su equivalente en `categories`. Idempotente: si el
-- slug o el vínculo legado ya existen, no cambia nada. Un solo bloque DO para
-- que se aplique completo o no se aplique (el runner no envuelve el archivo).
DO $$
DECLARE
    legacy_id INTEGER;
    linked_slug TEXT;
BEGIN
    SELECT pc.id
    INTO legacy_id
    FROM product_category pc
    WHERE lower(btrim(pc.name)) = 'celulares'
      AND pc.active
    ORDER BY (pc.parent_category_id IS NULL) DESC, pc.id
    LIMIT 1;

    IF legacy_id IS NULL THEN
        RAISE EXCEPTION 'No existe una product_category Celulares activa';
    END IF;

    IF EXISTS (SELECT 1 FROM categories WHERE slug = 'celulares') THEN
        RETURN;
    END IF;

    SELECT c.slug
    INTO linked_slug
    FROM categories c
    WHERE c.legacy_product_category_id = legacy_id
    LIMIT 1;

    IF linked_slug IS NOT NULL THEN
        RAISE NOTICE '024: product_category % ya está vinculada por categories.%', legacy_id, linked_slug;
        RETURN;
    END IF;

    INSERT INTO categories (slug, name, item_type, description, legacy_product_category_id)
    VALUES ('celulares', 'Celulares', 'product', 'Teléfonos móviles y accesorios', legacy_id);
END;
$$;
