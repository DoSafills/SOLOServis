"""SQLite persistence over the four-table schema.

Every function is synchronous and opens its own connection; the pipeline calls
them through ``asyncio.to_thread`` so the event loop is never blocked. WAL plus
``busy_timeout`` lets the control API read while a run writes.
"""

from __future__ import annotations

import json
import sqlite3
from collections.abc import Iterable, Sequence
from contextlib import closing
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

from scraper.errors import StoreMismatchError
from scraper.models import (
    RUN_STATUS_FAILED,
    RUN_STATUS_PENDING,
    RUN_STATUS_RUNNING,
    RUN_STATUSES,
    Counters,
    DataSource,
    Item,
    RunRecord,
    ScraperConfig,
)

SCHEMA_PATH = Path(__file__).with_name("schema.sql")


def now_iso() -> str:
    return datetime.now(UTC).isoformat(timespec="seconds").replace("+00:00", "Z")


def connect(db_path: str | Path) -> sqlite3.Connection:
    conn = sqlite3.connect(str(db_path), timeout=30)
    conn.row_factory = sqlite3.Row
    conn.execute("PRAGMA foreign_keys = ON")
    conn.execute("PRAGMA journal_mode = WAL")
    conn.execute("PRAGMA busy_timeout = 30000")
    return conn


def init_db(db_path: str | Path) -> None:
    Path(db_path).parent.mkdir(parents=True, exist_ok=True)
    with closing(connect(db_path)) as conn, conn:
        conn.executescript(SCHEMA_PATH.read_text(encoding="utf-8"))


# ---------------------------------------------------------------------------
# Sources and configs
# ---------------------------------------------------------------------------


def ensure_source(
    db_path: str | Path,
    *,
    name: str,
    base_url: str,
    store_id: int,
    rate_limit: str | None = None,
    spec_slug: str = "",
    spec_version: str = "",
    spec_hash: str = "",
) -> tuple[int, int, bool]:
    """Register a source + config, or validate the existing pair.

    Returns ``(source_id, config_id, created)``. An existing source keeps its
    identity, but a different ``store_id`` is a configuration bug: the spec and
    the database would disagree about who owns the offers.
    """
    parser_config = {
        "spec": spec_slug,
        "specVersion": spec_version,
        "specHash": spec_hash,
    }
    with closing(connect(db_path)) as conn, conn:
        row = conn.execute(
            "SELECT id, store_id FROM DATA_SOURCE WHERE base_url = ?", (base_url,)
        ).fetchone()
        if row is not None:
            if row["store_id"] is not None and store_id != row["store_id"]:
                raise StoreMismatchError(
                    f"DATA_SOURCE {base_url!r} has store_id={row['store_id']} but "
                    f"the spec declares storeId={store_id}"
                )
            config = conn.execute(
                "SELECT id FROM SCRAPER_CONFIG WHERE data_source_id = ?", (row["id"],)
            ).fetchone()
            if config is None:
                config_id = _insert_config(
                    conn, row["id"], rate_limit, parser_config
                )
            else:
                config_id = int(config["id"])
                conn.execute(
                    "UPDATE SCRAPER_CONFIG SET parser_config = ?, updated_at = CURRENT_TIMESTAMP"
                    " WHERE id = ?",
                    (json.dumps(parser_config), config_id),
                )
            if row["store_id"] is None:
                conn.execute(
                    "UPDATE DATA_SOURCE SET store_id = ?, updated_at = CURRENT_TIMESTAMP"
                    " WHERE id = ?",
                    (store_id, row["id"]),
                )
            return int(row["id"]), config_id, False

        cursor = conn.execute(
            "INSERT INTO DATA_SOURCE (name, base_url, source_type, store_id)"
            " VALUES (?, ?, 'web', ?)",
            (name, base_url, store_id),
        )
        source_id = int(cursor.lastrowid)
        config_id = _insert_config(conn, source_id, rate_limit, parser_config)
        return source_id, config_id, True


