-- =====================================================================
-- SOLOServis - Catalogo adicional de telefonos celulares
-- Ejecutar despues de 001_schema.sql, 002_views.sql y 003_seed.sql.
-- =====================================================================

BEGIN;

-- MARCAS
INSERT INTO brand (name, logo_url, website_url)
VALUES
    ('Samsung', 'https://cdn.simpleicons.org/samsung', 'https://www.samsung.com/cl/'),
    ('Apple', 'https://cdn.simpleicons.org/apple', 'https://www.apple.com/cl/'),
    ('Xiaomi', 'https://cdn.simpleicons.org/xiaomi', 'https://www.mi.com/cl/'),
    ('Motorola', 'https://cdn.simpleicons.org/motorola', 'https://www.motorola.cl/'),
    ('Huawei', 'https://cdn.simpleicons.org/huawei', 'https://consumer.huawei.com/cl/'),
    ('HONOR', 'https://cdn.simpleicons.org/honor', 'https://www.honor.com/cl/')
ON CONFLICT (name) DO UPDATE
SET logo_url = EXCLUDED.logo_url,
    website_url = EXCLUDED.website_url;

-- CATEGORIAS
INSERT INTO product_category (parent_category_id, name, description)
SELECT NULL, 'Celulares', 'Telefonos celulares y smartphones'
WHERE NOT EXISTS (
    SELECT 1
    FROM product_category
    WHERE parent_category_id IS NULL AND name = 'Celulares'
);

INSERT INTO product_category (parent_category_id, name, description)
SELECT root.id, 'Smartphones', 'Telefonos inteligentes de distintas marcas y gamas'
FROM product_category AS root
WHERE root.parent_category_id IS NULL
  AND root.name = 'Celulares'
  AND NOT EXISTS (
      SELECT 1
      FROM product_category AS existing
      WHERE existing.parent_category_id = root.id
        AND existing.name = 'Smartphones'
  );

-- PRODUCTOS
INSERT INTO product (category_id, brand_id, name, model, sku, description)
SELECT category.id, brand.id, items.name, items.model, items.sku, items.description
FROM (VALUES
    ('SAMSUNG-GALAXY-A55-256', 'Samsung', 'Samsung Galaxy A55 5G 256 GB', 'Galaxy A55 5G', 'SM-A556E', 'Smartphone 5G de gama media con pantalla Super AMOLED y almacenamiento de 256 GB.'),
    ('SAMSUNG-GALAXY-S24FE-256', 'Samsung', 'Samsung Galaxy S24 FE 256 GB', 'Galaxy S24 FE', 'SM-S721B', 'Smartphone de gama alta con pantalla AMOLED, camaras avanzadas y conectividad 5G.'),
    ('APPLE-IPHONE-15-128', 'Apple', 'Apple iPhone 15 128 GB', 'iPhone 15', 'A3090', 'Smartphone Apple con pantalla OLED, camara principal de 48 MP y almacenamiento de 128 GB.'),
    ('APPLE-IPHONE-16-128', 'Apple', 'Apple iPhone 16 128 GB', 'iPhone 16', 'A3287', 'Smartphone Apple con chip de nueva generacion, pantalla OLED y almacenamiento de 128 GB.'),
    ('XIAOMI-REDMI-NOTE-14-PRO-256', 'Xiaomi', 'Xiaomi Redmi Note 14 Pro 5G 256 GB', 'Redmi Note 14 Pro 5G', 'Redmi Note 14 Pro 5G', 'Smartphone 5G con pantalla AMOLED, camara de alta resolucion y almacenamiento de 256 GB.'),
    ('XIAOMI-14T-512', 'Xiaomi', 'Xiaomi 14T 512 GB', 'Xiaomi 14T', 'Xiaomi 14T', 'Smartphone de gama alta con pantalla AMOLED, sistema de camaras avanzado y 512 GB.'),
    ('MOTOROLA-MOTO-G85-256', 'Motorola', 'Motorola Moto G85 5G 256 GB', 'Moto G85 5G', 'XT2427', 'Smartphone 5G con pantalla pOLED, bateria de larga duracion y 256 GB de almacenamiento.'),
    ('MOTOROLA-EDGE-50-FUSION-256', 'Motorola', 'Motorola Edge 50 Fusion 256 GB', 'Edge 50 Fusion', 'XT2429', 'Smartphone con pantalla pOLED, carga rapida y camara principal con estabilizacion optica.'),
    ('HUAWEI-NOVA-12I-256', 'Huawei', 'Huawei nova 12i 256 GB', 'nova 12i', 'FOA-LX9', 'Smartphone con pantalla amplia, camara principal de alta resolucion y 256 GB.'),
    ('HONOR-MAGIC6-LITE-256', 'HONOR', 'HONOR Magic6 Lite 5G 256 GB', 'Magic6 Lite 5G', 'ALI-NX1', 'Smartphone 5G con pantalla AMOLED curva, bateria de gran capacidad y 256 GB.')
) AS items(sku, brand_name, name, model, model_code, description)
JOIN brand ON brand.name = items.brand_name
JOIN product_category AS category ON category.name = 'Smartphones'
JOIN product_category AS root
  ON root.id = category.parent_category_id
 AND root.name = 'Celulares'
