-- =====================================================================
-- Datos de prueba adicionales para el catálogo de productos
-- Amplía el árbol de categorías (3 niveles) y agrega marcas, productos,
-- especificaciones, ofertas, historial de precios y reseñas.
--
-- Es idempotente: cada bloque se salta si el dato ya existe, así que se
-- puede ejecutar varias veces sin duplicar filas. Las referencias se
-- resuelven por nombre y no por id, para no depender del orden de carga.
-- =====================================================================

-- ---------------------------------------------------------------------
-- CATEGORÍAS
-- ---------------------------------------------------------------------

INSERT INTO product_category (parent_category_id, name, description)
SELECT NULL, 'Tecnología', 'Categoría raíz de tecnología'
WHERE NOT EXISTS (
    SELECT 1 FROM product_category WHERE name = 'Tecnología' AND parent_category_id IS NULL
);

INSERT INTO product_category (parent_category_id, name, description)
SELECT (SELECT id FROM product_category WHERE name = 'Tecnología' AND parent_category_id IS NULL),
       'Computación', 'Equipos de computación'
WHERE NOT EXISTS (SELECT 1 FROM product_category WHERE name = 'Computación');

-- Tercer nivel: permite comprobar que el filtro por categoría recorre todo el subárbol
INSERT INTO product_category (parent_category_id, name, description)
SELECT (SELECT id FROM product_category WHERE name = 'Computación'),
       'Notebooks', 'Computadores portátiles'
WHERE NOT EXISTS (SELECT 1 FROM product_category WHERE name = 'Notebooks');

INSERT INTO product_category (parent_category_id, name, description)
SELECT (SELECT id FROM product_category WHERE name = 'Tecnología' AND parent_category_id IS NULL),
       'Celulares', 'Teléfonos móviles'
WHERE NOT EXISTS (SELECT 1 FROM product_category WHERE name = 'Celulares');

INSERT INTO product_category (parent_category_id, name, description)
SELECT (SELECT id FROM product_category WHERE name = 'Electrodomésticos' AND parent_category_id IS NULL),
       'Lavadoras', 'Lavadoras y secadoras'
WHERE NOT EXISTS (SELECT 1 FROM product_category WHERE name = 'Lavadoras');

-- ---------------------------------------------------------------------
-- MARCAS
-- ---------------------------------------------------------------------

INSERT INTO brand (name, logo_url, website_url)
SELECT 'Lenovo', 'https://cdn.example.com/logos/lenovo.png', 'https://lenovo.com'
WHERE NOT EXISTS (SELECT 1 FROM brand WHERE name = 'Lenovo');

INSERT INTO brand (name, logo_url, website_url)
SELECT 'Apple', 'https://cdn.example.com/logos/apple.png', 'https://apple.com'
WHERE NOT EXISTS (SELECT 1 FROM brand WHERE name = 'Apple');

INSERT INTO brand (name, logo_url, website_url)
SELECT 'Xiaomi', 'https://cdn.example.com/logos/xiaomi.png', 'https://xiaomi.com'
WHERE NOT EXISTS (SELECT 1 FROM brand WHERE name = 'Xiaomi');

-- ---------------------------------------------------------------------
-- TIENDAS
-- ---------------------------------------------------------------------

INSERT INTO store (name, website_url, logo_url, rating, reputation, shipping_information, general_conditions)
SELECT 'Ripley', 'https://ripley.cl', 'https://cdn.example.com/stores/ripley.png', 3.90, 'Regular',
       'Envío en 4-6 días hábiles', 'Garantía de 12 meses'
WHERE NOT EXISTS (SELECT 1 FROM store WHERE name = 'Ripley');

-- ---------------------------------------------------------------------
-- ESPECIFICACIONES POR CATEGORÍA
-- ---------------------------------------------------------------------

INSERT INTO product_category_specification (category_id, name, data_type, unit, required, comparable, display_order)
SELECT c.id, s.name, s.data_type, s.unit, s.required, s.comparable, s.display_order
FROM (VALUES
    ('Notebooks',  'Procesador',            'string', NULL,       TRUE,  TRUE, 1),
    ('Notebooks',  'RAM',                   'number', 'GB',       TRUE,  TRUE, 2),
    ('Notebooks',  'Almacenamiento',        'number', 'GB',       TRUE,  TRUE, 3),
    ('Notebooks',  'Pantalla',              'number', 'pulgadas', FALSE, TRUE, 4),
    ('Celulares',  'Pantalla',              'number', 'pulgadas', TRUE,  TRUE, 1),
    ('Celulares',  'RAM',                   'number', 'GB',       TRUE,  TRUE, 2),
    ('Celulares',  'Almacenamiento',        'number', 'GB',       TRUE,  TRUE, 3),
    ('Celulares',  'Batería',               'number', 'mAh',      FALSE, TRUE, 4),
    ('Lavadoras',  'Capacidad',             'number', 'kg',       TRUE,  TRUE, 1),
    ('Lavadoras',  'Eficiencia energética', 'string', NULL,       FALSE, TRUE, 2)
) AS s(category_name, name, data_type, unit, required, comparable, display_order)
JOIN product_category c ON c.name = s.category_name
WHERE NOT EXISTS (
    SELECT 1 FROM product_category_specification pcs
    WHERE pcs.category_id = c.id AND pcs.name = s.name
);

