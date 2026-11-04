"""The crawl pipeline: listing, discovery, detail pages and persistence.

One call to :func:`execute` owns exactly one ``SCRAPE_RUN`` row. The listing is
still the entry point, but a spec may now declare:

* ``match.detect`` — a page must still look like the store;
* ``crawl`` — which links may be followed, and how deep;
* ``detail`` — which fields to enrich from a product page.

Discovery is never generic: the crawler only follows ``crawl.linkSelector``
matches and the ``externalUrl`` of items the spec already extracted. Every URL
is re-checked against ``spec.match`` before it enters the frontier. A stop
request drains the page in flight and finalizes the run as ``failed`` with
``stopped by request``, exactly like the listing-only path.
"""

from __future__ import annotations

import asyncio
from collections import deque
from dataclasses import dataclass, field
from datetime import UTC, datetime
from typing import Any
from urllib.parse import urlparse

from scrapling import Selector

from scraper.config import Settings
from scraper.db import repository as repo
from scraper.errors import EmptyListingError, ExtractionError, IngestionError
from scraper.extract.engine import (
    RejectedItem,
    apply_detail,
    extract_detail,
    extract_page,
)
from scraper.extract.transforms import normalize_url
from scraper.fetch.client import Fetcher
from scraper.ingestion import IngestionClient
from scraper.models import (
    RUN_STATUS_FAILED,
    RUN_STATUS_SUCCESS,
    Counters,
    CrawlRules,
    DataSource,
    Item,
    ScraperConfig,
    Source,
    StoreSpec,
)
from scraper.output.catalog import CatalogResult, write_catalog
from scraper.output.raw import save_page_html, save_rejects, save_run_snapshot


@dataclass(slots=True)
class PipelineResult:
    run_id: int
    status: str
    counters: Counters
    catalog: CatalogResult | None = None
    error: str | None = None
    ingestion_run_id: int | None = None
    ingestion_error: str | None = None
    rejects: int = 0
    warnings: list[str] = field(default_factory=list)


