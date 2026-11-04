BEGIN;

INSERT INTO brand (name, logo_url, website_url) VALUES
    ('Apple', 'https://placehold.co/128x128?text=Apple', 'https://www.apple.com'),
    ('ASUS', 'https://placehold.co/128x128?text=ASUS', 'https://www.asus.com'),
    ('Bosch', 'https://placehold.co/128x128?text=Bosch', 'https://www.bosch-home.com'),
    ('Dell', 'https://placehold.co/128x128?text=Dell', 'https://www.dell.com'),
    ('HP', 'https://placehold.co/128x128?text=HP', 'https://www.hp.com'),
    ('JBL', 'https://placehold.co/128x128?text=JBL', 'https://www.jbl.com'),
    ('Lenovo', 'https://placehold.co/128x128?text=Lenovo', 'https://www.lenovo.com'),
    ('Midea', 'https://placehold.co/128x128?text=Midea', 'https://www.midea.com'),
    ('Motorola', 'https://placehold.co/128x128?text=Motorola', 'https://www.motorola.com'),
    ('Microsoft', 'https://placehold.co/128x128?text=Microsoft', 'https://www.microsoft.com'),
    ('Nintendo', 'https://placehold.co/128x128?text=Nintendo', 'https://www.nintendo.com'),
    ('Philips', 'https://placehold.co/128x128?text=Philips', 'https://www.philips.com'),
    ('Sony', 'https://placehold.co/128x128?text=Sony', 'https://www.sony.com'),
    ('TCL', 'https://placehold.co/128x128?text=TCL', 'https://www.tcl.com'),
    ('Xiaomi', 'https://placehold.co/128x128?text=Xiaomi', 'https://www.mi.com')
ON CONFLICT (name) DO NOTHING;

INSERT INTO product_category (name, description)
SELECT category.name, category.description
FROM (VALUES
    ('Refrigeracion', 'Refrigeradores y equipos de frio'),
    ('Lavado', 'Lavadoras y secadoras'),
    ('Telefonia', 'Telefonos celulares y accesorios'),
    ('Computacion', 'Notebooks y computadores'),
    ('Televisores', 'Televisores y pantallas'),
    ('Audio', 'Audifonos y parlantes'),
    ('Videojuegos', 'Consolas y equipos de videojuegos'),
    ('Hogar', 'Pequenos electrodomesticos y limpieza')
) AS category(name, description)
WHERE NOT EXISTS (
    SELECT 1
    FROM product_category existing
    WHERE LOWER(existing.name) = LOWER(category.name)
);

INSERT INTO product_category_specification
    (category_id, name, data_type, unit, required, comparable, display_order)
SELECT category.id, specification.name, specification.data_type,
       specification.unit, FALSE, TRUE, specification.display_order
FROM (VALUES
    ('Refrigeracion', 'Capacidad', 'number', 'L', 1),
    ('Refrigeracion', 'Eficiencia energetica', 'string', NULL, 2),
    ('Lavado', 'Capacidad', 'number', 'kg', 1),
    ('Lavado', 'Velocidad de centrifugado', 'number', 'rpm', 2),
    ('Telefonia', 'Almacenamiento', 'number', 'GB', 1),
    ('Telefonia', 'Memoria RAM', 'number', 'GB', 2),
    ('Telefonia', 'Pantalla', 'number', 'in', 3),
    ('Computacion', 'Almacenamiento', 'number', 'GB', 1),
    ('Computacion', 'Memoria RAM', 'number', 'GB', 2),
    ('Computacion', 'Pantalla', 'number', 'in', 3),
    ('Televisores', 'Pantalla', 'number', 'in', 1),
    ('Televisores', 'Resolucion', 'string', NULL, 2),
    ('Audio', 'Tipo', 'string', NULL, 1),
    ('Audio', 'Autonomia', 'number', 'h', 2),
    ('Videojuegos', 'Almacenamiento', 'number', 'GB', 1),
    ('Videojuegos', 'Plataforma', 'string', NULL, 2),
    ('Hogar', 'Potencia', 'number', 'W', 1),
    ('Hogar', 'Tipo', 'string', NULL, 2)
) AS specification(category_name, name, data_type, unit, display_order)
JOIN product_category category
    ON category.name = specification.category_name
