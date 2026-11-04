"""Declared discovery, detail pages and page detection."""

from __future__ import annotations

import asyncio
import copy
import json
from typing import Any

import pytest

from scraper.cli import _run_was_complete
from scraper.db import repository as repo
from scraper.errors import SpecError
from scraper.extract.engine import extract_detail
from scraper.fetch.client import Fetcher
from scraper.models import (
    RUN_STATUS_FAILED,
    RUN_STATUS_SUCCESS,
    CrawlRules,
    FetchResult,
)
from scraper.pipeline import execute
from scraper.spec.schema import parse_spec

LISTING_URL = "https://shop.test/catalog"
PAGE_2_URL = "https://shop.test/catalog/page2"
PRODUCT_1 = "https://shop.test/product/one"
PRODUCT_2 = "https://shop.test/product/two"
PRODUCT_3 = "https://shop.test/product/three"

SPEC: dict[str, Any] = {
    "schemaVersion": "1.1",
    "store": {
        "slug": "crawl-test",
        "name": "Crawl Test",
        "baseUrl": "https://shop.test",
        "storeId": 9,
    },
    "match": {
        "hosts": ["shop.test"],
        "pathPrefixes": ["/"],
        "detect": {"selector": "body.shop", "minMatches": 1},
    },
    "listing": {
        "url": LISTING_URL,
        "root": "a.card",
        "pagination": {"mode": "none"},
    },
    "item": {
        "externalId": {"attribute": "data-id"},
        "externalUrl": {"attribute": "href", "transform": "absolute_url"},
        "fields": {
            "product.name": {"selector": ".name", "type": "string", "required": True},
            "product.categorySlug": {"value": "test", "type": "slug"},
            "offer.price": {
                "selector": ".price",
                "type": "money",
                "required": True,
                "currency": "CLP",
            },
        },
    },
    "crawl": {
        "linkSelector": "a",
        "maxDepth": 1,
        "maxPages": 10,
        "allow": ["/catalog/", "/product/"],
        "deny": ["/cart"],
        "sameHost": True,
        "followItems": True,
    },
    "detail": {
        "root": "main.detail",
        "detect": "main.detail",
        "fields": {
            "product.description": {"selector": ".desc", "type": "string"},
            "product.sku": {"selector": ".sku", "type": "string"},
        },
    },
    "rules": {
        "dedupeBy": "externalId",
        "rejectIfMissing": ["externalUrl", "product.name", "offer.price"],
    },
}

LISTING = """
<html><body class="shop">
  <a class="card" data-id="P1" href="/product/one">
    <span class="name">One</span><span class="price">$1.000</span>
  </a>
  <a class="card" data-id="P2" href="/product/two">
    <span class="name">Two</span><span class="price">$2.000</span>
  </a>
  <a class="more" href="/catalog/page2">More</a>
  <a class="cart" href="/cart">Cart</a>
</body></html>
"""

PAGE_2 = """
<html><body class="shop">
  <a class="card" data-id="P3" href="/product/three">
    <span class="name">Three</span><span class="price">$3.000</span>
  </a>
</body></html>
"""

DETAIL_1 = """
<html><body class="shop"><main class="detail">
  <h1>One</h1><p class="desc">The first product in detail.</p>
  <span class="sku">SKU-1</span>
</main></body></html>
"""

DETAIL_2 = """
<html><body class="shop"><main class="detail">
  <h1>Two</h1><p class="desc">The second product in detail.</p>
  <span class="sku">SKU-2</span>
</main></body></html>
"""

DETAIL_3 = """
<html><body class="shop"><main class="detail">
  <h1>Three</h1>
</main></body></html>
"""


def _spec():
    return parse_spec(copy.deepcopy(SPEC), "inline")


def _prepare(settings, spec):
    repo.init_db(settings.db_path)
    repo.ensure_source(
        settings.db_path,
        name=spec.name,
        base_url=spec.base_url,
        store_id=spec.store_id,
        rate_limit=settings.rate_limit,
        spec_slug=spec.slug,
        spec_version=spec.schema_version,
        spec_hash=spec.digest,
    )
    source, config = repo.config_by_base_url(settings.db_path, spec.base_url)
    return source, config


