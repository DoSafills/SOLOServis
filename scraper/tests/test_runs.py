"""Registro de ejecuciones (TAL-210).

El run es la unidad de auditoría del scraper: qué se pidió, cuándo, qué contó y
cómo terminó. Estos tests fijan el ciclo de vida, la separación entre
contadores de páginas y de ítems, la recuperación tras un reinicio y lo que la
API de control expone.
"""

from __future__ import annotations

import asyncio

import httpx

from scraper.api.app import create_app
from scraper.api.manager import RunManager
from scraper.db import repository as repo
from scraper.models import (
    RUN_STATUS_FAILED,
    RUN_STATUS_PENDING,
    RUN_STATUS_RUNNING,
    RUN_STATUS_SUCCESS,
    Counters,
)


def _db(settings):
    repo.init_db(settings.db_path)
    _, config_id, _ = repo.ensure_source(
        settings.db_path,
        name="Falabella",
        base_url="https://www.falabella.com/falabella-cl",
        store_id=1,
        spec_slug="falabella-cl",
    )
    return config_id


def test_run_lifecycle_is_recorded(settings):
    config_id = _db(settings)

    run_id = repo.create_run(settings.db_path, config_id, status=RUN_STATUS_PENDING)
    created = repo.get_run(settings.db_path, run_id)
    assert created.status == RUN_STATUS_PENDING
    assert created.finished_at is None
    assert created.started_at
    assert repo.run_duration_ms(settings.db_path, run_id) is None

    repo.mark_run_running(settings.db_path, run_id)
    repo.finish_run(
        settings.db_path,
        run_id,
        RUN_STATUS_SUCCESS,
        counters=Counters(pages_found=2, pages_processed=2, items_found=25, items_sent=25),
    )

    done = repo.get_run(settings.db_path, run_id)
    assert done.status == RUN_STATUS_SUCCESS
    assert done.finished_at is not None
    assert done.error_message is None
    duration = repo.run_duration_ms(settings.db_path, run_id)
    assert duration is not None and duration >= 0


def test_page_and_item_counters_stay_separate(settings):
    config_id = _db(settings)
    run_id = repo.create_run(settings.db_path, config_id)

    repo.finish_run(
        settings.db_path,
        run_id,
        RUN_STATUS_SUCCESS,
        counters=Counters(
            pages_found=3,
            pages_processed=3,
            pages_failed=1,
            items_found=40,
            items_sent=37,
            items_failed=1,
            items_rejected=2,
        ),
    )

    record = repo.get_run(settings.db_path, run_id)
    assert (record.records_found, record.records_processed, record.records_failed) == (3, 3, 1)
    assert (record.items_found, record.items_sent, record.items_failed, record.items_rejected) == (
        40,
        37,
        1,
        2,
    )


def test_failure_keeps_the_reason(settings):
    config_id = _db(settings)
    run_id = repo.create_run(settings.db_path, config_id)

    repo.finish_run(
        settings.db_path,
        run_id,
        RUN_STATUS_FAILED,
        error_message="404 en /producto/1",
    )

    record = repo.get_run(settings.db_path, run_id)
    assert record.status == RUN_STATUS_FAILED
    assert record.error_message == "404 en /producto/1"


def test_finish_run_rejects_an_unknown_status(settings):
    config_id = _db(settings)
    run_id = repo.create_run(settings.db_path, config_id)

    try:
        repo.finish_run(settings.db_path, run_id, "casi-listo")
    except ValueError as exc:
        assert "casi-listo" in str(exc)
    else:  # pragma: no cover - el fallo es el mensaje
        raise AssertionError("un estado fuera de RUN_STATUSES debía rechazarse")

    assert repo.get_run(settings.db_path, run_id).status == RUN_STATUS_RUNNING


def test_orphan_runs_are_closed_on_restart(settings):
    config_id = _db(settings)
    running = repo.create_run(settings.db_path, config_id, status=RUN_STATUS_RUNNING)
    pending = repo.create_run(settings.db_path, config_id, status=RUN_STATUS_PENDING)
    finished = repo.create_run(settings.db_path, config_id, status=RUN_STATUS_RUNNING)
    repo.finish_run(settings.db_path, finished, RUN_STATUS_SUCCESS)

    closed = repo.recover_orphan_runs(settings.db_path)

    assert closed == 2
    for run_id in (running, pending):
        record = repo.get_run(settings.db_path, run_id)
        assert record.status == RUN_STATUS_FAILED
        assert record.error_message == "aborted: process restarted"
        assert record.finished_at is not None
    assert repo.get_run(settings.db_path, finished).status == RUN_STATUS_SUCCESS


def test_list_returns_newest_first_and_filters_by_status(settings):
    config_id = _db(settings)
    primero = repo.create_run(settings.db_path, config_id)
    repo.finish_run(settings.db_path, primero, RUN_STATUS_FAILED, error_message="boom")
    segundo = repo.create_run(settings.db_path, config_id)
    repo.finish_run(settings.db_path, segundo, RUN_STATUS_SUCCESS)

    todos = repo.list_runs(settings.db_path)
    assert [run.id for run in todos] == [segundo, primero]

    fallidos = repo.list_runs(settings.db_path, status=RUN_STATUS_FAILED)
    assert [run.id for run in fallidos] == [primero]

    assert repo.list_runs(settings.db_path, limit=1)[0].id == segundo


def test_api_exposes_the_ingestion_outcome(settings):
    config_id = _db(settings)

    async def scenario() -> None:
        manager = RunManager(settings)
        await manager.start()
        run_id = repo.create_run(settings.db_path, config_id)
        repo.finish_run(
            settings.db_path,
            run_id,
            RUN_STATUS_SUCCESS,
            ingestion_run_id=42,
            ingestion_error="lote 3 rechazado",
            counters=Counters(items_found=10, items_sent=8, items_rejected=2),
        )

        app = create_app(settings, manager)
        transport = httpx.ASGITransport(app=app)
        headers = {"Authorization": f"Bearer {settings.api_token}"}
        async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
            body = (await client.get(f"/runs/{run_id}", headers=headers)).json()

        assert body["ingestionRunId"] == 42
        assert body["ingestionError"] == "lote 3 rechazado"
        assert body["errors"] == ["lote 3 rechazado"]
        assert body["counters"]["itemsSent"] == 8

    asyncio.run(scenario())


def test_api_filter_by_status(settings):
    config_id = _db(settings)

    async def scenario() -> None:
        manager = RunManager(settings)
        await manager.start()
        ok = repo.create_run(settings.db_path, config_id)
        repo.finish_run(settings.db_path, ok, RUN_STATUS_SUCCESS)
        bad = repo.create_run(settings.db_path, config_id)
        repo.finish_run(settings.db_path, bad, RUN_STATUS_FAILED, error_message="boom")

        app = create_app(settings, manager)
        transport = httpx.ASGITransport(app=app)
        headers = {"Authorization": f"Bearer {settings.api_token}"}
        async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
            fallidos = (await client.get("/runs?status=failed", headers=headers)).json()["runs"]
            todos = (await client.get("/runs", headers=headers)).json()["runs"]
            invalido = await client.get("/runs?status=casi-listo", headers=headers)

        assert [run["runId"] for run in fallidos] == [bad]
        assert {run["runId"] for run in todos} == {ok, bad}
        assert invalido.status_code == 422

    asyncio.run(scenario())
