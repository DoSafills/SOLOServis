# TAL-219 — Seguimiento de errores

| Campo | Valor |
| --- | --- |
| Fecha | 2026-11-22 15:15 -0300 |
| Clúster | UCT, topología `dominios`, imagen `b7a495d` (desplegada el 2026-11-20 21:14) |
| Comando | `python scripts/testing/errores.py --md …` (solo lectura) |
| Cobertura | logs de pods desde el 2026-11-20 21:14; corridas y alertas de los últimos 14 días |

## Cómo se buscan

`scripts/testing/errores.py` junta los errores del AccessLog y del log de los
tres procesos Go, del nginx del frontend, de `scraper-api` y de PostgreSQL.
También revisa los reinicios, los Jobs fallidos, los eventos Warning, `scrape_run`,
las alertas `scraper_degraded` y el SQLite del scraper. Después los agrupa por
origen, ruta, code y causa. El tráfico de nuestras pruebas (pod de test y
`salud.py`, que ahora manda `X-Request-ID: salud-…`) se separa y no cuenta. Se
suma al reporte de testing con `reporte.py --errores`.

## Resultado

| Error | Veces | Desde → hasta | Origen |
| --- | --- | --- | --- |
| Mercado Libre: `the first listing page produced no cards` | 13 de 13 corridas | 2026-11-10 → 2026-11-22 | Anti-bots de Mercado Libre (ya conocido). El mensaje culpa a `listing.root`, pero la página llega sin tarjetas |

Una sola vez: un fallo igual de Falabella (2026-11-11), las corridas 3 y 4 del
2026-11-12 (`success` con 26/26 y 57/57 ítems fallidos, causa `capacidad_litros`
ya corregida en TAL-110), una corrida de diagnóstico y tres `ERROR` de consultas
manuales en PostgreSQL.

No hay ningún 5xx, ningún `internal_error`, ningún reinicio ni ningún Job fallido.
Los 4xx de las APIs (401 sin sesión, 404 en `/search`, `/comparisons`,
`/reviews` y `/auth`, y 429 en `/auth/login`) vienen todos del pod de test y de
`salud.py`. El nginx no registró pedidos públicos en estas 40 h.

## Por qué cuesta verlos

- Las corridas fallidas de Mercado Libre solo quedan en el SQLite del scraper.
  No están en `scrape_run`, no dejan WARNING en el log y no disparan alertas.
  TAL-172 y TAL-211 lo corrigen, pero están en `develop` y no desplegados.
- Ingestion guarda el `status` que manda el scraper, aunque no haya procesado ningún ítem.
- El AccessLog no registra el `code` de problem+json, así que los 4xx se agrupan por status.
- nginx no reenvía ni registra `X-Request-ID`. El 429 del gateway trae un
  `request_id` que no aparece en el backend.
- En `scraper-api`, el 99,7 % del log son sondas `/ready` y `/health`, y cada
  línea de scrapling sale dos veces.
- Los logs de los pods se pierden con cada redeploy. Lo único que dura es la
  base y el SQLite del scraper.

## Pendiente

Redesplegar para que entren TAL-172 y TAL-211. Los tickets que se propusieron
para los puntos anteriores quedan por confirmar.
