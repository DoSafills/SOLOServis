"""Command line entry point.

``scraper run`` executes one crawl in the foreground (useful for debugging a
spec); ``scraper serve`` runs the control API that Kubernetes uses; and
``scraper export`` rebuilds the catalog from the database without touching the
network.
"""

from __future__ import annotations

import argparse
import asyncio
import json
import logging
import os
import sys
from pathlib import Path

from scraper.config import Settings, load_settings
from scraper.db import repository as repo
from scraper.errors import ScraperError, SpecError
from scraper.models import (
    RUN_STATUS_SUCCESS,
    DataSource,
    ScraperConfig,
    StoreSpec,
)
from scraper.output.catalog import write_catalog
from scraper.pipeline import execute
from scraper.spec.loader import find_spec, load_specs

_log = logging.getLogger("scraper")


def _settings(args: argparse.Namespace) -> Settings:
    env = dict(os.environ)
    if getattr(args, "data_dir", None):
        env["SCRAPER_DATA_DIR"] = str(Path(args.data_dir).expanduser())
        for key in (
            "SCRAPER_DB_PATH",
            "SCRAPER_CATALOG_DIR",
            "SCRAPER_RUNS_DIR",
            "SCRAPER_REJECTS_DIR",
        ):
            env.pop(key, None)
    return load_settings(env)


def cmd_validate(args: argparse.Namespace) -> int:
    settings = _settings(args)
    specs = load_specs(settings.stores_dir)
    for spec in specs:
        owner = find_spec(specs, spec.listing.url)
        if owner.slug != spec.slug:
            raise SpecError(
                str(spec.path),
                f"listing URL is claimed by {owner.slug!r}, not {spec.slug!r}",
            )
        print(
            f"{spec.slug:20} store={spec.store_id:<3} fields={len(spec.fields):<2} "
            f"digest={spec.digest[:12]}"
        )
    print(f"{len(specs)} spec(s) valid")
    return 0


def cmd_init_db(args: argparse.Namespace) -> int:
    settings = _settings(args)
    settings.ensure_dirs()
    repo.init_db(settings.db_path)
    print(f"database ready: {settings.db_path}")
    return 0


def cmd_stores(args: argparse.Namespace) -> int:
    settings = _settings(args)
    for spec in load_specs(settings.stores_dir):
        print(f"{spec.slug:20} {spec.name:24} storeId={spec.store_id} {spec.listing.url}")
    return 0


def cmd_run(args: argparse.Namespace) -> int:
    settings = _settings(args)
    settings.ensure_dirs()
    repo.init_db(settings.db_path)
    repo.recover_orphan_runs(settings.db_path)
    specs = load_specs(settings.stores_dir)
    spec = _resolve_spec(specs, args.store, args.url)
    source, config = _ensure_pair(settings, spec)
    run_id = repo.create_run(settings.db_path, config.id)
    result = asyncio.run(
        execute(settings, spec=spec, source=source, config=config, run_id=run_id)
    )
    print(
        json.dumps(
            {
                "runId": result.run_id,
                "status": result.status,
                "store": spec.slug,
                "counters": result.counters.to_dict(),
                "catalog": result.catalog.to_dict() if result.catalog else None,
                "rejects": result.rejects,
                "error": result.error,
                "ingestionRunId": result.ingestion_run_id,
                "ingestionError": result.ingestion_error,
            },
            indent=2,
            ensure_ascii=False,
        )
    )
    return 0 if result.status == RUN_STATUS_SUCCESS else 1


def cmd_serve(args: argparse.Namespace) -> int:
    import uvicorn

    from scraper.api.app import create_app

    settings = _settings(args)
    settings.ensure_dirs()
    if not settings.api_token:
        _log.warning("SCRAPER_API_TOKEN is empty: /runs will answer 503 until it is set")
    app = create_app(settings)
    uvicorn.run(app, host=settings.api_host, port=settings.api_port, log_level="info")
    return 0


def cmd_export(args: argparse.Namespace) -> int:
    settings = _settings(args)
    settings.ensure_dirs()
    repo.init_db(settings.db_path)
    specs = load_specs(settings.stores_dir)
    selected = [spec for spec in specs if not args.store or spec.slug == args.store]
    if not selected:
        raise SpecError(args.store or "", "unknown store")
    exported = 0
    for spec in selected:
        pair = repo.config_by_base_url(settings.db_path, spec.base_url)
        if pair is None:
            print(f"{spec.slug}: not registered yet; run it once first", file=sys.stderr)
            continue
        _, config = pair
        run_id = repo.latest_run_for_config(settings.db_path, config.id)
        if run_id is None:
            print(f"{spec.slug}: no finished run to export", file=sys.stderr)
            continue
        items = repo.run_items(settings.db_path, run_id)
        previous_run = repo.previous_run(settings.db_path, run_id)
        previous = (
            repo.items_by_external_id(repo.run_items(settings.db_path, previous_run))
            if previous_run is not None
            else {}
        )
        result = write_catalog(
            settings.catalog_dir,
            store=spec.slug,
            run_id=run_id,
            items=items,
            previous=previous,
            complete=True,
        )
        exported += 1
        print(f"{spec.slug}: run #{run_id} -> {result.to_dict()}")
    print(f"{exported} store(s) exported to {settings.catalog_dir}")
    return 0


def _resolve_spec(specs: list[StoreSpec], store: str | None, url: str | None) -> StoreSpec:
    if url:
        return find_spec(specs, url)
    if store:
        for spec in specs:
            if spec.slug == store:
                return spec
        known = ", ".join(sorted(spec.slug for spec in specs))
        raise SpecError(store, f"unknown store; known stores: {known}")
    raise SpecError("run", "pass --store <slug> or --url <listing URL>")


def _ensure_pair(settings: Settings, spec: StoreSpec) -> tuple[DataSource, ScraperConfig]:
    repo.ensure_source(
        settings.db_path,
        name=spec.name,
        base_url=spec.base_url,
        store_id=spec.store_id,
        rate_limit=settings.rate_limit,
        spec_slug=spec.slug,
        spec_version=spec.schema_version,
        spec_hash=spec.digest,
    )
    pair = repo.config_by_base_url(settings.db_path, spec.base_url)
    assert pair is not None
    return pair


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="scraper", description=__doc__)
    parser.add_argument("--data-dir", help="override SCRAPER_DATA_DIR")
    parser.add_argument("-v", "--verbose", action="store_true")
    sub = parser.add_subparsers(dest="command", required=True)

    validate = sub.add_parser("validate", help="validate every store spec")
    validate.set_defaults(func=cmd_validate)

    init = sub.add_parser("init-db", help="create the SQLite schema")
    init.set_defaults(func=cmd_init_db)

    stores = sub.add_parser("stores", help="list store specs")
    stores.set_defaults(func=cmd_stores)

    run = sub.add_parser("run", help="run one crawl in the foreground")
    run.add_argument("--store", help="store slug")
    run.add_argument("--url", help="URL claimed by a store spec")
    run.set_defaults(func=cmd_run)

    serve = sub.add_parser("serve", help="start the control API")
    serve.set_defaults(func=cmd_serve)

    export = sub.add_parser("export", help="rebuild the catalog from the database")
    export.add_argument("--store", help="only this store slug")
    export.set_defaults(func=cmd_export)
    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    logging.basicConfig(
        level=logging.DEBUG if args.verbose else logging.INFO,
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
    )
    try:
        return int(args.func(args))
    except ScraperError as exc:
        _log.error("%s", exc)
        return 2
    except KeyboardInterrupt:
        return 130


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
