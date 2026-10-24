\echo 'Q01: URLs repetidas por tienda (store_id, product_url, cantidad)'
BEGIN READ ONLY;
SELECT store_id, product_url, COUNT(*) AS cantidad FROM product_offer
WHERE product_url IS NOT NULL AND BTRIM(product_url) <> ''
GROUP BY store_id, product_url HAVING COUNT(*) > 1 ORDER BY store_id, product_url;
ROLLBACK;

\echo 'Q02: Ofertas con URL NULL o vacia, total y por tienda'
BEGIN READ ONLY;
SELECT CASE WHEN GROUPING(s.id)=1 THEN 'TOTAL' ELSE s.id::text END AS store_id,
       CASE WHEN GROUPING(s.name)=1 THEN 'Todas las tiendas' ELSE s.name END AS tienda,
       COUNT(*) AS ofertas_sin_url
FROM product_offer po JOIN store s ON s.id=po.store_id
WHERE po.product_url IS NULL OR BTRIM(po.product_url)=''
GROUP BY GROUPING SETS ((s.id,s.name),()) ORDER BY GROUPING(s.id), s.id;
ROLLBACK;

\echo 'Q03: URLs con espacios al inicio o final'
BEGIN READ ONLY;
SELECT store_id, COUNT(*) AS cantidad FROM product_offer
WHERE product_url IS NOT NULL AND product_url <> BTRIM(product_url)
GROUP BY store_id ORDER BY store_id;
ROLLBACK;

\echo 'Q04: Conteo de URLs con parámetros de seguimiento y barra final'
BEGIN READ ONLY;
SELECT COUNT(*) FILTER (WHERE product_url ~* '(utm_[^=/?&]+|gclid|fbclid|ref=)') AS con_parametros_seguimiento,
       COUNT(*) FILTER (WHERE product_url ~ '/$') AS con_barra_final,
       COUNT(*) FILTER (WHERE product_url ~* '(utm_[^=/?&]+|gclid|fbclid|ref=)' AND product_url ~ '/$') AS con_ambos
FROM product_offer WHERE product_url IS NOT NULL;
ROLLBACK;

\echo 'Q05: Misma URL usada en distintas tiendas'
BEGIN READ ONLY;
SELECT product_url, COUNT(DISTINCT store_id) AS tiendas, COUNT(*) AS ofertas
FROM product_offer WHERE product_url IS NOT NULL AND BTRIM(product_url) <> ''
GROUP BY product_url HAVING COUNT(DISTINCT store_id) > 1 ORDER BY tiendas DESC, product_url;
ROLLBACK;

\echo 'Q06: Tiendas con nombre repetido ignorando mayusculas y espacios exteriores'
BEGIN READ ONLY;
SELECT LOWER(BTRIM(name)) AS nombre_normalizado, COUNT(*) AS tiendas,
       ARRAY_AGG(id ORDER BY id) AS store_ids, ARRAY_AGG(name ORDER BY id) AS nombres
FROM store GROUP BY LOWER(BTRIM(name)) HAVING COUNT(*) > 1 ORDER BY nombre_normalizado;
ROLLBACK;

\echo 'Q07: Tiendas con el mismo dominio de website_url'
BEGIN READ ONLY;
WITH domains AS (
  SELECT id, name,
         REGEXP_REPLACE(LOWER(SPLIT_PART(SPLIT_PART(REGEXP_REPLACE(BTRIM(website_url), '^[a-zA-Z][a-zA-Z0-9+.-]*://', ''), '/', 1), ':', 1)), '^www[.]', '') AS domain
  FROM store WHERE website_url IS NOT NULL AND BTRIM(website_url) <> ''
)
SELECT domain, COUNT(*) AS tiendas, ARRAY_AGG(id ORDER BY id) AS store_ids,
       ARRAY_AGG(name ORDER BY id) AS nombres
FROM domains GROUP BY domain HAVING COUNT(*) > 1 ORDER BY domain;
ROLLBACK;

\echo 'Q08: Categorias con nombre repetido bajo el mismo parent_category_id'
BEGIN READ ONLY;
SELECT parent_category_id, LOWER(BTRIM(name)) AS nombre_normalizado,
       COUNT(*) AS categorias, ARRAY_AGG(id ORDER BY id) AS category_ids
FROM product_category GROUP BY parent_category_id, LOWER(BTRIM(name))
HAVING COUNT(*) > 1 ORDER BY parent_category_id NULLS FIRST, nombre_normalizado;
ROLLBACK;

\echo 'Q09: Especificaciones repetidas por categoria y nombre'
BEGIN READ ONLY;
SELECT category_id, LOWER(BTRIM(name)) AS nombre_normalizado,
       COUNT(*) AS especificaciones, ARRAY_AGG(id ORDER BY id) AS specification_ids
FROM product_category_specification GROUP BY category_id, LOWER(BTRIM(name))
HAVING COUNT(*) > 1 ORDER BY category_id, nombre_normalizado;
ROLLBACK;

\echo 'Q10a: Productos con sku NULL'
BEGIN READ ONLY;
SELECT COUNT(*) AS productos_con_sku_null FROM product WHERE sku IS NULL;
ROLLBACK;

\echo 'Q10b: Productos con name + model repetidos'
BEGIN READ ONLY;
SELECT LOWER(BTRIM(name)) AS nombre_normalizado, LOWER(BTRIM(model)) AS modelo_normalizado,
       COUNT(*) AS productos, ARRAY_AGG(id ORDER BY id) AS product_ids
