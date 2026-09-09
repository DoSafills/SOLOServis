-- name: ListStores :many
SELECT
    id,
    name,
    website_url,
    logo_url,
    rating,
    reputation,
    shipping_information,
    general_conditions,
    active,
    created_at,
    updated_at
FROM store
WHERE active = true
ORDER BY name;


-- name: GetStoreByID :one
SELECT
    id,
    name,
    website_url,
    logo_url,
    rating,
    reputation,
    shipping_information,
    general_conditions,
    active,
    created_at,
    updated_at
FROM store
WHERE id = $1;
