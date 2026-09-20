-- name: GetProductByID :one
SELECT
    id,
    public_id,
    category_id,
    brand_id,
    name,
    model,
    sku,
    description,
    active,
    created_at,
    updated_at
FROM product
WHERE id = $1;


-- name: GetProductByPublicID :one
SELECT
    id,
    public_id,
    category_id,
    brand_id,
    name,
    model,
    sku,
    description,
    active,
    created_at,
    updated_at
FROM product
WHERE public_id = $1;



-- name: GetProductDetailByPublicID :one
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
    COALESCE(prs.derived_average_rating, 0) AS rating,
    COALESCE(prs.derived_review_count, 0) AS review_count
FROM product p
LEFT JOIN brand b ON b.id = p.brand_id
JOIN product_category pc ON pc.id = p.category_id
LEFT JOIN product_rating_summary prs ON prs.product_id = p.id
WHERE p.public_id = $1;


-- name: ListProducts :many
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
    COALESCE(prs.derived_average_rating, 0) AS rating,
    COALESCE(prs.derived_review_count, 0) AS review_count
FROM product p
LEFT JOIN brand b ON b.id = p.brand_id
JOIN product_category pc ON pc.id = p.category_id
LEFT JOIN product_rating_summary prs ON prs.product_id = p.id
WHERE p.active = true
ORDER BY p.id;


-- name: ListProductOffers :many
SELECT
    po.id,
    po.product_id,
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
JOIN store s ON s.id = po.store_id
WHERE po.product_id = $1
ORDER BY po.price ASC;


-- name: ListProductImages :many
SELECT
    id,
    product_id,
    image_url,
    alt_text,
    sort_order,
    active
FROM product_image
WHERE product_id = $1
  AND active = true
ORDER BY sort_order ASC, id ASC;


-- name: CreateProduct :one
INSERT INTO product (
    category_id,
    brand_id,
    name,
    model,
    sku,
    description
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING
    id,
    public_id,
    category_id,
    brand_id,
    name,
    model,
    sku,
    description,
    active,
    created_at,
    updated_at;


-- name: UpdateProduct :one
UPDATE product
SET
    category_id = $2,
    brand_id = $3,
    name = $4,
    model = $5,
    sku = $6,
    description = $7,
    updated_at = now()
WHERE id = $1
RETURNING
    id,
    public_id,
    category_id,
    brand_id,
    name,
    model,
    sku,
    description,
    active,
    created_at,
    updated_at;


-- name: DeactivateProduct :one
UPDATE product
SET
    active = false,
    updated_at = now()
WHERE id = $1
RETURNING
    id,
    public_id,
    category_id,
    brand_id,
    name,
    model,
    sku,
    description,
    active,
    created_at,
    updated_at;