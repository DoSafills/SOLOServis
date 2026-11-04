"""Canonical, store-independent record for extracted catalog data.

The extractor reads *tables*; this module reads *items*. A store page is not a
row: a single product card carries a product, an offer, images and attributes,
and every storefront arranges them differently. :class:`BundleSpec` declares
where each part lives in ``SCRAPER_CONFIG.parser_config``, and
:class:`BundleExtractor` collapses the parts into one :class:`NormalizedItem`.

The output is the wire contract of Ingestion API (TAL-77), plus an envelope
that Ingestion never sees:

* ``product`` and ``offer`` match ``internal/ingestion/model.go`` exactly
  (camelCase, no nulls, ``Decimal`` as a string, typed attributes).
* ``schemaVersion``, ``source`` and ``provenance`` travel with the record for
  audit and replay; :meth:`NormalizedItem.to_ingestion` strips them.

Three rules keep the boundary honest:

* The store is not part of the item. It is assigned once per run
  (``POST /ingestion/runs``), so a record can move between stores untouched.
* Foreign keys are never invented. The item carries natural keys
  (``sku``, ``brand``, ``categorySlug``); ids are the server's business.
* Failures are collected, never raised: one unreadable attribute must not cost
  the whole product card.
"""

import json
import re
import unicodedata
from dataclasses import dataclass, field
from datetime import UTC, datetime
from decimal import Decimal, InvalidOperation
from pathlib import Path
from typing import Any
from urllib.parse import urlparse

from extractor import (
    Column,
    EntitySpec,
    Extractor,
    Selector,
    to_boolean,
    to_integer,
    to_number,
    to_text,
)
from utils import normalize_url

SCHEMA_VERSION = "1.0"

# Limits transcribed from the ingestion contract and the production schema.
MAX_EXTERNAL_ID = 255      # scraped_data.external_id VARCHAR(255)
MAX_SOURCE_NAME = 150      # data_source.name VARCHAR(150)
MAX_NAME = 255             # products.MaxNameLength / product.name VARCHAR(255)
MAX_SKU = 100              # products.MaxSKULength / product.sku VARCHAR(100)
MAX_MODEL = 150            # products.MaxModelLength / product.model VARCHAR(150)
MAX_BRAND = 150            # ingestion.MaxBrandName / brand.name VARCHAR(150)
MAX_DESCRIPTION = 5000     # products.MaxDescriptionLength
MAX_URL = 2048             # pricing.maxURLLength
MAX_ATTRIBUTE_TEXT = 255   # category_attributes max_length ceiling
MAX_SLUG = 64              # categories.slug VARCHAR(64)

MONEY_RE = re.compile(r"^\d{1,10}(\.\d{1,2})?$")   # pricing.moneyPattern
SLUG_RE = re.compile(r"^[a-z0-9-]{1,64}$")          # categories.slug CHECK
CURRENCY_RE = re.compile(r"^[A-Z]{3}$")             # products.ValidateCurrencyCode
ATTRIBUTE_KEY_RE = re.compile(r"^[a-z][a-z0-9_]{0,63}$")

CONDITIONS: frozenset[str] = frozenset({"new", "used", "refurbished"})
ATTRIBUTE_TYPES: frozenset[str] = frozenset(
    {"string", "integer", "decimal", "boolean", "enum"})

# Keys a bundle accepts at the top level. Anything else is rejected loudly:
# a silently ignored key would ship a spec that quietly drops data.
_BUNDLE_KEYS: frozenset[str] = frozenset({
    "entity_type", "root", "product", "offer", "images", "external_id",
    "category_slug", "attributes", "source",
})


# ---------------------------------------------------------------------------
# Validation results
# ---------------------------------------------------------------------------


@dataclass(frozen=True, slots=True)
class FieldError:
    """One reason an item cannot be sent to Ingestion, in its vocabulary."""

    field: str
    code: str
    message: str

    def to_dict(self) -> dict[str, str]:
        return {"field": self.field, "code": self.code, "message": self.message}


def _error(field_name: str, code: str, message: str) -> FieldError:
    return FieldError(field=field_name, code=code, message=message)


def _required(errors: list[FieldError], name: str, value: Any) -> None:
    if value is None or (isinstance(value, str) and not value.strip()):
        errors.append(_error(name, "required", f"{name} es obligatorio"))


