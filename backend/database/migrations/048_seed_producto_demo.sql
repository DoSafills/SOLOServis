-- 048: producto de demostración completo para la ficha pública.
--
-- Inserta un único producto nuevo (SKU SEED-WHIRLPOOL-WRS571CIHZ) con su
-- marca, tres imágenes, tres ofertas de tiendas que ya existen (Falabella,
-- Ripley y Lider), historial de precios de más de doce meses y los cinco
-- atributos de la categoría "refrigeradores".
--
-- No crea usuarios ni reseñas: product_review.user_id es NOT NULL y la
-- migración 044 borra los usuarios del seed, así que la vista
-- product_rating_summary devuelve rating 0 y reviewCount 0.
--
-- Es idempotente por SKU y claves naturales: una segunda pasada no duplica
-- filas ni cambia los precios. Todo va en una transacción; si la validación
-- final falla, no queda nada aplicado.
BEGIN;

DO $$
DECLARE
    -- Identidad del producto. El prefijo SEED lo distingue de los SKU que
    -- borra 044_delete_demo_seed.sql.
    sku_producto CONSTANT TEXT := 'SEED-WHIRLPOOL-WRS571CIHZ';
    v_descripcion CONSTANT TEXT := $desc$Refrigerador side by side Whirlpool de 570 litros, con tecnología No Frost, compresor Inverter y eficiencia energética A++. Dispensador de agua y hielo, estantes de vidrio templado y cajón de verduras con control de humedad.

Incluye:
- Refrigerador Whirlpool WRS571CIHZ
- Manual de uso y certificado de garantía
- Kit de conexión de agua para el dispensador
- Bandeja de hielo y estantes ajustables

Cuotas y medios de pago:
- Hasta 12 cuotas sin interés con tarjetas participantes
- Hasta 24 cuotas con interés
- Débito, crédito, transferencia y pago en tienda

Despacho, garantía y devolución:
- Despacho a todo Chile; instalación opcional con costo adicional
- Garantía legal de 6 meses y garantía del fabricante de 12 meses
- Devolución gratuita dentro de 30 días si el producto no fue usado

Variantes y etiquetas:
- Disponible en inoxidable y negro mate
- Etiquetas: No Frost, A++, Inverter, Dispensador de agua, Side by Side$desc$;

    -- Categoría: hoja "refrigeradores" del árbol y su fila unificada.
    v_category_id   INTEGER;
    v_category_ref  INTEGER;
    v_brand_id      INTEGER;
    v_product_id    INTEGER;
    v_public_id     UUID;
    v_activo        BOOLEAN;

    -- Tiendas existentes, resueltas por nombre normalizado.
    v_store_falabella INTEGER;
    v_store_ripley    INTEGER;
    v_store_lider     INTEGER;

    -- Ofertas del producto (una por tienda).
    v_offer_falabella INTEGER;
    v_offer_ripley    INTEGER;
    v_offer_lider     INTEGER;

    -- Conteos y precios para la validación final.
    v_imagenes      INTEGER;
    v_ofertas       INTEGER;
    v_ofertas_ok    INTEGER;
    v_atributos     INTEGER;
    v_historial     INTEGER;
    v_precio_min    NUMERIC;
    v_precio_max    NUMERIC;
    v_resenas       BIGINT;
    v_rating        NUMERIC;