-- ---------------------------------------------------------------------
-- PRODUCTOS
-- ---------------------------------------------------------------------

INSERT INTO product (category_id, brand_id, name, model, sku, description)
SELECT c.id, b.id, p.name, p.model, p.sku, p.description
FROM (VALUES
    ('Notebooks', 'Lenovo', 'Notebook Lenovo IdeaPad Slim 3', 'IP3-15',   'SKU-LEN-001', 'Notebook de 15 pulgadas para uso diario'),
    ('Notebooks', 'Apple',  'MacBook Air 13 M2',              'MBA-M2',   'SKU-APP-001', 'Notebook ultraliviano con chip M2'),
    ('Celulares', 'Samsung','Samsung Galaxy A55 5G',          'SM-A556',  'SKU-SAMS-002','Gama media con pantalla AMOLED'),
    ('Celulares', 'Xiaomi', 'Xiaomi Redmi Note 13',           'RN13',     'SKU-XIA-001', 'Gama media con batería de larga duración'),
    ('Lavadoras', 'LG',     'Lavadora LG TurboWash 12 kg',    'FV1412',   'SKU-LG-002',  'Lavadora carga frontal con vapor')
) AS p(category_name, brand_name, name, model, sku, description)
JOIN product_category c ON c.name = p.category_name
JOIN brand b ON b.name = p.brand_name
WHERE NOT EXISTS (SELECT 1 FROM product WHERE sku = p.sku);

-- ---------------------------------------------------------------------
-- IMÁGENES
-- ---------------------------------------------------------------------

INSERT INTO product_image (product_id, image_url, alt_text, sort_order)
SELECT pr.id, i.image_url, i.alt_text, 0
FROM (VALUES
    ('SKU-LEN-001', 'https://cdn.example.com/products/lenovo-ideapad.jpg', 'Notebook Lenovo IdeaPad'),
    ('SKU-APP-001', 'https://cdn.example.com/products/macbook-air.jpg',    'MacBook Air M2'),
    ('SKU-SAMS-002','https://cdn.example.com/products/galaxy-a55.jpg',     'Samsung Galaxy A55'),
    ('SKU-XIA-001', 'https://cdn.example.com/products/redmi-note-13.jpg',  'Xiaomi Redmi Note 13'),
    ('SKU-LG-002',  'https://cdn.example.com/products/lg-turbowash.jpg',   'Lavadora LG TurboWash')
) AS i(sku, image_url, alt_text)
JOIN product pr ON pr.sku = i.sku
WHERE NOT EXISTS (SELECT 1 FROM product_image WHERE image_url = i.image_url);

-- ---------------------------------------------------------------------
-- VALORES DE ESPECIFICACIONES
-- ---------------------------------------------------------------------

INSERT INTO product_specification_value (product_id, specification_id, value)
SELECT pr.id, pcs.id, v.value
FROM (VALUES
    ('SKU-LEN-001', 'Procesador',            'Intel Core i5-1235U'),
    ('SKU-LEN-001', 'RAM',                   '16'),
    ('SKU-LEN-001', 'Almacenamiento',        '512'),
    ('SKU-LEN-001', 'Pantalla',              '15.6'),
    ('SKU-APP-001', 'Procesador',            'Apple M2'),
    ('SKU-APP-001', 'RAM',                   '8'),
    ('SKU-APP-001', 'Almacenamiento',        '256'),
    ('SKU-APP-001', 'Pantalla',              '13.6'),
    ('SKU-SAMS-002','Pantalla',              '6.6'),
    ('SKU-SAMS-002','RAM',                   '8'),
    ('SKU-SAMS-002','Almacenamiento',        '256'),
    ('SKU-SAMS-002','Batería',               '5000'),
    ('SKU-XIA-001', 'Pantalla',              '6.67'),
    ('SKU-XIA-001', 'RAM',                   '8'),
    ('SKU-XIA-001', 'Almacenamiento',        '256'),
    ('SKU-XIA-001', 'Batería',               '5000'),
    ('SKU-LG-002',  'Capacidad',             '12'),
    ('SKU-LG-002',  'Eficiencia energética', 'A+++')
) AS v(sku, spec_name, value)
JOIN product pr ON pr.sku = v.sku
JOIN product_category_specification pcs
  ON pcs.category_id = pr.category_id AND pcs.name = v.spec_name
ON CONFLICT (product_id, specification_id) DO NOTHING;

-- ---------------------------------------------------------------------
-- OFERTAS
-- ---------------------------------------------------------------------

