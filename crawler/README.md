# SOLOServisScrapper
Sistema asíncrono de crawling y scraping en Python. La arquitectura separa dos
responsabilidades (ver `AGENTS.md` para la especificación completa):

* **Crawler** — descubre, programa, deduplica y gestiona las URLs a visitar.
* **Scraper** — descarga páginas con [Scrapling](https://github.com/D4Vinci/Scrapling) y extrae datos estructurados.

## Stack

| Capa | Tecnología |
|---|---|
| Lenguaje | Python 3.12+ |
| HTTP / parsing | Scrapling (`AsyncFetcher`) |
| Concurrencia | `asyncio` + `asyncio.Queue` (workers configurables) |
| Persistencia | SQLite (esquema relacional `DATA_SOURCE`, `SCRAPER_CONFIG`, `SCRAPE_RUN`, `SCRAPED_DATA`) |

## Instalación

```bash
python3 -m venv venv && source venv/bin/activate
pip install -e ".[dev]"        # o: pip install -r requirements.txt
```

## Uso

```bash
python src/main.py
```

En la primera ejecución se inicializan las tablas. El crawler lee todas las
parejas activas de `DATA_SOURCE` + `SCRAPER_CONFIG`, crea su fila en
`SCRAPE_RUN` y arranca. Las seeds se registran vía variable de entorno o SQL
(ver abajo); no hay fuentes precargadas.

Para reintentar ítems normalizados desde un JSONL canónico:

```bash
python src/main.py replay out/rejected/run-<id>.jsonl
```

Cada línea debe ser un `NormalizedItem` versión `1.0` con `source.storeId`.
El comando valida los ítems localmente y los envía a Ingestion API en lotes;
no importa el espejo SQLite ni ejecuta SQL directamente. La categoría indicada
por `product.categorySlug` debe existir en PostgreSQL. El resumen distingue
creados, actualizados, fallidos y rechazados. El backend procesa cada ítem
por separado; un lote no es una transacción atómica. Este replay admite
productos, no ofertas de servicios.

## Fuentes múltiples (array de seeds)

Cada fila activa en `DATA_SOURCE` es una seed independiente: se crawlea con su
propio pool de workers, en paralelo con el resto.

**Forma rápida — array por entorno:**

```bash
SCRAPER_SEEDS="https://quotes.toscrape.com/,https://books.toscrape.com/catalogue/category/books/travel_2/index.html" \
python src/main.py
```

Las URLs sin esquema se asumen `https://` (`midominio.com/cat` →
`https://midominio.com/cat`). Las filas ya existentes no se duplican.

**Forma SQL — control completo** (persistente en la BD):

```sql
INSERT INTO DATA_SOURCE (name, base_url, source_type)
VALUES ('Books: Travel', 'https://books.toscrape.com/catalogue/category/books/travel_2/index.html', 'web');

INSERT INTO SCRAPER_CONFIG (data_source_id, scraper_type, target_type, rate_limit, parser_config)
VALUES (last_insert_rowid(), 'http', 'webpage', '30/min',
        '{"entity_type": "webpage", "external_id_selector": null}');
```

### Modelo de paralelismo

```
proceso main
 ├── hilo 0  → event loop asyncio propio
 │            ├── fuente A → asyncio.Queue + N workers (SCRAPER_MAX_CONCURRENCY)
 │            └── fuente B → asyncio.Queue + N workers propios
 └── hilo 1  → event loop asyncio propio
              └── fuente C → asyncio.Queue + N workers propios
```

* Concurrencia HTTP efectiva ≈ `fuentes_activas × MAX_CONCURRENCY`
  (limitada a `MAX_THREADS` grupos en paralelo).
* Rate limit **por dominio**: fuentes del mismo sitio comparten limitador,
  dominios distintos van a plena velocidad.
* Deduplicación por run (AGENTS.md): dos seeds del mismo dominio pueden
  solapar páginas sin problema; cada `SCRAPE_RUN` lleva sus propias stats.

## Configuración por entorno

| Variable | Default | Descripción |
|---|---|---|
| `SCRAPER_DB_PATH` | `scraper.db` | Ruta del fichero SQLite |
| `SCRAPER_SEEDS` | *(vacío)* | Array CSV de seeds; registra cada URL como DATA_SOURCE |
| `SCRAPER_MAX_THREADS` | `4` | Hilos paralelos (grupos de fuentes) |
| `SCRAPER_MAX_CONCURRENCY` | `10` | Workers asyncio por fuente |
| `SCRAPER_MAX_DEPTH` | `1` | Profundidad máxima desde cada seed |
| `SCRAPER_MAX_PAGES` | `50` | Tope de URLs descubiertas por run |
| `SCRAPER_RATE_LIMIT` | `60/min` | Rate limit default `<n>/<s\|min\|h>` |
| `SCRAPER_TIMEOUT` | `15` | Timeout HTTP en segundos |
| `SCRAPER_MAX_RETRIES` | `3` | Intentos ante errores transitorios (408/429/5xx) |
| `SCRAPER_BACKOFF_BASE` | `1.0` | Base del backoff exponencial |
| `SCRAPER_STORE_ID` | *(vacío)* | Tienda por defecto para las seeds de `SCRAPER_SEEDS` (`DATA_SOURCE.store_id`) |
| `INGESTION_API_URL` | `http://localhost:8094` | Base de Ingestion API |
| `INGESTION_API_TOKEN` | *(vacío)* | Bearer token; si falta o mide menos de 32 caracteres, la ingesta queda desactivada |
| `INGESTION_TIMEOUT` | `15` | Timeout por petición, en segundos |
| `INGESTION_MAX_RETRIES` | `3` | Reintentos ante 408/429/5xx y errores de red |
| `INGESTION_BACKOFF_BASE` | `1.0` | Base del backoff exponencial de la ingesta |
| `INGESTION_BATCH_SIZE` | `100` | Ítems por lote (1–100, límite del contrato) |
| `SCRAPER_REJECTS_DIR` | `out/rejected` | Carpeta de los JSONL con ítems descartados |

Ejemplo de run corto multi-sitio:

```bash
SCRAPER_MAX_PAGES=6 SCRAPER_MAX_DEPTH=1 SCRAPER_MAX_CONCURRENCY=3 SCRAPER_MAX_THREADS=2 \
python src/main.py
```

## Formato de `parser_config`

Columna JSON de `SCRAPER_CONFIG`. Claves soportadas:

```json
{
  "entity_type": "webpage",
  "external_id_selector": null
}
```

* `entity_type` — etiqueta guardada en `SCRAPED_DATA.entity_type`.
* `external_id_selector` — CSS de un `<meta>` cuyo `content` se usará como `external_id`.

El HTML completo de cada página se guarda tal cual en `SCRAPED_DATA.raw_data`;
los enlaces descubiertos se normalizan, se filtran por dominio y vuelven a la
cola del crawler.

## Extractor: del HTML a filas insertables

`scraper.py` archiva páginas enteras. `extractor.py` extrae *campos* de ellas y
los convierte a los tipos que exige el schema de producción
(`backend/database/migrations/001_schema.sql`). El spec declarativo dice
*dónde* está cada valor; los contratos de columna (largos `VARCHAR(n)`, escalas
`NUMERIC(p,s)`, enums de `CHECK`) están transcritos del schema, así que un precio
scrapeado llega como `NUMERIC(12,2)` y una condición como uno de los tres
valores permitidos.

```python
from scraper import Scraper
from extractor import EntitySpec, Extractor, export_jsonl

spec = EntitySpec.from_parser_config({
    "entity_type": "product_offer",
    "root": "article.product_pod",
    "fields": {
        "price":        ".price_color",
        "list_price":   ".price_color s",
        "currency":     {"selector": ".currency", "default": "CLP"},
        "available":    {"selector": ".instock", "mode": "list", "attribute": "class"},
        "condition":    {"selector": ".condition", "default": "new"},
        "product_url":  {"selector": "h3 a", "mode": "attr", "attribute": "href"},
    },
    "references": {"store": ".store-name"},
})

outcome = await Scraper().fetch(url)
rows = Extractor(spec).extract_all(outcome.document, outcome.final_url)
export_jsonl(rows, "out/offers.jsonl")   # una fila JSON por línea
```

### `parser_config` para extracción

| Clave | Significado |
|---|---|
| `entity_type` | Tabla destino: `product`, `product_offer`, `service`, `service_offer`, `store`, `provider`, `brand`, `location`, … |
| `root` | CSS del contenedor de cada item en páginas de listado. Sin `root`, el spec extrae una sola fila (página de detalle). |
| `fields` | Mapa `columna → selector CSS` o `columna → {selector, mode, attribute, transform, default}`. |
| `references` | Mapa de **claves naturales** (no ids) que la ingesta debe resolver. |

Un valor de `fields` puede ser un string CSS pelado (`".title::text"`) o un
objeto. Campos del objeto:

| Campo | Por defecto | Significado |
|---|---|---|
| `selector` | — | CSS o XPath. Obligatorio. |
| `mode` | `"text"` | `text` (primer match), `html` (HTML exterior), `attr` (atributo, primer match), `list` (**todos** los matches). |
| `attribute` | — | Nombre del atributo. Gana sobre el texto: `{"mode": "list", "attribute": "src"}` recolecta todos los `src`. |
| `transform` | el de la columna | `text`, `number`, `price`, `integer`, `boolean`, `currency`, `url`, `condition`, `billing_period`, `coordinate`. |
| `default` | el de la columna | Valor cuando el selector no matchea **o** el valor no se puede leer. Gana sobre el default de la columna. |

### Garantías

* **Nunca se inventan claves foráneas.** `product.category_id` es una identidad
  del servidor; un scraper solo puede emitir la clave natural. Por eso
  `references` es aparte de `fields`: la ingesta resuelve `category` →
  `product_category.id`.
* **Los fallos se acumulan, no se lanzan.** Una `Extraction` lleva `errors`; un
  campo ilegible no cuesta la página entera.
* **Sobre-longitud se recorta y se reporta** (`VARCHAR(n)` en PostgreSQL rechaza
  el valor, no lo trunca). Es una pérdida visible, nunca silenciosa.
* `Decimal` se serializa como **string** en el JSONB: PostgreSQL castea un
  literal numérico desde texto de forma exacta, mientras que un float ya habría
  perdido centavos al escribirse.

Tablas con contrato completo: `product`, `product_image`, `brand`,
`product_category`, `product_offer`, `service`, `service_category`,
`service_offer`, `store`, `provider`, `location`.

## Formato común de datos extraídos (`normalized.py`)

El extractor produce *filas*; una tarjeta de producto no es una fila. Una
tarjeta trae nombre, marca, precio, precio de lista, imagen, identificador y
atributos, y cada tienda los ordena distinto. `normalized.py` declara un
**bundle** (`BundleSpec`) con una raíz compartida y colapsa esas partes en un
único `NormalizedItem`, independiente de la tienda.

```python
import json
from scraper import Scraper
from normalized import BundleExtractor, BundleSpec, export_jsonl

spec = BundleSpec.from_parser_config(json.load(open("examples/ripley-cl.json")))
outcome = await Scraper().fetch("https://simple.ripley.cl/search/refrigerador")
items = BundleExtractor(spec).extract_all(outcome.document, outcome.final_url)

export_jsonl(items, "out/ripley.jsonl")     # envelope completo, para auditar
body = items[0].to_ingestion()              # body exacto de Ingestion API
```

### El registro

```json
{
  "schemaVersion": "1.0",
  "entityType": "product",
  "externalId": "testId-pod-14873869",
  "externalUrl": "https://www.falabella.com/falabella-cl/search?Ntt=refrigerador",
  "source": { "name": "Falabella Chile", "baseUrl": "https://www.falabella.com/falabella-cl" },
  "product": {
    "name": "Refrigerador Side by Side de 608 Litros Family Hub",
    "brand": "SAMSUNG",
    "categorySlug": "refrigeradores",
    "imageUrl": "https://media.falabella.com/falabellaCL/14873869_1/width=240"
  },
  "offer": {
    "price": "1349990.00", "listPrice": "2349990.00", "currency": "CLP",
    "shippingCost": "0.00", "shippingFree": false, "available": true, "condition": "new"
  },
  "provenance": {
    "scrapedAt": "2026-11-02T12:00:00Z", "pageUrl": "https://…",
    "references": { "brand": "SAMSUNG" }
  }
}
```

* `product` y `offer` son **el contrato de Ingestion API** (TAL-77): mismas
  claves, mismos límites. `to_ingestion()` proyecta exactamente
  `{externalId, product, offer}`; el envelope (`schemaVersion`, `source`,
  `provenance`) no se envía porque Ingestion rechaza campos desconocidos.
* **La tienda no está dentro del ítem.** Se asigna una vez por ejecución
  (`POST /ingestion/runs`), así que el mismo registro sirve para cualquier
  tienda.
* **Solo claves naturales.** `sku`, `brand`, `categorySlug`; nunca ids del
  servidor.
* **Los montos viajan como texto** (`"1349990.00"`): un float perdería centavos
  antes de llegar a `NUMERIC(12,2)`. Go acepta un número entrecomillado en un
  `json.Number`.
* **Los atributos son JSON nativos.** Un atributo `integer` sale como `608`, no
  como `"608"`; el backend no convierte tipos implícitamente.
* **`categorySlug` se declara, no se adivina.** El spec trae un literal o un
  selector + mapa de alias; si no hay forma de resolverlo, el ítem lo reporta y
  la validación falla en vez de inventar una categoría.

### `parser_config` de un bundle

| Clave | Obligatoria | Significado |
|---|---|---|
| `entity_type` | no (`product`) | Único valor soportado hoy; `service` se rechaza hasta que exista contrato. |
| `root` | no | CSS de la tarjeta. Sin `root`, el documento entero es un ítem (página de detalle). |
| `product` | sí | `{fields, references}` con las columnas de `product`; `brand` va en `references`. |
| `offer` | sí | `{fields}` con las columnas de `product_offer`. |
| `images` | no | `{fields: {image_url: …}}`; se toma el primer match. |
| `external_id` | no | Selector, o `{"attribute": "id"}` para leer el atributo del propio nodo. |
| `category_slug` | no | String literal o `{value \| selector, alias}`. |
| `attributes` | no | Mapa `clave → {selector, type, choices?, default?, max_length?}`. |
| `source` | no | `{name, base_url, store_id?}`; viaja en el envelope, nunca en `product`/`offer`. |

Las claves desconocidas se rechazan: un typo no puede descartar media tarjeta
en silencio. `entity_type`, `fields`, `mode`, `attribute`, `transform` y
`default` funcionan igual que en el extractor.

> Estas claves son las del **bundle**. El `parser_config` de página que consume
> `scraper.py` (`entity_type` + `external_id_selector`) no cambia: son dos
> niveles distintos. El bundle vive en `parser_config["bundle"]` y el crawler lo
> carga en cada run (ver la sección siguiente).

`crawler/examples/` tiene configuraciones reales de Falabella y Ripley, con el
DOM observado y las limitaciones de cada tienda documentadas en
`crawler/examples/README.md`. El contrato normativo (campos, límites,
versionado) está en `docs/estandares/formato-datos-extraidos.md`; el JSON Schema
en `crawler/schema/normalized-item.schema.json`.

## Integración con Ingestion API (flujo B)

El crawler extrae ítems normalizados de cada página y los entrega a Ingestion
API (contrato en `backend/README.md` §12.7, implementación en
`backend/internal/ingestion`). Las páginas se siguen archivando completas en
`SCRAPED_DATA.raw_data`: la entrega remota es adicional, no reemplaza la
auditoría local.

### Activación

La ingesta se activa solo cuando se cumplen las tres condiciones:

1. `DATA_SOURCE.store_id` no es `NULL` (la tienda es un dato de la ejecución,
   no del ítem);
2. `INGESTION_API_TOKEN` existe y tiene al menos 32 caracteres (el backend
   ignora tokens más cortos);
3. el `parser_config` declara un `bundle` válido.

Si falta cualquiera, el crawl termina igual y los ítems quedan solo en local.
`SCRAPER_STORE_ID` asigna la tienda a las seeds creadas con `SCRAPER_SEEDS`;
las fuentes insertadas por SQL declaran su propio `store_id`:

```sql
INSERT INTO DATA_SOURCE (name, base_url, source_type, store_id)
VALUES ('Falabella Chile', 'https://www.falabella.com/falabella-cl', 'web', 1);

INSERT INTO SCRAPER_CONFIG (data_source_id, scraper_type, target_type,
                            rate_limit, parser_config)
VALUES (
  last_insert_rowid(), 'http', 'webpage', '30/min',
  json('{
    "entity_type": "webpage",
    "external_id_selector": null,
    "bundle": { ... }   -- el bundle de examples/falabella-cl.json
  }')
);
```

No hay migraciones: `init_db` usa `CREATE TABLE IF NOT EXISTS`. Una base creada
antes de esta versión no recibe las columnas nuevas automáticamente; se
recomienda recrear el fichero SQLite (`*.db` es descartable y está ignorado por
git).

### Ciclo de una ejecución

```
SCRAPE_RUN (local)                    Ingestion API
  running ─────────────────────────►  POST /ingestion/runs        → running
     │                                   (abre antes de la 1.ª página)
     │  página → extraer → dedupe
     │         → validar → buffer
     │  buffer ≥ batch_size ─────────►  POST /runs/{id}/items
     │  fin de crawl (flush) ────────►  POST /runs/{id}/items
     └────────────────────────────────►  POST /runs/{id}/finish    → success|failed
```

* **Dedupe por ejecución.** Se usa `externalId`; si la tarjeta no lo publica,
  `externalUrl`. Falabella repite pods dentro del mismo listado y el duplicado
  no se vuelve a enviar.
* **Filtro local.** `NormalizedItem.validate()` aplica las mismas reglas del
  backend (campos obligatorios, largos, montos, URLs). Los ítems inválidos no
  viajan: se cuentan en `items_rejected` y se escriben en
  `out/rejected/run-{id}.jsonl` con el envelope completo y los errores.
* **Lotes de 1–100.** El cliente trocea y reintenta solo lo transitorio
  (408/429/5xx y red). Un ítem rechazado por Ingestion no detiene el lote; se
  registra su error y también va a `rejected/`. En cambio, un fallo de
  transporte en un lote corta los envíos siguientes (circuit breaker): lo que
  llegue después se guarda como rechazado para replay en vez de golpear un
  servicio caído.
* **Éxito remoto.** Solo si el crawl local terminó `success` y no hubo ningún
  error de ingesta. En cualquier otro caso el run remoto se cierra `failed` con
  el motivo real (`problem+json`: `code`, `detail`, `errors[]`).

### Si Ingestion no responde

El crawl local **no** se aborta. El comportamiento es:

| Situación | Crawl local | Run remoto | Qué queda registrado |
|---|---|---|---|
| `open` falla (red, 5xx, 422) | termina normal | no se crea | `SCRAPE_RUN.ingestion_error` |
| `items` falla | termina normal | se cierra `failed` | `ingestion_error`, ítems en `rejected/` |
| algún ítem es rechazado | termina normal | `success` | `items_failed`, detalle en `rejected/` |
| Ctrl+C | se cierra `failed` | se cierra `failed` | ítems en buffer van a `rejected/` |

En todos los casos `SCRAPED_DATA` conserva el HTML y las filas ya extraídas, y
`SCRAPE_RUN` guarda contadores parciales.

### Columnas locales

`SCRAPE_RUN` mantiene dos familias de contadores que no se mezclan:

| Columna | Significado |
|---|---|
| `records_found` / `records_processed` / `records_failed` | Páginas |
| `items_found` | Ítems únicos extraídos (antes de validar) |
| `items_sent` | Ítems aceptados por Ingestion (`processed`) |
| `items_failed` | Ítems enviados que Ingestion o el transporte rechazaron |
| `items_rejected` | Ítems descartados localmente por `validate()` |
| `ingestion_run_id` | Id de `scrape_run` en PostgreSQL |
| `ingestion_error` | Último error de ingesta (máx. 2000 caracteres) |

`DATA_SOURCE.store_id` es la única columna nueva del lado de la fuente; el
resto son columnas nullable con default `0` en la tabla de ejecuciones.

## Logging

Mensajes en inglés con etiqueta por componente:

| Tag | Contenido |
|---|---|
| `[INIT]` | Inicialización de BD, seeds registradas, scheduling de fuentes |
| `[SEED]` | Alta de nuevas `DATA_SOURCE` desde `SCRAPER_SEEDS` |
| `[TASK]` | Inicio/fin de cada run, errores por fuente |
| `[RETRY]` | Reintentos con backoff ante errores transitorios |
| `[INGEST]` | Apertura/cierre de la ejecución remota, lotes, rechazos y `problem+json` |
| `[SHUTDOWN]` | Cancelación de tareas, cierre de hilos y conexiones |
| `[SUMMARY]` | Resumen final: fuentes completadas/fallidas, páginas, errores |

```text
00:21:27 [MainThread] INFO    [INIT] 2 source(s) scheduled across 2 thread(s), 5 worker(s) per source
00:21:34 [MainThread] ERROR   [SHUTDOWN] Ctrl+C received; cancelling all crawl tasks...
```

## Apagado (Ctrl+C)

* `SIGINT` / `SIGTERM` cancelan **inmediatamente** todas las tareas en vuelo
  (salida típica < 1 s, exit code `130`).
* Cada run interrumpido se cierra en BD como `failed` con
  `error_message = 'interrupted by user (Ctrl+C); partial stats persisted'`
  conservando los contadores parciales (`records_found`, `records_processed`).
* Los datos ya extraídos quedan persistidos en `SCRAPED_DATA`; no hay corrupción.

## Tests

```bash
pytest            # suite offline, sin red
ruff check src tests
```

***Nota: Esta parte del proyecto fue un 50% vibecodeada para tenerla como punto de partida.
