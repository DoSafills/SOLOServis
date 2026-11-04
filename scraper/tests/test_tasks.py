"""Task files, schedules and the global paused state."""

from __future__ import annotations

import json

import pytest

from scraper.errors import TaskError, TaskNotFound
from scraper.tasks import (
    SCHEDULE_INTERVAL,
    SCHEDULE_MANUAL,
    TaskStore,
    build_schedule,
    make_task,
    new_task_id,
    now_iso,
    parse_duration,
    parse_task,
)


def test_parse_duration_accepts_human_units():
    assert parse_duration(90) == 90
    assert parse_duration("90s") == 90
    assert parse_duration("5m") == 300
    assert parse_duration("2h") == 7200
    assert parse_duration("1d") == 86400
    assert parse_duration(" 10 ") == 10


@pytest.mark.parametrize("raw", [0, -1, "0s", "", "soon", "1w", None, True])
def test_parse_duration_rejects_garbage(raw):
    with pytest.raises(TaskError):
        parse_duration(raw)


def test_build_schedule_interval_and_manual():
    interval = build_schedule("interval", every="1h")
    assert interval.kind == SCHEDULE_INTERVAL
    assert interval.every_seconds == 3600

    manual = build_schedule("manual")
    assert manual.kind == SCHEDULE_MANUAL
    assert manual.every_seconds is None


def test_build_schedule_rejects_ambiguous_input():
    with pytest.raises(TaskError, match="either"):
        build_schedule("interval", every="1h", every_seconds=3600)
    with pytest.raises(TaskError, match="manual"):
        build_schedule("manual", every="1h")
    with pytest.raises(TaskError, match="unknown"):
        build_schedule("cron")


def test_task_store_round_trip_and_atomic_file(tmp_path):
    store = TaskStore(tmp_path / "tasks")
    task = make_task(
        task_id="task-abc123",
        store="ripley-cl",
        name="Ripley hourly",
        schedule=build_schedule("interval", every_seconds=3600),
        max_runs=None,
        enabled=True,
        next_run_at=now_iso(),
    )
    store.save(task)

    path = tmp_path / "tasks" / "task-abc123.json"
    assert path.exists()
    raw = json.loads(path.read_text(encoding="utf-8"))
    assert raw["id"] == "task-abc123"
    assert raw["schedule"] == {"kind": "interval", "everySeconds": 3600}
    assert not list((tmp_path / "tasks").glob("*.tmp"))

    loaded = store.get("task-abc123")
    assert loaded.store == "ripley-cl"
    assert loaded.name == "Ripley hourly"
    assert loaded.schedule.every_seconds == 3600
    assert [task.id for task in store.list_all()] == ["task-abc123"]


def test_task_store_delete_and_missing(tmp_path):
    store = TaskStore(tmp_path / "tasks")
    task = make_task(
        task_id="task-delete",
        store="ripley-cl",
        name="",
        schedule=build_schedule("manual"),
        max_runs=1,
        enabled=False,
        next_run_at=None,
    )
    store.save(task)
    store.delete(task.id)
    with pytest.raises(TaskNotFound):
        store.get(task.id)
    store.delete(task.id)  # deleting twice is harmless


def test_task_store_paused_flag_is_persistent(tmp_path):
    directory = tmp_path / "tasks"
    store = TaskStore(directory)
    assert store.paused is False
    store.set_paused(True)

    reopened = TaskStore(directory)
    assert reopened.paused is True
    reopened.set_paused(False)
    assert reopened.paused is False


def test_parse_task_rejects_unknown_key(tmp_path):
    store = TaskStore(tmp_path / "tasks")
    task = make_task(
        task_id="task-strict",
        store="ripley-cl",
        name="",
        schedule=build_schedule("manual"),
        max_runs=None,
        enabled=True,
        next_run_at=None,
    )
    store.save(task)
    path = tmp_path / "tasks" / "task-strict.json"
    raw = json.loads(path.read_text(encoding="utf-8"))
    raw["cron"] = "* * * * *"
    with pytest.raises(TaskError, match="unknown key"):
        parse_task(raw, path)


def test_parse_task_rejects_interval_without_every(tmp_path):
    raw = {
        "schemaVersion": "1.0",
        "id": "task-bad",
        "store": "ripley-cl",
        "name": "",
        "enabled": True,
        "schedule": {"kind": "interval"},
        "createdAt": now_iso(),
        "updatedAt": now_iso(),
        "runCount": 0,
        "consecutiveFailures": 0,
        "runIds": [],
    }
    with pytest.raises(TaskError, match="everySeconds"):
        parse_task(raw, tmp_path / "task-bad.json")


def test_new_task_id_is_url_safe_and_unique():
    first = new_task_id()
    second = new_task_id({first})
    assert first != second
    assert first.startswith("task-")
    assert first.replace("-", "").isalnum()
