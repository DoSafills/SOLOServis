"""Artefactos locales por ejecución para auditoría y replay (TAL-136).

SQLite ya conserva el HTML crudo, pero no es un lugar cómodo para inspeccionar
una ejecución ni para entregarla a otra persona. Este módulo escribe el mismo
material en archivos con el ``run_id`` en el nombre:

* ``out/items/run-{id}.jsonl``: un ítem normalizado por línea, append-only.
* ``out/items/run-{id}.json``: manifiesto legible con metadatos y la lista.
* ``out/raw/run-{id}/<sha1>.html``: HTML completo de cada página.
* ``out/raw/run-{id}/index.jsonl``: mapa URL -> archivo para replay.

El camino de runtime **no vuelve a parsear**: exporta los ``NormalizedItem`` que
acaba de extraer el crawler. La excepción explícita es :func:`replay_run`, que
lee ``SCRAPED_DATA.raw_data`` y aplica un bundle para auditar ejecuciones
antiguas o probar selectores nuevos.
"""

import hashlib
import json
import os
from collections.abc import Iterable
from contextlib import closing
from dataclasses import dataclass, field
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

from database import connect
from normalized import (
    SCHEMA_VERSION,
    BundleExtractor,
    BundleSpec,
    NormalizedItem,
    SourceInfo,
)

# ---------------------------------------------------------------------------
# Rutas
# ---------------------------------------------------------------------------


def items_jsonl_path(items_dir: str | Path, run_id: int) -> Path:
    return Path(items_dir) / f"run-{run_id}.jsonl"


def items_json_path(items_dir: str | Path, run_id: int) -> Path:
    return Path(items_dir) / f"run-{run_id}.json"


def raw_run_dir(raw_dir: str | Path, run_id: int) -> Path:
    return Path(raw_dir) / f"run-{run_id}"


def raw_index_path(raw_dir: str | Path, run_id: int) -> Path:
    return raw_run_dir(raw_dir, run_id) / "index.jsonl"


def _now() -> str:
    return datetime.now(UTC).isoformat(timespec="seconds").replace(
        "+00:00", "Z")


# ---------------------------------------------------------------------------
# Ítems normalizados
# ---------------------------------------------------------------------------


def append_items(
    items: Iterable[NormalizedItem],
    path: str | Path,
    *,
    append: bool = True,
) -> int:
    """Write one canonical item per line. ``append=False`` truncates first."""
    target = Path(path)
    target.parent.mkdir(parents=True, exist_ok=True)
    mode = "a" if append else "w"
    written = 0
    with target.open(mode, encoding="utf-8") as handle:
        for item in items:
            handle.write(item.to_json() + "\n")
            written += 1
    return written


def read_items(path: str | Path) -> list[dict[str, Any]]:
    """Read a JSONL file written by :func:`append_items`."""
    target = Path(path)
    if not target.exists():
        return []
    items: list[dict[str, Any]] = []
    with target.open(encoding="utf-8") as handle:
        for line in handle:
            line = line.strip()
            if line:
                items.append(json.loads(line))
    return items


def write_items_json(
    items: list[dict[str, Any]],
    path: str | Path,
    metadata: dict[str, Any],
) -> Path:
    """Write the human-readable manifest atomically."""
    target = Path(path)
    target.parent.mkdir(parents=True, exist_ok=True)
    payload = {**metadata, "items": items}
    temporary = target.with_name(target.name + ".tmp")
    temporary.write_text(
        json.dumps(payload, indent=2, ensure_ascii=False) + "\n",
        encoding="utf-8",
    )
    os.replace(temporary, target)
    return target


def build_manifest(
    *,
    run_id: int,
    source_name: str,
    source_url: str,
    store_id: int | None,
    status: str,
    counters: dict[str, int],
    bundle: dict[str, Any] | None = None,
    error_message: str | None = None,
) -> dict[str, Any]:
    """Metadata shared by the runtime export and the replay export."""
    source: dict[str, Any] = {"name": source_name, "baseUrl": source_url}
    if store_id is not None:
        source["storeId"] = store_id
    manifest: dict[str, Any] = {
        "schemaVersion": SCHEMA_VERSION,
        "runId": run_id,
        "source": source,
        "status": status,
        "counters": counters,
    }
    if bundle is not None:
        manifest["bundle"] = bundle
    if error_message:
        manifest["errorMessage"] = error_message
    return manifest


# ---------------------------------------------------------------------------
# HTML crudo
# ---------------------------------------------------------------------------


def write_raw_page(
    raw_dir: str | Path,
    run_id: int,
    url: str,
    html: str,
    *,
    entity_type: str | None = None,
) -> dict[str, Any]:
    """Write one page as HTML and return its index entry.

    The file name is a hash of the URL: stable, filesystem-safe and independent
    of the store's path shape. The index keeps the readable mapping.
    """
    target_dir = raw_run_dir(raw_dir, run_id)
    target_dir.mkdir(parents=True, exist_ok=True)
    digest = hashlib.sha1(url.encode("utf-8")).hexdigest()[:20]
    filename = f"{digest}.html"
    target = target_dir / filename
    target.write_text(html, encoding="utf-8")
    return {
        "url": url,
        "file": filename,
        "bytes": len(html.encode("utf-8")),
        "entityType": entity_type,
        "scrapedAt": _now(),
    }


