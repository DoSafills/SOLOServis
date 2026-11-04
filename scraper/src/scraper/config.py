"""Environment configuration.

One immutable :class:`Settings` object is built once per process and passed
down explicitly; nothing in the package reads ``os.environ`` at import time.
That keeps tests hermetic and makes the K8s manifest the single place where
deployment values live.
"""

from __future__ import annotations

import os
from collections.abc import Mapping
from dataclasses import dataclass
from pathlib import Path

from scraper.errors import ConfigError

RATE_UNITS: dict[str, float] = {"s": 1.0, "min": 60.0, "h": 3600.0}


def parse_rate_limit(raw: str) -> float:
    """Turn ``"60/min"`` into the minimum number of seconds between requests."""
    text = (raw or "").strip().lower()
    if not text:
        return 0.0
    count_text, _, unit = text.partition("/")
    try:
        count = int(count_text)
    except ValueError as exc:
        raise ConfigError(f"invalid rate limit {raw!r}; expected <n>/<s|min|h>") from exc
    if count <= 0 or unit not in RATE_UNITS:
        raise ConfigError(f"invalid rate limit {raw!r}; expected <n>/<s|min|h>")
    return RATE_UNITS[unit] / count


def _int(env: Mapping[str, str], key: str, default: int) -> int:
    raw = env.get(key)
    if raw is None or raw.strip() == "":
        return default
    try:
        return int(raw)
    except ValueError as exc:
        raise ConfigError(f"{key} must be an integer, got {raw!r}") from exc


def _float(env: Mapping[str, str], key: str, default: float) -> float:
    raw = env.get(key)
    if raw is None or raw.strip() == "":
        return default
    try:
        return float(raw)
    except ValueError as exc:
        raise ConfigError(f"{key} must be a number, got {raw!r}") from exc


def _bool(env: Mapping[str, str], key: str, default: bool) -> bool:
    raw = env.get(key)
    if raw is None or raw.strip() == "":
        return default
    text = raw.strip().lower()
    if text in {"1", "true", "yes", "on"}:
        return True
    if text in {"0", "false", "no", "off"}:
        return False
    raise ConfigError(f"{key} must be a boolean, got {raw!r}")


@dataclass(frozen=True, slots=True)
class Settings:
    """Every knob the scraper reads, resolved and typed."""

    data_dir: Path
    db_path: Path
    stores_dir: Path
    catalog_dir: Path
    runs_dir: Path
    rejects_dir: Path
    tasks_dir: Path

    max_concurrency: int = 4
    max_pages: int = 20
    max_crawl_pages: int = 200
    rate_limit: str = "60/min"
    timeout: float = 15.0
    max_retries: int = 3
    backoff_base: float = 1.0

    max_parallel_tasks: int = 2
    scheduler_enabled: bool = True
    scheduler_tick: float = 5.0
    task_history_limit: int = 50

    api_host: str = "0.0.0.0"
    api_port: int = 8095
    api_token: str = ""
    stop_timeout: float = 20.0

    ingestion_url: str = ""
    ingestion_token: str = ""
    ingestion_timeout: float = 15.0
    ingestion_batch_size: int = 100
    ingestion_max_retries: int = 3

    @property
    def rate_interval(self) -> float:
        return parse_rate_limit(self.rate_limit)

    @property
    def ingestion_enabled(self) -> bool:
        return bool(self.ingestion_url and len(self.ingestion_token) >= 32)

    def ensure_dirs(self) -> None:
        """Create the directories the process writes to."""
        for path in (
            self.data_dir,
            self.catalog_dir,
            self.runs_dir,
            self.rejects_dir,
            self.tasks_dir,
            self.db_path.parent,
        ):
            path.mkdir(parents=True, exist_ok=True)


def load_settings(env: Mapping[str, str] | None = None) -> Settings:
    """Build :class:`Settings` from the environment (or an explicit mapping)."""
    source = os.environ if env is None else env
    data_dir = Path(source.get("SCRAPER_DATA_DIR", "out")).expanduser()
    db_path = Path(source.get("SCRAPER_DB_PATH", str(data_dir / "scraper.db")))
    stores_dir = Path(source.get("SCRAPER_STORES_DIR", "stores")).expanduser()

    settings = Settings(
        data_dir=data_dir,
        db_path=db_path,
        stores_dir=stores_dir,
        catalog_dir=Path(source.get("SCRAPER_CATALOG_DIR", str(data_dir / "catalog"))),
        runs_dir=Path(source.get("SCRAPER_RUNS_DIR", str(data_dir / "runs"))),
        rejects_dir=Path(source.get("SCRAPER_REJECTS_DIR", str(data_dir / "rejected"))),
        tasks_dir=Path(source.get("SCRAPER_TASKS_DIR", str(data_dir / "tasks"))),
        max_concurrency=_int(source, "SCRAPER_MAX_CONCURRENCY", 4),
        max_pages=_int(source, "SCRAPER_MAX_PAGES", 20),
        max_crawl_pages=_int(source, "SCRAPER_MAX_CRAWL_PAGES", 200),
        rate_limit=source.get("SCRAPER_RATE_LIMIT", "60/min"),
        timeout=_float(source, "SCRAPER_TIMEOUT", 15.0),
        max_retries=_int(source, "SCRAPER_MAX_RETRIES", 3),
        backoff_base=_float(source, "SCRAPER_BACKOFF_BASE", 1.0),
        max_parallel_tasks=_int(source, "SCRAPER_MAX_PARALLEL_TASKS", 2),
        scheduler_enabled=_bool(source, "SCRAPER_SCHEDULER_ENABLED", True),
        scheduler_tick=_float(source, "SCRAPER_SCHEDULER_TICK", 5.0),
        task_history_limit=_int(source, "SCRAPER_TASK_HISTORY_LIMIT", 50),
        api_host=source.get("SCRAPER_API_HOST", "0.0.0.0"),
        api_port=_int(source, "SCRAPER_API_PORT", 8095),
        api_token=source.get("SCRAPER_API_TOKEN")
        or source.get("INTERNAL_API_TOKEN", ""),
        stop_timeout=_float(source, "SCRAPER_STOP_TIMEOUT", 20.0),
        ingestion_url=source.get("INGESTION_API_URL", "").rstrip("/"),
        ingestion_token=source.get("INGESTION_API_TOKEN")
        or source.get("INTERNAL_API_TOKEN", ""),
        ingestion_timeout=_float(source, "INGESTION_TIMEOUT", 15.0),
        ingestion_batch_size=_int(source, "INGESTION_BATCH_SIZE", 100),
        ingestion_max_retries=_int(source, "INGESTION_MAX_RETRIES", 3),
    )
    if settings.max_concurrency < 1:
        raise ConfigError("SCRAPER_MAX_CONCURRENCY must be >= 1")
    if settings.max_pages < 1:
        raise ConfigError("SCRAPER_MAX_PAGES must be >= 1")
    if settings.max_crawl_pages < 1:
        raise ConfigError("SCRAPER_MAX_CRAWL_PAGES must be >= 1")
    if settings.max_parallel_tasks < 1:
        raise ConfigError("SCRAPER_MAX_PARALLEL_TASKS must be >= 1")
    if settings.scheduler_tick < 0.1:
        raise ConfigError("SCRAPER_SCHEDULER_TICK must be >= 0.1")
    if settings.task_history_limit < 1:
        raise ConfigError("SCRAPER_TASK_HISTORY_LIMIT must be >= 1")
    if not 1 <= settings.ingestion_batch_size <= 100:
        raise ConfigError("INGESTION_BATCH_SIZE must be between 1 and 100")
    return settings
