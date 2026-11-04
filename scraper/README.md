# SOLOServis scraper v2

A spec-first scraper for the SOLOServis catalog. It replaces the
configuration-by-dictionary approach of `crawler/` with **versioned store
specs**, stores every run in **SQLite using the existing UML schema**, writes a
**per-store catalog** that is easy to diff, and exposes a **runtime control
API** so Kubernetes can start, inspect and stop crawls without killing a
process mid-page.

The guiding rule is simple: **spec or nothing**. A store without a valid spec
raises before the first HTTP request. There are no fallback selectors, no
guessed URLs and no generic link discovery.

---

## 1. Why a new package

| Concern | `crawler/` (v1) | `scraper/` (v2) |
| --- | --- | --- |
| Store definition | `SCRAPER_CONFIG.parser_config` (opaque JSON) | `stores/<slug>/spec.json`, reviewed and hashed |
| Execution model | one process per cron tick | long-running Deployment + control API |
| Storage | SQLite + per-run JSONL | SQLite (4 UML tables) + per-store catalog |
| Output | one JSONL per run | stable `<catalog>/<store>/<product>.json` |
| Stop semantics | process signal | `POST /runs/{id}/stop`, drain in flight |
| Duplicate runs | cron overlap possible | queue + 409 + `Idempotency-Key` |
| Selectors | dict with implicit fallbacks | typed spec, strict unknown-key rejection |

Both packages can coexist while v2 reaches parity; v1 is removed afterwards.

---

## 2. Package layout

```
scraper/
├── pyproject.toml
├── stores/
│   ├── falabella-cl/spec.json
│   ├── ripley-cl/spec.json
│   └── mercadolibre-cl/spec.json
├── src/scraper/
│   ├── config.py          # Settings from the environment
│   ├── errors.py          # exception hierarchy
│   ├── models.py          # StoreSpec, Item, RunRecord, RunState
│   ├── validate.py        # the ingestion item contract (TAL-79)
│   ├── spec/
│   │   ├── schema.py      # strict parser
│   │   └── loader.py      # load, hash, resolve URL -> spec
│   ├── extract/
│   │   ├── fields.py      # CSS / attribute / text readers
│   │   ├── transforms.py  # money, boolean, URL, slug, whitespace
│   │   └── engine.py      # card -> Item, rejects
│   ├── fetch/
│   │   ├── client.py      # async fetch + retries
│   │   └── limiter.py     # shared request spacing
│   ├── db/
│   │   ├── schema.sql     # the four UML tables + indexes
│   │   └── repository.py  # all SQLite access
│   ├── output/
│   │   ├── catalog.py     # per-store JSON files, index, change log
│   │   └── raw.py         # page HTML, run.json, rejects
│   ├── pipeline.py        # listing + pagination + persistence
│   ├── ingestion.py       # Ingestion API client
│   ├── api/
│   │   ├── manager.py     # queue, state machine, stop
│   │   ├── models.py      # request/response bodies
│   │   └── app.py         # FastAPI app
│   ├── cli.py
│   └── __main__.py
└── tests/
    ├── fixtures/<store>/listing.html
    └── test_*.py
```

---

## 3. Quick start

```bash
cd scraper
python3.12 -m venv venv
./venv/bin/pip install -e '.[dev]'

# 1. Every spec must parse before anything else.
PYTHONPATH=src ./venv/bin/python -m scraper validate

# 2. Create the SQLite file (idempotent).
PYTHONPATH=src ./venv/bin/python -m scraper init-db

# 3. Run one store in the foreground.
PYTHONPATH=src ./venv/bin/python -m scraper run --store ripley-cl

# 4. Or start the control API.
export SCRAPER_API_TOKEN="$(openssl rand -hex 32)"
PYTHONPATH=src ./venv/bin/python -m scraper serve
```

`run` writes to `out/` by default:

```
out/
├── scraper.db
├── catalog/
│   └── ripley-cl/
│       ├── rip-2001-3e7a5f548f.json
│       ├── _index.json
│       └── _changes.jsonl
├── runs/
│   └── 1/
│       ├── run.json
│       └── pages/001-2.html
└── rejected/
    └── run-1.jsonl
```

`out/` is ignored by git. In production it lives on a PersistentVolumeClaim.

---

## 4. The store spec

