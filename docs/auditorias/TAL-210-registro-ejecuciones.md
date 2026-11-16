# TAL-210 — Registro de ejecuciones

| Campo | Valor |
| --- | --- |
| Fecha | 2026-11-16 |
| Alcance | `scraper/`: modelo de respuesta, manager, API de control y tests |
| Estado previo | El registro existía en SQLite y se podía consultar, pero la API no exponía el desenlace de la ingesta |

## 1. Lo que ya existía

La tabla `SCRAPE_RUN` guarda, por ejecución:

| Columna | Contenido |
| --- | --- |
| `status` | `pending`, `running`, `success`, `failed` (CHECK en el esquema) |
| `started_at`, `finished_at` | Marcas de tiempo |
| `records_found`, `records_processed`, `records_failed` | Contadores de **páginas** |
| `items_found`, `items_sent`, `items_failed`, `items_rejected` | Contadores de **productos** |
| `error_message` | Motivo del fallo del crawl |
| `ingestion_run_id`, `ingestion_error` | Enlace al run remoto y motivo del fallo de ingesta |

El repositorio ya tenía `create_run`, `mark_run_running`, `update_counters`,
`finish_run`, `get_run`, `list_runs`, `recover_orphan_runs` y
`run_duration_ms`. La API exponía `POST /runs`, `GET /runs`, `GET /runs/{id}` y
`POST /runs/{id}/stop`.

## 2. Huecos encontrados

| # | Hueco | Por qué importa |
| --- | --- | --- |
| 1 | `GET /runs` y `GET /runs/{id}` **no mostraban** `ingestion_run_id` ni `ingestion_error` | Para saber si la ingesta falló había que abrir SQLite; el run remoto quedaba invisible |
| 2 | `GET /runs` no permitía filtrar por estado | Encontrar los runs fallidos obligaba a traerse 50 y filtrar a mano |
| 3 | No había pruebas del ciclo de vida del registro | Los tests cubrían la API y el pipeline, no el registro en sí: duración, contadores, recuperación |

## 3. Cambios

| Archivo | Cambio |
| --- | --- |
| `src/scraper/models.py` | `RunState` gana `ingestion_run_id` e `ingestion_error`; `to_dict` los incluye sólo si existen |
| `src/scraper/api/manager.py` | `_state` propaga ambos campos; `list_runs` acepta `status` |
| `src/scraper/api/models.py` | `RunResponse` gana `ingestionRunId` e `ingestionError` |
| `src/scraper/api/app.py` | `GET /runs?status=` con `Literal` de los cuatro estados; un valor fuera de rango responde 422 |

Los campos de ingesta se **omiten** cuando no aplican, en vez de viajar como
`null`: un run sin tienda asociada no tiene run remoto.

## 4. Pruebas

`scraper/tests/test_runs.py` (nuevo, 8 pruebas):

| Test | Qué fija |
| --- | --- |
| `test_run_lifecycle_is_recorded` | `pending` sin `finished_at` ni duración; tras `finish`, estado, fecha y duración |
| `test_page_and_item_counters_stay_separate` | `records_*` y `items_*` no se mezclan |
| `test_failure_keeps_the_reason` | `error_message` sobrevive al cierre |
| `test_finish_run_rejects_an_unknown_status` | Un estado fuera de `RUN_STATUSES` lanza y no toca el registro |
| `test_orphan_runs_are_closed_on_restart` | Tras un reinicio, `pending`/`running` → `failed` con motivo; el `success` no se toca |
| `test_list_returns_newest_first_and_filters_by_status` | Orden descendente, filtro por estado y `limit` |
| `test_api_exposes_the_ingestion_outcome` | `ingestionRunId`, `ingestionError` y `errors` llegan al cliente |
| `test_api_filter_by_status` | `?status=failed` filtra; un valor inválido da 422 |

## 5. Verificación

```text
$ ./venv/bin/python -m pytest -o addopts="" -q
123 passed

$ ./venv/bin/ruff check src tests
I001 en tests/test_extract.py   # preexistente, corregido en el PR #97
```

Las 8 pruebas nuevas pasan. El fallo de `ruff` no es de este cambio: viene del
merge de TAL-173 y se corrige en el PR #97 para no mezclar áreas.

## 6. Riesgos residuales

| Riesgo | Nota |
| --- | --- |
| Los estados están declarados dos veces: el `CHECK` de SQLite y el `Literal` de la API | Si se agrega un estado hay que tocar ambos; un test de contrato lo detectaría |
| La duración usa `julianday` con resolución de milisegundos | Suficiente para un run que dura segundos |
| No hay retención: la tabla crece sin límite | Fuera de alcance; hoy son decenas de filas |

## 7. Cómo reproducir

```bash
cd scraper
./venv/bin/python -m pytest tests/test_runs.py -v
./venv/bin/python -m pytest -q
```
