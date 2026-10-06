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
  AND (sqlc.narg(public_id)::uuid IS NULL OR s.public_id = sqlc.narg(public_id))
ORDER BY s.id;

-- Las tres queries siguientes reciben varios servicios a la vez para que el
-- listado no haga una consulta por servicio.

-- name: ListServiceSpecifications :many
SELECT
    ssv.service_id,
    scs.name,
    ssv.value,
    scs.unit
FROM service_specification_value ssv
JOIN service_category_specification scs
    ON scs.id = ssv.specification_id
WHERE ssv.service_id = ANY(sqlc.arg(service_ids)::int[])
ORDER BY ssv.service_id, scs.display_order ASC, scs.id ASC;

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
WHERE so.service_id = ANY(sqlc.arg(service_ids)::int[])
  AND p.active = true
ORDER BY so.service_id, so.price ASC, so.id ASC;

-- name: ListServicePriceHistory :many
SELECT
    so.service_id,
    sph.service_offer_id,
    sph.price,
    sph.is_promotional,
    sph.recorded_at
FROM service_price_history sph
JOIN service_offer so
    ON so.id = sph.service_offer_id
WHERE so.service_id = ANY(sqlc.arg(service_ids)::int[])
ORDER BY so.service_id, sph.recorded_at ASC, sph.id ASC;
