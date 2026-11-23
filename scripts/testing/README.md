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
| `--recursos` | Suma el consumo de CPU y memoria de cada pod (corre `recursos.py` 30 s, sin generar carga) |
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

Muestra los recurrentes con su primera y última vez y un `request_id` o una corrida de ejemplo. Es de solo lectura. En los 4xx de Go la causa es el `code` del problem+json (TAL-250), y las líneas del nginx traen el mismo `request_id` que recibió el backend (TAL-251), así una llamada se sigue de punta a punta.

```bash
python scripts/testing/errores.py --md errores.md
```

El tráfico de nuestras pruebas no cuenta: los pedidos con `X-Request-ID: salud-…` o `carga-…` (los mandan `salud.py` y `carga.py`) y los que caen a menos de `--ventana` segundos (120 por defecto) de un pedido interno en el nginx o del Job de smoke test. Los logs de los pods solo cubren desde el último arranque de cada contenedor.

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

## Carga y consumo de recursos

`carga.py` (TAL-206) simula usuarios que navegan el sitio como el frontend: lista de productos (a veces por categoría), categorías, búsqueda, detalle y tiendas, con 1 a 3 s de espera entre páginas. Sube por escalones (5, 10 y 20 usuarios, 2 minutos cada uno, con 30 s de pausa) y reporta por escalón y por ruta las latencias p50/p95/p99, los errores por código y los pedidos por segundo.

Mientras corre, `recursos.py` (TAL-207) mide cada pod. En UCT `kubectl top` no funciona (el rol no puede leer `metrics.k8s.io`), así que lee el cgroup v2 de cada contenedor con un solo `kubectl exec` por pod: CPU en milicores, throttling, working set y pico de memoria, contra sus requests, limits y la cuota del namespace.

```bash
python scripts/testing/carga.py --sin-verificar-tls          # UCT por el host público
python scripts/testing/carga.py --local --login              # contenedores desechables
python scripts/testing/carga.py --local --directo --escalones 20,50,100 --espera 0 --max-rps 0
python scripts/testing/recursos.py --duracion 60             # solo el consumo, sin carga
```

- **Frenos:** tope global de 15 r/s (`--max-rps`), y la prueba se corta si en 15 s los errores pasan del 20 %, si el p95 pasa de 5 s o si un pod se reinicia. El clúster es compartido y el gateway limita a 30 r/s por IP: toda la carga sale de una sola IP.
- **TLS:** el host de UCT usa el certificado autofirmado del Ingress; `--sin-verificar-tls` lo acepta, como el aviso del navegador.
- **`--local`:** levanta `soloservis-carga-*` (PostgreSQL con las migraciones, comercial, usuarios y el gateway) con los mismos limits de CPU y memoria que en el clúster, en la red `soloservis-carga` y en los puertos 18443 (gateway) y 18080 (comercial). No toca los contenedores de `docker compose` y los borra al terminar. Ahí se usan `--login`, `--directo` (sin el gateway) y `--sin-frenos`.
- **Qué deja:** el reporte en `docs/reportes/carga/AAAA-MM-DD-HHMM-<entorno>.md` y los pedidos y muestras crudos en `tmp/reportes-carga/`.

Sale con código 1 si un freno cortó la prueba o hubo 5xx.
