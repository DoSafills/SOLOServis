-- =====================================================================
-- SOLOServis - Datos de prueba
-- Se insertan datos mínimos y coherentes para validar relaciones,
-- claves foráneas y restricciones definidas en 001_schema.sql
-- =====================================================================

-- USUARIOS
INSERT INTO user_account (role, name, email, password_hash) VALUES
('admin',  'Ricardo Rios',   'ricardo.rios@soloservis.cl',   'hash_demo_1'),
('client', 'Camila Fuentes', 'camila.fuentes@example.com',   'hash_demo_2'),
('client', 'Jorge Muñoz',    'jorge.munoz@example.com',      'hash_demo_3');

-- MARCAS
INSERT INTO brand (name, logo_url, website_url) VALUES
('Samsung', 'https://cdn.example.com/logos/samsung.png', 'https://samsung.com'),
('LG',      'https://cdn.example.com/logos/lg.png',      'https://lg.com');

-- CATEGORÍAS DE PRODUCTO (con jerarquía padre/hijo)
INSERT INTO product_category (parent_category_id, name, description) VALUES
(NULL, 'Electrodomésticos', 'Categoría raíz de electrodomésticos');

INSERT INTO product_category (parent_category_id, name, description) VALUES
(1, 'Refrigeradores', 'Refrigeradores y freezers');

-- ESPECIFICACIONES DE CATEGORÍA DE PRODUCTO
INSERT INTO product_category_specification (category_id, name, data_type, unit, required, comparable, display_order) VALUES
(2, 'Capacidad', 'number', 'litros', TRUE, TRUE, 1),
(2, 'Eficiencia energética', 'string', NULL, FALSE, TRUE, 2);

-- PRODUCTOS
INSERT INTO product (category_id, brand_id, name, model, sku, description) VALUES
(2, 1, 'Refrigerador Samsung No Frost', 'RT38K', 'SKU-SAMS-001', 'Refrigerador no frost de 380 litros'),
(2, 2, 'Refrigerador LG Side by Side',  'GS65',  'SKU-LG-001',   'Refrigerador side by side de 550 litros');

INSERT INTO product_image (product_id, image_url, alt_text, sort_order) VALUES
(1, 'https://cdn.example.com/products/1/main.jpg', 'Refrigerador Samsung frontal', 0),
(2, 'https://cdn.example.com/products/2/main.jpg', 'Refrigerador LG frontal', 0);

INSERT INTO product_specification_value (product_id, specification_id, value) VALUES
(1, 1, '380'),
(1, 2, 'A++'),
(2, 1, '550'),
(2, 2, 'A+++');

-- TIENDAS
INSERT INTO store (name, website_url, logo_url, rating, reputation, shipping_information, general_conditions) VALUES
('Falabella', 'https://falabella.com', 'https://cdn.example.com/stores/falabella.png', 4.20, 'Buena', 'Envío en 3-5 días hábiles', 'Garantía de 12 meses'),
('Paris',     'https://paris.cl',      'https://cdn.example.com/stores/paris.png',      4.00, 'Buena', 'Envío en 5-7 días hábiles', 'Garantía de 12 meses');

-- OFERTAS DE PRODUCTO
INSERT INTO product_offer (product_id, store_id, price, list_price, currency, shipping_cost, shipping_free, available, stock, condition, product_url) VALUES
(1, 1, 549990, 599990, 'CLP', 0,    TRUE,  TRUE, 15, 'new', 'https://falabella.com/producto/1'),
(1, 2, 559990, 559990, 'CLP', 5990, FALSE, TRUE, 8,  'new', 'https://paris.cl/producto/1'),
(2, 1, 899990, 949990, 'CLP', 0,    TRUE,  TRUE, 5,  'new', 'https://falabella.com/producto/2');

