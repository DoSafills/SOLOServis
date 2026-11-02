"""Crawler: URL discovery, scheduling, deduplication and concurrency control.

The crawler owns the ``asyncio.Queue`` worker pool, per-domain rate limiting,
depth/domain restrictions and retry scheduling. Extraction is delegated to the
scraper and persistence to :mod:`crawler.database`.
"""

import asyncio
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any
from urllib.parse import urlparse

import config as cfg
from database import (
    create_scrape_run,
    finish_scrape_run,
    save_scraped_data,
    set_ingestion_run,
    update_run_stats,
)
from ingestion import IngestionClient, IngestionError
from models import (
    RUN_STATUS_FAILED,
    RUN_STATUS_SUCCESS,
    DataSource,
    FetchOutcome,
    ScraperConfig,
    ScrapeResult,
)
from normalized import BundleExtractor, BundleSpec, NormalizedItem, export_jsonl
from processor import process_links
from scraper import Scraper
from utils import get_logger, parse_rate_limit, same_site

_retry_log = get_logger("RETRY")
_ingest_log = get_logger("INGEST")


@dataclass(slots=True)
class CrawlSummary:
    run_id: int
    pages_found: int = 0
    pages_processed: int = 0
    pages_failed: int = 0
    status: str = RUN_STATUS_SUCCESS
    error_message: str | None = None
    visited_urls: list[str] = field(default_factory=list)
    # Contadores de ítems, distintos de los de páginas: una tarjeta puede
    # producir N productos.
    items_found: int = 0
    items_sent: int = 0
    items_failed: int = 0
    items_rejected: int = 0
    ingestion_run_id: int | None = None
    ingestion_error: str | None = None


class RateLimiter:
    """Async minimum-interval limiter built from a ``"<count>/<unit>"`` string."""

    def __init__(self, rate_limit: str | None):
        self.interval = parse_rate_limit(rate_limit or cfg.DEFAULT_RATE_LIMIT)
        self._last = 0.0
        self._lock = asyncio.Lock()

    async def acquire(self) -> None:
        if self.interval <= 0:
            return
        async with self._lock:
            now = time.monotonic()
            delay = self._last + self.interval - now
            if delay > 0:
                await asyncio.sleep(delay)
            self._last = time.monotonic()


class _DomainRateLimiters:

    def __init__(self, default_rate_limit: str):
        self._default = default_rate_limit
        self._limiters: dict[str, RateLimiter] = {}

    def for_url(self, url: str) -> RateLimiter:
        try:
            domain = (urlparse(url).hostname or "").lower()
        except ValueError:
            domain = ""
        limiter = self._limiters.get(domain)
        if limiter is None:
            limiter = RateLimiter(self._default)
            self._limiters[domain] = limiter
        return limiter


def bundle_from_config(parser_config: dict | None) -> BundleSpec | None:
    """Build the item contract from ``parser_config["bundle"]``, if declared.

    Page-level keys (``entity_type``, ``external_id_selector``) stay untouched;
    the bundle lives one level below so both consumers can share one config.
    """
    raw = (parser_config or {}).get("bundle")
    if not raw:
        return None
    return BundleSpec.from_parser_config(raw)


