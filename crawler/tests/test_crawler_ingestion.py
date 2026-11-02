"""Integración del crawler con Ingestion API (flujo B).

Se usa un cliente de mentira en lugar de HTTP: lo que se verifica aquí no es el
transporte —eso vive en ``test_ingestion.py``— sino la política del crawler:
qué se abre, qué se envía, qué se descarta y cómo queda el ``SCRAPE_RUN`` local
cuando Ingestion falla.
"""

import asyncio
import json
import re

import pytest
from scrapling.parser import Adaptor

import config as cfg
import database as db
from crawler import Crawler, bundle_from_config
from ingestion import BatchResult, IngestionError, ItemResult, RunInfo
from models import DataSource, FetchOutcome, ScrapeResult
from normalized import BundleSpec

BUNDLE = {
    "entity_type": "product",
    "root": ".card",
    "product": {"fields": {"name": ".title"}},
    "offer": {"fields": {"price": ".price", "list_price": ".list"}},
    "images": {
        "fields": {
            "image_url": {
                "selector": "img",
                "mode": "attr",
                "attribute": "src",
            }
        }
    },
    "external_id": {"attribute": "data-id"},
    "category_slug": {"value": "refrigeradores"},
}

CARD = """
<div class="card" data-id="{sku}">
  <span class="title">{name}</span>
  <span class="price">{price}</span>
  <span class="list">{list_price}</span>
  <img src="https://img.test/{sku}.jpg">
</div>
"""

LINK_RE = re.compile(r'href="([^"]+)"')


def card(sku="sku-1", name="Refrigerador A", price="$ 1.349.990",
         list_price="$ 2.349.990"):
    return CARD.format(sku=sku, name=name, price=price, list_price=list_price)


class FakeScraper:
    """Devuelve un documento Scrapling por URL, sin tocar la red."""

    def __init__(self, pages):
        self.pages = pages

    async def fetch(self, url):
        if url not in self.pages:
            return FetchOutcome(status=404, error="HTTP 404")
        return FetchOutcome(
            status=200, final_url=url, document=Adaptor(self.pages[url]))

    def parse(self, outcome):
        html = self.pages[outcome.final_url]
        return ScrapeResult(
            None, outcome.final_url, "webpage", html,
            links=LINK_RE.findall(html))


class FakeIngestion:
    """Cliente de Ingestion que registra llamadas y simula fallos."""

    def __init__(self, *, fail_open=False, fail_send=False, fail_indices=(),
                 processed=None, batch_size=100):
        self.fail_open = fail_open
        self.fail_send = fail_send
        self.fail_indices = set(fail_indices)
        self.processed = processed
        self.batch_size = batch_size
        self.opened: list[dict] = []
        self.batches: list[list[dict]] = []
        self.finished: list[dict] = []

    async def open_run(self, source_name, base_url, store_id,
                       scraper_type="http"):
        if self.fail_open:
            raise IngestionError("connection refused", retriable=True)
        self.opened.append({
            "source_name": source_name,
            "base_url": base_url,
            "store_id": store_id,
            "scraper_type": scraper_type,
        })
        return RunInfo(id=99, source=source_name, status="running",
                       store_id=store_id)

    async def send_items(self, run_id, items):
        if self.fail_send:
            raise IngestionError("service unavailable", status=503,
                                 retriable=True)
        self.batches.append(items)
        if self.processed is not None:
            return BatchResult(
                run_id=run_id, received=len(items),
                processed=self.processed, failed=len(items) - self.processed,
                results=[])
        results = [
            ItemResult(index=index, status="failed")
            if index in self.fail_indices else
            ItemResult(index=index, status="created")
            for index in range(len(items))
        ]
        failed = sum(result.status == "failed" for result in results)
        return BatchResult(
            run_id=run_id, received=len(items),
            processed=len(items) - failed, failed=failed, results=results)

    async def finish_run(self, run_id, status, error_message=None):
        self.finished.append({
            "run_id": run_id,
            "status": status,
            "error_message": error_message,
        })
        return RunInfo(id=run_id, source="", status=status)


@pytest.fixture(autouse=True)
def fast_retries(monkeypatch):
    monkeypatch.setattr(cfg, "MAX_RETRIES", 1)
    monkeypatch.setattr(cfg, "BACKOFF_BASE_SECONDS", 0.01)


