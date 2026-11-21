-- =====================================================================
-- Reversa de 040: retira el relleno de product.category_ref y las filas de
-- categories sembradas por hoja (slug = slug de la hoja y legacy = hoja).
--
-- La siembra no guarda qué filas creó, así que se reconocen por su forma:
-- fila de producto enlazada a una hoja activa con el mismo slug. Una fila
-- solo se elimina si ningún producto la referencia (antes se les devuelve
-- el ref a NULL, salvo que tengan valores de atributo) y no tiene
-- definiciones de atributo (se borrarían en cascada). Si sigue en uso se
-- deja y se avisa por NOTICE.
--
-- Uso:
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f revert_040_seed_leaf_categories.sql
-- BEGIN/COMMIT propio: el runner no envuelve cada archivo en una transacción.
-- =====================================================================
BEGIN;

DO $$
DECLARE
    vaciados  INTEGER;
    omitidos  INTEGER;
    borradas  INTEGER;
    restantes INTEGER;
BEGIN
    CREATE TEMP TABLE tal040r_filas ON COMMIT DROP AS
    SELECT c.id
    FROM categories c
    JOIN product_category pc ON pc.id = c.legacy_product_category_id
    WHERE c.item_type = 'product'
      AND pc.active
      AND pc.level = 3
      AND c.slug = pc.slug;

    SELECT count(*) INTO restantes FROM tal040r_filas;
    RAISE NOTICE 'revert 040: % fila(s) de hoja identificada(s)', restantes;

    IF restantes = 0 THEN
        RETURN;
    END IF;

    SELECT count(*) INTO omitidos
    FROM product p
    WHERE p.category_ref IN (SELECT id FROM tal040r_filas)
      AND EXISTS (
          SELECT 1 FROM product_attribute_value pav WHERE pav.product_id = p.id
      );

    IF omitidos > 0 THEN
        RAISE NOTICE 'revert 040: % producto(s) con atributos conservan su category_ref', omitidos;
    END IF;

    UPDATE product p
    SET category_ref = NULL,
        updated_at = NOW()
    WHERE p.category_ref IN (SELECT id FROM tal040r_filas)
      AND NOT EXISTS (
          SELECT 1 FROM product_attribute_value pav WHERE pav.product_id = p.id
      );
    GET DIAGNOSTICS vaciados = ROW_COUNT;
    RAISE NOTICE 'revert 040: % producto(s) vuelven a category_ref NULL', vaciados;

    DELETE FROM categories c
    WHERE c.id IN (SELECT id FROM tal040r_filas)
      AND NOT EXISTS (SELECT 1 FROM product p WHERE p.category_ref = c.id)
      AND NOT EXISTS (SELECT 1 FROM category_attributes ca WHERE ca.category_id = c.id);
    GET DIAGNOSTICS borradas = ROW_COUNT;

    SELECT count(*) INTO restantes
    FROM tal040r_filas f
    JOIN categories c ON c.id = f.id;

    RAISE NOTICE 'revert 040: % fila(s) eliminada(s); % siguen en uso y quedan activas',
        borradas, restantes;
END;
$$;

DO $$
BEGIN
    IF to_regclass('public.schema_migrations') IS NOT NULL THEN
        DELETE FROM schema_migrations
        WHERE version = '040_seed_leaf_categories.sql';
    END IF;
END;
$$;

COMMIT;