def _max_length(errors: list[FieldError], name: str, value: Any,
                limit: int) -> None:
    if isinstance(value, str) and len(value) > limit:
        errors.append(_error(
            name, "max_length", f"{name} no puede superar {limit} caracteres"))


def _is_http_url(value: Any) -> bool:
    """Mirror the backend's ``isHTTPURL``: absolute http(s) with a host."""
    if not isinstance(value, str) or any(ch.isspace() for ch in value):
        return False
    try:
        parsed = urlparse(value)
    except ValueError:
        return False
    return parsed.scheme in {"http", "https"} and bool(parsed.netloc)


def _http_url(errors: list[FieldError], name: str, value: Any,
              limit: int | None = None) -> None:
    if not value:
        return
    if limit is not None:
        _max_length(errors, name, value, limit)
    if not _is_http_url(value):
        errors.append(_error(
            name, "format", f"{name} debe ser una URL http(s) absoluta"))


def _money(errors: list[FieldError], name: str, value: Any,
           required: bool = False) -> None:
    if not value:
        if required:
            errors.append(_error(name, "required", f"{name} es obligatorio"))
        return
    if not isinstance(value, str) or not MONEY_RE.match(value):
        errors.append(_error(
            name, "format",
            f"{name} debe ser un monto con hasta 10 enteros y 2 decimales"))


# ---------------------------------------------------------------------------
# Natural keys and per-field specs
# ---------------------------------------------------------------------------


@dataclass(frozen=True, slots=True)
class SourceInfo:
    """Where the record came from. The store is a run-level fact, not a field."""

    name: str
    base_url: str
    store_id: int | None = None

    @classmethod
    def from_config(cls, raw: Any) -> "SourceInfo":
        if not isinstance(raw, dict):
            raise ValueError("parser_config['source'] must be an object")
        return cls(
            name=str(raw.get("name") or "").strip(),
            base_url=str(raw.get("base_url") or "").strip(),
            store_id=raw.get("store_id"),
        )

    def to_dict(self) -> dict[str, Any]:
        payload: dict[str, Any] = {
            "name": self.name, "baseUrl": self.base_url}
        if self.store_id is not None:
            payload["storeId"] = self.store_id
        return payload


def to_slug(raw: str) -> str:
    """Fold free text into the only shape ``categories.slug`` accepts.

    Accents, punctuation, case and whitespace disappear; what is left is
    lower-case alphanumerics joined by single dashes, cut to 64 characters.
    """
    text = unicodedata.normalize("NFKD", raw or "")
    text = "".join(ch for ch in text if not unicodedata.combining(ch))
    text = re.sub(r"[^a-z0-9]+", "-", text.lower())
    text = re.sub(r"-{2,}", "-", text).strip("-")
    return text[:MAX_SLUG].strip("-")


def _alias_key(raw: str) -> str:
    """Compare breadcrumbs without caring about case, accents or spacing."""
    text = unicodedata.normalize("NFKD", raw or "")
    text = "".join(ch for ch in text if not unicodedata.combining(ch))
    return re.sub(r"\s+", " ", text).strip().casefold()


