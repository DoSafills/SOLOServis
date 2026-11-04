"""The extraction engine: a store spec plus a page becomes items.

The engine is deliberately literal. It selects the nodes the spec names, reads
the values the spec declares, coerces them to the contract types and then asks
:func:`scraper.validate.validate_item` whether the result is sendable. Anything
the spec does not describe is ignored; anything it describes but cannot read is
reported, never guessed.
"""

from __future__ import annotations

import re
from dataclasses import dataclass, field, replace
from typing import Any

from scrapling import Selector

from scraper.errors import ExtractionError
from scraper.extract.fields import read_value, select
from scraper.extract.transforms import (
    apply_transform,
    coerce,
    coerce_condition,
    coerce_currency,
)
from scraper.models import (
    FieldSpec,
    Item,
    Offer,
    Product,
    Provenance,
    Source,
    StoreSpec,
    ValueSpec,
)
from scraper.validate import validate_item

PLACEHOLDER_RE = re.compile(r"\{([a-zA-Z0-9_.]+)\}")

ATTRIBUTE_PREFIX = "product.attributes."


@dataclass(slots=True)
class RejectedItem:
    """One card the spec could not turn into a valid item."""

    index: int
    external_id: str | None
    external_url: str | None
    reasons: list[str]
    raw: dict[str, Any] = field(default_factory=dict)

    def to_dict(self) -> dict[str, Any]:
        return {
            "index": self.index,
            "externalId": self.external_id,
            "externalUrl": self.external_url,
            "reasons": self.reasons,
            "raw": self.raw,
        }


@dataclass(slots=True)
class ExtractionResult:
    items: list[Item] = field(default_factory=list)
    rejected: list[RejectedItem] = field(default_factory=list)
    warnings: list[str] = field(default_factory=list)

    @property
    def cards(self) -> int:
        return len(self.items) + len(self.rejected)


@dataclass(slots=True)
class DetailResult:
    """Partial fields read from a product detail page.

    A detail page is **not** a second source of items: it may only enrich an
    item the listing already produced. ``values`` is empty when the page does
    not contain the declared selectors.
    """

    external_id: str | None = None
    values: dict[str, Any] = field(default_factory=dict)
    warnings: list[str] = field(default_factory=list)

    @property
    def empty(self) -> bool:
        return not self.values


_PRODUCT_ATTRS = {
    "product.name": "name",
    "product.sku": "sku",
    "product.model": "model",
    "product.brand": "brand",
    "product.categorySlug": "category_slug",
    "product.description": "description",
    "product.imageUrl": "image_url",
}
_OFFER_ATTRS = {
    "offer.price": "price",
    "offer.listPrice": "list_price",
    "offer.currency": "currency",
    "offer.shippingCost": "shipping_cost",
    "offer.shippingFree": "shipping_free",
    "offer.available": "available",
    "offer.stock": "stock",
    "offer.condition": "condition",
    "offer.productUrl": "product_url",
}


def extract_detail(
    spec: StoreSpec,
    html: str,
    *,
    page_url: str,
) -> DetailResult:
    """Read the fields declared under ``detail`` from one detail page."""
    detail = spec.detail
    if detail is None:
        return DetailResult()

    document = Selector(html)
    root = document
    if detail.root:
        nodes = select(document, detail.root)
        if not nodes:
            return DetailResult(
                warnings=[f"detail root {detail.root!r} matched nothing on {page_url}"]
            )
        root = nodes[0]

    reasons: list[str] = []
    raw: dict[str, Any] = {}
    external_id = (
        _read_scalar(root, detail.external_id, page_url, raw, "externalId", reasons)
        if detail.external_id is not None
        else None
    )

    values: dict[str, Any] = {}
    for field_spec in detail.fields:
        if field_spec.source.template is not None:
            continue
        value = _read_field(
            root,
            field_spec,
            page_url,
            raw,
            external_id,
            reasons,
            reject_paths=set(),
        )
        if value is not None:
            values[field_spec.path] = value
        elif field_spec.required:
            reasons.append(f"{field_spec.path}: required detail value not found")
    for field_spec in detail.fields:
        if field_spec.source.template is None:
            continue
        value = _read_field(
            root,
            field_spec,
            page_url,
            raw,
            external_id,
            reasons,
            values,
            reject_paths=set(),
        )
        if value is not None:
            values[field_spec.path] = value
        elif field_spec.required:
            reasons.append(f"{field_spec.path}: required detail value not found")

    return DetailResult(
        external_id=external_id,
        values=values,
        warnings=[f"detail page {page_url}: {reason}" for reason in _unique(reasons)],
    )


