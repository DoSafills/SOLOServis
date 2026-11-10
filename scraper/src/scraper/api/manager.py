"""Task-aware run orchestration for the control API.

The manager owns three things:

* a bounded pool of runs (``SCRAPER_MAX_PARALLEL_TASKS``), with a per-store
  guard so two runs never write the same catalog at once;
* a recurring scheduler that reads task JSON files from the PVC;
* the global kill switch (``paused``) that stops every active run and keeps
  the scheduler from starting new ones.

Every request still creates a ``SCRAPE_RUN`` row immediately (``pending``), so
the caller gets an id before the crawl starts. A stop request flips an
:class:`asyncio.Event`; the pipeline checks it between pages and drains the page
in flight instead of being killed.
"""

from __future__ import annotations

import asyncio
import contextlib
import logging
from collections import deque
from dataclasses import dataclass, field
from datetime import timedelta
from typing import Any

from scraper.config import Settings
from scraper.db import repository as repo
from scraper.errors import (
    ConfigError,
    RunConflict,
    RunNotFound,
    SpecError,
    TaskError,
    TaskNotFound,
)
from scraper.ingestion import IngestionClient
from scraper.models import (
    RUN_STATUS_FAILED,
    RUN_STATUS_PENDING,
    RUN_STATUS_RUNNING,
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
from scraper.tasks import (
    SCHEDULE_INTERVAL,
    ScrapeTask,
    TaskSchedule,
    TaskStore,
    iso,
    make_task,
    new_task_id,
    now_iso,
    now_utc,
    parse_iso,
)

_log = logging.getLogger("scraper.manager")

TERMINAL_STATUSES = frozenset({RUN_STATUS_SUCCESS, RUN_STATUS_FAILED})
RETRY_CONFLICT_SECONDS = 60


@dataclass(slots=True)
class _Job:
    run_id: int
    spec: StoreSpec
    source: DataSource
    config: ScraperConfig
    task_id: str | None = None
    trigger: str = "api"
    stop_event: asyncio.Event = field(default_factory=asyncio.Event)
    done: asyncio.Event = field(default_factory=asyncio.Event)
    started: bool = False
    cancelled: bool = False
    abandoned: bool = False
    finalized: bool = False
    result: PipelineResult | None = None


class RunManager:
    """Owns the run pool, the recurring scheduler and the task files."""

    def __init__(self, settings: Settings, specs: list[StoreSpec] | None = None) -> None:
        self.settings = settings
        self.specs = specs if specs is not None else load_specs(settings.stores_dir)
        self._jobs: dict[int, _Job] = {}
        self._job_tasks: dict[int, asyncio.Task[None]] = {}
        self._active: dict[int, _Job] = {}
        self._idempotent: dict[str, int] = {}
        self._lock = asyncio.Lock()
        self._tick_lock = asyncio.Lock()
        self._semaphore = asyncio.Semaphore(settings.max_parallel_tasks)
        self._tasks = TaskStore(settings.tasks_dir)
        self._scheduler: asyncio.Task[None] | None = None
        self._started = False
        self._completed: deque[int] = deque(
            maxlen=max(50, settings.task_history_limit)
        )

    # -- lifecycle -----------------------------------------------------------

    async def start(self) -> None:
        if self._started:
            return
        await asyncio.to_thread(repo.init_db, self.settings.db_path)
        await asyncio.to_thread(repo.recover_orphan_runs, self.settings.db_path)
        self._reconcile_tasks()  # validate files and close runs lost to a restart
        if self.settings.scheduler_enabled:
            self._scheduler = asyncio.create_task(self._schedule_loop(), name="scraper-scheduler")
        self._started = True

    def _reconcile_tasks(self) -> None:
        """A task that was running when the pod died is no longer running.

        The database rows were already closed by ``recover_orphan_runs``; this
        keeps the task file honest and schedules its next interval.
        """
        for task in self._tasks.list_all():
            if task.last_status not in {RUN_STATUS_PENDING, RUN_STATUS_RUNNING}:
                continue
            task.last_status = RUN_STATUS_FAILED
            task.consecutive_failures += 1
            if task.schedule.kind == SCHEDULE_INTERVAL:
                next_at = task.schedule.next_after(now_utc())
                task.next_run_at = iso(next_at) if next_at is not None else None
            self._tasks.save(task)

    async def shutdown(self) -> None:
        if self._scheduler is not None:
            self._scheduler.cancel()
            with contextlib.suppress(asyncio.CancelledError):
                await self._scheduler
            self._scheduler = None

        jobs = [job for job in self._jobs.values() if not job.done.is_set()]
        for job in jobs:
            if job.started:
                job.stop_event.set()
            else:
                job.cancelled = True
                await self._finalize_cancelled(job)
        if jobs:
            with contextlib.suppress(TimeoutError):
                await asyncio.wait_for(
                    asyncio.gather(*(job.done.wait() for job in jobs)),
                    timeout=self.settings.stop_timeout,
                )

        pending = list(self._job_tasks.values())
        for task in pending:
            task.cancel()
        if pending:
            await asyncio.gather(*pending, return_exceptions=True)
        self._job_tasks.clear()
        self._started = False

    # -- runs ----------------------------------------------------------------

    async def start_run(
        self,
        *,
        store: str | None = None,
        url: str | None = None,
        idempotency_key: str | None = None,
    ) -> RunState:
        spec = self._resolve(store, url)
        async with self._tick_lock:
            if self._tasks.paused:
                raise RunConflict(
                    "the scheduler is paused; POST /tasks/start before starting a run"
                )
            async with self._lock:
                if idempotency_key and idempotency_key in self._idempotent:
                    return await self.status(self._idempotent[idempotency_key])
                job = await self._create_job_locked(spec, task_id=None, trigger="api")
                if idempotency_key:
                    self._idempotent[idempotency_key] = job.run_id
        return await self.status(job.run_id)

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
            and job.stop_event.is_set()
            and status not in TERMINAL_STATUSES
        ):
            status = "stopping"
        state = await self._state(record, status, duration)
        if job is not None:
            state.task_id = job.task_id
            state.trigger = job.trigger
        return state

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

        if not job.started:
            job.cancelled = True
            await self._finalize_cancelled(job)
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

    # -- tasks ---------------------------------------------------------------

    async def list_tasks(self) -> list[dict[str, Any]]:
        return [self._task_state(task) for task in self._tasks.list_all()]

    async def get_task(self, task_id: str) -> dict[str, Any]:
        return self._task_state(self._tasks.get(task_id))

    async def create_task(
        self,
        *,
        store: str,
        name: str | None = None,
        schedule: TaskSchedule,
        max_runs: int | None = None,
        enabled: bool = True,
        run_immediately: bool = False,
    ) -> dict[str, Any]:
        spec = self._resolve(store, None)
        if max_runs is not None and max_runs < 1:
            raise TaskError("maxRuns must be >= 1")
        existing = {task.id for task in self._tasks.list_all()}
        task = make_task(
            task_id=new_task_id(existing),
            store=spec.slug,
            name=name or spec.name,
            schedule=schedule,
            max_runs=max_runs,
            enabled=enabled,
            next_run_at=(
                now_iso()
                if enabled and run_immediately
                else self._next_run_at(schedule, enabled=enabled)
            ),
        )
        self._tasks.save(task)
        if run_immediately and enabled:
            async with self._tick_lock:
                if not self._tasks.paused:
                    with contextlib.suppress(RunConflict):
                        await self._launch_task(task, trigger="create")
        return self._task_state(self._tasks.get(task.id))

    async def update_task(
        self,
        task_id: str,
        *,
        name: str | None = None,
        enabled: bool | None = None,
        schedule: TaskSchedule | None = None,
        max_runs: int | None = None,
        set_max_runs: bool = False,
    ) -> dict[str, Any]:
        task = self._tasks.get(task_id)
        if name is not None:
            task.name = name
        if max_runs is not None and max_runs < 1:
            raise TaskError("maxRuns must be >= 1")
        if set_max_runs:
            task.max_runs = max_runs
        if schedule is not None and schedule != task.schedule:
            task.schedule = schedule
            task.next_run_at = self._next_run_at(schedule, enabled=task.enabled)
        if enabled is not None and enabled != task.enabled:
            task.enabled = enabled
            task.next_run_at = self._next_run_at(task.schedule, enabled=enabled)
        self._tasks.save(task)
        if task.enabled and not self._tasks.paused:
            await self._tick()
        return self._task_state(self._tasks.get(task.id))

    async def delete_task(self, task_id: str) -> None:
        async with self._tick_lock:
            task = self._tasks.get(task_id)
            job = self._task_job(task.id)
            if job is not None:
                await self.stop(job.run_id)
            self._tasks.delete(task.id)

    async def run_task(self, task_id: str) -> RunState:
        task = self._tasks.get(task_id)
        async with self._tick_lock:
            if self._tasks.paused:
                raise RunConflict(
                    "the scheduler is paused; POST /tasks/start before running a task"
                )
            if task.max_runs is not None and task.run_count >= task.max_runs:
                raise RunConflict(
                    f"task {task.id!r} already reached maxRuns={task.max_runs}"
                )
            job = await self._launch_task(task, trigger="manual")
        return await self.status(job.run_id)

    async def stop_task(self, task_id: str) -> dict[str, Any]:
        task = self._tasks.get(task_id)
        job = self._task_job(task.id)
        if job is not None:
            await self.stop(job.run_id)
        return self._task_state(self._tasks.get(task.id))

    async def start_all(self) -> dict[str, Any]:
        async with self._tick_lock:
            self._tasks.set_paused(False)
        await self._tick()
        return self._global_state()

    async def stop_all(self) -> dict[str, Any]:
        async with self._tick_lock:
            self._tasks.set_paused(True)
            jobs = [job for job in self._jobs.values() if not job.done.is_set()]
            for job in jobs:
                if job.started:
                    job.stop_event.set()
                else:
                    job.cancelled = True
                    await self._finalize_cancelled(job)
            if jobs:
                with contextlib.suppress(TimeoutError):
                    await asyncio.wait_for(
                        asyncio.gather(*(job.done.wait() for job in jobs)),
                        timeout=self.settings.stop_timeout,
                    )
        return self._global_state()

    async def run_all(self) -> dict[str, Any]:
        async with self._tick_lock:
            if self._tasks.paused:
                raise RunConflict(
                    "the scheduler is paused; POST /tasks/start before running all tasks"
                )
            triggered: list[int] = []
            skipped: list[str] = []
            for task in self._tasks.list_all():
                if not task.enabled:
                    continue
                if self._task_job(task.id) is not None:
                    skipped.append(task.id)
                    continue
                try:
                    job = await self._launch_task(task, trigger="run-all")
                except (RunConflict, SpecError, TaskError):
                    skipped.append(task.id)
                    continue
                triggered.append(job.run_id)
        return self._global_state(triggered=triggered, skipped=skipped)

    async def tasks_status(self) -> dict[str, Any]:
        return self._global_state()

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

    def _task_job(self, task_id: str) -> _Job | None:
        for job in self._jobs.values():
            if job.task_id == task_id and not job.done.is_set():
                return job
        return None

    def _task_state(self, task: ScrapeTask) -> dict[str, Any]:
        job = self._task_job(task.id)
        return task.to_state(
            running=job is not None,
            active_run_id=job.run_id if job is not None else None,
        )

    def _global_state(
        self,
        *,
        triggered: list[int] | None = None,
        skipped: list[str] | None = None,
    ) -> dict[str, Any]:
        tasks = self._tasks.list_all()
        active = [job for job in self._jobs.values() if not job.done.is_set()]
        return {
            "paused": self._tasks.paused,
            "tasks": len(tasks),
            "enabledTasks": sum(1 for task in tasks if task.enabled),
            "activeRuns": len(active),
            "activeTaskIds": sorted({job.task_id for job in active if job.task_id}),
            "triggeredRunIds": triggered or [],
            "skippedTaskIds": skipped or [],
        }

    def _next_run_at(self, schedule: TaskSchedule, *, enabled: bool) -> str | None:
        if not enabled or schedule.kind != SCHEDULE_INTERVAL:
            return None
        next_at = schedule.next_after(now_utc())
        return iso(next_at) if next_at is not None else None

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
        errors = [
            message for message in (record.error_message, record.ingestion_error) if message
        ]
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

    async def _create_job_locked(
        self, spec: StoreSpec, *, task_id: str | None, trigger: str
    ) -> _Job:
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
        del source_id
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
        job = _Job(
            run_id=run_id,
            spec=spec,
            source=source,
            config=config,
            task_id=task_id,
            trigger=trigger,
        )
        self._jobs[run_id] = job
        self._launch_job(job)
        return job

    def _launch_job(self, job: _Job) -> None:
        task = asyncio.create_task(
            self._run_job(job), name=f"scraper-run-{job.run_id}"
        )
        self._job_tasks[job.run_id] = task
        task.add_done_callback(lambda _: self._job_tasks.pop(job.run_id, None))

    def _mark_finalized(self, job: _Job) -> bool:
        if job.finalized:
            return False
        job.finalized = True
        return True

    async def _finalize_cancelled(self, job: _Job) -> None:
        if self._mark_finalized(job):
            await asyncio.to_thread(
                repo.finish_run,
                self.settings.db_path,
                job.run_id,
                RUN_STATUS_FAILED,
                error_message="cancelled before start",
            )
            job.result = PipelineResult(
                run_id=job.run_id,
                status=RUN_STATUS_FAILED,
                counters=Counters(),
                error="cancelled before start",
            )
            if job.task_id is not None:
                with contextlib.suppress(Exception):
                    await self._record_task_result(job)
        job.done.set()

    async def _run_job(self, job: _Job) -> None:
        try:
            async with self._semaphore:
                if job.cancelled:
                    await self._finalize_cancelled(job)
                    return
                job.started = True
                self._active[job.run_id] = job
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
                        # None si falta INGESTION_API_URL o el token: la corrida
                        # queda solo en SQLite y el catálogo.
                        ingestion=IngestionClient.from_settings(self.settings),
                        task_id=job.task_id,
                    )
                except asyncio.CancelledError:
                    job.result = PipelineResult(
                        run_id=job.run_id,
                        status=RUN_STATUS_FAILED,
                        counters=Counters(),
                        error="aborted: worker cancelled",
                    )
                    with contextlib.suppress(Exception):
                        await asyncio.to_thread(
                            repo.finish_run,
                            self.settings.db_path,
                            job.run_id,
                            RUN_STATUS_FAILED,
                            error_message="aborted: worker cancelled",
                        )
                    raise
                except Exception as exc:  # noqa: BLE001 - reported, never swallowed
                    with contextlib.suppress(Exception):
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
            self._active.pop(job.run_id, None)
            if job.task_id is not None and job.result is not None:
                with contextlib.suppress(Exception):
                    await self._record_task_result(job)
            job.done.set()
            self._remember_job(job.run_id)

    def _remember_job(self, run_id: int) -> None:
        """Keep a bounded history of completed jobs in memory.

        The DB and the task files already have the durable record. Without this
        bound, an hourly task would keep every ``_Job`` for the life of the pod.
        """
        if len(self._completed) == self._completed.maxlen:
            oldest = self._completed[0]
            self._completed.append(run_id)
            self._jobs.pop(oldest, None)
        else:
            self._completed.append(run_id)

    async def _launch_task(self, task: ScrapeTask, *, trigger: str) -> _Job:
        spec = self._resolve(task.store, None)
        async with self._lock:
            if self._task_job(task.id) is not None:
                raise RunConflict(f"task {task.id!r} already has an open run")
            job = await self._create_job_locked(spec, task_id=task.id, trigger=trigger)
            task.last_run_at = now_iso()
            task.last_status = RUN_STATUS_PENDING
            self._tasks.save(task)
            return job

    async def _record_task_result(self, job: _Job) -> None:
        assert job.task_id is not None
        try:
            task = self._tasks.get(job.task_id)
        except TaskNotFound:
            return
        if job.run_id in task.run_ids:
            return
        result = job.result
        status = result.status if result is not None else RUN_STATUS_FAILED
        task.last_run_id = job.run_id
        task.last_run_at = now_iso()
        task.last_status = status
        task.run_count += 1
        task.run_ids.append(job.run_id)
        task.run_ids = task.run_ids[-self.settings.task_history_limit :]
        if status == RUN_STATUS_SUCCESS:
            task.consecutive_failures = 0
        else:
            task.consecutive_failures += 1
        if task.schedule.kind == SCHEDULE_INTERVAL:
            next_at = task.schedule.next_after(now_utc())
            task.next_run_at = iso(next_at) if next_at is not None else None
        if task.max_runs is not None and task.run_count >= task.max_runs:
            task.enabled = False
            task.next_run_at = None
        self._tasks.save(task)

    # -- scheduler -----------------------------------------------------------

    async def _schedule_loop(self) -> None:
        while True:
            await asyncio.sleep(self.settings.scheduler_tick)
            try:
                await self._tick()
            except asyncio.CancelledError:
                raise
            except Exception:  # noqa: BLE001 - one bad task must not kill the loop
                _log.exception("scheduler tick failed")

    async def _tick(self) -> None:
        async with self._tick_lock:
            if self._tasks.paused:
                return
            now = now_utc()
            for task in self._tasks.list_all():
                if not task.enabled or task.schedule.kind != SCHEDULE_INTERVAL:
                    continue
                if task.max_runs is not None and task.run_count >= task.max_runs:
                    task.enabled = False
                    task.next_run_at = None
                    self._tasks.save(task)
                    continue
                if self._task_job(task.id) is not None:
                    continue
                due = parse_iso(task.next_run_at)
                if due is None:
                    task.next_run_at = self._next_run_at(task.schedule, enabled=True)
                    self._tasks.save(task)
                    continue
                if due > now:
                    continue
                if self._tasks.paused:
                    return
                try:
                    await self._launch_task(task, trigger="schedule")
                except RunConflict:
                    retry = min(task.schedule.every_seconds or RETRY_CONFLICT_SECONDS, 60)
                    task.next_run_at = iso(now + timedelta(seconds=retry))
                    self._tasks.save(task)
                except (SpecError, TaskError) as exc:
                    task.last_status = RUN_STATUS_FAILED
                    task.consecutive_failures += 1
                    retry = min(task.schedule.every_seconds or RETRY_CONFLICT_SECONDS, 300)
                    task.next_run_at = iso(now + timedelta(seconds=retry))
                    self._tasks.save(task)
                    _log.warning("task %s could not start: %s", task.id, exc)

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
