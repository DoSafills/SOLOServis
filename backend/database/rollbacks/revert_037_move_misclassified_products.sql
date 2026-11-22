-- =====================================================================
-- Reversa de 037: devuelve los productos demo a donde los dejó el seed 005.
--
-- Solo aplica a los SKU del catálogo demo; los productos reubicados que no
-- sean esos SKU no se pueden revertir con certeza (no se guarda su hoja
-- anterior) y se listan con NOTICE.
--
-- Orden: la reversa va en orden inverso al de aplicación: primero 040 y 039,
-- después 038 (que devuelve el slug muebles-y-decoracion) y recién entonces
-- este 037. Si se corre antes de 038, la hoja muebles-y-decoracion no existe
-- (038 la renombró a decoracion) y no se mueve ningún producto.
--
-- BEGIN/COMMIT propio: el runner no envuelve cada archivo en una transacción.
-- =====================================================================
BEGIN;

DO $$
DECLARE
    moved INTEGER;
    left_over TEXT;
BEGIN
    UPDATE product p
    SET category_id = target.id,
        updated_at = NOW()
    FROM product_category target
    WHERE target.slug = 'muebles-y-decoracion'
      AND target.level = 3
      AND target.active
      AND p.active
      AND p.sku IN ('DEMO-XIAOMI-ROBOT-VACUUM', 'DEMO-PHILIPS-AIRFRYER-62')
      AND p.category_id IS DISTINCT FROM target.id;
    GET DIAGNOSTICS moved = ROW_COUNT;
    RAISE NOTICE 'revert 037: % producto(s) demo devueltos a muebles-y-decoracion', moved;

    SELECT string_agg(format('%s (%s)', id, COALESCE(sku, name)), ', ' ORDER BY id)
    INTO left_over
    FROM product p
    JOIN product_category pc ON pc.id = p.category_id
    WHERE p.active
      AND pc.slug IN ('limpieza-del-hogar', 'cocina')
      AND p.sku IS DISTINCT FROM 'DEMO-XIAOMI-ROBOT-VACUUM'
      AND p.sku IS DISTINCT FROM 'DEMO-PHILIPS-AIRFRYER-62'
      AND (strpos(lower(p.name), 'aspirador') > 0
           OR strpos(lower(p.name), 'freidora') > 0);

    IF left_over IS NOT NULL THEN
        RAISE NOTICE 'revert 037: productos fuera del catálogo demo no se revierten: %', left_over;
    END IF;
END;
$$;

DO $$
BEGIN
    IF to_regclass('public.schema_migrations') IS NOT NULL THEN
        DELETE FROM schema_migrations
        WHERE version = '037_move_misclassified_products.sql';
    END IF;
END;
$$;

COMMIT;
