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
from scraper.extract.jsonld import build_index, match_key, value_at
from scraper.extract.transforms import (
    apply_regex,
    apply_transform,
    coerce,
    coerce_condition,
    coerce_currency,
)
from scraper.models import (
    TARGET_SERVICE,
    FieldSpec,
    Item,
    Offer,
    Product,
    Provenance,
    Provider,
    Service,
    ServiceOffer,
    Source,
    StoreSpec,
    Supplement,
    ValueSpec,
)
from scraper.validate import validate_item

PLACEHOLDER_RE = re.compile(r"\{([a-zA-Z0-9_.]+)\}")

ATTRIBUTE_PREFIX = "product.attributes."
SERVICE_ATTRIBUTE_PREFIX = "service.attributes."


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
_SERVICE_ATTRS = {
    "service.name": "name",
    "service.categorySlug": "category_slug",
    "service.description": "description",
    "service.imageUrl": "image_url",
}
_SERVICE_OFFER_ATTRS = {
    "offer.price": "price",
    "offer.currency": "currency",
    "offer.billingPeriod": "billing_period",
    "offer.installationCost": "installation_cost",
    "offer.contractPeriod": "contract_period",
    "offer.available": "available",
    "offer.coverageSummary": "coverage_summary",
    "offer.additionalCosts": "additional_costs",
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
    old_service = (
        replace(item.service, attributes=dict(item.service.attributes))
        if item.service is not None
        else None
    )
    old_service_offer = (
        replace(item.service_offer) if item.service_offer is not None else None
    )
    for path, value in values.items():
        _set_path(item, path, value)
    errors = validate_item(item)
    if errors:
        item.product = old_product
        item.offer = old_offer
        item.service = old_service
        item.service_offer = old_service_offer
        return [f"detail merge rejected: {error}" for error in errors]
    return []


def _set_path(item: Item, path: str, value: Any) -> None:
    """Write ``value`` at a contract ``path`` on ``item``.

    Accepts the attribute prefixes and the declared product, service and offer
    paths; anything else raises :class:`ExtractionError` because it would
    silently change a shape ingestion does not understand.
    """
    if item.entity_type == TARGET_SERVICE:
        if path.startswith(SERVICE_ATTRIBUTE_PREFIX):
            if item.service is not None:
                item.service.attributes[path[len(SERVICE_ATTRIBUTE_PREFIX) :]] = value
            return
        if path in _SERVICE_ATTRS and item.service is not None:
            setattr(item.service, _SERVICE_ATTRS[path], value)
            return
        if path in _SERVICE_OFFER_ATTRS and item.service_offer is not None:
            setattr(item.service_offer, _SERVICE_OFFER_ATTRS[path], value)
            return
        raise ExtractionError(f"detail field {path!r} is not a service contract path")

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

    # El JSON-LD describe el listado completo, no cada tarjeta, así que se
    # indexa una sola vez por página y lo comparten todos los ítems.
    jsonld: dict[str, dict[str, Any]] = {}
    if spec.supplements:
        jsonld, warnings = build_index(html)
        result.warnings.extend(warnings)

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
            jsonld=jsonld,
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
    jsonld: dict[str, dict[str, Any]],
) -> tuple[Item | None, RejectedItem | None]:
    """Turn one listing card into an item, or into a rejected card.

    Reads ``externalId``, ``externalUrl`` and every declared field, applies the
    declared defaults, fills what is still empty from the page JSON-LD through
    ``spec.supplements``, and only then checks ``rejectIfMissing`` and
    ``validate_item``. Returns ``(item, None)`` or ``(None, rejected)``; a card
    is never half-built.
    """
    reasons: list[str] = []
    raw: dict[str, Any] = {}

    external_id = _read_scalar(card, spec.external_id, page_url, raw, "externalId", reasons)
    if spec.external_url is not None:
        external_url = _read_external_url(card, spec, page_url, raw, external_id, reasons)
    else:
        external_url = _read_supplemented_url(card, spec, page_url, raw, jsonld, reasons)

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

    _apply_field_supplements(
        spec, card, page_url, raw, values, jsonld, reasons, reject_paths
    )

    missing = [
        path
        for path in spec.reject_if_missing
        if path not in {"externalId", "externalUrl"} and values.get(path) in (None, "")
    ]
    if not external_url:
        missing.append("externalUrl")
    if not external_id and "externalId" in reject_paths:
        # TAL-172: sin clave el ítem se descartaba en silencio al deduplicar.
        missing.append("externalId")
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
    """Read ``externalUrl`` from the card, a template or an attribute.

    A template is rendered with ``externalId``; anything else is read as a
    selector or an attribute and coerced to an absolute ``http(s)`` URL.
    Returns the URL or ``None``, appending the reason when the template misses
    a placeholder. ``None`` is expected when the spec supplies the URL through
    ``item.supplements`` instead.
    """
    source = spec.external_url
    if source is None:
        return None
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