def _insert_config(
    conn: sqlite3.Connection,
    source_id: int,
    rate_limit: str | None,
    parser_config: dict[str, Any],
) -> int:
    cursor = conn.execute(
        "INSERT INTO SCRAPER_CONFIG (data_source_id, scraper_type, target_type,"
        " rate_limit, parser_config) VALUES (?, 'http', 'product', ?, ?)",
        (source_id, rate_limit, json.dumps(parser_config)),
    )
    return int(cursor.lastrowid)


def get_active_jobs(db_path: str | Path) -> list[tuple[DataSource, ScraperConfig]]:
    with closing(connect(db_path)) as conn:
        rows = conn.execute(
            "SELECT s.id AS source_id, s.name, s.base_url, s.source_type, s.store_id,"
            "       s.active AS source_active,"
            "       c.id AS config_id, c.data_source_id, c.scraper_type,"
            "       c.target_type, c.rate_limit, c.parser_config, c.active AS config_active"
            " FROM DATA_SOURCE s JOIN SCRAPER_CONFIG c ON c.data_source_id = s.id"
            " WHERE s.active = 1 AND c.active = 1"
            " ORDER BY s.id"
        ).fetchall()
    jobs: list[tuple[DataSource, ScraperConfig]] = []
    for row in rows:
        try:
            parser_config = json.loads(row["parser_config"] or "{}")
        except json.JSONDecodeError:
            parser_config = {}
        jobs.append(
            (
                DataSource(
                    id=int(row["source_id"]),
                    name=row["name"],
                    base_url=row["base_url"],
                    source_type=row["source_type"],
                    store_id=row["store_id"],
                    active=bool(row["source_active"]),
                ),
                ScraperConfig(
                    id=int(row["config_id"]),
                    data_source_id=int(row["data_source_id"]),
                    scraper_type=row["scraper_type"],
                    target_type=row["target_type"],
                    rate_limit=row["rate_limit"],
                    parser_config=parser_config if isinstance(parser_config, dict) else {},
                    active=bool(row["config_active"]),
                ),
            )
        )
    return jobs


# ---------------------------------------------------------------------------
# Runs
# ---------------------------------------------------------------------------


def create_run(
    db_path: str | Path, config_id: int, *, status: str = RUN_STATUS_RUNNING
) -> int:
    if status not in RUN_STATUSES:
        raise ValueError(f"invalid SCRAPE_RUN status {status!r}")
    with closing(connect(db_path)) as conn, conn:
        cursor = conn.execute(
            "INSERT INTO SCRAPE_RUN (scraper_config_id, started_at, status)"
            " VALUES (?, ?, ?)",
            (config_id, now_iso(), status),
        )
        return int(cursor.lastrowid)


def mark_run_running(db_path: str | Path, run_id: int) -> None:
    with closing(connect(db_path)) as conn, conn:
        conn.execute(
            "UPDATE SCRAPE_RUN SET status = ?, started_at = ? WHERE id = ?",
            (RUN_STATUS_RUNNING, now_iso(), run_id),
        )


def update_counters(
    db_path: str | Path,
    run_id: int,
    *,
    counters: Counters | None = None,
    ingestion_run_id: int | None = None,
    ingestion_error: str | None = None,
) -> None:
    fields: list[str] = []
    values: list[Any] = []
    if counters is not None:
        mapping = {
            "records_found": counters.pages_found,
            "records_processed": counters.pages_processed,
            "records_failed": counters.pages_failed,
            "items_found": counters.items_found,
            "items_sent": counters.items_sent,
            "items_failed": counters.items_failed,
            "items_rejected": counters.items_rejected,
        }
        for column, value in mapping.items():
            fields.append(f"{column} = ?")
            values.append(value)
    if ingestion_run_id is not None:
        fields.append("ingestion_run_id = ?")
        values.append(ingestion_run_id)
    if ingestion_error is not None:
        fields.append("ingestion_error = ?")
        values.append(ingestion_error[:2000])
    if not fields:
        return
    values.append(run_id)
    with closing(connect(db_path)) as conn, conn:
        conn.execute(f"UPDATE SCRAPE_RUN SET {', '.join(fields)} WHERE id = ?", values)


