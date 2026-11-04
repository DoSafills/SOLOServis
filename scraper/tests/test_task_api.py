"""Task CRUD, parallel execution and the global kill switch."""

from __future__ import annotations

import asyncio
from dataclasses import replace

import httpx

from scraper.api import manager as manager_module
from scraper.api.app import create_app
from scraper.api.manager import RunManager
from scraper.db import repository as repo
from scraper.models import RUN_STATUS_FAILED, RUN_STATUS_SUCCESS, Counters
from scraper.pipeline import PipelineResult
from scraper.tasks import build_schedule, now_iso


def _parallel_execute(started: list[int], both_started: asyncio.Event):
    async def fake(settings, *, run_id, stop_event, **_kwargs):
        started.append(run_id)
        if len(started) >= 2:
            both_started.set()
        await stop_event.wait()
        await asyncio.to_thread(
            repo.finish_run,
            settings.db_path,
            run_id,
            RUN_STATUS_FAILED,
            error_message="stopped by request",
        )
        return PipelineResult(
            run_id=run_id,
            status=RUN_STATUS_FAILED,
            counters=Counters(),
            error="stopped by request",
        )

    return fake


def _success_execute():
    async def fake(settings, *, run_id, stop_event, **_kwargs):
        await asyncio.to_thread(
            repo.finish_run,
            settings.db_path,
            run_id,
            RUN_STATUS_SUCCESS,
            counters=Counters(),
        )
        return PipelineResult(
            run_id=run_id,
            status=RUN_STATUS_SUCCESS,
            counters=Counters(),
        )

    return fake


def _blocking_success_execute(release: asyncio.Event):
    async def fake(settings, *, run_id, stop_event, **_kwargs):
        await release.wait()
        await asyncio.to_thread(
            repo.finish_run,
            settings.db_path,
            run_id,
            RUN_STATUS_SUCCESS,
            counters=Counters(),
        )
        return PipelineResult(
            run_id=run_id,
            status=RUN_STATUS_SUCCESS,
            counters=Counters(),
        )

    return fake


def _headers(settings):
    return {"Authorization": f"Bearer {settings.api_token}"}


def test_task_crud_and_global_controls(settings, monkeypatch):
    async def scenario() -> None:
        manager = RunManager(settings)
        await manager.start()
        started: list[int] = []
        both_started = asyncio.Event()
        monkeypatch.setattr(
            manager_module, "execute", _parallel_execute(started, both_started)
        )
        app = create_app(settings, manager)
        transport = httpx.ASGITransport(app=app)

        async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
            headers = _headers(settings)
            assert (await client.get("/tasks")).status_code == 401

            created = await client.post(
                "/tasks",
                headers=headers,
                json={
                    "store": "ripley-cl",
                    "name": "Ripley every hour",
                    "schedule": {"kind": "interval", "every": "1h"},
                },
            )
            assert created.status_code == 201
            first = created.json()
            assert first["schedule"] == {"kind": "interval", "everySeconds": 3600}
            assert first["nextRunAt"] is not None
            assert first["running"] is False

            second = (
                await client.post(
                    "/tasks",
                    headers=headers,
                    json={
                        "store": "falabella-cl",
                        "schedule": {"kind": "manual"},
                    },
                )
            ).json()
            assert second["schedule"]["kind"] == "manual"
            assert second["schedule"]["everySeconds"] is None
            assert second["nextRunAt"] is None

            listed = await client.get("/tasks", headers=headers)
            assert listed.status_code == 200
            assert {task["id"] for task in listed.json()["tasks"]} == {
                first["id"],
                second["id"],
            }

            fetched = await client.get(f"/tasks/{first['id']}", headers=headers)
            assert fetched.status_code == 200
            assert fetched.json()["store"] == "ripley-cl"

            updated = await client.patch(
                f"/tasks/{first['id']}",
                headers=headers,
                json={"name": "Renamed", "enabled": False},
            )
            assert updated.status_code == 200
            assert updated.json()["name"] == "Renamed"
            assert updated.json()["enabled"] is False
            assert updated.json()["nextRunAt"] is None

            assert (
                await client.post(
                    "/tasks",
                    headers=headers,
                    json={"store": "ripley-cl", "schedule": {"kind": "interval"}},
                )
            ).status_code == 400
            assert (
                await client.post(
                    "/tasks",
                    headers=headers,
                    json={"store": "nope", "schedule": {"kind": "manual"}},
                )
            ).status_code == 400
            assert (await client.get("/tasks/nope", headers=headers)).status_code == 404

            # Re-enable the first task as manual so the scheduler never fires.
            patched = await client.patch(
                f"/tasks/{first['id']}",
                headers=headers,
                json={"enabled": True, "schedule": {"kind": "manual"}},
            )
            assert patched.status_code == 200

            run_one = await client.post(f"/tasks/{first['id']}/run", headers=headers)
            run_two = await client.post(f"/tasks/{second['id']}/run", headers=headers)
            assert run_one.status_code == 202
            assert run_two.status_code == 202
            assert run_one.json()["taskId"] == first["id"]
            assert run_two.json()["taskId"] == second["id"]

            await asyncio.wait_for(both_started.wait(), timeout=5)
            status_running = (await client.get("/tasks/status", headers=headers)).json()
            assert status_running["activeRuns"] == 2
            assert set(status_running["activeTaskIds"]) == {first["id"], second["id"]}

            stopped = await client.post("/tasks/stop", headers=headers)
            assert stopped.status_code == 200
            assert stopped.json()["paused"] is True
            assert stopped.json()["activeRuns"] == 0

            for run in (run_one.json()["runId"], run_two.json()["runId"]):
                record = repo.get_run(settings.db_path, run)
                assert record.status == RUN_STATUS_FAILED
                assert record.error_message == "stopped by request"

            blocked = await client.post("/tasks/run", headers=headers)
            assert blocked.status_code == 409
            blocked_run = await client.post(
                "/runs", headers=headers, json={"store": "ripley-cl"}
            )
            assert blocked_run.status_code == 409

            resumed = await client.post("/tasks/start", headers=headers)
            assert resumed.status_code == 200
            assert resumed.json()["paused"] is False

            # run-all starts both tasks; stop them immediately.
            started.clear()
            both_started.clear()
            triggered = await client.post("/tasks/run", headers=headers)
            assert triggered.status_code == 200
            assert len(triggered.json()["triggeredRunIds"]) == 2
            await asyncio.wait_for(both_started.wait(), timeout=5)
            await client.post("/tasks/stop", headers=headers)

            deleted = await client.delete(f"/tasks/{first['id']}", headers=headers)
            assert deleted.status_code == 204
            assert (await client.get(f"/tasks/{first['id']}", headers=headers)).status_code == 404

        await manager.shutdown()

    asyncio.run(scenario())