async def execute(
    settings: Settings,
    *,
    spec: StoreSpec,
    source: DataSource,
    config: ScraperConfig,
    run_id: int,
    stop_event: asyncio.Event | None = None,
    fetcher: Fetcher | None = None,
    ingestion: IngestionClient | None = None,
    task_id: str | None = None,
) -> PipelineResult:
    """Run one crawl to completion (or to a controlled stop)."""
    stop = stop_event or asyncio.Event()
    fetcher = fetcher or Fetcher(settings)
    store_source = Source(name=spec.name, base_url=spec.base_url, store_id=spec.store_id)
    counters = Counters()
    items: dict[str, Item] = {}
    items_by_id: dict[str, Item] = {}
    items_by_url: dict[str, Item] = {}
    rejects: list[RejectedItem] = []
    warnings: list[str] = []
    error: str | None = None

    previous = await _previous_items(settings, config.id)
    max_pages = min(spec.listing.pagination.max_pages, settings.max_pages)
    url: str | None = spec.listing.url
    page_urls: set[str] = set()
    fetched_urls: set[str] = set()
    frontier: deque[tuple[str, int]] = deque()
    queued: set[str] = set()
    page_index = 0
    saved_pages = 0
    started_at = _now_iso()

    # -- phase 1: the listing and its declared pagination --------------------
    while url and page_index < max_pages:
        if stop.is_set():
            error = "stopped by request"
            break
        page_index += 1
        page_urls.add(url)
        fetched_urls.add(url)
        counters.pages_found += 1

        fetched = await fetcher.fetch(url)
        if not fetched.ok:
            counters.pages_failed += 1
            error = f"page {url} failed: {fetched.error or f'HTTP {fetched.status}'}"
            break

        try:
            detect_ok = _detect_ok(spec, fetched.body)
        except ExtractionError as exc:
            counters.pages_failed += 1
            error = f"page {url}: {exc}"
            break
        if not detect_ok:
            counters.pages_failed += 1
            error = (
                f"page {url} did not match match.detect "
                f"({spec.match.detect.selector!r}); the spec is out of date or the "
                "page is not the store"
            )
            break

        saved_pages += 1
        await asyncio.to_thread(
            repo.save_page, settings.db_path, run_id, url=url, html=fetched.body
        )
        await asyncio.to_thread(
            save_page_html, settings.runs_dir / str(run_id), saved_pages, url, fetched.body
        )

        try:
            result = extract_page(
                spec,
                fetched.body,
                page_url=url,
                source=store_source,
                run_id=run_id,
                config_id=config.id,
                scraped_at=_now_iso(),
            )
        except ExtractionError as exc:
            counters.pages_failed += 1
            error = f"page {url}: {exc}"
            break

        counters.pages_processed += 1
        rejects.extend(result.rejected)
        warnings.extend(result.warnings)

        if result.cards == 0:
            if page_index == 1 and not spec.listing.allow_empty:
                counters.pages_failed += 1
                error = str(
                    EmptyListingError(
                        f"{spec.slug}: the first listing page produced no cards; "
                        f"check 'listing.root' ({spec.listing.root!r}) against the live page"
                    )
                )
            break

        new_items = _dedupe(result.items, items, spec.dedupe_by)
        _index_items(new_items, items_by_id, items_by_url)
        if new_items:
            await asyncio.to_thread(repo.save_items, settings.db_path, run_id, new_items)
        counters.items_found = len(items)
        await asyncio.to_thread(
            repo.update_counters, settings.db_path, run_id, counters=counters
        )

        if page_index == 1 and not items and result.rejected:
            counters.pages_failed += 1
            error = (
                f"every card on the first page was rejected ({len(result.rejected)}); "
                "the spec is out of date"
            )
            break

        _enqueue_detail_urls(
            spec, new_items, frontier, queued, fetched_urls, warnings
        )
        if spec.crawl is not None:
            _discover_links(
                spec,
                spec.crawl,
                fetched.body,
                url,
                depth=0,
                frontier=frontier,
                queued=queued,
                fetched_urls=fetched_urls,
                warnings=warnings,
            )

        url = _next_url(spec, fetched.body, url, page_index, page_urls)
        if url is None:
            break

    # -- phase 2: declared discovery and detail pages ------------------------
    crawl_complete = True
    if error is None and (spec.crawl is not None or spec.detail is not None):
        error, crawl_complete = await _crawl(
            settings,
            spec=spec,
            source=store_source,
            config=config,
            run_id=run_id,
            stop=stop,
            fetcher=fetcher,
            counters=counters,
            items=items,
            items_by_id=items_by_id,
            items_by_url=items_by_url,
            rejects=rejects,
            warnings=warnings,
            frontier=frontier,
            queued=queued,
            fetched_urls=fetched_urls,
            saved_pages=saved_pages,
        )

    status = RUN_STATUS_FAILED if error else RUN_STATUS_SUCCESS
    # A truncated crawl means "we did not look at everything", so the catalog
    # must not delete products that are merely absent from this run.
    complete = status == RUN_STATUS_SUCCESS and crawl_complete
    counters.items_found = len(items)

    catalog = await asyncio.to_thread(
        write_catalog,
        settings.catalog_dir,
        store=spec.slug,
        run_id=run_id,
        items=[item.to_dict() for item in items.values()],
        previous=previous,
        complete=complete,
        generated_at=_now_iso(),
    )

    ingestion_run_id: int | None = None
    ingestion_error: str | None = None
    if status == RUN_STATUS_SUCCESS and ingestion is not None and items:
        ingestion_run_id, ingestion_error, sent, failed = await _ingest(
            ingestion, source, config, list(items.values())
        )
        counters.items_sent = sent
        counters.items_failed = failed

    rejects_path = await asyncio.to_thread(
        save_rejects, settings.rejects_dir, run_id, [reject.to_dict() for reject in rejects]
    )
    snapshot = {
        "runId": run_id,
        "store": spec.slug,
        "sourceUrl": spec.listing.url,
        "startedAt": started_at,
        "finishedAt": _now_iso(),
        "status": status,
        "spec": spec.snapshot(),
        "configId": config.id,
        "sourceId": source.id,
        "storeId": spec.store_id,
        "taskId": task_id,
        "counters": counters.to_dict(),
        "catalog": catalog.to_dict(),
        "rejects": len(rejects),
        "rejectsFile": str(rejects_path) if rejects_path else None,
        "warnings": warnings,
        "error": error,
        "ingestionRunId": ingestion_run_id,
        "ingestionError": ingestion_error,
    }
    await asyncio.to_thread(save_run_snapshot, settings.runs_dir / str(run_id), snapshot)
    await asyncio.to_thread(
        repo.finish_run,
        settings.db_path,
        run_id,
        status,
        error_message=error,
        counters=counters,
        ingestion_run_id=ingestion_run_id,
        ingestion_error=ingestion_error,
    )

    return PipelineResult(
        run_id=run_id,
        status=status,
        counters=counters,
        catalog=catalog,
        error=error,
        ingestion_run_id=ingestion_run_id,
        ingestion_error=ingestion_error,
        rejects=len(rejects),
        warnings=warnings,
    )


