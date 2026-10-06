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


-- name: ListProducts :many
-- Sin filtros devuelve todo el catálogo activo. Con category_id, los productos
-- de esa categoría y de todas sus subcategorías; con public_id, ese producto.
WITH RECURSIVE subtree AS (
    SELECT c.id
    FROM product_category c
    WHERE c.id = sqlc.narg(category_id)::int
      AND c.active = true
    UNION ALL
    SELECT child.id
    FROM product_category child
    JOIN subtree s ON child.parent_category_id = s.id
    WHERE child.active = true
)
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
    parent.name AS parent_category_name,
    COALESCE(prs.derived_average_rating, 0) AS rating,
    COALESCE(prs.derived_review_count, 0) AS review_count
FROM product p
LEFT JOIN brand b ON b.id = p.brand_id
JOIN product_category pc ON pc.id = p.category_id
LEFT JOIN product_category parent ON parent.id = pc.parent_category_id
LEFT JOIN product_rating_summary prs ON prs.product_id = p.id
WHERE p.active = true
  AND (
      sqlc.narg(category_id)::int IS NULL
      OR p.category_id IN (SELECT id FROM subtree)
  )
  AND (sqlc.narg(public_id)::uuid IS NULL OR p.public_id = sqlc.narg(public_id))
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
WHERE po.product_id = ANY(sqlc.arg(product_ids)::int[])
ORDER BY po.product_id, po.price ASC;


-- name: ListProductImages :many
SELECT
    id,
    product_id,
    image_url,
    alt_text,
    sort_order,
    active
FROM product_image
WHERE product_id = ANY(sqlc.arg(product_ids)::int[])
  AND active = true
ORDER BY product_id, sort_order ASC, id ASC;


-- name: ListProductCategories :many
SELECT
    c.id,
    c.parent_category_id,
    c.name,
    c.description
FROM product_category c
WHERE c.active = true
ORDER BY c.parent_category_id NULLS FIRST, c.name;


-- name: ListProductReviews :many
SELECT
    r.id,
    r.rating,
    r.title,
    r.content,
    r.created_at,
    u.name AS author_name,
    u.email_verified AS author_verified
FROM product_review r
JOIN user_account u ON u.id = r.user_id
WHERE r.product_id = $1
ORDER BY r.created_at DESC, r.id DESC;


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


-- name: CreateProductImage :exec
INSERT INTO product_image (product_id, image_url, alt_text, sort_order)
VALUES ($1, $2, $3, 0);


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


-- name: UpdateProductByPublicID :one
UPDATE product
SET
    category_id = $2,
    brand_id = $3,
    name = $4,
    model = $5,
    sku = $6,
    description = $7,
    updated_at = now()
WHERE public_id = $1
  AND active = true
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


-- name: DeactivateProductByPublicID :one
UPDATE product
SET
    active = false,
    updated_at = now()
WHERE public_id = $1
  AND active = true
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


-- name: ListProductSpecifications :many
SELECT
    pcs.name,
    psv.value,
    pcs.unit
FROM product_specification_value psv
JOIN product_category_specification pcs
    ON pcs.id = psv.specification_id
WHERE psv.product_id = $1
ORDER BY pcs.display_order ASC, pcs.id ASC;


-- name: ListProductPriceHistory :many
SELECT
    pph.recorded_at,
    pph.price,
    pph.is_promotional
FROM product_price_history pph
JOIN product_offer po
    ON po.id = pph.product_offer_id
WHERE po.product_id = $1
ORDER BY pph.recorded_at ASC, pph.id ASC;