WHERE NOT EXISTS (
    SELECT 1
    FROM product_category_specification existing
    WHERE existing.category_id = category.id
      AND existing.name = specification.name
);

INSERT INTO store
    (name, website_url, logo_url, rating, reputation, shipping_information, general_conditions)
SELECT store.name, store.website_url, store.logo_url, store.rating,
       store.reputation, store.shipping_information, store.general_conditions
FROM (VALUES
    ('Ripley', 'https://simple.ripley.cl', 'https://placehold.co/128x128?text=Ripley', 4.10, 'Buena', 'Despacho estimado de 2 a 6 dias habiles', 'Garantia legal y del fabricante'),
    ('Mercado Libre', 'https://www.mercadolibre.cl', 'https://placehold.co/128x128?text=Mercado+Libre', 4.30, 'Muy buena', 'Despacho segun vendedor y ubicacion', 'Proteccion de compra segun publicacion'),
    ('Lider', 'https://www.lider.cl', 'https://placehold.co/128x128?text=Lider', 3.90, 'Buena', 'Despacho estimado de 2 a 5 dias habiles', 'Garantia legal y del fabricante'),
    ('Sodimac', 'https://www.sodimac.cl', 'https://placehold.co/128x128?text=Sodimac', 4.00, 'Buena', 'Despacho estimado de 2 a 7 dias habiles', 'Garantia legal y del fabricante'),
    ('PC Factory', 'https://www.pcfactory.cl', 'https://placehold.co/128x128?text=PC+Factory', 4.20, 'Muy buena', 'Despacho estimado de 1 a 4 dias habiles', 'Garantia legal y del fabricante')
) AS store(name, website_url, logo_url, rating, reputation, shipping_information, general_conditions)
WHERE NOT EXISTS (
    SELECT 1 FROM store existing WHERE LOWER(existing.name) = LOWER(store.name)
);

