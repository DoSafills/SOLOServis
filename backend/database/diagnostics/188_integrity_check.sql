-- 188: diagnóstico de registros incompletos, inconsistentes y corruptos.
--
-- Solo lectura: no modifica datos ni esquema. Cada fila devuelve el check,
-- la tabla y la cantidad de filas afectadas; el resultado va ordenado de
-- mayor a menor cantidad. Los hallazgos de duplicados se cruzan con los datos
-- de usuario antes de fusionar productos o servicios.
--
-- Uso: psql "$DATABASE_URL" -f backend/database/diagnostics/188_integrity_check.sql
-- Un check con cantidad 0 no encontró nada; uno con cantidad > 0 es hallazgo.

WITH textos AS (
    -- Texto visible al usuario: se revisa por caracteres de reemplazo (U+FFFD)
    -- y por las secuencias típicas de mojibake UTF-8 leído como Latin-1.
    SELECT 'product'::text AS tabla, name AS valor FROM product
    UNION ALL SELECT 'product', model FROM product
    UNION ALL SELECT 'product', sku FROM product
    UNION ALL SELECT 'product', description FROM product
    UNION ALL SELECT 'product_image', image_url FROM product_image
    UNION ALL SELECT 'product_image', alt_text FROM product_image
    UNION ALL SELECT 'product_offer', product_url FROM product_offer
    UNION ALL SELECT 'product_specification_value', value FROM product_specification_value
    UNION ALL SELECT 'product_attribute_value', value FROM product_attribute_value
    UNION ALL SELECT 'product_category', name FROM product_category
    UNION ALL SELECT 'product_category', description FROM product_category
    UNION ALL SELECT 'product_review', title FROM product_review
    UNION ALL SELECT 'product_review', content FROM product_review
    UNION ALL SELECT 'brand', name FROM brand
    UNION ALL SELECT 'store', name FROM store
    UNION ALL SELECT 'store', shipping_information FROM store
    UNION ALL SELECT 'store', general_conditions FROM store
    UNION ALL SELECT 'store_review', title FROM store_review
    UNION ALL SELECT 'store_review', content FROM store_review
    UNION ALL SELECT 'service', name FROM service
    UNION ALL SELECT 'service', description FROM service
    UNION ALL SELECT 'service', image_url FROM service
    UNION ALL SELECT 'service_specification_value', value FROM service_specification_value
    UNION ALL SELECT 'service_offer', service_url FROM service_offer
    UNION ALL SELECT 'service_offer', coverage_summary FROM service_offer
    UNION ALL SELECT 'service_offer', additional_costs_summary FROM service_offer
    UNION ALL SELECT 'service_category', name FROM service_category
    UNION ALL SELECT 'service_category', description FROM service_category
    UNION ALL SELECT 'service_review', title FROM service_review
    UNION ALL SELECT 'service_review', content FROM service_review
    UNION ALL SELECT 'provider', name FROM provider
    UNION ALL SELECT 'provider', general_conditions FROM provider
    UNION ALL SELECT 'user_account', name FROM user_account
    UNION ALL SELECT 'user_account', email FROM user_account
    UNION ALL SELECT 'categories', name FROM categories
    UNION ALL SELECT 'categories', slug FROM categories
    UNION ALL SELECT 'categories', description FROM categories
    UNION ALL SELECT 'category_attributes', label FROM category_attributes
    UNION ALL SELECT 'saved_search', name FROM saved_search
    UNION ALL SELECT 'saved_search', query FROM saved_search
    UNION ALL SELECT 'notification', title FROM notification
    UNION ALL SELECT 'notification', message FROM notification
    UNION ALL SELECT 'search_query', raw_query FROM search_query
    UNION ALL SELECT 'category_artworks', svg_content FROM category_artworks
),
urls AS (
    -- Solo URLs no vacías: las vacías se cuentan como incompletas.
    SELECT 'product_image'::text AS tabla, image_url AS url
    FROM product_image WHERE btrim(COALESCE(image_url, '')) <> ''
    UNION ALL
    SELECT 'product_offer', product_url
    FROM product_offer WHERE btrim(COALESCE(product_url, '')) <> ''
    UNION ALL
    SELECT 'service', image_url
    FROM service WHERE btrim(COALESCE(image_url, '')) <> ''
    UNION ALL
    SELECT 'service_offer', service_url
    FROM service_offer WHERE btrim(COALESCE(service_url, '')) <> ''
    UNION ALL
    SELECT 'store', website_url
    FROM store WHERE btrim(COALESCE(website_url, '')) <> ''
    UNION ALL
    SELECT 'provider', website_url
    FROM provider WHERE btrim(COALESCE(website_url, '')) <> ''
    UNION ALL
    SELECT 'brand', website_url
    FROM brand WHERE btrim(COALESCE(website_url, '')) <> ''
    UNION ALL
    SELECT 'brand', logo_url
    FROM brand WHERE btrim(COALESCE(logo_url, '')) <> ''
    UNION ALL
    SELECT 'data_source', base_url
    FROM data_source WHERE btrim(COALESCE(base_url, '')) <> ''
),
checks("check", tabla, cantidad) AS (
    -- 1) INCOMPLETOS: columnas clave nulas o vacías ('' o solo espacios).

    SELECT 'producto activo sin category_ref', 'product', count(*)
    FROM product WHERE active AND category_ref IS NULL

    UNION ALL SELECT 'producto con name vacío', 'product', count(*)
    FROM product WHERE btrim(COALESCE(name, '')) = ''

    UNION ALL SELECT 'producto con model vacío', 'product', count(*)
    FROM product WHERE btrim(COALESCE(model, '')) = ''

    UNION ALL SELECT 'producto con sku vacío', 'product', count(*)
    FROM product WHERE btrim(COALESCE(sku, '')) = ''

    UNION ALL SELECT 'producto con description vacía', 'product', count(*)
    FROM product WHERE btrim(COALESCE(description, '')) = ''

    UNION ALL SELECT 'imagen de producto sin image_url', 'product_image', count(*)
    FROM product_image WHERE btrim(COALESCE(image_url, '')) = ''

    UNION ALL SELECT 'oferta de producto sin product_url', 'product_offer', count(*)
    FROM product_offer WHERE btrim(COALESCE(product_url, '')) = ''

    UNION ALL SELECT 'oferta de producto con currency vacía', 'product_offer', count(*)
    FROM product_offer WHERE btrim(COALESCE(currency, '')) = ''

    UNION ALL SELECT 'oferta de producto con price nulo', 'product_offer', count(*)
    FROM product_offer WHERE price IS NULL

    UNION ALL SELECT 'precio histórico nulo', 'product_price_history', count(*)
    FROM product_price_history WHERE price IS NULL

    UNION ALL SELECT 'servicio con name vacío', 'service', count(*)
    FROM service WHERE btrim(COALESCE(name, '')) = ''

    UNION ALL SELECT 'servicio sin image_url', 'service', count(*)
    FROM service WHERE btrim(COALESCE(image_url, '')) = ''

    UNION ALL SELECT 'servicio con description vacía', 'service', count(*)
    FROM service WHERE btrim(COALESCE(description, '')) = ''

    UNION ALL SELECT 'oferta de servicio sin service_url', 'service_offer', count(*)
    FROM service_offer WHERE btrim(COALESCE(service_url, '')) = ''

    UNION ALL SELECT 'oferta de servicio con price nulo', 'service_offer', count(*)
    FROM service_offer WHERE price IS NULL

    UNION ALL SELECT 'precio histórico de servicio nulo', 'service_price_history', count(*)
    FROM service_price_history WHERE price IS NULL

    UNION ALL SELECT 'categoría con name vacío', 'categories', count(*)
    FROM categories WHERE btrim(COALESCE(name, '')) = ''

    UNION ALL SELECT 'categoría con slug vacío', 'categories', count(*)
    FROM categories WHERE btrim(COALESCE(slug, '')) = ''

    UNION ALL SELECT 'categoría de producto activa sin nodo legado', 'categories', count(*)
    FROM categories
    WHERE item_type = 'product' AND active AND legacy_product_category_id IS NULL

    UNION ALL SELECT 'categoría de servicio activa sin nodo legado', 'categories', count(*)
    FROM categories
    WHERE item_type = 'service' AND active AND legacy_service_category_id IS NULL

    UNION ALL SELECT 'atributo de categoría con label vacío', 'category_attributes', count(*)
    FROM category_attributes WHERE btrim(COALESCE(label, '')) = ''

    UNION ALL SELECT 'atributo de categoría con key vacía', 'category_attributes', count(*)
    FROM category_attributes WHERE btrim(COALESCE(key, '')) = ''

    UNION ALL SELECT 'categoría de producto con name vacío', 'product_category', count(*)
    FROM product_category WHERE btrim(COALESCE(name, '')) = ''

    UNION ALL SELECT 'categoría de producto activa sin slug', 'product_category', count(*)
    FROM product_category WHERE active AND btrim(COALESCE(slug, '')) = ''

    UNION ALL SELECT 'categoría de servicio con name vacío', 'service_category', count(*)
    FROM service_category WHERE btrim(COALESCE(name, '')) = ''

    UNION ALL SELECT 'marca con name vacío', 'brand', count(*)
    FROM brand WHERE btrim(COALESCE(name, '')) = ''

    UNION ALL SELECT 'tienda con name vacío', 'store', count(*)
    FROM store WHERE btrim(COALESCE(name, '')) = ''

    UNION ALL SELECT 'tienda sin website_url', 'store', count(*)
    FROM store WHERE btrim(COALESCE(website_url, '')) = ''

    UNION ALL SELECT 'proveedor con name vacío', 'provider', count(*)
    FROM provider WHERE btrim(COALESCE(name, '')) = ''

    UNION ALL SELECT 'usuario con name vacío', 'user_account', count(*)
    FROM user_account WHERE btrim(COALESCE(name, '')) = ''

    UNION ALL SELECT 'usuario con email vacío', 'user_account', count(*)
    FROM user_account WHERE btrim(COALESCE(email, '')) = ''

    UNION ALL SELECT 'usuario con password_hash vacío', 'user_account', count(*)
    FROM user_account WHERE btrim(COALESCE(password_hash, '')) = ''

    UNION ALL SELECT 'búsqueda guardada con name vacío', 'saved_search', count(*)
    FROM saved_search WHERE btrim(COALESCE(name, '')) = ''

    UNION ALL SELECT 'búsqueda guardada con query vacía', 'saved_search', count(*)
    FROM saved_search WHERE btrim(COALESCE(query, '')) = ''

    UNION ALL SELECT 'notificación con title vacío', 'notification', count(*)
    FROM notification WHERE btrim(COALESCE(title, '')) = ''

    UNION ALL SELECT 'notificación con message vacío', 'notification', count(*)
    FROM notification WHERE btrim(COALESCE(message, '')) = ''

    UNION ALL SELECT 'consulta de búsqueda vacía', 'search_query', count(*)
    FROM search_query WHERE btrim(COALESCE(raw_query, '')) = ''

    UNION ALL SELECT 'valor de especificación de producto vacío', 'product_specification_value', count(*)
    FROM product_specification_value WHERE btrim(COALESCE(value, '')) = ''

    UNION ALL SELECT 'valor de atributo de producto vacío', 'product_attribute_value', count(*)
    FROM product_attribute_value WHERE btrim(COALESCE(value, '')) = ''

    UNION ALL SELECT 'valor de especificación de servicio vacío', 'service_specification_value', count(*)
    FROM service_specification_value WHERE btrim(COALESCE(value, '')) = ''

    UNION ALL SELECT 'ilustración de categoría vacía', 'category_artworks', count(*)
    FROM category_artworks WHERE btrim(COALESCE(svg_content, '')) = ''

    -- 2) INCONSISTENTES: valores fuera de regla, huérfanos, duplicados,
    -- fechas incoherentes y URLs inválidas.

    UNION ALL SELECT 'oferta de producto con price <= 0', 'product_offer', count(*)
    FROM product_offer WHERE price <= 0

    UNION ALL SELECT 'oferta de producto con list_price <= 0', 'product_offer', count(*)
    FROM product_offer WHERE list_price IS NOT NULL AND list_price <= 0

    UNION ALL SELECT 'oferta de producto con list_price < price', 'product_offer', count(*)
    FROM product_offer WHERE list_price IS NOT NULL AND list_price < price

    UNION ALL SELECT 'oferta de producto con costo de envío negativo', 'product_offer', count(*)
    FROM product_offer WHERE shipping_cost < 0

    UNION ALL SELECT 'oferta de producto con envío gratis y costo > 0', 'product_offer', count(*)
    FROM product_offer WHERE shipping_free AND shipping_cost > 0

    UNION ALL SELECT 'oferta de producto con stock negativo', 'product_offer', count(*)
    FROM product_offer WHERE stock < 0

    UNION ALL SELECT 'oferta disponible con stock nulo', 'product_offer', count(*)
    FROM product_offer WHERE available AND stock IS NULL

    UNION ALL SELECT 'oferta de producto con condition inválida', 'product_offer', count(*)
    FROM product_offer WHERE condition NOT IN ('new', 'used', 'refurbished')

    UNION ALL SELECT 'oferta de producto con currency fuera de ISO 4217', 'product_offer', count(*)
    FROM product_offer WHERE btrim(currency) !~ '^[A-Z]{3}$'

    UNION ALL SELECT 'precio histórico de producto <= 0', 'product_price_history', count(*)
    FROM product_price_history WHERE price <= 0

    UNION ALL SELECT 'oferta de servicio con price <= 0', 'service_offer', count(*)
    FROM service_offer WHERE price <= 0

    UNION ALL SELECT 'oferta de servicio con installation_cost negativo', 'service_offer', count(*)
    FROM service_offer WHERE installation_cost < 0

    UNION ALL SELECT 'precio histórico de servicio <= 0', 'service_price_history', count(*)
    FROM service_price_history WHERE price <= 0

    UNION ALL SELECT 'carrito con saved_price <= 0', 'cart_item', count(*)
    FROM cart_item WHERE saved_price <= 0

    UNION ALL SELECT 'carrito con cantidad <= 0', 'cart_item', count(*)
    FROM cart_item WHERE quantity <= 0

    UNION ALL SELECT 'carrito con producto distinto al de la oferta', 'cart_item', count(*)
    FROM cart_item ci
    JOIN product_offer po ON po.id = ci.offer_id
    WHERE ci.product_id <> po.product_id

    UNION ALL SELECT 'tienda con rating fuera de 0..5', 'store', count(*)
    FROM store WHERE rating IS NOT NULL AND rating NOT BETWEEN 0 AND 5

    UNION ALL SELECT 'proveedor con rating fuera de 0..5', 'provider', count(*)
    FROM provider WHERE rating IS NOT NULL AND rating NOT BETWEEN 0 AND 5

    UNION ALL SELECT 'reseña de producto con rating fuera de 1..5', 'product_review', count(*)
    FROM product_review WHERE rating NOT BETWEEN 1 AND 5

    UNION ALL SELECT 'reseña de servicio con rating fuera de 1..5', 'service_review', count(*)
    FROM service_review WHERE rating NOT BETWEEN 1 AND 5

    UNION ALL SELECT 'reseña de tienda con rating fuera de 1..5', 'store_review', count(*)
    FROM store_review WHERE rating NOT BETWEEN 1 AND 5

    UNION ALL SELECT 'ubicación con latitud fuera de rango', 'location', count(*)
    FROM location WHERE latitude IS NOT NULL AND latitude NOT BETWEEN -90 AND 90

    UNION ALL SELECT 'ubicación con longitud fuera de rango', 'location', count(*)
    FROM location WHERE longitude IS NOT NULL AND longitude NOT BETWEEN -180 AND 180

    UNION ALL SELECT 'imagen de producto con sort_order negativo', 'product_image', count(*)
    FROM product_image WHERE sort_order < 0

    UNION ALL SELECT 'especificación con display_order negativo', 'product_category_specification', count(*)
    FROM product_category_specification WHERE display_order < 0

    UNION ALL SELECT 'especificación de servicio con display_order negativo', 'service_category_specification', count(*)
    FROM service_category_specification WHERE display_order < 0

    UNION ALL SELECT 'atributo con display_order negativo', 'category_attributes', count(*)
    FROM category_attributes WHERE display_order < 0

    UNION ALL SELECT 'comparación de producto con position negativa', 'product_comparison_item', count(*)
    FROM product_comparison_item WHERE position < 0

    UNION ALL SELECT 'comparación de servicio con position negativa', 'service_comparison_item', count(*)
    FROM service_comparison_item WHERE position < 0

    UNION ALL SELECT 'oferta de producto apunta a producto inexistente', 'product_offer', count(*)
    FROM product_offer o
    LEFT JOIN product p ON p.id = o.product_id
    WHERE p.id IS NULL

    UNION ALL SELECT 'oferta de producto apunta a tienda inexistente', 'product_offer', count(*)
    FROM product_offer o
    LEFT JOIN store s ON s.id = o.store_id
    WHERE s.id IS NULL

    UNION ALL SELECT 'imagen apunta a producto inexistente', 'product_image', count(*)
    FROM product_image i
    LEFT JOIN product p ON p.id = i.product_id
    WHERE p.id IS NULL

    UNION ALL SELECT 'precio histórico apunta a oferta inexistente', 'product_price_history', count(*)
    FROM product_price_history h
    LEFT JOIN product_offer o ON o.id = h.product_offer_id
    WHERE o.id IS NULL

    UNION ALL SELECT 'valor de atributo apunta a producto inexistente', 'product_attribute_value', count(*)
    FROM product_attribute_value v
    LEFT JOIN product p ON p.id = v.product_id
    WHERE p.id IS NULL

    UNION ALL SELECT 'valor de atributo apunta a atributo inexistente', 'product_attribute_value', count(*)
    FROM product_attribute_value v
    LEFT JOIN category_attributes a ON a.id = v.attribute_id
    WHERE a.id IS NULL

    UNION ALL SELECT 'producto con category_ref inexistente o no de producto', 'product', count(*)
    FROM product p
    LEFT JOIN categories c ON c.id = p.category_ref AND c.item_type = 'product'
    WHERE p.category_ref IS NOT NULL AND c.id IS NULL

    UNION ALL SELECT 'producto con category_ref inactiva', 'product', count(*)
    FROM product p
    JOIN categories c ON c.id = p.category_ref
    WHERE NOT c.active

    UNION ALL SELECT 'categoría con nodo legado de producto inexistente', 'categories', count(*)
    FROM categories c
    LEFT JOIN product_category pc ON pc.id = c.legacy_product_category_id
    WHERE c.legacy_product_category_id IS NOT NULL AND pc.id IS NULL

    UNION ALL SELECT 'categoría con nodo legado de servicio inexistente', 'categories', count(*)
    FROM categories c
    LEFT JOIN service_category sc ON sc.id = c.legacy_service_category_id
    WHERE c.legacy_service_category_id IS NOT NULL AND sc.id IS NULL

    UNION ALL SELECT 'oferta de servicio apunta a servicio inexistente', 'service_offer', count(*)
    FROM service_offer o
    LEFT JOIN service s ON s.id = o.service_id
    WHERE s.id IS NULL

    UNION ALL SELECT 'oferta de servicio apunta a proveedor inexistente', 'service_offer', count(*)
    FROM service_offer o
    LEFT JOIN provider p ON p.id = o.provider_id
    WHERE p.id IS NULL

    UNION ALL SELECT 'precio histórico apunta a oferta de servicio inexistente', 'service_price_history', count(*)
    FROM service_price_history h
    LEFT JOIN service_offer o ON o.id = h.service_offer_id
    WHERE o.id IS NULL

    UNION ALL SELECT 'fuente de datos de tienda sin store_id', 'data_source', count(*)
    FROM data_source WHERE source_type = 'store' AND store_id IS NULL

    UNION ALL SELECT 'fuente de datos apunta a tienda inexistente', 'data_source', count(*)
    FROM data_source d
    LEFT JOIN store s ON s.id = d.store_id
    WHERE d.store_id IS NOT NULL AND s.id IS NULL

    UNION ALL SELECT 'carrito apunta a usuario inexistente', 'cart_item', count(*)
    FROM cart_item c
    LEFT JOIN user_account u ON u.id = c.user_id
    WHERE u.id IS NULL

    UNION ALL SELECT 'carrito apunta a oferta inexistente', 'cart_item', count(*)
    FROM cart_item c
    LEFT JOIN product_offer o ON o.id = c.offer_id
    WHERE o.id IS NULL

    UNION ALL SELECT 'búsqueda guardada apunta a usuario inexistente', 'saved_search', count(*)
    FROM saved_search s
    LEFT JOIN user_account u ON u.id = s.user_id
    WHERE u.id IS NULL

    UNION ALL SELECT 'notificación apunta a usuario inexistente', 'notification', count(*)
    FROM notification n
    LEFT JOIN user_account u ON u.id = n.user_id
    WHERE u.id IS NULL

    UNION ALL SELECT 'sesión apunta a usuario inexistente', 'user_session', count(*)
    FROM user_session s
    LEFT JOIN user_account u ON u.id = s.user_id
    WHERE u.id IS NULL

    UNION ALL SELECT 'procesamiento IA apunta a búsqueda inexistente', 'ai_query_process', count(*)
    FROM ai_query_process a
    LEFT JOIN search_query s ON s.id = a.search_query_id
    WHERE s.id IS NULL

    UNION ALL SELECT 'consulta IA apunta a usuario inexistente', 'ai_comparison_query', count(*)
    FROM ai_comparison_query a
    LEFT JOIN user_account u ON u.id = a.user_id
    WHERE a.user_id IS NOT NULL AND u.id IS NULL

    UNION ALL SELECT 'consulta IA referencia un producto inexistente', 'ai_comparison_query', count(*)
    FROM ai_comparison_query a
    WHERE EXISTS (
        SELECT 1
        FROM unnest(a.product_ids) AS u(pid)
        WHERE NOT EXISTS (SELECT 1 FROM product p WHERE p.public_id = u.pid)
    )

    UNION ALL SELECT 'comparación de producto apunta a usuario inexistente', 'product_comparison', count(*)
    FROM product_comparison c
    LEFT JOIN user_account u ON u.id = c.user_id
    WHERE u.id IS NULL

    UNION ALL SELECT 'interés de producto apunta a producto inexistente', 'product_user_interest', count(*)
    FROM product_user_interest i
    LEFT JOIN product p ON p.id = i.product_id
    WHERE p.id IS NULL

    UNION ALL SELECT 'reseña de producto apunta a producto inexistente', 'product_review', count(*)
    FROM product_review r
    LEFT JOIN product p ON p.id = r.product_id
    WHERE p.id IS NULL

    UNION ALL SELECT 'reseña de tienda apunta a tienda inexistente', 'store_review', count(*)
    FROM store_review r
    LEFT JOIN store s ON s.id = r.store_id
    WHERE s.id IS NULL

    UNION ALL SELECT 'reseña de servicio apunta a servicio inexistente', 'service_review', count(*)
    FROM service_review r
    LEFT JOIN service s ON s.id = r.service_id
    WHERE s.id IS NULL

    UNION ALL SELECT 'marca duplicada por nombre normalizado', 'brand', COALESCE(sum(cnt), 0)::bigint
    FROM (
        SELECT count(*) AS cnt
        FROM brand
        GROUP BY lower(btrim(name))
        HAVING count(*) > 1
    ) d

    UNION ALL SELECT 'tienda duplicada por nombre normalizado', 'store', COALESCE(sum(cnt), 0)::bigint
    FROM (
        SELECT count(*) AS cnt
        FROM store
        GROUP BY lower(btrim(name))
        HAVING count(*) > 1
    ) d

    UNION ALL SELECT 'proveedor duplicado por nombre normalizado', 'provider', COALESCE(sum(cnt), 0)::bigint
    FROM (
        SELECT count(*) AS cnt
        FROM provider
        GROUP BY lower(btrim(name))
        HAVING count(*) > 1
    ) d

    UNION ALL SELECT 'usuario duplicado por email normalizado', 'user_account', COALESCE(sum(cnt), 0)::bigint
    FROM (
        SELECT count(*) AS cnt
        FROM user_account
        GROUP BY lower(btrim(email))
        HAVING count(*) > 1
    ) d

    UNION ALL SELECT 'producto duplicado por sku normalizado', 'product', COALESCE(sum(cnt), 0)::bigint
    FROM (
        SELECT count(*) AS cnt
        FROM product
        WHERE btrim(COALESCE(sku, '')) <> ''
        GROUP BY lower(btrim(sku))
        HAVING count(*) > 1
    ) d

    UNION ALL SELECT 'producto activo duplicado por marca, categoría y nombre', 'product', COALESCE(sum(cnt), 0)::bigint
    FROM (
        SELECT count(*) AS cnt
        FROM product
        WHERE active AND brand_id IS NOT NULL AND category_ref IS NOT NULL
        GROUP BY brand_id, category_ref, lower(btrim(name))
        HAVING count(*) > 1
    ) d

    UNION ALL SELECT 'servicio activo duplicado por categoría y nombre', 'service', COALESCE(sum(cnt), 0)::bigint
    FROM (
        SELECT count(*) AS cnt
        FROM service
        WHERE active
        GROUP BY category_id, lower(btrim(name))
        HAVING count(*) > 1
    ) d

    UNION ALL SELECT 'categoría duplicada por tipo y nombre normalizado', 'categories', COALESCE(sum(cnt), 0)::bigint
    FROM (
        SELECT count(*) AS cnt
        FROM categories
        GROUP BY item_type, lower(btrim(name))
        HAVING count(*) > 1
    ) d

    UNION ALL SELECT 'oferta duplicada por URL normalizada en la misma tienda', 'product_offer', COALESCE(sum(cnt), 0)::bigint
    FROM (
        SELECT count(*) AS cnt
        FROM product_offer
        WHERE btrim(COALESCE(product_url, '')) <> ''
        GROUP BY store_id, split_part(normalize_product_offer_url(product_url), '?', 1)
        HAVING count(*) > 1
    ) d

    UNION ALL SELECT 'imagen duplicada por URL en el mismo producto', 'product_image', COALESCE(sum(cnt), 0)::bigint
    FROM (
        SELECT count(*) AS cnt
        FROM product_image
        GROUP BY product_id, lower(btrim(image_url))
        HAVING count(*) > 1
    ) d

    UNION ALL SELECT 'producto con updated_at anterior a created_at', 'product', count(*)
    FROM product WHERE updated_at < created_at

    UNION ALL SELECT 'servicio con updated_at anterior a created_at', 'service', count(*)
    FROM service WHERE updated_at < created_at

    UNION ALL SELECT 'usuario con updated_at anterior a created_at', 'user_account', count(*)
    FROM user_account WHERE updated_at < created_at

    UNION ALL SELECT 'categoría de producto con updated_at anterior a created_at', 'product_category', count(*)
    FROM product_category WHERE updated_at < created_at

    UNION ALL SELECT 'categoría de servicio con updated_at anterior a created_at', 'service_category', count(*)
    FROM service_category WHERE updated_at < created_at

    UNION ALL SELECT 'tienda con updated_at anterior a created_at', 'store', count(*)
    FROM store WHERE updated_at < created_at

    UNION ALL SELECT 'proveedor con updated_at anterior a created_at', 'provider', count(*)
    FROM provider WHERE updated_at < created_at

    UNION ALL SELECT 'categoría con updated_at anterior a created_at', 'categories', count(*)
    FROM categories WHERE updated_at < created_at

    UNION ALL SELECT 'reseña de producto con updated_at anterior a created_at', 'product_review', count(*)
    FROM product_review WHERE updated_at < created_at

    UNION ALL SELECT 'reseña de servicio con updated_at anterior a created_at', 'service_review', count(*)
    FROM service_review WHERE updated_at < created_at

    UNION ALL SELECT 'reseña de tienda con updated_at anterior a created_at', 'store_review', count(*)
    FROM store_review WHERE updated_at < created_at

    UNION ALL SELECT 'carrito con updated_at anterior a created_at', 'cart_item', count(*)
    FROM cart_item WHERE updated_at < created_at

    UNION ALL SELECT 'búsqueda guardada con updated_at anterior a created_at', 'saved_search', count(*)
    FROM saved_search WHERE updated_at < created_at

    UNION ALL SELECT 'notificación leída antes de crearse', 'notification', count(*)
    FROM notification WHERE read_at IS NOT NULL AND read_at < created_at

    UNION ALL SELECT 'ejecución de ingesta completada antes de crearse', 'ingestion_run', count(*)
    FROM ingestion_run WHERE completed_at IS NOT NULL AND completed_at < created_at

    UNION ALL SELECT 'ejecución de scraping terminada antes de iniciar', 'scrape_run', count(*)
    FROM scrape_run WHERE finished_at IS NOT NULL AND finished_at < started_at

    UNION ALL SELECT 'sesión con refresh anterior al access', 'user_session', count(*)
    FROM user_session WHERE refresh_expires_at < access_expires_at

    UNION ALL SELECT 'producto creado en el futuro', 'product', count(*)
    FROM product WHERE created_at > NOW()

    UNION ALL SELECT 'servicio creado en el futuro', 'service', count(*)
    FROM service WHERE created_at > NOW()

    UNION ALL SELECT 'usuario creado en el futuro', 'user_account', count(*)
    FROM user_account WHERE created_at > NOW()

    UNION ALL SELECT 'oferta de producto actualizada en el futuro', 'product_offer', count(*)
    FROM product_offer WHERE last_updated > NOW()

    UNION ALL SELECT 'oferta de servicio actualizada en el futuro', 'service_offer', count(*)
    FROM service_offer WHERE last_updated > NOW()

    UNION ALL SELECT 'precio histórico registrado en el futuro', 'product_price_history', count(*)
    FROM product_price_history WHERE recorded_at > NOW()

    UNION ALL SELECT 'precio histórico de servicio registrado en el futuro', 'service_price_history', count(*)
    FROM service_price_history WHERE recorded_at > NOW()

    UNION ALL SELECT 'atributo numérico sin value_numeric', 'product_attribute_value', count(*)
    FROM product_attribute_value v
    JOIN category_attributes a ON a.id = v.attribute_id
    WHERE a.data_type IN ('integer', 'decimal') AND v.value_numeric IS NULL

    UNION ALL SELECT 'atributo no numérico con value_numeric', 'product_attribute_value', count(*)
    FROM product_attribute_value v
    JOIN category_attributes a ON a.id = v.attribute_id
    WHERE a.data_type NOT IN ('integer', 'decimal') AND v.value_numeric IS NOT NULL

    UNION ALL SELECT 'atributo con min_value > max_value', 'category_attributes', count(*)
    FROM category_attributes
    WHERE min_value IS NOT NULL AND max_value IS NOT NULL AND min_value > max_value

    UNION ALL SELECT 'atributo no numérico con min_value o max_value', 'category_attributes', count(*)
    FROM category_attributes
    WHERE data_type NOT IN ('integer', 'decimal')
      AND (min_value IS NOT NULL OR max_value IS NOT NULL)

    UNION ALL SELECT 'atributo con max_length fuera de 1..255', 'category_attributes', count(*)
    FROM category_attributes WHERE max_length IS NOT NULL AND max_length NOT BETWEEN 1 AND 255

    UNION ALL SELECT 'atributo enum sin allowed_values', 'category_attributes', count(*)
    FROM category_attributes
    WHERE data_type = 'enum' AND (allowed_values IS NULL OR cardinality(allowed_values) = 0)

    UNION ALL SELECT 'URL inválida (no http/https)', tabla, count(*)
    FROM urls
    WHERE url !~* '^https?://[^[:space:]/?#]+([/?#][^[:space:]]*)?$'
    GROUP BY tabla

    -- 3) CORRUPTOS: caracteres de reemplazo, mojibake y JSON mal formado.

    UNION ALL SELECT 'texto con U+FFFD o mojibake', tabla, count(*)
    FROM textos
    WHERE valor LIKE '%' || chr(65533) || '%'
       OR valor LIKE '%' || U&'\00c3' || '%'
       OR valor LIKE '%' || U&'\00e2\20ac' || '%'
    GROUP BY tabla

    UNION ALL SELECT 'caracteres de control en name', 'product', count(*)
    FROM product WHERE name ~ '[[:cntrl:]]'

    UNION ALL SELECT 'caracteres de control en name', 'service', count(*)
    FROM service WHERE name ~ '[[:cntrl:]]'

    UNION ALL SELECT 'filters no es un objeto JSON', 'saved_search', count(*)
    FROM saved_search WHERE jsonb_typeof(filters) <> 'object'

    UNION ALL SELECT 'data no es un objeto JSON', 'notification', count(*)
    FROM notification WHERE jsonb_typeof(data) <> 'object'

    UNION ALL SELECT 'details no es un objeto JSON', 'ingestion_run', count(*)
    FROM ingestion_run WHERE jsonb_typeof(details) <> 'object'

    UNION ALL SELECT 'structured_json no es un objeto JSON', 'ai_query_process', count(*)
    FROM ai_query_process WHERE structured_json IS NOT NULL AND jsonb_typeof(structured_json) <> 'object'

    UNION ALL SELECT 'parser_config no es un objeto JSON', 'scraper_config', count(*)
    FROM scraper_config WHERE parser_config IS NOT NULL AND jsonb_typeof(parser_config) <> 'object'

    UNION ALL SELECT 'raw_data no es un objeto JSON', 'scraped_data', count(*)
    FROM scraped_data WHERE raw_data IS NOT NULL AND jsonb_typeof(raw_data) <> 'object'

    UNION ALL SELECT 'filters con U+FFFD o mojibake', 'saved_search', count(*)
    FROM saved_search
    WHERE filters::text LIKE '%' || chr(65533) || '%'
       OR filters::text LIKE '%' || U&'\00c3' || '%'
       OR filters::text LIKE '%' || U&'\00e2\20ac' || '%'

    UNION ALL SELECT 'data con U+FFFD o mojibake', 'notification', count(*)
    FROM notification
    WHERE data::text LIKE '%' || chr(65533) || '%'
       OR data::text LIKE '%' || U&'\00c3' || '%'
       OR data::text LIKE '%' || U&'\00e2\20ac' || '%'

    UNION ALL SELECT 'details con U+FFFD o mojibake', 'ingestion_run', count(*)
    FROM ingestion_run
    WHERE details::text LIKE '%' || chr(65533) || '%'
       OR details::text LIKE '%' || U&'\00c3' || '%'
       OR details::text LIKE '%' || U&'\00e2\20ac' || '%'

    UNION ALL SELECT 'structured_json con U+FFFD o mojibake', 'ai_query_process', count(*)
    FROM ai_query_process
    WHERE structured_json IS NOT NULL
      AND (structured_json::text LIKE '%' || chr(65533) || '%'
           OR structured_json::text LIKE '%' || U&'\00c3' || '%'
           OR structured_json::text LIKE '%' || U&'\00e2\20ac' || '%')
)
SELECT "check", tabla, cantidad
FROM checks
ORDER BY cantidad DESC, "check", tabla;
