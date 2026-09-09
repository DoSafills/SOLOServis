-- =====================================================================
-- SOLOServis - Historial de precios adicional
--
-- 003_seed.sql solo carga historial para 1 de las 37 ofertas (RTX 4060 en
-- PC Factory). Esto agrega historial real para una oferta más por cada
-- categoría, para que el gráfico de historial de precios no se vea vacío
-- en la mayoría de los productos.
-- =====================================================================

-- RTX 4070 MSI en Spdigital (Tarjetas de Video)
INSERT INTO product_price_history (product_offer_id, price, is_promotional, recorded_at)
SELECT po.id, v.price, v.promo, NOW() - v.days_ago
FROM (VALUES
  (499990, FALSE, INTERVAL '60 days'),
  (489990, FALSE, INTERVAL '30 days'),
  (469990, TRUE,  INTERVAL '5 days')
) AS v(price, promo, days_ago)
JOIN product_offer po
  ON po.product_id = (SELECT id FROM product WHERE sku = 'SKU-GPU-002')
 AND po.store_id   = (SELECT id FROM store WHERE name = 'Spdigital');

-- Ryzen 7 7800X3D en Spdigital (Procesadores)
INSERT INTO product_price_history (product_offer_id, price, is_promotional, recorded_at)
SELECT po.id, v.price, v.promo, NOW() - v.days_ago
FROM (VALUES
  (419990, FALSE, INTERVAL '50 days'),
  (399990, FALSE, INTERVAL '25 days'),
  (379990, TRUE,  INTERVAL '3 days')
) AS v(price, promo, days_ago)
JOIN product_offer po
  ON po.product_id = (SELECT id FROM product WHERE sku = 'SKU-CPU-003')
 AND po.store_id   = (SELECT id FROM store WHERE name = 'Spdigital');

-- Notebook ASUS Vivobook 15 en Spdigital (Notebooks)
INSERT INTO product_price_history (product_offer_id, price, is_promotional, recorded_at)
SELECT po.id, v.price, v.promo, NOW() - v.days_ago
FROM (VALUES
  (459990, FALSE, INTERVAL '40 days'),
  (449990, FALSE, INTERVAL '20 days'),
  (429990, TRUE,  INTERVAL '4 days')
) AS v(price, promo, days_ago)
JOIN product_offer po
  ON po.product_id = (SELECT id FROM product WHERE sku = 'SKU-NB-003')
 AND po.store_id   = (SELECT id FROM store WHERE name = 'Spdigital');

-- Monitor LG UltraGear 24" en WEI (Monitores)
INSERT INTO product_price_history (product_offer_id, price, is_promotional, recorded_at)
SELECT po.id, v.price, v.promo, NOW() - v.days_ago
FROM (VALUES
  (149990, FALSE, INTERVAL '35 days'),
  (139990, FALSE, INTERVAL '15 days'),
  (129990, TRUE,  INTERVAL '2 days')
) AS v(price, promo, days_ago)
JOIN product_offer po
  ON po.product_id = (SELECT id FROM product WHERE sku = 'SKU-MON-002')
 AND po.store_id   = (SELECT id FROM store WHERE name = 'WEI');

-- Mouse Logitech G203 en WEI (Periféricos)
INSERT INTO product_price_history (product_offer_id, price, is_promotional, recorded_at)
SELECT po.id, v.price, v.promo, NOW() - v.days_ago
FROM (VALUES
  (16990, FALSE, INTERVAL '45 days'),
  (15990, FALSE, INTERVAL '10 days'),
  (14990, TRUE,  INTERVAL '1 day')
) AS v(price, promo, days_ago)
JOIN product_offer po
  ON po.product_id = (SELECT id FROM product WHERE sku = 'SKU-PER-001')
 AND po.store_id   = (SELECT id FROM store WHERE name = 'WEI');
