-- TAL-209: control de cambios por fuente (solo lectura).
--
-- Cruza data_source → scraper_config → scrape_run → scraped_data y resume por
-- fuente: ejecuciones fallidas recientes, ítems de la última ejecución frente
-- al promedio de las anteriores, ofertas de producto desactualizadas y precios
-- nulos o en 0. El umbral de días es el parámetro psql "dias" (por defecto 7).
--
-- Columnas que faltan en el modelo (no se inventan):
--   * data_source.provider_id: no existe; las ofertas de servicio
--     (service_offer) no se pueden atribuir a una fuente.
--   * product_offer.source_id: no existe; solo store_id, así que la
--     desactualización se mide únicamente para fuentes con store_id.
--   * scraped_data.price / offer_price: no existe; el precio se lee del
--     contrato en raw_data, ruta offer.price.
--   * scraped_data.records_count: no existe; los ítems salen de
--     scrape_run.records_found o de contar filas de scraped_data.
--   * ingestion_run.data_source_id / scraper_config_id / store_id: no
--     existen; solo tiene source (texto), status y details JSONB, sin ítems
--     ni precios por ejecución.

\set dias 7

WITH runs AS (
    SELECT ds.id AS fuente_id,
           sr.id AS run_id,
           sr.started_at,
           sr.status,
           sr.records_found,
           sr.finished_at
    FROM data_source ds
    JOIN scraper_config sc ON sc.data_source_id = ds.id
    JOIN scrape_run sr ON sr.scraper_config_id = sc.id
),
ultimas AS (
    SELECT r.*,
           row_number() OVER (PARTITION BY r.fuente_id
                              ORDER BY r.started_at DESC, r.run_id DESC) AS rn
    FROM runs r
    WHERE r.finished_at IS NOT NULL
),
fallidas AS (
    SELECT r.fuente_id, count(*) AS fallidas
    FROM runs r
    WHERE r.status = 'failed'
      AND r.started_at >= NOW() - make_interval(days => :dias)
    GROUP BY r.fuente_id
),
items AS (
    SELECT r.fuente_id,
           max(r.records_found) FILTER (WHERE r.rn = 1) AS items_ultima,
           round(avg(r.records_found) FILTER (WHERE r.rn BETWEEN 2 AND 6), 1) AS promedio_previas
    FROM ultimas r
    GROUP BY r.fuente_id
),
precios AS (
    SELECT r.fuente_id,
           count(*) FILTER (
               WHERE COALESCE(btrim(sd.raw_data #>> '{offer,price}'), '') = ''
                  OR ((sd.raw_data #>> '{offer,price}') ~ '^[-+]?[0-9]+(\.[0-9]+)?$'
                      AND (sd.raw_data #>> '{offer,price}')::numeric = 0)
           ) AS precios_nulos_0
    FROM scraped_data sd
    JOIN ultimas r ON r.run_id = sd.scrape_run_id AND r.rn = 1
    GROUP BY r.fuente_id
),
ofertas AS (
    SELECT ds.id AS fuente_id,
           count(*) FILTER (
               WHERE o.last_updated < NOW() - make_interval(days => :dias)
           ) AS desactualizadas,
           count(*) AS total_ofertas
    FROM data_source ds
    JOIN product_offer o ON o.store_id = ds.store_id
    WHERE ds.source_type = 'store'
      AND ds.store_id IS NOT NULL
    GROUP BY ds.id
)
SELECT ds.name AS fuente,
       ds.source_type AS tipo,
       COALESCE(f.fallidas, 0) AS ejecuciones_fallidas,
       i.items_ultima,
       i.promedio_previas,
       CASE WHEN i.promedio_previas > 0
            THEN round(100 * (i.items_ultima - i.promedio_previas) / i.promedio_previas, 1)
       END AS caida_pct,
       p.precios_nulos_0,
       o.desactualizadas AS ofertas_desactualizadas,
       o.total_ofertas
FROM data_source ds
LEFT JOIN fallidas f ON f.fuente_id = ds.id
LEFT JOIN items i ON i.fuente_id = ds.id
LEFT JOIN precios p ON p.fuente_id = ds.id
LEFT JOIN ofertas o ON o.fuente_id = ds.id
ORDER BY ds.name;

-- ingestion_run no tiene FK a data_source ni columnas de ítems/precios: se
-- resume por el texto source y solo con lo que existe (status y fechas).
SELECT source AS fuente,
       count(*) FILTER (
           WHERE status = 'failed'
             AND created_at >= NOW() - make_interval(days => :dias)
       ) AS ejecuciones_fallidas,
       count(*) AS ejecuciones,
       max(created_at) AS ultima_ejecucion
FROM ingestion_run
GROUP BY source
ORDER BY source;
