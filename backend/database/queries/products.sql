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
        COUNT(po.id)::BIGINT AS product_count
FROM store s
LEFT JOIN product_offer po ON po.store_id = s.id
WHERE s.active = TRUE
GROUP BY s.id
ORDER BY s.name;


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
        COUNT(po.id)::BIGINT AS product_count
FROM store s
LEFT JOIN product_offer po ON po.store_id = s.id
WHERE s.id = $1
    AND s.active = TRUE
GROUP BY s.id;


-- name: ListServices :many
SELECT
        s.id,
        s.public_id,
        s.name,
        s.description,
        s.image_url,
        sc.name AS category_name,
        COALESCE(srs.derived_average_rating, 0) AS rating,
        COALESCE(srs.derived_review_count, 0) AS review_count,
        MIN(so.price) FILTER (WHERE so.available = TRUE) AS starting_price
FROM service s
JOIN service_category sc ON sc.id = s.category_id
LEFT JOIN service_rating_summary srs ON srs.service_id = s.id
LEFT JOIN service_offer so ON so.service_id = s.id
WHERE s.active = TRUE
    AND (@category::text IS NULL OR sc.name = @category)
GROUP BY s.id, sc.name, srs.derived_average_rating, srs.derived_review_count
ORDER BY s.name;


-- name: GetServiceDetail :one
SELECT
        s.id,
        s.public_id,
        s.name,
        s.description,
        s.image_url,
        sc.name AS category_name,
        COALESCE(srs.derived_average_rating, 0) AS rating,
        COALESCE(srs.derived_review_count, 0) AS review_count
FROM service s
JOIN service_category sc ON sc.id = s.category_id
LEFT JOIN service_rating_summary srs ON srs.service_id = s.id
WHERE s.public_id = $1
    AND s.active = TRUE;


-- name: ListServiceOffers :many
SELECT
        so.id,
        so.service_id,
        so.provider_id,
        p.name AS provider_name,
        p.logo_url AS provider_logo_url,
        p.rating AS provider_rating,
        so.price,
        so.currency,
        so.billing_period,
        so.installation_cost,
        so.contract_period,
        so.available,
        so.coverage_summary,
        so.additional_costs_summary,
        so.service_url,
        so.last_updated
FROM service_offer so
JOIN provider p ON p.id = so.provider_id
WHERE so.service_id = $1
ORDER BY so.price ASC;


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


-- name: ListProductSpecifications :many
SELECT
    pcs.id AS specification_id,
    pcs.name,
    pcs.data_type,
    pcs.unit,
    pcs.comparable,
    pcs.display_order,
    psv.value
FROM product_specification_value psv
JOIN product_category_specification pcs ON pcs.id = psv.specification_id
WHERE psv.product_id = $1
ORDER BY pcs.display_order ASC, pcs.id ASC;


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