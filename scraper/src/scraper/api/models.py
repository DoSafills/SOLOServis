"""Request and response bodies of the control API.

``extra="forbid"`` on the request means a typo (``"stroe"``) is a 422 instead
of a silently ignored field that starts the wrong crawl.
"""

from __future__ import annotations

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