def finish_run(
    db_path: str | Path,
    run_id: int,
    status: str,
    *,
    error_message: str | None = None,
    counters: Counters | None = None,
    ingestion_run_id: int | None = None,
    ingestion_error: str | None = None,
) -> None:
    if status not in RUN_STATUSES:
        raise ValueError(f"invalid SCRAPE_RUN status {status!r}: expected {sorted(RUN_STATUSES)}")
    fields = ["status = ?", "finished_at = ?"]
    values: list[Any] = [status, now_iso()]
    if error_message is not None:
        fields.append("error_message = ?")
        values.append(error_message[:2000])
    if counters is not None:
        for column, value in {
            "records_found": counters.pages_found,
            "records_processed": counters.pages_processed,
            "records_failed": counters.pages_failed,
            "items_found": counters.items_found,
            "items_sent": counters.items_sent,
            "items_failed": counters.items_failed,
            "items_rejected": counters.items_rejected,
        }.items():
            fields.append(f"{column} = ?")
            values.append(value)
    if ingestion_run_id is not None:
        fields.append("ingestion_run_id = ?")
        values.append(ingestion_run_id)
    if ingestion_error is not None:
        fields.append("ingestion_error = ?")
        values.append(ingestion_error[:2000])
    values.append(run_id)
    with closing(connect(db_path)) as conn, conn:
        conn.execute(f"UPDATE SCRAPE_RUN SET {', '.join(fields)} WHERE id = ?", values)


def get_run(db_path: str | Path, run_id: int) -> RunRecord | None:
    with closing(connect(db_path)) as conn:
        row = conn.execute(
            "SELECT *, CAST(ROUND((julianday(finished_at) - julianday(started_at))"
            " * 86400000) AS INTEGER) AS duration_ms"
            " FROM SCRAPE_RUN WHERE id = ?",
            (run_id,),
        ).fetchone()
    return _run_from_row(row) if row is not None else None


def list_runs(
    db_path: str | Path, *, status: str | None = None, limit: int = 50
) -> list[RunRecord]:
    query = (
        "SELECT *, CAST(ROUND((julianday(finished_at) - julianday(started_at))"
        " * 86400000) AS INTEGER) AS duration_ms FROM SCRAPE_RUN"
    )
    params: list[Any] = []
    if status:
        query += " WHERE status = ?"
        params.append(status)
    query += " ORDER BY id DESC LIMIT ?"
    params.append(limit)
    with closing(connect(db_path)) as conn:
        rows = conn.execute(query, params).fetchall()
    return [_run_from_row(row) for row in rows]


def active_run_for_config(db_path: str | Path, config_id: int) -> int | None:
    with closing(connect(db_path)) as conn:
        row = conn.execute(
            "SELECT id FROM SCRAPE_RUN WHERE scraper_config_id = ? AND status = ?"
            " ORDER BY id DESC LIMIT 1",
            (config_id, RUN_STATUS_RUNNING),
        ).fetchone()
    return int(row["id"]) if row is not None else None


def open_run_for_config(db_path: str | Path, config_id: int) -> int | None:
    """Return a ``pending`` or ``running`` run for this config, if any."""
    with closing(connect(db_path)) as conn, conn:
        row = conn.execute(
            "SELECT id FROM SCRAPE_RUN WHERE scraper_config_id = ?"
            " AND status IN (?, ?) ORDER BY id DESC LIMIT 1",
            (config_id, RUN_STATUS_PENDING, RUN_STATUS_RUNNING),
        ).fetchone()
    return int(row["id"]) if row is not None else None


def recover_orphan_runs(db_path: str | Path) -> int:
    """Close runs left ``pending``/``running`` by a previous process.

    A single-replica scraper cannot have a legitimate open row at startup: the
    process that owned it is gone.
    """
    with closing(connect(db_path)) as conn, conn:
        cursor = conn.execute(
            "UPDATE SCRAPE_RUN SET status = ?, finished_at = ?,"
            " error_message = 'aborted: process restarted'"
            " WHERE status IN (?, ?)",
            (RUN_STATUS_FAILED, now_iso(), RUN_STATUS_PENDING, RUN_STATUS_RUNNING),
        )
        return cursor.rowcount