def apply_detail(item: Item, values: dict[str, Any]) -> list[str]:
    """Merge detail values onto ``item``; revert if the result is invalid.

    Detail pages are more specific than listing cards, so a readable value
    wins. A missing selector simply leaves the listing value in place. The
    merged item is validated before it is kept.
    """
    if not values:
        return []
    old_product = replace(item.product, attributes=dict(item.product.attributes))
    old_offer = replace(item.offer)
    for path, value in values.items():
        _set_path(item, path, value)
    errors = validate_item(item)
    if errors:
        item.product = old_product
        item.offer = old_offer
        return [f"detail merge rejected: {error}" for error in errors]
    return []


def _set_path(item: Item, path: str, value: Any) -> None:
    if path.startswith(ATTRIBUTE_PREFIX):
        key = path[len(ATTRIBUTE_PREFIX) :]
        item.product.attributes[key] = value
        return
    if path in _PRODUCT_ATTRS:
        setattr(item.product, _PRODUCT_ATTRS[path], value)
        return
    if path in _OFFER_ATTRS:
        setattr(item.offer, _OFFER_ATTRS[path], value)
        return
    raise ExtractionError(f"detail field {path!r} is not a contract path")


def extract_page(
    spec: StoreSpec,
    html: str,
    *,
    page_url: str,
    source: Source,
    run_id: int | None = None,
    config_id: int | None = None,
    scraped_at: str = "",
) -> ExtractionResult:
    """Extract every card on ``html`` using ``spec``.

    ``page_url`` is the URL the HTML was fetched from; it is used to absolutize
    relative links and recorded in provenance.
    """
    document = Selector(html)
    root = spec.listing.root
    cards = select(document, root) if root else [document]
    result = ExtractionResult()
    if not cards:
        return result

    for index, card in enumerate(cards):
        item, rejected = _extract_card(
            spec,
            card,
            index=index,
            page_url=page_url,
            source=source,
            run_id=run_id,
            config_id=config_id,
            scraped_at=scraped_at,
        )
        if item is not None:
            result.items.append(item)
        elif rejected is not None:
            result.rejected.append(rejected)
    return result


def _extract_card(
    spec: StoreSpec,
    card: Selector,
    *,
    index: int,
    page_url: str,
    source: Source,
    run_id: int | None,
    config_id: int | None,
    scraped_at: str,
) -> tuple[Item | None, RejectedItem | None]:
    reasons: list[str] = []
    raw: dict[str, Any] = {}

    external_id = _read_scalar(card, spec.external_id, page_url, raw, "externalId", reasons)
    external_url = _read_external_url(card, spec, page_url, raw, external_id, reasons)

    values: dict[str, Any] = {}
    reject_paths = set(spec.reject_if_missing)
    for field_spec in spec.fields:
        if field_spec.source.template is not None:
            continue
        value = _read_field(
            card, field_spec, page_url, raw, external_id, reasons, reject_paths=reject_paths
        )
        if value is not None:
            values[field_spec.path] = value

    for field_spec in spec.fields:
        if field_spec.source.template is None:
            continue
        value = _read_field(
            card,
            field_spec,
            page_url,
            raw,
            external_id,
            reasons,
            values,
            reject_paths=reject_paths,
        )
        if value is not None:
            values[field_spec.path] = value

    for field_spec in spec.fields:
        if field_spec.path not in values and field_spec.default is not None:
            values[field_spec.path] = field_spec.default

    missing = [
        path
        for path in spec.reject_if_missing
        if path not in {"externalId", "externalUrl"} and values.get(path) in (None, "")
    ]
    if not external_url:
        missing.append("externalUrl")
    for path in missing:
        reasons.append(f"{path}: required value not found")

    if reasons:
        return None, RejectedItem(
            index=index,
            external_id=external_id,
            external_url=external_url,
            reasons=_unique(reasons),
            raw=raw,
        )

    item = _build_item(
        spec,
        values,
        external_id=external_id,
        external_url=external_url,
        source=source,
        page_url=page_url,
        run_id=run_id,
        config_id=config_id,
        scraped_at=scraped_at,
    )

    errors = validate_item(item)
    if errors:
        return None, RejectedItem(
            index=index,
            external_id=external_id,
            external_url=external_url,
            reasons=[f"{error.field}: {error.message}" for error in errors],
            raw=raw,
        )
    return item, None


def _read_external_url(
    card: Selector,
    spec: StoreSpec,
    page_url: str,
    raw: dict[str, Any],
    external_id: str | None,
    reasons: list[str],
) -> str | None:
    source = spec.external_url
    if source.template is not None:
        rendered, missing = _render_template(source.template, {"externalId": external_id})
        if missing:
            reasons.append(f"externalUrl: missing template value(s) {missing}")
            return None
        raw["externalUrl"] = rendered
        url, error = coerce(rendered, "url", page_url=page_url)
        if error:
            reasons.append(f"externalUrl: {error}")
            return None
        return url  # type: ignore[return-value]

    return _read_scalar(card, source, page_url, raw, "externalUrl", reasons)


