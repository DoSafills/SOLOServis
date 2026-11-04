"""Recurring task definitions, persisted as JSON on the PVC.

The UML schema has exactly four tables and none of them can hold a task: a
task is not a source, a config or a run. Rather than adding a table, tasks are
small versioned JSON files under ``<data>/tasks/``. They are validated with the
same strictness as a store spec and written atomically, so a crash can never
leave half a task behind.

A task only names a store and a schedule. The store itself is still defined by
``stores/<slug>/spec.json``; a task cannot invent selectors or URLs.
"""

from __future__ import annotations

import contextlib
import json
import os
import re
import secrets
import tempfile
import threading
from dataclasses import dataclass, field
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any

from scraper.errors import TaskError, TaskNotFound
from scraper.models import RUN_STATUSES

TASK_SCHEMA_VERSION = "1.0"

SCHEDULE_INTERVAL = "interval"
SCHEDULE_MANUAL = "manual"
SCHEDULE_KINDS = frozenset({SCHEDULE_INTERVAL, SCHEDULE_MANUAL})

_TASK_ID_RE = re.compile(r"^[a-z0-9][a-z0-9-]{0,63}$")
_DURATION_RE = re.compile(r"^(\d+)\s*(s|m|h|d)?$", re.IGNORECASE)
_UNIT_SECONDS = {"s": 1, "m": 60, "h": 3600, "d": 86400}

_TASK_KEYS = frozenset(
    {
        "schemaVersion",
        "id",
        "store",
        "name",
        "enabled",
        "schedule",
        "maxRuns",
        "createdAt",
        "updatedAt",
        "lastRunId",
        "lastRunAt",
        "lastStatus",
        "nextRunAt",
        "runCount",
        "consecutiveFailures",
        "runIds",
    }
)
_SCHEDULE_KEYS = frozenset({"kind", "everySeconds"})
_STATE_KEYS = frozenset({"schemaVersion", "paused", "updatedAt"})


# ---------------------------------------------------------------------------
# Time and duration helpers
# ---------------------------------------------------------------------------


def now_utc() -> datetime:
    return datetime.now(UTC)


def iso(moment: datetime) -> str:
    return moment.astimezone(UTC).isoformat(timespec="seconds").replace("+00:00", "Z")


def now_iso() -> str:
    return iso(now_utc())


def parse_iso(raw: str | None) -> datetime | None:
    if not raw:
        return None
    text = raw.strip()
    if text.endswith("Z"):
        text = text[:-1] + "+00:00"
    try:
        parsed = datetime.fromisoformat(text)
    except ValueError as exc:
        raise TaskError(f"invalid ISO-8601 timestamp {raw!r}") from exc
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=UTC)
    return parsed.astimezone(UTC)


def parse_duration(raw: Any) -> int:
    """Turn ``90``, ``"90s"``, ``"5m"``, ``"2h"`` or ``"1d"`` into seconds."""
    if isinstance(raw, bool):
        raise TaskError("a duration must be an integer or a string such as '5m'")
    if isinstance(raw, int):
        seconds = raw
    elif isinstance(raw, str):
        match = _DURATION_RE.match(raw.strip())
        if match is None:
            raise TaskError(
                f"invalid duration {raw!r}; expected an integer or <n><s|m|h|d>"
            )
        seconds = int(match.group(1)) * _UNIT_SECONDS[(match.group(2) or "s").lower()]
    else:
        raise TaskError("a duration must be an integer or a string such as '5m'")
    if seconds < 1:
        raise TaskError("a duration must be at least one second")
    return seconds


# ---------------------------------------------------------------------------
# Task model
# ---------------------------------------------------------------------------


@dataclass(frozen=True, slots=True)
class TaskSchedule:
    """When a task runs. ``manual`` tasks only run when asked explicitly."""

    kind: str = SCHEDULE_INTERVAL
    every_seconds: int | None = None

    def next_after(self, moment: datetime) -> datetime | None:
        if self.kind != SCHEDULE_INTERVAL or not self.every_seconds:
            return None
        return moment + timedelta(seconds=self.every_seconds)

    def to_dict(self) -> dict[str, Any]:
        payload: dict[str, Any] = {"kind": self.kind}
        if self.every_seconds is not None:
            payload["everySeconds"] = self.every_seconds
        return payload


