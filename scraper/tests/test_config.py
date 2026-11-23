"""Ingestion configuration must fail loudly, never silently."""

from __future__ import annotations

import pytest

from scraper.config import load_settings
from scraper.errors import ConfigError


def test_short_ingestion_token_is_rejected() -> None:
    with pytest.raises(ConfigError, match="at least 32"):
        load_settings(
            {
                "INGESTION_API_URL": "http://ingestion:8080",
                "INGESTION_API_TOKEN": "too-short",
            }
        )


def test_missing_url_is_disabled_with_a_reason() -> None:
    settings = load_settings({})
    assert not settings.ingestion_enabled
    assert settings.ingestion_disabled_reason is not None
    assert "INGESTION_API_URL" in settings.ingestion_disabled_reason
    assert settings.ingestion_status.startswith("disabled:")


def test_url_without_token_is_disabled_not_fatal() -> None:
    settings = load_settings({"INGESTION_API_URL": "http://ingestion:8080"})
    assert not settings.ingestion_enabled
    assert "empty" in settings.ingestion_disabled_reason


def test_full_config_enables_ingestion() -> None:
    settings = load_settings(
        {
            "INGESTION_API_URL": "http://ingestion:8080",
            "INGESTION_API_TOKEN": "t" * 32,
        }
    )
    assert settings.ingestion_enabled
    assert settings.ingestion_disabled_reason is None
    assert settings.ingestion_status == "enabled"


def test_short_control_api_token_is_rejected() -> None:
    """A control API token under 32 characters stops the process (TAL-199, H10)."""
    with pytest.raises(ConfigError, match="SCRAPER_API_TOKEN must be at least 32"):
        load_settings({"SCRAPER_API_TOKEN": "too-short"})


def test_short_internal_token_fallback_is_rejected() -> None:
    """The INTERNAL_API_TOKEN fallback follows the same rule as SCRAPER_API_TOKEN."""
    with pytest.raises(ConfigError, match="SCRAPER_API_TOKEN must be at least 32"):
        load_settings({"INTERNAL_API_TOKEN": "x" * 31})


def test_empty_control_api_token_still_starts() -> None:
    """Without a token the process starts and the protected routes answer 503."""
    assert load_settings({}).api_token == ""
