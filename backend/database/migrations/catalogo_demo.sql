-- SOLOServis - Ampliacion del catalogo de productos
-- Seed repetible para enriquecer productos, especificaciones y ofertas.

BEGIN;

-- CATEGORIAS
INSERT INTO product_category (parent_category_id, name, description)
SELECT NULL, 'Televisores', 'Televisores inteligentes y de alta definicion'
WHERE NOT EXISTS (SELECT 1 FROM product_category WHERE name = 'Televisores');

INSERT INTO product_category (parent_category_id, name, description)
SELECT NULL, 'Lavadoras', 'Lavadoras y soluciones para el cuidado de la ropa'
WHERE NOT EXISTS (SELECT 1 FROM product_category WHERE name = 'Lavadoras');

INSERT INTO product_category (parent_category_id, name, description)
SELECT NULL, 'Celulares', 'Telefonos inteligentes y accesorios moviles'
WHERE NOT EXISTS (SELECT 1 FROM product_category WHERE name = 'Celulares');

-- MARCAS
INSERT INTO brand (name, logo_url, website_url)
SELECT 'Sony', 'https://cdn.example.com/logos/sony.png', 'https://sony.cl'
WHERE NOT EXISTS (SELECT 1 FROM brand WHERE name = 'Sony');

INSERT INTO brand (name, logo_url, website_url)
SELECT 'Mabe', 'https://cdn.example.com/logos/mabe.png', 'https://mabe.cl'
WHERE NOT EXISTS (SELECT 1 FROM brand WHERE name = 'Mabe');

INSERT INTO brand (name, logo_url, website_url)
SELECT 'Xiaomi', 'https://cdn.example.com/logos/xiaomi.png', 'https://mi.com/cl'
WHERE NOT EXISTS (SELECT 1 FROM brand WHERE name = 'Xiaomi');

-- ESPECIFICACIONES COMPARABLES
INSERT INTO product_category_specification (category_id, name, data_type, unit, required, comparable, display_order)
SELECT c.id, 'Tamano de pantalla', 'number', 'pulgadas', TRUE, TRUE, 1
FROM product_category c
WHERE c.name = 'Televisores'
  AND NOT EXISTS (
      SELECT 1 FROM product_category_specification s
      WHERE s.category_id = c.id AND s.name = 'Tamano de pantalla'
  );

INSERT INTO product_category_specification (category_id, name, data_type, unit, required, comparable, display_order)
SELECT c.id, 'Resolucion', 'string', NULL, TRUE, TRUE, 2
FROM product_category c
WHERE c.name = 'Televisores'
  AND NOT EXISTS (
      SELECT 1 FROM product_category_specification s
      WHERE s.category_id = c.id AND s.name = 'Resolucion'
  );

INSERT INTO product_category_specification (category_id, name, data_type, unit, required, comparable, display_order)
SELECT c.id, 'Capacidad', 'number', 'kg', TRUE, TRUE, 1
FROM product_category c
WHERE c.name = 'Lavadoras'
  AND NOT EXISTS (
      SELECT 1 FROM product_category_specification s
      WHERE s.category_id = c.id AND s.name = 'Capacidad'
  );

INSERT INTO product_category_specification (category_id, name, data_type, unit, required, comparable, display_order)
SELECT c.id, 'Tipo de pantalla', 'string', NULL, TRUE, TRUE, 1
FROM product_category c
WHERE c.name = 'Celulares'
  AND NOT EXISTS (
      SELECT 1 FROM product_category_specification s
      WHERE s.category_id = c.id AND s.name = 'Tipo de pantalla'
  );

-- PRODUCTOS
INSERT INTO product (category_id, brand_id, name, model, sku, description)
SELECT c.id, b.id, v.name, v.model, v.sku, v.description
FROM (VALUES
    ('Televisores', 'Sony', 'Smart TV Sony Bravia 55 pulgadas', 'XR-55X90L', 'SKU-SONY-TV-001', 'Televisor 4K HDR con Google TV'),
    ('Televisores', 'Samsung', 'Smart TV Samsung Crystal UHD 50 pulgadas', 'UN50CU7000', 'SKU-SAMS-TV-001', 'Televisor 4K con plataforma Smart TV'),
    ('Lavadoras', 'Mabe', 'Lavadora Mabe Aqua Saver 19 kg', 'LMH72201WBAB0', 'SKU-MABE-LAV-001', 'Lavadora de carga superior con ahorro de agua'),
    ('Lavadoras', 'LG', 'Lavadora LG carga frontal 10.5 kg', 'WM10WVC4S6', 'SKU-LG-LAV-001', 'Lavadora inverter con ciclos inteligentes'),
    ('Celulares', 'Xiaomi', 'Xiaomi Redmi Note 13 Pro', '2312DRA50G', 'SKU-XIAOMI-CEL-001', 'Celular 5G con camara de alta resolucion'),
    ('Celulares', 'Samsung', 'Samsung Galaxy A55 5G', 'SM-A556E', 'SKU-SAMS-CEL-001', 'Celular 5G con pantalla Super AMOLED')
) AS v(category_name, brand_name, name, model, sku, description)
JOIN product_category c ON c.name = v.category_name
JOIN brand b ON b.name = v.brand_name
WHERE NOT EXISTS (SELECT 1 FROM product p WHERE p.sku = v.sku);

