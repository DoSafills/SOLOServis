"""Ingestion client: batching, auth, retries and the disabled case."""

from __future__ import annotations

import asyncio
import json

import httpx
import pytest

from scraper.config import load_settings
from scraper.errors import ConfigError
from scraper.ingestion import IngestionClient
from scraper.models import Item, Offer, Product


def _item(external_id: str) -> Item:
    return Item(
        product=Product(name=f"Producto {external_id}", category_slug="refrigeradores"),
        offer=Offer(price="1000.00"),
        external_id=external_id,
        external_url=f"https://example.test/p/{external_id}",
    )


def test_client_opens_sends_batches_and_finishes():
    calls: list[tuple[str, dict]] = []

    def handler(request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content)
        calls.append((request.url.path, body))
        if request.url.path.endswith("/items"):
            return httpx.Response(
                200,
                json={
                    "runId": 9,
                    "received": len(body["items"]),
                    "processed": len(body["items"]),
                    "failed": 0,
                    "results": [],
                },
            )
        if request.url.path.endswith("/finish"):
            return httpx.Response(200, json={"id": 9, "status": body["status"]})
        return httpx.Response(201, json={"id": 9, "source": "Ripley", "storeId": 2})

    client = IngestionClient(
        "http://ingest",
        "x" * 32,
        batch_size=1,
        transport=httpx.MockTransport(handler),
    )

    async def scenario():
        run = await client.open_run("Ripley Chile", "https://simple.ripley.cl", 2)
        assert run.id == 9
        batch = await client.send_items(9, [_item("A"), _item("B")])
        assert batch.processed == 2
        await client.finish_run(9, "success")

    asyncio.run(scenario())
    paths = [path for path, _ in calls]
    assert paths.count("/ingestion/runs/9/items") == 2  # batch_size=1
    assert paths[-1] == "/ingestion/runs/9/finish"
    assert calls[0][1]["storeId"] == 2
    assert calls[0][1]["source"]["name"] == "Ripley Chile"


def test_transient_failure_is_retried():
    attempts = {"n": 0}

    def handler(request: httpx.Request) -> httpx.Response:
        attempts["n"] += 1
        if attempts["n"] == 1:
            return httpx.Response(503, json={"detail": "busy"})
        return httpx.Response(201, json={"id": 1})

    client = IngestionClient(
        "http://ingest",
        "x" * 32,
        max_retries=2,
        backoff=0,
        transport=httpx.MockTransport(handler),
    )
    run = asyncio.run(client.open_run("Store", "https://example.test", 1))
    assert run.id == 1
    assert attempts["n"] == 2


def test_short_token_is_a_configuration_error():
    with pytest.raises(ConfigError, match="at least 32"):
        load_settings(
            {
                "INGESTION_API_URL": "http://ingest",
                "INGESTION_API_TOKEN": "too-short",
            }
        )
