# SOLOServis scraper v2

A spec-first scraper for the SOLOServis catalog. It replaces the
configuration-by-dictionary approach of `crawler/` with **versioned store
specs**, stores every run in **SQLite using the existing UML schema**, writes a
**per-store catalog** that is easy to diff, and exposes a **runtime control
API** so Kubernetes can start, inspect and stop crawls without killing a
process mid-page.

The guiding rule is simple: **spec or nothing**. A store without a valid spec
raises before the first HTTP request. There are no fallback selectors, no
guessed URLs and no link discovery the spec did not declare.

The daemon also owns **recurring tasks** and **declared crawling**:

* tasks are JSON files on the PVC (the UML schema keeps its four tables);
* `crawl` tells the crawler which links it may follow, and how deep;
* `detail` enriches a listing item from its product page;
* `match.detect` rejects a page that no longer looks like the store;
* a bounded pool runs several tasks in parallel, with a global kill switch.

---

## 1. Why a new package

| Concern | `crawler/` (v1) | `scraper/` (v2) |
| --- | --- | --- |
| Store definition | `SCRAPER_CONFIG.parser_config` (opaque JSON) | `stores/<slug>/spec.json`, reviewed and hashed |
| Execution model | one process per cron tick | long-running Deployment + recurring tasks + control API |
| Storage | SQLite + per-run JSONL | SQLite (4 UML tables) + per-store catalog + task JSON |
| Output | one JSONL per run | stable `<catalog>/<store>/<product>.json` |
| Stop semantics | process signal | `POST /runs/{id}/stop`, `POST /tasks/stop`, drain in flight |
| Duplicate runs | cron overlap possible | bounded pool + per-store guard + 409 + `Idempotency-Key` |
| Selectors | dict with implicit fallbacks | typed spec, strict unknown-key rejection |
| Discovery | generic link following | only `crawl.linkSelector` / item URLs declared by the spec |
| Task definitions | one implicit config row per store | versioned JSON files, no new tables |

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
│   ├── pipeline.py        # listing + discovery + detail + persistence
│   ├── tasks.py           # recurring task files, schedules, paused state
│   ├── ingestion.py       # Ingestion API client
│   ├── api/
│   │   ├── manager.py     # run pool, scheduler, stop, global controls
│   │   ├── models.py      # request/response bodies
│   │   └── app.py         # FastAPI app
│   ├── cli.py
│   └── __main__.py
└── tests/
    ├── fixtures/<store>/listing.html
    ├── test_tasks.py      # task files and schedules
    ├── test_crawl.py      # discovery, detect, detail merge
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

# 5. With the API running, create a recurring task.
curl -sS -X POST http://localhost:8095/tasks \
  -H "Authorization: Bearer $SCRAPER_API_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"store":"ripley-cl","schedule":{"kind":"interval","every":"1h"}}'
```

`run` writes to `out/` by default:

```
out/
├── scraper.db
├── tasks/                 # task JSON files + state.json (on the PVC)
│   ├── task-1a2b3c4d5e6f.json
│   └── state.json
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

## 3.1 Requirements and store access

### Runtime dependencies (TAL-83)

| Requirement | Why |
| --- | --- |
| Python 3.12+ | `pyproject.toml` (`requires-python = ">=3.12"`) |
| `scrapling[fetchers]` | HTTP fetcher with `curl_cffi` impersonation |
| `libcurl4` | runtime library the fetcher links against |
| `fastapi`, `uvicorn`, `httpx` | control API and ingestion client |

There is **no browser dependency**: no Playwright, no Chromium, no Selenium.
The chosen extraction method is HTTP plus CSS/attribute readers, so installing
a browser would add hundreds of megabytes for nothing. The container installs
`libcurl4` and nothing else:

```dockerfile
RUN apt-get install -y --no-install-recommends libcurl4
```

Verify the environment before crawling:

```bash
PYTHONPATH=src ./venv/bin/python -m scraper validate   # every spec parses
PYTHONPATH=src ./venv/bin/python -m scraper stores     # storeId, name, URL
```

### Store access conditions (TAL-85)

The initial store is **Ripley** (`simple.ripley.cl`): it serves the listing
without a browser, returns the product URL in the `href`, and its fixture is the
one used by the pipeline tests.