INSERT INTO product (category_id, brand_id, name, model, sku, description)
SELECT category.id, brand.id, product.name, product.model, product.sku, product.description
FROM (VALUES
    ('Refrigeracion', 'Midea', 'Refrigerador Midea No Frost 400 L', 'MDRT489', 'DEMO-MIDEA-FRIDGE-400', 'Refrigerador familiar con tecnologia No Frost y dispensador interior.'),
    ('Refrigeracion', 'LG', 'Refrigerador LG French Door 530 L', 'GM53', 'DEMO-LG-FRIDGE-530', 'Refrigerador French Door de gran capacidad y control digital.'),
    ('Lavado', 'Samsung', 'Lavadora Samsung EcoBubble 11 kg', 'WW11T', 'DEMO-SAMSUNG-WASHER-11', 'Lavadora frontal con programas de lavado rapido y ahorro de energia.'),
    ('Lavado', 'Bosch', 'Lavadora Bosch Serie 4 9 kg', 'WAN242', 'DEMO-BOSCH-WASHER-9', 'Lavadora frontal con multiples programas y centrifugado variable.'),
    ('Telefonia', 'Apple', 'Apple iPhone 15 128 GB', 'iPhone 15', 'DEMO-APPLE-IPHONE15-128', 'Telefono desbloqueado con pantalla OLED y almacenamiento de 128 GB.'),
    ('Telefonia', 'Samsung', 'Samsung Galaxy S24 256 GB', 'Galaxy S24', 'DEMO-SAMSUNG-S24-256', 'Telefono Android con pantalla AMOLED y 256 GB de almacenamiento.'),
    ('Telefonia', 'Xiaomi', 'Xiaomi Redmi Note 13 256 GB', 'Redmi Note 13', 'DEMO-XIAOMI-REDMI13-256', 'Telefono de gama media con pantalla AMOLED y bateria de larga duracion.'),
    ('Telefonia', 'Motorola', 'Motorola Edge 50 Fusion 256 GB', 'Edge 50 Fusion', 'DEMO-MOTO-EDGE50-256', 'Telefono Android con camara de alta resolucion y carga rapida.'),
    ('Computacion', 'Lenovo', 'Notebook Lenovo IdeaPad Slim 5', 'IdeaPad Slim 5', 'DEMO-LENOVO-IDEAPAD-16', 'Notebook para productividad con 16 GB de RAM y almacenamiento SSD.'),
    ('Computacion', 'HP', 'Notebook HP Pavilion 15', 'Pavilion 15', 'DEMO-HP-PAVILION-15', 'Notebook multiproposito con pantalla Full HD y unidad SSD.'),
    ('Computacion', 'ASUS', 'Notebook ASUS TUF Gaming A15', 'TUF A15', 'DEMO-ASUS-TUF-A15', 'Notebook para juegos con graficos dedicados y pantalla de alta frecuencia.'),
    ('Televisores', 'Sony', 'Televisor Sony Bravia 55 pulgadas 4K', 'Bravia X80L', 'DEMO-SONY-TV-55-4K', 'Televisor LED 4K con aplicaciones de streaming integradas.'),
    ('Televisores', 'LG', 'Televisor LG OLED 65 pulgadas 4K', 'OLED C3', 'DEMO-LG-TV-OLED-65', 'Televisor OLED 4K con alta tasa de refresco y funciones inteligentes.'),
    ('Televisores', 'TCL', 'Televisor TCL QLED 50 pulgadas 4K', 'C645', 'DEMO-TCL-TV-QLED-50', 'Televisor QLED 4K con Google TV y control por voz.'),
    ('Audio', 'Sony', 'Audifonos Sony WH-1000XM5', 'WH-1000XM5', 'DEMO-SONY-HEADPHONES-XM5', 'Audifonos inalambricos con cancelacion activa de ruido.'),
    ('Audio', 'JBL', 'Parlante JBL Flip 6', 'Flip 6', 'DEMO-JBL-FLIP6', 'Parlante Bluetooth portatil resistente al agua.'),
    ('Audio', 'Apple', 'Apple AirPods Pro segunda generacion', 'AirPods Pro 2', 'DEMO-APPLE-AIRPODS-PRO2', 'Audifonos inalambricos con cancelacion activa de ruido.'),
    ('Videojuegos', 'Sony', 'Consola Sony PlayStation 5 Slim', 'PS5 Slim', 'DEMO-SONY-PS5-SLIM', 'Consola de videojuegos con unidad SSD de alta velocidad.'),
    ('Videojuegos', 'Nintendo', 'Consola Nintendo Switch OLED', 'Switch OLED', 'DEMO-NINTENDO-SWITCH-OLED', 'Consola hibrida con pantalla OLED y controles desmontables.'),
    ('Videojuegos', 'Microsoft', 'Consola Xbox Series X 1 TB', 'Xbox Series X', 'DEMO-XBOX-SERIES-X-1TB', 'Consola de videojuegos con almacenamiento SSD de 1 TB.')
) AS product(category_name, brand_name, name, model, sku, description)
JOIN product_category category ON category.name = product.category_name
JOIN brand ON brand.name = product.brand_name
ON CONFLICT (sku) DO UPDATE
SET category_id = EXCLUDED.category_id,
    brand_id = EXCLUDED.brand_id,
    name = EXCLUDED.name,
    model = EXCLUDED.model,
    description = EXCLUDED.description;

INSERT INTO product_image (product_id, image_url, alt_text, sort_order)
SELECT product.id,
       'https://placehold.co/640x640?text=' || product.sku,
       product.name,
       0
FROM product
WHERE product.sku LIKE 'DEMO-%'
  AND NOT EXISTS (
      SELECT 1 FROM product_image image
      WHERE image.product_id = product.id
        AND image.image_url = 'https://placehold.co/640x640?text=' || product.sku
  );