def _read_scalar(
    card: Selector,
    spec: ValueSpec | None,
    page_url: str,
    raw: dict[str, Any],
    label: str,
    reasons: list[str],
) -> str | None:
    if spec is None:
        return None
    value, error = read_value(card, spec, page_url)
    if error:
        reasons.append(f"{label}: {error}")
        return None
    if value is None:
        return None
    text = str(value)
    text = apply_transform(text, spec.transform, page_url)
    raw[label] = text
    return text or None


def _read_field(
    card: Selector,
    field_spec: FieldSpec,
    page_url: str,
    raw: dict[str, Any],
    external_id: str | None,
    reasons: list[str],
    values: dict[str, Any] | None = None,
    *,
    reject_paths: set[str] | None = None,
) -> Any:
    source = field_spec.source
    path = field_spec.path

    if source.template is not None:
        context = dict(values or {})
        context["externalId"] = external_id
        rendered, missing = _render_template(source.template, context)
        if missing:
            reasons.append(f"{path}: missing template value(s) {missing}")
            return None
        raw[path] = rendered
        value, error = _coerce_field(field_spec, rendered, page_url)
    else:
        value, read_error = read_value(card, source, page_url)
        if read_error:
            reasons.append(f"{path}: {read_error}")
            return None
        if value is None:
            return None
        if isinstance(value, list):
            if not path.startswith(ATTRIBUTE_PREFIX):
                reasons.append(f"{path}: selector matched several values for a scalar field")
                return None
            raw[path] = value
            return [str(entry) for entry in value if str(entry).strip()]
        text = apply_transform(str(value), source.transform, page_url)
        raw[path] = text
        if not text.strip():
            return None
        value, error = _coerce_field(field_spec, text, page_url)

    if error:
        if field_spec.required or path in (reject_paths or set()):
            reasons.append(f"{path}: {error}")
        return None
    return value


def _coerce_field(
    field_spec: FieldSpec, text: str, page_url: str
) -> tuple[Any, str | None]:
    if field_spec.path == "offer.condition":
        return coerce_condition(text)
    if field_spec.path == "offer.currency":
        return coerce_currency(text)
    return coerce(text, field_spec.type, currency=field_spec.currency, page_url=page_url)


def _render_template(template: str, context: dict[str, Any]) -> tuple[str, list[str]]:
    missing: list[str] = []

    def replace(match: re.Match[str]) -> str:
        key = match.group(1)
        value = context.get(key)
        if value is None or value == "":
            missing.append(key)
            return ""
        return str(value)

    return PLACEHOLDER_RE.sub(replace, template), missing


def _build_item(
    spec: StoreSpec,
    values: dict[str, Any],
    *,
    external_id: str | None,
    external_url: str,
    source: Source,
    page_url: str,
    run_id: int | None,
    config_id: int | None,
    scraped_at: str,
) -> Item:
    attributes = {
        path[len(ATTRIBUTE_PREFIX) :]: value
        for path, value in values.items()
        if path.startswith(ATTRIBUTE_PREFIX) and value not in (None, "")
    }
    product = Product(
        name=values.get("product.name") or "",
        sku=values.get("product.sku"),
        model=values.get("product.model"),
        brand=values.get("product.brand"),
        category_slug=values.get("product.categorySlug"),
        description=values.get("product.description"),
        image_url=values.get("product.imageUrl"),
        attributes=attributes,
    )
    currency = values.get("offer.currency") or _price_currency(spec)
    offer = Offer(
        price=values.get("offer.price") or "",
        list_price=values.get("offer.listPrice"),
        currency=currency,
        shipping_cost=values.get("offer.shippingCost") or "0.00",
        shipping_free=bool(values.get("offer.shippingFree", False)),
        available=bool(values.get("offer.available", True)),
        stock=values.get("offer.stock"),
        condition=values.get("offer.condition") or "new",
        product_url=values.get("offer.productUrl") or external_url,
    )
    provenance = Provenance(
        scraped_at=scraped_at,
        page_url=page_url,
        run_id=run_id,
        config_id=config_id,
    )
    return Item(
        product=product,
        offer=offer,
        external_url=external_url,
        external_id=external_id,
        source=source,
        provenance=provenance,
    )


def _price_currency(spec: StoreSpec) -> str:
    price = spec.field("offer.price")
    return price.currency if price and price.currency else "CLP"


def _unique(values: list[str]) -> list[str]:
    seen: set[str] = set()
    result: list[str] = []
    for value in values:
        if value not in seen:
            seen.add(value)
            result.append(value)
    return result


__all__ = [
    "DetailResult",
    "ExtractionError",
    "ExtractionResult",
    "RejectedItem",
    "apply_detail",
    "extract_detail",
    "extract_page",
]