@dataclass(slots=True)
class ScrapeTask:
    """One recurring scrape definition."""

    id: str
    store: str
    name: str = ""
    enabled: bool = True
    schedule: TaskSchedule = field(default_factory=TaskSchedule)
    max_runs: int | None = None
    created_at: str = ""
    updated_at: str = ""
    last_run_id: int | None = None
    last_run_at: str | None = None
    last_status: str | None = None
    next_run_at: str | None = None
    run_count: int = 0
    consecutive_failures: int = 0
    run_ids: list[int] = field(default_factory=list)

    def to_dict(self) -> dict[str, Any]:
        return {
            "schemaVersion": TASK_SCHEMA_VERSION,
            "id": self.id,
            "store": self.store,
            "name": self.name or self.store,
            "enabled": self.enabled,
            "schedule": self.schedule.to_dict(),
            "maxRuns": self.max_runs,
            "createdAt": self.created_at,
            "updatedAt": self.updated_at,
            "lastRunId": self.last_run_id,
            "lastRunAt": self.last_run_at,
            "lastStatus": self.last_status,
            "nextRunAt": self.next_run_at,
            "runCount": self.run_count,
            "consecutiveFailures": self.consecutive_failures,
            "runIds": list(self.run_ids),
        }

    def to_state(
        self, *, running: bool = False, active_run_id: int | None = None
    ) -> dict[str, Any]:
        payload = self.to_dict()
        payload["running"] = running
        payload["activeRunId"] = active_run_id
        return payload


def build_schedule(
    kind: str | None = None,
    *,
    every: Any = None,
    every_seconds: int | None = None,
) -> TaskSchedule:
    """Validate the API representation of a schedule."""
    resolved = (kind or SCHEDULE_INTERVAL).strip().lower()
    if resolved not in SCHEDULE_KINDS:
        raise TaskError(f"unknown schedule kind {kind!r}; expected {sorted(SCHEDULE_KINDS)}")
    if resolved == SCHEDULE_MANUAL:
        if every is not None or every_seconds is not None:
            raise TaskError("a manual schedule cannot declare 'every' or 'everySeconds'")
        return TaskSchedule(kind=SCHEDULE_MANUAL)
    if every is not None and every_seconds is not None:
        raise TaskError("declare either 'every' or 'everySeconds', not both")
    if every is None and every_seconds is None:
        raise TaskError("an interval schedule requires 'every' or 'everySeconds'")
    seconds = every_seconds if every_seconds is not None else parse_duration(every)
    return TaskSchedule(kind=SCHEDULE_INTERVAL, every_seconds=seconds)


def make_task(
    *,
    task_id: str,
    store: str,
    name: str,
    schedule: TaskSchedule,
    max_runs: int | None,
    enabled: bool,
    next_run_at: str | None,
) -> ScrapeTask:
    timestamp = now_iso()
    return ScrapeTask(
        id=task_id,
        store=store,
        name=name or store,
        enabled=enabled,
        schedule=schedule,
        max_runs=max_runs,
        created_at=timestamp,
        updated_at=timestamp,
        next_run_at=next_run_at,
    )


# ---------------------------------------------------------------------------
# Strict JSON parsing
# ---------------------------------------------------------------------------


def _task_error(path: Path, message: str) -> TaskError:
    return TaskError(f"{path}: {message}")


def _object(path: Path, where: str, raw: Any, allowed: frozenset[str]) -> dict[str, Any]:
    if not isinstance(raw, dict):
        raise _task_error(path, f"{where}: must be a JSON object")
    unknown = set(raw) - allowed
    if unknown:
        raise _task_error(
            path,
            f"{where}: unknown key(s) {sorted(unknown)}; expected: {', '.join(sorted(allowed))}",
        )
    return raw


def _text(path: Path, where: str, value: Any, *, required: bool = False) -> str | None:
    if value is None:
        if required:
            raise _task_error(path, f"{where}: is required")
        return None
    if not isinstance(value, str):
        raise _task_error(path, f"{where}: must be a string")
    text = value.strip()
    if required and not text:
        raise _task_error(path, f"{where}: must not be empty")
    return text or None


