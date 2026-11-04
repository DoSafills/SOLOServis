"""Run orchestration for the control API.

A single worker consumes a FIFO queue, so two runs never execute at once even
if the scheduler fires twice. Every request creates a ``SCRAPE_RUN`` row
immediately (``pending``), which gives the caller an id before the crawl
starts. A stop request flips an :class:`asyncio.Event`; the pipeline checks it
between pages and drains the page in flight instead of being killed.
"""

from __future__ import annotations

import asyncio
import contextlib
from dataclasses import dataclass, field
from typing import Any

from scraper.config import Settings
from scraper.db import repository as repo
from scraper.errors import (
    ConfigError,
    RunConflict,
    RunNotFound,
    SpecError,
)
from scraper.models import (
    RUN_STATUS_FAILED,
    RUN_STATUS_PENDING,
    RUN_STATUS_SUCCESS,
    Counters,
    DataSource,
    RunRecord,
    RunState,
    ScraperConfig,
    StoreSpec,
)
from scraper.pipeline import PipelineResult, execute
from scraper.spec.loader import find_spec, load_specs

TERMINAL_STATUSES = frozenset({RUN_STATUS_SUCCESS, RUN_STATUS_FAILED})


@dataclass(slots=True)
class _Job:
    run_id: int
    spec: StoreSpec
    source: DataSource
    config: ScraperConfig
    stop_event: asyncio.Event = field(default_factory=asyncio.Event)
    done: asyncio.Event = field(default_factory=asyncio.Event)
    cancelled: bool = False
    abandoned: bool = False
    result: PipelineResult | None = None