def _run_from_row(row: sqlite3.Row) -> RunRecord:
    return RunRecord(
        id=int(row["id"]),
        scraper_config_id=int(row["scraper_config_id"]),
        started_at=row["started_at"],
        finished_at=row["finished_at"],
        records_found=row["records_found"] or 0,
        records_processed=row["records_processed"] or 0,
        records_failed=row["records_failed"] or 0,
        status=row["status"],
        error_message=row["error_message"],
        ingestion_run_id=row["ingestion_run_id"],
        items_found=row["items_found"] or 0,
        items_sent=row["items_sent"] or 0,
        items_failed=row["items_failed"] or 0,
        items_rejected=row["items_rejected"] or 0,
        ingestion_error=row["ingestion_error"],
    )


def run_duration_ms(db_path: str | Path, run_id: int) -> int | None:
    with closing(connect(db_path)) as conn:
        row = conn.execute(
            "SELECT CAST(ROUND((julianday(finished_at) - julianday(started_at))"
            " * 86400000) AS INTEGER) AS duration_ms FROM SCRAPE_RUN WHERE id = ?",
            (run_id,),
        ).fetchone()
    return row["duration_ms"] if row is not None else None


# ---------------------------------------------------------------------------
# Pages and items
# ---------------------------------------------------------------------------


def save_page(
    db_path: str | Path, run_id: int, *, url: str, html: str, external_id: str | None = None
) -> None:
    with closing(connect(db_path)) as conn, conn:
        conn.execute(
            "INSERT INTO SCRAPED_DATA"
            " (scrape_run_id, external_id, external_url, entity_type, raw_data)"
            " VALUES (?, ?, ?, 'webpage', ?)",
            (run_id, external_id, url, html),
        )


def save_items(db_path: str | Path, run_id: int, items: Iterable[Item]) -> int:
    rows = [
        (run_id, item.external_id, item.external_url, item.entity_type, item.to_json())
        for item in items
    ]
    if not rows:
        return 0
    with closing(connect(db_path)) as conn, conn:
        conn.executemany(
            "INSERT INTO SCRAPED_DATA"
            " (scrape_run_id, external_id, external_url, entity_type, raw_data)"
            " VALUES (?, ?, ?, ?, ?)",
            rows,
        )
    return len(rows)


def run_items(db_path: str | Path, run_id: int) -> list[dict[str, Any]]:
    with closing(connect(db_path)) as conn:
        rows = conn.execute(
            "SELECT raw_data FROM SCRAPED_DATA"
            " WHERE scrape_run_id = ? AND entity_type = 'product' ORDER BY id",
            (run_id,),
        ).fetchall()
    return [json.loads(row["raw_data"]) for row in rows if row["raw_data"]]


def run_pages(db_path: str | Path, run_id: int) -> list[dict[str, Any]]:
    with closing(connect(db_path)) as conn:
        rows = conn.execute(
            "SELECT external_url, raw_data FROM SCRAPED_DATA"
            " WHERE scrape_run_id = ? AND entity_type = 'webpage' ORDER BY id",
            (run_id,),
        ).fetchall()
    return [{"url": row["external_url"], "html": row["raw_data"]} for row in rows]


def latest_run_for_config(db_path: str | Path, config_id: int) -> int | None:
    with closing(connect(db_path)) as conn:
        row = conn.execute(
            "SELECT id FROM SCRAPE_RUN WHERE scraper_config_id = ?"
            " AND status IN ('success', 'failed') ORDER BY id DESC LIMIT 1",
            (config_id,),
        ).fetchone()
    return int(row["id"]) if row is not None else None