ON CONFLICT (sku) DO NOTHING;

-- OFERTAS EN FALABELLA Y PARIS (precios de referencia en CLP)
INSERT INTO product_offer (
    product_id, store_id, price, list_price, currency, shipping_cost,
    shipping_free, available, stock, condition, product_url
)
SELECT product.id, store.id, offers.price, offers.list_price, 'CLP',
       offers.shipping_cost, offers.shipping_cost = 0, TRUE, offers.stock,
       'new', offers.product_url
FROM (VALUES
    ('SAMSUNG-GALAXY-A55-256', 'Falabella', 329990, 399990, 0, 12, 'https://falabella.com/producto/samsung-galaxy-a55-256'),
    ('SAMSUNG-GALAXY-A55-256', 'Paris', 349990, 399990, 3990, 8, 'https://paris.cl/producto/samsung-galaxy-a55-256'),
    ('SAMSUNG-GALAXY-S24FE-256', 'Falabella', 599990, 699990, 0, 7, 'https://falabella.com/producto/samsung-galaxy-s24fe-256'),
    ('SAMSUNG-GALAXY-S24FE-256', 'Paris', 629990, 699990, 3990, 5, 'https://paris.cl/producto/samsung-galaxy-s24fe-256'),
    ('APPLE-IPHONE-15-128', 'Falabella', 699990, 799990, 0, 9, 'https://falabella.com/producto/apple-iphone-15-128'),
    ('APPLE-IPHONE-15-128', 'Paris', 729990, 799990, 3990, 6, 'https://paris.cl/producto/apple-iphone-15-128'),
    ('APPLE-IPHONE-16-128', 'Falabella', 899990, 999990, 0, 6, 'https://falabella.com/producto/apple-iphone-16-128'),
    ('APPLE-IPHONE-16-128', 'Paris', 929990, 999990, 3990, 4, 'https://paris.cl/producto/apple-iphone-16-128'),
    ('XIAOMI-REDMI-NOTE-14-PRO-256', 'Falabella', 279990, 329990, 0, 14, 'https://falabella.com/producto/xiaomi-redmi-note-14-pro-256'),
    ('XIAOMI-REDMI-NOTE-14-PRO-256', 'Paris', 299990, 329990, 3990, 10, 'https://paris.cl/producto/xiaomi-redmi-note-14-pro-256'),
    ('XIAOMI-14T-512', 'Falabella', 499990, 599990, 0, 8, 'https://falabella.com/producto/xiaomi-14t-512'),
    ('XIAOMI-14T-512', 'Paris', 529990, 599990, 3990, 5, 'https://paris.cl/producto/xiaomi-14t-512'),
    ('MOTOROLA-MOTO-G85-256', 'Falabella', 219990, 279990, 0, 15, 'https://falabella.com/producto/motorola-moto-g85-256'),
    ('MOTOROLA-MOTO-G85-256', 'Paris', 239990, 279990, 3990, 10, 'https://paris.cl/producto/motorola-moto-g85-256'),
    ('MOTOROLA-EDGE-50-FUSION-256', 'Falabella', 299990, 349990, 0, 10, 'https://falabella.com/producto/motorola-edge-50-fusion-256'),
    ('MOTOROLA-EDGE-50-FUSION-256', 'Paris', 319990, 349990, 3990, 7, 'https://paris.cl/producto/motorola-edge-50-fusion-256'),
    ('HUAWEI-NOVA-12I-256', 'Falabella', 199990, 249990, 0, 12, 'https://falabella.com/producto/huawei-nova-12i-256'),
    ('HUAWEI-NOVA-12I-256', 'Paris', 219990, 249990, 3990, 8, 'https://paris.cl/producto/huawei-nova-12i-256'),
    ('HONOR-MAGIC6-LITE-256', 'Falabella', 249990, 299990, 0, 11, 'https://falabella.com/producto/honor-magic6-lite-256'),
    ('HONOR-MAGIC6-LITE-256', 'Paris', 269990, 299990, 3990, 7, 'https://paris.cl/producto/honor-magic6-lite-256')
) AS offers(sku, store_name, price, list_price, shipping_cost, stock, product_url)
JOIN product ON product.sku = offers.sku
JOIN store ON store.name = offers.store_name
ON CONFLICT (product_id, store_id) DO UPDATE
SET price = EXCLUDED.price,
    list_price = EXCLUDED.list_price,
    currency = EXCLUDED.currency,
    shipping_cost = EXCLUDED.shipping_cost,
    shipping_free = EXCLUDED.shipping_free,
    available = EXCLUDED.available,
    stock = EXCLUDED.stock,
    condition = EXCLUDED.condition,
    product_url = EXCLUDED.product_url,
    last_updated = NOW();

