-- =====================================================================
-- 037: reubica productos mal clasificados en hojas del árbol unificado.
--
-- Por qué: el catálogo demo (005/006) dejó una aspiradora robot y una
-- freidora de aire en "Muebles y decoración", que no corresponde a su
-- categoría. Con el árbol de 036 ya se pueden mover a su hoja natural:
--   - aspiradoras            -> limpieza-del-hogar (nivel 3)
--   - freidoras de aire      -> cocina              (nivel 3)
--
-- Reglas:
--   - las hojas se resuelven por slug desde product_category, nunca por id;
--   - solo se mueve lo activo y lo que aún no está en la hoja destino;
--   - si el producto tiene category_ref, solo se mueve cuando esa fila de
--     categories apunta a la misma hoja destino: mover category_id con
--     category_ref enlazado a otra hoja dejaría atributos incoherentes;
--   - category_ref no cambia; los productos del catálogo demo no tienen.
--
-- Idempotente: los UPDATE comparan category_id y no tocan filas ya movidas.
-- Reversa: backend/database/rollbacks/revert_037_move_misclassified_products.sql
-- BEGIN/COMMIT propio: el runner no envuelve cada archivo en una transacción.
-- =====================================================================
BEGIN;

CREATE OR REPLACE FUNCTION pg_temp.tal037_normalize_label(label_value TEXT)
RETURNS TEXT
LANGUAGE SQL
IMMUTABLE
AS $$
    SELECT translate(lower(btrim(label_value)),
                     U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                     'aeiouun')
$$;

DO $$
DECLARE
    moved INTEGER;
BEGIN
    UPDATE product p
    SET category_id = target.id,
        updated_at = NOW()
    FROM product_category target
    WHERE target.slug = 'limpieza-del-hogar'
      AND target.level = 3
      AND target.active
      AND p.active
      AND p.category_id IS DISTINCT FROM target.id
      AND (p.category_ref IS NULL OR EXISTS (
              SELECT 1 FROM categories c
              WHERE c.id = p.category_ref
                AND c.legacy_product_category_id = target.id))
      AND strpos(pg_temp.tal037_normalize_label(p.name), 'aspirador') > 0;
    GET DIAGNOSTICS moved = ROW_COUNT;
    RAISE NOTICE '037: % aspiradora(s) movidas a limpieza-del-hogar', moved;

    UPDATE product p
    SET category_id = target.id,
        updated_at = NOW()
    FROM product_category target
    WHERE target.slug = 'cocina'
      AND target.level = 3
      AND target.active
      AND p.active
      AND p.category_id IS DISTINCT FROM target.id
      AND (p.category_ref IS NULL OR EXISTS (
              SELECT 1 FROM categories c
              WHERE c.id = p.category_ref
                AND c.legacy_product_category_id = target.id))
      AND (
          strpos(pg_temp.tal037_normalize_label(p.name), 'freidora') > 0
          OR strpos(pg_temp.tal037_normalize_label(p.name), 'airfryer') > 0
          OR strpos(pg_temp.tal037_normalize_label(p.name), 'air fryer') > 0
          OR strpos(pg_temp.tal037_normalize_label(COALESCE(p.sku, '')), 'airfryer') > 0);
    GET DIAGNOSTICS moved = ROW_COUNT;
    RAISE NOTICE '037: % freidora(s) movidas a cocina', moved;
END;
$$;

COMMIT;
