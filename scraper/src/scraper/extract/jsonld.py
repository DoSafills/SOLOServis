"""Structured data (JSON-LD) that a page publishes about the items it lists.

Some storefronts render only the first cards of a listing with their images and
links: the rest arrive as skeletons, filled later by client-side JavaScript,
which this scraper never runs. The very same HTML usually carries an
``ItemList`` with the image and the canonical URL of every product, so this
module reads it and indexes it by product id.

It reads, it never guesses: an entry without a product URL is skipped, and a
malformed script produces a warning instead of failing the run.
"""

from __future__ import annotations

import json
import re
from collections.abc import Iterator
from html import unescape
from typing import Any

#: ``<script type="application/ld+json">`` blocks, whatever the attribute order.
SCRIPT_RE = re.compile(
    r"<script[^>]*\btype\s*=\s*[\"']application/ld\+json[\"'][^>]*>(.*?)</script>",
    re.DOTALL | re.IGNORECASE,
)

#: The numeric product id inside a canonical product URL.
PRODUCT_ID_RE = re.compile(r"/product/(\d+)")

#: The first run of digits of a value; used to normalize join keys.
DIGITS_RE = re.compile(r"(\d+)")


def build_index(markup: str) -> tuple[dict[str, dict[str, Any]], list[str]]:
    """Index the page's JSON-LD items by product id.

    Reads every ``application/ld+json`` script in ``markup``, keeps the
    ``ListItem`` entries that point at a ``/product/<id>/`` URL and returns
    ``({id: item}, warnings)``. The item is the object under ``item`` (or the
    entry itself when it carries no wrapper). A script that is not valid JSON
    is reported in ``warnings`` and ignored; duplicates keep the last entry,
    which is what a reader of the page would see.
    """
    index: dict[str, dict[str, Any]] = {}
    warnings: list[str] = []
    for position, payload in enumerate(_script_payloads(markup), start=1):
        try:
            data = json.loads(payload)
        except json.JSONDecodeError as exc:
            warnings.append(f"jsonld script {position}: invalid JSON ({exc.msg})")
            continue
        for entry in _list_items(data):
            key = product_key(entry)
            if key is not None:
                index[key] = entry
    return index, warnings


def product_key(entry: dict[str, Any]) -> str | None:
    """Return the product id declared by a JSON-LD entry, or ``None``.

    The id is the numeric segment of the canonical ``url``
    (``/product/15267403/slug``). An entry without it cannot be joined to a
    card, so it is ignored instead of matched by position.
    """
    url = entry.get("url")
    if not isinstance(url, str):
        return None
    match = PRODUCT_ID_RE.search(url)
    return match.group(1) if match else None


def match_key(value: Any) -> str | None:
    """Normalize a card value into the join key used by :func:`build_index`.

    Takes the first run of digits of ``value``: the card may expose the id as
    ``15267403`` or as ``testId-pod-15267403``, and both must find the same
    JSON-LD entry. Returns ``None`` when there is no number to match.
    """
    if value is None:
        return None
    match = DIGITS_RE.search(str(value))
    return match.group(1) if match else None


def value_at(entry: dict[str, Any], path: str) -> Any:
    """Read a dotted path (``image``, ``offers.0.price``) out of ``entry``.

    Returns ``None`` as soon as a segment is missing or not a container, so a
    new JSON-LD shape degrades into "no value" instead of an exception.
    """
    current: Any = entry
    for segment in path.split("."):
        if isinstance(current, dict):
            current = current.get(segment)
        elif isinstance(current, list) and segment.isdigit():
            position = int(segment)
            current = current[position] if position < len(current) else None
        else:
            return None
        if current is None:
            return None
    return current


def _script_payloads(markup: str) -> Iterator[str]:
    """Yield the text of every JSON-LD script, HTML entities decoded.

    Some storefronts ship the block escaped (``&quot;`` inside the script), so
    the entities are decoded before parsing; in a well-formed block the
    decoding is a no-op.
    """
    for match in SCRIPT_RE.finditer(markup):
        payload = unescape(match.group(1)).strip()
        if payload:
            yield payload


def _list_items(data: Any) -> Iterator[dict[str, Any]]:
    """Yield the product entries of a parsed JSON-LD document.

    Accepts an ``ItemList`` (``itemListElement``), an ``@graph``, a bare array
    or a single object. Each yielded entry is the value under ``item`` when it
    is an object, so callers can read ``url``/``image`` directly.
    """
    for node in _walk(data):
        if not isinstance(node, dict):
            continue
        node_type = node.get("@type")
        is_list_item = node_type == "ListItem" or (
            isinstance(node_type, list) and "ListItem" in node_type
        )
        if not is_list_item:
            continue
        inner = node.get("item")
        yield inner if isinstance(inner, dict) else node


def _walk(node: Any) -> Iterator[Any]:
    """Yield ``node`` and every nested dict/list, depth first.

    Lets the parser find an ``ItemList`` wherever the storefront put it
    (top level, under ``@graph`` or wrapping another object) without assuming
    one exact shape.
    """
    yield node
    if isinstance(node, dict):
        for value in node.values():
            yield from _walk(value)
    elif isinstance(node, list):
        for value in node:
            yield from _walk(value)


__all__ = ["build_index", "match_key", "product_key", "value_at"]
