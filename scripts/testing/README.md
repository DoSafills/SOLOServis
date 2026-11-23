# Reporte de testing

`reporte.py` corre las suites de pruebas del repositorio y registra **resultados, errores y estados** en un reporte Markdown (TAL-168). Solo usa la librería estándar de Python 3.9+.

```bash
python scripts/testing/reporte.py
```

| Opción | Qué hace |
| --- | --- |
| `--suites backend,scraper` | Corre solo esas suites (`backend`, `scraper`, `crawler`, `frontend`) |
| `--postgres-docker` | Levanta un PostgreSQL 18 desechable con las migraciones, para los tests de integración del backend. Necesita Docker encendido |
| `--smoke` | Suma las verificaciones del pod de test de Kubernetes (lee el log del Job `soloservis-smoke-test`) |
| `--errores` | Suma los errores recurrentes del clúster (corre `errores.py`, ver abajo) |
| `--python <ruta>` | Intérprete que tiene instaladas las dependencias del scraper y del crawler |
| `--out <carpeta>` | Dónde dejar el reporte (por defecto `docs/reportes/testing`) |

## Qué corre

| Suite | Comando | Requisito |
| --- | --- | --- |
| Backend | `go test -json ./...` | Go. Sin `TEST_DATABASE_URL` ni `--postgres-docker`, los tests de integración quedan omitidos |
| Scraper v2 | `pytest --junitxml` en `scraper/` | `pip install -e "./scraper[dev]"` |
| Crawler v1 | `pytest --junitxml` en `crawler/` | `pip install -r crawler/requirements.txt` |
| Frontend | `pnpm format:check`, `lint`, `build` y `audit` | `pnpm install` en `fronted/`. No hay tests unitarios |
| Kubernetes | `kubectl logs job/soloservis-smoke-test` | Acceso al clúster y un deploy reciente |

Una suite que no se puede ejecutar queda como **no ejecutada**, con el motivo, y no detiene el reporte.

## Qué deja

- **El reporte:** `docs/reportes/testing/AAAA-MM-DD-HHMM-<commit>.md`. Se sube al repositorio como evidencia.
- **Los datos crudos** (JSON de `go test`, XML de `pytest` y salidas completas): `tmp/reportes-testing/…`, ignorado por git.

El reporte trae: fecha, rama, commit y versiones; el resumen por suite (total, aprobadas, fallidas, omitidas, duración); el error de cada prueba fallida; las pruebas omitidas agrupadas por motivo, y el detalle por suite.

El script termina con código `1` si alguna prueba falla, así que sirve como paso de CI.

## Errores recurrentes del clúster

`errores.py` (TAL-219) junta los errores que deja el despliegue y los agrupa por origen, ruta, code y causa:

- los 4xx/5xx y las líneas WARN/ERROR de los procesos Go, nginx, `scraper-api` y PostgreSQL;
- los reinicios, los Jobs fallidos y los eventos Warning;
- las corridas fallidas de `scrape_run` y del SQLite del scraper, y las alertas `scraper_degraded`.

Muestra los recurrentes con su primera y última vez y un `request_id` o una corrida de ejemplo. Es de solo lectura.

```bash
python scripts/testing/errores.py --md errores.md
```

El tráfico de nuestras pruebas no cuenta: los pedidos con `X-Request-ID: salud-…` (los manda `salud.py`) y los que caen a menos de `--ventana` segundos (120 por defecto) de un pedido interno en el nginx o del Job de smoke test. Los logs de los pods solo cubren desde el último arranque de cada contenedor.

## Consumo del asistente de IA

`consumo_ia.py` lee la auditoría del asistente (`ai_comparison_query`, migración 029) y responde cuánto se gastó, quién consumió y qué errores se repitieron. Es de solo lectura: abre una sesión `READ ONLY`.

```bash
python scripts/testing/consumo_ia.py              # PostgreSQL de docker compose
python scripts/testing/consumo_ia.py --cluster    # PostgreSQL del clúster con kubectl
python scripts/testing/consumo_ia.py --dias 30    # ventana de 30 días (7 por defecto)
python scripts/testing/consumo_ia.py --json       # salida para procesar
```

| Bloque | Qué muestra |
| --- | --- |
| Resumen | Consultas, cuántas fueron al proveedor y cuántas a la caché, tokens de entrada y salida, latencia media y p95 |
| Por día | El mismo consumo agrupado por fecha |
| Por usuario | `anónimo` es lo normal: la ruta es pública y solo guarda `user_id` con sesión |
| Por proveedor y modelo | Para comparar el efecto de cambiar `AI_MODEL` |
| Errores | Mensajes agrupados: clave, cuota o red |
| Preguntas repetidas | Las que la caché ya resuelve gratis |

Las consultas están en `docs/sql/consumo_ia.sql`, con un marcador `-- name:` por bloque. Se pueden correr a mano con `psql -v dias=7 -f docs/sql/consumo_ia.sql`.

Termina con código `1` si no hay consultas en la ventana o si falta la migración 029.