async def _crawl(
    settings: Settings,
    *,
    spec: StoreSpec,
    source: Source,
    config: ScraperConfig,
    run_id: int,
    stop: asyncio.Event,
    fetcher: Fetcher,
    counters: Counters,
    items: dict[str, Item],
    items_by_id: dict[str, Item],
    items_by_url: dict[str, Item],
    rejects: list[RejectedItem],
    warnings: list[str],
    frontier: deque[tuple[str, int]],
    queued: set[str],
    fetched_urls: set[str],
    saved_pages: int,
) -> tuple[str | None, bool]:
    """Walk the declared frontier.

    Returns ``(error, complete)``. ``error`` is set only for a stop; ``complete``
    is false when a page failed or the frontier was truncated, which tells the
    catalog not to delete anything.
    """
    crawl = spec.crawl
    max_pages = min(
        crawl.max_pages if crawl is not None else settings.max_crawl_pages,
        settings.max_crawl_pages,
    )
    batch_size = max(1, settings.max_concurrency)
    crawled = 0
    complete = True

    while frontier and crawled < max_pages:
        if stop.is_set():
            return "stopped by request", False

        batch: list[tuple[str, int]] = []
        while frontier and len(batch) < batch_size and crawled + len(batch) < max_pages:
            candidate, depth = frontier.popleft()
            queued.discard(candidate)
            if candidate in fetched_urls:
                continue
            fetched_urls.add(candidate)
            batch.append((candidate, depth))
        if not batch:
            break

        fetched_batch = await asyncio.gather(
            *(fetcher.fetch(candidate) for candidate, _ in batch)
        )
        detail_pages: list[tuple[str, str]] = []
        for (candidate, depth), fetched in zip(batch, fetched_batch, strict=True):
            if stop.is_set():
                return "stopped by request", False
            crawled += 1
            counters.pages_found += 1
            if not fetched.ok:
                complete = False
                counters.pages_failed += 1
                warnings.append(
                    f"crawl page {candidate} failed: "
                    f"{fetched.error or f'HTTP {fetched.status}'}"
                )
                continue

            try:
                detect_ok = _detect_ok(spec, fetched.body)
            except ExtractionError as exc:
                complete = False
                counters.pages_failed += 1
                warnings.append(f"crawl page {candidate}: {exc}")
                continue
            if not detect_ok:
                complete = False
                counters.pages_failed += 1
                warnings.append(
                    f"crawl page {candidate} did not match match.detect "
                    f"({spec.match.detect.selector!r})"
                )
                continue

            saved_pages += 1
            await asyncio.to_thread(
                repo.save_page, settings.db_path, run_id, url=candidate, html=fetched.body
            )
            await asyncio.to_thread(
                save_page_html,
                settings.runs_dir / str(run_id),
                saved_pages,
                candidate,
                fetched.body,
            )

            # Detail pages are processed after every listing page in the
            # batch, so a item discovered later in the same batch can still be
            # enriched. A detail page is not a source of new items.
            if _is_detail_candidate(spec, candidate, fetched.body, items_by_url):
                counters.pages_processed += 1
                detail_pages.append((candidate, fetched.body))
                continue

            try:
                result = extract_page(
                    spec,
                    fetched.body,
                    page_url=candidate,
                    source=source,
                    run_id=run_id,
                    config_id=config.id,
                    scraped_at=_now_iso(),
                )
            except ExtractionError as exc:
                complete = False
                counters.pages_failed += 1
                warnings.append(f"crawl page {candidate}: {exc}")
                continue

            counters.pages_processed += 1
            rejects.extend(result.rejected)
            warnings.extend(result.warnings)
            new_items = _dedupe(result.items, items, spec.dedupe_by)
            _index_items(new_items, items_by_id, items_by_url)
            if new_items:
                await asyncio.to_thread(
                    repo.save_items, settings.db_path, run_id, new_items
                )
            counters.items_found = len(items)

            _enqueue_detail_urls(
                spec, new_items, frontier, queued, fetched_urls, warnings
            )
            if crawl is not None:
                _discover_links(
                    spec,
                    crawl,
                    fetched.body,
                    candidate,
                    depth=depth,
                    frontier=frontier,
                    queued=queued,
                    fetched_urls=fetched_urls,
                    warnings=warnings,
                )

            await asyncio.to_thread(
                repo.update_counters, settings.db_path, run_id, counters=counters
            )

        for candidate, html in detail_pages:
            if stop.is_set():
                return "stopped by request", False
            await _enrich_from_detail(
                settings,
                run_id,
                spec,
                candidate,
                html,
                items_by_id=items_by_id,
                items_by_url=items_by_url,
                items=items,
                counters=counters,
                warnings=warnings,
            )
            await asyncio.to_thread(
                repo.update_counters, settings.db_path, run_id, counters=counters
            )

    if frontier and crawled >= max_pages:
        complete = False
        warnings.append(
            f"crawl stopped at maxPages={max_pages}; {len(frontier)} link(s) were not visited"
        )
    return None, complete