def _bool(path: Path, where: str, value: Any, default: bool = False) -> bool:
    if value is None:
        return default
    if not isinstance(value, bool):
        raise _task_error(path, f"{where}: must be a boolean")
    return value


def _integer(path: Path, where: str, value: Any, *, minimum: int = 0) -> int:
    if isinstance(value, bool) or not isinstance(value, int):
        raise _task_error(path, f"{where}: must be an integer")
    if value < minimum:
        raise _task_error(path, f"{where}: must be >= {minimum}")
    return value


def _parse_schedule(path: Path, raw: Any) -> TaskSchedule:
    obj = _object(path, "schedule", raw, _SCHEDULE_KEYS)
    kind = (_text(path, "schedule.kind", obj.get("kind")) or SCHEDULE_INTERVAL).lower()
    if kind not in SCHEDULE_KINDS:
        raise _task_error(path, f"schedule.kind: must be one of {sorted(SCHEDULE_KINDS)}")
    every_seconds = obj.get("everySeconds")
    if every_seconds is not None:
        every_seconds = _integer(path, "schedule.everySeconds", every_seconds, minimum=1)
    if kind == SCHEDULE_INTERVAL and every_seconds is None:
        raise _task_error(path, "schedule.everySeconds: is required for an interval")
    if kind == SCHEDULE_MANUAL and every_seconds is not None:
        raise _task_error(path, "schedule.everySeconds: not valid for a manual schedule")
    return TaskSchedule(kind=kind, every_seconds=every_seconds)


def parse_task(raw: Any, path: str | Path) -> ScrapeTask:
    """Validate one task file. Unknown keys are a hard error."""
    target = Path(path)
    obj = _object(target, "task", raw, _TASK_KEYS)
    version = _text(target, "schemaVersion", obj.get("schemaVersion"), required=True)
    if version != TASK_SCHEMA_VERSION:
        raise _task_error(
            target,
            f"schemaVersion: unsupported version {version!r}; expected {TASK_SCHEMA_VERSION!r}",
        )
    task_id = _text(target, "id", obj.get("id"), required=True)
    assert task_id is not None
    if not _TASK_ID_RE.match(task_id):
        raise _task_error(
            target,
            "id: must match ^[a-z0-9][a-z0-9-]{0,63}$",
        )
    store = _text(target, "store", obj.get("store"), required=True)
    assert store is not None
    name = _text(target, "name", obj.get("name")) or store
    enabled = _bool(target, "enabled", obj.get("enabled"), default=True)
    schedule = _parse_schedule(target, obj.get("schedule") or {"kind": SCHEDULE_INTERVAL})
    max_runs = obj.get("maxRuns")
    if max_runs is not None:
        max_runs = _integer(target, "maxRuns", max_runs, minimum=1)
    created_at = _text(target, "createdAt", obj.get("createdAt"), required=True)
    updated_at = _text(target, "updatedAt", obj.get("updatedAt"), required=True)
    assert created_at is not None and updated_at is not None
    parse_iso(created_at)
    parse_iso(updated_at)
    last_run_id = obj.get("lastRunId")
    if last_run_id is not None:
        last_run_id = _integer(target, "lastRunId", last_run_id, minimum=1)
    last_run_at = _text(target, "lastRunAt", obj.get("lastRunAt"))
    if last_run_at:
        parse_iso(last_run_at)
    last_status = _text(target, "lastStatus", obj.get("lastStatus"))
    if last_status is not None and last_status not in RUN_STATUSES:
        raise _task_error(
            target, f"lastStatus: must be one of {sorted(RUN_STATUSES)}"
        )
    next_run_at = _text(target, "nextRunAt", obj.get("nextRunAt"))
    if next_run_at:
        parse_iso(next_run_at)
    run_count = _integer(target, "runCount", obj.get("runCount", 0), minimum=0)
    consecutive_failures = _integer(
        target, "consecutiveFailures", obj.get("consecutiveFailures", 0), minimum=0
    )
    run_ids_raw = obj.get("runIds") or []
    if not isinstance(run_ids_raw, list):
        raise _task_error(target, "runIds: must be an array of integers")
    run_ids: list[int] = []
    for index, value in enumerate(run_ids_raw):
        run_ids.append(_integer(target, f"runIds[{index}]", value, minimum=1))
    return ScrapeTask(
        id=task_id,
        store=store,
        name=name,
        enabled=enabled,
        schedule=schedule,
        max_runs=max_runs,
        created_at=created_at,
        updated_at=updated_at,
        last_run_id=last_run_id,
        last_run_at=last_run_at,
        last_status=last_status,
        next_run_at=next_run_at,
        run_count=run_count,
        consecutive_failures=consecutive_failures,
        run_ids=run_ids,
    )