-- HISTORIAL DE PRECIOS: precio anterior y precio promocional de Falabella.
INSERT INTO product_price_history (product_offer_id, price, is_promotional, recorded_at)
SELECT offer.id, history.price, history.is_promotional,
       NOW() - (history.days_ago || ' days')::INTERVAL
FROM (VALUES
    ('SAMSUNG-GALAXY-A55-256', 399990, FALSE, 30), ('SAMSUNG-GALAXY-A55-256', 329990, TRUE, 3),
    ('SAMSUNG-GALAXY-S24FE-256', 699990, FALSE, 25), ('SAMSUNG-GALAXY-S24FE-256', 599990, TRUE, 2),
    ('APPLE-IPHONE-15-128', 799990, FALSE, 28), ('APPLE-IPHONE-15-128', 699990, TRUE, 4),
    ('APPLE-IPHONE-16-128', 999990, FALSE, 22), ('APPLE-IPHONE-16-128', 899990, TRUE, 3),
    ('XIAOMI-REDMI-NOTE-14-PRO-256', 329990, FALSE, 20), ('XIAOMI-REDMI-NOTE-14-PRO-256', 279990, TRUE, 2),
    ('XIAOMI-14T-512', 599990, FALSE, 18), ('XIAOMI-14T-512', 499990, TRUE, 3),
    ('MOTOROLA-MOTO-G85-256', 279990, FALSE, 21), ('MOTOROLA-MOTO-G85-256', 219990, TRUE, 3),
    ('MOTOROLA-EDGE-50-FUSION-256', 349990, FALSE, 19), ('MOTOROLA-EDGE-50-FUSION-256', 299990, TRUE, 2),
    ('HUAWEI-NOVA-12I-256', 249990, FALSE, 17), ('HUAWEI-NOVA-12I-256', 199990, TRUE, 3),
    ('HONOR-MAGIC6-LITE-256', 299990, FALSE, 16), ('HONOR-MAGIC6-LITE-256', 249990, TRUE, 2)
) AS history(sku, price, is_promotional, days_ago)
JOIN product ON product.sku = history.sku
JOIN product_offer AS offer ON offer.product_id = product.id
JOIN store ON store.id = offer.store_id AND store.name = 'Falabella'
WHERE NOT EXISTS (
    SELECT 1
    FROM product_price_history AS existing
    WHERE existing.product_offer_id = offer.id
      AND existing.price = history.price
      AND existing.is_promotional = history.is_promotional
      AND existing.recorded_at::DATE = CURRENT_DATE - history.days_ago
);

COMMIT;