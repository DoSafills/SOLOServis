"""End-to-end scraper -> Ingestion API contract, with a fake transport (TAL-145).

The pipeline, the real :class:`IngestionClient` and the JSON bodies are
exercised together. Only the network is replaced, so a change in the payload
shape or in the client paths fails here instead of in production.
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
  <a class="product-link" data-id="RIP-2003" href="/product/refrigerador-mabe-400l">
    <span class="product-item--brand">Mabe</span>
    <span class="product-item--name">Refrigerador Mabe 400 L</span>
    <span class="product-price-price">$799.990</span>
  </a>
</body></html>
"""


class FakeIngestion:
    """Minimal Ingestion API: opens a run, accepts items and finishes it."""

    def __init__(self) -> None:
        self.opened: list[dict] = []
        self.items: list[dict] = []
        self.finished: list[dict] = []
        self.calls: list[str] = []

    def handler(self, request: httpx.Request) -> httpx.Response:
        path = request.url.path
        payload = json.loads(request.content or b"{}")
        self.calls.append(path)

        if path == "/ingestion/runs":
            self.opened.append(payload)
            return httpx.Response(
                201,
                json={
                    "id": 7,
                    "source": payload["source"]["name"],
                    "status": "running",
                    "storeId": payload["storeId"],
                },
            )
        if path.endswith("/items"):
            batch = payload["items"]
            self.items.extend(batch)
            return httpx.Response(
                200,
                json={
                    "runId": 7,
                    "received": len(batch),
                    "processed": len(batch),
                    "failed": 0,
                    "results": [],
                },
            )
        if path.endswith("/finish"):
            self.finished.append(payload)
            return httpx.Response(200, json={"id": 7, "status": payload["status"]})
        return httpx.Response(404, json={"detail": f"unexpected {path}"})


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


def test_pipeline_sends_items_end_to_end(settings, spec_by_slug):
    spec = spec_by_slug["ripley-cl"]
    source, config = _prepare(settings, spec)
    run_id = repo.create_run(settings.db_path, config.id)

    fake = FakeIngestion()
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
        transport=httpx.MockTransport(fake.handler),
    )

    pages = {RIPLEY_PAGE_1: fixture_html("ripley-cl"), RIPLEY_PAGE_2: PAGE_2}
    result = asyncio.run(
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

    assert result.status == RUN_STATUS_SUCCESS
    assert result.ingestion_error is None
    assert result.ingestion_run_id == 7
    assert result.counters.items_found == 3
    assert result.counters.items_sent == 3
    assert result.counters.items_failed == 0

    # The API received exactly the scraped items, one envelope per product.
    assert fake.calls == [
        "/ingestion/runs",
        "/ingestion/runs/7/items",
        "/ingestion/runs/7/finish",
    ]
    assert len(fake.items) == 3
    assert all(item.get("externalId", "").startswith("RIP-") for item in fake.items)
    for item in fake.items:
        assert "product" in item and "offer" in item
        assert item["product"]["name"]
        assert item["offer"]["price"]

    assert fake.opened[0]["storeId"] == spec.store_id
    assert fake.finished == [{"status": "success"}]

    # The local records agree with what was sent.
    record = repo.get_run(settings.db_path, result.run_id)
    assert record.items_sent == 3
    assert record.ingestion_run_id == 7
    snapshot = json.loads(
        (settings.runs_dir / str(result.run_id) / "run.json").read_text(encoding="utf-8")
    )
    assert snapshot["ingestionRunId"] == 7
    assert snapshot["counters"]["itemsSent"] == 3