# ---------------------------------------------------------------------------
# Store
# ---------------------------------------------------------------------------


class TaskStore:
    """Reads and writes task JSON files and the global paused flag."""

    STATE_FILE = "state.json"

    def __init__(self, directory: str | Path) -> None:
        self.directory = Path(directory)
        self.directory.mkdir(parents=True, exist_ok=True)
        self._lock = threading.RLock()

    # -- tasks ---------------------------------------------------------------

    def list_all(self) -> list[ScrapeTask]:
        with self._lock:
            tasks: list[ScrapeTask] = []
            for path in sorted(self.directory.glob("task-*.json")):
                try:
                    raw = json.loads(path.read_text(encoding="utf-8"))
                except FileNotFoundError:
                    continue
                except json.JSONDecodeError as exc:
                    raise _task_error(path, f"invalid JSON: {exc}") from exc
                tasks.append(parse_task(raw, path))
            tasks.sort(key=lambda task: (task.created_at, task.id))
            return tasks

    def get(self, task_id: str) -> ScrapeTask:
        with self._lock:
            path = self._path(task_id)
            if not path.exists():
                raise TaskNotFound(f"task {task_id!r} does not exist")
            try:
                raw = json.loads(path.read_text(encoding="utf-8"))
            except json.JSONDecodeError as exc:
                raise _task_error(path, f"invalid JSON: {exc}") from exc
            return parse_task(raw, path)

    def save(self, task: ScrapeTask) -> None:
        with self._lock:
            task.updated_at = now_iso()
            _atomic_write(self._path(task.id), json.dumps(task.to_dict(), indent=2) + "\n")

    def delete(self, task_id: str) -> None:
        with self._lock:
            with contextlib.suppress(FileNotFoundError):
                self._path(task_id).unlink()

    # -- global state --------------------------------------------------------

    @property
    def paused(self) -> bool:
        with self._lock:
            path = self.directory / self.STATE_FILE
            if not path.exists():
                return False
            try:
                raw = json.loads(path.read_text(encoding="utf-8"))
            except json.JSONDecodeError as exc:
                raise _task_error(path, f"invalid JSON: {exc}") from exc
            obj = _object(path, "state", raw, _STATE_KEYS)
            return _bool(path, "paused", obj.get("paused"), default=False)

    def set_paused(self, paused: bool) -> None:
        with self._lock:
            payload = {
                "schemaVersion": TASK_SCHEMA_VERSION,
                "paused": bool(paused),
                "updatedAt": now_iso(),
            }
            _atomic_write(
                self.directory / self.STATE_FILE,
                json.dumps(payload, indent=2) + "\n",
            )

    # -- internals -----------------------------------------------------------

    def _path(self, task_id: str) -> Path:
        if not _TASK_ID_RE.match(task_id or ""):
            raise TaskNotFound(f"task {task_id!r} does not exist")
        return self.directory / f"{task_id}.json"


def new_task_id(existing: set[str] | None = None) -> str:
    """Generate a stable-looking, URL-safe task id."""
    known = existing or set()
    while True:
        candidate = f"task-{secrets.token_hex(6)}"
        if candidate not in known:
            return candidate


def _atomic_write(path: Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temp = tempfile.mkstemp(dir=path.parent, prefix=f".{path.name}.", suffix=".tmp")
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            handle.write(text)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temp, path)
    except BaseException:
        with contextlib.suppress(FileNotFoundError):
            os.unlink(temp)
        raise


__all__ = [
    "SCHEDULE_INTERVAL",
    "SCHEDULE_MANUAL",
    "ScrapeTask",
    "TaskSchedule",
    "TaskStore",
    "build_schedule",
    "iso",
    "make_task",
    "new_task_id",
    "now_iso",
    "now_utc",
    "parse_duration",
    "parse_iso",
    "parse_task",
]