INSERT INTO product_specification_value (product_id, specification_id, value)
SELECT product.id, specification.id, value.value
FROM (VALUES
    ('DEMO-MIDEA-FRIDGE-400', 'Capacidad', '400'),
    ('DEMO-MIDEA-FRIDGE-400', 'Eficiencia energetica', 'A++'),
    ('DEMO-LG-FRIDGE-530', 'Capacidad', '530'),
    ('DEMO-LG-FRIDGE-530', 'Eficiencia energetica', 'A+'),
    ('DEMO-SAMSUNG-WASHER-11', 'Capacidad', '11'),
    ('DEMO-SAMSUNG-WASHER-11', 'Velocidad de centrifugado', '1400'),
    ('DEMO-BOSCH-WASHER-9', 'Capacidad', '9'),
    ('DEMO-BOSCH-WASHER-9', 'Velocidad de centrifugado', '1200'),
    ('DEMO-APPLE-IPHONE15-128', 'Almacenamiento', '128'),
    ('DEMO-APPLE-IPHONE15-128', 'Memoria RAM', '6'),
    ('DEMO-APPLE-IPHONE15-128', 'Pantalla', '6.1'),
    ('DEMO-SAMSUNG-S24-256', 'Almacenamiento', '256'),
    ('DEMO-SAMSUNG-S24-256', 'Memoria RAM', '8'),
    ('DEMO-SAMSUNG-S24-256', 'Pantalla', '6.2'),
    ('DEMO-XIAOMI-REDMI13-256', 'Almacenamiento', '256'),
    ('DEMO-XIAOMI-REDMI13-256', 'Memoria RAM', '8'),
    ('DEMO-XIAOMI-REDMI13-256', 'Pantalla', '6.67'),
    ('DEMO-MOTO-EDGE50-256', 'Almacenamiento', '256'),
    ('DEMO-MOTO-EDGE50-256', 'Memoria RAM', '12'),
    ('DEMO-MOTO-EDGE50-256', 'Pantalla', '6.7'),
    ('DEMO-LENOVO-IDEAPAD-16', 'Almacenamiento', '512'),
    ('DEMO-LENOVO-IDEAPAD-16', 'Memoria RAM', '16'),
    ('DEMO-LENOVO-IDEAPAD-16', 'Pantalla', '16'),
    ('DEMO-HP-PAVILION-15', 'Almacenamiento', '512'),
    ('DEMO-HP-PAVILION-15', 'Memoria RAM', '16'),
    ('DEMO-HP-PAVILION-15', 'Pantalla', '15.6'),
    ('DEMO-ASUS-TUF-A15', 'Almacenamiento', '1000'),
    ('DEMO-ASUS-TUF-A15', 'Memoria RAM', '16'),
    ('DEMO-ASUS-TUF-A15', 'Pantalla', '15.6'),
    ('DEMO-SONY-TV-55-4K', 'Pantalla', '55'),
    ('DEMO-SONY-TV-55-4K', 'Resolucion', '4K UHD'),
    ('DEMO-LG-TV-OLED-65', 'Pantalla', '65'),
    ('DEMO-LG-TV-OLED-65', 'Resolucion', '4K UHD'),
    ('DEMO-TCL-TV-QLED-50', 'Pantalla', '50'),
    ('DEMO-TCL-TV-QLED-50', 'Resolucion', '4K UHD'),
    ('DEMO-SONY-HEADPHONES-XM5', 'Tipo', 'Audifonos inalambricos'),
    ('DEMO-SONY-HEADPHONES-XM5', 'Autonomia', '30'),
    ('DEMO-JBL-FLIP6', 'Tipo', 'Parlante Bluetooth'),
    ('DEMO-JBL-FLIP6', 'Autonomia', '12'),
    ('DEMO-APPLE-AIRPODS-PRO2', 'Tipo', 'Audifonos inalambricos'),
    ('DEMO-APPLE-AIRPODS-PRO2', 'Autonomia', '6'),
    ('DEMO-SONY-PS5-SLIM', 'Almacenamiento', '1000'),
    ('DEMO-SONY-PS5-SLIM', 'Plataforma', 'PlayStation'),
    ('DEMO-NINTENDO-SWITCH-OLED', 'Almacenamiento', '64'),
    ('DEMO-NINTENDO-SWITCH-OLED', 'Plataforma', 'Nintendo Switch'),
    ('DEMO-XBOX-SERIES-X-1TB', 'Almacenamiento', '1000'),
    ('DEMO-XBOX-SERIES-X-1TB', 'Plataforma', 'Xbox'),
    ('DEMO-XIAOMI-ROBOT-VACUUM', 'Potencia', '55'),
    ('DEMO-XIAOMI-ROBOT-VACUUM', 'Tipo', 'Aspiradora robot')
) AS value(sku, specification_name, value)
JOIN product ON product.sku = value.sku
JOIN product_category_specification specification
    ON specification.category_id = product.category_id
   AND specification.name = value.specification_name
