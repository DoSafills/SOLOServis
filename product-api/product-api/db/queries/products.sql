-- name: ListProducts :many
-- Lista productos con paginación y precio mínimo/máximo entre tiendas.
SELECT
    p.id,
    p.public_id,
    p.name,
    p.model,
    p.sku,
    p.description,
    b.name AS brand_name,
    c.name AS category_name,
    MIN(po.price) FILTER (WHERE po.available)::numeric AS min_price,
    MAX(po.price) FILTER (WHERE po.available)::numeric AS max_price,
    COUNT(po.id) FILTER (WHERE po.available) AS offer_count
FROM product p
LEFT JOIN brand b ON b.id = p.brand_id
LEFT JOIN product_category c ON c.id = p.category_id
LEFT JOIN product_offer po ON po.product_id = p.id
WHERE p.active = TRUE
  AND (sqlc.narg('category_id')::bigint IS NULL OR p.category_id = sqlc.narg('category_id'))
  AND (sqlc.narg('search')::text IS NULL OR p.name ILIKE '%' || sqlc.narg('search') || '%')
GROUP BY p.id, b.name, c.name
ORDER BY p.id
LIMIT $1 OFFSET $2;

-- name: CountProducts :one
SELECT COUNT(*)
FROM product p
WHERE p.active = TRUE
  AND (sqlc.narg('category_id')::bigint IS NULL OR p.category_id = sqlc.narg('category_id'))
  AND (sqlc.narg('search')::text IS NULL OR p.name ILIKE '%' || sqlc.narg('search') || '%');

-- name: GetProductByID :one
SELECT
    p.id,
    p.public_id,
    p.name,
    p.model,
    p.sku,
    p.description,
    b.name AS brand_name,
    c.name AS category_name
FROM product p
LEFT JOIN brand b ON b.id = p.brand_id
LEFT JOIN product_category c ON c.id = p.category_id
WHERE p.id = $1 AND p.active = TRUE;

-- name: ListOffersByProductID :many
-- Precios del mismo producto en las distintas tiendas.
SELECT
    po.id,
    po.store_id,
    s.name AS store_name,
    s.logo_url AS store_logo_url,
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
JOIN store s ON s.id = po.store_id
WHERE po.product_id = $1 AND s.active = TRUE
ORDER BY po.price ASC;
