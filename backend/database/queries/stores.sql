-- name: ListStores :many
SELECT
    s.id,
    s.name,
    s.website_url,
    s.logo_url,
    s.rating,
    s.reputation,
    s.shipping_information,
    s.general_conditions,
    s.active,
    s.created_at,
    s.updated_at,
    COUNT(DISTINCT po.product_id) AS product_count
FROM store s
LEFT JOIN product_offer po
    ON po.store_id = s.id
    AND po.available = true
WHERE s.active = true
GROUP BY s.id
ORDER BY s.id;


-- name: GetStore :one
SELECT
    s.id,
    s.name,
    s.website_url,
    s.logo_url,
    s.rating,
    s.reputation,
    s.shipping_information,
    s.general_conditions,
    s.active,
    s.created_at,
    s.updated_at,
    COUNT(DISTINCT po.product_id) AS product_count
FROM store s
LEFT JOIN product_offer po
    ON po.store_id = s.id
    AND po.available = true
WHERE s.id = $1
  AND s.active = true
GROUP BY s.id;
