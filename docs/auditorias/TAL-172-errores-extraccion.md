# TAL-172 — Corrección de errores de extracción

| Campo | Valor |
| --- | --- |
| Fecha | 2026-11-21 |
| Alcance | `scraper/` (pipeline, motor de extracción, spec de Falabella), `backend/internal/ingestion/` |
| Migraciones | Ninguna |
| Datos revisados | 43 corridas reales del clúster UCT, del 2026-11-05 al 2026-11-21 |

## 1. Qué se midió antes de tocar código

La tarea pedía «solucionar fallos asociados a cambios en las estructuras y
respuestas de las fuentes». La medición dice que hoy no hay rechazos que
corregir:

| Señal | Resultado |
| --- | --- |
| Archivos de rechazo (`/data/rejected`) | **0**, el directorio está vacío |
| Rechazos en las 43 corridas (`run.json`) | **0** |
| `scrape_run.records_failed` en PostgreSQL | sólo las 3 corridas de diagnóstico del 2026-11-12 (`f=1 p=0 x=1`, `f=26 p=0 x=26`, `f=57 p=0 x=57`) |
| Desde el 2026-11-13 | todo `records_failed = 0` |
| Falabella | 25-27 ítems por corrida |
| Ripley | 57-58 ítems por corrida |
| Mercado Libre | todas las corridas fallan |

Ese «25-27 de 28 tarjetas» de Falabella no era una pérdida. Con el HTML real de
la corrida 41 y el extractor desplegado:

```text
tarjetas 28   rechazos 0
claves repetidas: testId-pod-80526714 x2, testId-pod-151021732 x2, testId-pod-144883806 x2
data-key: 28 en total, 25 únicos
id="testId-pod-N": 28 en total, 25 únicos
```

El listado trae 28 tarjetas con **25 productos únicos**: tres refrigeradores
aparecen dos veces (posición patrocinada y posición normal). El dedupe hacía lo
correcto; lo que faltaba era contarlo.

## 2. Los tres defectos que sí existían

### 2.1 Un fallo total no dejaba rastro

`pipeline.execute` sólo abría corrida en Ingestion cuando la corrida terminaba
en `success` y había ítems:

```python
if status == RUN_STATUS_SUCCESS and ingestion is not None and items:
    ..._ingest(...)
```

Una tienda que no responde no llega a esa rama, así que no hay fila en
`scrape_run`, ni contadores, ni alerta de TAL-211. Por eso **Mercado Libre no
existe en PostgreSQL**: sus corridas fallan en el fetch y sólo quedaban en el
SQLite del scraper y en el log del Job.

### 2.2 El `externalId` de Falabella era un contador del DOM

El spec tomaba `item.externalId = {"attribute": "id"}`, o sea
`testId-pod-151021732`. Contra el JSON de la página:

| `data-key` (SKU) | `id` del DOM |
| --- | --- |
| `80526714` | `testId-pod-80526714` |
| `151021731` | `testId-pod-151021732` |
| `144883805` | `testId-pod-144883806` |

No es el SKU: es un contador de posición que a veces coincide. Además, sin
`id` el ítem quedaba con clave vacía y `_dedupe` lo descartaba sin dejar razón.

### 2.3 El descarte por duplicado era invisible

`_dedupe` eliminaba por clave sin contar cuántos dejaba fuera. Un listado que
repite tarjetas por error se veía como «la tienda vendió menos» y la regla de
TAL-211 lo habría atribuido a una caída.

## 3. Qué se cambió

| Archivo | Cambio |
| --- | --- |
| `scraper/src/scraper/pipeline.py` | `_report_run`: abre y cierra la corrida en Ingestion también cuando no hay ítems o la corrida falló. `_dedupe` devuelve `(nuevos, descartados)` y alimenta `items_duplicated` |
| `scraper/src/scraper/models.py` | `Counters.items_duplicated` y su nombre de contrato `itemsDuplicated` |
| `scraper/src/scraper/extract/engine.py` | `externalId` puede exigirse en `rejectIfMissing`; sin clave la tarjeta se rechaza con razón en vez de desaparecer |
| `scraper/stores/falabella-cl/spec.json` | `externalId` sale de `data-key`; `externalId` agregado a `rejectIfMissing` |
| `backend/internal/ingestion/model.go` | `FinishRequest.ItemsDuplicated` opcional |
| `backend/internal/ingestion/handler.go` | Validación 0..1 000 000 con 422 por campo (`itemsDuplicated/min`, `/max`) |
| `backend/internal/ingestion/degrade.go` | La caída se mide con `sent + duplicated`: los duplicados no son una tienda caída, y si igual cae la razón dice cuántos se descartaron |
| `backend/internal/ingestion/repository.go` | `RecentFound` ignora las corridas exitosas con `records_found = 0`, para que un vacío puntual no baje la mediana |

## 4. Verificación

Contra el HTML real de producción (corrida 41, 2,27 MB), con el spec y el
extractor nuevos:

```text
tarjetas 28   rechazos 0   únicos 25   duplicados 3
ids de ejemplo: 80526714, 151021731, 144883805, 80563776
con prefijo testId-pod: 0
```

```text
$ pytest                 → 172 passed (169 de feat/backend + 3 nuevos)
$ ruff check src tests   → All checks passed!
$ go build ./...         → sin errores
$ go vet ./...           → sin observaciones
$ go test ./... -count=1 → 19 paquetes ok, 0 FAIL
```

Pruebas nuevas:

| Archivo | Qué fija |
| --- | --- |
| `scraper/tests/test_extract.py` | La clave es el SKU y no el contador del DOM; la tarjeta sin `data-key` se rechaza con razón |
| `scraper/tests/test_pipeline.py` | Una corrida fallida abre y cierra corrida en Ingestion con sus contadores; el descarte por duplicado se cuenta |
| `scraper/tests/test_e2e_ingestion.py` | `itemsDuplicated` viaja en el cierre real del pipeline |
| `backend/internal/ingestion/degrade_test.go` | Los duplicados explican la caída sin alertar; si no alcanzan, la razón los nombra; no evitan la alerta por rechazos |
| `backend/internal/ingestion/handler_test.go` | `itemsDuplicated` fuera de rango responde 422 con el campo culpable |

## 5. Límites declarados

- **Mercado Libre sigue fallando.** Redirige a su verificación de cuenta y la
  norma del proyecto (AGENTS §10) es no saltar protecciones anti-bot. Lo que
  cambia es que ahora la caída queda registrada en `scrape_run` y no sólo en el
  log.
- **Los duplicados no se envían.** Se cuentan, no se ingieren: dos tarjetas del
  mismo SKU son un producto, no dos ofertas.
- **El contador es opcional.** Un scraper anterior sigue cerrando la corrida
  como antes; un backend anterior rechazaría `itemsDuplicated`, así que backend
  y scraper deben desplegarse juntos (en el clúster salen en la misma fase).
- **El clúster está atrasado.** La imagen desplegada (`b7a495d`, pods de las
  00:15 UTC) es anterior al PR #152 y a TAL-242, y faltan aplicar las
  migraciones `026`, `027` y `028`. Nada de esto se ve en producción hasta el
  próximo despliegue.
