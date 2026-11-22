-- TAL-194: información antigua o innecesaria (solo lectura).
--
-- Cuenta filas candidatas a limpieza y mide el tamaño total de cada tabla
-- para priorizar. El umbral de antigüedad es el parámetro psql "meses"
-- (por defecto 12): cámbialo con "psql -v meses=6 -f ..." o edítalo aquí.
-- No modifica nada: solo SELECT.

\set meses 12

WITH checks("check", tabla, cantidad, tamano) AS (
    -- 1) Sesiones vencidas
    SELECT 'sesión con refresh vencido', 'user_session', count(*), NULL
    FROM user_session
    WHERE refresh_expires_at < NOW()

    -- 2) Historial de precios: antiguo o repetido
    UNION ALL SELECT 'precio de producto con más de N meses', 'product_price_history', count(*), NULL
    FROM product_price_history
    WHERE recorded_at < NOW() - make_interval(months => :meses)

    UNION ALL SELECT 'precio de producto repetido respecto a la fila anterior', 'product_price_history', count(*), NULL
    FROM (
        SELECT price, is_promotional,
               lag(price) OVER w AS prev_price,
               lag(is_promotional) OVER w AS prev_promo
        FROM product_price_history
        WINDOW w AS (PARTITION BY product_offer_id ORDER BY recorded_at, id)
    ) h
    WHERE h.prev_price IS NOT NULL
      AND h.price = h.prev_price
      AND h.is_promotional = h.prev_promo

    UNION ALL SELECT 'precio de servicio con más de N meses', 'service_price_history', count(*), NULL
    FROM service_price_history
    WHERE recorded_at < NOW() - make_interval(months => :meses)

    UNION ALL SELECT 'precio de servicio repetido respecto a la fila anterior', 'service_price_history', count(*), NULL
    FROM (
        SELECT price, is_promotional,
               lag(price) OVER w AS prev_price,
               lag(is_promotional) OVER w AS prev_promo
        FROM service_price_history
        WINDOW w AS (PARTITION BY service_offer_id ORDER BY recorded_at, id)
    ) h
    WHERE h.prev_price IS NOT NULL
      AND h.price = h.prev_price
      AND h.is_promotional = h.prev_promo

    -- 3) Ofertas inactivas o sin actualizar
    UNION ALL SELECT 'oferta de producto inactiva', 'product_offer', count(*), NULL
    FROM product_offer
    WHERE NOT available

    UNION ALL SELECT 'oferta de producto sin actualizar hace más de N meses', 'product_offer', count(*), NULL
    FROM product_offer
    WHERE last_updated < NOW() - make_interval(months => :meses)

    UNION ALL SELECT 'oferta de servicio inactiva', 'service_offer', count(*), NULL
    FROM service_offer
    WHERE NOT available

    UNION ALL SELECT 'oferta de servicio sin actualizar hace más de N meses', 'service_offer', count(*), NULL
    FROM service_offer
    WHERE last_updated < NOW() - make_interval(months => :meses)

    -- 4) Productos y servicios inactivos sin uso
    UNION ALL SELECT 'producto inactivo sin ofertas activas, carrito ni favoritos', 'product', count(*), NULL
    FROM product p
    WHERE NOT p.active
      AND NOT EXISTS (SELECT 1 FROM product_offer o WHERE o.product_id = p.id AND o.available)
      AND NOT EXISTS (SELECT 1 FROM cart_item c WHERE c.product_id = p.id)
      AND NOT EXISTS (SELECT 1 FROM product_user_interest f WHERE f.product_id = p.id AND f.is_favorite)

    UNION ALL SELECT 'servicio inactivo sin ofertas activas, carrito ni favoritos', 'service', count(*), NULL
    FROM service s
    WHERE NOT s.active
      AND NOT EXISTS (SELECT 1 FROM service_offer o WHERE o.service_id = s.id AND o.available)
      AND NOT EXISTS (
          SELECT 1 FROM cart_item c
          JOIN service_offer o ON o.id = c.service_offer_id
          WHERE o.service_id = s.id)
      AND NOT EXISTS (SELECT 1 FROM service_user_interest f WHERE f.service_id = s.id AND f.is_favorite)

    -- 5) Huérfanos y datos antiguos de usuario
    UNION ALL SELECT 'carrito de usuario inactivo', 'cart_item', count(*), NULL
    FROM cart_item c
    JOIN user_account u ON u.id = c.user_id
    WHERE NOT u.active

    UNION ALL SELECT 'favorito de producto de usuario inactivo', 'product_user_interest', count(*), NULL
    FROM product_user_interest f
    JOIN user_account u ON u.id = f.user_id
    WHERE f.is_favorite AND NOT u.active

    UNION ALL SELECT 'favorito de servicio de usuario inactivo', 'service_user_interest', count(*), NULL
    FROM service_user_interest f
    JOIN user_account u ON u.id = f.user_id
    WHERE f.is_favorite AND NOT u.active

    UNION ALL SELECT 'ítem de carrito vacío', 'cart_item', count(*), NULL
    FROM cart_item
    WHERE product_id IS NULL AND offer_id IS NULL AND service_offer_id IS NULL

    UNION ALL SELECT 'búsqueda guardada sin actualizar hace más de N meses', 'saved_search', count(*), NULL
    FROM saved_search
    WHERE updated_at < NOW() - make_interval(months => :meses)

    UNION ALL SELECT 'consulta de búsqueda con más de N meses', 'search_query', count(*), NULL
    FROM search_query
    WHERE created_at < NOW() - make_interval(months => :meses)

    UNION ALL SELECT 'consulta al asistente con más de N meses', 'ai_comparison_query', count(*), NULL
    FROM ai_comparison_query
    WHERE created_at < NOW() - make_interval(months => :meses)

    -- 6) Tamaño total por tabla, para priorizar la limpieza
    UNION ALL SELECT 'tamaño total de tabla', c.relname::text,
                     pg_total_relation_size(c.oid)::bigint,
                     pg_size_pretty(pg_total_relation_size(c.oid))
    FROM pg_class c
    JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE n.nspname = 'public'
      AND c.relkind IN ('r', 'p')
)
SELECT "check", tabla, cantidad, tamano
FROM checks
ORDER BY (tamano IS NOT NULL), cantidad DESC, tabla;
