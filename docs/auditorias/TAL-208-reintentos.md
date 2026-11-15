# TAL-208 — Sistema de reintentos automáticos

Fecha: 2026-11-12
Alcance: `scraper/` — descarga de páginas y entrega a Ingestion API.

## 1. Política de descarga

`scraper/src/scraper/fetch/client.py`:

| Aspecto | Comportamiento |
|---|---|
| Errores reintentables | red/TLS/timeout y `408, 425, 429, 500, 502, 503, 504` |
| Errores no reintentables | `4xx` de cliente (`404`, `403`, `422`…) |
| Intentos | `SCRAPER_MAX_RETRIES` (default 3) |
| Espera | exponencial: `SCRAPER_BACKOFF_BASE * 2^(n-1)` (default 1 s) |
| Rate limit | compartido entre reintentos (`SCRAPER_RATE_LIMIT`) |
| Trazabilidad | `attempts` en `FetchResult` y en `run.json` |

Un `404` no se reintenta: reintentarlo sólo multiplica peticiones y retrasa el
diagnóstico de un spec roto.

## 2. Política de ingesta

`scraper/src/scraper/ingestion.py`:

| Aspecto | Comportamiento |
|---|---|
| Errores reintentables | `408, 429, 500, 502, 503, 504` y fallos de transporte |
| Errores no reintentables | `4xx` de validación (`400`, `409`, `422`) |
| Intentos | `INGESTION_MAX_RETRIES` (default 3) |
| Espera | exponencial con la misma base |
| Efecto local | si se agota, el crawl local **no falla**: registra `ingestionError` y sigue |

## 3. Qué no se reintenta

- Un spec inválido: falla antes de la primera petición.
- Una tarjeta que no cumple el contrato: va a `out/rejected/run-{id}.jsonl`.
- Un `409` de tienda: es un conflicto de configuración, no un fallo temporal.

## 4. Pruebas

`scraper/tests/test_fetch.py` (nuevo):

| Test | Garantía |
|---|---|
| `test_transient_status_is_retried_until_success` | 503 → 200 usa 2 intentos |
| `test_client_error_is_not_retried` | 404 usa 1 intento |
| `test_network_error_is_retried` | error de red → éxito usa 2 intentos |
| `test_retries_stop_at_the_configured_limit` | se detiene en `max_retries` |
| `test_timeout_counts_as_a_retryable_error` | timeout se reintenta |

`scraper/tests/test_ingestion.py` ya cubría `test_transient_failure_is_retried`
para la apertura de runs.

## 5. Cómo reproducir

```bash
cd scraper
./venv/bin/python -m pytest tests/test_fetch.py tests/test_ingestion.py -v
```

## 6. Conclusión

La política existía pero no estaba fijada por pruebas. Ahora cualquier cambio
que reintente un `404` o que deje de reintentar un `503` rompe la suite.
