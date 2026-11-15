"""Failure recovery around ingestion (TAL-147).

These tests answer a single question: when the Ingestion API is unreachable,
rejects a batch or fails to close the run, does the local crawl still end
cleanly and leave a diagnosable record?

PostgreSQL availability is not the scraper's concern: it only speaks to
SQLite and HTTP. The backend side is covered by
``backend/internal/integration/unavailable_test.go``.
"""

from __future__ import annotations

import asyncio
import json
from dataclasses import replace

import httpx
from conftest import fixture_html, transport_for

from scraper.db import repository as repo
from scraper.fetch.client import Fetcher
from scraper.ingestion import IngestionClient
from scraper.models import RUN_STATUS_SUCCESS
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
    return repo.config_by_base_url(settings.db_path, spec.base_url)


def _execute(settings, spec, handler):
    source, config = _prepare(settings, spec)
    run_id = repo.create_run(settings.db_path, config.id)
    enabled = replace(
        settings,
        ingestion_url="http://ingestion:8080",
        ingestion_token="t" * 32,
        ingestion_max_retries=0,
    )
    client = IngestionClient(
        enabled.ingestion_url,
        enabled.ingestion_token,
        timeout=5,
        max_retries=0,
        backoff=0,
        transport=httpx.MockTransport(handler),
    )
    pages = {RIPLEY_PAGE_1: fixture_html("ripley-cl"), RIPLEY_PAGE_2: PAGE_2}
    return asyncio.run(
        execute(
            enabled,
            spec=spec,
            source=source,
            config=config,
            run_id=run_id,
            stop_event=asyncio.Event(),
            fetcher=Fetcher(enabled, transport=transport_for(pages)),
            ingestion=client,
        )
    )


def test_unreachable_ingestion_does_not_fail_the_run(settings, spec_by_slug):
    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("connection refused", request=request)

    result = _execute(settings, spec_by_slug["ripley-cl"], handler)

    assert result.status == RUN_STATUS_SUCCESS
    assert result.counters.items_found == 2
    assert result.counters.items_sent == 0
    assert result.ingestion_run_id is None
    assert result.ingestion_error is not None
    assert "could not open ingestion run" in result.ingestion_error

    snapshot = json.loads(
        (settings.runs_dir / str(result.run_id) / "run.json").read_text(encoding="utf-8")
    )
    assert snapshot["ingestionError"] == result.ingestion_error
    assert snapshot["catalog"]["added"] == 2


def test_rejected_batch_closes_the_run_as_failed(settings, spec_by_slug):
    finished: list[dict] = []

    def handler(request: httpx.Request) -> httpx.Response:
        path = request.url.path
        if path == "/ingestion/runs":
            return httpx.Response(201, json={"id": 9, "source": "Ripley", "status": "running"})
        if path.endswith("/items"):
            return httpx.Response(422, json={"detail": "category requires capacidad_litros"})
        if path.endswith("/finish"):
            finished.append(json.loads(request.content))
            return httpx.Response(200, json={"id": 9, "status": "failed"})
        return httpx.Response(404)

    result = _execute(settings, spec_by_slug["ripley-cl"], handler)

    assert result.status == RUN_STATUS_SUCCESS
    assert result.counters.items_sent == 0
    assert result.ingestion_run_id == 9
    assert result.ingestion_error is not None
    assert "could not send items" in result.ingestion_error
    assert "capacidad_litros" in result.ingestion_error
    assert finished[0]["status"] == "failed"
    assert "capacidad_litros" in finished[0]["errorMessage"]


def test_finished_run_failure_keeps_the_sent_count(settings, spec_by_slug):
    def handler(request: httpx.Request) -> httpx.Response:
        path = request.url.path
        if path == "/ingestion/runs":
            return httpx.Response(201, json={"id": 9, "source": "Ripley", "status": "running"})
        if path.endswith("/items"):
            payload = json.loads(request.content)
            return httpx.Response(
                200,
                json={
                    "runId": 9,
                    "received": len(payload["items"]),
                    "processed": len(payload["items"]),
                    "failed": 0,
                },
            )
        if path.endswith("/finish"):
            return httpx.Response(500, json={"detail": "database is down"})
        return httpx.Response(404)

    result = _execute(settings, spec_by_slug["ripley-cl"], handler)

    assert result.status == RUN_STATUS_SUCCESS
    assert result.counters.items_sent == 2
    assert result.ingestion_error is not None
    assert "could not finish ingestion run" in result.ingestion_error
    assert "database is down" in result.ingestion_error


def test_cancelled_crawl_mid_page_leaves_no_open_run(settings, spec_by_slug):
    spec = spec_by_slug["ripley-cl"]
    source, config = _prepare(settings, spec)
    run_id = repo.create_run(settings.db_path, config.id)
    stop_event = asyncio.Event()
    stop_event.set()
    pages = {RIPLEY_PAGE_1: fixture_html("ripley-cl")}
    result = asyncio.run(
        execute(
            settings,
            spec=spec,
            source=source,
            config=config,
            run_id=run_id,
            stop_event=stop_event,
            fetcher=Fetcher(settings, transport=transport_for(pages)),
        )
    )
    assert result.status == "failed"
    assert result.error == "stopped by request"
    record = repo.get_run(settings.db_path, run_id)
    assert record.status == "failed"
    assert record.finished_at is not None