One file per store: `stores/<slug>/spec.json`. The parser rejects unknown keys,
unknown field paths, unsupported versions and ambiguous match rules. A typo is
a load error, never a silently ignored selector.

### 4.1 Top level

```json
{
  "schemaVersion": "1.0",
  "store": {
    "slug": "ripley-cl",
    "name": "Ripley Chile",
    "baseUrl": "https://simple.ripley.cl",
    "storeId": 2
  },
  "match": {
    "hosts": ["simple.ripley.cl"],
    "pathPrefixes": ["/"]
  },
  "listing": {
    "url": "https://simple.ripley.cl/search/refrigerador",
    "root": "a.product-link",
    "pagination": { "mode": "next-link", "selector": "a[rel=\"next\"]", "maxPages": 20 },
    "allowEmpty": false
  },
  "item": {
    "externalId": { "attribute": "data-id" },
    "externalUrl": { "attribute": "href", "transform": "absolute_url" },
    "fields": { "...": "§4.3" }
  },
  "rules": {
    "dedupeBy": "externalId",
    "rejectIfMissing": ["externalUrl", "product.name", "offer.price"]
  }
}
```

| Key | Required | Rule |
| --- | --- | --- |
| `schemaVersion` | yes | Exactly `"1.0"`. |
| `store.slug` | yes | `^[a-z0-9][a-z0-9-]{0,63}$`, unique. |
| `store.name` | yes | Human label, also `DATA_SOURCE.name`. |
| `store.baseUrl` | yes | Absolute `http(s)` origin. |
| `store.storeId` | yes | `>= 1`, must match `DATA_SOURCE.store_id`. |
| `match.hosts` | yes | Non-empty hostnames, lowercase. |
| `match.pathPrefixes` | no | Paths starting with `/`; empty means the whole host. |
| `listing.url` | yes | Absolute `http(s)` URL. |
| `listing.root` | no | CSS selector for one card. Omit for a single-item page. |
| `listing.pagination` | no | `mode: none` by default. |
| `listing.allowEmpty` | no | `false`: an empty first page fails the run. |
| `item.externalId` | no | `ValueSpec`; a selector or an attribute. |
| `item.externalUrl` | yes | `ValueSpec`; selector, attribute or template. |
| `item.fields` | yes | Contract paths → field specs (§4.3). |
| `rules.dedupeBy` | no | `externalId` (default) or `externalUrl`. |
| `rules.rejectIfMissing` | no | Defaults to every `required` field plus `externalUrl`. |

### 4.2 Reading a value (`ValueSpec`)

A value comes from exactly one of:

| Form | Meaning |
| --- | --- |
| `{"selector": ".price"}` | Text of the first match, whitespace collapsed. |
| `{"selector": ".price", "mode": "html"}` | `innerHTML` of the first match. |
| `{"selector": ".price", "mode": "list"}` | Text of every match (attributes only). |
| `{"selector": "img", "attribute": "src"}` | Attribute of the first match. |
| `{"attribute": "data-id"}` | Attribute of the card itself (no selector). |
| `{"value": "CLP"}` | Constant. |
| `{"template": "https://…/product/{externalId}"}` | Built from already-read values. |

Rules enforced at load time:

* `selector`, `value` and `template` are mutually exclusive.
* `attribute` implies `mode: "attr"`; `mode: "attr"` requires `attribute`.
* A constant or template cannot also declare `mode`/`attribute`.
* Templates may reference `{externalId}` and **non-template** field paths.
  Chaining templates is rejected, which makes cycles impossible.
* `externalId` cannot be a template.

Transforms (`transform`): `trim`, `collapse_ws`, `lower`, `upper`, `slugify`,
`absolute_url`. They run on the raw string before type coercion.

### 4.3 Fields

Keys are **contract paths**, not arbitrary names:

```
product.name            product.sku             product.model
product.brand           product.categorySlug    product.description
product.imageUrl        product.attributes.<key>
offer.price             offer.listPrice         offer.currency
offer.shippingCost      offer.shippingFree      offer.available
offer.stock             offer.condition         offer.productUrl
```

Each field declares a `type`:

| Type | Result |
| --- | --- |
| `string` | Whitespace-collapsed text. |
| `money` | Decimal, quantized to two places, serialized as `"549990.00"`. |
| `integer` | Whole number; a fractional value is an error. |
| `boolean` | Spanish/English availability words. |
| `url` | Absolute `http(s)` URL, resolved against the page. |
| `slug` | `^[a-z0-9-]{1,64}$`. |