FROM product WHERE model IS NOT NULL AND BTRIM(model) <> ''
GROUP BY LOWER(BTRIM(name)), LOWER(BTRIM(model)) HAVING COUNT(*) > 1
ORDER BY nombre_normalizado, modelo_normalizado;
ROLLBACK;

\echo 'Q11: Valores de especificacion cuya categoria difiere de la categoria del producto'
BEGIN READ ONLY;
SELECT psv.id AS value_id, psv.product_id, p.category_id AS product_category_id,
       psv.specification_id, pcs.category_id AS specification_category_id, psv.value
FROM product_specification_value psv
JOIN product p ON p.id=psv.product_id
JOIN product_category_specification pcs ON pcs.id=psv.specification_id
WHERE pcs.category_id <> p.category_id ORDER BY psv.id;
ROLLBACK;

\echo 'Q12: Valores incompatibles con data_type number o boolean usando patrones sin casts'
BEGIN READ ONLY;
SELECT psv.id AS value_id, psv.product_id, pcs.category_id, pcs.name AS specification_name,
       pcs.data_type, psv.value
FROM product_specification_value psv
JOIN product_category_specification pcs ON pcs.id=psv.specification_id
WHERE (LOWER(BTRIM(pcs.data_type))='number'
       AND BTRIM(psv.value) !~ '^[+-]?([0-9]+([.][0-9]*)?|[.][0-9]+)$')
   OR (LOWER(BTRIM(pcs.data_type))='boolean'
       AND LOWER(BTRIM(psv.value)) !~ '^(true|false|t|f|yes|no|y|n|1|0)$')
ORDER BY psv.id;
ROLLBACK;

\echo 'Q13: Ofertas sin ninguna fila de historial'
BEGIN READ ONLY;
SELECT po.id AS offer_id, po.product_id, po.store_id, po.price
FROM product_offer po WHERE NOT EXISTS
  (SELECT 1 FROM product_price_history pph WHERE pph.product_offer_id=po.id)
ORDER BY po.id;
ROLLBACK;

\echo 'Q14: Ofertas con multiples historiales y ofertas con precios consecutivos iguales'
BEGIN READ ONLY;
WITH ordered_history AS (
  SELECT product_offer_id, price,
         LAG(price) OVER (PARTITION BY product_offer_id ORDER BY recorded_at, id) AS previous_price
  FROM product_price_history
), per_offer AS (
  SELECT product_offer_id, COUNT(*) AS history_rows,
         COUNT(*) FILTER (WHERE price=previous_price) AS consecutive_equal_price_pairs
  FROM ordered_history GROUP BY product_offer_id
)
SELECT COUNT(*) FILTER (WHERE history_rows > 1) AS ofertas_con_multiples_filas,
       COUNT(*) FILTER (WHERE history_rows > 1 AND consecutive_equal_price_pairs > 0) AS ofertas_consecutivas_mismo_precio,
       COALESCE(SUM(consecutive_equal_price_pairs),0) AS pares_consecutivos_mismo_precio
FROM per_offer;
ROLLBACK;

\echo 'Q15: Ofertas cuyo ultimo precio historico difiere del precio actual'
BEGIN READ ONLY;
WITH latest AS (
  SELECT DISTINCT ON (product_offer_id) product_offer_id, price
  FROM product_price_history ORDER BY product_offer_id, recorded_at DESC, id DESC
)
SELECT po.id AS offer_id, po.price AS precio_actual, latest.price AS ultimo_precio_historico
FROM product_offer po JOIN latest ON latest.product_offer_id=po.id
WHERE latest.price <> po.price ORDER BY po.id;
ROLLBACK;

\echo 'Q16a: Conteo de ofertas por currency'
BEGIN READ ONLY;
SELECT currency, COUNT(*) AS ofertas FROM product_offer GROUP BY currency ORDER BY currency;
ROLLBACK;

\echo 'Q16b: Conteo de ofertas por condition'
BEGIN READ ONLY;
SELECT condition, COUNT(*) AS ofertas FROM product_offer GROUP BY condition ORDER BY condition;
ROLLBACK;

\echo 'Q17: Ofertas con list_price menor que price'
BEGIN READ ONLY;
SELECT id AS offer_id, product_id, store_id, list_price, price
FROM product_offer WHERE list_price < price ORDER BY id;
ROLLBACK;

\echo 'Q18: Ofertas no disponibles o con stock cero'
BEGIN READ ONLY;
SELECT id AS offer_id, product_id, store_id, available, stock
FROM product_offer WHERE available=FALSE OR stock=0 ORDER BY id;
ROLLBACK;

\echo 'Q19: Conteo de filas por tablas auditadas'
BEGIN READ ONLY;
SELECT 'product' AS tabla, COUNT(*) AS filas FROM product
UNION ALL SELECT 'product_offer', COUNT(*) FROM product_offer
UNION ALL SELECT 'store', COUNT(*) FROM store
UNION ALL SELECT 'product_price_history', COUNT(*) FROM product_price_history
UNION ALL SELECT 'product_specification_value', COUNT(*) FROM product_specification_value
ORDER BY tabla;
ROLLBACK;

\echo 'Q20: Ofertas que excederian UNIQUE(store_id, product_url); NULL no colisiona en UNIQUE normal'
BEGIN READ ONLY;
WITH duplicates AS (
  SELECT store_id, product_url, COUNT(*) AS n
  FROM product_offer WHERE product_url IS NOT NULL
  GROUP BY store_id, product_url HAVING COUNT(*) > 1
)
SELECT COUNT(*) AS grupos_duplicados,
       COALESCE(SUM(n),0) AS filas_en_grupos_duplicados,
       COALESCE(SUM(n-1),0) AS filas_que_exceden_una_por_clave
FROM duplicates;
ROLLBACK;