def _transport(pages: dict[str, str]):
    requested: list[str] = []

    async def transport(url: str, _timeout: float) -> FetchResult:
        requested.append(url)
        if url in pages:
            return FetchResult(url=url, status=200, body=pages[url])
        return FetchResult(url=url, status=404, body="")

    return transport, requested


def _run(settings, spec, pages):
    source, config = _prepare(settings, spec)
    run_id = repo.create_run(settings.db_path, config.id)
    transport, requested = _transport(pages)
    fetcher = Fetcher(settings, transport=transport)
    result = asyncio.run(
        execute(
            settings,
            spec=spec,
            source=source,
            config=config,
            run_id=run_id,
            fetcher=fetcher,
        )
    )
    return result, requested


def test_spec_parses_crawl_detail_and_detect():
    spec = _spec()
    assert spec.match.detect is not None
    assert spec.match.detect.selector == "body.shop"
    assert spec.crawl == CrawlRules(
        link_selector="a",
        max_depth=1,
        max_pages=10,
        allow=("/catalog/", "/product/"),
        deny=("/cart",),
        same_host=True,
        follow_items=True,
    )
    assert spec.detail is not None
    assert spec.detail.root == "main.detail"
    assert [field.path for field in spec.detail.fields] == [
        "product.description",
        "product.sku",
    ]


def test_crawl_and_detail_enrich_listing_items(settings):
    spec = _spec()
    pages = {
        LISTING_URL: LISTING,
        PAGE_2_URL: PAGE_2,
        PRODUCT_1: DETAIL_1,
        PRODUCT_2: DETAIL_2,
        # PRODUCT_3 is intentionally missing: the run must still succeed.
    }
    result, requested = _run(settings, spec, pages)

    assert result.status == RUN_STATUS_SUCCESS
    assert result.error is None
    assert result.counters.items_found == 3
    assert result.counters.pages_found == 5  # listing + page2 + 3 detail pages
    assert any("crawl page https://shop.test/product/three failed" in w for w in result.warnings)
    assert "https://shop.test/cart" not in requested

    items = {item["externalId"]: item for item in repo.run_items(settings.db_path, result.run_id)}
    assert set(items) == {"P1", "P2", "P3"}
    assert items["P1"]["product"]["description"] == "The first product in detail."
    assert items["P1"]["product"]["sku"] == "SKU-1"
    assert items["P2"]["product"]["description"] == "The second product in detail."
    assert "description" not in items["P3"]["product"]


def test_detail_can_match_by_external_id(settings):
    spec = _spec()
    # The detail page is reached under a different URL, but its declared
    # externalId points at P1.
    alias = "https://shop.test/product/alias"
    raw = copy.deepcopy(SPEC)
    raw["crawl"]["allow"] = ["/catalog/", "/product/"]
    raw["detail"]["externalId"] = {"attribute": "data-product-id"}
    spec = parse_spec(raw, "inline")
    detail = DETAIL_1.replace(
        '<main class="detail">', '<main class="detail" data-product-id="P1">'
    )
    listing = LISTING.replace('href="/product/one"', f'href="{alias}"')
    pages = {
        LISTING_URL: listing,
        PAGE_2_URL: PAGE_2,
        alias: detail,
        PRODUCT_2: DETAIL_2,
    }
    result, _ = _run(settings, spec, pages)

    assert result.status == RUN_STATUS_SUCCESS
    items = {item["externalId"]: item for item in repo.run_items(settings.db_path, result.run_id)}
    assert items["P1"]["product"]["sku"] == "SKU-1"


