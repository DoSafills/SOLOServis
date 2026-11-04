"""FastAPI control plane for the scraper.

The service is internal: the Go backend proxies ``/admin/scraper/*`` to it, so
it is never exposed publicly. Every mutating call needs the shared
``INTERNAL_API_TOKEN``; health and readiness stay open for probes.

Endpoints::

    GET    /health
    GET    /ready
    GET    /stores
    POST   /runs                  {store} | {url}   -> 202
    GET    /runs?limit=50
    GET    /runs/{run_id}
    POST   /runs/{run_id}/stop

    GET    /tasks
    POST   /tasks
    GET    /tasks/status
    POST   /tasks/start           resume the scheduler
    POST   /tasks/stop            pause the scheduler and stop every run
    POST   /tasks/run             run every enabled task now
    GET    /tasks/{task_id}
    PATCH  /tasks/{task_id}
    DELETE /tasks/{task_id}
    POST   /tasks/{task_id}/run
    POST   /tasks/{task_id}/stop
"""

from __future__ import annotations

import secrets
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

from fastapi import (
    Depends,
    FastAPI,
    Header,
    HTTPException,
    Query,
    Request,
    Response,
    status,
)
from fastapi.responses import JSONResponse
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer

from scraper.api.manager import RunManager
from scraper.api.models import (
    CreateTaskRequest,
    HealthResponse,
    ReadyResponse,
    RunListResponse,
    RunResponse,
    StartRunRequest,
    StoreListResponse,
    StoreResponse,
    TaskControlResponse,
    TaskListResponse,
    TaskResponse,
    TaskScheduleRequest,
    UpdateTaskRequest,
)
from scraper.config import Settings
from scraper.db import repository as repo
from scraper.errors import (
    ConfigError,
    RunConflict,
    RunNotFound,
    ScraperError,
    SpecError,
    StoreMismatchError,
    TaskError,
    TaskNotFound,
)
from scraper.tasks import TaskSchedule, build_schedule

bearer = HTTPBearer(auto_error=False)