-- HISTORIAL DE PRECIOS
INSERT INTO product_price_history (product_offer_id, price, is_promotional, recorded_at) VALUES
(1, 599990, FALSE, NOW() - INTERVAL '30 days'),
(1, 549990, TRUE,  NOW() - INTERVAL '2 days'),
(3, 949990, FALSE, NOW() - INTERVAL '20 days'),
(3, 899990, TRUE,  NOW() - INTERVAL '1 days');

-- COMPARACIONES DE PRODUCTOS
INSERT INTO product_comparison (user_id, category_id, name) VALUES
(2, 2, 'Comparando refrigeradores para depto nuevo');

INSERT INTO product_comparison_item (comparison_id, product_id, position) VALUES
(1, 1, 0),
(1, 2, 1);

-- INTERÉS DE USUARIO EN PRODUCTOS (favoritos / alertas de precio)
INSERT INTO product_user_interest (user_id, product_id, target_price, notify_price_drop, is_favorite) VALUES
(2, 1, 500000, TRUE, TRUE),
(3, 2, 850000, TRUE, FALSE);

-- RESEÑAS DE PRODUCTOS
INSERT INTO product_review (user_id, product_id, rating, title, content, verified) VALUES
(2, 1, 5, 'Excelente refrigerador', 'Muy silencioso y espacioso, lo recomiendo.', TRUE),
(3, 1, 4, 'Buena relación precio-calidad', 'Cumple lo esperado, buen enfriamiento.', FALSE);

-- =====================================================================
-- SERVICIOS
-- =====================================================================

INSERT INTO service_type (name) VALUES
('instalación'),
('suscripción'),
('pago_único'),
('consultoría');

INSERT INTO service_category (service_type_id, parent_category_id, name, description) VALUES
(2, NULL, 'Internet y Telefonía', 'Servicios de conectividad para el hogar');

INSERT INTO service_category (service_type_id, parent_category_id, name, description) VALUES
(2, 1, 'Internet Fibra Óptica', 'Planes de internet por fibra');

INSERT INTO service_category_specification (category_id, name, data_type, unit, required, comparable, display_order) VALUES
(2, 'Velocidad de bajada', 'number', 'Mbps', TRUE, TRUE, 1),
(2, 'Velocidad de subida', 'number', 'Mbps', TRUE, TRUE, 2);

INSERT INTO service (category_id, name, description, image_url) VALUES
(2, 'Plan Fibra 400 Megas', 'Plan hogar de internet por fibra óptica simétrica', 'https://cdn.example.com/services/1/main.jpg'),
(2, 'Plan Fibra 600 Megas', 'Plan hogar de internet por fibra óptica simétrica', 'https://cdn.example.com/services/2/main.jpg');

INSERT INTO service_specification_value (service_id, specification_id, value) VALUES
(1, 1, '400'),
(1, 2, '400'),
(2, 1, '600'),
(2, 2, '600');

-- PROVEEDORES
INSERT INTO provider (name, website_url, logo_url, rating, reputation, general_conditions) VALUES
('Movistar', 'https://movistar.cl', 'https://cdn.example.com/providers/movistar.png', 3.80, 'Regular', 'Contrato mínimo de 12 meses'),
('Entel',    'https://entel.cl',    'https://cdn.example.com/providers/entel.png',    4.10, 'Buena',   'Sin permanencia');

-- OFERTAS DE SERVICIO
INSERT INTO service_offer (service_id, provider_id, price, currency, billing_period, installation_cost, contract_period, available, coverage_summary, service_url) VALUES
(1, 1, 24990, 'CLP', 'monthly', 0,     '12 meses', TRUE, 'Disponible en zonas urbanas de la RM', 'https://movistar.cl/planes/fibra-400'),
(1, 2, 22990, 'CLP', 'monthly', 15000, 'Sin permanencia', TRUE, 'Disponible en gran parte de Chile', 'https://entel.cl/planes/fibra-400'),
(2, 2, 29990, 'CLP', 'monthly', 0,     'Sin permanencia', TRUE, 'Disponible en gran parte de Chile', 'https://entel.cl/planes/fibra-600');

