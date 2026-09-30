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
    psv.value
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

-- name: ListProductsFiltered :many
-- Devuelve los productos activos que coinciden con los filtros opcionales.
-- Cada parámetro opcional se declara una sola vez para que sqlc genere un
-- único parámetro por filtro. Un parámetro NULL no restringe el resultado, de
-- modo que omitir `limit` conserva el comportamiento de devolver todo.
WITH filters AS (
    SELECT
        COALESCE(sqlc.narg('search')::text, '')   AS search,
        COALESCE(sqlc.narg('category')::text, '') AS category,
        COALESCE(sqlc.narg('brand')::text, '')    AS brand
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
    COALESCE(prs.derived_average_rating, 0) AS rating,
    COALESCE(prs.derived_review_count, 0) AS review_count
FROM product p
CROSS JOIN filters f
LEFT JOIN brand b ON b.id = p.brand_id
JOIN product_category pc ON pc.id = p.category_id
LEFT JOIN product_rating_summary prs ON prs.product_id = p.id
WHERE p.active = true
  AND (
        f.search = ''
     OR p.name ILIKE '%' || f.search || '%'
     OR p.model ILIKE '%' || f.search || '%'
     OR p.sku ILIKE '%' || f.search || '%'
  )
  AND (f.category = '' OR pc.name ILIKE '%' || f.category || '%')
  AND (f.brand = '' OR b.name ILIKE '%' || f.brand || '%')
ORDER BY p.id
LIMIT sqlc.narg('row_limit')::integer
OFFSET sqlc.narg('row_offset')::integer;

-- name: ListProductImagesByProductIDs :many
-- Carga las imágenes de varios productos en una sola consulta para evitar el
-- patrón N+1 al listar productos.
SELECT
    pi.id,
    pi.product_id,
    pi.image_url,
    pi.alt_text,
    pi.sort_order,
    pi.active
FROM product_image pi
WHERE pi.product_id = ANY(sqlc.arg('product_ids')::integer[])
  AND pi.active = true
ORDER BY pi.product_id ASC, pi.sort_order ASC, pi.id ASC;

-- name: ListProductOffersByProductIDs :many
-- Carga las ofertas de varios productos en una sola consulta. El orden por
-- precio se mantiene dentro de cada producto.
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
WHERE po.product_id = ANY(sqlc.arg('product_ids')::integer[])
ORDER BY po.product_id ASC, po.price ASC, po.id ASC;
