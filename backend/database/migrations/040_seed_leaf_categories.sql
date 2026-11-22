-- =====================================================================
-- 040: toda hoja activa del árbol tiene su fila en categories y los
-- productos sin category_ref quedan enlazados a la fila de su hoja.
--
-- Por qué: las escrituras de producto (POST/PUT y la Ingestion API) resuelven
-- la categoría por categories.slug; una hoja sin fila no se puede usar aunque
-- exista en product_category. Las filas existentes no se tocan.
--
-- Reglas:
--   - solo se inserta si la hoja no tiene fila (legacy_product_category_id)
--     y su slug no está ocupado por otra fila de categories; si está ocupado
--     no se cambia nada y se avisa por NOTICE;
--   - el relleno de category_ref solo toca productos con NULL, jamás con un
--     valor puesto, y no puede chocar con atributos (un ref NULL no admite
--     valores por la FK compuesta).
--
-- Idempotente: la segunda pasada no inserta ni actualiza nada.
-- Reversa: backend/database/rollbacks/revert_040_seed_leaf_categories.sql
-- BEGIN/COMMIT propio: el runner no envuelve cada archivo en una transacción.
-- =====================================================================
BEGIN;

DO $$
DECLARE
    leaf          RECORD;
    creadas       INTEGER := 0;
    creadas_lista TEXT := '';
    ocupado       TEXT := '';
    rellenados    INTEGER;
    huerfanos     INTEGER;
    huerfanos_lista TEXT;
BEGIN
    FOR leaf IN
        SELECT pc.id, pc.slug, pc.name, pc.description
        FROM product_category pc
        WHERE pc.active
          AND pc.level = 3
        ORDER BY pc.id
    LOOP
        IF EXISTS (
            SELECT 1 FROM categories
            WHERE legacy_product_category_id = leaf.id
        ) THEN
            CONTINUE;
        END IF;

        IF EXISTS (
            SELECT 1 FROM categories WHERE slug = leaf.slug
        ) THEN
            ocupado := ocupado || format('%s (slug %s); ', leaf.name, leaf.slug);
            CONTINUE;
        END IF;

        INSERT INTO categories (slug, name, item_type, description, legacy_product_category_id)
        VALUES (leaf.slug, leaf.name, 'product', leaf.description, leaf.id);

        creadas := creadas + 1;
        creadas_lista := creadas_lista || format('%s (%s); ', leaf.name, leaf.slug);
    END LOOP;

    IF creadas > 0 THEN
        RAISE NOTICE '040: % fila(s) de categories creada(s): %', creadas, creadas_lista;
    ELSE
        RAISE NOTICE '040: todas las hojas activas ya tenían fila en categories';
    END IF;

    IF ocupado <> '' THEN
        RAISE NOTICE '040: hojas sin fila porque el slug está ocupado por otra categoría: %', ocupado;
    END IF;

    UPDATE product p
    SET category_ref = c.id,
        updated_at = NOW()
    FROM categories c
    WHERE c.item_type = 'product'
      AND c.legacy_product_category_id = p.category_id
      AND p.category_ref IS NULL;
    GET DIAGNOSTICS rellenados = ROW_COUNT;

    RAISE NOTICE '040: % producto(s) con category_ref NULL rellenado(s)', rellenados;

    SELECT count(*),
           string_agg(format('%s (%s)', p.id, pc.slug), '; ' ORDER BY p.id)
    INTO huerfanos, huerfanos_lista
    FROM product p
    JOIN product_category pc ON pc.id = p.category_id
    WHERE p.category_ref IS NULL
      AND NOT EXISTS (
          SELECT 1
          FROM categories c
          WHERE c.item_type = 'product'
            AND c.legacy_product_category_id = p.category_id
      );

    IF huerfanos > 0 THEN
        RAISE NOTICE '040: % producto(s) siguen con category_ref NULL porque su categoría no tiene fila: %',
            huerfanos, huerfanos_lista;
    END IF;
END;
$$;

COMMIT;
