"""Strict parser for ``stores/<slug>/spec.json``.

The spec is code: it is versioned, reviewed and deployed with the image. This
module therefore rejects anything it does not understand. A typo in a key or a
field name is a hard error, never a silently ignored selector.
"""

from __future__ import annotations

import hashlib
import json
import re
from pathlib import Path
from typing import Any

from scraper.errors import SpecError
from scraper.models import (
    CrawlRules,
    DetailSpec,
    DetectRule,
    FieldSpec,
    Listing,
    MatchRules,
    Pagination,
    StoreSpec,
    ValueSpec,
)

SPEC_SCHEMA_VERSION = "1.1"
SUPPORTED_SPEC_VERSIONS = frozenset({"1.0", "1.1"})

FIELD_TYPES = frozenset({"string", "money", "integer", "boolean", "url", "slug"})
MODES = frozenset({"text", "attr", "html", "list"})
TRANSFORMS = frozenset(
    {"trim", "collapse_ws", "lower", "upper", "slugify", "absolute_url"}
)
PAGINATION_MODES = frozenset({"none", "next-link", "url-template"})
DEDUPE_KEYS = frozenset({"externalId", "externalUrl"})

_SLUG_RE = re.compile(r"^[a-z0-9][a-z0-9-]{0,63}$")
_ATTR_KEY_RE = re.compile(r"^[a-z][a-z0-9_]{0,63}$")
_CURRENCY_RE = re.compile(r"^[A-Z]{3}$")
_HOST_RE = re.compile(r"^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$")

# Contract paths a field may target. ``product.attributes.<key>`` is handled
# separately because the key is open.
PRODUCT_FIELDS = frozenset(
    {
        "product.name",
        "product.sku",
        "product.model",
        "product.brand",
        "product.categorySlug",
        "product.description",
        "product.imageUrl",
    }
)
OFFER_FIELDS = frozenset(
    {
        "offer.price",
        "offer.listPrice",
        "offer.currency",
        "offer.shippingCost",
        "offer.shippingFree",
        "offer.available",
        "offer.stock",
        "offer.condition",
        "offer.productUrl",
    }
)
REQUIRED_BY_CONTRACT = ("product.name", "offer.price")
REQUIRED_PATH = "product.categorySlug"

_TOP_KEYS = frozenset(
    {"schemaVersion", "store", "match", "listing", "item", "rules", "crawl", "detail"}
)
_STORE_KEYS = frozenset({"slug", "name", "baseUrl", "storeId"})
_MATCH_KEYS = frozenset({"hosts", "pathPrefixes", "detect"})
_DETECT_KEYS = frozenset({"selector", "minMatches"})
_LISTING_KEYS = frozenset({"url", "root", "pagination", "allowEmpty"})
_PAGINATION_KEYS = frozenset({"mode", "selector", "template", "maxPages"})
_ITEM_KEYS = frozenset({"externalId", "externalUrl", "fields"})
_VALUE_KEYS = frozenset(
    {"selector", "mode", "attribute", "transform", "regex", "value", "template"}
)
_FIELD_KEYS = frozenset(
    {
        "selector",
        "mode",
        "attribute",
        "transform",
        "regex",
        "type",
        "value",
        "required",
        "default",
        "currency",
        "template",
    }
)
_RULES_KEYS = frozenset({"dedupeBy", "rejectIfMissing"})
_CRAWL_KEYS = frozenset(
    {
        "linkSelector",
        "linkAttribute",
        "maxDepth",
        "maxPages",
        "allow",
        "deny",
        "sameHost",
        "followItems",
    }
)
_DETAIL_KEYS = frozenset({"root", "detect", "externalId", "fields"})

MAX_CRAWL_DEPTH = 10


def _fail(path: str, where: str, message: str) -> SpecError:
    return SpecError(path, f"{where}: {message}")


def _object(path: str, where: str, raw: Any, allowed: frozenset[str]) -> dict[str, Any]:
    if not isinstance(raw, dict):
        raise _fail(path, where, "must be a JSON object")
    unknown = set(raw) - allowed
    if unknown:
        expected = ", ".join(sorted(allowed))
        raise _fail(path, where, f"unknown key(s) {sorted(unknown)}; expected: {expected}")
    return raw


