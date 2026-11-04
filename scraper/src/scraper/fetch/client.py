"""HTTP fetching with a shared rate limit and bounded transient retries.

The transport is injectable so tests run offline: a test passes a coroutine
that returns canned HTML. Nothing else in the package calls the network
directly, which keeps the "spec or nothing" promise easy to audit.
"""

from __future__ import annotations

import asyncio
from collections.abc import Awaitable, Callable
from typing import Any

from scraper.config import Settings
from scraper.fetch.limiter import RateLimiter
from scraper.models import FetchResult

Transport = Callable[[str, float], Awaitable[FetchResult]]

RETRYABLE_STATUSES = frozenset({408, 425, 429, 500, 502, 503, 504})


async def scrapling_transport(url: str, timeout: float) -> FetchResult:
    """Fetch ``url`` with Scrapling's HTTP client (no browser)."""
    from scrapling.fetchers import AsyncFetcher

    try:
        response = await AsyncFetcher.get(
            url,
            timeout=timeout,
            stealthy_headers=True,
            follow_redirects=True,
        )
    except Exception as exc:  # network, TLS and fingerprint failures
        return FetchResult(url=url, error=f"{type(exc).__name__}: {exc}")

    body = getattr(response, "body", b"")
    if isinstance(body, bytes):
        body = body.decode(getattr(response, "encoding", None) or "utf-8", errors="replace")
    return FetchResult(url=url, status=int(getattr(response, "status", 0) or 0), body=str(body))


class Fetcher:
    """Fetch pages for one run, respecting the configured rate limit."""

    def __init__(
        self,
        settings: Settings,
        *,
        transport: Transport | None = None,
        limiter: RateLimiter | None = None,
    ) -> None:
        self.settings = settings
        self.transport: Transport = transport or scrapling_transport
        self.limiter = limiter or RateLimiter(settings.rate_interval)

    async def fetch(self, url: str) -> FetchResult:
        """Fetch ``url``, retrying transient failures with exponential backoff."""
        attempts = 0
        last = FetchResult(url=url, error="no attempt was made")
        while attempts < max(1, self.settings.max_retries):
            attempts += 1
            async with self.limiter:
                result = await self.transport(url, self.settings.timeout)
            result.attempts = attempts
            if result.ok:
                return result
            last = result
            if not _retryable(result) or attempts >= self.settings.max_retries:
                break
            delay = self.settings.backoff_base * (2 ** (attempts - 1))
            await asyncio.sleep(delay)
        last.attempts = attempts
        return last


def _retryable(result: FetchResult) -> bool:
    if result.error is not None:
        return True
    return result.status in RETRYABLE_STATUSES


def fetch_all(
    fetcher: Fetcher, urls: list[str], *, max_concurrency: int
) -> list[FetchResult]:
    """Fetch several URLs concurrently, preserving input order.

    This is a helper for callers that already have the whole URL list (for
    example a test or an export tool); the pipeline walks pages sequentially
    because page *n+1* is discovered on page *n*.
    """

    async def run() -> list[FetchResult]:
        semaphore = asyncio.Semaphore(max(1, max_concurrency))

        async def one(url: str) -> FetchResult:
            async with semaphore:
                return await fetcher.fetch(url)

        return list(await asyncio.gather(*(one(url) for url in urls)))

    return asyncio.run(run())


def as_dict(result: FetchResult) -> dict[str, Any]:
    return {
        "url": result.url,
        "status": result.status,
        "attempts": result.attempts,
        "error": result.error,
        "bytes": len(result.body),
    }
