-- name: ListCategories :many
SELECT
    id,
    parent_category_id,
    name,
    description,
    active,
    created_at,
    updated_at
FROM product_category
WHERE active = true
ORDER BY name;


-- name: GetCategoryByID :one
SELECT
    id,
    parent_category_id,
    name,
    description,
    active,
    created_at,
    updated_at
FROM product_category
WHERE id = $1;