Optional keys per field: `required` (default `false`), `default`, `currency`
(money only, default `CLP`).

Hard contract requirements (rejected at load time if absent):

* `product.name` and `offer.price` must exist and set `"required": true`.
* `product.categorySlug` must exist.
* `externalUrl` must be declared.

`offer.condition` is mapped through Spanish aliases (`nuevo` → `new`,
`usado` → `used`, `reacondicionado` → `refurbished`). `offer.currency` extracts
an ISO-4217 code from labels such as `"$usd"`.

### 4.4 Pagination

| Mode | Behaviour |
| --- | --- |
| `none` | Fetch `listing.url` once. |
| `next-link` | Read `selector`, take its `href`, resolve it against the page. Stop when it is absent, repeated or already visited. |
| `url-template` | Replace `{page}` with `2, 3, …`; the template must contain `{page}`. |

`maxPages` is capped by `SCRAPER_MAX_PAGES`. A repeated URL always stops the
walk, so a broken "next" link cannot loop forever.

### 4.5 Why Falabella uses a template

Falabella injects the product `href` with JavaScript, so the listing HTML has
the card `id` but no usable link. The spec therefore declares:

```json
"externalUrl": {
  "template": "https://www.falabella.com/falabella-cl/product/{externalId}"
}
```

That is an explicit, reviewed rule. v2 never falls back to the listing URL: an
item without a real product URL is rejected.

---

## 5. Data model

The scraper writes **only** the four UML tables. No new tables or columns.

| Table | What the scraper stores |
| --- | --- |
| `DATA_SOURCE` | One row per store (`name`, `base_url`, `source_type='web'`, `store_id`). |
| `SCRAPER_CONFIG` | One row per source; `parser_config` stores only `{spec, specVersion, specHash}`. |
| `SCRAPE_RUN` | One row per execution, with page/item counters and status. |
| `SCRAPED_DATA` | One row per page (`entity_type='webpage'`, HTML) and per item (`entity_type='product'`, canonical item JSON). |

`SCRAPE_RUN.status` is one of `pending | running | success | failed`. The API-only
`stopping` state is never persisted.

### 5.1 Run lifecycle

```
pending ──► running ──► success
   │           │
   │           ├──► failed   (fetch error, empty listing, every card rejected)
   │           └──► failed   ("stopped by request")
   └──► failed              ("cancelled before start")
```

* On startup, any `pending`/`running` row is marked `failed` with
  `aborted: process restarted`. A single replica cannot legitimately own an
  open row after a restart.
* The run snapshot (`run.json`) records the spec digest, counters, catalog
  summary, reject file and error.
* `run_items()` reads items back from `SCRAPED_DATA`; that is what `export`
  uses to rebuild files.

### 5.2 Why SQLite

SQLite with WAL and `busy_timeout=30000` lets the control API read while a run
writes. Every write goes through `asyncio.to_thread`, so the event loop is never
blocked. The DB is the source of truth; the catalog is a projection.

---

## 6. Catalog output

For each store, one file per product:

```
<catalog>/<store>/<slug-of-key>-<hash10>.json
<catalog>/<store>/_index.json
<catalog>/<store>/_changes.jsonl
```

* The filename is stable for a given `externalId`/`externalUrl`.
* Files are written atomically (`mkstemp` + `fsync` + `os.replace`).
* A product whose **product/offer content** did not change is not rewritten.
  `provenance` is excluded from the comparison (it changes every run) but is
  still written when the product does change.
* `_index.json` is the manifest: store, run id, timestamp, count and one entry
  per file.
* `_changes.jsonl` is append-only: `added`, `updated`, `removed`.
* A `success` run removes files for products that disappeared. A failed or
  stopped run **never deletes** anything, because absence may mean "not
  reached".

Rebuild the catalog from the DB at any time:

```bash
PYTHONPATH=src ./venv/bin/python -m scraper export --store ripley-cl
```

---

## 7. Control API