@dataclass(frozen=True, slots=True)
class SlugSpec:
    """How to turn a store's category label into a catalog slug.

    A literal ``value`` wins; otherwise the selector is read, matched against
    the alias map (full text, then the most specific breadcrumb segment) and
    finally folded with :func:`to_slug`. A slug is never invented silently: if
    nothing usable is found, the item reports it.
    """

    value: str | None = None
    selector: str | None = None
    mode: str = "text"
    attribute: str | None = None
    aliases: dict[str, str] = field(default_factory=dict)

    @classmethod
    def from_config(cls, raw: Any) -> "SlugSpec":
        if isinstance(raw, str):
            return cls(value=raw.strip())
        if not isinstance(raw, dict):
            raise ValueError(
                "parser_config['category_slug'] must be a string or an object")
        aliases = raw.get("alias") or raw.get("aliases") or {}
        if not isinstance(aliases, dict):
            raise ValueError("category_slug['alias'] must be an object")
        return cls(
            value=(str(raw["value"]).strip() if raw.get("value") else None),
            selector=(str(raw["selector"]).strip()
                      if raw.get("selector") else None),
            mode=str(raw.get("mode") or "text").strip().lower(),
            attribute=raw.get("attribute"),
            aliases={str(k): str(v) for k, v in aliases.items()},
        )

    def resolve(self, node: Any) -> tuple[str | None, list[str]]:
        errors: list[str] = []
        if self.value:
            return self._validated(self.value, errors)
        if not self.selector:
            return None, ["category_slug: no value or selector declared"]

        raw = next(iter(_select(node, self.selector, self.mode,
                                self.attribute)), "")
        if not raw:
            return None, ["category_slug: value not found"]

        alias = self._lookup_alias(raw)
        if alias:
            return self._validated(alias, errors)

        slug = to_slug(raw)
        if not slug:
            return None, [f"category_slug: could not derive a slug from {raw!r}"]
        return self._validated(slug, errors)

    def _lookup_alias(self, raw: str) -> str | None:
        by_key = {_alias_key(key): value for key, value in self.aliases.items()}
        full = by_key.get(_alias_key(raw))
        if full:
            return full
        for separator in (">", "|", "/"):
            if separator in raw:
                tail = raw.rsplit(separator, 1)[-1]
                found = by_key.get(_alias_key(tail))
                if found:
                    return found
        return None

    @staticmethod
    def _validated(slug: str, errors: list[str]) -> tuple[str | None, list[str]]:
        if SLUG_RE.match(slug):
            return slug, errors
        errors.append(
            f"category_slug: {slug!r} does not match ^[a-z0-9-]{{1,64}}$")
        return None, errors


@dataclass(frozen=True, slots=True)
class ExternalIdSpec:
    """Where the store's own identifier lives, if it publishes one."""

    selector: str
    mode: str = "text"
    attribute: str | None = None

    @classmethod
    def from_config(cls, raw: Any) -> "ExternalIdSpec":
        if isinstance(raw, str):
            return cls(selector=raw.strip())
        if not isinstance(raw, dict):
            raise ValueError(
                "parser_config['external_id'] must be a selector or an object")
        selector = str(raw.get("selector") or "").strip()
        attribute = raw.get("attribute")
        if not selector and not attribute:
            raise ValueError(
                "parser_config['external_id'] needs a selector or an attribute")
        return cls(
            selector=selector,
            mode=str(raw.get("mode") or ("attr" if attribute else "text")).strip().lower(),
            attribute=attribute,
        )

    def read(self, node: Any) -> str | None:
        """Read the id from a descendant, or from the card itself.

        An empty selector plus an attribute reads the card's own attribute:
        Falabella puts the product id on the card element itself
        (``id="testId-pod-14873869"``), where no descendant selector can see it.
        """
        if not self.selector:
            raw = node.attrib.get(self.attribute or "") if hasattr(
                node, "attrib") else ""
        else:
            raw = next(iter(_select(node, self.selector, self.mode,
                                    self.attribute)), "")
        return to_text(raw, Column("external_id", "TEXT"), None) or None


