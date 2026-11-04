"""Control API: auth, queueing, duplicate prevention and controlled stop."""

from __future__ import annotations

import asyncio

import httpx

from scraper.api import manager as manager_module
from scraper.api.app import create_app
from scraper.api.manager import RunManager
from scraper.db import repository as repo
from scraper.models import RUN_STATUS_FAILED, Counters
from scraper.pipeline import PipelineResult


def _fake_execute():
    async def fake(settings, *, run_id, stop_event, **_kwargs):
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


def test_control_api_end_to_end(settings, monkeypatch):
    async def scenario() -> None:
        manager = RunManager(settings)
        await manager.start()
        monkeypatch.setattr(manager_module, "execute", _fake_execute())
        app = create_app(settings, manager)
        transport = httpx.ASGITransport(app=app)
        headers = {"Authorization": f"Bearer {settings.api_token}"}

        async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
            assert (await client.get("/health")).json() == {"status": "ok"}
            ready = await client.get("/ready")
            assert ready.status_code == 200
            assert ready.json()["stores"] == 3

            assert (await client.get("/stores")).status_code == 401
            assert (
                await client.get("/stores", headers={"Authorization": "Bearer wrong"})
            ).status_code == 401
            stores = (await client.get("/stores", headers=headers)).json()["stores"]
            assert {store["slug"] for store in stores} == {
                "falabella-cl",
                "ripley-cl",
                "mercadolibre-cl",
            }

            keyed = {**headers, "Idempotency-Key": "schedule-2026-11-03"}
            first = await client.post("/runs", json={"store": "ripley-cl"}, headers=keyed)
            assert first.status_code == 202
            run_id = first.json()["runId"]
            again = await client.post("/runs", json={"store": "ripley-cl"}, headers=keyed)
            assert again.json()["runId"] == run_id

            conflict = await client.post("/runs", json={"store": "ripley-cl"}, headers=headers)
            assert conflict.status_code == 409

            listed = await client.get("/runs", headers=headers)
            assert listed.status_code == 200
            assert any(run["runId"] == run_id for run in listed.json()["runs"])

            stop = await client.post(f"/runs/{run_id}/stop", headers=headers)
            assert stop.status_code == 200
            assert stop.json()["status"] == RUN_STATUS_FAILED
            assert stop.json()["errors"] == ["stopped by request"]

            restarted = await client.post("/runs", json={"store": "ripley-cl"}, headers=headers)
            assert restarted.status_code == 202
            await client.post(f"/runs/{restarted.json()['runId']}/stop", headers=headers)

            assert (await client.get("/runs/99999", headers=headers)).status_code == 404
            assert (
                await client.post("/runs", json={"store": "nope"}, headers=headers)
            ).status_code == 400
            assert (
                await client.post("/runs", json={"stroe": "typo"}, headers=headers)
            ).status_code == 422

        await manager.shutdown()

    asyncio.run(scenario())