class Crawler:
    """Crawls one (DATA_SOURCE, SCRAPER_CONFIG) pair inside a SCRAPE_RUN."""

    def __init__(
        self,
        source: DataSource,
        config: ScraperConfig,
        run_id: int,
        db_path: str = cfg.DB_PATH,
        scraper: Scraper | None = None,
        max_depth: int | None = None,
        max_pages: int | None = None,
        concurrency: int | None = None,
        ingestion: IngestionClient | None = None,
        rejects_dir: str | None = None,
        bundle: BundleSpec | None = None,
    ):
        self.source = source
        self.config = config
        self.run_id = run_id
        self.db_path = db_path
        self.scraper = scraper or Scraper(config.parser_config)
        self.max_depth = cfg.MAX_DEPTH if max_depth is None else max_depth
        self.max_pages = cfg.MAX_PAGES_PER_RUN if max_pages is None else max_pages
        self.concurrency = min(cfg.MAX_CONCURRENCY, concurrency
                               or cfg.MAX_CONCURRENCY)

        # Ingestion (flujo B). Sin bundle no hay ítems que enviar; sin tienda o
        # sin token la integración queda desactivada y el crawl sigue local.
        self.bundle = bundle if bundle is not None else bundle_from_config(
            config.parser_config)
        self.ingestion = ingestion
        self.rejects_dir = rejects_dir or cfg.REJECTS_DIR
        self.ingestion_run_id: int | None = None
        self.ingestion_error: str | None = None
        # Solo se envía cuando hay cliente, tienda y ejecución remota abierta.
        self._ingesting = False
        self.items_found = 0
        self.items_sent = 0
        self.items_failed = 0
        self.items_rejected = 0
        self._buffer: list[NormalizedItem] = []
        self._seen_items: set[str] = set()
        self._ingest_lock = asyncio.Lock()
        self._reject_lock = asyncio.Lock()
        self._bundle_extractor = (BundleExtractor(
            self.bundle,
            source=None,
            run_id=run_id,
            config_id=config.id,
        ) if self.bundle else None)

        self.queue: asyncio.Queue[tuple[str, int]] = asyncio.Queue()
        self.seen: set[str] = set()
        self.visited: set[str] = set()
        self.limiters = _DomainRateLimiters(config.rate_limit
                                            or cfg.DEFAULT_RATE_LIMIT)

        self._active = 0
        self._idle = asyncio.Event()
        self._idle.set()
        self._stopping = False
        self._workers: list[asyncio.Task] = []
        self.records_found = 0
        self.records_processed = 0
        self.records_failed = 0
        self.errors: list[str] = []

    # -- scheduling ----------------------------------------------------------

    def _submit(self, url: str, depth: int) -> bool:
        normalized = self.seen  # readability
        if url in normalized:
            return False
        if len(self.seen) >= self.max_pages:
            return False
        self.seen.add(url)
        self._active += 1
        self._idle.clear()
        self.queue.put_nowait((url, depth))
        self.records_found = len(self.seen)
        return True

    async def _worker(self) -> None:
        while True:
            url, depth = await self.queue.get()
            try:
                await self._process(url, depth)
            finally:
                self.queue.task_done()
                self._active -= 1
                if self._active == 0:
                    self._idle.set()

    async def _process(self, url: str, depth: int) -> None:
        await self.limiters.for_url(url).acquire()

        try:
            outcome = await self._fetch_with_retries(url)

            result: ScrapeResult | None = None
            if outcome.ok:
                result = await asyncio.to_thread(self.scraper.parse, outcome)

            if result is None:
                error = outcome.error or "unparseable response"
                self.records_failed += 1
                self.errors.append(f"{url}: {error}")
                return

            result.links = process_links(result.links, result.external_url)
            await asyncio.to_thread(save_scraped_data, self.db_path,
                                    self.run_id, result)
            self.visited.add(url)
            self.records_processed += 1

            # Flujo B: la misma página que se archiva como HTML produce los
            # ítems normalizados que viajan a Ingestion.
            await self._ingest_page(outcome.document, result.external_url)

            if depth < self.max_depth:
                base = self.source.base_url
                for link in result.links:
                    if len(self.seen) >= self.max_pages:
                        break
                    if not same_site(link, base):
                        continue
                    self._submit(link, depth + 1)
        except Exception as exc:  # never let one page kill the worker
            self.records_failed += 1
            self.errors.append(f"{url}: {type(exc).__name__}: {exc}")
        finally:
            await self._persist_stats()

    # -- ingestion (flujo B) --------------------------------------------------

    async def _ingest_page(self, document: Any, page_url: str) -> None:
        """Extract, dedupe, filter and buffer the items of one page."""
        if self._bundle_extractor is None or document is None:
            return

        try:
            items = await asyncio.to_thread(
                self._bundle_extractor.extract_all, document, page_url)
        except Exception as exc:  # a bad selector must not kill the page
            self.errors.append(
                f"{page_url}: item extraction failed: {type(exc).__name__}: {exc}")
            _ingest_log.error("run #%d: item extraction failed for %s: %s",
                              self.run_id, page_url, exc)
            return

        valid: list[NormalizedItem] = []
        rejected: list[NormalizedItem] = []
        for item in items:
            key = item.external_id or item.external_url
            if key in self._seen_items:
                continue  # Falabella repite pods en el mismo listado
            self._seen_items.add(key)
            self.items_found += 1

            problems = item.validate()
            if problems:
                item.provenance.errors.extend(
                    f"validation.{problem.field}: {problem.message}"
                    for problem in problems)
                rejected.append(item)
            else:
                valid.append(item)

        if rejected:
            self.items_rejected += len(rejected)
            await self._record_rejects(rejected, "invalid item")

        # Sin ejecución remota no hay a quién entregar: los ítems válidos
        # quedan en el HTML archivado y en la base local.
        if valid and self._ingesting:
            async with self._ingest_lock:
                self._buffer.extend(valid)
                await self._drain_locked(force=False)

    async def _drain_locked(self, *, force: bool) -> None:
        """Send full batches; with ``force``, send whatever is buffered."""
        size = self.ingestion.batch_size if self.ingestion else 100
        while self._buffer and (force or len(self._buffer) >= size):
            batch = self._buffer[:size]
            del self._buffer[:size]
            await self._send_batch(batch)

    async def _send_batch(self, batch: list[NormalizedItem]) -> None:
        # Un fallo previo corta los envíos: reintentar cada lote contra un
        # servicio caído solo alarga el crawl. Lo que llegue después se
        # conserva como rechazado para replay.
        if (self.ingestion is None or self.ingestion_run_id is None
                or self.ingestion_error):
            reason = self.ingestion_error or "ingestion disabled"
            self.items_failed += len(batch)
            await self._record_rejects(batch, reason)
            return

        try:
            result = await self.ingestion.send_items(
                self.ingestion_run_id,
                [item.to_ingestion() for item in batch],
            )
        except IngestionError as exc:
            self.ingestion_error = str(exc)
            self.items_failed += len(batch)
            await self._record_rejects(batch, f"ingestion: {exc}")
            _ingest_log.error(
                "run #%d: batch of %d item(s) could not be sent: %s",
                self.run_id, len(batch), exc)
            return

        self.items_sent += result.processed
        self.items_failed += result.failed
        if not result.failed:
            return

        failed: list[NormalizedItem] = []
        for outcome in result.results:
            if outcome.status != "failed":
                continue
            if 0 <= outcome.index < len(batch):
                item = batch[outcome.index]
                item.provenance.errors.extend(
                    f"ingestion.{error.get('field', '')}: "
                    f"{error.get('message', '')}"
                    for error in outcome.errors)
                failed.append(item)
        if failed:
            await self._record_rejects(failed, "rejected by ingestion")

    async def _record_rejects(
        self, items: list[NormalizedItem], reason: str,
    ) -> None:
        """Append rejected items to a per-run JSONL for review and replay."""
        if not items:
            return
        for item in items:
            item.provenance.errors.append(f"rejected: {reason}")
        path = Path(self.rejects_dir) / f"run-{self.run_id}.jsonl"
        try:
            async with self._reject_lock:
                await asyncio.to_thread(export_jsonl, items, path)
        except OSError as exc:
            _ingest_log.error("run #%d: could not write %s: %s",
                              self.run_id, path, exc)

    async def _open_ingestion(self) -> None:
        if self._bundle_extractor is None:
            return
        if self.ingestion is None:
            _ingest_log.info(
                "run #%d: ingestion disabled; extracted items stay local",
                self.run_id)
            return
        if not self.source.store_id:
            self.ingestion_error = (
                "source has no store_id; items stay local")
            _ingest_log.warning("run #%d: %s", self.run_id, self.ingestion_error)
            return

        try:
            run = await self.ingestion.open_run(
                self.source.name,
                self.source.base_url,
                self.source.store_id,
                self.config.scraper_type or "http",
            )
        except IngestionError as exc:
            self.ingestion_error = f"could not open ingestion run: {exc}"
            _ingest_log.error("run #%d: %s", self.run_id, self.ingestion_error)
            return

        self.ingestion_run_id = run.id
        self._ingesting = True
        await asyncio.to_thread(
            set_ingestion_run, self.db_path, self.run_id, run.id)
        _ingest_log.info("run #%d linked to ingestion run #%d (store %d)",
                         self.run_id, run.id, self.source.store_id)

    async def _finish_ingestion(
        self, status: str, error_message: str | None,
    ) -> None:
        if self.ingestion is None or self.ingestion_run_id is None:
            return
        remote_status = (
            "success" if status == RUN_STATUS_SUCCESS and not self.ingestion_error
            else "failed")
        reason = self.ingestion_error or error_message
        try:
            await self.ingestion.finish_run(
                self.ingestion_run_id, remote_status, reason)
        except IngestionError as exc:
            self.ingestion_error = self.ingestion_error or (
                f"could not finish ingestion run: {exc}")
            _ingest_log.error("run #%d: could not finish ingestion run #%d: %s",
                              self.run_id, self.ingestion_run_id, exc)
        finally:
            self._ingesting = False

    async def abort_ingestion(self, message: str) -> None:
        """Close an in-flight ingestion run as ``failed`` after an abort.

        Buffered items are written to the rejects file instead of being sent:
        the crawl is being cancelled, not drained.
        """
        if self._buffer:
            await self._record_rejects(self._buffer, message)
            self._buffer.clear()
        if self.ingestion is None or self.ingestion_run_id is None:
            return
        try:
            await self.ingestion.finish_run(
                self.ingestion_run_id, "failed", message)
        except IngestionError as exc:
            self.ingestion_error = f"could not finish ingestion run: {exc}"
            _ingest_log.error("run #%d: %s", self.run_id, self.ingestion_error)
        finally:
            self._ingesting = False

    async def _fetch_with_retries(self, url: str) -> FetchOutcome:
        last_outcome = FetchOutcome(error="not attempted")
        for attempt in range(max(1, cfg.MAX_RETRIES)):
            outcome = await self.scraper.fetch(url)
            if outcome.ok:
                return outcome

            transient = outcome.status in cfg.TRANSIENT_STATUS_CODES or outcome.status is None
            last_outcome = outcome
            if transient and attempt < cfg.MAX_RETRIES - 1:
                delay = cfg.BACKOFF_BASE_SECONDS * (2**attempt)
                reason = outcome.error or f"HTTP {outcome.status}"
                _retry_log.warning(
                    "attempt %d/%d failed for %s (%s); retrying in %.1fs",
                    attempt + 1, cfg.MAX_RETRIES, url, reason, delay,
                )
                await asyncio.sleep(delay)
            else:
                break
        return last_outcome

    async def _persist_stats(self) -> None:
        await asyncio.to_thread(
            update_run_stats,
            self.db_path,
            self.run_id,
            records_found=self.records_found,
            records_processed=self.records_processed,
            records_failed=self.records_failed,
            items_found=self.items_found,
            items_sent=self.items_sent,
            items_failed=self.items_failed,
            items_rejected=self.items_rejected,
        )

    # -- public API ------------------------------------------------------------

    async def run(self) -> CrawlSummary:
        """Run the crawl and finalize its SCRAPE_RUN row.

        Raises :class:`asyncio.CancelledError` *without* touching the SCRAPE_RUN
        row when :meth:`aclose` was called while the crawl was still draining.
        An aborted crawl never finalizes itself: it re-raises so the caller owns
        the final state (see ``main._finalize_interrupted``).
        """
        seeds = [self.source.base_url]
        for seed in seeds:
            self._submit(seed, depth=0)

        # La ejecución remota se abre antes de la primera página: si Ingestion
        # no responde, el crawl local sigue y el motivo queda registrado.
        await self._open_ingestion()

        self._workers = [
            asyncio.create_task(self._worker(), name=f"crawler-worker-{i}")
            for i in range(self.concurrency)
        ]

        await self._idle.wait()
        await self._reap_workers()

        if self._stopping:
            # Workers were cancelled from the outside: this is an abort, not a
            # graceful drain. Cancelling workers also trips `_idle`, so without
            # this check the run would be finalized as a success even though
            # most of its pages were never visited.
            raise asyncio.CancelledError

        # Enviar lo que quedó en el buffer antes de cerrar la ejecución.
        async with self._ingest_lock:
            await self._drain_locked(force=True)

        status = RUN_STATUS_SUCCESS
        error_message: str | None = None
        if self.records_failed > 0 and self.records_processed == 0:
            status = RUN_STATUS_FAILED
            error_message = "; ".join(self.errors[:10]) or None
        elif self.errors:
            error_message = "; ".join(self.errors[:10])

        await self._finish_ingestion(status, error_message)

        summary = CrawlSummary(
            run_id=self.run_id,
            pages_found=self.records_found,
            pages_processed=self.records_processed,
            pages_failed=self.records_failed,
            status=status,
            error_message=error_message,
            visited_urls=sorted(self.visited),
            items_found=self.items_found,
            items_sent=self.items_sent,
            items_failed=self.items_failed,
            items_rejected=self.items_rejected,
            ingestion_run_id=self.ingestion_run_id,
            ingestion_error=self.ingestion_error,
        )

        await asyncio.to_thread(
            finish_scrape_run,
            self.db_path,
            self.run_id,
            summary.status,
            error_message,
            summary.pages_found,
            summary.pages_processed,
            summary.pages_failed,
            items_found=summary.items_found,
            items_sent=summary.items_sent,
            items_failed=summary.items_failed,
            items_rejected=summary.items_rejected,
            ingestion_run_id=summary.ingestion_run_id,
            ingestion_error=summary.ingestion_error,
        )
        return summary

    async def _reap_workers(self) -> None:
        """Cancel and reap every worker task; safe to call multiple times."""
        workers, self._workers = self._workers, []
        for worker in workers:
            if not worker.done():
                worker.cancel()
        if workers:
            await asyncio.gather(*workers, return_exceptions=True)

    async def aclose(self) -> None:
        """Abort the crawl and tear down the worker pool.

        Marks the crawler as stopping *before* cancelling the workers, so
        :meth:`run` can tell an abort apart from a graceful drain. Idempotent.
        """
        self._stopping = True
        await self._reap_workers()


def start_run(
    source: DataSource,
    config: ScraperConfig,
    db_path: str = cfg.DB_PATH,
    ingestion: IngestionClient | None = None,
) -> Crawler:
    """Create a SCRAPE_RUN row and bind it to a fresh crawler instance."""
    run_id = create_scrape_run(db_path, config.id)
    return Crawler(source, config, run_id, db_path=db_path, ingestion=ingestion)