def test_scheduler_starts_a_due_interval_task_and_reschedules(settings, monkeypatch):
    async def scenario() -> None:
        fast = replace(settings, scheduler_tick=0.1, task_history_limit=10)
        manager = RunManager(fast)
        await manager.start()
        monkeypatch.setattr(manager_module, "execute", _success_execute())

        task = await manager.create_task(
            store="ripley-cl",
            schedule=build_schedule("interval", every_seconds=3600),
        )
        stored = manager._tasks.get(task["id"])
        stored.next_run_at = now_iso()  # force the scheduler to see it as due
        manager._tasks.save(stored)

        current = task
        for _ in range(60):
            await asyncio.sleep(0.05)
            current = await manager.get_task(task["id"])
            if current["runCount"] >= 1:
                break

        assert current["runCount"] == 1
        assert current["lastStatus"] == RUN_STATUS_SUCCESS
        assert current["lastRunId"] is not None
        assert current["nextRunAt"] is not None
        await manager.shutdown()

    asyncio.run(scenario())


def test_startup_reconciles_a_task_that_was_running(settings, monkeypatch):
    async def scenario() -> None:
        quiet = replace(settings, scheduler_enabled=False)
        manager = RunManager(quiet)
        await manager.start()
        task = await manager.create_task(
            store="ripley-cl",
            schedule=build_schedule("interval", every_seconds=3600),
        )
        stored = manager._tasks.get(task["id"])
        stored.last_status = "running"
        stored.next_run_at = None
        manager._tasks.save(stored)
        await manager.shutdown()

        restarted = RunManager(quiet)
        await restarted.start()
        current = await restarted.get_task(task["id"])
        assert current["lastStatus"] == RUN_STATUS_FAILED
        assert current["consecutiveFailures"] == 1
        assert current["nextRunAt"] is not None
        await restarted.shutdown()

    asyncio.run(scenario())


def test_task_run_immediately_and_max_runs(settings, monkeypatch):
    async def scenario() -> None:
        manager = RunManager(settings)
        await manager.start()
        release = asyncio.Event()
        monkeypatch.setattr(
            manager_module, "execute", _blocking_success_execute(release)
        )
        app = create_app(settings, manager)
        transport = httpx.ASGITransport(app=app)

        async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
            headers = _headers(settings)
            created = await client.post(
                "/tasks",
                headers=headers,
                json={
                    "store": "mercadolibre-cl",
                    "schedule": {"kind": "manual"},
                    "maxRuns": 1,
                    "runImmediately": True,
                },
            )
            assert created.status_code == 201
            task_id = created.json()["id"]
            assert created.json()["running"] is True

            job = manager._task_job(task_id)
            assert job is not None
            release.set()
            await asyncio.wait_for(job.done.wait(), timeout=5)

            task = (await client.get(f"/tasks/{task_id}", headers=headers)).json()
            assert task["runCount"] == 1
            assert task["lastStatus"] == RUN_STATUS_SUCCESS
            assert task["enabled"] is False
            assert task["maxRuns"] == 1

            again = await client.post(f"/tasks/{task_id}/run", headers=headers)
            assert again.status_code == 409

        await manager.shutdown()

    asyncio.run(scenario())