def _string(
    path: str,
    where: str,
    value: Any,
    *,
    required: bool = True,
    pattern: re.Pattern[str] | None = None,
    choices: frozenset[str] | None = None,
) -> str | None:
    if value is None:
        if required:
            raise _fail(path, where, "is required")
        return None
    if not isinstance(value, str):
        raise _fail(path, where, "must be a string")
    text = value.strip()
    if required and not text:
        raise _fail(path, where, "must not be empty")
    if choices is not None and text and text not in choices:
        raise _fail(path, where, f"must be one of {sorted(choices)}")
    if pattern is not None and text and not pattern.match(text):
        raise _fail(path, where, f"{text!r} does not match {pattern.pattern}")
    return text or None


def _bool(path: str, where: str, value: Any, default: bool = False) -> bool:
    if value is None:
        return default
    if not isinstance(value, bool):
        raise _fail(path, where, "must be a boolean")
    return value


def _int(path: str, where: str, value: Any, *, minimum: int = 1) -> int:
    if isinstance(value, bool) or not isinstance(value, int):
        raise _fail(path, where, "must be an integer")
    if value < minimum:
        raise _fail(path, where, f"must be >= {minimum}")
    return value


def _string_list(path: str, where: str, value: Any) -> tuple[str, ...]:
    if not isinstance(value, list) or not value:
        raise _fail(path, where, "must be a non-empty array of strings")
    result: list[str] = []
    for index, item in enumerate(value):
        text = _string(path, f"{where}[{index}]", item)
        result.append(text)  # type: ignore[arg-type]
    return tuple(result)


def _value_spec(
    path: str, where: str, raw: Any, *, allow_value: bool, allow_template: bool = True
) -> ValueSpec:
    obj = _object(path, where, raw, _VALUE_KEYS)
    selector = _string(path, f"{where}.selector", obj.get("selector"), required=False)
    has_value = "value" in obj
    template = _string(path, f"{where}.template", obj.get("template"), required=False)
    if template and not allow_template:
        raise _fail(path, where, "'template' is not allowed here")
    declared = sum(1 for item in (selector, has_value, template) if item)
    if declared > 1:
        raise _fail(path, where, "declare exactly one of 'selector', 'value' or 'template'")
    if has_value and not allow_value:
        raise _fail(path, where, "'value' is not allowed here")
    attribute = _string(path, f"{where}.attribute", obj.get("attribute"), required=False)
    mode = _string(
        path, f"{where}.mode", obj.get("mode"), required=False, choices=MODES
    ) or ("attr" if attribute else "text")
    transform = _string(
        path, f"{where}.transform", obj.get("transform"), required=False, choices=TRANSFORMS
    )
    if mode == "attr" and not attribute:
        raise _fail(path, where, "mode 'attr' requires 'attribute'")
    if mode != "attr" and attribute:
        raise _fail(path, where, "'attribute' is only valid with mode 'attr'")
    if declared == 0 and not (mode == "attr" and attribute):
        raise _fail(
            path,
            where,
            "needs 'selector', 'value', 'template' or 'attribute' (to read the card itself)",
        )
    if (has_value or template) and (mode != "text" or attribute):
        raise _fail(path, where, "a constant or template cannot use mode/attribute")
    regex = _string(path, f"{where}.regex", obj.get("regex"), required=False)
    if regex:
        if has_value or template or mode == "list":
            raise _fail(path, where, "'regex' needs a single value read from the page")
        try:
            re.compile(regex)
        except re.error as exc:
            raise _fail(path, f"{where}.regex", f"invalid regular expression: {exc}") from exc
    return ValueSpec(
        selector=selector,
        mode=mode,
        attribute=attribute,
        transform=transform,
        regex=regex,
        value=obj.get("value") if has_value else None,
        template=template,
    )


_PLACEHOLDER_RE = re.compile(r"\{([a-zA-Z0-9_.]+)\}")