def test_truncated_crawl_never_removes_catalog_files(settings):
    spec = _spec()
    pages = {
        LISTING_URL: LISTING,
        PAGE_2_URL: PAGE_2,
        PRODUCT_1: DETAIL_1,
        PRODUCT_2: DETAIL_2,
        PRODUCT_3: DETAIL_3,
    }
    first, _ = _run(settings, spec, pages)
    assert first.status == RUN_STATUS_SUCCESS
    assert first.catalog is not None
    assert set(first.catalog.files) == {"P1", "P2", "P3"}
    p3_file = settings.catalog_dir / "crawl-test" / first.catalog.files["P3"]
    assert p3_file.exists()

    # The second run stops the crawl after one page, so P3 is simply not
    # reached. A complete=True catalog would delete it; it must not.
    truncated_raw = copy.deepcopy(SPEC)
    truncated_raw["crawl"]["maxPages"] = 1
    truncated = parse_spec(truncated_raw, "inline")
    second, _ = _run(settings, truncated, pages)
    assert second.status == RUN_STATUS_SUCCESS
    assert second.catalog is not None
    assert second.catalog.removed == 0
    assert p3_file.exists()
    assert any("maxPages=1" in warning for warning in second.warnings)
    index = json.loads(
        (settings.catalog_dir / "crawl-test" / "_index.json").read_text(encoding="utf-8")
    )
    assert index["complete"] is False

    assert _run_was_complete(settings, first.run_id) is True
    assert _run_was_complete(settings, second.run_id) is False


def test_detail_without_crawl_only_visits_item_urls(settings):
    raw = copy.deepcopy(SPEC)
    del raw["crawl"]
    spec = parse_spec(raw, "inline")
    pages = {
        LISTING_URL: LISTING,
        PRODUCT_1: DETAIL_1,
        PRODUCT_2: DETAIL_2,
    }
    result, requested = _run(settings, spec, pages)

    assert result.status == RUN_STATUS_SUCCESS
    assert result.counters.items_found == 2
    assert PAGE_2_URL not in requested
    assert "https://shop.test/cart" not in requested
    items = {item["externalId"]: item for item in repo.run_items(settings.db_path, result.run_id)}
    assert items["P1"]["product"]["sku"] == "SKU-1"
    assert items["P2"]["product"]["sku"] == "SKU-2"


def test_detail_url_outside_match_is_never_fetched(settings):
    spec = _spec()
    listing = LISTING.replace(
        'href="/product/one"', 'href="https://evil.test/product/one"'
    )
    result, requested = _run(
        settings,
        spec,
        {LISTING_URL: listing, PRODUCT_2: DETAIL_2},
    )

    assert result.status == RUN_STATUS_SUCCESS
    assert "https://evil.test/product/one" not in requested
    assert any("not claimed by spec.match" in warning for warning in result.warnings)


def test_detect_rejects_a_page_that_is_not_the_store(settings):
    spec = _spec()
    result, _ = _run(settings, spec, {LISTING_URL: "<html><body>bot wall</body></html>"})
    assert result.status == RUN_STATUS_FAILED
    assert result.error is not None and "match.detect" in result.error
    assert result.counters.pages_found == 1
    assert repo.run_items(settings.db_path, result.run_id) == []


def test_spec_10_cannot_use_new_blocks():
    raw = copy.deepcopy(SPEC)
    raw["schemaVersion"] = "1.0"
    with pytest.raises(SpecError, match="requires schemaVersion"):
        parse_spec(raw, "inline")

    raw = copy.deepcopy(SPEC)
    raw["schemaVersion"] = "1.0"
    del raw["crawl"]
    del raw["detail"]
    with pytest.raises(SpecError, match="requires schemaVersion"):
        parse_spec(raw, "inline")


def test_crawl_without_selector_needs_a_detail_block():
    raw = copy.deepcopy(SPEC)
    del raw["crawl"]["linkSelector"]
    del raw["detail"]
    with pytest.raises(SpecError, match="needs a 'detail'"):
        parse_spec(raw, "inline")


def test_crawl_rejects_unknown_keys():
    raw = copy.deepcopy(SPEC)
    raw["crawl"]["fallbackSelector"] = ".product"
    with pytest.raises(SpecError, match="unknown key"):
        parse_spec(raw, "inline")


def test_detail_extract_reads_partial_values():
    spec = _spec()
    result = extract_detail(spec, DETAIL_1, page_url=PRODUCT_1)
    assert result.values == {
        "product.description": "The first product in detail.",
        "product.sku": "SKU-1",
    }
    assert result.warnings == []


def test_detail_root_missing_is_a_warning_not_an_item():
    spec = _spec()
    result = extract_detail(spec, "<html><body class='shop'></body></html>", page_url=PRODUCT_1)
    assert result.values == {}
    assert result.warnings and "matched nothing" in result.warnings[0]