@pytest.fixture()
def env(tmp_path):
    path = str(tmp_path / "crawler.db")
    db.init_db(path)
    db.ensure_source(path, "https://site.test/", name="Tienda Test",
                     store_id=4)
    source, config = db.get_active_jobs(path)[0]
    return {
        "db_path": path,
        "rejects_dir": str(tmp_path / "rejects"),
        "source": source,
        "config": config,
    }


def _crawler(env, pages, ingestion=None, store_id=4, **kwargs):
    source = env["source"]
    if store_id is None:
        source = DataSource(id=source.id, name=source.name,
                            base_url=source.base_url,
                            source_type=source.source_type, store_id=None)
    run_id = db.create_scrape_run(env["db_path"], env["config"].id)
    defaults = dict(
        max_depth=1,
        max_pages=10,
        concurrency=2,
        ingestion=ingestion,
        rejects_dir=env["rejects_dir"],
        bundle=BundleSpec.from_parser_config(BUNDLE),
    )
    defaults.update(kwargs)
    return Crawler(
        source,
        env["config"],
        run_id,
        db_path=env["db_path"],
        scraper=FakeScraper(pages),
        **defaults,
    )


def _run(crawler):
    return asyncio.run(crawler.run())


class TestHappyPath:
    def test_sends_items_and_links_the_run(self, env):
        ingestion = FakeIngestion()
        pages = {
            "https://site.test/": card("sku-1") + card("sku-2"),
            "https://site.test/p": card("sku-3"),
        }
        # La segunda página se descubre desde la primera.
        pages["https://site.test/"] += '<a href="/p">más</a>'

        summary = _run(_crawler(env, pages, ingestion))

        assert ingestion.opened == [{
            "source_name": "Tienda Test",
            "base_url": "https://site.test/",
            "store_id": 4,
            "scraper_type": "http",
        }]
        sent = [item for batch in ingestion.batches for item in batch]
        assert {item["externalId"] for item in sent} == {"sku-1", "sku-2",
                                                         "sku-3"}
        assert ingestion.finished == [{
            "run_id": 99, "status": "success", "error_message": None}]

        assert summary.status == "success"
        assert summary.items_found == 3
        assert summary.items_sent == 3
        assert summary.items_failed == 0
        assert summary.items_rejected == 0
        assert summary.ingestion_run_id == 99

        row = db.get_run(env["db_path"], summary.run_id)
        assert row["ingestion_run_id"] == 99
        assert row["items_found"] == 3
        assert row["items_sent"] == 3
        assert row["ingestion_error"] is None

    def test_sends_in_batches_and_flushes_the_remainder(self, env):
        ingestion = FakeIngestion(batch_size=2)
        pages = {"https://site.test/": "".join(
            card(f"sku-{i}") for i in range(5))}

        summary = _run(_crawler(env, pages, ingestion))

        assert [len(batch) for batch in ingestion.batches] == [2, 2, 1]
        assert summary.items_sent == 5

    def test_dedupes_by_external_id_across_pages(self, env):
        ingestion = FakeIngestion()
        pages = {
            "https://site.test/": card("sku-1") + '<a href="/p">más</a>',
            "https://site.test/p": card("sku-1") + card("sku-2"),
        }

        summary = _run(_crawler(env, pages, ingestion))

        sent = [item for batch in ingestion.batches for item in batch]
        assert [item["externalId"] for item in sent] == ["sku-1", "sku-2"]
        assert summary.items_found == 2

    def test_filters_locally_invalid_items(self, env):
        ingestion = FakeIngestion()
        pages = {
            "https://site.test/": card("sku-1") + card("sku-2", price=""),
        }

        summary = _run(_crawler(env, pages, ingestion))

        assert summary.items_found == 2
        assert summary.items_rejected == 1
        assert summary.items_sent == 1
        sent = [item for batch in ingestion.batches for item in batch]
        assert [item["externalId"] for item in sent] == ["sku-1"]

        rejects = (env["rejects_dir"] + f"/run-{summary.run_id}.jsonl")
        lines = [json.loads(line) for line in open(rejects, encoding="utf-8")]
        assert len(lines) == 1
        assert lines[0]["externalId"] == "sku-2"
        assert any("offer.price" in error
                   for error in lines[0]["provenance"]["errors"])