INSERT INTO product_offer (product_id, store_id, price, list_price, currency, shipping_cost, shipping_free, available, stock, condition, product_url)
SELECT pr.id, st.id, o.price, o.list_price, 'CLP', o.shipping_cost, o.shipping_free, o.available, o.stock, 'new', o.product_url
FROM (VALUES
    ('SKU-LEN-001', 'Falabella', 549990::numeric, 629990::numeric,     0::numeric, TRUE,  TRUE, 12, 'https://falabella.com/producto/lenovo-ideapad'),
    ('SKU-LEN-001', 'Ripley',    569990::numeric, 599990::numeric,  5990::numeric, FALSE, TRUE,  4, 'https://ripley.cl/producto/lenovo-ideapad'),
    ('SKU-APP-001', 'Falabella', 999990::numeric,1099990::numeric,     0::numeric, TRUE,  TRUE,  6, 'https://falabella.com/producto/macbook-air-m2'),
    ('SKU-APP-001', 'Paris',    1049990::numeric,1049990::numeric,     0::numeric, TRUE,  FALSE, 0, 'https://paris.cl/producto/macbook-air-m2'),
    ('SKU-SAMS-002','Paris',     329990::numeric, 379990::numeric,  3990::numeric, FALSE, TRUE, 20, 'https://paris.cl/producto/galaxy-a55'),
    ('SKU-SAMS-002','Ripley',    319990::numeric, 359990::numeric,     0::numeric, TRUE,  TRUE,  9, 'https://ripley.cl/producto/galaxy-a55'),
    ('SKU-XIA-001', 'Falabella', 229990::numeric, 259990::numeric,  2990::numeric, FALSE, TRUE, 30, 'https://falabella.com/producto/redmi-note-13'),
    ('SKU-LG-002',  'Falabella', 449990::numeric, 519990::numeric,     0::numeric, TRUE,  TRUE,  7, 'https://falabella.com/producto/lg-turbowash'),
    ('SKU-LG-002',  'Paris',     459990::numeric, 459990::numeric,  6990::numeric, FALSE, TRUE,  3, 'https://paris.cl/producto/lg-turbowash')
) AS o(sku, store_name, price, list_price, shipping_cost, shipping_free, available, stock, product_url)
JOIN product pr ON pr.sku = o.sku
JOIN store st ON st.name = o.store_name
WHERE NOT EXISTS (
    SELECT 1 FROM product_offer po WHERE po.product_id = pr.id AND po.store_id = st.id
);

-- ---------------------------------------------------------------------
-- HISTORIAL DE PRECIOS (para el gráfico del detalle)
-- ---------------------------------------------------------------------

INSERT INTO product_price_history (product_offer_id, price, is_promotional, recorded_at)
SELECT po.id, h.price, h.is_promotional, NOW() - (h.days_ago || ' days')::interval
FROM (VALUES
    ('SKU-LEN-001', 'Falabella', 629990::numeric, FALSE, 45),
    ('SKU-LEN-001', 'Falabella', 599990::numeric, FALSE, 20),
    ('SKU-LEN-001', 'Falabella', 549990::numeric, TRUE,   3),
    ('SKU-APP-001', 'Falabella',1099990::numeric, FALSE, 40),
    ('SKU-APP-001', 'Falabella', 999990::numeric, TRUE,   5),
    ('SKU-SAMS-002','Paris',     379990::numeric, FALSE, 30),
    ('SKU-SAMS-002','Paris',     329990::numeric, TRUE,   2),
    ('SKU-LG-002',  'Falabella', 519990::numeric, FALSE, 35),
    ('SKU-LG-002',  'Falabella', 449990::numeric, TRUE,   4)
) AS h(sku, store_name, price, is_promotional, days_ago)
JOIN product pr ON pr.sku = h.sku
JOIN store st ON st.name = h.store_name
JOIN product_offer po ON po.product_id = pr.id AND po.store_id = st.id
WHERE NOT EXISTS (
    SELECT 1 FROM product_price_history ph
    WHERE ph.product_offer_id = po.id AND ph.price = h.price
);

-- ---------------------------------------------------------------------
-- RESEÑAS
-- ---------------------------------------------------------------------

INSERT INTO product_review (user_id, product_id, rating, title, content, verified)
SELECT u.id, pr.id, r.rating, r.title, r.content, FALSE
FROM (VALUES
    ('camila.fuentes@example.com', 'SKU-LEN-001', 4, 'Buen notebook para estudiar', 'Rápido para tareas y clases, la pantalla podría ser mejor.'),
    ('jorge.munoz@example.com',    'SKU-LEN-001', 3, 'Cumple', 'Anda bien, pero se calienta con varias pestañas abiertas.'),
    ('camila.fuentes@example.com', 'SKU-APP-001', 5, 'Excelente autonomía', 'La batería dura todo el día de trabajo sin problemas.'),
    ('jorge.munoz@example.com',    'SKU-SAMS-002',4, 'Muy buena pantalla', 'La pantalla se ve muy bien y la batería rinde.'),
    ('camila.fuentes@example.com', 'SKU-LG-002',  5, 'Lava muy bien', 'Silenciosa y con buena capacidad para una familia.')
) AS r(email, sku, rating, title, content)
JOIN user_account u ON u.email = r.email
JOIN product pr ON pr.sku = r.sku
ON CONFLICT (user_id, product_id) DO NOTHING;
