-- name: ListServices :many
SELECT
    s.id,
    s.public_id,
    s.category_id,
    s.name,
    s.description,
    s.image_url,
    s.active,
    s.created_at,
    s.updated_at,
    sc.name AS category_name,
    parent_sc.name AS subcategory_name,
    COALESCE(srs.derived_average_rating, 0) AS rating,
    COALESCE(srs.derived_review_count, 0) AS review_count
FROM service s
JOIN service_category sc
    ON sc.id = s.category_id
LEFT JOIN service_category parent_sc
    ON parent_sc.id = sc.parent_category_id
LEFT JOIN service_rating_summary srs
    ON srs.service_id = s.id
WHERE s.active = true
ORDER BY s.id;

-- name: GetServiceByPublicID :one
SELECT
    s.id,
    s.public_id,
    s.category_id,
    s.name,
    s.description,
    s.image_url,
    s.active,
    s.created_at,
    s.updated_at,
    sc.name AS category_name,
    parent_sc.name AS subcategory_name,
    COALESCE(srs.derived_average_rating, 0) AS rating,
    COALESCE(srs.derived_review_count, 0) AS review_count
FROM service s
JOIN service_category sc
    ON sc.id = s.category_id
LEFT JOIN service_category parent_sc
    ON parent_sc.id = sc.parent_category_id
LEFT JOIN service_rating_summary srs
    ON srs.service_id = s.id
WHERE s.public_id = $1
  AND s.active = true;

-- name: ListServiceSpecifications :many
SELECT
    scs.name,
    ssv.value
FROM service_specification_value ssv
JOIN service_category_specification scs
    ON scs.id = ssv.specification_id
WHERE ssv.service_id = $1
ORDER BY scs.display_order ASC, scs.id ASC;

-- name: ListServiceOffers :many
SELECT
    so.id,
    so.service_id,
    so.provider_id,
    p.name AS provider_name,
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
JOIN provider p
    ON p.id = so.provider_id
WHERE so.service_id = $1
  AND p.active = true
ORDER BY so.price ASC, so.id ASC;

-- name: ListServicePriceHistory :many
SELECT
    sph.service_offer_id,
    sph.price,
    sph.is_promotional,
    sph.recorded_at
FROM service_price_history sph
JOIN service_offer so
    ON so.id = sph.service_offer_id
WHERE so.service_id = $1
ORDER BY sph.recorded_at ASC, sph.id ASC;

-- name: ListServiceOffersByServiceIDs :many
-- Carga las ofertas de varios servicios en una sola consulta para evitar el
-- patrón N+1 al listar servicios.
SELECT
    so.id,
    so.service_id,
    so.provider_id,
    p.name AS provider_name,
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
JOIN provider p
    ON p.id = so.provider_id
WHERE so.service_id = ANY(sqlc.arg('service_ids')::integer[])
  AND p.active = true
ORDER BY so.service_id ASC, so.price ASC, so.id ASC;

-- name: ListServiceSpecificationsByServiceIDs :many
SELECT
    ssv.service_id,
    scs.name,
    ssv.value
FROM service_specification_value ssv
JOIN service_category_specification scs
    ON scs.id = ssv.specification_id
WHERE ssv.service_id = ANY(sqlc.arg('service_ids')::integer[])
ORDER BY ssv.service_id ASC, scs.display_order ASC, scs.id ASC;

-- name: ListServicePriceHistoryByServiceIDs :many
SELECT
    so.service_id,
    sph.service_offer_id,
    sph.price,
    sph.is_promotional,
    sph.recorded_at
FROM service_price_history sph
JOIN service_offer so
    ON so.id = sph.service_offer_id
WHERE so.service_id = ANY(sqlc.arg('service_ids')::integer[])
ORDER BY so.service_id ASC, sph.recorded_at ASC, sph.id ASC;
