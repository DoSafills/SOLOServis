-- name: ListBrands :many
SELECT
    id,
    name,
    logo_url,
    website_url,
    active
FROM brand
WHERE active = true
ORDER BY name;


-- name: GetBrandByID :one
SELECT
    id,
    name,
    logo_url,
    website_url,
    active
FROM brand
WHERE id = $1;