def _template_placeholders(template: str) -> set[str]:
    return set(_PLACEHOLDER_RE.findall(template))


def _check_template(path: str, where: str, template: str, known: set[str]) -> None:
    for placeholder in _template_placeholders(template):
        if placeholder not in known:
            raise _fail(
                path,
                where,
                f"unknown placeholder {{{placeholder}}}; available: {sorted(known)} "
                "(a template may only reference externalId or non-template fields)",
            )


def _field_path(path: str, where: str, raw: Any) -> str:
    name = _string(path, where, raw)
    assert name is not None
    if name in PRODUCT_FIELDS or name in OFFER_FIELDS:
        return name
    prefix = "product.attributes."
    if name.startswith(prefix):
        key = name[len(prefix) :]
        if not _ATTR_KEY_RE.match(key):
            raise _fail(
                path,
                where,
                f"{key!r} is not a valid attribute key (lowercase, digits, underscore)",
            )
        return name
    known = ", ".join(sorted(PRODUCT_FIELDS | OFFER_FIELDS)) + ", product.attributes.<key>"
    raise _fail(path, where, f"unknown field {name!r}; expected one of: {known}")


def _field_spec(path: str, where: str, raw: Any) -> FieldSpec:
    obj = _object(path, where, raw, _FIELD_KEYS)
    field_type = _string(path, f"{where}.type", obj.get("type"), choices=FIELD_TYPES)
    assert field_type is not None
    value_raw = {key: obj[key] for key in _VALUE_KEYS if key in obj}
    source = _value_spec(path, where, value_raw, allow_value=True)
    required = _bool(path, f"{where}.required", obj.get("required"))
    currency = _string(
        path,
        f"{where}.currency",
        obj.get("currency"),
        required=False,
        pattern=_CURRENCY_RE,
    )
    if currency and field_type != "money":
        raise _fail(path, where, "'currency' is only valid for type 'money'")
    default = obj.get("default")
    if default is not None and source.value is not None:
        raise _fail(path, where, "declare either 'value' or 'default', not both")
    return FieldSpec(
        path=where,
        type=field_type,
        source=source,
        required=required,
        default=default,
        currency=currency or "CLP",
    )


def _prefix_list(path: str, where: str, value: Any) -> tuple[str, ...]:
    """An array of path prefixes; unlike ``pathPrefixes`` it may be empty."""
    if value is None:
        return ()
    if not isinstance(value, list):
        raise _fail(path, where, "must be an array of strings")
    prefixes: list[str] = []
    for index, entry in enumerate(value):
        text = _string(path, f"{where}[{index}]", entry)
        assert text is not None
        if not text.startswith("/"):
            raise _fail(path, f"{where}[{index}]", "must start with '/'")
        prefixes.append(text)
    return tuple(prefixes)


def _detect_rule(path: str, where: str, raw: Any) -> DetectRule | None:
    if raw is None:
        return None
    if isinstance(raw, str):
        selector = _string(path, where, raw)
        assert selector is not None
        return DetectRule(selector=selector, min_matches=1)
    obj = _object(path, where, raw, _DETECT_KEYS)
    selector = _string(path, f"{where}.selector", obj.get("selector"), required=True)
    assert selector is not None
    min_matches = _int(
        path, f"{where}.minMatches", obj.get("minMatches", 1), minimum=1
    )
    return DetectRule(selector=selector, min_matches=min_matches)


