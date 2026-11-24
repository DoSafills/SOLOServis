"""Domain models shared by the spec loader, the pipeline, the DB and the API.

Two families live here:

* the **store spec** (immutable, loaded from ``stores/<slug>/spec.json``);
* the **item** and **run** records, whose JSON shapes are the public contract
  with Ingestion and with the control API.
"""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from decimal import Decimal
from pathlib import Path
from typing import Any
from urllib.parse import urlparse

SCHEMA_VERSION = "1.0"

# Limits transcribed from the ingestion contract and the production schema.
MAX_EXTERNAL_ID = 255
MAX_NAME = 255
MAX_SKU = 100
MAX_MODEL = 150
MAX_BRAND = 150
MAX_DESCRIPTION = 5000
MAX_URL = 2048
MAX_SLUG = 64
MAX_SOURCE_NAME = 150
MAX_PROVIDER_NAME = 150
MAX_SUMMARY = 2000

CONDITIONS = frozenset({"new", "used", "refurbished"})

#: Períodos de cobro que acepta la Ingestion API para una oferta de servicio.
SERVICE_BILLING_PERIODS = frozenset({"monthly", "yearly", "one_time"})

#: Objetivos de corrida del scraper y de ``scraper_config.target_type``.
TARGET_PRODUCT = "product"
TARGET_SERVICE = "service"
TARGETS = frozenset({TARGET_PRODUCT, TARGET_SERVICE})

RUN_STATUS_PENDING = "pending"
RUN_STATUS_RUNNING = "running"
RUN_STATUS_SUCCESS = "success"
RUN_STATUS_FAILED = "failed"
RUN_STATUSES = frozenset(
    {RUN_STATUS_PENDING, RUN_STATUS_RUNNING, RUN_STATUS_SUCCESS, RUN_STATUS_FAILED}
)

# API-only states; the database keeps ``running`` until the run is finalized.
RUN_STATE_STOPPING = "stopping"
RUN_STATE_IDLE = "idle"


# ---------------------------------------------------------------------------
# Store spec
# ---------------------------------------------------------------------------


@dataclass(frozen=True, slots=True)
class DetectRule:
    """A CSS check that proves a fetched page still belongs to the store.

    It is a guard against soft-404 pages, bot walls and wrong redirects. A
    page that does not match is rejected; it is never parsed with stale
    selectors.
    """

    selector: str
    min_matches: int = 1


@dataclass(frozen=True, slots=True)
class MatchRules:
    """Which URLs belong to a store."""

    hosts: tuple[str, ...]
    path_prefixes: tuple[str, ...] = ()
    detect: DetectRule | None = None

    def matches(self, url: str) -> bool:
        parsed = urlparse(url)
        host = (parsed.hostname or "").lower()
        if host not in {item.lower() for item in self.hosts}:
            return False
        if not self.path_prefixes:
            return True
        path = parsed.path or "/"
        return any(path.startswith(prefix) for prefix in self.path_prefixes)


@dataclass(frozen=True, slots=True)
class Pagination:
    """How a listing advances to the next page."""

    mode: str = "none"  # none | next-link | url-template
    selector: str | None = None
    template: str | None = None
    max_pages: int = 1


@dataclass(frozen=True, slots=True)
class Listing:
    """The entry point of a store and how to walk it."""

    url: str
    root: str | None = None
    pagination: Pagination = field(default_factory=Pagination)
    allow_empty: bool = False


@dataclass(frozen=True, slots=True)
class ValueSpec:
    """Where a value comes from: a selector, an attribute, a constant or a template."""

    selector: str | None = None
    mode: str = "text"  # text | attr | html | list
    attribute: str | None = None
    transform: str | None = None
    regex: str | None = None
    value: Any = None
    template: str | None = None


@dataclass(frozen=True, slots=True)
class FieldSpec:
    """One typed field of the item, addressed by its contract path."""

    path: str
    type: str
    source: ValueSpec
    required: bool = False
    default: Any = None
    currency: str | None = None


@dataclass(frozen=True, slots=True)
class Supplement:
    """A second source for one item value, read once per page.

    ``path`` is ``externalUrl`` or a declared field path. ``match_by`` reads the
    product id out of the card and ``value_path`` is the dotted path inside the
    page entry (for example ``image`` inside a JSON-LD ``ListItem``). A
    supplement only fills a value the card did not provide; it never overwrites
    one. It exists because some listings render the images and links in the
    browser while publishing them as structured data in the same HTML.
    """

    path: str
    match_by: ValueSpec
    value_path: str