Internal only. The Go backend proxies `/admin/scraper/*` to `scraper-api:8095`;
the Service is not exposed through the Ingress.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/health` | Liveness, no auth. |
| `GET` | `/ready` | DB + specs loaded, no auth. |
| `GET` | `/stores` | Known specs. |
| `POST` | `/runs` | `{"store": "ripley-cl"}` or `{"url": "https://…"}`. |
| `GET` | `/runs?limit=50` | Recent runs. |
| `GET` | `/runs/{id}` | One run. |
| `POST` | `/runs/{id}/stop` | Controlled stop. |

Auth: `Authorization: Bearer $SCRAPER_API_TOKEN` (falls back to
`INTERNAL_API_TOKEN`). If no token is configured, `/runs` answers `503`; health
and readiness stay available for probes.

```bash
curl -sS -X POST http://scraper-api:8095/runs \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: schedule-2026-11-03-ripley' \
  -d '{"store":"ripley-cl"}'

curl -sS http://scraper-api:8095/runs/42 -H "Authorization: Bearer $TOKEN"
curl -sS -X POST http://scraper-api:8095/runs/42/stop -H "Authorization: Bearer $TOKEN"
```

### 7.1 Duplicate prevention

* A single worker consumes a FIFO queue: two runs never execute at once.
* If the same store already has a `pending` or `running` row, `POST /runs`
  returns `409` instead of starting a duplicate.
* `Idempotency-Key` makes a retried scheduler request return the same run id.

### 7.2 Controlled stop

`POST /runs/{id}/stop` sets an `asyncio.Event`. The pipeline checks it before
each page, finishes the page in flight, writes the partial catalog without
deletions, and finalizes the run as `failed` with `stopped by request`.
SIGTERM follows the same path. If the grace period (`SCRAPER_STOP_TIMEOUT`)
expires, the run is finalized as failed and the worker moves on; the process is
not killed abruptly.

---

## 8. Ingestion

The client speaks the same contract as v1:

1. `POST /ingestion/runs` — opens a remote run with `storeId`.
2. `POST /ingestion/runs/{id}/items` — batches of 1..100.
3. `POST /ingestion/runs/{id}/finish` — `success` or `failed`.

Ingestion is enabled only when `INGESTION_API_URL` is set and
`INGESTION_API_TOKEN` (or `INTERNAL_API_TOKEN`) has at least 32 characters.
Only `success` runs are ingested. A delivery failure does not change the local
run status; it is recorded in `SCRAPE_RUN.ingestion_error`, and the items stay
in `SCRAPED_DATA` and the catalog for replay.

---

## 9. Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `SCRAPER_DATA_DIR` | `out` | Root for DB, catalog, runs and rejects. |
| `SCRAPER_DB_PATH` | `<data>/scraper.db` | SQLite file. |
| `SCRAPER_STORES_DIR` | `stores` | Directory of `<slug>/spec.json`. |
| `SCRAPER_CATALOG_DIR` | `<data>/catalog` | Per-store JSON output. |
| `SCRAPER_RUNS_DIR` | `<data>/runs` | `run.json` and page HTML. |
| `SCRAPER_REJECTS_DIR` | `<data>/rejected` | Rejected cards per run. |
| `SCRAPER_MAX_CONCURRENCY` | `4` | Reserved for concurrent fetches. |
| `SCRAPER_MAX_PAGES` | `20` | Hard cap on pagination. |
| `SCRAPER_RATE_LIMIT` | `60/min` | `<n>/<s|min|h>` spacing between requests. |
| `SCRAPER_TIMEOUT` | `15` | Per-request timeout, seconds. |
| `SCRAPER_MAX_RETRIES` | `3` | Attempts per URL (transient only). |
| `SCRAPER_BACKOFF_BASE` | `1.0` | Exponential backoff base, seconds. |
| `SCRAPER_API_HOST` | `0.0.0.0` | Bind address. |
| `SCRAPER_API_PORT` | `8095` | Bind port. |
| `SCRAPER_API_TOKEN` | `INTERNAL_API_TOKEN` | Bearer token for the control API. |
| `SCRAPER_STOP_TIMEOUT` | `20` | Grace period for a controlled stop. |
| `INGESTION_API_URL` | empty | Ingestion base URL. |
| `INGESTION_API_TOKEN` | `INTERNAL_API_TOKEN` | Bearer token (≥ 32 chars). |
| `INGESTION_TIMEOUT` | `15` | Ingestion request timeout. |
| `INGESTION_BATCH_SIZE` | `100` | Items per batch (1..100). |
| `INGESTION_MAX_RETRIES` | `3` | Retries for transient failures. |

---

## 10. Testing

Tests are offline. Each store has an HTML fixture; the fetch transport is
injected, so no test touches the network.

```bash
cd scraper
./venv/bin/python -m pytest -q
./venv/bin/ruff check src tests
```

Coverage:

| File | What it locks down |
| --- | --- |
| `test_spec.py` | strict keys, required paths, template rules, overlap detection |
| `test_extract.py` | money/URL/boolean parsing against all three fixtures, rejects |
| `test_catalog.py` | atomic files, no rewrite when unchanged, removals only on complete runs |
| `test_pipeline.py` | pagination, dedupe, DB persistence, empty listing, stop, fetch failure |
| `test_api.py` | auth, queue, 409, idempotency, stop, 404/422 |

---

## 11. Deployment

The scraper runs as a **Deployment**, not a CronJob, because it is a service
with state and a stop protocol.

```
Deployment scraper-api (1 replica, strategy Recreate)
  └── PVC soloservis-scraper-data (ReadWriteOnce) mounted at /data
