"""Artefactos locales por ejecución y replay (TAL-136 / TAL-90)."""

import json
from contextlib import closing

import pytest

import artifacts
import database as db
from models import ScrapeResult
from normalized import BundleSpec, NormalizedItem, OfferPayload, ProductPayload

BUNDLE = {
    "entity_type": "product",
    "root": ".card",
    "product": {"fields": {"name": ".title"}},
    "offer": {"fields": {"price": ".price"}},
    "external_id": {"attribute": "data-id"},
    "category_slug": {"value": "refrigeradores"},
}

CARD = """
<div class="card" data-id="{sku}">
  <span class="title">{name}</span>
  <span class="price">{price}</span>
</div>
"""

PAGE = (
    CARD.format(sku="sku-1", name="Refrigerador A", price="$ 1.349.990")
    + CARD.format(sku="sku-2", name="Refrigerador B", price="$ 599.990")
    + CARD.format(sku="sku-3", name="", price="")
)


def item(external_id="sku-1", url="https://site.test/p/sku-1") -> NormalizedItem:
    return NormalizedItem(
        product=ProductPayload(name="Refrigerador A",
                               category_slug="refrigeradores"),
        offer=OfferPayload(price="1349990.00"),
        external_id=external_id,
        external_url=url,
    )


@pytest.fixture()
def run_env(tmp_path):
    path = str(tmp_path / "crawler.db")
    db.init_db(path)
    source_id = db.ensure_source(path, "https://site.test/",
                                 name="Tienda Test", store_id=4)[0]
    with closing(db.connect(path)) as conn, conn:
        conn.execute(
            "UPDATE SCRAPER_CONFIG SET parser_config = ? WHERE data_source_id = ?",
            (json.dumps({"entity_type": "webpage", "bundle": BUNDLE}),
             source_id),
        )
        config_id = conn.execute(
            "SELECT id FROM SCRAPER_CONFIG WHERE data_source_id = ?",
            (source_id,),
        ).fetchone()["id"]
    run_id = db.create_scrape_run(path, config_id)
    return {"db_path": path, "run_id": run_id, "config_id": config_id}


class TestPaths:
    def test_run_scoped_names(self, tmp_path):
        assert artifacts.items_jsonl_path(tmp_path, 7).name == "run-7.jsonl"
        assert artifacts.items_json_path(tmp_path, 7).name == "run-7.json"
        assert artifacts.raw_run_dir(tmp_path, 7).name == "run-7"
        assert artifacts.raw_index_path(tmp_path, 7).name == "index.jsonl"


class TestItemsFile:
    def test_append_is_incremental_and_roundtrips(self, tmp_path):
        path = tmp_path / "items" / "run-1.jsonl"

        assert artifacts.append_items([item("a"), item("b")], path) == 2
        assert artifacts.append_items([item("c")], path) == 1

        records = artifacts.read_items(path)
        assert [record["externalId"] for record in records] == ["a", "b", "c"]
        assert records[0]["schemaVersion"] == "1.0"

    def test_read_missing_file_is_empty(self, tmp_path):
        assert artifacts.read_items(tmp_path / "nope.jsonl") == []

    def test_manifest_json_is_atomic_and_pretty(self, tmp_path):
        path = tmp_path / "run-1.json"
        metadata = artifacts.build_manifest(
            run_id=1,
            source_name="Tienda",
            source_url="https://site.test/",
            store_id=4,
            status="success",
            counters={"found": 2},
            bundle=BUNDLE,
        )

        artifacts.write_items_json([item().to_dict()], path, metadata)

        text = path.read_text(encoding="utf-8")
        assert "\n  " in text  # indentado, legible por humanos
        payload = json.loads(text)
        assert payload["runId"] == 1
        assert payload["source"] == {
            "name": "Tienda",
            "baseUrl": "https://site.test/",
            "storeId": 4,
        }
        assert payload["bundle"]["root"] == ".card"
        assert payload["items"][0]["externalId"] == "sku-1"
        assert not path.with_name("run-1.json.tmp").exists()