@dataclass(frozen=True, slots=True)
class CrawlRules:
    """Spec-declared link discovery.

    The crawler never guesses links: it only follows elements matched by
    ``link_selector`` (and, when ``follow_items`` is true, the ``externalUrl``
    of the items the listing already produced). ``allow``/``deny`` are path
    prefixes; ``match`` still has the final say over which URLs are owned by
    the store.
    """

    link_selector: str | None = None
    link_attribute: str = "href"
    max_depth: int = 1
    max_pages: int = 50
    allow: tuple[str, ...] = ()
    deny: tuple[str, ...] = ()
    same_host: bool = True
    follow_items: bool = True


@dataclass(frozen=True, slots=True)
class DetailSpec:
    """How to enrich an item from its product detail page."""

    root: str | None = None
    detect: str | None = None
    external_id: ValueSpec | None = None
    fields: tuple[FieldSpec, ...] = ()


@dataclass(frozen=True, slots=True)
class StoreSpec:
    """A versioned definition of how one store is scraped."""

    schema_version: str
    slug: str
    name: str
    base_url: str
    store_id: int
    match: MatchRules
    listing: Listing
    external_id: ValueSpec | None
    external_url: ValueSpec | None
    fields: tuple[FieldSpec, ...]
    supplements: tuple[Supplement, ...] = ()
    dedupe_by: str = "externalId"
    reject_if_missing: tuple[str, ...] = ()
    crawl: CrawlRules | None = None
    detail: DetailSpec | None = None
    #: ``product`` (por defecto) o ``service``. Un spec de servicio declara el
    #: proveedor y campos ``service.*``; no usa ``storeId``.
    target_type: str = TARGET_PRODUCT
    provider_slug: str = ""
    provider_name: str = ""
    digest: str = ""
    path: Path | None = None

    def field(self, path: str) -> FieldSpec | None:
        for spec in self.fields:
            if spec.path == path:
                return spec
        return None

    def snapshot(self) -> dict[str, Any]:
        """The exact spec that produced a run, ready for ``run.json``."""
        return {
            "schemaVersion": self.schema_version,
            "slug": self.slug,
            "name": self.name,
            "targetType": self.target_type,
            "storeId": self.store_id,
            "provider": self.provider_slug or None,
            "digest": self.digest,
            "path": str(self.path) if self.path else None,
            "crawl": self.crawl is not None,
            "detail": self.detail is not None,
        }


# ---------------------------------------------------------------------------
# Item (the Ingestion contract)
# ---------------------------------------------------------------------------


def _compact(payload: dict[str, Any]) -> dict[str, Any]:
    return {key: value for key, value in payload.items() if value is not None and value != ""}


def money_text(value: Any) -> str | None:
    """Render an amount as the fixed two-decimal string NUMERIC(12,2) expects."""
    if value is None:
        return None
    if isinstance(value, Decimal):
        number = value
    else:
        try:
            number = Decimal(str(value))
        except Exception:
            return str(value)
    try:
        return str(number.quantize(Decimal("0.01")))
    except Exception:
        return str(value)


@dataclass(slots=True)
class Product:
    name: str = ""
    sku: str | None = None
    model: str | None = None
    brand: str | None = None
    category_slug: str | None = None
    description: str | None = None
    image_url: str | None = None
    attributes: dict[str, Any] = field(default_factory=dict)

    def to_dict(self) -> dict[str, Any]:
        payload: dict[str, Any] = {
            "name": self.name,
            "sku": self.sku,
            "model": self.model,
            "brand": self.brand,
            "categorySlug": self.category_slug,
            "description": self.description,
            "imageUrl": self.image_url,
        }
        if self.attributes:
            payload["attributes"] = {
                key: _json_value(value) for key, value in sorted(self.attributes.items())
            }
        return _compact(payload)


@dataclass(slots=True)
class Offer:
    price: str = ""
    list_price: str | None = None
    currency: str = "CLP"
    shipping_cost: str = "0.00"
    shipping_free: bool = False
    available: bool = True
    stock: int | None = None
    condition: str = "new"
    product_url: str | None = None

    def to_dict(self) -> dict[str, Any]:
        return _compact(
            {
                "price": self.price,
                "listPrice": self.list_price,
                "currency": self.currency,
                "shippingCost": self.shipping_cost,
                "shippingFree": self.shipping_free,
                "available": self.available,
                "stock": self.stock,
                "condition": self.condition,
                "productUrl": self.product_url,
            }
        )