| Store | Listing | Access | Notes |
| --- | --- | --- | --- |
| `ripley-cl` | `https://simple.ripley.cl/search/refrigerador` | Works | `storeId = 4`. One listing page returns ~57 items |
| `falabella-cl` | `https://www.falabella.com/falabella-cl` | Works | `storeId = 1`. `href` is injected by JavaScript, so the spec builds the product URL from `externalId` |
| `mercadolibre-cl` | `https://www.mercadolibre.cl` | **Blocked** | `storeId = 6`. Redirects automated traffic to `/gz/account-verification`; its runs fail without affecting the other stores |

A real run is the acceptance test:

```bash
PYTHONPATH=src ./venv/bin/python -m scraper run --store ripley-cl
jq '.counters' out/runs/<id>/run.json   # itemsFound > 0, itemsSent > 0
```

If `itemsSent` is `0` while `itemsFound > 0`, check `ingestion_disabled_reason`
or the `ingestionError` field before blaming the store.

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
| `schemaVersion` | yes | `"1.0"` or `"1.1"`. New blocks require `"1.1"`. |
| `store.slug` | yes | `^[a-z0-9][a-z0-9-]{0,63}$`, unique. |
| `store.name` | yes | Human label, also `DATA_SOURCE.name`. |
| `store.baseUrl` | yes | Absolute `http(s)` origin. |
| `store.storeId` | yes | `>= 1`, must match `DATA_SOURCE.store_id`. |
| `match.hosts` | yes | Non-empty hostnames, lowercase. |
| `match.pathPrefixes` | no | Paths starting with `/`; empty means the whole host. |
| `match.detect` | no | Selector (or `{selector, minMatches}`) that proves the page belongs to the store. Requires `"1.1"`. |
| `listing.url` | yes | Absolute `http(s)` URL. |
| `listing.root` | no | CSS selector for one card. Omit for a single-item page. |
| `listing.pagination` | no | `mode: none` by default. |
| `listing.allowEmpty` | no | `false`: an empty first page fails the run. |
| `item.externalId` | no | `ValueSpec`; a selector or an attribute. |
| `item.externalUrl` | yes | `ValueSpec`; selector, attribute or template. |
| `item.fields` | yes | Contract paths → field specs (§4.3). |
| `crawl` | no | Declared link discovery (§4.6). Requires `"1.1"`. |
| `detail` | no | Product-page enrichment (§4.7). Requires `"1.1"`. |
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

`regex` (optional, only for values read from the page) keeps the part of the
text that matches, after the transform and before type coercion: the first
capture group when the pattern has one, otherwise the whole match. Matching
ignores case. No match means the value is absent, not an error. It is how a
required category attribute is read out of a product name:

```json
"product.attributes.capacidad_litros": {
  "selector": ".product-item--name",
  "regex": "(\\d{2,4})\\s*(?:lts?|litros?|l)\\b",
  "type": "integer"
}
```

`"REFRIGERADOR SIDE BY SIDE 664 LITROS"` becomes `664`. A constant, a template
or `mode: "list"` cannot declare `regex`, and an invalid pattern is rejected at
load time.

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

### 4.6 Declared discovery (`crawl`)

`schemaVersion` must be `"1.1"`. The block is optional; without it the crawler
only walks `listing.pagination`.

```json
"crawl": {
  "linkSelector": "a.product-link",
  "linkAttribute": "href",
  "maxDepth": 2,
  "maxPages": 100,
  "allow": ["/product/", "/catalogo/"],
  "deny": ["/cart", "/checkout", "/search"],
  "sameHost": true,
  "followItems": true
}
```

| Key | Default | Meaning |
| --- | --- | --- |
| `linkSelector` | none | CSS selector whose matches are candidate links. There is no generic `a[href]` fallback. |
| `linkAttribute` | `href` | Attribute read from each match. |
| `maxDepth` | `1` | Link depth from the listing page. Capped at 10. |
| `maxPages` | `50` | Pages fetched during the crawl phase. Capped by `SCRAPER_MAX_CRAWL_PAGES`. |
| `allow` | `[]` | Path prefixes that may be followed. Empty means all paths allowed by `match`. |
| `deny` | `[]` | Path prefixes never followed. `deny` wins over `allow`. |
| `sameHost` | `true` | Restrict links to the `listing.url` host. Multi-host stores must set `false`. |
| `followItems` | `true` | Follow the `externalUrl` of every item the listing produced. |

Rules that are always enforced:

1. A discovered URL must match `spec.match` (host + `pathPrefixes`). A link to
   another store is never fetched, even if the selector matches it.
