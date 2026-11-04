"""Reading raw values out of a DOM node.

This module knows about CSS, attributes and text; it knows nothing about
types. :mod:`scraper.extract.engine` applies coercion afterwards.
"""

from __future__ import annotations

from typing import Any

from scrapling import Selector

from scraper.errors import ExtractionError
from scraper.models import ValueSpec


def select(node: Selector, selector: str) -> list[Selector]:
    """Run a CSS selector, turning a malformed one into a spec error."""
    try:
        return list(node.css(selector))
    except Exception as exc:  # scrapling raises parser-specific errors
        raise ExtractionError(f"invalid CSS selector {selector!r}: {exc}") from exc


def read_value(node: Selector, spec: ValueSpec, page_url: str) -> tuple[Any, str | None]:
    """Read one value from ``node`` according to ``spec``.

    Returns ``(raw_value, error)``. ``raw_value`` is a string, a list of
    strings, or ``None`` when the selector matched nothing.
    """
    if spec.value is not None:
        return spec.value, None
    if spec.template is not None:
        # Resolved by the engine once the values it references are known.
        return None, None

    if spec.selector:
        elements = select(node, spec.selector)
        if not elements:
            return None, None
        target = elements[0]
    else:
        elements = [node]
        target = node

    if spec.mode == "attr":
        if not spec.attribute:
            return None, "mode 'attr' without an attribute"
        return target.attrib.get(spec.attribute), None
    if spec.mode == "html":
        return target.html_content, None
    if spec.mode == "list":
        return [
            _text(element) for element in (elements if spec.selector else [target])
        ], None
    return _text(target), None


def _text(node: Selector) -> str:
    """All visible text of a node, collapsed to one line."""
    return " ".join(node.get_all_text(separator=" ", strip=True).split())