@dataclass(slots=True)
class Provider:
    """El proveedor de un servicio: identidad estable (slug) y nombre visible."""

    slug: str = ""
    name: str = ""

    def to_dict(self) -> dict[str, Any]:
        return _compact({"slug": self.slug, "name": self.name})


@dataclass(slots=True)
class Service:
    """Un plan de servicio, normalizado para la Ingestion API (issue #191)."""

    name: str = ""
    category_slug: str | None = None
    description: str | None = None
    image_url: str | None = None
    attributes: dict[str, Any] = field(default_factory=dict)

    def to_dict(self) -> dict[str, Any]:
        payload: dict[str, Any] = {
            "name": self.name,
            "categorySlug": self.category_slug,
            "description": self.description,
            "imageUrl": self.image_url,
        }
        if self.attributes:
            payload["attributes"] = {
                key: _json_value(value) for key, value in sorted(self.attributes.items())
            }
        return _compact(payload)


@dataclass(slots=True)
class ServiceOffer:
    """Precio y condiciones de un plan en un proveedor."""

    price: str = ""
    currency: str = "CLP"
    billing_period: str | None = "monthly"
    installation_cost: str | None = None
    contract_period: str | None = None
    available: bool = True
    coverage_summary: str | None = None
    additional_costs: str | None = None

    def to_dict(self) -> dict[str, Any]:
        return _compact(
            {
                "price": self.price,
                "currency": self.currency,
                "billingPeriod": self.billing_period,
                "installationCost": self.installation_cost,
                "contractPeriod": self.contract_period,
                "available": self.available,
                "coverageSummary": self.coverage_summary,
                "additionalCosts": self.additional_costs,
            }
        )


@dataclass(slots=True)
class Source:
    name: str
    base_url: str
    store_id: int | None = None

    def to_dict(self) -> dict[str, Any]:
        payload: dict[str, Any] = {"name": self.name, "baseUrl": self.base_url}
        if self.store_id is not None:
            payload["storeId"] = self.store_id
        return payload


@dataclass(slots=True)
class Provenance:
    scraped_at: str = ""
    page_url: str | None = None
    run_id: int | None = None
    config_id: int | None = None
    references: dict[str, str] = field(default_factory=dict)
    errors: list[str] = field(default_factory=list)

    def to_dict(self) -> dict[str, Any]:
        payload: dict[str, Any] = {
            "scrapedAt": self.scraped_at,
            "pageUrl": self.page_url,
            "runId": self.run_id,
            "configId": self.config_id,
        }
        if self.references:
            payload["references"] = dict(sorted(self.references.items()))
        if self.errors:
            payload["errors"] = list(self.errors)
        return _compact(payload)


@dataclass(slots=True)
class Item:
    """One card, store-independent and ready for review or ingestion.

    Un ítem de producto usa ``product`` + ``offer``; uno de servicio
    (``entity_type == "service"``) usa ``provider`` + ``service`` +
    ``service_offer``. Los dos caminos comparten identidad, procedencia y
    ``to_ingestion``, que elige el body según el tipo.
    """

    product: Product
    offer: Offer
    external_url: str = ""
    external_id: str | None = None
    entity_type: str = "product"
    provider: Provider | None = None
    service: Service | None = None
    service_offer: ServiceOffer | None = None
    source: Source | None = None
    provenance: Provenance = field(default_factory=Provenance)

    @property
    def key(self) -> str:
        return self.external_id or self.external_url

    def to_dict(self) -> dict[str, Any]:
        if self.entity_type == TARGET_SERVICE:
            payload: dict[str, Any] = {
                "schemaVersion": SCHEMA_VERSION,
                "entityType": self.entity_type,
                "externalId": self.external_id,
                "externalUrl": self.external_url,
                "provider": (self.provider or Provider()).to_dict(),
                "service": (self.service or Service()).to_dict(),
                "offer": (self.service_offer or ServiceOffer()).to_dict(),
                "provenance": self.provenance.to_dict(),
            }
        else:
            payload = {
                "schemaVersion": SCHEMA_VERSION,
                "entityType": self.entity_type,
                "externalId": self.external_id,
                "externalUrl": self.external_url,
                "product": self.product.to_dict(),
                "offer": self.offer.to_dict(),
                "provenance": self.provenance.to_dict(),
            }
        if self.source is not None:
            payload["source"] = self.source.to_dict()
        return _compact(payload)

    def to_ingestion(self) -> dict[str, Any]:
        """Project the record onto the exact body of the ingestion endpoint."""
        if self.entity_type == TARGET_SERVICE:
            payload: dict[str, Any] = {
                "externalUrl": self.external_url,
                "provider": (self.provider or Provider()).to_dict(),
                "service": (self.service or Service()).to_dict(),
                "offer": (self.service_offer or ServiceOffer()).to_dict(),
            }
        else:
            payload = {
                "product": self.product.to_dict(),
                "offer": self.offer.to_dict(),
            }
        if self.external_id:
            payload["externalId"] = self.external_id
        return payload

    def to_json(self) -> str:
        return json.dumps(self.to_dict(), ensure_ascii=False, sort_keys=True)