BEGIN
    -- 1. Categoría del producto: la hoja del árbol y la fila unificada que
    --    la representa. Ambas deben existir y estar activas.
    SELECT pc.id, c.id
    INTO v_category_id, v_category_ref
    FROM product_category pc
    JOIN categories c
      ON c.legacy_product_category_id = pc.id
     AND c.item_type = 'product'
     AND c.active
    WHERE pc.slug = 'refrigeradores'
      AND pc.active;

    IF v_category_id IS NULL OR v_category_ref IS NULL THEN
        RAISE EXCEPTION 'seed 048: no existe la categoría activa refrigeradores';
    END IF;

    -- 2. Tiendas que ya siembran 003_seed y 005_demo_catalog. Si falta
    --    alguna, se falla con un mensaje claro en vez de inventarla.
    SELECT id INTO v_store_falabella FROM store WHERE LOWER(BTRIM(name)) = LOWER('Falabella') AND active;
    SELECT id INTO v_store_ripley    FROM store WHERE LOWER(BTRIM(name)) = LOWER('Ripley')    AND active;
    SELECT id INTO v_store_lider     FROM store WHERE LOWER(BTRIM(name)) = LOWER('Lider')     AND active;

    IF v_store_falabella IS NULL OR v_store_ripley IS NULL OR v_store_lider IS NULL THEN
        RAISE EXCEPTION 'seed 048: faltan las tiendas Falabella, Ripley o Lider';
    END IF;

    -- 3. Marca.
    INSERT INTO brand (name, logo_url, website_url)
    VALUES ('Whirlpool', 'https://placehold.co/128x128?text=Whirlpool', 'https://www.whirlpool.cl')
    ON CONFLICT (name) DO UPDATE SET active = TRUE
    RETURNING id INTO v_brand_id;

    -- 4. Producto, idempotente por SKU.
    INSERT INTO product (category_id, category_ref, brand_id, name, model, sku, description, active)
    VALUES (
        v_category_id,
        v_category_ref,
        v_brand_id,
        'Refrigerador Whirlpool Side by Side 570 L Inoxidable',
        'WRS571CIHZ',
        sku_producto,
        v_descripcion,
        TRUE
    )
    ON CONFLICT (sku) DO UPDATE
    SET category_id  = EXCLUDED.category_id,
        category_ref = EXCLUDED.category_ref,
        brand_id     = EXCLUDED.brand_id,
        name         = EXCLUDED.name,
        model        = EXCLUDED.model,
        description  = EXCLUDED.description,
        active       = TRUE,
        updated_at   = NOW()
    RETURNING id INTO v_product_id;

    -- 5. Imágenes, idempotentes por (producto, URL).
    INSERT INTO product_image (product_id, image_url, alt_text, sort_order)
    SELECT v_product_id, imagen.url, imagen.alt, imagen.orden
    FROM (VALUES
        (0, 'https://placehold.co/1200x900?text=Refrigerador+Whirlpool+Side+by+Side+570+L',
         'Refrigerador Whirlpool Side by Side 570 L, vista frontal'),
        (1, 'https://placehold.co/1200x900?text=Interior+No+Frost+570+L',
         'Interior del refrigerador con estantes de vidrio y cajón de verduras'),
        (2, 'https://placehold.co/1200x900?text=Dispensador+de+agua+Inoxidable',
         'Dispensador de agua y hielo en acero inoxidable')
    ) AS imagen(orden, url, alt)
    WHERE NOT EXISTS (
        SELECT 1
        FROM product_image existente
        WHERE existente.product_id = v_product_id
          AND existente.image_url = imagen.url
    );

    -- 6. Ofertas. Cada precio es el de lista menos su descuento:
    --    Falabella 1.199.990 - 200.000 = 999.990
    --    Ripley    1.199.990 - 150.000 = 1.049.990
    --    Lider     1.149.990 - 120.000 = 1.029.990
    --    La identidad de la oferta es (producto, tienda, URL normalizada).
    INSERT INTO product_offer (
        product_id, store_id, price, list_price, currency, shipping_cost,
        shipping_free, available, stock, condition, product_url, last_updated
    )
    SELECT
        v_product_id,
        oferta.store_id,
        oferta.price,
        oferta.list_price,
        'CLP',
        oferta.shipping_cost,
        oferta.shipping_free,
        TRUE,
        oferta.stock,
        'new',
        oferta.product_url,
        TIMESTAMPTZ '2026-11-20 10:00:00-03'
    FROM (VALUES
        (v_store_falabella, 999990::NUMERIC, 1199990::NUMERIC, 0::NUMERIC, TRUE, 12,
         'https://www.falabella.com/falabella-cl/product/113456789/Refrigerador-Whirlpool-Side-by-Side-570-L-Inoxidable'),
        (v_store_ripley, 1049990::NUMERIC, 1199990::NUMERIC, 19990::NUMERIC, FALSE, 7,
         'https://simple.ripley.cl/refrigerador-whirlpool-side-by-side-570-l-inoxidable-2000412345678p'),
        (v_store_lider, 1029990::NUMERIC, 1149990::NUMERIC, 0::NUMERIC, TRUE, 4,
         'https://www.lider.cl/refrigerador-whirlpool-side-by-side-570-l-inoxidable/p/2000412345')
    ) AS oferta(store_id, price, list_price, shipping_cost, shipping_free, stock, product_url)
    ON CONFLICT (product_id, store_id, (COALESCE(normalize_product_offer_url(product_url), '')))
    DO UPDATE SET
        price         = EXCLUDED.price,
        list_price    = EXCLUDED.list_price,
        currency      = EXCLUDED.currency,
        shipping_cost = EXCLUDED.shipping_cost,
        shipping_free = EXCLUDED.shipping_free,
        available     = EXCLUDED.available,
        stock         = EXCLUDED.stock,
        condition     = EXCLUDED."condition",
        last_updated  = EXCLUDED.last_updated;

    SELECT id INTO v_offer_falabella FROM product_offer WHERE product_id = v_product_id AND store_id = v_store_falabella;
    SELECT id INTO v_offer_ripley    FROM product_offer WHERE product_id = v_product_id AND store_id = v_store_ripley;
    SELECT id INTO v_offer_lider     FROM product_offer WHERE product_id = v_product_id AND store_id = v_store_lider;

    -- 7. Historial: doce puntos mensuales por oferta (primer día del mes a las
    --    09:00) más la fila que crea el trigger de 010 con el precio actual.
    --    Los meses 7 y 2 quedan marcados como promocionales (Cyber).
    INSERT INTO product_price_history (product_offer_id, price, is_promotional, recorded_at)
    SELECT
        v_offer_falabella,
        punto.precio,
        (13 - punto.pos) IN (7, 2),
        date_trunc('month', CURRENT_DATE)
            - make_interval(months => (13 - punto.pos)::INTEGER)
            + INTERVAL '9 hours'
    FROM unnest(ARRAY[
        1249990, 1239990, 1219990, 1199990, 1189990, 1179990,
        1159990, 1149990, 1129990, 1099990, 1079990, 1049990
    ]::NUMERIC[]) WITH ORDINALITY AS punto(precio, pos)
    ON CONFLICT (product_offer_id, price, recorded_at) DO NOTHING;

    INSERT INTO product_price_history (product_offer_id, price, is_promotional, recorded_at)
    SELECT
        v_offer_ripley,
        punto.precio,
        (13 - punto.pos) IN (7, 2),
        date_trunc('month', CURRENT_DATE)
            - make_interval(months => (13 - punto.pos)::INTEGER)
            + INTERVAL '9 hours'
    FROM unnest(ARRAY[
        1229990, 1219990, 1199990, 1179990, 1169990, 1149990,
        1139990, 1119990, 1099990, 1079990, 1069990, 1059990
    ]::NUMERIC[]) WITH ORDINALITY AS punto(precio, pos)
    ON CONFLICT (product_offer_id, price, recorded_at) DO NOTHING;

    INSERT INTO product_price_history (product_offer_id, price, is_promotional, recorded_at)
    SELECT
        v_offer_lider,
        punto.precio,
        (13 - punto.pos) IN (7, 2),
        date_trunc('month', CURRENT_DATE)
            - make_interval(months => (13 - punto.pos)::INTEGER)
            + INTERVAL '9 hours'
    FROM unnest(ARRAY[
        1199990, 1189990, 1169990, 1149990, 1139990, 1119990,
        1099990, 1079990, 1059990, 1049990, 1039990, 1029990
    ]::NUMERIC[]) WITH ORDINALITY AS punto(precio, pos)
    ON CONFLICT (product_offer_id, price, recorded_at) DO NOTHING;

    -- 8. Atributos de la categoría (013_categories_seed.sql), con la forma
    --    canónica de categories.ValidateAttributes: "570", "98.5", "true".
    INSERT INTO product_attribute_value (product_id, category_id, attribute_id, value, value_numeric)
    SELECT v_product_id, v_category_ref, atributo.id, valor.texto, valor.numero
    FROM (VALUES
        ('capacidad_litros',       '570',        570::NUMERIC),
        ('eficiencia_energetica',  'A++',        NULL::NUMERIC),
        ('no_frost',               'true',       NULL::NUMERIC),
        ('peso_kg',                '98.5',       98.5::NUMERIC),
        ('color',                  'Inoxidable', NULL::NUMERIC)
    ) AS valor(key, texto, numero)
    JOIN category_attributes atributo
      ON atributo.category_id = v_category_ref
     AND atributo.key = valor.key
    ON CONFLICT (product_id, attribute_id) DO UPDATE
    SET value         = EXCLUDED.value,
        value_numeric = EXCLUDED.value_numeric;

    -- 9. Validación final: si algo no cuadra, se revierte toda la migración.
    SELECT active, category_ref, public_id
    INTO v_activo, v_category_ref, v_public_id
    FROM product
    WHERE id = v_product_id;

    IF NOT v_activo OR v_category_ref IS DISTINCT FROM (SELECT id FROM categories WHERE slug = 'refrigeradores' AND item_type = 'product') THEN
        RAISE EXCEPTION 'seed 048: el producto no quedó activo en la categoría refrigeradores';
    END IF;

    SELECT count(*) INTO v_imagenes
    FROM product_image
    WHERE product_id = v_product_id AND active;

    SELECT count(*) INTO v_ofertas
    FROM product_offer
    WHERE product_id = v_product_id;

    SELECT count(*) INTO v_ofertas_ok
    FROM product_offer
    WHERE product_id = v_product_id
      AND (
            (store_id = v_store_falabella AND price = 999990  AND list_price = 1199990)
         OR (store_id = v_store_ripley    AND price = 1049990 AND list_price = 1199990)
         OR (store_id = v_store_lider     AND price = 1029990 AND list_price = 1149990)
      );

    IF EXISTS (
        SELECT 1 FROM product_offer
        WHERE product_id = v_product_id AND price > list_price
    ) THEN
        RAISE EXCEPTION 'seed 048: una oferta tiene precio mayor que el de lista';
    END IF;

    SELECT count(*) INTO v_atributos
    FROM product_attribute_value
    WHERE product_id = v_product_id;

    SELECT count(*), min(price), max(price)
    INTO v_historial, v_precio_min, v_precio_max
    FROM product_price_history
    WHERE product_offer_id IN (v_offer_falabella, v_offer_ripley, v_offer_lider);

    SELECT derived_review_count, derived_average_rating
    INTO v_resenas, v_rating
    FROM product_rating_summary
    WHERE product_id = v_product_id;

    IF v_imagenes <> 3 THEN
        RAISE EXCEPTION 'seed 048: se esperaban 3 imágenes y hay %', v_imagenes;
    END IF;
    IF v_ofertas <> 3 OR v_ofertas_ok <> 3 THEN
        RAISE EXCEPTION 'seed 048: ofertas=% y coincidencias de precio=% (se esperaban 3 y 3)', v_ofertas, v_ofertas_ok;
    END IF;
    IF v_atributos <> 5 THEN
        RAISE EXCEPTION 'seed 048: se esperaban 5 atributos y hay %', v_atributos;
    END IF;
    IF v_historial < 39 THEN
        RAISE EXCEPTION 'seed 048: se esperaban al menos 39 puntos de historial y hay %', v_historial;
    END IF;
    IF v_precio_min <> 999990 OR v_precio_max <> 1249990 THEN
        RAISE EXCEPTION 'seed 048: historial fuera de rango (min=%, max=%)', v_precio_min, v_precio_max;
    END IF;

    RAISE NOTICE 'seed 048: sku=% id=% public_id=% imagenes=% ofertas=% atributos=% historial=% precio_min=% precio_max=% resenas=% rating=%',
        sku_producto, v_product_id, v_public_id, v_imagenes, v_ofertas, v_atributos,
        v_historial, v_precio_min, v_precio_max, v_resenas, v_rating;
END;
$$;

COMMIT;
