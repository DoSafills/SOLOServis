"""Client for the SOLOServis Ingestion API.

The contract is the same one the v1 crawler speaks:

* ``POST /ingestion/runs`` opens a run (one per local ``SCRAPE_RUN``);
* ``POST /ingestion/runs/{id}/items`` sends 1..100 items;
* ``POST /ingestion/runs/{id}/finish`` closes it as ``success`` or ``failed``.

The client is disabled unless both a URL and a token of at least 32 characters
are configured, because the backend's authz ignores shorter tokens. Retries
are limited to transient failures (408/429/5xx and network errors); the offer
upsert is idempotent, but a retried batch can still inflate counters.
"""

from __future__ import annotations

import asyncio
from dataclasses import dataclass, field
from typing import Any

import httpx

from scraper.config import Settings
from scraper.errors import IngestionError
from scraper.models import Item

TRANSIENT_STATUS = frozenset({408, 429, 500, 502, 503, 504})


@dataclass(slots=True)
class IngestionRun:
    id: int
    source: str = ""
    status: str = ""
    source_url: str | None = None
    store_id: int | None = None
    records_found: int = 0
    records_processed: int = 0
    records_failed: int = 0

    @classmethod
    def from_payload(cls, payload: dict[str, Any]) -> IngestionRun:
        return cls(
            id=int(payload.get("id") or 0),
            source=str(payload.get("source") or ""),
            status=str(payload.get("status") or ""),
            source_url=payload.get("sourceUrl"),
            store_id=payload.get("storeId"),
            records_found=int(payload.get("recordsFound") or 0),
            records_processed=int(payload.get("recordsProcessed") or 0),
            records_failed=int(payload.get("recordsFailed") or 0),
        )


@dataclass(slots=True)
class ItemResult:
    index: int
    status: str
    external_id: str | None = None
    product_id: str | None = None
    errors: list[dict[str, Any]] = field(default_factory=list)

    @classmethod
    def from_payload(cls, payload: dict[str, Any]) -> ItemResult:
        return cls(
            index=int(payload.get("index") or 0),
            status=str(payload.get("status") or ""),
            external_id=payload.get("externalId"),
            product_id=payload.get("productId"),
            errors=list(payload.get("errors") or []),
        )


@dataclass(slots=True)
class BatchResult:
    run_id: int
    received: int = 0
    processed: int = 0
    failed: int = 0
    results: list[ItemResult] = field(default_factory=list)

    @classmethod
    def from_payload(cls, payload: dict[str, Any]) -> BatchResult:
        return cls(
            run_id=int(payload.get("runId") or 0),
            received=int(payload.get("received") or 0),
            processed=int(payload.get("processed") or 0),
            failed=int(payload.get("failed") or 0),
            results=[ItemResult.from_payload(entry) for entry in payload.get("results") or ()],
        )


class IngestionClient:
    """Async, stateless client. Use :meth:`from_settings` to honor config."""

    def __init__(
        self,
        base_url: str,
        token: str,
        *,
        timeout: float = 15.0,
        max_retries: int = 3,
        backoff: float = 1.0,
        batch_size: int = 100,
        transport: httpx.AsyncBaseTransport | None = None,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.token = token
        self.timeout = timeout
        self.max_retries = max(0, max_retries)
        self.backoff = backoff
        self.batch_size = max(1, min(100, batch_size))
        self._transport = transport

    @classmethod
    def from_settings(
        cls, settings: Settings, *, transport: httpx.AsyncBaseTransport | None = None
    ) -> IngestionClient | None:
        if not settings.ingestion_enabled:
            return None
        return cls(
            settings.ingestion_url,
            settings.ingestion_token,
            timeout=settings.ingestion_timeout,
            max_retries=settings.ingestion_max_retries,
            batch_size=settings.ingestion_batch_size,
            transport=transport,
        )

    async def open_run(
        self, source_name: str, base_url: str | None, store_id: int, scraper_type: str = "http"
    ) -> IngestionRun:
        payload = {
            "source": {"name": source_name, "baseUrl": base_url or ""},
            "storeId": store_id,
            "scraperType": scraper_type,
        }
        status, body = await self._call("/ingestion/runs", payload)
        if status != 201:
            raise IngestionError(f"expected 201 opening the run, got {status}", status=status)
        return IngestionRun.from_payload(body)

    async def send_items(self, run_id: int, items: list[Item]) -> BatchResult:
        if not items:
            raise ValueError("send_items needs at least one item")
        merged = BatchResult(run_id=run_id)
        for offset in range(0, len(items), self.batch_size):
            chunk = items[offset : offset + self.batch_size]
            status, body = await self._call(
                f"/ingestion/runs/{run_id}/items",
                {"items": [item.to_ingestion() for item in chunk]},
            )
            if status != 200:
                raise IngestionError(f"expected 200 sending items, got {status}", status=status)
            batch = BatchResult.from_payload(body)
            merged.received += batch.received
            merged.processed += batch.processed
            merged.failed += batch.failed
            merged.results.extend(batch.results)
        return merged

    async def finish_run(
        self, run_id: int, status: str, error_message: str | None = None
    ) -> IngestionRun:
        if status not in {"success", "failed"}:
            raise ValueError(f"invalid ingestion status {status!r}")
        payload: dict[str, Any] = {"status": status}
        if error_message:
            payload["errorMessage"] = error_message[:2000]
        http_status, body = await self._call(f"/ingestion/runs/{run_id}/finish", payload)
        if http_status != 200:
            raise IngestionError(
                f"expected 200 finishing the run, got {http_status}", status=http_status
            )
        return IngestionRun.from_payload(body)

    async def _call(self, path: str, payload: dict[str, Any]) -> tuple[int, dict[str, Any]]:
        url = f"{self.base_url}{path}"
        headers = {
            "Authorization": f"Bearer {self.token}",
            "Content-Type": "application/json",
            "Accept": "application/json",
            "User-Agent": "SOLOServis-scraper/2.0",
        }
        attempt = 0
        async with httpx.AsyncClient(
            timeout=self.timeout, transport=self._transport, follow_redirects=False
        ) as client:
            while True:
                try:
                    response = await client.post(url, json=payload, headers=headers)
                except (httpx.TransportError, httpx.TimeoutException) as exc:
                    error = IngestionError(f"could not reach {url}: {exc}")
                    if attempt >= self.max_retries:
                        raise error from exc
                else:
                    body = _decode(response)
                    if response.status_code < 400:
                        return response.status_code, body
                    error = IngestionError(
                        str(body.get("detail") or body.get("title") or response.reason_phrase),
                        status=response.status_code,
                    )
                    if response.status_code not in TRANSIENT_STATUS or attempt >= self.max_retries:
                        raise error
                delay = self.backoff * (2**attempt)
                await asyncio.sleep(delay)
                attempt += 1


def _decode(response: httpx.Response) -> dict[str, Any]:
    try:
        payload = response.json()
    except ValueError:
        return {"detail": response.text[:500]}
    return payload if isinstance(payload, dict) else {"detail": str(payload)[:500]}