@dataclass(frozen=True, slots=True)
class AttributeSpec:
    """One dynamic attribute, typed the way ``ValidateAttributes`` expects.

    The backend performs no implicit conversions: an ``integer`` attribute must
    arrive as a JSON number, never as ``"350"``. Declaring the type here is
    therefore mandatory, not cosmetic.
    """

    key: str
    selector: str
    data_type: str = "string"
    mode: str = "text"
    attribute: str | None = None
    choices: tuple[str, ...] = ()
    default: Any = None
    max_length: int | None = None

    @classmethod
    def from_config(cls, key: str, raw: Any) -> "AttributeSpec":
        if not ATTRIBUTE_KEY_RE.match(key):
            raise ValueError(
                f"attribute {key!r} must match ^[a-z][a-z0-9_]{{0,63}}$")
        if isinstance(raw, str):
            return cls(key=key, selector=raw.strip())
        if not isinstance(raw, dict) or not (
                raw.get("selector") or raw.get("attribute")):
            raise ValueError(
                f"attribute {key!r} needs a selector or an attribute")

        data_type = str(raw.get("type") or "string").strip().lower()
        if data_type not in ATTRIBUTE_TYPES:
            known = ", ".join(sorted(ATTRIBUTE_TYPES))
            raise ValueError(
                f"attribute {key!r} has unknown type {data_type!r}; "
                f"expected one of: {known}")
        choices = tuple(str(choice) for choice in (raw.get("choices") or ()))
        if data_type == "enum" and not choices:
            raise ValueError(f"attribute {key!r} of type enum needs choices")

        selector = str(raw.get("selector") or "").strip()
        attribute = raw.get("attribute")
        return cls(
            key=key,
            selector=selector,
            data_type=data_type,
            mode=str(raw.get("mode") or ("attr" if attribute and not selector
                                         else "text")).strip().lower(),
            attribute=attribute,
            choices=choices,
            default=raw.get("default"),
            max_length=raw.get("max_length"),
        )

    def read(self, node: Any) -> tuple[Any, list[str]]:
        """Return the JSON-native value plus any problems found."""
        if not self.selector:
            own = node.attrib.get(self.attribute or "") if hasattr(
                node, "attrib") else ""
            raw_values = [own] if own else []
        else:
            raw_values = _select(node, self.selector, self.mode, self.attribute)
        raw = raw_values[0] if raw_values else ""
        if not raw:
            return (self.default, []) if self.default is not None else (
                None, [])

        if self.data_type == "integer":
            return self._integer(raw)
        if self.data_type == "decimal":
            return self._decimal(raw)
        if self.data_type == "boolean":
            value = to_boolean(raw, Column(self.key, "BOOLEAN"), None)
            if value is None:
                return None, [
                    f"{self.key}: could not read {raw!r} as boolean"]
            return value, []
        if self.data_type == "enum":
            value = to_text(raw, Column(self.key, "TEXT"), None)
            if value not in self.choices:
                return None, [
                    f"{self.key}: {value!r} is not one of {list(self.choices)}"]
            return value, []

        value = to_text(raw, Column(self.key, "TEXT"), None)
        limit = self.max_length or MAX_ATTRIBUTE_TEXT
        if len(value) > limit:
            return None, [
                f"{self.key}: {len(value)} chars over the {limit} allowed"]
        return value, []

    def _integer(self, raw: str) -> tuple[int | None, list[str]]:
        try:
            value = to_integer(raw, Column(self.key, "INTEGER"), None)
        except ValueError as exc:
            return None, [f"{self.key}: {exc}"]
        if value is None:
            return None, [f"{self.key}: could not read {raw!r} as integer"]
        return value, []

    def _decimal(self, raw: str) -> tuple[float | None, list[str]]:
        value = to_number(raw, Column(self.key, "NUMERIC"), None)
        if value is None:
            return None, [f"{self.key}: could not read {raw!r} as decimal"]
        # The backend parses attributes as float64; emitting a JSON number is
        # what ValidateAttributes accepts, unlike a quoted decimal string.
        return float(value), []


def _select(node: Any, selector: str, mode: str,
            attribute: str | None) -> list[str]:
    """Read the raw strings a selector points at, never raising."""
    try:
        nodes = node.css(selector)
    except Exception:
        return []
    reader = Selector(column=Column("_", "TEXT"), selector=selector, mode=mode,
                      attribute=attribute)
    return [value for value in reader.read(nodes) if value]


# ---------------------------------------------------------------------------
# Payloads
# ---------------------------------------------------------------------------


def _compact(payload: dict[str, Any]) -> dict[str, Any]:
    return {key: value for key, value in payload.items()
            if value is not None and value != ""}


def _money_text(value: Any) -> str | None:
    """Render an amount as the fixed two-decimal string NUMERIC(12,2) expects."""
    if value is None:
        return None
    if isinstance(value, Decimal):
        number = value
    else:
        try:
            number = Decimal(str(value))
        except InvalidOperation:
            return str(value)
    try:
        return str(number.quantize(Decimal("0.01")))
    except InvalidOperation:
        return str(number)


def _json_attribute(value: Any) -> Any:
    if isinstance(value, Decimal):
        return float(value)
    if isinstance(value, (list, tuple)):
        return [_json_attribute(item) for item in value]
    return value


@dataclass(slots=True)
class ProductPayload:
    """The ``product`` object of the ingestion contract."""

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
                key: _json_attribute(value)
                for key, value in sorted(self.attributes.items())}
        return _compact(payload)


@dataclass(slots=True)
class OfferPayload:
    """The ``offer`` object of the ingestion contract, with explicit defaults."""

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
        return _compact({
            "price": self.price,
            "listPrice": self.list_price,
            "currency": self.currency,
            "shippingCost": self.shipping_cost,
            "shippingFree": self.shipping_free,
            "available": self.available,
            "stock": self.stock,
            "condition": self.condition,
            "productUrl": self.product_url,
        })