def create_app(settings: Settings, manager: RunManager | None = None) -> FastAPI:
    """Build the app. Tests pass a manager with an injected transport."""
    runner = manager

    @asynccontextmanager
    async def lifespan(app: FastAPI) -> AsyncIterator[None]:
        nonlocal runner
        if runner is None:
            runner = RunManager(settings)
        await runner.start()
        app.state.manager = runner
        try:
            yield
        finally:
            await runner.shutdown()

    app = FastAPI(
        title="SOLOServis scraper control API",
        version="2.0.0",
        lifespan=lifespan,
    )
    app.state.settings = settings
    if runner is not None:
        # Tests inject a manager and drive its lifecycle themselves; the
        # lifespan still owns the real one.
        app.state.manager = runner

    def current_manager(request: Request) -> RunManager:
        return request.app.state.manager

    async def require_token(
        credentials: HTTPAuthorizationCredentials | None = Depends(bearer),  # noqa: B008
    ) -> None:
        if not settings.api_token:
            raise HTTPException(
                status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
                detail="SCRAPER_API_TOKEN is not configured",
            )
        if credentials is None or not secrets.compare_digest(
            credentials.credentials, settings.api_token
        ):
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED,
                detail="missing or invalid bearer token",
                headers={"WWW-Authenticate": "Bearer"},
            )

    @app.exception_handler(SpecError)
    async def _spec_error(_: Request, exc: SpecError) -> JSONResponse:
        return JSONResponse(status_code=400, content={"detail": str(exc)})

    @app.exception_handler(RunConflict)
    async def _conflict(_: Request, exc: RunConflict) -> JSONResponse:
        return JSONResponse(status_code=409, content={"detail": str(exc)})

    @app.exception_handler(RunNotFound)
    async def _not_found(_: Request, exc: RunNotFound) -> JSONResponse:
        return JSONResponse(status_code=404, content={"detail": str(exc)})

    @app.exception_handler(StoreMismatchError)
    async def _mismatch(_: Request, exc: StoreMismatchError) -> JSONResponse:
        return JSONResponse(status_code=409, content={"detail": str(exc)})

    @app.exception_handler(ConfigError)
    async def _config(_: Request, exc: ConfigError) -> JSONResponse:
        return JSONResponse(status_code=500, content={"detail": str(exc)})

    @app.exception_handler(TaskNotFound)
    async def _task_not_found(_: Request, exc: TaskNotFound) -> JSONResponse:
        return JSONResponse(status_code=404, content={"detail": str(exc)})

    @app.exception_handler(TaskError)
    async def _task_error(_: Request, exc: TaskError) -> JSONResponse:
        return JSONResponse(status_code=400, content={"detail": str(exc)})

    @app.exception_handler(ScraperError)
    async def _scraper(_: Request, exc: ScraperError) -> JSONResponse:
        return JSONResponse(status_code=500, content={"detail": str(exc)})

    @app.get("/health", response_model=HealthResponse)
    async def health() -> HealthResponse:
        return HealthResponse(status="ok")

    @app.get("/ready", response_model=ReadyResponse)
    async def ready(request: Request) -> ReadyResponse:
        try:
            count = await _to_thread(repo.count_rows, settings.db_path, "SCRAPE_RUN")
        except Exception as exc:  # noqa: BLE001 - readiness reports, never raises
            raise HTTPException(status_code=503, detail=f"database not ready: {exc}") from exc
        specs = current_manager(request).specs
        if not specs:
            raise HTTPException(status_code=503, detail="no store specs loaded")
        return ReadyResponse(status="ready", database=f"ok ({count} runs)", stores=len(specs))

    @app.get(
        "/stores",
        response_model=StoreListResponse,
        dependencies=[Depends(require_token)],
    )
    async def stores(request: Request) -> StoreListResponse:
        entries = [StoreResponse(**entry) for entry in current_manager(request).store_catalog()]
        return StoreListResponse(stores=entries)

    @app.post(
        "/runs",
        response_model=RunResponse,
        status_code=status.HTTP_202_ACCEPTED,
        dependencies=[Depends(require_token)],
    )
    async def start_run(
        body: StartRunRequest,
        request: Request,
        idempotency_key: str | None = Header(default=None, alias="Idempotency-Key"),
    ) -> RunResponse:
        state = await current_manager(request).start_run(
            store=body.store,
            url=body.url,
            idempotency_key=idempotency_key,
        )
        return RunResponse(**state.to_dict())

    @app.get(
        "/runs",
        response_model=RunListResponse,
        dependencies=[Depends(require_token)],
    )
    async def list_runs(
        request: Request, limit: int = Query(default=50, ge=1, le=200)
    ) -> RunListResponse:
        states = await current_manager(request).list_runs(limit=limit)
        return RunListResponse(runs=[RunResponse(**state.to_dict()) for state in states])

    @app.get(
        "/runs/{run_id}",
        response_model=RunResponse,
        dependencies=[Depends(require_token)],
    )
    async def get_run(run_id: int, request: Request) -> RunResponse:
        state = await current_manager(request).status(run_id)
        return RunResponse(**state.to_dict())

    @app.post(
        "/runs/{run_id}/stop",
        response_model=RunResponse,
        dependencies=[Depends(require_token)],
    )
    async def stop_run(run_id: int, request: Request) -> RunResponse:
        state = await current_manager(request).stop(run_id)
        return RunResponse(**state.to_dict())

    # -- recurring tasks -----------------------------------------------------
    # Static /tasks/* routes are declared before /tasks/{task_id} so "status",
    # "start", "stop" and "run" are never parsed as task ids.

    @app.get(
        "/tasks",
        response_model=TaskListResponse,
        dependencies=[Depends(require_token)],
    )
    async def list_tasks(request: Request) -> TaskListResponse:
        tasks = await current_manager(request).list_tasks()
        return TaskListResponse(tasks=[TaskResponse(**task) for task in tasks])

    @app.post(
        "/tasks",
        response_model=TaskResponse,
        status_code=status.HTTP_201_CREATED,
        dependencies=[Depends(require_token)],
    )
    async def create_task(body: CreateTaskRequest, request: Request) -> TaskResponse:
        state = await current_manager(request).create_task(
            store=body.store,
            name=body.name,
            schedule=_task_schedule(body.schedule),
            max_runs=body.maxRuns,
            enabled=body.enabled,
            run_immediately=body.runImmediately,
        )
        return TaskResponse(**state)

    @app.get(
        "/tasks/status",
        response_model=TaskControlResponse,
        dependencies=[Depends(require_token)],
    )
    async def tasks_status(request: Request) -> TaskControlResponse:
        return TaskControlResponse(**await current_manager(request).tasks_status())

    @app.post(
        "/tasks/start",
        response_model=TaskControlResponse,
        dependencies=[Depends(require_token)],
    )
    async def start_all_tasks(request: Request) -> TaskControlResponse:
        return TaskControlResponse(**await current_manager(request).start_all())

    @app.post(
        "/tasks/stop",
        response_model=TaskControlResponse,
        dependencies=[Depends(require_token)],
    )
    async def stop_all_tasks(request: Request) -> TaskControlResponse:
        return TaskControlResponse(**await current_manager(request).stop_all())

    @app.post(
        "/tasks/run",
        response_model=TaskControlResponse,
        dependencies=[Depends(require_token)],
    )
    async def run_all_tasks(request: Request) -> TaskControlResponse:
        return TaskControlResponse(**await current_manager(request).run_all())

    @app.get(
        "/tasks/{task_id}",
        response_model=TaskResponse,
        dependencies=[Depends(require_token)],
    )
    async def get_task(task_id: str, request: Request) -> TaskResponse:
        return TaskResponse(**await current_manager(request).get_task(task_id))

    @app.patch(
        "/tasks/{task_id}",
        response_model=TaskResponse,
        dependencies=[Depends(require_token)],
    )
    async def update_task(
        task_id: str, body: UpdateTaskRequest, request: Request
    ) -> TaskResponse:
        schedule = _task_schedule(body.schedule) if body.schedule is not None else None
        state = await current_manager(request).update_task(
            task_id,
            name=body.name,
            enabled=body.enabled,
            schedule=schedule,
            max_runs=body.maxRuns,
            set_max_runs="maxRuns" in body.model_fields_set,
        )
        return TaskResponse(**state)

    @app.delete(
        "/tasks/{task_id}",
        status_code=status.HTTP_204_NO_CONTENT,
        dependencies=[Depends(require_token)],
    )
    async def delete_task(task_id: str, request: Request) -> Response:
        await current_manager(request).delete_task(task_id)
        return Response(status_code=status.HTTP_204_NO_CONTENT)

    @app.post(
        "/tasks/{task_id}/run",
        response_model=RunResponse,
        status_code=status.HTTP_202_ACCEPTED,
        dependencies=[Depends(require_token)],
    )
    async def run_task(task_id: str, request: Request) -> RunResponse:
        state = await current_manager(request).run_task(task_id)
        return RunResponse(**state.to_dict())

    @app.post(
        "/tasks/{task_id}/stop",
        response_model=TaskResponse,
        dependencies=[Depends(require_token)],
    )
    async def stop_task(task_id: str, request: Request) -> TaskResponse:
        return TaskResponse(**await current_manager(request).stop_task(task_id))

    return app


def _task_schedule(body: TaskScheduleRequest) -> TaskSchedule:
    return build_schedule(
        body.kind,
        every=body.every,
        every_seconds=body.everySeconds,
    )


async def _to_thread(func, *args, **kwargs):  # type: ignore[no-untyped-def]
    import asyncio

    return await asyncio.to_thread(func, *args, **kwargs)
