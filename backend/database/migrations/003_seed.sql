-- =====================================================================
-- SOLOServis - Datos de prueba REALES (tarjeta Trello: "Crear datos de
-- prueba reales")
--
-- Requisitos de la tarjeta:
--   - 10 a 20 productos
--   - varias categorías
--   - varias tiendas
--   - múltiples ofertas por producto (ej: RTX 4060 en 3 tiendas)
--
-- Diseño: se referencian FKs por nombre/SKU (subconsultas) en vez de IDs
-- fijos, para no chocar con los IDs ya insertados por 003_seed.sql y
-- para que el script sea legible y fácil de mantener.
-- =====================================================================

-- =====================================================================
-- CATEGORÍAS (Tecnología > 5 subcategorías)
-- =====================================================================

INSERT INTO product_category (parent_category_id, name, description) VALUES
(NULL, 'Tecnología', 'Categoría raíz de productos tecnológicos');

INSERT INTO product_category (parent_category_id, name, description)
SELECT id, 'Tarjetas de Video', 'GPUs para PC de escritorio'
FROM product_category WHERE name = 'Tecnología';

INSERT INTO product_category (parent_category_id, name, description)
SELECT id, 'Procesadores', 'CPUs de escritorio'
FROM product_category WHERE name = 'Tecnología';

INSERT INTO product_category (parent_category_id, name, description)
SELECT id, 'Notebooks', 'Laptops de uso general y gaming'
FROM product_category WHERE name = 'Tecnología';

INSERT INTO product_category (parent_category_id, name, description)
SELECT id, 'Monitores', 'Monitores para PC'
FROM product_category WHERE name = 'Tecnología';

INSERT INTO product_category (parent_category_id, name, description)
SELECT id, 'Periféricos', 'Mouse, teclados y audífonos'
FROM product_category WHERE name = 'Tecnología';

-- =====================================================================
-- MARCAS
-- =====================================================================

INSERT INTO brand (name, website_url) VALUES
('ASUS', 'https://asus.com'),
('MSI', 'https://msi.com'),
('Gigabyte', 'https://gigabyte.com'),
('AMD', 'https://amd.com'),
('Intel', 'https://intel.com'),
('Lenovo', 'https://lenovo.com'),
('HP', 'https://hp.com'),
('Samsung', 'https://samsung.com'),
('LG', 'https://lg.com'),
('Logitech', 'https://logitech.com')
ON CONFLICT (name) DO NOTHING;

-- =====================================================================
-- TIENDAS
-- =====================================================================

INSERT INTO store (name, website_url, rating, reputation, shipping_information, general_conditions) VALUES
('PC Factory', 'https://pcfactory.cl', 4.10, 'Buena', 'Envío en 2-4 días hábiles', 'Garantía de 12 meses'),
('WEI',        'https://wei.cl',       3.90, 'Buena', 'Envío en 3-5 días hábiles', 'Garantía de 12 meses'),
('Spdigital',  'https://spdigital.cl', 4.30, 'Muy buena', 'Retiro en tienda o envío en 1-3 días', 'Garantía de 12 meses'),
('Winpy',      'https://winpy.cl',     3.80, 'Regular', 'Envío en 4-6 días hábiles', 'Garantía de 12 meses');

-- =====================================================================
-- PRODUCTOS (15 productos, 5 categorías)
-- =====================================================================

INSERT INTO product (category_id, brand_id, name, model, sku, description)
SELECT c.id, b.id, v.name, v.model, v.sku, v.description
FROM (VALUES
  ('Tarjetas de Video', 'ASUS',     'Tarjeta de Video ASUS Dual GeForce RTX 4060 OC',      'DUAL-RTX4060-O8G',     'SKU-GPU-001', 'GPU NVIDIA RTX 4060 8GB GDDR6, edición OC'),
  ('Tarjetas de Video', 'MSI',      'Tarjeta de Video MSI GeForce RTX 4070 Ventus 3X',     'RTX4070-VENTUS3X',     'SKU-GPU-002', 'GPU NVIDIA RTX 4070 12GB GDDR6X'),
  ('Tarjetas de Video', 'Gigabyte', 'Tarjeta de Video Gigabyte Radeon RX 7600 Eagle',       'RX7600-EAGLE-8G',      'SKU-GPU-003', 'GPU AMD RX 7600 8GB GDDR6'),
  ('Tarjetas de Video', 'Gigabyte', 'Tarjeta de Video Gigabyte GeForce RTX 4060 Ti Windforce', 'RTX4060TI-WF-8G',  'SKU-GPU-004', 'GPU NVIDIA RTX 4060 Ti 8GB GDDR6'),
  ('Procesadores',      'AMD',      'Procesador AMD Ryzen 5 7600X',                         'RYZEN5-7600X',         'SKU-CPU-001', 'CPU AM5, 6 núcleos / 12 hilos, hasta 5.3GHz'),
  ('Procesadores',      'Intel',    'Procesador Intel Core i5-13400F',                      'I5-13400F',            'SKU-CPU-002', 'CPU LGA1700, 10 núcleos / 16 hilos'),
  ('Procesadores',      'AMD',      'Procesador AMD Ryzen 7 7800X3D',                       'RYZEN7-7800X3D',       'SKU-CPU-003', 'CPU AM5 con 3D V-Cache, ideal para gaming'),
  ('Notebooks',         'Lenovo',   'Notebook Lenovo IdeaPad Slim 3 15.6" i5 8GB 512GB',    'IDEAPAD-SLIM3-15',     'SKU-NB-001',  'Notebook uso general, SSD 512GB'),
  ('Notebooks',         'HP',       'Notebook HP Pavilion 15 Ryzen 5 8GB 512GB',            'PAVILION-15-R5',       'SKU-NB-002',  'Notebook uso general, pantalla 15.6"'),
  ('Notebooks',         'ASUS',     'Notebook ASUS Vivobook 15 i7 16GB 512GB',              'VIVOBOOK15-I7',        'SKU-NB-003',  'Notebook gama media-alta, 16GB RAM'),
  ('Monitores',         'Samsung',  'Monitor Samsung Odyssey G5 27" 165Hz Curvo',           'ODYSSEY-G5-27',        'SKU-MON-001', 'Monitor gamer curvo QHD 165Hz'),
  ('Monitores',         'LG',       'Monitor LG UltraGear 24" 144Hz',                       'ULTRAGEAR-24-144',     'SKU-MON-002', 'Monitor gamer Full HD 144Hz'),
  ('Periféricos',       'Logitech', 'Mouse Logitech G203 Lightsync',                        'G203-LIGHTSYNC',       'SKU-PER-001', 'Mouse gamer alámbrico 8000 DPI'),
  ('Periféricos',       'Logitech', 'Teclado Logitech K380 Bluetooth Multi-Dispositivo',    'K380-BT',              'SKU-PER-002', 'Teclado inalámbrico compacto'),
  ('Periféricos',       'Logitech', 'Audífonos Logitech G435 Inalámbricos',                 'G435-WIRELESS',        'SKU-PER-003', 'Audífonos gamer inalámbricos livianos')
) AS v(category_name, brand_name, name, model, sku, description)
JOIN product_category c ON c.name = v.category_name
JOIN brand b ON b.name = v.brand_name;

-- =====================================================================
-- OFERTAS (2 a 4 tiendas por producto, precios en CLP)
-- Ejemplo tarjeta Trello -> RTX 4060: PC Factory 299990 / WEI 319990 / Spdigital 289990
-- =====================================================================

INSERT INTO product_offer (product_id, store_id, price, list_price, currency, shipping_cost, shipping_free, available, stock, condition, product_url)
SELECT p.id, s.id, v.price, v.list_price, 'CLP', v.shipping_cost, v.shipping_free, TRUE, v.stock, 'new', v.url
FROM (VALUES
  -- RTX 4060 ASUS -> 3 tiendas (mismo patrón que el ejemplo de la tarjeta)
  ('SKU-GPU-001', 'PC Factory', 299990, 319990, 0,    TRUE,  12, 'https://pcfactory.cl/p/rtx4060-asus'),
  ('SKU-GPU-001', 'WEI',        319990, 319990, 4990, FALSE, 6,  'https://wei.cl/p/rtx4060-asus'),
  ('SKU-GPU-001', 'Spdigital',  289990, 309990, 0,    TRUE,  9,  'https://spdigital.cl/p/rtx4060-asus'),

  -- RTX 4070 MSI -> 3 tiendas
  ('SKU-GPU-002', 'PC Factory', 479990, 499990, 0,    TRUE,  5,  'https://pcfactory.cl/p/rtx4070-msi'),
  ('SKU-GPU-002', 'Spdigital',  469990, 489990, 0,    TRUE,  4,  'https://spdigital.cl/p/rtx4070-msi'),
  ('SKU-GPU-002', 'Winpy',      489990, 489990, 5990, FALSE, 3,  'https://winpy.cl/p/rtx4070-msi'),

  -- RX 7600 Gigabyte -> 2 tiendas
  ('SKU-GPU-003', 'WEI',        239990, 259990, 0,    TRUE,  10, 'https://wei.cl/p/rx7600-gigabyte'),
  ('SKU-GPU-003', 'Winpy',      249990, 249990, 4990, FALSE, 7,  'https://winpy.cl/p/rx7600-gigabyte'),

  -- RTX 4060 Ti Gigabyte -> 3 tiendas
  ('SKU-GPU-004', 'PC Factory', 389990, 409990, 0,    TRUE,  6,  'https://pcfactory.cl/p/rtx4060ti-gigabyte'),
  ('SKU-GPU-004', 'WEI',        399990, 399990, 4990, FALSE, 5,  'https://wei.cl/p/rtx4060ti-gigabyte'),
  ('SKU-GPU-004', 'Spdigital',  379990, 399990, 0,    TRUE,  8,  'https://spdigital.cl/p/rtx4060ti-gigabyte'),

  -- Ryzen 5 7600X -> 2 tiendas
  ('SKU-CPU-001', 'PC Factory', 199990, 209990, 0,    TRUE,  15, 'https://pcfactory.cl/p/ryzen5-7600x'),
  ('SKU-CPU-001', 'Spdigital',  189990, 199990, 0,    TRUE,  11, 'https://spdigital.cl/p/ryzen5-7600x'),

  -- Intel i5-13400F -> 3 tiendas
  ('SKU-CPU-002', 'PC Factory', 159990, 169990, 0,    TRUE,  20, 'https://pcfactory.cl/p/i5-13400f'),
  ('SKU-CPU-002', 'WEI',        149990, 159990, 3990, FALSE, 18, 'https://wei.cl/p/i5-13400f'),
  ('SKU-CPU-002', 'Winpy',      164990, 164990, 0,    TRUE,  9,  'https://winpy.cl/p/i5-13400f'),

  -- Ryzen 7 7800X3D -> 2 tiendas
  ('SKU-CPU-003', 'Spdigital',  379990, 399990, 0,    TRUE,  7,  'https://spdigital.cl/p/ryzen7-7800x3d'),
  ('SKU-CPU-003', 'PC Factory', 399990, 399990, 0,    TRUE,  4,  'https://pcfactory.cl/p/ryzen7-7800x3d'),

  -- Notebook Lenovo IdeaPad -> 2 tiendas
  ('SKU-NB-001', 'Winpy',      349990, 379990, 0,    TRUE,  10, 'https://winpy.cl/p/ideapad-slim3'),
  ('SKU-NB-001', 'PC Factory', 359990, 379990, 0,    TRUE,  6,  'https://pcfactory.cl/p/ideapad-slim3'),

  -- Notebook HP Pavilion -> 2 tiendas
  ('SKU-NB-002', 'WEI',        399990, 429990, 0,    TRUE,  5,  'https://wei.cl/p/pavilion15'),
  ('SKU-NB-002', 'Winpy',      414990, 429990, 5990, FALSE, 4,  'https://winpy.cl/p/pavilion15'),

  -- Notebook ASUS Vivobook -> 3 tiendas
  ('SKU-NB-003', 'PC Factory', 449990, 459990, 0,    TRUE,  8,  'https://pcfactory.cl/p/vivobook15'),
  ('SKU-NB-003', 'Spdigital',  429990, 449990, 0,    TRUE,  6,  'https://spdigital.cl/p/vivobook15'),
  ('SKU-NB-003', 'WEI',        439990, 449990, 4990, FALSE, 5,  'https://wei.cl/p/vivobook15'),

  -- Monitor Samsung Odyssey G5 -> 2 tiendas
  ('SKU-MON-001', 'PC Factory', 179990, 199990, 0,    TRUE,  12, 'https://pcfactory.cl/p/odyssey-g5'),
  ('SKU-MON-001', 'Spdigital',  189990, 199990, 0,    TRUE,  9,  'https://spdigital.cl/p/odyssey-g5'),

  -- Monitor LG UltraGear -> 3 tiendas
  ('SKU-MON-002', 'WEI',        129990, 149990, 0,    TRUE,  15, 'https://wei.cl/p/ultragear24'),
  ('SKU-MON-002', 'PC Factory', 139990, 149990, 0,    TRUE,  10, 'https://pcfactory.cl/p/ultragear24'),
  ('SKU-MON-002', 'Winpy',      144990, 144990, 3990, FALSE, 7,  'https://winpy.cl/p/ultragear24'),

  -- Mouse Logitech G203 -> 3 tiendas
  ('SKU-PER-001', 'PC Factory', 15990, 17990, 0,    TRUE,  40, 'https://pcfactory.cl/p/g203'),
  ('SKU-PER-001', 'WEI',        14990, 16990, 2990, FALSE, 35, 'https://wei.cl/p/g203'),
  ('SKU-PER-001', 'Spdigital',  16490, 17990, 0,    TRUE,  25, 'https://spdigital.cl/p/g203'),

  -- Teclado Logitech K380 -> 2 tiendas
  ('SKU-PER-002', 'PC Factory', 25990, 27990, 0,    TRUE,  20, 'https://pcfactory.cl/p/k380'),
  ('SKU-PER-002', 'Spdigital',  24990, 26990, 0,    TRUE,  18, 'https://spdigital.cl/p/k380'),

  -- Audífonos Logitech G435 -> 2 tiendas
  ('SKU-PER-003', 'WEI',        34990, 39990, 2990, FALSE, 22, 'https://wei.cl/p/g435'),
  ('SKU-PER-003', 'PC Factory', 36990, 39990, 0,    TRUE,  16, 'https://pcfactory.cl/p/g435')
) AS v(sku, store_name, price, list_price, shipping_cost, shipping_free, stock, url)
JOIN product p ON p.sku = v.sku
JOIN store s ON s.name = v.store_name;

-- =====================================================================
-- HISTORIAL DE PRECIOS (bonus: alimenta el componente PriceHistory del
-- frontend) — 3 puntos de historial para la RTX 4060 en PC Factory
-- =====================================================================

INSERT INTO product_price_history (product_offer_id, price, is_promotional, recorded_at)
SELECT po.id, v.price, v.promo, NOW() - v.days_ago
FROM (VALUES
  (319990, FALSE, INTERVAL '45 days'),
  (309990, FALSE, INTERVAL '20 days'),
  (299990, TRUE,  INTERVAL '2 days')
) AS v(price, promo, days_ago)
JOIN product_offer po
  ON po.product_id = (SELECT id FROM product WHERE sku = 'SKU-GPU-001')
 AND po.store_id   = (SELECT id FROM store WHERE name = 'PC Factory');