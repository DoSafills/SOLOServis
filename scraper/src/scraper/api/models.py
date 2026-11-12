"""Request and response bodies of the control API.

``extra="forbid"`` on the request means a typo (``"stroe"``) is a 422 instead
of a silently ignored field that starts the wrong crawl.
"""

from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, ConfigDict, Field


class StartRunRequest(BaseModel):
    model_config = ConfigDict(extra="forbid")

    store: str | None = Field(default=None, description="Store slug, e.g. falabella-cl")
    url: str | None = Field(default=None, description="A URL that a store spec claims")


class RunResponse(BaseModel):
    runId: int
    status: str
    store: str
    sourceUrl: str
    startedAt: str | None = None
    finishedAt: str | None = None
    durationMs: int | None = None
    counters: dict[str, int]
    errors: list[str]
    taskId: str | None = None
    trigger: str | None = None


class RunListResponse(BaseModel):
    runs: list[RunResponse]


class StoreResponse(BaseModel):
    slug: str
    name: str
    storeId: int
    baseUrl: str
    listingUrl: str
    digest: str


class StoreListResponse(BaseModel):
    stores: list[StoreResponse]


class HealthResponse(BaseModel):
    status: str


class ReadyResponse(BaseModel):
    status: str
    database: str
    stores: int
    ingestion: str = "unknown"


# ---------------------------------------------------------------------------
# Recurring tasks
# ---------------------------------------------------------------------------


class TaskScheduleRequest(BaseModel):
    """``{"kind": "interval", "every": "1h"}`` or ``{"kind": "manual"}``."""

    model_config = ConfigDict(extra="forbid")

    kind: Literal["interval", "manual"] = "interval"
    every: str | int | None = Field(
        default=None, description="Duration such as '30s', '5m', '1h' or 3600"
    )
    everySeconds: int | None = Field(
        default=None, ge=1, description="Alternative to 'every', in seconds"
    )


class CreateTaskRequest(BaseModel):
    model_config = ConfigDict(extra="forbid")

    store: str = Field(min_length=1, description="Store slug, e.g. ripley-cl")
    name: str | None = None
    enabled: bool = True
    schedule: TaskScheduleRequest = Field(default_factory=TaskScheduleRequest)
    maxRuns: int | None = Field(default=None, ge=1)
    runImmediately: bool = False


class UpdateTaskRequest(BaseModel):
    model_config = ConfigDict(extra="forbid")

    name: str | None = None
    enabled: bool | None = None
    schedule: TaskScheduleRequest | None = None
    maxRuns: int | None = Field(default=None, ge=1)


class TaskScheduleResponse(BaseModel):
    kind: str
    everySeconds: int | None = None


class TaskResponse(BaseModel):
    id: str
    store: str
    name: str
    enabled: bool
    schedule: TaskScheduleResponse
    maxRuns: int | None = None
    createdAt: str
    updatedAt: str
    lastRunId: int | None = None
    lastRunAt: str | None = None
    lastStatus: str | None = None
    nextRunAt: str | None = None
    runCount: int
    consecutiveFailures: int
    runIds: list[int]
    running: bool
    activeRunId: int | None = None


class TaskListResponse(BaseModel):
    tasks: list[TaskResponse]


class TaskControlResponse(BaseModel):
    paused: bool
    tasks: int
    enabledTasks: int
    activeRuns: int
    activeTaskIds: list[str]
    triggeredRunIds: list[int]
    skippedTaskIds: list[str]
