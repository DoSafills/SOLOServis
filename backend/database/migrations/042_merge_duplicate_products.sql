-- 042: fusiona productos activos duplicados que comparten marca, categoría y
-- nombre normalizado (TAL-191). Es la limpieza de lo que ya existía antes del
-- control de duplicados: los huérfanos sin oferta (como #45) y las ofertas del
-- mismo producto repartidas en dos filas (Fensa #66/#75, Altus #51/#102).
--
-- Elige como superviviente al producto con más ofertas, luego más imágenes
-- activas y, en empate, el de menor id. Mueve las ofertas (y con ellas el
-- historial de precios) y copia atributos, especificaciones e imágenes activas
-- que el superviviente no tenga. No borra datos de usuario: si un duplicado
-- tiene interés, reseña, carrito o comparación, aborta para fusionarlo a mano.
-- Tampoco mueve una oferta que choque con el índice único de 009.
--
-- Es idempotente: si no quedan grupos duplicados no hace nada.

BEGIN;

CREATE OR REPLACE FUNCTION pg_temp.tal042_norm(label TEXT)
RETURNS TEXT
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT lower(btrim(regexp_replace(label, '[[:space:]]+', ' ', 'g')))
$$;

DO $$
DECLARE
    grupo RECORD;
    keeper_id INTEGER;
    dup_ids INTEGER[];
    dup_id INTEGER;
BEGIN
    FOR grupo IN
        SELECT p.brand_id, p.category_ref, pg_temp.tal042_norm(p.name) AS nombre
        FROM product p
        WHERE p.active
          AND p.brand_id IS NOT NULL
          AND p.category_ref IS NOT NULL
        GROUP BY p.brand_id, p.category_ref, pg_temp.tal042_norm(p.name)
        HAVING count(*) > 1
    LOOP
        SELECT p.id
        INTO keeper_id
        FROM product p
        WHERE p.active
          AND p.brand_id = grupo.brand_id
          AND p.category_ref = grupo.category_ref
          AND pg_temp.tal042_norm(p.name) = grupo.nombre
        ORDER BY (SELECT count(*) FROM product_offer o WHERE o.product_id = p.id) DESC,
                 (SELECT count(*) FROM product_image i WHERE i.product_id = p.id AND i.active) DESC,
                 p.id
        LIMIT 1;

        SELECT array_agg(p.id ORDER BY p.id)
        INTO dup_ids
        FROM product p
        WHERE p.active
          AND p.brand_id = grupo.brand_id
          AND p.category_ref = grupo.category_ref
          AND pg_temp.tal042_norm(p.name) = grupo.nombre
          AND p.id <> keeper_id;

        FOREACH dup_id IN ARRAY dup_ids
        LOOP
            IF EXISTS (SELECT 1 FROM product_user_interest WHERE product_id = dup_id)
               OR EXISTS (SELECT 1 FROM product_review WHERE product_id = dup_id)
               OR EXISTS (SELECT 1 FROM cart_item WHERE product_id = dup_id)
               OR EXISTS (SELECT 1 FROM product_comparison_item WHERE product_id = dup_id) THEN
                RAISE EXCEPTION '042: el producto % tiene datos de usuario; fusionar a mano', dup_id;
            END IF;

            IF EXISTS (
                SELECT 1
                FROM product_offer d
                JOIN product_offer k
                  ON k.product_id = keeper_id
                 AND k.store_id = d.store_id
                 AND COALESCE(normalize_product_offer_url(k.product_url), '') =
                     COALESCE(normalize_product_offer_url(d.product_url), '')
                WHERE d.product_id = dup_id
            ) THEN
                RAISE EXCEPTION '042: la oferta del producto % choca con una del %; fusionar a mano',
                    dup_id, keeper_id;
            END IF;

            INSERT INTO product_attribute_value (product_id, category_id, attribute_id, value, value_numeric)
            SELECT keeper_id, v.category_id, v.attribute_id, v.value, v.value_numeric
            FROM product_attribute_value v
            WHERE v.product_id = dup_id
            ON CONFLICT (product_id, attribute_id) DO NOTHING;

            INSERT INTO product_specification_value (product_id, specification_id, value)
            SELECT keeper_id, s.specification_id, s.value
            FROM product_specification_value s
            WHERE s.product_id = dup_id
            ON CONFLICT (product_id, specification_id) DO NOTHING;

            -- Las imágenes activas con URL distinta se conservan en el superviviente.
            UPDATE product_image i
            SET product_id = keeper_id
            WHERE i.product_id = dup_id
              AND i.active
              AND NOT EXISTS (
                  SELECT 1 FROM product_image k
                  WHERE k.product_id = keeper_id AND k.image_url = i.image_url);

            -- Las ofertas arrastran su historial de precios.
            UPDATE product_offer SET product_id = keeper_id WHERE product_id = dup_id;

            DELETE FROM product WHERE id = dup_id;

            RAISE NOTICE '042: producto % fusionado en %', dup_id, keeper_id;
        END LOOP;
    END LOOP;
END $$;

COMMIT;