def _crawl_rules(path: str, raw: Any) -> CrawlRules:
    obj = _object(path, "spec.crawl", raw, _CRAWL_KEYS)
    link_selector = _string(
        path, "spec.crawl.linkSelector", obj.get("linkSelector"), required=False
    )
    link_attribute = (
        _string(path, "spec.crawl.linkAttribute", obj.get("linkAttribute"), required=False)
        or "href"
    )
    max_depth = _int(path, "spec.crawl.maxDepth", obj.get("maxDepth", 1), minimum=1)
    if max_depth > MAX_CRAWL_DEPTH:
        raise _fail(
            path,
            "spec.crawl.maxDepth",
            f"must be <= {MAX_CRAWL_DEPTH}; deeper crawls are almost always a spec bug",
        )
    max_pages = _int(path, "spec.crawl.maxPages", obj.get("maxPages", 50), minimum=1)
    allow = _prefix_list(path, "spec.crawl.allow", obj.get("allow"))
    deny = _prefix_list(path, "spec.crawl.deny", obj.get("deny"))
    same_host = _bool(path, "spec.crawl.sameHost", obj.get("sameHost"), default=True)
    follow_items = _bool(
        path, "spec.crawl.followItems", obj.get("followItems"), default=True
    )
    if not link_selector and not follow_items:
        raise _fail(
            path,
            "spec.crawl",
            "declare 'linkSelector', 'followItems': true, or both; the crawler never guesses links",
        )
    return CrawlRules(
        link_selector=link_selector,
        link_attribute=link_attribute,
        max_depth=max_depth,
        max_pages=max_pages,
        allow=allow,
        deny=deny,
        same_host=same_host,
        follow_items=follow_items,
    )


def _detail_spec(path: str, raw: Any) -> DetailSpec:
    obj = _object(path, "spec.detail", raw, _DETAIL_KEYS)
    root = _string(path, "spec.detail.root", obj.get("root"), required=False)
    detect = _string(path, "spec.detail.detect", obj.get("detect"), required=False)
    external_id = (
        _value_spec(
            path,
            "spec.detail.externalId",
            obj["externalId"],
            allow_value=False,
            allow_template=False,
        )
        if obj.get("externalId") is not None
        else None
    )
    fields_raw = obj.get("fields")
    if not isinstance(fields_raw, dict) or not fields_raw:
        raise _fail(path, "spec.detail.fields", "must be a non-empty object")
    fields: list[FieldSpec] = []
    seen: set[str] = set()
    for field_path, field_raw in fields_raw.items():
        resolved = _field_path(path, f"spec.detail.fields.{field_path}", field_path)
        if resolved in seen:
            raise _fail(path, f"spec.detail.fields.{resolved}", "declared twice")
        seen.add(resolved)
        fields.append(_field_spec(path, resolved, field_raw))

    # Detail templates may only use values read on the detail page itself.
    known = {"externalId"} | {field.path for field in fields}
    template_paths = {field.path for field in fields if field.source.template}
    for field in fields:
        if field.source.template:
            _check_template(
                path,
                f"spec.detail.fields.{field.path}",
                field.source.template,
                known - template_paths - {field.path},
            )
    return DetailSpec(
        root=root,
        detect=detect,
        external_id=external_id,
        fields=tuple(fields),
    )