def previous_run(db_path: str | Path, run_id: int) -> int | None:
    with closing(connect(db_path)) as conn:
        row = conn.execute(
            "SELECT id FROM SCRAPE_RUN WHERE id < ? AND status IN ('success', 'failed')"
            " ORDER BY id DESC LIMIT 1",
            (run_id,),
        ).fetchone()
    return int(row["id"]) if row is not None else None


def config_for_run(db_path: str | Path, run_id: int) -> tuple[DataSource, ScraperConfig] | None:
    with closing(connect(db_path)) as conn:
        row = conn.execute(
            "SELECT s.id AS source_id, s.name, s.base_url, s.source_type, s.store_id,"
            "       s.active AS source_active,"
            "       c.id AS config_id, c.data_source_id, c.scraper_type,"
            "       c.target_type, c.rate_limit, c.parser_config, c.active AS config_active"
            " FROM SCRAPE_RUN r"
            " JOIN SCRAPER_CONFIG c ON c.id = r.scraper_config_id"
            " JOIN DATA_SOURCE s ON s.id = c.data_source_id"
            " WHERE r.id = ?",
            (run_id,),
        ).fetchone()
    if row is None:
        return None
    try:
        parser_config = json.loads(row["parser_config"] or "{}")
    except json.JSONDecodeError:
        parser_config = {}
    return (
        DataSource(
            id=int(row["source_id"]),
            name=row["name"],
            base_url=row["base_url"],
            source_type=row["source_type"],
            store_id=row["store_id"],
            active=bool(row["source_active"]),
        ),
        ScraperConfig(
            id=int(row["config_id"]),
            data_source_id=int(row["data_source_id"]),
            scraper_type=row["scraper_type"],
            target_type=row["target_type"],
            rate_limit=row["rate_limit"],
            parser_config=parser_config if isinstance(parser_config, dict) else {},
            active=bool(row["config_active"]),
        ),
    )


def config_by_base_url(
    db_path: str | Path, base_url: str
) -> tuple[DataSource, ScraperConfig] | None:
    """Look up the source/config pair a spec owns, without creating it."""
    with closing(connect(db_path)) as conn, conn:
        row = conn.execute(
            "SELECT s.id AS source_id, s.name, s.base_url, s.source_type, s.store_id,"
            "       s.active AS source_active,"
            "       c.id AS config_id, c.data_source_id, c.scraper_type,"
            "       c.target_type, c.rate_limit, c.parser_config, c.active AS config_active"
            " FROM DATA_SOURCE s JOIN SCRAPER_CONFIG c ON c.data_source_id = s.id"
            " WHERE s.base_url = ? ORDER BY c.id DESC LIMIT 1",
            (base_url,),
        ).fetchone()
    if row is None:
        return None
    try:
        parser_config = json.loads(row["parser_config"] or "{}")
    except json.JSONDecodeError:
        parser_config = {}
    return (
        DataSource(
            id=int(row["source_id"]),
            name=row["name"],
            base_url=row["base_url"],
            source_type=row["source_type"],
            store_id=row["store_id"],
            active=bool(row["source_active"]),
        ),
        ScraperConfig(
            id=int(row["config_id"]),
            data_source_id=int(row["data_source_id"]),
            scraper_type=row["scraper_type"],
            target_type=row["target_type"],
            rate_limit=row["rate_limit"],
            parser_config=parser_config if isinstance(parser_config, dict) else {},
            active=bool(row["config_active"]),
        ),
    )


def count_rows(db_path: str | Path, table: str) -> int:
    if table not in {"DATA_SOURCE", "SCRAPER_CONFIG", "SCRAPE_RUN", "SCRAPED_DATA"}:
        raise ValueError(f"unknown table {table!r}")
    with closing(connect(db_path)) as conn:
        return int(conn.execute(f"SELECT COUNT(*) FROM {table}").fetchone()[0])


def items_by_external_id(items: Sequence[dict[str, Any]]) -> dict[str, dict[str, Any]]:
    """Collapse a run's items to one per ``externalId`` (last one wins)."""
    result: dict[str, dict[str, Any]] = {}
    for item in items:
        key = item.get("externalId") or item.get("externalUrl")
        if key:
            result[str(key)] = item
    return result
