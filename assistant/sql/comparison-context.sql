-- Consulta de referencia del contexto del asistente.
--
-- NO se ejecuta desde esta carpeta. Al conectar el backend:
--   1. portar estas consultas a backend/database/queries/
--   2. regenerar con sqlc
--   3. armar el JSON segun prompts/context-template.md
--
-- Parametro unico: $1 = arreglo de product.public_id (UUID) de los productos 
-- que el usuario esta comparando en el frontend. Maximo 5, minimo 2.
-- Solo lectura. No tocar user_account, user_session, cart_item ni el resto
-- del catalogo (ver context/allowed-fields.json).

-- ---------------------------------------------------------------------------
-- 0. Guardia: count y categorias. Debe dar product_count 2..5 y category_count = 1.
-- ---------------------------------------------------------------------------
SELECT
    count(*)::int                  AS product_count,
    count(DISTINCT p.category_id)::int AS category_count
FROM product p
WHERE p.public_id = ANY($1::uuid[])
  AND p.active;

-- ---------------------------------------------------------------------------
-- 1. Productos y mejor oferta disponible (precio + envio).
--    No se selecciona image_url ni product_url.
-- ---------------------------------------------------------------------------
SELECT
    p.public_id::text AS id,
    p.name,
    COALESCE(b.name, '') AS brand,
    COALESCE(p.model, '') AS model,
    best.total::text AS best_price,
    best.store AS best_store,
    (
        SELECT count(*)
        FROM product_offer o
        JOIN store s ON s.id = o.store_id AND s.active
        WHERE o.product_id = p.id AND o.available
    )::bigint AS offer_count
FROM product p
LEFT JOIN brand b ON b.id = p.brand_id
LEFT JOIN LATERAL (
    SELECT
        o.price + CASE WHEN o.shipping_free THEN 0 ELSE COALESCE(o.shipping_cost, 0) END AS total,
        s.name AS store
    FROM product_offer o
    JOIN store s ON s.id = o.store_id AND s.active
    WHERE o.product_id = p.id AND o.available
    ORDER BY total, o.price, o.id
    LIMIT 1
) best ON TRUE
WHERE p.public_id = ANY($1::uuid[])
  AND p.active
ORDER BY array_position($1::uuid[], p.public_id);

-- ---------------------------------------------------------------------------
-- 2. Atributos comparables (category_attributes + product_attribute_value),
--    en el orden de la categoria. Los productos sin valor quedan en NULL.
-- ---------------------------------------------------------------------------
WITH compared AS (
    SELECT p.id, p.category_ref
    FROM product p
    WHERE p.public_id = ANY($1::uuid[])
      AND p.active
), target_category AS (
    SELECT category_ref FROM compared LIMIT 1
)
SELECT
    ca.key,
    ca.label,
    ca.data_type,
    ca.unit,
    v.product_id,
    v.value
FROM category_attributes ca
JOIN target_category tc ON ca.category_id = tc.category_ref
LEFT JOIN product_attribute_value v
       ON v.attribute_id = ca.id
      AND v.product_id IN (SELECT id FROM compared)
WHERE ca.comparable
ORDER BY ca.display_order, ca.id;

-- ---------------------------------------------------------------------------
-- 3. Especificaciones anteriores comparables (product_category_specification
--    + product_specification_value), en el orden de la categoria.
-- ---------------------------------------------------------------------------
WITH compared AS (
    SELECT p.id, p.category_id
    FROM product p
    WHERE p.public_id = ANY($1::uuid[])
      AND p.active
), target_category AS (
    SELECT category_id FROM compared LIMIT 1
)
SELECT
    pcs.name,
    pcs.unit,
    psv.product_id,
    psv.value
FROM product_category_specification pcs
JOIN target_category tc ON pcs.category_id = tc.category_id
LEFT JOIN product_specification_value psv
       ON psv.specification_id = pcs.id
      AND psv.product_id IN (SELECT id FROM compared)
WHERE pcs.comparable
ORDER BY pcs.display_order, pcs.id;

-- ---------------------------------------------------------------------------
-- 4. Auditoria (fase 2): tabla nueva ai_comparison_query, migracion aparte.
--    No reutilizar ai_query_process: depende de search_query.
-- ---------------------------------------------------------------------------
