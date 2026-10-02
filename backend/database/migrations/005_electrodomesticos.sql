-- =====================================================================
-- SOLOServis - Catalogo adicional de electrodomesticos
-- Ejecutar despues de 001_schema.sql, 002_views.sql y 003_seed.sql.
-- =====================================================================

BEGIN;

-- MARCAS
INSERT INTO brand (name, logo_url, website_url)
VALUES
    ('Samsung', 'https://cdn.simpleicons.org/samsung', 'https://www.samsung.com/cl/'),
    ('LG', 'https://cdn.simpleicons.org/lg', 'https://www.lg.com/cl/'),
    ('Whirlpool', 'https://cdn.simpleicons.org/whirlpool', 'https://www.whirlpool.cl/'),
    ('Bosch', 'https://cdn.simpleicons.org/bosch', 'https://www.bosch-home.cl/'),
    ('Electrolux', 'https://cdn.simpleicons.org/electrolux', 'https://www.electrolux.cl/'),
    ('Midea', 'https://cdn.simpleicons.org/midea', 'https://www.midea.com/cl/')
ON CONFLICT (name) DO UPDATE
SET logo_url = EXCLUDED.logo_url,
    website_url = EXCLUDED.website_url;

-- CATEGORIAS
INSERT INTO product_category (parent_category_id, name, description)
SELECT NULL, 'Electrodomésticos', 'Electrodomésticos para el hogar'
WHERE NOT EXISTS (
    SELECT 1
    FROM product_category
    WHERE parent_category_id IS NULL AND name = 'Electrodomésticos'
);

INSERT INTO product_category (parent_category_id, name, description)
SELECT root.id, categories.name, categories.description
FROM product_category AS root
CROSS JOIN (VALUES
    ('Refrigeradores', 'Refrigeradores y congeladores para el hogar'),
    ('Lavadoras', 'Lavadoras de ropa'),
    ('Microondas', 'Hornos microondas'),
    ('Lavavajillas', 'Lavavajillas para el hogar'),
    ('Aspiradoras', 'Aspiradoras y equipos de limpieza'),
    ('Freidoras de aire', 'Freidoras de aire para cocina')
) AS categories(name, description)
WHERE root.parent_category_id IS NULL
  AND root.name = 'Electrodomésticos'
  AND NOT EXISTS (
      SELECT 1
      FROM product_category AS existing
      WHERE existing.parent_category_id = root.id
        AND existing.name = categories.name
  );

