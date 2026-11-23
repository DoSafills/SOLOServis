-- TAL-241: borra los productos, usuarios y marcas de demostración del seed sin
-- tocar lo que trae el scraper.
--
-- Qué se borra:
--   - Los 38 productos que siembran 003, 005_demo_catalog, 005_seed_catalogo,
--     006_refrigeradores, 007_Computacion y 008_Telefonia, identificados por su
--     SKU. Ingestion no inventa SKU y el scraper no los manda: un producto sin
--     SKU o con otro SKU no se toca. Las FK borran en cascada sus ofertas,
--     historial, imágenes, specs, atributos, reseñas, intereses y carrito.
--   - Los usuarios del seed 003, cuyo password_hash es "hash_demo_N": nadie
--     podía entrar con ellos. Sus reseñas, comparaciones e intereses se van en
--     cascada; search_query y ai_comparison_query quedan con user_id NULL.
--   - Las marcas del seed que quedan sin ningún producto, porque
--     GET /search/brands lista todas las marcas activas. Si el scraper trae
--     después un producto de esa marca, ingestion la vuelve a crear.
--
-- Rescate: el scraper reconoce un producto por la URL de su oferta y, si dos
-- productos comparten URL, elige el de menor id. Así actualizó la oferta de un
-- producto demo (el Midea MDRS710 de Ripley, que comparte URL con el producto
-- scrapeado). Antes de borrar, el historial que el scraper dejó en esa oferta
-- (posterior al alta del producto real) se copia a la oferta del producto real,
-- y esta toma el precio, la disponibilidad y la fecha más recientes. Sin
-- normalize_product_offer_url (migración 009) el rescate se omite con un aviso.
--
-- Va en un solo bloque: o se aplica completo o no cambia nada. Es idempotente:
-- una segunda pasada no encuentra filas que borrar. No tiene reversa: el
-- respaldo es el pg_dump previo al deploy.
DO $$
DECLARE
    -- SKU de los productos del seed, en el orden de las migraciones que los crean.
    skus_demo CONSTANT text[] := ARRAY[
        -- 003_seed
        'SKU-SAMS-001', 'SKU-LG-001', 'SKU-SAMS-002', 'SKU-LG-002',
        -- 005_demo_catalog
        'DEMO-APPLE-AIRPODS-PRO2', 'DEMO-APPLE-IPHONE15-128', 'DEMO-ASUS-TUF-A15',
        'DEMO-BOSCH-WASHER-9', 'DEMO-HP-PAVILION-15', 'DEMO-JBL-FLIP6',
        'DEMO-LENOVO-IDEAPAD-16', 'DEMO-LG-FRIDGE-530', 'DEMO-LG-TV-OLED-65',
        'DEMO-MIDEA-FRIDGE-400', 'DEMO-MOTO-EDGE50-256', 'DEMO-NINTENDO-SWITCH-OLED',
        'DEMO-PHILIPS-AIRFRYER-62', 'DEMO-SAMSUNG-S24-256', 'DEMO-SAMSUNG-WASHER-11',
        'DEMO-SONY-HEADPHONES-XM5', 'DEMO-SONY-PS5-SLIM', 'DEMO-SONY-TV-55-4K',
        'DEMO-TCL-TV-QLED-50', 'DEMO-XBOX-SERIES-X-1TB', 'DEMO-XIAOMI-REDMI13-256',
        'DEMO-XIAOMI-ROBOT-VACUUM',
        -- 005_seed_catalogo
        'SKU-LEN-001', 'SKU-APP-001', 'SKU-XIA-001',
        -- 006_refrigeradores
        'SAMSUNG-MKF9HNZ5C6', 'LG-LM22SGPK', 'MIDEA-MDRS710FGE50IN',
        -- 007_Computacion
        'HP-VICTUS-15-FB3004LA', 'ASUS-TUF-A15-FA506-R7-RTX3050', 'LENOVO-LOQ-R7-RTX5060-16-512',
        -- 008_Telefonia
        'APPLE-IPHONE-16-128', 'SAMSUNG-GALAXY-S26-ULTRA-256', 'XIAOMI-17-ULTRA-512'];
    -- Marcas que crean esos seeds. Solo se borran las que quedan sin productos.
    marcas_demo CONSTANT text[] := ARRAY[
        'Apple', 'ASUS', 'Bosch', 'Dell', 'HP', 'JBL', 'Lenovo', 'LG', 'Microsoft',
        'Midea', 'Motorola', 'Nintendo', 'Philips', 'Samsung', 'Sony', 'TCL', 'Xiaomi'];
    demo_ids integer[];
    ultimo_historial integer;
    historial_copiado integer := 0;
    ofertas_actualizadas integer := 0;
    productos_borrados integer;
    usuarios_borrados integer;
    marcas_borradas integer;
