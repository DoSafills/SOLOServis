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
GROUP BY
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
    s.updated_at
ORDER BY s.id;

-- name: GetStoreByID :one
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
LEFT JOIN product p
    ON p.id = po.product_id
    AND p.active = true
WHERE s.id = $1
  AND s.active = true
GROUP BY
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
    s.updated_at;


-- name: ListStoreProducts :many
SELECT
    p.id,
    p.public_id,
    p.category_id,
    p.brand_id,
    p.name,
    p.model,
    p.sku,
    p.description,
    p.active,
    p.created_at,
    p.updated_at,
    b.name AS brand_name,
    pc.name AS category_name,
    po.id AS offer_id,
    po.price,
    po.list_price,
    po.currency,
    po.shipping_cost,
    po.shipping_free,
    po.available,
    po.stock,
    po.condition,
    po.product_url,
    po.last_updated
FROM product_offer po
JOIN product p
    ON p.id = po.product_id
LEFT JOIN brand b
    ON b.id = p.brand_id
JOIN product_category pc
    ON pc.id = p.category_id
WHERE po.store_id = $1
  AND po.available = true
  AND p.active = true
ORDER BY p.id;


-- name: ListStoreLocations :many
SELECT
    sl.id,
    sl.store_id,
    sl.location_id,
    sl.address,
    sl.postal_code,
    sl.active,
    l.country,
    l.region,
    l.city,
    l.commune
FROM store_location sl
JOIN location l
    ON l.id = sl.location_id
WHERE sl.store_id = $1
  AND sl.active = true
ORDER BY sl.id;
