"""Tests for the JSON-LD reader used by ``item.supplements`` (TAL-242)."""

from __future__ import annotations

import json

from scraper.extract.jsonld import build_index, match_key, product_key, value_at


def _page(*items: dict) -> str:
    """Wrap ``items`` in the ``<script>`` block a storefront publishes."""
    payload = json.dumps(
        {"@context": "https://schema.org", "@type": "ItemList", "itemListElement": list(items)}
    )
    return f'<html><body><script type="application/ld+json">{payload}</script></body></html>'


def _entry(product_id: str, image: str = "https://media.test/1/public") -> dict:
    """Build one ``ListItem`` pointing at ``product_id``."""
    return {
        "@type": "ListItem",
        "item": {
            "@type": "Product",
            "url": f"https://shop.test/product/{product_id}/un-producto",
            "image": image,
        },
    }


def test_build_index_keeps_the_products_with_a_url():
    markup = _page(_entry("15267403"), {"@type": "ListItem", "item": {"image": "x"}})

    index, warnings = build_index(markup)

    assert list(index) == ["15267403"]
    assert index["15267403"]["image"] == "https://media.test/1/public"
    assert warnings == []


def test_build_index_reads_several_scripts_and_nested_graphs():
    first = _page(_entry("1"))
    second = json.dumps({"@graph": [{"@type": "ItemList", "itemListElement": [_entry("2")]}]})
    markup = first.replace(
        "</body>", f'<script type="application/ld+json">{second}</script></body>'
    )

    index, warnings = build_index(markup)

    assert sorted(index) == ["1", "2"]
    assert warnings == []


def test_build_index_ignores_malformed_json_with_a_warning():
    markup = '<script type="application/ld+json">{"@type": "ItemList",}</script>'

    index, warnings = build_index(markup)

    assert index == {}
    assert len(warnings) == 1
    assert "invalid JSON" in warnings[0]


def test_build_index_decodes_escaped_scripts():
    payload = json.dumps({"@type": "ItemList", "itemListElement": [_entry("7")]})
    markup = '<script type="application/ld+json">' + payload.replace('"', "&quot;") + "</script>"

    index, warnings = build_index(markup)

    assert list(index) == ["7"]
    assert warnings == []


def test_match_key_accepts_the_prefixed_and_the_bare_id():
    assert match_key("15267403") == "15267403"
    assert match_key("testId-pod-15267403") == "15267403"
    assert match_key("sin-numero") is None
    assert match_key(None) is None


def test_value_at_walks_dicts_and_lists():
    entry = {"image": "a", "offers": [{"price": "10"}], "brand": {"name": "LG"}}

    assert value_at(entry, "image") == "a"
    assert value_at(entry, "offers.0.price") == "10"
    assert value_at(entry, "brand.name") == "LG"
    assert value_at(entry, "brand.nope") is None
    assert value_at(entry, "offers.9.price") is None
    assert value_at(entry, "image.len") is None


def test_product_key_needs_a_product_url():
    assert product_key({"url": "https://shop.test/product/9/slug"}) == "9"
    assert product_key({"url": "https://shop.test/otra/9"}) is None
    assert product_key({}) is None
