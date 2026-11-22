-- 045: limpieza manual de datos obsoletos (TAL-194).
--
-- Define cleanup_obsolete_data(meses, dry_run). Con dry_run=true (por
-- defecto) solo cuenta; con dry_run=false borra:
--   1. user_session con refresh_expires_at vencido;
--   2. filas repetidas de product/service_price_history (mismo precio y
--      promoción que la fila anterior de la misma oferta) con más de 24
--      meses de antigüedad (nunca menos, aunque "meses" sea menor);
--   3. cart_item de usuarios inactivos.
-- No toca productos, ofertas ni usuarios, y no programa ninguna ejecución.
-- Es idempotente: una segunda pasada devuelve 0 y no borra nada.

BEGIN;

CREATE OR REPLACE FUNCTION cleanup_obsolete_data(meses INTEGER DEFAULT 12, dry_run BOOLEAN DEFAULT TRUE)
RETURNS TABLE (tabla TEXT, filas BIGINT)
LANGUAGE plpgsql
AS $$
DECLARE
    corte TIMESTAMPTZ := NOW() - make_interval(months => GREATEST(meses, 24));
    n_sesiones BIGINT;
    n_precios_producto BIGINT;
    n_precios_servicio BIGINT;
    n_carritos BIGINT;
BEGIN
    SELECT count(*) INTO n_sesiones
    FROM user_session
    WHERE refresh_expires_at < NOW();

    SELECT count(*) INTO n_precios_producto
    FROM (
        SELECT price, is_promotional, recorded_at,
               lag(price) OVER w AS prev_price,
               lag(is_promotional) OVER w AS prev_promo
        FROM product_price_history
        WINDOW w AS (PARTITION BY product_offer_id ORDER BY recorded_at, id)
    ) h
    WHERE h.prev_price IS NOT NULL
      AND h.price = h.prev_price
      AND h.is_promotional = h.prev_promo
      AND h.recorded_at < corte;

    SELECT count(*) INTO n_precios_servicio
    FROM (
        SELECT price, is_promotional, recorded_at,
               lag(price) OVER w AS prev_price,
               lag(is_promotional) OVER w AS prev_promo
        FROM service_price_history
        WINDOW w AS (PARTITION BY service_offer_id ORDER BY recorded_at, id)
    ) h
    WHERE h.prev_price IS NOT NULL
      AND h.price = h.prev_price
      AND h.is_promotional = h.prev_promo
      AND h.recorded_at < corte;

    SELECT count(*) INTO n_carritos
    FROM cart_item c
    JOIN user_account u ON u.id = c.user_id
    WHERE NOT u.active;

    IF NOT dry_run THEN
        DELETE FROM user_session
        WHERE refresh_expires_at < NOW();

        DELETE FROM product_price_history p
        USING (
            SELECT id
            FROM (
                SELECT id, price, is_promotional, recorded_at,
                       lag(price) OVER w AS prev_price,
                       lag(is_promotional) OVER w AS prev_promo
                FROM product_price_history
                WINDOW w AS (PARTITION BY product_offer_id ORDER BY recorded_at, id)
            ) h
            WHERE h.prev_price IS NOT NULL
              AND h.price = h.prev_price
              AND h.is_promotional = h.prev_promo
              AND h.recorded_at < corte
        ) d
        WHERE p.id = d.id;

        DELETE FROM service_price_history p
        USING (
            SELECT id
            FROM (
                SELECT id, price, is_promotional, recorded_at,
                       lag(price) OVER w AS prev_price,
                       lag(is_promotional) OVER w AS prev_promo
                FROM service_price_history
                WINDOW w AS (PARTITION BY service_offer_id ORDER BY recorded_at, id)
            ) h
            WHERE h.prev_price IS NOT NULL
              AND h.price = h.prev_price
              AND h.is_promotional = h.prev_promo
              AND h.recorded_at < corte
        ) d
        WHERE p.id = d.id;

        DELETE FROM cart_item c
        USING user_account u
        WHERE c.user_id = u.id
          AND NOT u.active;

        RAISE NOTICE '045: limpieza aplicada (sesiones %, precios producto %, precios servicio %, carritos %)',
            n_sesiones, n_precios_producto, n_precios_servicio, n_carritos;
    END IF;

    RETURN QUERY
    SELECT 'user_session', n_sesiones
    UNION ALL SELECT 'product_price_history', n_precios_producto
    UNION ALL SELECT 'service_price_history', n_precios_servicio
    UNION ALL SELECT 'cart_item', n_carritos;
END;
$$;

COMMENT ON FUNCTION cleanup_obsolete_data(INTEGER, BOOLEAN) IS
    'TAL-194: con dry_run=true solo cuenta filas obsoletas; con dry_run=false las borra. No se ejecuta sola.';

REVOKE EXECUTE ON FUNCTION cleanup_obsolete_data(INTEGER, BOOLEAN) FROM PUBLIC;

COMMIT;