-- IMAGENES
INSERT INTO product_image (product_id, image_url, alt_text, sort_order)
SELECT p.id, v.image_url, v.alt_text, 0
FROM (VALUES
    ('SKU-SONY-TV-001', 'https://cdn.example.com/products/sony-tv-55.jpg', 'Smart TV Sony Bravia de 55 pulgadas'),
    ('SKU-SAMS-TV-001', 'https://cdn.example.com/products/samsung-tv-50.jpg', 'Smart TV Samsung Crystal UHD de 50 pulgadas'),
    ('SKU-MABE-LAV-001', 'https://cdn.example.com/products/mabe-lavadora-19kg.jpg', 'Lavadora Mabe de 19 kg'),
    ('SKU-LG-LAV-001', 'https://cdn.example.com/products/lg-lavadora-10kg.jpg', 'Lavadora LG de 10.5 kg'),
    ('SKU-XIAOMI-CEL-001', 'https://cdn.example.com/products/xiaomi-redmi-note-13-pro.jpg', 'Xiaomi Redmi Note 13 Pro'),
    ('SKU-SAMS-CEL-001', 'https://cdn.example.com/products/samsung-galaxy-a55.jpg', 'Samsung Galaxy A55 5G')
) AS v(sku, image_url, alt_text)
JOIN product p ON p.sku = v.sku
WHERE NOT EXISTS (
    SELECT 1 FROM product_image i WHERE i.product_id = p.id AND i.image_url = v.image_url
);

-- VALORES DE ESPECIFICACIONES
INSERT INTO product_specification_value (product_id, specification_id, value)
SELECT p.id, s.id, v.value
FROM (VALUES
    ('SKU-SONY-TV-001', 'Tamano de pantalla', '55'),
    ('SKU-SONY-TV-001', 'Resolucion', '4K UHD'),
    ('SKU-SAMS-TV-001', 'Tamano de pantalla', '50'),
    ('SKU-SAMS-TV-001', 'Resolucion', '4K UHD'),
    ('SKU-MABE-LAV-001', 'Capacidad', '19'),
    ('SKU-LG-LAV-001', 'Capacidad', '10.5'),
    ('SKU-XIAOMI-CEL-001', 'Tipo de pantalla', 'AMOLED'),
    ('SKU-SAMS-CEL-001', 'Tipo de pantalla', 'Super AMOLED')
) AS v(sku, specification_name, value)
JOIN product p ON p.sku = v.sku
JOIN product_category_specification s ON s.name = v.specification_name AND s.category_id = p.category_id
ON CONFLICT (product_id, specification_id) DO UPDATE SET value = EXCLUDED.value;

-- OFERTAS EN LAS TIENDAS EXISTENTES
INSERT INTO product_offer (product_id, store_id, price, list_price, currency, shipping_cost, shipping_free, available, stock, condition, product_url)
SELECT p.id, s.id, v.price, v.list_price, 'CLP', v.shipping_cost, v.shipping_free, TRUE, v.stock, 'new', v.product_url
FROM (VALUES
    ('SKU-SONY-TV-001', 'Falabella', 649990, 699990, 0, TRUE, 8, 'https://falabella.com/sony-bravia-55'),
    ('SKU-SONY-TV-001', 'Ripley', 639990, 699990, 4990, FALSE, 6, 'https://ripley.cl/sony-bravia-55'),
    ('SKU-SAMS-TV-001', 'Paris', 429990, 479990, 0, TRUE, 10, 'https://paris.cl/samsung-crystal-50'),
    ('SKU-MABE-LAV-001', 'La Polar', 329990, 379990, 5990, FALSE, 5, 'https://lapolar.cl/mabe-aqua-saver-19'),
    ('SKU-LG-LAV-001', 'Falabella', 449990, 499990, 0, TRUE, 7, 'https://falabella.com/lg-lavadora-10kg'),
    ('SKU-XIAOMI-CEL-001', 'Paris', 299990, 349990, 0, TRUE, 12, 'https://paris.cl/xiaomi-redmi-note-13-pro'),
    ('SKU-SAMS-CEL-001', 'Ripley', 399990, 449990, 0, TRUE, 9, 'https://ripley.cl/samsung-galaxy-a55')
) AS v(sku, store_name, price, list_price, shipping_cost, shipping_free, stock, product_url)
JOIN product p ON p.sku = v.sku
JOIN store s ON s.name = v.store_name
WHERE NOT EXISTS (
    SELECT 1 FROM product_offer o WHERE o.product_id = p.id AND o.store_id = s.id
);

COMMIT;
