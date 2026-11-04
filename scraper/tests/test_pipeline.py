"""End-to-end pipeline tests with an offline transport."""

from __future__ import annotations

import asyncio
import json

from conftest import fixture_html, transport_for

from scraper.db import repository as repo
from scraper.fetch.client import Fetcher
from scraper.models import RUN_STATUS_FAILED, RUN_STATUS_SUCCESS
from scraper.pipeline import execute

RIPLEY_PAGE_1 = "https://simple.ripley.cl/search/refrigerador"
RIPLEY_PAGE_2 = "https://simple.ripley.cl/search/refrigerador?page=2"

PAGE_2 = """
<html><body>
  <a class="product-link" data-id="RIP-2001" href="/product/refrigerador-samsung-300l">
    <span class="product-item--brand">Samsung</span>
    <span class="product-item--name">Refrigerador No Frost 300 L</span>
    <span class="product-price-price">$529.990</span>
  </a>
  <a class="product-link" data-id="RIP-2003" href="/product/refrigerador-mabe-400l">
    <span class="product-item--brand">Mabe</span>
    <span class="product-item--name">Refrigerador Mabe 400 L</span>
    <span class="product-price-price">$799.990</span>
  </a>
</body></html>
"""


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


def _run(settings, spec, pages, *, stop=False):
    source, config = _prepare(settings, spec)
    run_id = repo.create_run(settings.db_path, config.id)
    stop_event = asyncio.Event()
    if stop:
        stop_event.set()
    fetcher = Fetcher(settings, transport=transport_for(pages))
    return asyncio.run(
        execute(
            settings,
            spec=spec,
            source=source,
            config=config,
            run_id=run_id,
            stop_event=stop_event,
            fetcher=fetcher,
        )
    )


def test_pipeline_paginates_dedupes_and_persists(settings, spec_by_slug):
    spec = spec_by_slug["ripley-cl"]
    pages = {RIPLEY_PAGE_1: fixture_html("ripley-cl"), RIPLEY_PAGE_2: PAGE_2}
    result = _run(settings, spec, pages)

    assert result.status == RUN_STATUS_SUCCESS
    assert result.counters.pages_found == 2
    assert result.counters.pages_processed == 2
    assert result.counters.items_found == 3  # RIP-2001 is deduped
    assert result.catalog is not None
    assert result.catalog.added == 3

    items = repo.run_items(settings.db_path, result.run_id)
    assert {item["externalId"] for item in items} == {"RIP-2001", "RIP-2002", "RIP-2003"}
    assert len(repo.run_pages(settings.db_path, result.run_id)) == 2

    record = repo.get_run(settings.db_path, result.run_id)
    assert record.status == RUN_STATUS_SUCCESS
    assert record.items_found == 3
    assert record.finished_at is not None

    snapshot = json.loads(
        (settings.runs_dir / str(result.run_id) / "run.json").read_text(encoding="utf-8")
    )
    assert snapshot["store"] == "ripley-cl"
    assert snapshot["spec"]["digest"] == spec.digest
    assert snapshot["catalog"]["added"] == 3
    assert (settings.runs_dir / str(result.run_id) / "pages" / "001-2.html").exists() or any(
        (settings.runs_dir / str(result.run_id) / "pages").iterdir()
    )


def test_second_identical_run_changes_nothing(settings, spec_by_slug):
    spec = spec_by_slug["ripley-cl"]
    pages = {RIPLEY_PAGE_1: fixture_html("ripley-cl"), RIPLEY_PAGE_2: PAGE_2}
    first = _run(settings, spec, pages)
    second = _run(settings, spec, pages)
    assert second.status == RUN_STATUS_SUCCESS
    assert second.catalog.added == 0
    assert second.catalog.updated == 0
    assert second.catalog.unchanged == 3
    assert first.run_id != second.run_id


def test_changed_price_updates_one_catalog_file(settings, spec_by_slug):
    spec = spec_by_slug["ripley-cl"]
    pages = {RIPLEY_PAGE_1: fixture_html("ripley-cl"), RIPLEY_PAGE_2: PAGE_2}
    _run(settings, spec, pages)
    changed = PAGE_2.replace("$799.990", "$749.990")
    result = _run(
        settings,
        spec,
        {RIPLEY_PAGE_1: fixture_html("ripley-cl"), RIPLEY_PAGE_2: changed},
    )
    assert result.catalog.updated == 1
    assert result.catalog.unchanged == 2


def test_empty_first_listing_fails_without_guessing(settings, spec_by_slug):
    spec = spec_by_slug["ripley-cl"]
    result = _run(settings, spec, {RIPLEY_PAGE_1: "<html><body></body></html>"})
    assert result.status == RUN_STATUS_FAILED
    assert result.error is not None and "no cards" in result.error
    assert result.counters.pages_failed == 1


def test_stop_before_first_page_is_controlled(settings, spec_by_slug):
    spec = spec_by_slug["ripley-cl"]
    result = _run(settings, spec, {RIPLEY_PAGE_1: fixture_html("ripley-cl")}, stop=True)
    assert result.status == RUN_STATUS_FAILED
    assert result.error == "stopped by request"
    assert result.counters.pages_found == 0


def test_fetch_failure_marks_the_run_failed(settings, spec_by_slug):
    spec = spec_by_slug["ripley-cl"]
    result = _run(settings, spec, {})
    assert result.status == RUN_STATUS_FAILED
    assert result.error is not None and "failed" in result.error
    record = repo.get_run(settings.db_path, result.run_id)
    assert record.status == RUN_STATUS_FAILED
