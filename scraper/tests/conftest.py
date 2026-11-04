"""Shared fixtures: temporary data dirs, real specs and offline transports."""

from __future__ import annotations

import json
from collections.abc import Callable
from pathlib import Path
from typing import Any

import pytest

from scraper.config import Settings, load_settings
from scraper.models import FetchResult, StoreSpec
from scraper.spec.loader import load_specs

ROOT = Path(__file__).resolve().parent.parent
STORES = ROOT / "stores"
FIXTURES = Path(__file__).resolve().parent / "fixtures"


@pytest.fixture
def settings(tmp_path: Path) -> Settings:
    return load_settings(
        {
            "SCRAPER_DATA_DIR": str(tmp_path / "out"),
            "SCRAPER_STORES_DIR": str(STORES),
            "SCRAPER_RATE_LIMIT": "1000/s",
            "SCRAPER_MAX_RETRIES": "2",
            "SCRAPER_BACKOFF_BASE": "0",
            "SCRAPER_API_TOKEN": "test-token-that-is-long-enough",
        }
    )


@pytest.fixture
def specs() -> list[StoreSpec]:
    return load_specs(STORES)


@pytest.fixture
def spec_by_slug(specs: list[StoreSpec]) -> dict[str, StoreSpec]:
    return {spec.slug: spec for spec in specs}


def fixture_html(store: str, name: str = "listing.html") -> str:
    return (FIXTURES / store / name).read_text(encoding="utf-8")


def spec_raw(slug: str) -> dict[str, Any]:
    return json.loads((STORES / slug / "spec.json").read_text(encoding="utf-8"))


def transport_for(
    pages: dict[str, str],
) -> Callable[[str, float], Any]:
    """A transport that serves canned HTML and 404s everything else."""

    async def transport(url: str, _timeout: float) -> FetchResult:
        if url in pages:
            return FetchResult(url=url, status=200, body=pages[url])
        return FetchResult(url=url, status=404, body="", error=None)

    return transport