def _json_value(value: Any) -> Any:
    if isinstance(value, Decimal):
        return float(value)
    if isinstance(value, (list, tuple)):
        return [_json_value(item) for item in value]
    return value


# ---------------------------------------------------------------------------
# Operational records
# ---------------------------------------------------------------------------


@dataclass(slots=True)
class DataSource:
    id: int
    name: str
    base_url: str
    source_type: str
    store_id: int | None = None
    active: bool = True


@dataclass(slots=True)
class ScraperConfig:
    id: int
    data_source_id: int
    scraper_type: str
    target_type: str
    rate_limit: str | None = None
    parser_config: dict[str, Any] = field(default_factory=dict)
    active: bool = True


@dataclass(slots=True)
class Job:
    source: DataSource
    config: ScraperConfig
    spec: StoreSpec


@dataclass(slots=True)
class RunRecord:
    id: int
    scraper_config_id: int
    started_at: str
    finished_at: str | None = None
    records_found: int = 0
    records_processed: int = 0
    records_failed: int = 0
    status: str = RUN_STATUS_PENDING
    error_message: str | None = None
    ingestion_run_id: int | None = None
    items_found: int = 0
    items_sent: int = 0
    items_failed: int = 0
    items_rejected: int = 0
    ingestion_error: str | None = None


@dataclass(slots=True)
class Counters:
    """Números de una corrida: páginas, ítems enviados, rechazados y duplicados.

    ``items_duplicated`` cuenta las tarjetas que se descartaron porque repetían
    un producto ya visto en la misma corrida (TAL-172): así un listado que
    repite tarjetas se distingue de una tienda que dejó de entregar datos.
    """

    pages_found: int = 0
    pages_processed: int = 0
    pages_failed: int = 0
    items_found: int = 0
    items_sent: int = 0
    items_failed: int = 0
    items_rejected: int = 0
    items_duplicated: int = 0

    def to_dict(self) -> dict[str, int]:
        """Devuelve los contadores con los nombres que usa el contrato JSON."""
        return {
            "pagesFound": self.pages_found,
            "pagesProcessed": self.pages_processed,
            "pagesFailed": self.pages_failed,
            "itemsFound": self.items_found,
            "itemsSent": self.items_sent,
            "itemsFailed": self.items_failed,
            "itemsRejected": self.items_rejected,
            "itemsDuplicated": self.items_duplicated,
        }


@dataclass(slots=True)
class RunState:
    """What the control API reports for a run."""

    run_id: int
    status: str
    store: str
    source_url: str
    started_at: str | None = None
    finished_at: str | None = None
    duration_ms: int | None = None
    counters: dict[str, int] = field(default_factory=dict)
    errors: list[str] = field(default_factory=list)
    task_id: str | None = None
    trigger: str | None = None
    ingestion_run_id: int | None = None
    ingestion_error: str | None = None

    def to_dict(self) -> dict[str, Any]:
        payload = {
            "runId": self.run_id,
            "status": self.status,
            "store": self.store,
            "sourceUrl": self.source_url,
            "startedAt": self.started_at,
            "finishedAt": self.finished_at,
            "durationMs": self.duration_ms,
            "counters": self.counters,
            "errors": self.errors,
        }
        if self.task_id:
            payload["taskId"] = self.task_id
        if self.trigger:
            payload["trigger"] = self.trigger
        # El run remoto y su motivo de fallo sólo existen cuando hubo ingesta:
        # se omiten en vez de mandar null para no ensuciar la respuesta.
        if self.ingestion_run_id is not None:
            payload["ingestionRunId"] = self.ingestion_run_id
        if self.ingestion_error:
            payload["ingestionError"] = self.ingestion_error
        return payload


@dataclass(slots=True)
class FetchResult:
    url: str
    status: int | None = None
    body: str = ""
    error: str | None = None
    attempts: int = 1

    @property
    def ok(self) -> bool:
        return self.error is None and self.status is not None and self.status < 400