-- PRODUCTOS
INSERT INTO product (category_id, brand_id, name, model, sku, description)
SELECT category.id, brand.id, items.name, items.model, items.sku, items.description
FROM (VALUES
    ('SAMSUNG-REFRI-RT38K', 'Samsung', 'Refrigeradores', 'Refrigerador Samsung No Frost 384 L', 'RT38K5930S8', 'Refrigerador No Frost de 384 litros con dispensador de agua y eficiencia para uso familiar.'),
    ('LG-REFRI-INSTAVIEW-GS66', 'LG', 'Refrigeradores', 'Refrigerador LG InstaView Side by Side', 'GS66', 'Refrigerador Side by Side de gran capacidad con tecnologia InstaView y dispensador.'),
    ('WHIRLPOOL-LAVADORA-WTW4816', 'Whirlpool', 'Lavadoras', 'Lavadora Whirlpool carga superior 17 kg', 'WTW4816FW', 'Lavadora de carga superior con capacidad para cargas grandes y ciclos de lavado multiples.'),
    ('LG-LAVADORA-F4V5RYP2T', 'LG', 'Lavadoras', 'Lavadora LG carga frontal 10.5 kg', 'F4V5RYP2T', 'Lavadora de carga frontal con motor inverter y capacidad de 10.5 kg.'),
    ('SAMSUNG-MICROONDAS-MS23K', 'Samsung', 'Microondas', 'Microondas Samsung 23 L', 'MS23K3513AK', 'Microondas de 23 litros con modos de coccion y descongelado.'),
    ('MIDEA-MICROONDAS-MMW20', 'Midea', 'Microondas', 'Microondas Midea 20 L', 'MMW20S', 'Microondas compacto de 20 litros para calentar y descongelar alimentos.'),
    ('BOSCH-LAVAVAJILLAS-SMS4H', 'Bosch', 'Lavavajillas', 'Lavavajillas Bosch Serie 4', 'SMS4HMI00G', 'Lavavajillas independiente con programas automaticos y capacidad familiar.'),
    ('ELECTROLUX-ASPIRADORA-EAS30', 'Electrolux', 'Aspiradoras', 'Aspiradora Electrolux Easybox', 'EAS30', 'Aspiradora de trineo para limpieza diaria de pisos y tapiceria.'),
    ('MIDEA-FREIDORA-AIRE-MF40', 'Midea', 'Freidoras de aire', 'Freidora de aire Midea 4 L', 'MF-CY40A', 'Freidora de aire de 4 litros para preparar alimentos con poco aceite.')
) AS items(sku, brand_name, category_name, name, model, description)
JOIN brand ON brand.name = items.brand_name
JOIN product_category AS category ON category.name = items.category_name
JOIN product_category AS root
  ON root.id = category.parent_category_id
 AND root.name = 'Electrodomésticos'
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
    ('SAMSUNG-REFRI-RT38K', 'Falabella', 749990, 899990, 0, 8, 'https://falabella.com/producto/samsung-refri-rt38k'),
    ('SAMSUNG-REFRI-RT38K', 'Paris', 779990, 899990, 5990, 5, 'https://paris.cl/producto/samsung-refri-rt38k'),
    ('LG-REFRI-INSTAVIEW-GS66', 'Falabella', 1499990, 1699990, 0, 4, 'https://falabella.com/producto/lg-refri-instaview-gs66'),
    ('LG-REFRI-INSTAVIEW-GS66', 'Paris', 1549990, 1699990, 7990, 3, 'https://paris.cl/producto/lg-refri-instaview-gs66'),
    ('WHIRLPOOL-LAVADORA-WTW4816', 'Falabella', 449990, 549990, 0, 10, 'https://falabella.com/producto/whirlpool-lavadora-wtw4816'),
    ('WHIRLPOOL-LAVADORA-WTW4816', 'Paris', 479990, 549990, 4990, 6, 'https://paris.cl/producto/whirlpool-lavadora-wtw4816'),
    ('LG-LAVADORA-F4V5RYP2T', 'Falabella', 529990, 649990, 0, 7, 'https://falabella.com/producto/lg-lavadora-f4v5ryp2t'),
    ('LG-LAVADORA-F4V5RYP2T', 'Paris', 559990, 649990, 4990, 5, 'https://paris.cl/producto/lg-lavadora-f4v5ryp2t'),
    ('SAMSUNG-MICROONDAS-MS23K', 'Falabella', 89990, 119990, 0, 20, 'https://falabella.com/producto/samsung-microondas-ms23k'),
    ('SAMSUNG-MICROONDAS-MS23K', 'Paris', 94990, 119990, 3990, 14, 'https://paris.cl/producto/samsung-microondas-ms23k'),
    ('MIDEA-MICROONDAS-MMW20', 'Falabella', 59990, 79990, 0, 24, 'https://falabella.com/producto/midea-microondas-mmw20'),
    ('MIDEA-MICROONDAS-MMW20', 'Paris', 64990, 79990, 3990, 18, 'https://paris.cl/producto/midea-microondas-mmw20'),
    ('BOSCH-LAVAVAJILLAS-SMS4H', 'Falabella', 699990, 849990, 0, 5, 'https://falabella.com/producto/bosch-lavavajillas-sms4h'),
    ('BOSCH-LAVAVAJILLAS-SMS4H', 'Paris', 729990, 849990, 5990, 3, 'https://paris.cl/producto/bosch-lavavajillas-sms4h'),
    ('ELECTROLUX-ASPIRADORA-EAS30', 'Falabella', 89990, 119990, 0, 12, 'https://falabella.com/producto/electrolux-aspiradora-eas30'),
    ('ELECTROLUX-ASPIRADORA-EAS30', 'Paris', 94990, 119990, 3990, 8, 'https://paris.cl/producto/electrolux-aspiradora-eas30'),
    ('MIDEA-FREIDORA-AIRE-MF40', 'Falabella', 69990, 99990, 0, 16, 'https://falabella.com/producto/midea-freidora-aire-mf40'),
    ('MIDEA-FREIDORA-AIRE-MF40', 'Paris', 74990, 99990, 3990, 11, 'https://paris.cl/producto/midea-freidora-aire-mf40')
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
    ('SAMSUNG-REFRI-RT38K', 899990, FALSE, 30), ('SAMSUNG-REFRI-RT38K', 749990, TRUE, 3),
    ('LG-REFRI-INSTAVIEW-GS66', 1699990, FALSE, 25), ('LG-REFRI-INSTAVIEW-GS66', 1499990, TRUE, 2),
    ('WHIRLPOOL-LAVADORA-WTW4816', 549990, FALSE, 28), ('WHIRLPOOL-LAVADORA-WTW4816', 449990, TRUE, 4),
    ('LG-LAVADORA-F4V5RYP2T', 649990, FALSE, 22), ('LG-LAVADORA-F4V5RYP2T', 529990, TRUE, 3),
    ('SAMSUNG-MICROONDAS-MS23K', 119990, FALSE, 20), ('SAMSUNG-MICROONDAS-MS23K', 89990, TRUE, 2),
    ('MIDEA-MICROONDAS-MMW20', 79990, FALSE, 18), ('MIDEA-MICROONDAS-MMW20', 59990, TRUE, 3),
    ('BOSCH-LAVAVAJILLAS-SMS4H', 849990, FALSE, 26), ('BOSCH-LAVAVAJILLAS-SMS4H', 699990, TRUE, 4),
    ('ELECTROLUX-ASPIRADORA-EAS30', 119990, FALSE, 21), ('ELECTROLUX-ASPIRADORA-EAS30', 89990, TRUE, 3),
    ('MIDEA-FREIDORA-AIRE-MF40', 99990, FALSE, 16), ('MIDEA-FREIDORA-AIRE-MF40', 69990, TRUE, 2)
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