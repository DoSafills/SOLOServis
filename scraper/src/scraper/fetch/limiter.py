"""Async rate limiter shared by every request in a run."""

from __future__ import annotations

import asyncio
import time


class RateLimiter:
    """Serialize request starts so that at most one starts per ``interval``.

    Concurrency is still allowed: the limiter only spaces out the *start* of
    each request, which is what a storefront rate limit measures. The lock is
    held while waiting, so two coroutines can never observe the same slot.
    """

    def __init__(self, interval: float) -> None:
        self.interval = max(0.0, interval)
        self._lock = asyncio.Lock()
        self._next_at = 0.0

    async def __aenter__(self) -> RateLimiter:
        await self.acquire()
        return self

    async def __aexit__(self, *exc_info: object) -> None:
        return None

    async def acquire(self) -> None:
        if self.interval <= 0:
            return
        async with self._lock:
            now = time.monotonic()
            wait = self._next_at - now
            if wait > 0:
                await asyncio.sleep(wait)
                now = time.monotonic()
            self._next_at = now + self.interval
