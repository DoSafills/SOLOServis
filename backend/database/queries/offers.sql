-- name: ListOffers :many
SELECT
    po.id,
    po.product_id,
    p.public_id AS product_public_id,
    p.name AS product_name,
    po.store_id,
    s.name AS store_name,
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
JOIN product p ON p.id = po.product_id
JOIN store s ON s.id = po.store_id
ORDER BY po.id;


-- name: GetOfferByID :one
SELECT
    po.id,
    po.product_id,
    p.public_id AS product_public_id,
    p.name AS product_name,
    po.store_id,
    s.name AS store_name,
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
JOIN product p ON p.id = po.product_id
JOIN store s ON s.id = po.store_id
WHERE po.id = $1;