class RunManager:
    """Owns the queue, the active job and the database rows behind them."""

    def __init__(self, settings: Settings, specs: list[StoreSpec] | None = None) -> None:
        self.settings = settings
        self.specs = specs if specs is not None else load_specs(settings.stores_dir)
        self._queue: asyncio.Queue[_Job] = asyncio.Queue()
        self._jobs: dict[int, _Job] = {}
        self._idempotent: dict[str, int] = {}
        self._active: _Job | None = None
        self._lock = asyncio.Lock()
        self._worker: asyncio.Task[None] | None = None

    # -- lifecycle -----------------------------------------------------------

    async def start(self) -> None:
        await asyncio.to_thread(repo.init_db, self.settings.db_path)
        await asyncio.to_thread(repo.recover_orphan_runs, self.settings.db_path)
        self._worker = asyncio.create_task(self._consume(), name="scraper-worker")

    async def shutdown(self) -> None:
        if self._active is not None:
            self._active.stop_event.set()
            with contextlib.suppress(TimeoutError):
                await asyncio.wait_for(
                    self._active.done.wait(), timeout=self.settings.stop_timeout
                )
        if self._worker is not None:
            self._worker.cancel()
            with contextlib.suppress(asyncio.CancelledError):
                await self._worker
            self._worker = None

    # -- operations ----------------------------------------------------------

    async def start_run(
        self,
        *,
        store: str | None = None,
        url: str | None = None,
        idempotency_key: str | None = None,
    ) -> RunState:
        spec = self._resolve(store, url)
        async with self._lock:
            if idempotency_key and idempotency_key in self._idempotent:
                return await self.status(self._idempotent[idempotency_key])

            source_id, config_id, _ = await asyncio.to_thread(
                repo.ensure_source,
                self.settings.db_path,
                name=spec.name,
                base_url=spec.base_url,
                store_id=spec.store_id,
                rate_limit=self.settings.rate_limit,
                spec_slug=spec.slug,
                spec_version=spec.schema_version,
                spec_hash=spec.digest,
            )
            open_run = await asyncio.to_thread(
                repo.open_run_for_config, self.settings.db_path, config_id
            )
            if open_run is not None:
                raise RunConflict(
                    f"store {spec.slug!r} already has an open run (#{open_run}); "
                    "wait for it to finish or stop it first"
                )

            run_id = await asyncio.to_thread(
                repo.create_run, self.settings.db_path, config_id, status=RUN_STATUS_PENDING
            )
            pair = await asyncio.to_thread(
                repo.config_for_run, self.settings.db_path, run_id
            )
            if pair is None:  # pragma: no cover - just inserted
                raise ConfigError("run row disappeared immediately after insert")
            source, config = pair
            job = _Job(run_id=run_id, spec=spec, source=source, config=config)
            self._jobs[run_id] = job
            if idempotency_key:
                self._idempotent[idempotency_key] = run_id
            await self._queue.put(job)
            return await self.status(run_id)

    async def status(self, run_id: int) -> RunState:
        record = await asyncio.to_thread(repo.get_run, self.settings.db_path, run_id)
        if record is None:
            raise RunNotFound(f"run #{run_id} does not exist")
        duration = await asyncio.to_thread(
            repo.run_duration_ms, self.settings.db_path, run_id
        )
        job = self._jobs.get(run_id)
        status = record.status
        if (
            job is not None
            and self._active is job
            and job.stop_event.is_set()
            and status not in TERMINAL_STATUSES
        ):
            status = "stopping"
        return await self._state(record, status, duration)

    async def list_runs(self, limit: int = 50) -> list[RunState]:
        records = await asyncio.to_thread(
            repo.list_runs, self.settings.db_path, limit=limit
        )
        states: list[RunState] = []
        for record in records:
            duration = await asyncio.to_thread(
                repo.run_duration_ms, self.settings.db_path, record.id
            )
            states.append(await self._state(record, record.status, duration))
        return states

    async def stop(self, run_id: int) -> RunState:
        job = self._jobs.get(run_id)
        record = await asyncio.to_thread(repo.get_run, self.settings.db_path, run_id)
        if record is None:
            raise RunNotFound(f"run #{run_id} does not exist")
        if record.status in TERMINAL_STATUSES:
            return await self.status(run_id)

        if job is None:
            await asyncio.to_thread(
                repo.finish_run,
                self.settings.db_path,
                run_id,
                RUN_STATUS_FAILED,
                error_message="stopped: run is not owned by this process",
            )
            return await self.status(run_id)

        if self._active is not job:
            job.cancelled = True
            await asyncio.to_thread(
                repo.finish_run,
                self.settings.db_path,
                run_id,
                RUN_STATUS_FAILED,
                error_message="cancelled before start",
            )
            job.done.set()
            return await self.status(run_id)

        job.stop_event.set()
        try:
            await asyncio.wait_for(job.done.wait(), timeout=self.settings.stop_timeout)
        except TimeoutError:
            job.abandoned = True
            await asyncio.to_thread(
                repo.finish_run,
                self.settings.db_path,
                run_id,
                RUN_STATUS_FAILED,
                error_message="stopped: grace period expired",
            )
        return await self.status(run_id)

    # -- internals -----------------------------------------------------------

    def _resolve(self, store: str | None, url: str | None) -> StoreSpec:
        if store:
            for spec in self.specs:
                if spec.slug == store:
                    return spec
            known = ", ".join(sorted(spec.slug for spec in self.specs))
            raise SpecError(store, f"unknown store; known stores: {known}")
        if url:
            return find_spec(self.specs, url)
        raise SpecError(
            "request",
            "'store' or 'url' is required; the API never crawls an unclaimed URL",
        )

    async def _state(
        self, record: RunRecord, status: str, duration: int | None
    ) -> RunState:
        pair = await asyncio.to_thread(
            repo.config_for_run, self.settings.db_path, record.id
        )
        store = ""
        source_url = ""
        if pair is not None:
            source, config = pair
            store = str(config.parser_config.get("spec") or source.name)
            source_url = source.base_url
        errors = [message for message in (record.error_message, record.ingestion_error) if message]
        return RunState(
            run_id=record.id,
            status=status,
            store=store,
            source_url=source_url,
            started_at=record.started_at,
            finished_at=record.finished_at,
            duration_ms=duration,
            counters={
                "pagesFound": record.records_found,
                "pagesProcessed": record.records_processed,
                "pagesFailed": record.records_failed,
                "itemsFound": record.items_found,
                "itemsSent": record.items_sent,
                "itemsFailed": record.items_failed,
                "itemsRejected": record.items_rejected,
            },
            errors=errors,
        )

    async def _consume(self) -> None:
        while True:
            job = await self._queue.get()
            try:
                if job.cancelled:
                    continue
                self._active = job
                await asyncio.to_thread(
                    repo.mark_run_running, self.settings.db_path, job.run_id
                )
                try:
                    result = await execute(
                        self.settings,
                        spec=job.spec,
                        source=job.source,
                        config=job.config,
                        run_id=job.run_id,
                        stop_event=job.stop_event,
                    )
                except asyncio.CancelledError:
                    await asyncio.to_thread(
                        repo.finish_run,
                        self.settings.db_path,
                        job.run_id,
                        RUN_STATUS_FAILED,
                        error_message="aborted: worker cancelled",
                    )
                    raise
                except Exception as exc:  # noqa: BLE001 - reported, never swallowed
                    await asyncio.to_thread(
                        repo.finish_run,
                        self.settings.db_path,
                        job.run_id,
                        RUN_STATUS_FAILED,
                        error_message=f"unhandled error: {type(exc).__name__}: {exc}",
                    )
                    result = PipelineResult(
                        run_id=job.run_id,
                        status=RUN_STATUS_FAILED,
                        counters=Counters(),
                        error=f"{type(exc).__name__}: {exc}",
                    )
                if job.abandoned and result.status == RUN_STATUS_SUCCESS:
                    await asyncio.to_thread(
                        repo.finish_run,
                        self.settings.db_path,
                        job.run_id,
                        RUN_STATUS_FAILED,
                        error_message="stopped: grace period expired",
                    )
                    result.status = RUN_STATUS_FAILED
                job.result = result
            finally:
                self._active = None
                job.done.set()
                self._queue.task_done()

    def store_catalog(self) -> list[dict[str, Any]]:
        return [
            {
                "slug": spec.slug,
                "name": spec.name,
                "storeId": spec.store_id,
                "baseUrl": spec.base_url,
                "listingUrl": spec.listing.url,
                "digest": spec.digest,
            }
            for spec in sorted(self.specs, key=lambda item: item.slug)
        ]
