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
