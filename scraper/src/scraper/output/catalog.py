"""Per-store catalog output.

The database is the source of truth. This module projects one run onto a
directory of stable, one-file-per-product JSON documents so a human (or a
diff tool) can see exactly what changed between runs:

    <catalog>/<store>/<product>.json
    <catalog>/<store>/_index.json
    <catalog>/<store>/_changes.jsonl

Files are written atomically, and a product whose canonical JSON did not
change is not rewritten: its mtime stays put, which is what makes change
tracking cheap.
"""

from __future__ import annotations

import hashlib
import json
import os
import tempfile
from dataclasses import dataclass, field
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

from scraper.extract.transforms import slugify

INDEX_FILE = "_index.json"
CHANGES_FILE = "_changes.jsonl"


@dataclass(slots=True)
class CatalogResult:
    added: int = 0
    updated: int = 0
    unchanged: int = 0
    removed: int = 0
    complete: bool = True
    files: dict[str, str] = field(default_factory=dict)

    def to_dict(self) -> dict[str, Any]:
        return {
            "added": self.added,
            "updated": self.updated,
            "unchanged": self.unchanged,
            "removed": self.removed,
            "complete": self.complete,
            "files": len(self.files),
        }


def item_key(item: dict[str, Any]) -> str:
    return str(item.get("externalId") or item.get("externalUrl") or "")


def product_filename(item: dict[str, Any]) -> str:
    """A stable, collision-free filename derived from the item key."""
    key = item_key(item)
    digest = hashlib.sha256(key.encode("utf-8")).hexdigest()[:10]
    stem = slugify(key)[:80] or "item"
    return f"{stem}-{digest}.json"


def comparable(item: dict[str, Any]) -> dict[str, Any]:
    """The part of an item that describes the product, not the crawl.

    ``provenance`` changes on every run (timestamp, run id). Comparing it would
    rewrite every file every time, which defeats change tracking, so it is
    excluded from the comparison while still being written on real changes.
    """
    return {key: value for key, value in item.items() if key != "provenance"}


def canonical(item: dict[str, Any]) -> str:
    """Compact canonical form, used to decide whether a file changed."""
    return json.dumps(
        comparable(item), ensure_ascii=False, sort_keys=True, separators=(",", ":")
    )


def render(item: dict[str, Any]) -> str:
    """The on-disk form: sorted keys and indentation for readable diffs."""
    return json.dumps(item, ensure_ascii=False, sort_keys=True, indent=2) + "\n"


def write_catalog(
    root: str | Path,
    *,
    store: str,
    run_id: int,
    items: list[dict[str, Any]],
    previous: dict[str, dict[str, Any]] | None = None,
    complete: bool = True,
    generated_at: str | None = None,
) -> CatalogResult:
    """Write one run's snapshot for ``store`` and return what changed.

    ``complete`` controls deletion: a failed or stopped run must never remove
    a product from the catalog, because the absence may simply mean the crawl
    did not reach that page.
    """
    directory = Path(root) / store
    directory.mkdir(parents=True, exist_ok=True)
    stamp = generated_at or _now_iso()
    previous = previous or {}

    current: dict[str, dict[str, Any]] = {}
    for item in items:
        key = item_key(item)
        if key:
            current[key] = item

    result = CatalogResult(complete=complete)
    changes: list[dict[str, Any]] = []

    for key, item in current.items():
        filename = product_filename(item)
        result.files[key] = filename
        path = directory / filename
        content = render(item)
        before = previous.get(key)
        if before is None:
            _atomic_write(path, content)
            result.added += 1
            changes.append(_change(stamp, run_id, "added", key, filename))
        elif canonical(before) != canonical(item):
            _atomic_write(path, content)
            result.updated += 1
            changes.append(_change(stamp, run_id, "updated", key, filename))
        else:
            result.unchanged += 1

    if complete:
        for key, item in previous.items():
            if key in current:
                continue
            filename = product_filename(item)
            path = directory / filename
            if path.exists():
                path.unlink()
            result.removed += 1
            changes.append(_change(stamp, run_id, "removed", key, filename))

    _write_index(
        directory, store=store, run_id=run_id, stamp=stamp, items=current, complete=complete
    )
    _append_changes(directory, changes)
    return result


def _write_index(
    directory: Path,
    *,
    store: str,
    run_id: int,
    stamp: str,
    items: dict[str, dict[str, Any]],
    complete: bool = True,
) -> None:
    entries: dict[str, dict[str, Any]] = {}
    for key, item in sorted(items.items()):
        product = item.get("product") or {}
        offer = item.get("offer") or {}
        entries[product_filename(item)] = {
            "externalId": item.get("externalId"),
            "externalUrl": item.get("externalUrl"),
            "name": product.get("name"),
            "price": offer.get("price"),
            "currency": offer.get("currency"),
            "key": key,
        }
    payload = {
        "store": store,
        "runId": run_id,
        "generatedAt": stamp,
        "complete": complete,
        "count": len(entries),
        "items": entries,
    }
    _atomic_write(directory / INDEX_FILE, render(payload))


def _append_changes(directory: Path, changes: list[dict[str, Any]]) -> None:
    if not changes:
        return
    with (directory / CHANGES_FILE).open("a", encoding="utf-8") as handle:
        for change in changes:
            handle.write(json.dumps(change, ensure_ascii=False, sort_keys=True) + "\n")


def _change(stamp: str, run_id: int, kind: str, key: str, filename: str) -> dict[str, Any]:
    return {
        "at": stamp,
        "runId": run_id,
        "type": kind,
        "key": key,
        "file": filename,
    }


def _atomic_write(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    handle, temp = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        with os.fdopen(handle, "w", encoding="utf-8") as stream:
            stream.write(content)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temp, path)
    except BaseException:
        Path(temp).unlink(missing_ok=True)
        raise


def _now_iso() -> str:
    return datetime.now(UTC).isoformat(timespec="seconds").replace("+00:00", "Z")
