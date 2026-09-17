-- =====================================================================
-- SOLOServis - Datos adicionales para tiendas y servicios
-- Este seed es repetible y no duplica registros existentes.
-- =====================================================================

BEGIN;

-- TIENDAS
INSERT INTO store (name, website_url, logo_url, rating, reputation, shipping_information, general_conditions)
SELECT 'Ripley', 'https://ripley.cl', 'https://cdn.example.com/stores/ripley.png', 4.10,
       'Buena', 'Envío en 3-6 días hábiles', 'Garantía legal y devolución en 30 días'
WHERE NOT EXISTS (SELECT 1 FROM store WHERE name = 'Ripley');

INSERT INTO store (name, website_url, logo_url, rating, reputation, shipping_information, general_conditions)
SELECT 'La Polar', 'https://lapolar.cl', 'https://cdn.example.com/stores/lapolar.png', 3.90,
       'Regular', 'Envío en 5-8 días hábiles', 'Garantía legal y devolución según condiciones'
WHERE NOT EXISTS (SELECT 1 FROM store WHERE name = 'La Polar');

-- OFERTAS ADICIONALES DE PRODUCTOS
INSERT INTO product_offer (product_id, store_id, price, list_price, currency, shipping_cost, shipping_free, available, stock, condition, product_url)
SELECT p.id, s.id, 539990, 579990, 'CLP', 0, TRUE, TRUE, 10, 'new', 'https://ripley.cl/producto/refrigerador-samsung'
FROM product p
JOIN store s ON s.name = 'Ripley'
WHERE p.sku = 'SKU-SAMS-001'
  AND NOT EXISTS (
      SELECT 1 FROM product_offer po WHERE po.product_id = p.id AND po.store_id = s.id
  );

INSERT INTO product_offer (product_id, store_id, price, list_price, currency, shipping_cost, shipping_free, available, stock, condition, product_url)
SELECT p.id, s.id, 879990, 929990, 'CLP', 4990, FALSE, TRUE, 4, 'new', 'https://lapolar.cl/producto/refrigerador-lg'
FROM product p
JOIN store s ON s.name = 'La Polar'
WHERE p.sku = 'SKU-LG-001'
  AND NOT EXISTS (
      SELECT 1 FROM product_offer po WHERE po.product_id = p.id AND po.store_id = s.id
  );

-- CATEGORIA ADICIONAL DE SERVICIOS
INSERT INTO service_category (service_type_id, parent_category_id, name, description)
SELECT sc.service_type_id, NULL, 'Telefonia Movil', 'Planes moviles de voz y datos'
FROM (
    SELECT service_type_id
    FROM service_category
    WHERE name LIKE 'Internet%'
    ORDER BY id
    LIMIT 1
) sc
WHERE NOT EXISTS (SELECT 1 FROM service_category WHERE name = 'Telefonia Movil');

-- PROVEEDORES
INSERT INTO provider (name, website_url, logo_url, rating, reputation, general_conditions)
SELECT 'WOM', 'https://wom.cl', 'https://cdn.example.com/providers/wom.png', 4.00,
       'Buena', 'Sin permanencia en planes seleccionados'
WHERE NOT EXISTS (SELECT 1 FROM provider WHERE name = 'WOM');

INSERT INTO provider (name, website_url, logo_url, rating, reputation, general_conditions)
SELECT 'Claro', 'https://claro.cl', 'https://cdn.example.com/providers/claro.png', 3.90,
       'Buena', 'Contrato sujeto a la oferta contratada'
WHERE NOT EXISTS (SELECT 1 FROM provider WHERE name = 'Claro');

-- SERVICIOS
INSERT INTO service (category_id, name, description, image_url)
SELECT sc.id, 'Plan Movil 100 GB',
       'Plan movil con 100 GB, llamadas ilimitadas y cobertura 5G.',
       'https://cdn.example.com/services/movil-100gb.jpg'
FROM (SELECT id FROM service_category WHERE name = 'Telefonia Movil' ORDER BY id LIMIT 1) sc
WHERE NOT EXISTS (SELECT 1 FROM service WHERE name = 'Plan Movil 100 GB');

INSERT INTO service (category_id, name, description, image_url)
SELECT sc.id, 'Plan Movil 200 GB',
       'Plan movil con 200 GB, llamadas ilimitadas y roaming regional.',
       'https://cdn.example.com/services/movil-200gb.jpg'
FROM (SELECT id FROM service_category WHERE name = 'Telefonia Movil' ORDER BY id LIMIT 1) sc
WHERE NOT EXISTS (SELECT 1 FROM service WHERE name = 'Plan Movil 200 GB');

-- OFERTAS DE SERVICIOS
INSERT INTO service_offer (service_id, provider_id, price, currency, billing_period, installation_cost, contract_period, available, coverage_summary, service_url)
SELECT s.id, p.id, 14990, 'CLP', 'monthly', 0, 'Sin permanencia', TRUE,
       'Cobertura 5G en las principales ciudades de Chile', 'https://wom.cl/planes/movil-100gb'
FROM service s
JOIN provider p ON p.name = 'WOM'
WHERE s.name = 'Plan Movil 100 GB'
  AND NOT EXISTS (
      SELECT 1 FROM service_offer so WHERE so.service_id = s.id AND so.provider_id = p.id
  );

INSERT INTO service_offer (service_id, provider_id, price, currency, billing_period, installation_cost, contract_period, available, coverage_summary, service_url)
SELECT s.id, p.id, 19990, 'CLP', 'monthly', 0, '12 meses', TRUE,
       'Cobertura nacional en zonas habilitadas', 'https://claro.cl/planes/movil-200gb'
FROM service s
JOIN provider p ON p.name = 'Claro'
WHERE s.name = 'Plan Movil 200 GB'
  AND NOT EXISTS (
      SELECT 1 FROM service_offer so WHERE so.service_id = s.id AND so.provider_id = p.id
  );

COMMIT;