-- HISTORIAL DE PRECIOS DE SERVICIOS
INSERT INTO service_price_history (service_offer_id, price, is_promotional, recorded_at) VALUES
(1, 26990, FALSE, NOW() - INTERVAL '60 days'),
(1, 24990, TRUE,  NOW() - INTERVAL '3 days');

-- COMPARACIONES DE SERVICIOS
INSERT INTO service_comparison (user_id, category_id, name) VALUES
(3, 2, 'Comparando planes de internet');

INSERT INTO service_comparison_item (comparison_id, service_id, position) VALUES
(1, 1, 0),
(1, 2, 1);

-- INTERÉS DE USUARIO EN SERVICIOS
INSERT INTO service_user_interest (user_id, service_id, target_price, notify_price_drop, is_favorite) VALUES
(3, 1, 20000, TRUE, TRUE);

-- RESEÑAS DE SERVICIOS
INSERT INTO service_review (user_id, service_id, rating, title, content, verified) VALUES
(3, 1, 3, 'Cumple pero con caídas', 'La velocidad es buena pero ha tenido cortes.', TRUE);

-- =====================================================================
-- UBICACIONES
-- =====================================================================

INSERT INTO location (country, region, city, commune, latitude, longitude) VALUES
('Chile', 'Metropolitana', 'Santiago', 'Providencia', -33.426, -70.610),
('Chile', 'Araucanía',     'Temuco',   'Temuco',      -38.739, -72.598);

INSERT INTO store_location (store_id, location_id, address, postal_code) VALUES
(1, 1, 'Av. Providencia 1234', '7500000'),
(2, 2, 'Av. Alemania 567',     '4780000');

INSERT INTO provider_location (provider_id, location_id, address, postal_code, service_radius) VALUES
(1, 1, 'Av. Apoquindo 4400', '7550000', 'Región Metropolitana'),
(2, 2, 'Manuel Montt 890',   '4781000', 'Región de la Araucanía');

-- =====================================================================
-- BÚSQUEDA E IA
-- =====================================================================

INSERT INTO search_query (user_id, raw_query, search_type) VALUES
(2, 'refrigerador no frost barato', 'product'),
(3, 'mejor plan de internet fibra', 'service');

INSERT INTO search_query_filter (search_query_id, filter_name, filter_value) VALUES
(1, 'category', 'Refrigeradores'),
(1, 'max_price', '600000'),
(2, 'category', 'Internet Fibra Óptica');

INSERT INTO ai_query_process (search_query_id, intent, entity_type, structured_json, response_time_ms, success) VALUES
(1, 'buscar_producto', 'product', '{"category": "Refrigeradores", "max_price": 600000}', 120, TRUE),
(2, 'buscar_servicio', 'service', '{"category": "Internet Fibra Óptica"}', 95, TRUE);

-- =====================================================================
-- SCRAPING
-- =====================================================================

INSERT INTO data_source (name, base_url, source_type) VALUES
('Falabella Scraper', 'https://falabella.com', 'store'),
('Movistar Scraper',  'https://movistar.cl',   'provider');

INSERT INTO scraper_config (data_source_id, scraper_type, target_type, schedule, rate_limit) VALUES
(1, 'scrapy',      'product', '0 3 * * *', '1 req/s'),
(2, 'playwright',  'service', '0 4 * * *', '1 req/2s');

INSERT INTO scrape_run (scraper_config_id, started_at, finished_at, records_found, records_processed, records_failed, status) VALUES
(1, NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day' + INTERVAL '10 minutes', 120, 118, 2, 'success'),
(2, NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day' + INTERVAL '5 minutes',  40,  40,  0, 'success');

INSERT INTO scraped_data (scrape_run_id, external_id, external_url, entity_type, raw_data) VALUES
(1, 'FAL-12345', 'https://falabella.com/producto/12345', 'product', '{"name": "Refrigerador Samsung", "price": 549990}'),
(2, 'MOV-PLAN-400', 'https://movistar.cl/planes/fibra-400', 'service', '{"name": "Plan Fibra 400", "price": 24990}');