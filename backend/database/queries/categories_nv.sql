-- name: ListCategoryNV1Active :many
SELECT id, name, slug
FROM categories_nv1
WHERE is_active
ORDER BY name, id;

-- name: ListCategoryNV2Active :many
SELECT nv2.id, nv2.nv1_id, nv2.name, nv2.slug
FROM categories_nv2 nv2
JOIN categories_nv1 nv1 ON nv1.id = nv2.nv1_id AND nv1.is_active
WHERE nv2.is_active
ORDER BY nv2.name, nv2.id;

-- name: ListCategoryNV3Active :many
SELECT nv3.id, nv3.nv2_id, nv3.name, nv3.slug
FROM categories_nv3 nv3
JOIN categories_nv2 nv2 ON nv2.id = nv3.nv2_id AND nv2.is_active
JOIN categories_nv1 nv1 ON nv1.id = nv2.nv1_id AND nv1.is_active
WHERE nv3.is_active
ORDER BY nv3.name, nv3.id;

-- name: ListCategoryNV3Bridge :many
SELECT nv3.id, nv3.slug, nv3.legacy_product_category_id
FROM categories_nv3 nv3
JOIN categories_nv2 nv2 ON nv2.id = nv3.nv2_id AND nv2.is_active
JOIN categories_nv1 nv1 ON nv1.id = nv2.nv1_id AND nv1.is_active
WHERE nv3.is_active
ORDER BY nv3.id;

-- name: ResolveCategoryNVScope :many
WITH matched AS (
    SELECT 'nv1'::text AS level, nv1.id AS ref_id, nv1.name AS ref_name
    FROM categories_nv1 nv1
    WHERE nv1.is_active
      AND ((sqlc.narg('id')::integer IS NOT NULL AND nv1.id = sqlc.narg('id')::integer)
           OR (sqlc.narg('slug')::text IS NOT NULL AND nv1.slug = sqlc.narg('slug')::text))
    UNION ALL
    SELECT 'nv2'::text, nv2.id, nv2.name
    FROM categories_nv2 nv2
    JOIN categories_nv1 nv1 ON nv1.id = nv2.nv1_id AND nv1.is_active
    WHERE nv2.is_active
      AND ((sqlc.narg('id')::integer IS NOT NULL AND nv2.id = sqlc.narg('id')::integer)
           OR (sqlc.narg('slug')::text IS NOT NULL AND nv2.slug = sqlc.narg('slug')::text))
    UNION ALL
    SELECT 'nv3'::text, nv3.id, nv3.name
    FROM categories_nv3 nv3
    JOIN categories_nv2 nv2 ON nv2.id = nv3.nv2_id AND nv2.is_active
    JOIN categories_nv1 nv1 ON nv1.id = nv2.nv1_id AND nv1.is_active
    WHERE nv3.is_active
      AND ((sqlc.narg('id')::integer IS NOT NULL AND nv3.id = sqlc.narg('id')::integer)
           OR (sqlc.narg('slug')::text IS NOT NULL AND nv3.slug = sqlc.narg('slug')::text))
),
leaf_scope AS (
    SELECT m.level, m.ref_id, nv3.id, nv3.name
    FROM matched m
    JOIN categories_nv2 nv2 ON
        (m.level = 'nv1' AND nv2.nv1_id = m.ref_id)
        OR (m.level = 'nv2' AND nv2.id = m.ref_id)
        OR (m.level = 'nv3' AND nv2.id = (SELECT nv2_id FROM categories_nv3 WHERE id = m.ref_id))
    JOIN categories_nv1 nv1 ON nv1.id = nv2.nv1_id AND nv1.is_active
    JOIN categories_nv3 nv3 ON nv3.nv2_id = nv2.id AND nv3.is_active
    WHERE nv2.is_active
      AND (m.level <> 'nv3' OR nv3.id = m.ref_id)
)
SELECT m.level, m.ref_id, m.ref_name,
       leaf.id AS nv3_id, leaf.name AS nv3_name
FROM matched m
LEFT JOIN leaf_scope leaf
    ON leaf.level = m.level AND leaf.ref_id = m.ref_id
ORDER BY m.level, m.ref_id, leaf.id;

-- name: ResolveCategoryNVScopeByLegacy :many
WITH RECURSIVE legacy_subtree AS (
    SELECT pc.id
    FROM product_category pc
    WHERE pc.id = sqlc.arg('legacy_id')::integer
      AND pc.active
    UNION ALL
    SELECT child.id
    FROM product_category child
    JOIN legacy_subtree parent ON parent.id = child.parent_category_id
    WHERE child.active
) CYCLE id SET is_cycle USING path,
mapped AS (
    SELECT nv3.id, nv3.name
    FROM legacy_subtree path
    JOIN categories_nv3 nv3
      ON nv3.legacy_product_category_id = path.id
     AND nv3.is_active
    JOIN categories_nv2 nv2 ON nv2.id = nv3.nv2_id AND nv2.is_active
    JOIN categories_nv1 nv1 ON nv1.id = nv2.nv1_id AND nv1.is_active
    WHERE NOT path.is_cycle
)
SELECT 'legacy'::text AS level,
       node.name AS ref_name,
       mapped.id AS nv3_id,
       mapped.name AS nv3_name
FROM (
    SELECT pc.name
    FROM product_category pc
    WHERE pc.id = sqlc.arg('legacy_id')::integer
      AND pc.active
) node
LEFT JOIN mapped ON TRUE
ORDER BY mapped.id;

-- Lectura del árbol de categorías NV1 > NV2 > NV3 (exactamente tres
-- niveles: NV4 no existe) y del alcance de una categoría.
--
-- Reglas comunes:
--   - solo filas activas y solo si la cadena de padres también lo está;
--   - el alcance devuelve ids de NV3: una NV1 devuelve todos sus NV3, una
--     NV2 los suyos y una NV3 solo a sí misma; si no tiene descendientes
--     activos, ResolveCategoryNVScope devuelve una fila con nv3_id NULL;
--   - no hay recursión: son exactamente tres niveles;
--   - el id legacy se traduce por categories_nv3.legacy_product_category_id
--     (034) y su subárbol se recorre con la única CTE recursiva del
--     paquete; un id legacy sin fila activa no devuelve filas.
