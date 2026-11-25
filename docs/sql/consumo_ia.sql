-- =====================================================================
-- Reporte de consumo del asistente de IA (TAL-225).
--
-- Lee ai_comparison_query, que llena el asistente en cada consulta
-- (migración 029). Solo lectura: ninguna consulta escribe.
--
-- Cada bloque lleva un marcador `-- name: <clave>`; scripts/testing/consumo_ia.py
-- los ejecuta por separado. También se puede usar a mano:
--
--     psql -v dias=7 -f docs/sql/consumo_ia.sql
--
-- `:dias` es la ventana hacia atrás en días. No cambiar el nombre: el script
-- lo pasa como variable de psql.
-- =====================================================================


-- name: resumen
-- Totales de la ventana: cuántas consultas fueron al proveedor, cuántas se
-- sirvieron de la caché, cuántos tokens se gastaron y qué latencia se pagó.
-- Las consultas cacheadas no tienen tokens ni latencia propias, por eso los
-- promedios se calculan solo sobre las que llamaron al modelo.
SELECT
    count(*)                                          AS consultas,
    count(*) FILTER (WHERE cached)                    AS desde_cache,
    count(*) FILTER (WHERE NOT cached)                AS al_proveedor,
    count(*) FILTER (WHERE NOT success)               AS errores,
    coalesce(sum(prompt_tokens), 0)                   AS tokens_entrada,
    coalesce(sum(output_tokens), 0)                   AS tokens_salida,
    round(avg(latency_ms) FILTER (WHERE NOT cached))  AS latencia_media_ms,
    round(percentile_cont(0.95) WITHIN GROUP (ORDER BY latency_ms)
          FILTER (WHERE NOT cached))                  AS latencia_p95_ms
FROM ai_comparison_query
WHERE created_at >= now() - make_interval(days => :dias);


-- name: por_dia
-- Consumo día por día, para ver picos y si la caché está haciendo su trabajo.
SELECT
    to_char(date_trunc('day', created_at), 'YYYY-MM-DD') AS dia,
    count(*)                                             AS consultas,
    count(*) FILTER (WHERE cached)                       AS desde_cache,
    coalesce(sum(prompt_tokens), 0)                      AS tokens_entrada,
    coalesce(sum(output_tokens), 0)                      AS tokens_salida,
    round(avg(latency_ms) FILTER (WHERE NOT cached))     AS latencia_media_ms
FROM ai_comparison_query
WHERE created_at >= now() - make_interval(days => :dias)
GROUP BY 1
ORDER BY 1;


-- name: por_usuario
-- Quién consume. `anónimo` son las consultas sin sesión, que es el caso normal:
-- la ruta es pública y solo guarda el user_id cuando hay sesión.
SELECT
    coalesce(user_id::text, 'anónimo')                     AS usuario,
    count(*)                                              AS consultas,
    count(*) FILTER (WHERE cached)                        AS desde_cache,
    coalesce(sum(prompt_tokens), 0)                       AS tokens_entrada,
    coalesce(sum(output_tokens), 0)                       AS tokens_salida,
    to_char(max(created_at), 'YYYY-MM-DD HH24:MI')        AS ultima
FROM ai_comparison_query
WHERE created_at >= now() - make_interval(days => :dias)
GROUP BY 1
ORDER BY consultas DESC, tokens_entrada DESC
LIMIT 20;


-- name: por_modelo
-- Compara proveedores y modelos: si algún día se cambia AI_MODEL, aquí se ve
-- el efecto en tokens y latencia.
SELECT
    provider                                          AS proveedor,
    model                                             AS modelo,
    count(*)                                          AS consultas,
    coalesce(sum(prompt_tokens), 0)                   AS tokens_entrada,
    coalesce(sum(output_tokens), 0)                   AS tokens_salida,
    round(avg(latency_ms))                            AS latencia_media_ms
FROM ai_comparison_query
WHERE created_at >= now() - make_interval(days => :dias)
  AND NOT cached
GROUP BY 1, 2
ORDER BY consultas DESC;


-- name: errores
-- Fallos del proveedor o de la configuración, agrupados por mensaje, para
-- saber si el problema es la clave, la cuota o la red.
SELECT
    coalesce(error_message, '(sin detalle)')          AS error,
    count(*)                                          AS veces,
    to_char(max(created_at), 'YYYY-MM-DD HH24:MI')    AS ultima
FROM ai_comparison_query
WHERE created_at >= now() - make_interval(days => :dias)
  AND NOT success
GROUP BY 1
ORDER BY veces DESC
LIMIT 20;


-- name: repetidas
-- Preguntas repetidas: son las que la caché ya está resolviendo gratis, y las
-- candidatas a pre-prompt si vuelven a aparecer sin sesión.
SELECT
    left(lower(btrim(question)), 80)                  AS pregunta,
    count(*)                                          AS veces,
    count(*) FILTER (WHERE cached)                    AS desde_cache
FROM ai_comparison_query
WHERE created_at >= now() - make_interval(days => :dias)
GROUP BY 1
HAVING count(*) > 1
ORDER BY veces DESC, pregunta
LIMIT 10;