def _read_supplemented_url(
    card: Selector,
    spec: StoreSpec,
    page_url: str,
    raw: dict[str, Any],
    jsonld: dict[str, dict[str, Any]],
    reasons: list[str],
) -> str | None:
    """Read ``externalUrl`` from the page JSON-LD when the spec declares no source.

    Used by stores whose listing has no usable link in the card: the canonical
    URL is published once per product in the structured data. Returns the URL
    or ``None`` and records the coercion error in ``reasons`` when the JSON-LD
    value is not a valid absolute URL.
    """
    supplement = next(
        (item for item in spec.supplements if item.path == "externalUrl"), None
    )
    if supplement is None:
        return None
    provided = _supplement_value(card, supplement, page_url, jsonld)
    if provided is None:
        return None
    raw["externalUrl"] = provided
    url, error = coerce(provided, "url", page_url=page_url)
    if error:
        reasons.append(f"externalUrl: {error}")
        return None
    return url  # type: ignore[return-value]


def _apply_field_supplements(
    spec: StoreSpec,
    card: Selector,
    page_url: str,
    raw: dict[str, Any],
    values: dict[str, Any],
    jsonld: dict[str, dict[str, Any]],
    reasons: list[str],
    reject_paths: set[str],
) -> None:
    """Fill the declared fields the card left empty from the page JSON-LD.

    Never overwrites a value the card already produced, so the visible markup
    keeps precedence. A JSON-LD value that does not coerce to the field type is
    reported only when the field is required or listed in ``rejectIfMissing``,
    matching how the card-level reader behaves.
    """
    for supplement in spec.supplements:
        if supplement.path == "externalUrl":
            continue
        if values.get(supplement.path) not in (None, ""):
            continue
        field_spec = spec.field(supplement.path)
        if field_spec is None:
            continue
        provided = _supplement_value(card, supplement, page_url, jsonld)
        if provided is None:
            continue
        value, error = _coerce_field(field_spec, provided, page_url)
        if error:
            if field_spec.required or supplement.path in reject_paths:
                reasons.append(f"{supplement.path}: {error}")
            continue
        raw[supplement.path] = provided
        values[supplement.path] = value


def _supplement_value(
    card: Selector,
    supplement: Supplement,
    page_url: str,
    jsonld: dict[str, dict[str, Any]],
) -> str | None:
    """Look up one supplement value for this card.

    Reads the join key with ``supplement.match_by`` (an attribute such as
    ``data-key``), normalizes it to its digits, finds the JSON-LD entry and
    returns the string at ``supplement.value_path``. Returns ``None`` when the
    page has no structured data, the card has no key, the entry is missing or
    the value is not a non-empty string.
    """
    if not jsonld:
        return None
    key_value, error = read_value(card, supplement.match_by, page_url)
    if error is not None or key_value is None:
        return None
    key = match_key(key_value)
    if key is None:
        return None
    entry = jsonld.get(key)
    if entry is None:
        return None
    raw_value = value_at(entry, supplement.value_path)
    if isinstance(raw_value, str) and raw_value.strip():
        return raw_value.strip()
    return None