def append_raw_index(
    raw_dir: str | Path,
    run_id: int,
    entry: dict[str, Any],
    *,
    append: bool = True,
) -> None:
    target = raw_index_path(raw_dir, run_id)
    target.parent.mkdir(parents=True, exist_ok=True)
    mode = "a" if append else "w"
    with target.open(mode, encoding="utf-8") as handle:
        handle.write(json.dumps(entry, ensure_ascii=False) + "\n")


def read_raw_index(raw_dir: str | Path, run_id: int) -> list[dict[str, Any]]:
    target = raw_index_path(raw_dir, run_id)
    if not target.exists():
        return []
    entries: list[dict[str, Any]] = []
    with target.open(encoding="utf-8") as handle:
        for line in handle:
            line = line.strip()
            if line:
                entries.append(json.loads(line))
    return entries


# ---------------------------------------------------------------------------
# Replay desde SQLite
# ---------------------------------------------------------------------------


@dataclass(slots=True)
class RunContext:
    """The run's source and configuration, as stored when it happened."""

    run_id: int
    config_id: int
    status: str
    source_name: str
    base_url: str
    store_id: int | None
    parser_config: dict[str, Any] = field(default_factory=dict)

    @property
    def bundle(self) -> dict[str, Any] | None:
        raw = self.parser_config.get("bundle")
        return raw if isinstance(raw, dict) else None


@dataclass(slots=True)
class ReplayResult:
    items: list[NormalizedItem] = field(default_factory=list)
    rejected: list[NormalizedItem] = field(default_factory=list)
    pages: int = 0


def load_run_context(db_path: str, run_id: int) -> RunContext:
    """Read the run, its source and its ``parser_config`` from SQLite."""
    with closing(connect(db_path)) as conn:
        row = conn.execute(
            """
            SELECT r.id, r.scraper_config_id, r.status,
                   s.name, s.base_url, s.store_id, c.parser_config
            FROM SCRAPE_RUN r
            JOIN SCRAPER_CONFIG c ON c.id = r.scraper_config_id
            JOIN DATA_SOURCE s ON s.id = c.data_source_id
            WHERE r.id = ?
            """,
            (run_id, ),
        ).fetchone()
    if row is None:
        raise ValueError(f"run {run_id} does not exist in {db_path}")
    parser_config = _decode_json(row["parser_config"])
    return RunContext(
        run_id=int(row["id"]),
        config_id=int(row["scraper_config_id"]),
        status=str(row["status"]),
        source_name=str(row["name"]),
        base_url=str(row["base_url"]),
        store_id=row["store_id"],
        parser_config=parser_config,
    )


def replay_run(
    db_path: str,
    run_id: int,
    spec: BundleSpec,
    *,
    context: RunContext | None = None,
) -> ReplayResult:
    """Re-extract one run from ``SCRAPED_DATA.raw_data``.

    This is the only path that parses HTML again. It is intentionally explicit:
    the bundle may have changed since the run, so the caller must decide which
    spec to apply.
    """
    context = context or load_run_context(db_path, run_id)
    source = SourceInfo(
        name=context.source_name,
        base_url=context.base_url,
        store_id=context.store_id,
    )
    extractor = BundleExtractor(
        spec,
        source=source,
        run_id=run_id,
        config_id=context.config_id,
    )

    with closing(connect(db_path)) as conn:
        rows = conn.execute(
            "SELECT external_url, raw_data FROM SCRAPED_DATA"
            " WHERE scrape_run_id = ? ORDER BY id",
            (run_id, ),
        ).fetchall()

    from scrapling.parser import Adaptor  # import local: replay es opcional

    result = ReplayResult()
    seen: set[str] = set()
    for row in rows:
        html = row["raw_data"]
        if not html or html.strip() in {"", "{}"}:
            continue
        result.pages += 1
        page_url = row["external_url"]
        for item in extractor.extract_all(Adaptor(html), page_url):
            key = item.external_id or item.external_url
            if key in seen:
                continue
            seen.add(key)
            problems = item.validate()
            if problems:
                item.provenance.errors.extend(
                    f"validation.{problem.field}: {problem.message}"
                    for problem in problems
                )
                result.rejected.append(item)
            else:
                result.items.append(item)
    return result


def replay_manifest(result: ReplayResult,
                    context: RunContext) -> dict[str, Any]:
    counters = {
        "found": len(result.items) + len(result.rejected),
        "sent": 0,
        "failed": 0,
        "rejected": len(result.rejected),
    }
    return build_manifest(
        run_id=context.run_id,
        source_name=context.source_name,
        source_url=context.base_url,
        store_id=context.store_id,
        status="replay",
        counters=counters,
        bundle=context.bundle,
        error_message=None,
    )


def _decode_json(raw: str | None) -> dict[str, Any]:
    if not raw:
        return {}
    try:
        decoded = json.loads(raw)
    except json.JSONDecodeError:
        return {}
    return decoded if isinstance(decoded, dict) else {}