2. `deny` wins over `allow`; both are path prefixes, not regular expressions.
   They filter *discovered* links. An item's `externalUrl` is declared by the
   `item` block, so it is followed when it matches `spec.match` (and
   `crawl.sameHost`, when set), even if it falls outside `allow`.
3. A URL is fetched once per run (`seen` set), so loops are harmless.
4. At `maxDepth` the crawler stops discovering new links, but a detail page
   reached at that depth is still enriched.
5. `crawl` without `linkSelector` is valid only when `followItems` is true and
   a `detail` block exists: it then visits exactly the item URLs, which is the
   common detail-page case.

If `detail` is declared without `crawl`, the crawler visits item URLs at depth
1 and extracts detail fields; it does not discover any other link.

### 4.7 Detail pages (`detail`)

```json
"detail": {
  "root": "main.product-detail",
  "detect": "main.product-detail",
  "externalId": { "attribute": "data-product-id" },
  "fields": {
    "product.description": { "selector": ".description", "type": "string" },
    "product.sku": { "selector": ".sku", "type": "string" },
    "product.attributes.color": { "selector": ".color", "type": "string" }
  }
}
```

* `root` scopes the selectors to the detail container.
* `detect` is an optional CSS check; without it, a page is a detail candidate
  only when its URL matches an item's `externalUrl`.
* `externalId` lets a detail page match an item whose URL changed.
* `fields` accepts the same contract paths and types as `item.fields`.

Merge policy:

* The listing is the source of truth for **which items exist**. A detail page
  never creates a new item.
* A readable detail value wins over the listing value. A missing selector leaves
  the listing value untouched.
* If no item matches the detail page, the values are ignored and a warning is
  recorded in `run.json`.
* The merged item is validated with `validate_item`. If the merge would make it
  invalid, the merge is reverted as a whole and a warning is recorded.
* Detail extraction failures never fail the run; only the listing phase (or a
  stop request) can do that.

### 4.8 Page detection (`match.detect`)

```json
"match": {
  "hosts": ["simple.ripley.cl"],
  "pathPrefixes": ["/"],
  "detect": { "selector": "body.store-ripley", "minMatches": 1 }
}
```

Every fetched page (listing, crawl and detail) is checked before extraction.
A page that does not match is rejected and counted in `pagesFailed`; the
listing phase fails the run with a clear error, while a crawl/detail page only
adds a warning. This catches soft-404 pages, bot walls and wrong redirects
before a stale selector produces garbage items.

The selector must match **any** page of the store, not just the listing. If it
is listing-specific, leave `match.detect` out and use `detail.detect` for the
detail container.

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

### 5.0 Tasks are files, not tables

The UML has no task table, and adding one is not an option. A task is therefore
a small, versioned JSON file under `<data>/tasks/`:

```json
{
  "schemaVersion": "1.0",
  "id": "task-1a2b3c4d5e6f",
  "store": "ripley-cl",
  "name": "Ripley hourly",
  "enabled": true,
  "schedule": { "kind": "interval", "everySeconds": 3600 },
  "maxRuns": null,
  "createdAt": "2026-11-04T03:00:00Z",
  "updatedAt": "2026-11-04T03:00:00Z",
  "lastRunId": 42,
  "lastRunAt": "2026-11-04T04:00:02Z",
  "lastStatus": "success",
  "nextRunAt": "2026-11-04T05:00:02Z",
  "runCount": 7,
  "consecutiveFailures": 0,
  "runIds": [36, 42]
}
```

* Files are written atomically (`mkstemp` + `fsync` + `os.replace`).
* Unknown keys, invalid timestamps and invalid schedules are hard errors.
* `state.json` holds the global `paused` flag, so a kill switch survives a pod
  restart.
* The task record keeps the last `SCRAPER_TASK_HISTORY_LIMIT` run ids; there is
  no extra table and no change to `SCRAPE_RUN`.
* `run.json` includes `taskId`, so a run can be traced back to the task that
  started it without touching SQLite.

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
* `_index.json` is the manifest: store, run id, timestamp, `complete`, count
  and one entry per file.
* `_changes.jsonl` is append-only: `added`, `updated`, `removed`.
* A `success` run removes files for products that disappeared. A failed or
  stopped run **never deletes** anything, because absence may mean "not
  reached". The same applies to a crawl that hit `maxPages` or had a failed
  page: the run can still be `success`, but its catalog is written with
  `complete=false` and no file is removed.

Rebuild the catalog from the DB at any time:

```bash
PYTHONPATH=src ./venv/bin/python -m scraper export --store ripley-cl
```