class TestManifest:
    def test_optional_fields_are_omitted(self):
        manifest = artifacts.build_manifest(
            run_id=1,
            source_name="Tienda",
            source_url="https://site.test/",
            store_id=None,
            status="success",
            counters={},
        )

        assert "storeId" not in manifest["source"]
        assert "bundle" not in manifest
        assert "errorMessage" not in manifest

    def test_error_message_is_included(self):
        manifest = artifacts.build_manifest(
            run_id=1,
            source_name="Tienda",
            source_url="https://site.test/",
            store_id=1,
            status="failed",
            counters={},
            error_message="boom",
        )

        assert manifest["errorMessage"] == "boom"


class TestRawArchive:
    def test_page_is_written_and_indexed(self, tmp_path):
        html = "<html><body>hola</body></html>"
        entry = artifacts.write_raw_page(
            tmp_path, 3, "https://site.test/a", html, entity_type="webpage")

        target = artifacts.raw_run_dir(tmp_path, 3) / entry["file"]
        assert target.read_text(encoding="utf-8") == html
        assert entry["url"] == "https://site.test/a"
        assert entry["bytes"] == len(html.encode("utf-8"))

        artifacts.append_raw_index(tmp_path, 3, entry)
        artifacts.append_raw_index(tmp_path, 3, {**entry, "url": "https://site.test/b"})
        index = artifacts.read_raw_index(tmp_path, 3)
        assert [row["url"] for row in index] == [
            "https://site.test/a", "https://site.test/b"]

    def test_same_url_reuses_the_file_name(self, tmp_path):
        first = artifacts.write_raw_page(tmp_path, 1, "https://x.test/", "<p>a</p>")
        second = artifacts.write_raw_page(tmp_path, 1, "https://x.test/", "<p>b</p>")
        assert first["file"] == second["file"]

    def test_index_of_unknown_run_is_empty(self, tmp_path):
        assert artifacts.read_raw_index(tmp_path, 99) == []


class TestRunContext:
    def test_reads_source_config_and_bundle(self, run_env):
        context = artifacts.load_run_context(run_env["db_path"],
                                             run_env["run_id"])

        assert context.source_name == "Tienda Test"
        assert context.base_url == "https://site.test/"
        assert context.store_id == 4
        assert context.bundle["root"] == ".card"

    def test_unknown_run_raises(self, run_env):
        with pytest.raises(ValueError, match="does not exist"):
            artifacts.load_run_context(run_env["db_path"], 999)


class TestReplay:
    def _seed_page(self, run_env, html):
        db.save_scraped_data(
            run_env["db_path"],
            run_env["run_id"],
            ScrapeResult(None, "https://site.test/", "webpage", html),
        )

    def test_replay_reparses_the_stored_html(self, run_env):
        self._seed_page(run_env, PAGE)
        spec = BundleSpec.from_parser_config(BUNDLE)

        result = artifacts.replay_run(run_env["db_path"], run_env["run_id"], spec)

        assert result.pages == 1
        assert [item.external_id for item in result.items] == ["sku-1", "sku-2"]
        assert len(result.rejected) == 1
        assert result.rejected[0].external_id == "sku-3"
        assert any("validation" in error
                   for error in result.rejected[0].provenance.errors)

    def test_replay_dedupes_and_skips_empty_payloads(self, run_env):
        self._seed_page(run_env, PAGE)
        db.save_scraped_data(
            run_env["db_path"],
            run_env["run_id"],
            ScrapeResult(None, "https://site.test/empty", "webpage", "{}"),
        )
        spec = BundleSpec.from_parser_config(BUNDLE)

        result = artifacts.replay_run(run_env["db_path"], run_env["run_id"], spec)

        assert result.pages == 1
        assert [item.external_id for item in result.items] == ["sku-1", "sku-2"]

    def test_replay_manifest_marks_the_origin(self, run_env):
        self._seed_page(run_env, PAGE)
        spec = BundleSpec.from_parser_config(BUNDLE)
        context = artifacts.load_run_context(run_env["db_path"], run_env["run_id"])

        result = artifacts.replay_run(run_env["db_path"], run_env["run_id"], spec,
                                      context=context)
        manifest = artifacts.replay_manifest(result, context)

        assert manifest["status"] == "replay"
        assert manifest["runId"] == run_env["run_id"]
        assert manifest["counters"] == {
            "found": 3, "sent": 0, "failed": 0, "rejected": 1}
        assert manifest["bundle"]["root"] == ".card"