async def _enrich_from_detail(
    settings: Settings,
    run_id: int,
    spec: StoreSpec,
    page_url: str,
    html: str,
    *,
    items_by_id: dict[str, Item],
    items_by_url: dict[str, Item],
    items: dict[str, Item],
    counters: Counters,
    warnings: list[str],
) -> None:
    detail = spec.detail
    if detail is None:
        return
    if not _is_detail_candidate(spec, page_url, html, items_by_url):
        return
    result = extract_detail(spec, html, page_url=page_url)
    warnings.extend(result.warnings)
    if result.empty:
        return

    target: Item | None = None
    if result.external_id and result.external_id in items_by_id:
        target = items_by_id[result.external_id]
    if target is None:
        target = items_by_url.get(_url_key(page_url))
    if target is None:
        warnings.append(
            f"detail page {page_url} did not match any listing item; values were ignored"
        )
        return

    merge_warnings = apply_detail(target, result.values)
    warnings.extend(merge_warnings)
    if not merge_warnings:
        await asyncio.to_thread(repo.update_item, settings.db_path, run_id, target)
    counters.items_found = len(items)


def _detect_ok(spec: StoreSpec, html: str) -> bool:
    rule = spec.match.detect
    if rule is None:
        return True
    document = Selector(html)
    try:
        matches = document.css(rule.selector)
    except Exception as exc:
        raise ExtractionError(
            f"invalid match.detect selector {rule.selector!r}: {exc}"
        ) from exc
    return len(matches) >= rule.min_matches


def _is_detail_candidate(
    spec: StoreSpec,
    page_url: str,
    html: str,
    items_by_url: dict[str, Item],
) -> bool:
    detail = spec.detail
    if detail is None:
        return False
    if detail.detect:
        try:
            return bool(Selector(html).css(detail.detect))
        except Exception:
            return False
    return _url_key(page_url) in items_by_url


def _enqueue_detail_urls(
    spec: StoreSpec,
    new_items: list[Item],
    frontier: deque[tuple[str, int]],
    queued: set[str],
    fetched_urls: set[str],
    warnings: list[str],
) -> None:
    if spec.detail is None:
        return
    if spec.crawl is not None and not spec.crawl.follow_items:
        return
    max_depth = spec.crawl.max_depth if spec.crawl is not None else 1
    if max_depth < 1:
        return
    listing_host = (urlparse(spec.listing.url).hostname or "").lower()
    for item in new_items:
        candidate = normalize_url(item.external_url)
        if candidate is None:
            continue
        # The item block declared this URL, but the spec still has to own its
        # host. ``allow``/``deny`` are discovery filters and do not apply here.
        if not spec.match.matches(candidate):
            warnings.append(
                f"item {candidate} is not claimed by spec.match; detail page skipped"
            )
            continue
        if spec.crawl is not None and spec.crawl.same_host:
            host = (urlparse(candidate).hostname or "").lower()
            if host != listing_host:
                warnings.append(
                    f"item {candidate} is on another host; "
                    "set crawl.sameHost=false to visit it"
                )
                continue
        _enqueue(candidate, 1, frontier, queued, fetched_urls)


def _discover_links(
    spec: StoreSpec,
    crawl: CrawlRules,
    html: str,
    page_url: str,
    *,
    depth: int,
    frontier: deque[tuple[str, int]],
    queued: set[str],
    fetched_urls: set[str],
    warnings: list[str],
) -> None:
    if not crawl.link_selector or depth >= crawl.max_depth:
        return
    document = Selector(html)
    try:
        elements = document.css(crawl.link_selector)
    except Exception as exc:
        warnings.append(f"invalid crawl.linkSelector {crawl.link_selector!r}: {exc}")
        return
    for element in elements:
        href = element.attrib.get(crawl.link_attribute)
        if not href:
            continue
        candidate = normalize_url(href, page_url)
        if candidate is None:
            continue
        if not _allowed_by_crawl(spec, crawl, candidate):
            continue
        _enqueue(candidate, depth + 1, frontier, queued, fetched_urls)


