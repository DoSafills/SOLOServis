-- PROPUESTA OPCIONAL - no aprobada - no aplicar a bases del proyecto
-- Depende de revisar los datos de la base objetivo antes de aprobarla.
-- Columnas verificadas en backend/database/migrations/001_schema.sql:121,132.
BEGIN;

DO $$
DECLARE
    duplicate_rows BIGINT;
BEGIN
    SELECT COALESCE(SUM(grouped.row_count - 1), 0)
      INTO duplicate_rows
      FROM (
          SELECT COUNT(*)::BIGINT AS row_count
            FROM product_offer
           WHERE product_url IS NOT NULL
             AND BTRIM(product_url) <> ''
           GROUP BY store_id, product_url
          HAVING COUNT(*) > 1
      ) AS grouped;

    IF duplicate_rows > 0 THEN
        RAISE EXCEPTION
            'No se puede crear el indice unico: hay % filas duplicadas adicionales por (store_id, product_url)',
            duplicate_rows;
    END IF;
END
$$;

CREATE UNIQUE INDEX IF NOT EXISTS idx_product_offer_store_url_nonempty_unique
    ON product_offer (store_id, product_url)
    WHERE product_url IS NOT NULL
      AND BTRIM(product_url) <> '';

COMMIT;
