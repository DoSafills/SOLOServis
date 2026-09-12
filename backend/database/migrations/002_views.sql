-- =====================================================================
-- Vistas para campos "derived_*" marcados como "remove from storage" en el MER
-- Se calculan en tiempo real a partir de las reviews en vez de guardarse
-- como columnas físicas (evita datos desincronizados).
-- =====================================================================

CREATE OR REPLACE VIEW product_rating_summary AS
SELECT
    p.id AS product_id,
    COALESCE(AVG(r.rating), 0)::NUMERIC(3,2) AS derived_average_rating,
    COUNT(r.id) AS derived_review_count
FROM product p
LEFT JOIN product_review r ON r.product_id = p.id
GROUP BY p.id;

CREATE OR REPLACE VIEW service_rating_summary AS
SELECT
    s.id AS service_id,
    COALESCE(AVG(r.rating), 0)::NUMERIC(3,2) AS derived_average_rating,
    COUNT(r.id) AS derived_review_count
FROM service s
LEFT JOIN service_review r ON r.service_id = s.id
GROUP BY s.id;