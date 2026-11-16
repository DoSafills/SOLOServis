"""Retry policy for page fetches (TAL-208).

Transient failures are retried with exponential backoff; client errors are not.
These tests pin both halves so a future change cannot turn a 404 into three
requests or a 503 into a silent empty page.
"""

from __future__ import annotations

import asyncio

from scraper.fetch.client import Fetcher
from scraper.models import FetchResult


def _transport(responses, calls):
    async def transport(url: str, _timeout: float) -> FetchResult:
        calls.append(url)
        result = responses.pop(0)
        return FetchResult(url=url, status=result[0], error=result[1])

    return transport


def test_transient_status_is_retried_until_success(settings):
    calls: list[str] = []
    fetcher = Fetcher(
        settings,
        transport=_transport([(503, None), (200, None)], calls),
    )

    result = asyncio.run(fetcher.fetch("https://example.test/p/1"))

    assert result.ok
    assert result.status == 200
    assert result.attempts == 2
    assert len(calls) == 2


def test_client_error_is_not_retried(settings):
    calls: list[str] = []
    fetcher = Fetcher(settings, transport=_transport([(404, None)], calls))

    result = asyncio.run(fetcher.fetch("https://example.test/p/1"))

    assert not result.ok
    assert result.status == 404
    assert result.attempts == 1
    assert len(calls) == 1


def test_network_error_is_retried(settings):
    calls: list[str] = []
    fetcher = Fetcher(
        settings,
        transport=_transport([(None, "ConnectError"), (200, None)], calls),
    )

    result = asyncio.run(fetcher.fetch("https://example.test/p/1"))

    assert result.ok
    assert result.attempts == 2
    assert len(calls) == 2


def test_retries_stop_at_the_configured_limit(settings):
    calls: list[str] = []
    fetcher = Fetcher(
        settings,
        transport=_transport([(503, None)] * settings.max_retries, calls),
    )

    result = asyncio.run(fetcher.fetch("https://example.test/p/1"))

    assert not result.ok
    assert result.status == 503
    assert result.attempts == settings.max_retries
    assert len(calls) == settings.max_retries


def test_timeout_counts_as_a_retryable_error(settings):
    calls: list[str] = []
    fetcher = Fetcher(
        settings,
        transport=_transport([(None, "TimeoutException"), (200, None)], calls),
    )

    result = asyncio.run(fetcher.fetch("https://example.test/p/1"))

    assert result.ok
    assert result.attempts == 2