ON CONFLICT (product_id, specification_id) DO UPDATE
SET value = EXCLUDED.value;

INSERT INTO product (category_id, brand_id, name, model, sku, description)
SELECT category.id, brand.id, 'Aspiradora robot Xiaomi S10', 'Robot Vacuum S10',
       'DEMO-XIAOMI-ROBOT-VACUUM',
       'Aspiradora robot con navegacion laser y control desde el telefono.'
FROM product_category category
JOIN brand ON brand.name = 'Xiaomi'
WHERE category.name = 'Hogar'
ON CONFLICT (sku) DO UPDATE
SET category_id = EXCLUDED.category_id,
    brand_id = EXCLUDED.brand_id,
    name = EXCLUDED.name,
    model = EXCLUDED.model,
    description = EXCLUDED.description;

INSERT INTO product (category_id, brand_id, name, model, sku, description)
SELECT category.id, brand.id, 'Freidora de aire Philips Essential 6.2 L', 'Airfryer HD9270',
       'DEMO-PHILIPS-AIRFRYER-62',
       'Freidora de aire con capacidad familiar y controles digitales.'
FROM product_category category
JOIN brand ON brand.name = 'Philips'
WHERE category.name = 'Hogar'
ON CONFLICT (sku) DO UPDATE
SET category_id = EXCLUDED.category_id,
    brand_id = EXCLUDED.brand_id,
    name = EXCLUDED.name,
    model = EXCLUDED.model,
    description = EXCLUDED.description;

INSERT INTO product_specification_value (product_id, specification_id, value)
SELECT product.id, specification.id, value.value
FROM (VALUES
    ('DEMO-PHILIPS-AIRFRYER-62', 'Potencia', '2000'),
    ('DEMO-PHILIPS-AIRFRYER-62', 'Tipo', 'Freidora de aire'),
    ('DEMO-XIAOMI-ROBOT-VACUUM', 'Potencia', '55'),
    ('DEMO-XIAOMI-ROBOT-VACUUM', 'Tipo', 'Aspiradora robot')
) AS value(sku, specification_name, value)
JOIN product ON product.sku = value.sku
JOIN product_category_specification specification
    ON specification.category_id = product.category_id
   AND specification.name = value.specification_name
ON CONFLICT (product_id, specification_id) DO UPDATE
SET value = EXCLUDED.value;

INSERT INTO product_image (product_id, image_url, alt_text, sort_order)
SELECT product.id,
       'https://placehold.co/640x640?text=' || product.sku,
       product.name,
       0
FROM product
WHERE product.sku IN ('DEMO-XIAOMI-ROBOT-VACUUM', 'DEMO-PHILIPS-AIRFRYER-62')
  AND NOT EXISTS (
      SELECT 1 FROM product_image image
      WHERE image.product_id = product.id
        AND image.image_url = 'https://placehold.co/640x640?text=' || product.sku
  );

INSERT INTO product_offer
    (product_id, store_id, price, list_price, currency, shipping_cost, shipping_free,
     available, stock, condition, product_url)
SELECT product.id, store.id, offer.price, offer.list_price, 'CLP', offer.shipping_cost,
       offer.shipping_free, offer.available, offer.stock, offer.condition,
       'https://demo.soloservis.cl/products/' || product.sku || '?store=' || LOWER(REPLACE(store.name, ' ', '-'))