def parse_spec(raw: Any, path: str | Path) -> StoreSpec:
    """Validate ``raw`` and return an immutable :class:`StoreSpec`."""
    label = str(path)
    top = _object(label, "spec", raw, _TOP_KEYS)

    schema_version = _string(
        label, "spec.schemaVersion", top.get("schemaVersion"), required=True
    )
    assert schema_version is not None
    if schema_version not in SUPPORTED_SPEC_VERSIONS:
        raise _fail(
            label,
            "spec.schemaVersion",
            f"unsupported version {schema_version!r}; "
            f"expected one of {sorted(SUPPORTED_SPEC_VERSIONS)}",
        )
    if schema_version == "1.0":
        for key in ("crawl", "detail"):
            if top.get(key) is not None:
                raise _fail(
                    label,
                    f"spec.{key}",
                    f"requires schemaVersion {SPEC_SCHEMA_VERSION!r}",
                )
        match_probe = top.get("match")
        if isinstance(match_probe, dict) and match_probe.get("detect") is not None:
            raise _fail(
                label,
                "spec.match.detect",
                f"requires schemaVersion {SPEC_SCHEMA_VERSION!r}",
            )

    store = _object(label, "spec.store", top.get("store"), _STORE_KEYS)
    slug = _string(label, "spec.store.slug", store.get("slug"), pattern=_SLUG_RE)
    assert slug is not None
    name = _string(label, "spec.store.name", store.get("name"))
    base_url = _string(label, "spec.store.baseUrl", store.get("baseUrl"))
    assert base_url is not None
    if not base_url.startswith(("http://", "https://")):
        raise _fail(label, "spec.store.baseUrl", "must start with http:// or https://")
    store_id = _int(label, "spec.store.storeId", store.get("storeId"), minimum=1)

    match = _object(label, "spec.match", top.get("match"), _MATCH_KEYS)
    hosts = _string_list(label, "spec.match.hosts", match.get("hosts"))
    for index, host in enumerate(hosts):
        if not _HOST_RE.match(host.lower()):
            raise _fail(label, f"spec.match.hosts[{index}]", f"{host!r} is not a hostname")
    path_prefixes = match.get("pathPrefixes") or []
    if not isinstance(path_prefixes, list):
        raise _fail(label, "spec.match.pathPrefixes", "must be an array of strings")
    prefixes: list[str] = []
    for index, prefix in enumerate(path_prefixes):
        text = _string(label, f"spec.match.pathPrefixes[{index}]", prefix)
        assert text is not None
        if not text.startswith("/"):
            raise _fail(
                label, f"spec.match.pathPrefixes[{index}]", "must start with '/'"
            )
        prefixes.append(text)
    detect = _detect_rule(label, "spec.match.detect", match.get("detect"))

    listing_raw = _object(label, "spec.listing", top.get("listing"), _LISTING_KEYS)
    listing_url = _string(label, "spec.listing.url", listing_raw.get("url"))
    assert listing_url is not None
    if not listing_url.startswith(("http://", "https://")):
        raise _fail(label, "spec.listing.url", "must start with http:// or https://")
    root = _string(label, "spec.listing.root", listing_raw.get("root"), required=False)
    allow_empty = _bool(label, "spec.listing.allowEmpty", listing_raw.get("allowEmpty"))

    pagination_raw = _object(
        label,
        "spec.listing.pagination",
        listing_raw.get("pagination") or {"mode": "none"},
        _PAGINATION_KEYS,
    )
    pagination_mode = (
        _string(
            label,
            "spec.listing.pagination.mode",
            pagination_raw.get("mode"),
            choices=PAGINATION_MODES,
        )
        or "none"
    )
    max_pages = _int(
        label,
        "spec.listing.pagination.maxPages",
        pagination_raw.get("maxPages", 1),
        minimum=1,
    )
    pagination_selector = _string(
        label, "spec.listing.pagination.selector", pagination_raw.get("selector"), required=False
    )
    pagination_template = _string(
        label, "spec.listing.pagination.template", pagination_raw.get("template"), required=False
    )
    if pagination_mode == "next-link":
        if not pagination_selector:
            raise _fail(label, "spec.listing.pagination", "mode 'next-link' requires 'selector'")
        if pagination_template:
            raise _fail(label, "spec.listing.pagination", "'template' is for 'url-template'")
    elif pagination_mode == "url-template":
        if not pagination_template:
            raise _fail(label, "spec.listing.pagination", "mode 'url-template' requires 'template'")
        if "{page}" not in pagination_template:
            raise _fail(
                label,
                "spec.listing.pagination.template",
                "must contain the {page} placeholder",
            )
        if pagination_selector:
            raise _fail(label, "spec.listing.pagination", "'selector' is for 'next-link'")
    else:
        max_pages = 1

    item_raw = _object(label, "spec.item", top.get("item"), _ITEM_KEYS)
    external_id = (
        _value_spec(
            label,
            "spec.item.externalId",
            item_raw["externalId"],
            allow_value=False,
            allow_template=False,
        )
        if item_raw.get("externalId") is not None
        else None
    )
    external_url = _value_spec(
        label, "spec.item.externalUrl", item_raw.get("externalUrl"), allow_value=False
    )

    fields_raw = item_raw.get("fields")
    if not isinstance(fields_raw, dict) or not fields_raw:
        raise _fail(label, "spec.item.fields", "must be a non-empty object")
    fields: list[FieldSpec] = []
    seen: set[str] = set()
    for field_path, field_raw in fields_raw.items():
        resolved = _field_path(label, f"spec.item.fields.{field_path}", field_path)
        if resolved in seen:
            raise _fail(label, f"spec.item.fields.{resolved}", "declared twice")
        seen.add(resolved)
        fields.append(_field_spec(label, resolved, field_raw))

    for required_path in REQUIRED_BY_CONTRACT:
        spec = next((f for f in fields if f.path == required_path), None)
        if spec is None:
            raise _fail(
                label,
                "spec.item.fields",
                f"{required_path} is required by the ingestion contract",
            )
        if not spec.required:
            raise _fail(
                label,
                f"spec.item.fields.{required_path}",
                "must set 'required': true (the contract cannot send an empty value)",
            )
    if not any(f.path == REQUIRED_PATH for f in fields):
        raise _fail(
            label,
            "spec.item.fields",
            f"{REQUIRED_PATH} is required by the ingestion contract",
        )

    # Templates may only reference values that already exist at extraction
    # time: ``externalId`` and the other fields. A typo would otherwise fail
    # on the first live page instead of at ``scraper validate``.
    known_placeholders = {"externalId"} | {field.path for field in fields}
    template_paths = {field.path for field in fields if field.source.template}
    if external_url.template:
        _check_template(
            label,
            "spec.item.externalUrl",
            external_url.template,
            known_placeholders - template_paths,
        )
    for field in fields:
        if field.source.template:
            _check_template(
                label,
                f"spec.item.fields.{field.path}",
                field.source.template,
                known_placeholders - template_paths - {field.path},
            )

    rules_raw = _object(label, "spec.rules", top.get("rules") or {}, _RULES_KEYS)
    dedupe_by = (
        _string(
            label, "spec.rules.dedupeBy", rules_raw.get("dedupeBy"), choices=DEDUPE_KEYS
        )
        or "externalId"
    )
    reject_if_missing = rules_raw.get("rejectIfMissing")
    if reject_if_missing is None:
        missing = tuple(f.path for f in fields if f.required) + ("externalUrl",)
    else:
        if not isinstance(reject_if_missing, list) or not reject_if_missing:
            raise _fail(
                label, "spec.rules.rejectIfMissing", "must be a non-empty array of paths"
            )
        known = {f.path for f in fields} | {"externalUrl", "externalId"}
        missing_list: list[str] = []
        for index, entry in enumerate(reject_if_missing):
            text = _string(label, f"spec.rules.rejectIfMissing[{index}]", entry)
            assert text is not None
            if text not in known:
                raise _fail(
                    label,
                    f"spec.rules.rejectIfMissing[{index}]",
                    f"unknown path {text!r}",
                )
            missing_list.append(text)
        missing = tuple(missing_list)

    crawl = _crawl_rules(label, top["crawl"]) if top.get("crawl") is not None else None
    detail = _detail_spec(label, top["detail"]) if top.get("detail") is not None else None
    if crawl is not None and crawl.link_selector is None and detail is None:
        raise _fail(
            label,
            "spec.crawl",
            "'followItems' only follows item URLs, so it needs a 'detail' block; "
            "declare 'linkSelector' to discover other pages",
        )

    digest = hashlib.sha256(
        json.dumps(raw, sort_keys=True, separators=(",", ":")).encode("utf-8")
    ).hexdigest()

    return StoreSpec(
        schema_version=schema_version,
        slug=slug,
        name=name or slug,
        base_url=base_url,
        store_id=store_id,
        match=MatchRules(hosts=hosts, path_prefixes=tuple(prefixes), detect=detect),
        listing=Listing(
            url=listing_url,
            root=root,
            pagination=Pagination(
                mode=pagination_mode,
                selector=pagination_selector,
                template=pagination_template,
                max_pages=max_pages,
            ),
            allow_empty=allow_empty,
        ),
        external_id=external_id,
        external_url=external_url,
        fields=tuple(fields),
        dedupe_by=dedupe_by,
        reject_if_missing=missing,
        crawl=crawl,
        detail=detail,
        digest=digest,
        path=Path(path),
    )
