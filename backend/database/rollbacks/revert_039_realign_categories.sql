-- =====================================================================
-- Reversa de 039: devuelve la hoja Accesorios de computación a la fila
-- computacion y retira la fila accesorios-de-computacion si nadie la usa.
--
-- Lo que NO revierte, y por qué: el repunte de product.category_ref en la
-- hoja Notebooks no guarda el valor anterior; adivinarlo por nombre sería
-- peor. Se informa por NOTICE cuántos productos quedan apuntando a la fila
-- de Notebooks.
--
-- La fila accesorios-de-computacion solo se elimina si ningún producto la
-- referencia como category_ref y no tiene definiciones de atributo
-- (category_attributes se borraría en cascada). Si no, queda activa.
--
-- Uso:
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f revert_039_realign_categories.sql
-- BEGIN/COMMIT propio: el runner no envuelve cada archivo en una transacción.
-- =====================================================================
BEGIN;

DO $$
DECLARE
    accesorios_leaf INTEGER;
    notebooks_leaf  INTEGER;
    accesorios_row  INTEGER;
    notebooks_row   INTEGER;
    referenciada    INTEGER;
BEGIN
    SELECT id
    INTO accesorios_leaf
    FROM product_category
    WHERE slug = 'accesorios-de-computacion' AND level = 3;

    IF accesorios_leaf IS NULL THEN
        RAISE NOTICE 'revert 039: no existe la hoja accesorios-de-computacion';
        RETURN;
    END IF;

    SELECT id INTO accesorios_row
    FROM categories
    WHERE slug = 'accesorios-de-computacion' AND item_type = 'product';

    IF accesorios_row IS NOT NULL THEN
        SELECT count(*) INTO referenciada
        FROM product
        WHERE category_ref = accesorios_row;

        IF referenciada > 0
           OR EXISTS (SELECT 1 FROM category_attributes WHERE category_id = accesorios_row) THEN
            RAISE NOTICE 'revert 039: accesorios-de-computacion sigue en uso (% producto(s) o atributos); se deja como está',
                referenciada;
            RETURN;
        END IF;

        DELETE FROM categories
        WHERE id = accesorios_row
          AND legacy_product_category_id = accesorios_leaf;

        IF EXISTS (
            SELECT 1 FROM categories
            WHERE legacy_product_category_id = accesorios_leaf
        ) THEN
            RAISE NOTICE 'revert 039: otra fila de categories quedó enlazada a accesorios-de-computacion; no se toca computacion';
            RETURN;
        END IF;
    END IF;

    IF EXISTS (
        SELECT 1 FROM categories
        WHERE slug = 'computacion' AND item_type = 'product'
    ) THEN
        UPDATE categories
        SET legacy_product_category_id = accesorios_leaf,
            updated_at = NOW()
        WHERE slug = 'computacion'
          AND item_type = 'product'
          AND legacy_product_category_id IS DISTINCT FROM accesorios_leaf;
        RAISE NOTICE 'revert 039: computacion recupera la hoja accesorios-de-computacion';
    END IF;

    SELECT pc.id INTO notebooks_leaf
    FROM product_category pc
    WHERE pc.slug = 'notebooks' AND pc.level = 3;

    IF notebooks_leaf IS NOT NULL THEN
        SELECT id INTO notebooks_row
        FROM categories
        WHERE item_type = 'product'
          AND legacy_product_category_id = notebooks_leaf;

        IF notebooks_row IS NOT NULL THEN
            SELECT count(*) INTO referenciada
            FROM product
            WHERE category_ref = notebooks_row;

            IF referenciada > 0 THEN
                RAISE NOTICE 'revert 039: % producto(s) siguen con category_ref a la fila de notebooks (no se revierte el repunte)',
                    referenciada;
            END IF;
        END IF;
    END IF;
END;
$$;

DO $$
BEGIN
    IF to_regclass('public.schema_migrations') IS NOT NULL THEN
        DELETE FROM schema_migrations
        WHERE version = '039_realign_categories.sql';
    END IF;
END;
$$;

COMMIT;