Service scraper-api:8095
CronJob scraper-trigger (schedule) ──► POST /runs per store
```

* `Recreate` + RWO PVC avoids two pods writing the same SQLite file.
* `SCRAPER_DATA_DIR=/data` keeps the DB and catalog on the PVC.
* The trigger CronJob is stateless: it only calls the API. It sends
  `Idempotency-Key: <schedule>-<store>` so a retried job cannot duplicate a run.
* `readOnlyRootFilesystem` is safe because `/data` is the only writable path.
* Liveness `/health`, readiness `/ready`.

Manifests live in `k8s/base/scraper/`. The image is built from
`docker/scraper/Dockerfile`.

### 11.1 Schedule change

v1 ran the crawl inside the CronJob. v2 separates them:

1. The CronJob calls `GET /stores` and `POST /runs` for each slug.
2. The API queues the runs and executes them one at a time.
3. The CronJob exits as soon as the requests are accepted; it never waits for
   the crawl.

This is what makes overlapping schedules harmless: the second trigger gets a
`409` (or the same run id with an idempotency key) instead of a second crawl.

---

## 12. Adding a store

1. Create `stores/<slug>/spec.json` following §4.
2. Add a fixture in `tests/fixtures/<slug>/listing.html`.
3. Run `scraper validate`; it must print the new slug.
4. Add extraction assertions in `tests/test_extract.py`.
5. Run the crawl once with `scraper run --store <slug>` and inspect
   `out/rejected/run-*.jsonl` if items are missing.
6. Open the PR with the spec and fixture in the same commit. The spec digest
   changes, and the next run records the new hash in `run.json`.

Never add a fallback selector to make a page pass. If the store changes, change
the spec and its fixture.

---

## 13. Troubleshooting

| Symptom | Likely cause |
| --- | --- |
| `no store spec matches this URL` | `match.hosts`/`pathPrefixes` do not cover the URL. |
| `matched by several specs` | Two specs claim the same host + prefix; narrow one. |
| `the first listing page produced no cards` | `listing.root` drifted or the page is a bot wall. Check the saved HTML. |
| `every card on the first page was rejected` | A required field selector drifted. Inspect `out/rejected/run-*.jsonl`. |
| `409 store already has an open run` | A previous run is still `pending`/`running`; stop it or wait. |
| `503 SCRAPER_API_TOKEN is not configured` | Set the token; probes still work. |
| Ingestion `401` | Token shorter than 32 characters or wrong secret. |
| Files not updating | The product content is identical; only `provenance` changed. |

---

## 14. Ticket map

| Ticket | Where it lives |
| --- | --- |
| TAL-79 — item contract | `validate.py`, `models.py`, `docs/estandares/formato-datos-extraidos.md` |
| TAL-86 — normalized output | `models.Item.to_dict()` / `to_ingestion()` |
| TAL-90 — per-store output | `output/catalog.py` |
| TAL-129 — runtime control API | `api/app.py`, `api/manager.py` |
| TAL-130 — start/status/stop | `POST /runs`, `GET /runs/{id}`, `POST /runs/{id}/stop` |
| TAL-132 — ingestion client | `ingestion.py` |
| TAL-136 — catalog export | `scraper export`, `output/catalog.py` |
| TAL-137 — duplicate prevention | queue, 409, `Idempotency-Key`, orphan recovery |
| TAL-144 — deployment | `k8s/base/scraper/`, `docker/scraper/Dockerfile` |
