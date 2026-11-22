-- =====================================================================
-- Reversa de 038: retira las hojas nuevas de Muebles y devuelve
-- Decoración a muebles-y-decoracion.
--
-- Una hoja solo se elimina si ningún producto la referencia; si tiene
-- productos se deja activa y se avisa por NOTICE (no se rompe la FK).
--
-- BEGIN/COMMIT propio: el runner no envuelve cada archivo en una transacción.
-- =====================================================================
BEGIN;

DO $$
DECLARE
    muebles_id INTEGER;
    leaf_slug  TEXT;
    removed    INTEGER;
BEGIN
    SELECT id
    INTO muebles_id
    FROM product_category
    WHERE slug = 'muebles' AND level = 2 AND active;

    IF muebles_id IS NULL THEN
        RAISE EXCEPTION 'revert 038: no existe la categoría muebles de nivel 2 activa';
    END IF;

    FOREACH leaf_slug IN ARRAY ARRAY[
        'sofas', 'mesas-de-comedor', 'mesitas-de-noche', 'camas', 'closets-y-armarios'
    ] LOOP
        DELETE FROM product_category pc
        WHERE pc.slug = leaf_slug
          AND pc.parent_category_id = muebles_id
          AND NOT EXISTS (SELECT 1 FROM product p WHERE p.category_id = pc.id);
        GET DIAGNOSTICS removed = ROW_COUNT;

        IF removed = 0 AND EXISTS (
            SELECT 1
            FROM product_category pc
            WHERE pc.slug = leaf_slug AND pc.parent_category_id = muebles_id
        ) THEN
            RAISE NOTICE 'revert 038: % conserva productos; la hoja queda activa', leaf_slug;
        END IF;
    END LOOP;

    IF EXISTS (SELECT 1 FROM product_category WHERE slug = 'decoracion')
       AND NOT EXISTS (SELECT 1 FROM product_category WHERE slug = 'muebles-y-decoracion') THEN
        UPDATE product_category
        SET slug = 'muebles-y-decoracion',
            name = 'Muebles y decoración',
            description = 'Muebles, decoración y productos para el hogar',
            updated_at = NOW()
        WHERE slug = 'decoracion'
          AND parent_category_id = muebles_id
          AND level = 3;
        RAISE NOTICE 'revert 038: decoracion devuelta a muebles-y-decoracion';
    END IF;
END;
$$;

DO $$
BEGIN
    IF to_regclass('public.schema_migrations') IS NOT NULL THEN
        DELETE FROM schema_migrations
        WHERE version = '038_muebles_subcategories.sql';
    END IF;
END;
$$;

COMMIT;