@dataclass(slots=True)
class Provenance:
    """Everything Ingestion does not need but an operator eventually will."""

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
class NormalizedItem:
    """One product card, store-independent and ready for review or ingestion."""

    product: ProductPayload
    offer: OfferPayload
    external_url: str = ""
    external_id: str | None = None
    entity_type: str = "product"
    source: SourceInfo | None = None
    provenance: Provenance = field(default_factory=Provenance)

    @property
    def is_valid(self) -> bool:
        return not self.validate()

    @property
    def errors(self) -> list[str]:
        return list(self.provenance.errors)

    def to_dict(self) -> dict[str, Any]:
        payload: dict[str, Any] = {
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
        """Project the record onto the exact body of ``POST /runs/{id}/items``.

        The envelope is dropped here: Ingestion rejects unknown fields, and
        ``source``/``provenance`` are run-level or audit data.
        """
        payload: dict[str, Any] = {
            "product": self.product.to_dict(),
            "offer": self.offer.to_dict(),
        }
        if self.external_id:
            payload["externalId"] = self.external_id
        return payload

    def to_json(self) -> str:
        return json.dumps(self.to_dict(), ensure_ascii=False, sort_keys=True)

    @classmethod
    def from_replay_json(cls, raw: str) -> "NormalizedItem":
        try:
            payload = json.loads(raw)
        except json.JSONDecodeError as exc:
            raise ValueError(f"invalid JSON: {exc.msg}") from exc
        if not isinstance(payload, dict):
            raise ValueError("JSONL record must be an object")
        if payload.get("schemaVersion") != SCHEMA_VERSION:
            raise ValueError("unsupported schemaVersion")
        if payload.get("entityType") != "product":
            raise ValueError("entityType must be 'product'")

        def object_field(value: Any, name: str) -> dict[str, Any]:
            if not isinstance(value, dict):
                raise ValueError(f"{name} must be an object")
            return value

        def string_field(
            value: Any, name: str, *, optional: bool = False,
        ) -> str | None:
            if value is None and optional:
                return None
            if not isinstance(value, str):
                raise ValueError(f"{name} must be a string")
            return value

        def optional_integer(value: Any, name: str) -> int | None:
            if value is None:
                return None
            if isinstance(value, bool) or not isinstance(value, int):
                raise ValueError(f"{name} must be an integer")
            return value

        product = object_field(payload.get("product"), "product")
        offer = object_field(payload.get("offer"), "offer")
        source = object_field(payload.get("source"), "source")
        provenance = object_field(payload.get("provenance"), "provenance")

        attributes = product.get("attributes", {})
        if not isinstance(attributes, dict) or any(
            not isinstance(key, str) for key in attributes
        ):
            raise ValueError("product.attributes must be an object with string keys")

        references = provenance.get("references", {})
        if not isinstance(references, dict) or any(
            not isinstance(key, str) or not isinstance(value, str)
            for key, value in references.items()
        ):
            raise ValueError("provenance.references must contain string pairs")
        errors = provenance.get("errors", [])
        if not isinstance(errors, list) or any(
            not isinstance(error, str) for error in errors
        ):
            raise ValueError("provenance.errors must be a list of strings")

        store_id = optional_integer(source.get("storeId"), "source.storeId")
        if store_id is None or not 0 < store_id <= 2_147_483_647:
            raise ValueError("source.storeId must be a positive 32-bit integer")
        stock = optional_integer(offer.get("stock"), "offer.stock")
        raw_shipping_free = offer.get("shippingFree")
        raw_available = offer.get("available")
        if not isinstance(raw_shipping_free, bool):
            raise ValueError("offer.shippingFree must be a boolean")
        if not isinstance(raw_available, bool):
            raise ValueError("offer.available must be a boolean")

        return cls(
            product=ProductPayload(
                name=string_field(product.get("name"), "product.name") or "",
                sku=string_field(product.get("sku"), "product.sku", optional=True),
                model=string_field(
                    product.get("model"), "product.model", optional=True),
                brand=string_field(
                    product.get("brand"), "product.brand", optional=True),
                category_slug=string_field(
                    product.get("categorySlug"), "product.categorySlug",
                    optional=True),
                description=string_field(
                    product.get("description"), "product.description",
                    optional=True),
                image_url=string_field(
                    product.get("imageUrl"), "product.imageUrl", optional=True),
                attributes=attributes,
            ),
            offer=OfferPayload(
                price=string_field(offer.get("price"), "offer.price") or "",
                list_price=string_field(
                    offer.get("listPrice"), "offer.listPrice", optional=True),
                currency=string_field(
                    offer.get("currency"), "offer.currency") or "",
                shipping_cost=string_field(
                    offer.get("shippingCost"),
                    "offer.shippingCost") or "",
                shipping_free=raw_shipping_free,
                available=raw_available,
                stock=stock,
                condition=string_field(
                    offer.get("condition"), "offer.condition") or "",
                product_url=string_field(
                    offer.get("productUrl"), "offer.productUrl", optional=True),
            ),
            external_url=string_field(
                payload.get("externalUrl"), "externalUrl") or "",
            external_id=string_field(
                payload.get("externalId"), "externalId", optional=True),
            source=SourceInfo(
                name=string_field(source.get("name"), "source.name") or "",
                base_url=string_field(
                    source.get("baseUrl"), "source.baseUrl") or "",
                store_id=store_id,
            ),
            provenance=Provenance(
                scraped_at=string_field(
                    provenance.get("scrapedAt", ""), "provenance.scrapedAt") or "",
                page_url=string_field(
                    provenance.get("pageUrl"), "provenance.pageUrl",
                    optional=True),
                run_id=optional_integer(
                    provenance.get("runId"), "provenance.runId"),
                config_id=optional_integer(
                    provenance.get("configId"), "provenance.configId"),
                references=references,
                errors=errors,
            ),
        )

    def validate(self) -> list[FieldError]:
        """Apply the ingestion rules locally, with the same field codes."""
        errors: list[FieldError] = []

        _required(errors, "externalUrl", self.external_url)
        _http_url(errors, "externalUrl", self.external_url)
        _max_length(errors, "externalId", self.external_id, MAX_EXTERNAL_ID)
        if self.source is not None:
            _required(errors, "source.name", self.source.name)
            _max_length(errors, "source.name", self.source.name,
                        MAX_SOURCE_NAME)
            _required(errors, "source.baseUrl", self.source.base_url)
            _http_url(errors, "source.baseUrl", self.source.base_url)

        product = self.product
        _required(errors, "product.name", product.name)
        _max_length(errors, "product.name", product.name, MAX_NAME)
        _max_length(errors, "product.sku", product.sku, MAX_SKU)
        _max_length(errors, "product.model", product.model, MAX_MODEL)
        _max_length(errors, "product.brand", product.brand, MAX_BRAND)
        _max_length(errors, "product.description", product.description,
                    MAX_DESCRIPTION)
        _http_url(errors, "product.imageUrl", product.image_url)

        _required(errors, "product.categorySlug", product.category_slug)
        if product.category_slug and not SLUG_RE.match(product.category_slug):
            errors.append(_error(
                "product.categorySlug", "format",
                "product.categorySlug debe tener entre 1 y 64 caracteres: "
                "minúsculas, dígitos o guiones"))

        for key, value in product.attributes.items():
            field_name = f"product.attributes.{key}"
            if not ATTRIBUTE_KEY_RE.match(key):
                errors.append(_error(
                    field_name, "format",
                    f"{field_name} no es una clave válida"))
            if isinstance(value, str):
                _max_length(errors, field_name, value, MAX_ATTRIBUTE_TEXT)

        offer = self.offer
        _money(errors, "offer.price", offer.price, required=True)
        if isinstance(offer.price, str) and MONEY_RE.match(offer.price):
            price = Decimal(offer.price)
            if price <= 0:
                errors.append(_error(
                    "offer.price", "min", "offer.price debe ser mayor que 0"))
            elif price != price.to_integral_value():
                errors.append(_error(
                    "offer.price", "integer",
                    "offer.price debe ser un monto entero en CLP"))
        _money(errors, "offer.listPrice", offer.list_price)
        _money(errors, "offer.shippingCost", offer.shipping_cost)

        if not CURRENCY_RE.match(offer.currency or ""):
            errors.append(_error(
                "offer.currency", "format",
                "offer.currency debe tener exactamente tres letras mayúsculas"))
        if offer.condition not in CONDITIONS:
            errors.append(_error(
                "offer.condition", "one_of",
                "offer.condition debe ser new, used o refurbished"))
        if offer.stock is not None and offer.stock < 0:
            errors.append(_error(
                "offer.stock", "min", "offer.stock debe ser mayor o igual a 0"))
        _http_url(errors, "offer.productUrl", offer.product_url, MAX_URL)

        return errors


# ---------------------------------------------------------------------------
# Bundle specs
# ---------------------------------------------------------------------------


@dataclass(frozen=True, slots=True)
class BundleSpec:
    """A product card: one root, several entities, one item per match."""

    product: EntitySpec
    offer: EntitySpec
    images: EntitySpec | None = None
    external_id: ExternalIdSpec | None = None
    category_slug: SlugSpec | None = None
    attributes: tuple[AttributeSpec, ...] = ()
    root: str | None = None
    source: SourceInfo | None = None

    @property
    def entity_type(self) -> str:
        return self.product.entity_type

    @classmethod
    def from_parser_config(cls, parser_config: dict | None) -> "BundleSpec":
        """Build a bundle from ``SCRAPER_CONFIG.parser_config``.

        Unlike a single-table :class:`EntitySpec`, a bundle is explicit about
        its parts: ``product`` and ``offer`` are mandatory, and unknown keys are
        rejected so a typo cannot silently drop half the card.
        """
        config = parser_config or {}
        if not isinstance(config, dict):
            raise ValueError("parser_config must be an object")

        unknown = set(config) - _BUNDLE_KEYS
        if unknown:
            known = ", ".join(sorted(_BUNDLE_KEYS))
            raise ValueError(
                f"unknown parser_config keys {sorted(unknown)}; expected: {known}")

        entity_type = str(config.get("entity_type") or "product").strip()
        if entity_type != "product":
            raise ValueError(
                f"bundle entity_type {entity_type!r} is not supported yet; "
                "only 'product' has an ingestion contract")

        product = _entity_spec(config.get("product"), "product")
        offer = _entity_spec(config.get("offer"), "product_offer")
        images = _optional_entity_spec(config.get("images"), "product_image")

        attributes = config.get("attributes") or {}
        if not isinstance(attributes, dict):
            raise ValueError("parser_config['attributes'] must be an object")

        source = config.get("source")
        return cls(
            product=product,
            offer=offer,
            images=images,
            external_id=(ExternalIdSpec.from_config(config["external_id"])
                         if config.get("external_id") else None),
            category_slug=(SlugSpec.from_config(config["category_slug"])
                           if config.get("category_slug") is not None else None),
            attributes=tuple(AttributeSpec.from_config(key, raw)
                             for key, raw in attributes.items()),
            root=str(config["root"]).strip() if config.get("root") else None,
            source=SourceInfo.from_config(source) if source else None,
        )


def _entity_spec(raw: Any, entity_type: str) -> EntitySpec:
    if not isinstance(raw, dict) or not raw.get("fields"):
        raise ValueError(
            f"parser_config[{entity_type!r}] must be an object with 'fields'")
    return EntitySpec.from_parser_config({
        "entity_type": entity_type,
        "fields": raw.get("fields") or {},
        "references": raw.get("references") or {},
    })


def _optional_entity_spec(raw: Any, entity_type: str) -> EntitySpec | None:
    if raw is None:
        return None
    return _entity_spec(raw, entity_type)


# ---------------------------------------------------------------------------
# Extraction
# ---------------------------------------------------------------------------


class BundleExtractor:
    """Applies a :class:`BundleSpec` to Scrapling documents."""

    def __init__(
        self,
        spec: BundleSpec,
        source: SourceInfo | None = None,
        run_id: int | None = None,
        config_id: int | None = None,
        clock: Any = None,
    ):
        self.spec = spec
        self.source = source or spec.source
        self.run_id = run_id
        self.config_id = config_id
        self._clock = clock or (lambda: datetime.now(UTC))

    def extract_all(self, document: Any,
                    base_url: str | None = None) -> list[NormalizedItem]:
        """Extract one item per ``root`` match; one item without a root."""
        roots = self._roots(document)
        if roots is None:
            return [self.extract(document, base_url)]
        return [self.extract(node, base_url) for node in roots]

    def extract(self, node: Any,
                base_url: str | None = None) -> NormalizedItem:
        """Build one item from a card node (or a detail-page document)."""
        product_row = Extractor(self.spec.product).extract(node, base_url)
        offer_row = Extractor(self.spec.offer).extract(node, base_url)
        image_row = (Extractor(self.spec.images).extract(node, base_url)
                     if self.spec.images else None)

        errors = [
            *(f"product.{error}" for error in product_row.errors),
            *(f"offer.{error}" for error in offer_row.errors),
            *(f"image.{error}" for error in (image_row.errors if image_row else [])),
        ]

        product = self._product(node, product_row, image_row, errors)
        offer = self._offer(offer_row)
        external_url = self._external_url(node, base_url, offer, errors)

        return NormalizedItem(
            product=product,
            offer=offer,
            external_url=external_url or "",
            external_id=(self.spec.external_id.read(node)
                         if self.spec.external_id else None),
            entity_type=self.spec.entity_type,
            source=self.source,
            provenance=Provenance(
                scraped_at=self._timestamp(),
                page_url=base_url,
                run_id=self.run_id,
                config_id=self.config_id,
                references=dict(product_row.references),
                errors=errors,
            ),
        )

    def _roots(self, document: Any) -> list[Any] | None:
        if not self.spec.root:
            return None
        try:
            return list(document.css(self.spec.root))
        except Exception:
            return None

    def _product(
        self,
        node: Any,
        row: Any,
        image_row: Any,
        errors: list[str],
    ) -> ProductPayload:
        values = row.values
        attributes: dict[str, Any] = {}
        for attribute in self.spec.attributes:
            value, problems = attribute.read(node)
            errors.extend(f"attribute.{problem}" for problem in problems)
            if value is not None:
                attributes[attribute.key] = value

        category_slug = None
        if self.spec.category_slug is not None:
            category_slug, problems = self.spec.category_slug.resolve(node)
            errors.extend(problems)

        image_url = _first(image_row.values.get("image_url")) \
            if image_row else None

        return ProductPayload(
            name=values.get("name") or "",
            sku=values.get("sku"),
            model=values.get("model"),
            brand=row.references.get("brand"),
            category_slug=category_slug,
            description=values.get("description"),
            image_url=image_url,
            attributes=attributes,
        )

    def _offer(self, row: Any) -> OfferPayload:
        values = row.values
        return OfferPayload(
            price=_money_text(values.get("price")) or "",
            list_price=_money_text(values.get("list_price")),
            currency=values.get("currency") or "CLP",
            shipping_cost=_money_text(values.get("shipping_cost") or 0) or "0.00",
            shipping_free=bool(values.get("shipping_free", False)),
            available=bool(values.get("available", True)),
            stock=values.get("stock"),
            condition=values.get("condition") or "new",
            product_url=values.get("product_url"),
        )

    def _external_url(
        self,
        node: Any,
        base_url: str | None,
        offer: OfferPayload,
        errors: list[str],
    ) -> str | None:
        if offer.product_url:
            return offer.product_url
        href = node.attrib.get("href") if hasattr(node, "attrib") else None
        if href:
            resolved = normalize_url(href, base_url)
            if resolved:
                return resolved
        if self.spec.root is None:
            return base_url
        # A card without a link (Falabella renders ``href=""`` and fills it with
        # JS) still has a page to point at; the record stays auditable, and the
        # weakness is reported instead of hidden.
        errors.append(
            "externalUrl: the card has no href and no product_url selector; "
            "falling back to the listing page")
        return base_url

    def _timestamp(self) -> str:
        moment = self._clock()
        if moment.tzinfo is None:
            moment = moment.replace(tzinfo=UTC)
        return moment.astimezone(UTC).isoformat(timespec="seconds").replace(
            "+00:00", "Z")


def _first(value: Any) -> str | None:
    if isinstance(value, (list, tuple)):
        return value[0] if value else None
    return value


def export_jsonl(items: list[NormalizedItem], path: str | Path) -> int:
    """Append one canonical record per line, UTF-8, deterministic ordering."""
    target = Path(path)
    target.parent.mkdir(parents=True, exist_ok=True)
    written = 0
    with target.open("a", encoding="utf-8") as handle:
        for item in items:
            handle.write(item.to_json() + "\n")
            written += 1
    return written