---

## 7. Control API

Internal only. The Go backend proxies `/admin/scraper/*` to `scraper-api:8095`;
the Service is not exposed through the Ingress.

### 7.1 Runs

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/health` | Liveness, no auth. |
| `GET` | `/ready` | DB + specs loaded, no auth. |
| `GET` | `/stores` | Known specs. |
| `POST` | `/runs` | `{"store": "ripley-cl"}` or `{"url": "https://…"}`. |
| `GET` | `/runs?limit=50` | Recent runs. |
| `GET` | `/runs/{id}` | One run. |
| `POST` | `/runs/{id}/stop` | Controlled stop. |

```bash
curl -sS -X POST http://scraper-api:8095/runs \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: schedule-2026-11-03-ripley' \
  -d '{"store":"ripley-cl"}'

curl -sS http://scraper-api:8095/runs/42 -H "Authorization: Bearer $TOKEN"
curl -sS -X POST http://scraper-api:8095/runs/42/stop -H "Authorization: Bearer $TOKEN"
```

Auth: `Authorization: Bearer $SCRAPER_API_TOKEN` (falls back to
`INTERNAL_API_TOKEN`). If no token is configured, every protected endpoint
(`/stores`, `/runs`, `/tasks`) answers `503`; health and readiness stay
available for probes.

### 7.2 Recurring tasks

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/tasks` | List tasks with their last run state. |
| `POST` | `/tasks` | Create a task (`201`). |
| `GET` | `/tasks/status` | Global scheduler state. |
| `POST` | `/tasks/start` | Resume the scheduler (`paused: false`). |
| `POST` | `/tasks/stop` | Pause the scheduler **and** stop every active run. |
| `POST` | `/tasks/run` | Run every enabled task now (refuses while paused). |
| `GET` | `/tasks/{id}` | One task. |
| `PATCH` | `/tasks/{id}` | Change `name`, `enabled`, `schedule` or `maxRuns`. |
| `DELETE` | `/tasks/{id}` | Delete a task; an active run is stopped first. |
| `POST` | `/tasks/{id}/run` | Start this task now (`202`). |
| `POST` | `/tasks/{id}/stop` | Stop this task's active run. |

Create an hourly task:

```bash
curl -sS -X POST http://scraper-api:8095/tasks \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "store": "ripley-cl",
    "name": "Ripley refrigeradores",
    "schedule": {"kind": "interval", "every": "1h"},
    "maxRuns": null,
    "runImmediately": false
  }'
```

Schedule formats:

| Body | Meaning |
| --- | --- |
| `{"kind": "interval", "every": "30s"}` | Every 30 seconds. |
| `{"kind": "interval", "every": "5m"}` | Every 5 minutes. |
| `{"kind": "interval", "every": "2h"}` | Every 2 hours. |
| `{"kind": "interval", "everySeconds": 3600}` | Same as `1h`. |
| `{"kind": "manual"}` | Only `/tasks/{id}/run` starts it. |

Other task fields:

* `enabled` — `false` keeps the file but the scheduler ignores it. An explicit
  `POST /tasks/{id}/run` still works as a manual override.
* `maxRuns` — after this many completed runs the task disables itself.
* `runImmediately` — creates the task and starts it immediately (subject to the
  global brake and the per-store guard).

A task response looks like this:

```json
{
  "id": "task-1a2b3c4d5e6f",
  "store": "ripley-cl",
  "name": "Ripley refrigeradores",
  "enabled": true,
  "schedule": { "kind": "interval", "everySeconds": 3600 },
  "maxRuns": null,
  "createdAt": "2026-11-04T03:00:00Z",
  "updatedAt": "2026-11-04T05:00:02Z",
  "lastRunId": 42,
  "lastRunAt": "2026-11-04T04:00:02Z",
  "lastStatus": "success",
  "nextRunAt": "2026-11-04T05:00:02Z",
  "runCount": 7,
  "consecutiveFailures": 0,
  "runIds": [36, 42],
  "running": false,
  "activeRunId": null
}
```

`running`/`activeRunId` are computed by the daemon; everything else is stored
in the task file.

`nextRunAt` is computed from the **completion** of the previous run, so a slow
crawl cannot pile up overlapping runs. A task whose `nextRunAt` is in the past
on startup runs on the first scheduler tick.

Schedules are **intervals**, not cron expressions. A `24h` task runs 24 hours
after the previous completion; it does not pin a local clock time. If you need
a fixed local window, create the task at that time, or keep the legacy
`scraper-trigger` CronJob for the wall-clock trigger.

