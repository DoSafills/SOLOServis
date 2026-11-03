#!/usr/bin/env python3
"""Export or replay the artifacts of one crawler run (TAL-136).

Two modes, both offline (the crawler does not need to be running):

* **default** — read ``out/items/run-{id}.jsonl`` and (re)write the readable
  ``run-{id}.json``. No HTML is parsed: the JSONL already holds the normalized
  items that were extracted and validated.
* **``--from-db``** — explicit replay: read ``SCRAPED_DATA.raw_data`` for the
  run, apply a bundle and write an audit JSON. This *does* parse HTML again, so
  the bundle must be supplied (``--config``) unless the run already stored one
  in ``parser_config["bundle"]``.

Examples
--------
    python scripts/export_items.py --run-id 12
    python scripts/export_items.py --run-id 12 --items-dir /data/items
    python scripts/export_items.py --run-id 12 --from-db --config examples/falabella-cl.json
    python scripts/export_items.py --run-id 12 --from-db --out out/audit/run-12.json
"""

from __future__ import annotations

import argparse
import json
import sqlite3
import sys
from pathlib import Path

# ``src/`` uses plain absolute imports (see pyproject.toml), so the script has
# to put it on sys.path when run as ``python scripts/export_items.py``.
SRC = Path(__file__).resolve().parents[1] / "src"
if str(SRC) not in sys.path:
    sys.path.insert(0, str(SRC))

import artifacts  # noqa: E402
import config as cfg  # noqa: E402
from normalized import BundleSpec  # noqa: E402


def _parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Exporta o reproduce los artefactos de una ejecución.")
    parser.add_argument("--run-id", type=int, required=True,
                        help="id de SCRAPE_RUN / run-{id}")
    parser.add_argument("--items-dir", default=cfg.ITEMS_DIR,
                        help=f"carpeta de ítems (default: {cfg.ITEMS_DIR})")
    parser.add_argument("--out", default=None,
                        help="ruta del JSON de salida (default: items-dir/run-{id}.json)")
    parser.add_argument("--db", default=cfg.DB_PATH,
                        help=f"SQLite local (default: {cfg.DB_PATH})")
    parser.add_argument("--from-db", action="store_true",
                        help="replay explícito desde SCRAPED_DATA.raw_data")
    parser.add_argument("--config", default=None,
                        help="bundle JSON para --from-db (si el run no guardó uno)")
    parser.add_argument("--raw-dir", default=cfg.RAW_DIR,
                        help=f"carpeta de HTML crudo (default: {cfg.RAW_DIR})")
    return parser.parse_args(argv)


def _load_bundle(path: str) -> BundleSpec:
    with open(path, encoding="utf-8") as handle:
        return BundleSpec.from_parser_config(json.load(handle))


def _bundle_from_run(run_id: int, db_path: str) -> BundleSpec | None:
    context = artifacts.load_run_context(db_path, run_id)
    stored = context.bundle
    if not stored:
        return None
    return BundleSpec.from_parser_config(stored)


def export_from_jsonl(args: argparse.Namespace) -> Path:
    jsonl = artifacts.items_jsonl_path(args.items_dir, args.run_id)
    if not jsonl.exists():
        raise SystemExit(
            f"no existe {jsonl}; usa --from-db para reconstruir desde SQLite")
    items = artifacts.read_items(jsonl)
    try:
        context = artifacts.load_run_context(args.db, args.run_id)
        metadata = artifacts.build_manifest(
            run_id=context.run_id,
            source_name=context.source_name,
            source_url=context.base_url,
            store_id=context.store_id,
            status=context.status,
            counters={
                "found": len(items),
                "exported": len(items),
                "sent": 0,
                "failed": 0,
                "rejected": 0,
            },
            bundle=context.bundle,
        )
    except (ValueError, OSError, sqlite3.Error):
        # El JSONL puede copiarse solo (sin la base). Cada ítem ya lleva su
        # `source`, así que el manifiesto se reconstruye desde ahí.
        source = items[0].get("source", {}) if items else {}
        metadata = artifacts.build_manifest(
            run_id=args.run_id,
            source_name=str(source.get("name") or ""),
            source_url=str(source.get("baseUrl") or ""),
            store_id=source.get("storeId"),
            status="unknown",
            counters={"found": len(items), "exported": len(items)},
        )
    target = Path(args.out) if args.out else artifacts.items_json_path(
        args.items_dir, args.run_id)
    artifacts.write_items_json(items, target, metadata)
    return target


def export_from_db(args: argparse.Namespace) -> Path:
    spec = _load_bundle(args.config) if args.config else _bundle_from_run(
        args.run_id, args.db)
    if spec is None:
        raise SystemExit(
            "--from-db necesita un bundle: pasa --config o usa un run cuyo "
            "parser_config['bundle'] exista")
    context = artifacts.load_run_context(args.db, args.run_id)
    result = artifacts.replay_run(args.db, args.run_id, spec, context=context)
    manifest = artifacts.replay_manifest(result, context)
    target = Path(args.out) if args.out else (
        Path(args.items_dir) / f"run-{args.run_id}.replay.json")
    artifacts.write_items_json(
        [item.to_dict() for item in result.items], target, manifest)
    if result.rejected:
        artifacts.write_items_json(
            [item.to_dict() for item in result.rejected],
            Path(cfg.REJECTS_DIR) / f"run-{args.run_id}.replay.json",
            {**manifest, "status": "replay-rejected"})
    return target


def main(argv: list[str] | None = None) -> int:
    args = _parse_args(argv)
    target = export_from_db(args) if args.from_db else export_from_jsonl(args)
    print(target)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