class TestFailures:
    def test_open_failure_keeps_the_crawl_local(self, env):
        ingestion = FakeIngestion(fail_open=True)

        summary = _run(_crawler(env, {"https://site.test/": card()}, ingestion))

        assert summary.status == "success"
        assert summary.pages_processed == 1
        assert summary.ingestion_run_id is None
        assert "could not open ingestion run" in summary.ingestion_error
        assert ingestion.batches == []
        assert ingestion.finished == []
        # El HTML y el ítem quedaron guardados localmente igual.
        assert db.count_table(env["db_path"], "SCRAPED_DATA") == 1
        row = db.get_run(env["db_path"], summary.run_id)
        assert row["status"] == "success"
        assert "could not open ingestion run" in row["ingestion_error"]

    def test_send_failure_closes_the_remote_run_as_failed(self, env):
        ingestion = FakeIngestion(fail_send=True)

        summary = _run(_crawler(env, {"https://site.test/": card()}, ingestion))

        assert summary.status == "success"  # el crawl local terminó bien
        assert summary.items_failed == 1
        assert summary.items_sent == 0
        assert "service unavailable" in summary.ingestion_error
        assert ingestion.finished == [{
            "run_id": 99, "status": "failed",
            "error_message": summary.ingestion_error,
        }]
        row = db.get_run(env["db_path"], summary.run_id)
        assert row["status"] == "success"
        assert row["items_failed"] == 1
        assert row["ingestion_error"]

    def test_item_rejected_by_ingestion_is_counted_and_kept(self, env):
        ingestion = FakeIngestion(fail_indices={1})

        summary = _run(_crawler(
            env, {"https://site.test/": card("sku-1") + card("sku-2")},
            ingestion))

        assert summary.items_sent == 1
        assert summary.items_failed == 1
        assert ingestion.finished[0]["status"] == "success"
        rejects = (env["rejects_dir"] + f"/run-{summary.run_id}.jsonl")
        lines = [json.loads(line) for line in open(rejects, encoding="utf-8")]
        assert [line["externalId"] for line in lines] == ["sku-2"]

    def test_source_without_store_stays_local(self, env):
        ingestion = FakeIngestion()

        summary = _run(_crawler(
            env, {"https://site.test/": card()}, ingestion, store_id=None))

        assert ingestion.opened == []
        assert summary.ingestion_run_id is None
        assert summary.ingestion_error == "source has no store_id; items stay local"
        assert summary.items_found == 1
        assert summary.items_sent == 0


class TestAbort:
    def test_abort_finishes_failed_and_keeps_buffered_items(self, env):
        ingestion = FakeIngestion()
        crawler = _crawler(env, {"https://site.test/": card("sku-1")},
                           ingestion)
        crawler.items_found = 1

        async def scenario():
            await crawler._open_ingestion()
            # Un ítem alcanzó a bufferizarse antes de la interrupción.
            await crawler._ingest_page(
                Adaptor(card("sku-1")), "https://site.test/")
            assert crawler._buffer
            await crawler.abort_ingestion("interrupted by user")

        asyncio.run(scenario())

        assert ingestion.finished == [{
            "run_id": 99, "status": "failed",
            "error_message": "interrupted by user",
        }]
        assert crawler._buffer == []
        rejects = (env["rejects_dir"] + f"/run-{crawler.run_id}.jsonl")
        lines = [json.loads(line) for line in open(rejects, encoding="utf-8")]
        assert [line["externalId"] for line in lines] == ["sku-1"]


class TestDisabledIngestion:
    def test_items_stay_local_without_a_client(self, env):
        summary = _run(_crawler(env, {"https://site.test/": card()}, None))

        assert summary.status == "success"
        assert summary.items_found == 1
        assert summary.items_sent == 0
        assert summary.ingestion_run_id is None
        assert summary.ingestion_error is None
        assert db.count_table(env["db_path"], "SCRAPED_DATA") == 1

    def test_invalid_bundle_is_reported_before_crawling(self):
        bad = {"entity_type": "product", "root": ".card"}

        with pytest.raises(ValueError):
            bundle_from_config({"bundle": bad})