### 7.3 Parallelism and the global kill switch

* `SCRAPER_MAX_PARALLEL_TASKS` bounds how many runs execute at once. The
  default is 2.
* Runs for **different stores** can execute in parallel. Two runs for the same
  store are serialized: `POST /runs` and `/tasks/{id}/run` answer `409` while
  that store has a `pending`/`running` row. This keeps one catalog writer per
  store.
* `POST /tasks/stop` is the brake: it sets `paused` (persisted in
  `tasks/state.json`), stops every active run with the normal controlled-stop
  protocol, and cancels queued work. `POST /runs`, `POST /tasks/run` and
  `/tasks/{id}/run` return `409` until `POST /tasks/start`. This also blocks a
  CronJob fan-out while the brake is on.
* `POST /tasks/start` resumes scheduling. It does not retroactively start every
  task; tasks run when their `nextRunAt` is due. Use `POST /tasks/run` for an
  immediate fan-out.
* `GET /tasks/status` returns `paused`, `tasks`, `enabledTasks`, `activeRuns`,
  `activeTaskIds`, `triggeredRunIds` and `skippedTaskIds`.

### 7.4 Duplicate prevention

* A bounded pool consumes the runs; the same store cannot have two open runs.
* If the same store already has a `pending` or `running` row, `POST /runs`
  returns `409` instead of starting a duplicate.
* A task cannot have two open runs, even if the scheduler and an operator press
  "run" at the same time.
* `Idempotency-Key` makes a retried scheduler request return the same run id.

### 7.5 Controlled stop

`POST /runs/{id}/stop` (or `/tasks/stop`, or SIGTERM) sets an
`asyncio.Event`. The pipeline checks it before each page, finishes the page in
flight, writes the partial catalog without deletions, and finalizes the run as
`failed` with `stopped by request`. If the grace period
(`SCRAPER_STOP_TIMEOUT`) expires, the run is finalized as failed and the worker
moves on; the process is not killed abruptly.

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
| `SCRAPER_TASKS_DIR` | `<data>/tasks` | Task JSON files and `state.json`. |
| `SCRAPER_MAX_CONCURRENCY` | `4` | Pages fetched concurrently in the crawl phase. |
| `SCRAPER_MAX_PAGES` | `20` | Hard cap on listing pagination. |
| `SCRAPER_MAX_CRAWL_PAGES` | `200` | Hard cap on the crawl/detail phase. |
| `SCRAPER_MAX_PARALLEL_TASKS` | `2` | Runs that may execute at the same time. |
| `SCRAPER_SCHEDULER_ENABLED` | `true` | Start the recurring scheduler with `serve`. |
| `SCRAPER_SCHEDULER_TICK` | `5` | Seconds between scheduler checks. |
| `SCRAPER_TASK_HISTORY_LIMIT` | `50` | Run ids kept in each task file. |
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
| `test_crawl.py` | declared discovery, allow/deny, `match.detect`, detail merge, `schemaVersion` |
| `test_tasks.py` | duration parsing, task JSON, atomic writes, paused state |
| `test_task_api.py` | task CRUD, parallel runs, per-store conflict, global stop/start/run |
| `test_api.py` | auth, pool, 409, idempotency, stop, 404/422 |

---

## 11. Deployment

The scraper runs as a **Deployment**, not a CronJob, because it is a service
with state and a stop protocol.

```
Deployment scraper-api (1 replica, strategy Recreate)
  ├── scheduler (recurring tasks)
  ├── run pool (SCRAPER_MAX_PARALLEL_TASKS)
  └── PVC soloservis-scraper-data (ReadWriteOnce) mounted at /data
        └── /data/tasks/*.json + state.json
Service scraper-api:8095
CronJob scraper-trigger (optional legacy fan-out) ──► POST /runs per store
```

* `Recreate` + RWO PVC avoids two pods writing the same SQLite file.
* `SCRAPER_DATA_DIR=/data` keeps the DB, catalog, runs and **task files** on
  the PVC.
* The scheduler is part of the Deployment. Tasks are created through
  `POST /tasks`; they survive a pod restart because they are files, not
  in-memory queue entries.
* The trigger CronJob is now optional. It still works (`POST /runs` per store
  with an `Idempotency-Key`), but it bypasses tasks and duplicates their
  schedule. Remove it from `k8s/base/kustomization.yaml` once every store has a
  task, or keep it as a manual daily safety net.