def _read_scalar(
    card: Selector,
    spec: ValueSpec | None,
    page_url: str,
    raw: dict[str, Any],
    label: str,
    reasons: list[str],
) -> str | None:
    """Read one optional scalar (``externalId``, a plain URL) from the card.

    Applies the declared transform and regex, records the text in ``raw`` for
    diagnostics and returns ``None`` when the spec, the value or the text is
    empty. Read errors are appended to ``reasons``.
    """
    if spec is None:
        return None
    value, error = read_value(card, spec, page_url)
    if error:
        reasons.append(f"{label}: {error}")
        return None
    if value is None:
        return None
    text = str(value)
    text = apply_regex(apply_transform(text, spec.transform, page_url), spec.regex)
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
    """Read and coerce one declared field from the card.

    Handles the three sources of a :class:`FieldSpec`: a template (rendered
    with ``externalId`` and the values already read), a page read (selector,
    attribute, text or list) and a regex/transform pipeline. Returns the
    coerced value, or ``None`` when it is absent; only required or rejected
    paths add a reason.
    """
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
            if not path.startswith((ATTRIBUTE_PREFIX, SERVICE_ATTRIBUTE_PREFIX)):
                reasons.append(f"{path}: selector matched several values for a scalar field")
                return None
            raw[path] = value
            return [str(entry) for entry in value if str(entry).strip()]
        text = apply_regex(apply_transform(str(value), source.transform, page_url), source.regex)
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
    """Coerce ``text`` to the type declared by ``field_spec``.

    Condition and currency have their own parsers; everything else goes through
    :func:`scraper.extract.transforms.coerce`. Returns ``(value, error)``, with
    exactly one of them set.
    """
    if field_spec.path == "offer.condition":
        return coerce_condition(text)
    if field_spec.path == "offer.currency":
        return coerce_currency(text)
    return coerce(text, field_spec.type, currency=field_spec.currency, page_url=page_url)


def _render_template(template: str, context: dict[str, Any]) -> tuple[str, list[str]]:
    """Replace ``{path}`` placeholders in ``template`` with ``context``.

    Returns the rendered text and the list of placeholders that had no value,
    so the caller decides whether a missing one rejects the card or just skips
    the field.
    """
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
    """Assemble the ``Item`` sent to ingestion from the extracted values.

    For a product spec it splits ``product.attributes.<key>`` into the attribute
    map, fills the offer defaults the contract requires (currency, shipping,
    condition, product URL) and records provenance. For a service spec it does
    the same with ``service.attributes.<key>``, the provider and the service
    offer. Returns the item; validation happens in the caller.
    """
    provenance = Provenance(
        scraped_at=scraped_at,
        page_url=page_url,
        run_id=run_id,
        config_id=config_id,
    )
    if spec.target_type == TARGET_SERVICE:
        attributes = {
            path[len(SERVICE_ATTRIBUTE_PREFIX) :]: value
            for path, value in values.items()
            if path.startswith(SERVICE_ATTRIBUTE_PREFIX) and value not in (None, "")
        }
        service = Service(
            name=values.get("service.name") or "",
            category_slug=values.get("service.categorySlug"),
            description=values.get("service.description"),
            image_url=values.get("service.imageUrl"),
            attributes=attributes,
        )
        offer = ServiceOffer(
            price=values.get("offer.price") or "",
            currency=values.get("offer.currency") or _price_currency(spec),
            billing_period=values.get("offer.billingPeriod") or "monthly",
            installation_cost=values.get("offer.installationCost"),
            contract_period=values.get("offer.contractPeriod"),
            available=bool(values.get("offer.available", True)),
            coverage_summary=values.get("offer.coverageSummary"),
            additional_costs=values.get("offer.additionalCosts"),
        )
        return Item(
            product=Product(),
            offer=Offer(),
            external_url=external_url,
            external_id=external_id,
            entity_type=TARGET_SERVICE,
            provider=Provider(slug=spec.provider_slug, name=spec.provider_name),
            service=service,
            service_offer=offer,
            source=source,
            provenance=provenance,
        )

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
    return Item(
        product=product,
        offer=offer,
        external_url=external_url,
        external_id=external_id,
        source=source,
        provenance=provenance,
    )


def _price_currency(spec: StoreSpec) -> str:
    """Return the currency declared for ``offer.price``, or ``CLP``.

    The price is required by the contract, but the currency field is optional;
    this keeps the offer coherent when the spec omits it.
    """
    price = spec.field("offer.price")
    return price.currency if price and price.currency else "CLP"


def _unique(values: list[str]) -> list[str]:
    """Drop duplicates from ``values`` keeping the first occurrence.

    Reasons arrive from several readers and the same one can be produced twice;
    the rejected card should list each cause once, in the order it was found.
    """
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
