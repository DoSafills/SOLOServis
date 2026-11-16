# TAL-134 — Auditoría del funcionamiento actual del scraper

Fecha: 2026-11-12
Alcance: `scraper/` (v2) en `develop`. No cubre `crawler/` (v1), que sigue en el
repositorio hasta alcanzar paridad.

## 1. Método

La auditoría se hizo sobre el código y sobre el estado real del clúster, no
sobre la documentación previa:

- lectura de `scraper/src/scraper/` completo y de `scraper/stores/*/spec.json`;
- ejecución de la suite (`75 passed`);
- inspección del PVC del `scraper-api` (`/data/runs/*/run.json`);
- consultas de solo lectura a PostgreSQL en el namespace del curso;
- contraste con `TAL-110-despliegue-y-validacion.md` §7.

## 2. Qué es y cómo se ejecuta

`scraper/` es un daemon de larga vida, no un cron que arranca un proceso por
tienda. El flujo es:

```
POST /runs  ->  RunManager  ->  pipeline.execute()
                                   ├── fase 1: listing (paginación declarada)
                                   ├── fase 2: crawl/detail (solo lo declarado)
                                   ├── SQLite (4 tablas UML)
                                   ├── catálogo por tienda
                                   └── Ingestion API (si está configurada)
```

| Camino | Entrada | Uso |
| --- | --- | --- |
| CLI | `python -m scraper run --store <slug>` | depuración de un spec |
| API | `POST /runs` (bearer) | producción y CronJob |
| Scheduler | tareas JSON en el PVC | recurrencia sin CronJob por tienda |

El CronJob `scraper-trigger` ya no crawlea: llama `POST /runs` por tienda con
`Idempotency-Key` y termina.

## 3. Fuentes de información

| Tienda | Spec | `storeId` | Estado |
| --- | --- | --- | --- |
| Falabella | `stores/falabella-cl/spec.json` | 1 | Funciona |
| Ripley | `stores/ripley-cl/spec.json` | 4 | Funciona; tienda inicial |
| Mercado Libre | `stores/mercadolibre-cl/spec.json` | 6 | Bloqueada por verificación de cuenta |

El catálogo de destino es `refrigeradores`. Un spec sin `match` válido no
arranca: la regla del paquete es *spec o nada*, sin selectores de respaldo.

## 4. Proceso de extracción

1. **Listing.** Se lee `listing.root` y se buscan las tarjetas. La paginación
   sólo avanza si la página declara el siguiente enlace; no se adivinan URLs.
2. **Contrato.** Cada tarjeta pasa por `extract/engine.py` y `validate.py`
   (TAL-79). Un campo obligatorio ilegible descarta la tarjeta y la registra en
   `out/rejected/run-{id}.jsonl`; no se inventa un valor.
3. **Detalle.** Si el spec declara `detail`, se enriquece el ítem desde su
   página. Si el merge rompe el contrato, se revierte.
4. **Deduplicación.** Por `externalId` (fallback `externalUrl`).
5. **Persistencia.** Se escribe SQLite antes de pedir la página siguiente.

## 5. Datos obtenidos

| Destino | Contenido |
| --- | --- |
| SQLite `SCRAPED_DATA` | ítem crudo por corrida y por tarjeta |
| SQLite `SCRAPE_RUN` | estado, contadores de páginas e ítems, errores, `ingestion_run_id` |
| `out/catalog/<store>/<producto>.json` | un archivo estable por producto |
| `out/catalog/<store>/_index.json` | índice del catálogo |
| `out/catalog/<store>/_changes.jsonl` | historial de altas, cambios y bajas |
| `out/runs/<id>/run.json` | snapshot completo de la corrida |
| `out/rejected/run-{id}.jsonl` | tarjetas descartadas y motivo |

Los archivos del catálogo se escriben de forma atómica. Un crawl truncado
marca el catálogo como incompleto y **no** borra productos ausentes.

## 6. Dependencias

- Python 3.12+, `scrapling[fetchers]`, `fastapi`, `uvicorn`, `httpx`.
- `libcurl4` en la imagen; **ningún navegador** (no Playwright ni Selenium).
- SQLite embebido; sin ORM ni migraciones propias.

## 7. Mecanismo de ejecución en Kubernetes

| Recurso | Configuración |
| --- | --- |
| Deployment `scraper-api` | 1 réplica, `Recreate`, puerto 8095, `/ready` con SQLite y specs |
| PVC `scraper-data` | 1 Gi montado en `/data` |
| ConfigMap `scraper-config` | límites, rate limit, URLs y token de ingesta |
| Secret `soloservis-internal` | `INTERNAL_API_TOKEN` compartido con Ingestion |
| CronJob `scraper-trigger` | dispara `POST /runs` por tienda |

## 8. Hallazgos

| # | Hallazgo | Evidencia |
| --- | --- | --- |
| 1 | La extracción es determinista y auditable: todo valor proviene de un selector declarado | `extract/`, `spec/schema.py` |
| 2 | Los ítems rechazados quedan en disco con motivo, no se descartan en silencio | `output/raw.py`, `pipeline.py` |
| 3 | El estado de cada corrida es consultable sin abrir la base | `GET /runs/{id}`, `run.json` |
| 4 | La ingesta funciona de punta a punta desde el PR #64 | runs 12–16 con `itemsSent > 0`; 203 productos en PostgreSQL |
| 5 | El apagado silencioso de la ingesta está corregido en el PR #70 | `/ready` publica `ingestion` y el run marca `ingestionError` si procesa 0 |
| 6 | El `storeId` está hardcodeado en cada spec | `spec.json`; depende del orden de inserción de `005_demo_catalog.sql` |
| 7 | Mercado Libre bloquea el tráfico automatizado | redirección a `/gz/account-verification` |
| 8 | El PVC de 1 Gi no tiene política de retención | `out/runs/*/pages` crece por corrida (~6.7 MB) |

## 9. Riesgos y recomendaciones

| Prioridad | Riesgo | Recomendación |
| --- | --- | --- |
| Alta | `storeId` depende del entorno y puede asignar ofertas a la tienda equivocada | Resolver la tienda por nombre en Ingestion o validar el mapeo al arrancar |
| Media | PVC sin retención termina llenándose | Purgar `runs/` y `rejected/` por antigüedad o mover el catálogo fuera del PVC |
| Media | Mercado Libre inutilizable | Evaluar su API oficial o retirar el spec del scheduler |
| Baja | El catálogo depende del orden de inserción de las tiendas demo | Sembrar con IDs explícitos |

## 10. Cómo reproducir

```bash
cd scraper
./venv/bin/python -m pytest -q          # 75 passed
./venv/bin/ruff check src tests
PYTHONPATH=src ./venv/bin/python -m scraper validate
PYTHONPATH=src ./venv/bin/python -m scraper run --store ripley-cl
jq '.counters' out/runs/<id>/run.json
```

En clúster:

```bash
kubectl -n student-marcelo-vidal-22843887e32d exec postgres-0 -- \
  psql -U postgres -d soloservis -c \
  "select count(*) from product; select id, status, items_sent from scrape_run order by id desc limit 5;"
```

## 11. Conclusión

El scraper hace lo que declara: specs estrictos, extracción verificable,
persistencia local antes de la red, ingesta observable y parada controlada.
Los riesgos abiertos son de configuración y operación —mapeo de tiendas,
retención de disco y una tienda bloqueada—, no del diseño del pipeline.