* `readOnlyRootFilesystem` is safe because `/data` is the only writable path.
* Liveness `/health`, readiness `/ready`.

Manifests live in `k8s/base/scraper/`. The image is built from
`docker/scraper/Dockerfile`.

### 11.1 Schedule change

v1 ran the crawl inside the CronJob. v2 can schedule from inside the daemon:

1. An operator creates a task with `POST /tasks`.
2. The scheduler wakes every `SCRAPER_SCHEDULER_TICK` seconds and starts due
   tasks through the bounded pool.
3. The run completes, the task file is updated with `lastRunId`, `lastStatus`
   and the next `nextRunAt`.
4. The CronJob is no longer required. If it is kept, overlapping triggers are
   harmless: the second one gets a `409` (or the same run id with an
   idempotency key) instead of a second crawl.

To stop everything in an emergency:

```bash
curl -sS -X POST http://scraper-api:8095/tasks/stop \
  -H "Authorization: Bearer $TOKEN"
```

To resume:

```bash
curl -sS -X POST http://scraper-api:8095/tasks/start \
  -H "Authorization: Bearer $TOKEN"
```

---

## 12. Adding a store

1. Create `stores/<slug>/spec.json` following §4.
2. Add a fixture in `tests/fixtures/<slug>/listing.html` (plus detail fixtures
   if the spec declares `detail`).
3. Run `scraper validate`; it must print the new slug.
4. Add extraction assertions in `tests/test_extract.py`.
5. Run the crawl once with `scraper run --store <slug>` and inspect
   `out/rejected/run-*.jsonl` if items are missing. With `crawl`/`detail`, also
   inspect `warnings` in `out/runs/<id>/run.json`.
6. Create a task only after the one-off crawl is clean:
   `POST /tasks {"store": "<slug>", "schedule": {"kind": "interval", "every": "1h"}}`.
7. Open the PR with the spec and fixture in the same commit. The spec digest
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
| `409 the scheduler is paused` | `POST /tasks/start` before running tasks. |
| `409 task already has an open run` | The scheduler and an operator raced; wait or stop the task. |
| `did not match match.detect` | The page is a soft-404, bot wall or the selector drifted. Inspect the saved HTML. |
| A detail page adds no fields | `detail.root`/`detail.fields` drifted, or the page did not match any item. Check `warnings` in `run.json`. |
| `detail merge rejected` | The detail value would break the ingestion contract; the listing value was kept. |
| A crawl stops early | `maxPages`/`maxDepth`, `allow`/`deny`, or `spec.match` rejected the rest. Check `warnings`. |
| Task never runs | `enabled` is false, `nextRunAt` is in the future, `paused` is true, or the store already has an open run. |
| Task file is rejected at startup | Unknown key, bad timestamp or invalid schedule; fix or delete the file. |
| `503 SCRAPER_API_TOKEN is not configured` | Set the token; probes still work. |
| Ingestion `401` | Token shorter than 32 characters or wrong secret. |
| Files not updating | The product content is identical; only `provenance` changed. |

---

## 14. Ticket map

| Ticket | Where it lives |
| --- | --- |
| TAL-79 — item contract | `validate.py`, `models.py`, `docs/estandares/formato-datos-extraidos.md` |
| TAL-83 — runtime dependencies | §3.1, `pyproject.toml`, `docker/scraper/Dockerfile` |
| TAL-85 — initial store access | §3.1, `stores/ripley-cl/spec.json`, `tests/fixtures/ripley-cl/` |
| TAL-86 — normalized output | `models.Item.to_dict()` / `to_ingestion()` |
| TAL-90 — per-store output | `output/catalog.py` |
| TAL-129 — runtime control API | `api/app.py`, `api/manager.py` |
| TAL-130 — start/status/stop | `POST /runs`, `GET /runs/{id}`, `POST /runs/{id}/stop` |
| TAL-132 — ingestion client | `ingestion.py` |
| TAL-136 — catalog export | `scraper export`, `output/catalog.py` |
| TAL-137 — duplicate prevention | bounded pool, per-store guard, 409, `Idempotency-Key`, orphan recovery |
| TAL-144 — deployment | `k8s/base/scraper/`, `docker/scraper/Dockerfile` |
| Recurring tasks | `tasks.py`, `api/manager.py`, `POST /tasks` |
| Declared crawling | `spec/schema.py` (`crawl`, `detail`, `match.detect`), `pipeline.py` |