BEGIN
    demo_ids := ARRAY(SELECT id FROM product WHERE sku = ANY (skus_demo));

    IF to_regprocedure('normalize_product_offer_url(text)') IS NULL THEN
        RAISE NOTICE 'delete_demo_seed: falta normalize_product_offer_url (migración 009); se omite el rescate de ofertas';
    ELSE
        -- Cada oferta demo con la misma tienda y la misma URL (sin parámetros)
        -- que una oferta de un producto real, con el mismo desempate que ingestion.
        CREATE TEMP TABLE tal241_rescate ON COMMIT DROP AS
        SELECT DISTINCT ON (demo.id)
               demo.id AS demo_offer_id, real_offer.id AS real_offer_id, real_product.created_at AS desde
        FROM product_offer demo
        JOIN product_offer real_offer
          ON real_offer.store_id = demo.store_id
         AND split_part(normalize_product_offer_url(real_offer.product_url), '?', 1) =
             split_part(normalize_product_offer_url(demo.product_url), '?', 1)
        JOIN product real_product ON real_product.id = real_offer.product_id
        WHERE demo.product_id = ANY (demo_ids)
          AND real_product.id <> ALL (demo_ids)
        ORDER BY demo.id, real_product.active DESC, real_product.id, real_offer.id;

        -- El historial que el scraper dejó en la oferta demo después del alta
        -- del producto real; el del seed es anterior y se descarta.
        INSERT INTO product_price_history (product_offer_id, price, recorded_at)
        SELECT r.real_offer_id, h.price, h.recorded_at
        FROM tal241_rescate r
        JOIN product_price_history h ON h.product_offer_id = r.demo_offer_id
        WHERE h.recorded_at > r.desde
        ON CONFLICT DO NOTHING;
        GET DIAGNOSTICS historial_copiado = ROW_COUNT;

        -- La oferta real toma los datos de la demo si esta se actualizó después.
        -- El trigger de 010 anota el cambio de precio con la hora de esta
        -- migración; esa fila se quita porque el precio ya está en el historial
        -- copiado con la hora en que el scraper lo vio.
        SELECT COALESCE(max(id), 0) INTO ultimo_historial FROM product_price_history;
        UPDATE product_offer real_offer
        SET price = demo.price,
            list_price = demo.list_price,
            available = demo.available,
            stock = demo.stock,
            last_updated = demo.last_updated
        FROM (
            SELECT DISTINCT ON (r.real_offer_id) r.real_offer_id, o.*
            FROM tal241_rescate r
            JOIN product_offer o ON o.id = r.demo_offer_id
            ORDER BY r.real_offer_id, o.last_updated DESC
        ) demo
        WHERE real_offer.id = demo.real_offer_id
          AND demo.last_updated > real_offer.last_updated;
        GET DIAGNOSTICS ofertas_actualizadas = ROW_COUNT;
        DELETE FROM product_price_history
        WHERE id > ultimo_historial
          AND product_offer_id IN (SELECT real_offer_id FROM tal241_rescate);
    END IF;

    DELETE FROM product WHERE id = ANY (demo_ids);
    GET DIAGNOSTICS productos_borrados = ROW_COUNT;

    DELETE FROM user_account WHERE password_hash LIKE 'hash\_demo\_%';
    GET DIAGNOSTICS usuarios_borrados = ROW_COUNT;

    DELETE FROM brand b
    WHERE b.name = ANY (marcas_demo)
      AND NOT EXISTS (SELECT 1 FROM product p WHERE p.brand_id = b.id);
    GET DIAGNOSTICS marcas_borradas = ROW_COUNT;

    RAISE NOTICE 'delete_demo_seed: % productos, % usuarios y % marcas borrados; % precios rescatados, % ofertas actualizadas',
        productos_borrados, usuarios_borrados, marcas_borradas, historial_copiado, ofertas_actualizadas;
END;
$$;