FROM (VALUES
    ('DEMO-MIDEA-FRIDGE-400', 'Falabella', 499990, 579990, 0, TRUE, TRUE, 12, 'new'),
    ('DEMO-MIDEA-FRIDGE-400', 'Ripley', 519990, 579990, 5990, FALSE, TRUE, 5, 'new'),
    ('DEMO-MIDEA-FRIDGE-400', 'Lider', 489990, 549990, 0, TRUE, TRUE, 7, 'new'),
    ('DEMO-LG-FRIDGE-530', 'Paris', 899990, 999990, 0, TRUE, TRUE, 4, 'new'),
    ('DEMO-LG-FRIDGE-530', 'Ripley', 929990, 999990, 7990, FALSE, TRUE, 2, 'new'),
    ('DEMO-SAMSUNG-WASHER-11', 'Falabella', 429990, 499990, 0, TRUE, TRUE, 8, 'new'),
    ('DEMO-SAMSUNG-WASHER-11', 'Lider', 449990, 499990, 0, TRUE, TRUE, 0, 'new'),
    ('DEMO-BOSCH-WASHER-9', 'Sodimac', 549990, 629990, 0, TRUE, TRUE, 3, 'new'),
    ('DEMO-BOSCH-WASHER-9', 'Paris', 579990, 629990, 5990, FALSE, TRUE, 6, 'new'),
    ('DEMO-APPLE-IPHONE15-128', 'Falabella', 699990, 799990, 0, TRUE, TRUE, 10, 'new'),
    ('DEMO-APPLE-IPHONE15-128', 'Mercado Libre', 679990, 799990, 0, TRUE, TRUE, 4, 'new'),
    ('DEMO-APPLE-IPHONE15-128', 'Ripley', 729990, 799990, 4990, FALSE, TRUE, 3, 'new'),
    ('DEMO-SAMSUNG-S24-256', 'Paris', 749990, 899990, 0, TRUE, TRUE, 9, 'new'),
    ('DEMO-SAMSUNG-S24-256', 'Mercado Libre', 719990, 899990, 0, TRUE, TRUE, 2, 'refurbished'),
    ('DEMO-XIAOMI-REDMI13-256', 'Lider', 219990, 269990, 0, TRUE, TRUE, 15, 'new'),
    ('DEMO-XIAOMI-REDMI13-256', 'PC Factory', 229990, 269990, 3990, FALSE, TRUE, 6, 'new'),
    ('DEMO-MOTO-EDGE50-256', 'Ripley', 329990, 399990, 0, TRUE, TRUE, 5, 'new'),
    ('DEMO-MOTO-EDGE50-256', 'Mercado Libre', 299990, 379990, 0, TRUE, TRUE, 1, 'used'),
    ('DEMO-LENOVO-IDEAPAD-16', 'PC Factory', 649990, 749990, 0, TRUE, TRUE, 7, 'new'),
    ('DEMO-LENOVO-IDEAPAD-16', 'Falabella', 679990, 749990, 5990, FALSE, TRUE, 2, 'new'),
    ('DEMO-HP-PAVILION-15', 'Paris', 599990, 699990, 0, TRUE, TRUE, 4, 'new'),
    ('DEMO-HP-PAVILION-15', 'Mercado Libre', 579990, 699990, 0, TRUE, TRUE, 3, 'new'),
    ('DEMO-ASUS-TUF-A15', 'PC Factory', 999990, 1199990, 0, TRUE, TRUE, 3, 'new'),
    ('DEMO-ASUS-TUF-A15', 'Ripley', 1049990, 1199990, 7990, FALSE, TRUE, 2, 'new'),
    ('DEMO-SONY-TV-55-4K', 'Falabella', 499990, 599990, 0, TRUE, TRUE, 6, 'new'),
    ('DEMO-SONY-TV-55-4K', 'Lider', 519990, 599990, 0, TRUE, TRUE, 9, 'new'),
    ('DEMO-LG-TV-OLED-65', 'Paris', 1499990, 1799990, 0, TRUE, TRUE, 2, 'new'),
    ('DEMO-LG-TV-OLED-65', 'Ripley', 1549990, 1799990, 9990, FALSE, TRUE, 1, 'new'),
    ('DEMO-TCL-TV-QLED-50', 'Lider', 349990, 429990, 0, TRUE, TRUE, 8, 'new'),
    ('DEMO-TCL-TV-QLED-50', 'Falabella', 369990, 429990, 0, TRUE, TRUE, 5, 'new'),
    ('DEMO-SONY-HEADPHONES-XM5', 'Mercado Libre', 299990, 399990, 0, TRUE, TRUE, 5, 'new'),
    ('DEMO-SONY-HEADPHONES-XM5', 'Paris', 329990, 399990, 4990, FALSE, TRUE, 2, 'new'),
    ('DEMO-JBL-FLIP6', 'Ripley', 99990, 129990, 0, TRUE, TRUE, 11, 'new'),
    ('DEMO-JBL-FLIP6', 'Falabella', 109990, 129990, 3990, FALSE, TRUE, 7, 'new'),
    ('DEMO-APPLE-AIRPODS-PRO2', 'Paris', 229990, 279990, 0, TRUE, TRUE, 3, 'new'),
    ('DEMO-APPLE-AIRPODS-PRO2', 'Mercado Libre', 219990, 279990, 0, TRUE, TRUE, 2, 'new'),
    ('DEMO-SONY-PS5-SLIM', 'Falabella', 549990, 649990, 0, TRUE, TRUE, 5, 'new'),
    ('DEMO-SONY-PS5-SLIM', 'Ripley', 569990, 649990, 0, TRUE, TRUE, 3, 'new'),
    ('DEMO-NINTENDO-SWITCH-OLED', 'Paris', 299990, 349990, 0, TRUE, TRUE, 6, 'new'),
    ('DEMO-NINTENDO-SWITCH-OLED', 'Mercado Libre', 289990, 349990, 0, TRUE, TRUE, 4, 'new'),
    ('DEMO-XBOX-SERIES-X-1TB', 'Lider', 549990, 649990, 0, TRUE, TRUE, 0, 'new'),
    ('DEMO-XBOX-SERIES-X-1TB', 'PC Factory', 579990, 649990, 4990, FALSE, TRUE, 2, 'new'),
    ('DEMO-XIAOMI-ROBOT-VACUUM', 'Sodimac', 249990, 329990, 0, TRUE, TRUE, 4, 'new'),
    ('DEMO-XIAOMI-ROBOT-VACUUM', 'Mercado Libre', 239990, 329990, 0, TRUE, TRUE, 3, 'new'),
    ('DEMO-PHILIPS-AIRFRYER-62', 'Falabella', 119990, 159990, 0, TRUE, TRUE, 9, 'new'),
    ('DEMO-PHILIPS-AIRFRYER-62', 'Sodimac', 129990, 159990, 4990, FALSE, TRUE, 5, 'new')
) AS offer(sku, store_name, price, list_price, shipping_cost, shipping_free, available, stock, condition)
JOIN product ON product.sku = offer.sku
JOIN store ON store.name = offer.store_name
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

INSERT INTO product_price_history (product_offer_id, price, is_promotional, recorded_at)
SELECT offer.id, history.price, history.is_promotional, history.recorded_at
FROM product_offer offer
JOIN product ON product.id = offer.product_id
CROSS JOIN LATERAL (
    VALUES
        (COALESCE(offer.list_price, offer.price), FALSE, date_trunc('day', NOW()) - INTERVAL '14 days'),
        (offer.price, offer.price < COALESCE(offer.list_price, offer.price), date_trunc('day', NOW()) - INTERVAL '1 day')
) AS history(price, is_promotional, recorded_at)
WHERE product.sku LIKE 'DEMO-%'
  AND NOT EXISTS (
      SELECT 1
      FROM product_price_history existing
      WHERE existing.product_offer_id = offer.id
        AND existing.price = history.price
        AND existing.recorded_at = history.recorded_at
  );

COMMIT;
