"""Raw artifacts: page HTML, rejected cards and the run snapshot.

These files are diagnostics, not the catalog. They live under ``out/`` so a
failed selector can be debugged without re-fetching the store.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from scraper.output.catalog import _atomic_write, render


def page_filename(index: int, url: str) -> str:
    stem = url.rstrip("/").rsplit("/", 1)[-1] or "page"
    safe = "".join(char if char.isalnum() or char in "-_." else "-" for char in stem)[:60]
    return f"{index:03d}-{safe or 'page'}.html"


def save_page_html(run_dir: str | Path, index: int, url: str, html: str) -> Path:
    directory = Path(run_dir) / "pages"
    directory.mkdir(parents=True, exist_ok=True)
    path = directory / page_filename(index, url)
    _atomic_write(path, html)
    return path


def save_run_snapshot(run_dir: str | Path, payload: dict[str, Any]) -> Path:
    directory = Path(run_dir)
    directory.mkdir(parents=True, exist_ok=True)
    path = directory / "run.json"
    _atomic_write(path, render(payload))
    return path


def save_rejects(
    rejects_dir: str | Path, run_id: int, rejects: list[dict[str, Any]]
) -> Path | None:
    if not rejects:
        return None
    directory = Path(rejects_dir)
    directory.mkdir(parents=True, exist_ok=True)
    path = directory / f"run-{run_id}.jsonl"
    with path.open("w", encoding="utf-8") as handle:
        for reject in rejects:
            handle.write(json.dumps(reject, ensure_ascii=False, sort_keys=True) + "\n")
    return path