def _allowed_by_crawl(spec: StoreSpec, crawl: CrawlRules, url: str) -> bool:
    # The spec's match block is the final authority: a discovered link that is
    # not owned by this store is never fetched.
    if not spec.match.matches(url):
        return False
    parsed = urlparse(url)
    host = (parsed.hostname or "").lower()
    if crawl.same_host:
        listing_host = (urlparse(spec.listing.url).hostname or "").lower()
        if host != listing_host:
            return False
    path = parsed.path or "/"
    if crawl.allow and not any(path.startswith(prefix) for prefix in crawl.allow):
        return False
    if crawl.deny and any(path.startswith(prefix) for prefix in crawl.deny):
        return False
    return True


def _enqueue(
    url: str,
    depth: int,
    frontier: deque[tuple[str, int]],
    queued: set[str],
    fetched_urls: set[str],
) -> None:
    if url in fetched_urls or url in queued:
        return
    queued.add(url)
    frontier.append((url, depth))


def _url_key(url: str) -> str:
    return normalize_url(url) or url


def _index_items(
    new_items: list[Item],
    items_by_id: dict[str, Item],
    items_by_url: dict[str, Item],
) -> None:
    for item in new_items:
        if item.external_id:
            items_by_id[item.external_id] = item
        if item.external_url:
            items_by_url[_url_key(item.external_url)] = item


def _dedupe(candidates: list[Item], known: dict[str, Item], dedupe_by: str) -> list[Item]:
    fresh: list[Item] = []
    for item in candidates:
        key = item.external_url if dedupe_by == "externalUrl" else item.key
        if not key or key in known:
            continue
        known[key] = item
        fresh.append(item)
    return fresh


def _next_url(
    spec: StoreSpec,
    html: str,
    current_url: str,
    page_index: int,
    seen_urls: set[str],
) -> str | None:
    pagination = spec.listing.pagination
    if pagination.mode == "none":
        return None
    if pagination.mode == "url-template":
        assert pagination.template is not None
        candidate = pagination.template.replace("{page}", str(page_index + 1))
        return None if candidate in seen_urls else candidate
    if pagination.mode == "next-link":
        assert pagination.selector is not None
        document = Selector(html)
        try:
            elements = document.css(pagination.selector)
        except Exception as exc:
            raise ExtractionError(
                f"invalid pagination selector {pagination.selector!r}: {exc}"
            ) from exc
        if not elements:
            return None
        href = elements[0].attrib.get("href")
        if not href:
            return None
        candidate = normalize_url(href, current_url)
        if candidate is None or candidate in seen_urls or candidate == current_url:
            return None
        return candidate
    return None


async def _previous_items(
    settings: Settings, config_id: int
) -> dict[str, dict[str, Any]]:
    previous_run = await asyncio.to_thread(
        repo.latest_run_for_config, settings.db_path, config_id
    )
    if previous_run is None:
        return {}
    rows = await asyncio.to_thread(repo.run_items, settings.db_path, previous_run)
    return repo.items_by_external_id(rows)


async def _ingest(
    client: IngestionClient,
    source: DataSource,
    config: ScraperConfig,
    items: list[Item],
) -> tuple[int | None, str | None, int, int]:
    try:
        run = await client.open_run(
            source.name, source.base_url, source.store_id or 0, config.scraper_type or "http"
        )
    except IngestionError as exc:
        return None, f"could not open ingestion run: {exc}", 0, len(items)
    try:
        batch = await client.send_items(run.id, items)
    except IngestionError as exc:
        await _finish_ingestion(client, run.id, "failed", str(exc))
        return run.id, f"could not send items: {exc}", 0, len(items)
    try:
        await client.finish_run(run.id, "success")
    except IngestionError as exc:
        return run.id, f"could not finish ingestion run: {exc}", batch.processed, batch.failed
    return run.id, None, batch.processed, batch.failed


async def _finish_ingestion(
    client: IngestionClient, run_id: int, status: str, reason: str
) -> None:
    try:
        await client.finish_run(run_id, status, reason)
    except IngestionError:
        return


def _now_iso() -> str:
    return datetime.now(UTC).isoformat(timespec="seconds").replace("+00:00", "Z")
