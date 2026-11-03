"""CLI offline ``scripts/export_items.py`` (TAL-136)."""

import importlib.util
import json
from contextlib import closing
from pathlib import Path

import pytest

import artifacts
import database as db
from models import ScrapeResult

SCRIPT = Path(__file__).resolve().parent.parent / "scripts" / "export_items.py"

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


@pytest.fixture(scope="module")
def cli():
    spec = importlib.util.spec_from_file_location("export_items", SCRIPT)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


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
    db.finish_scrape_run(path, run_id, "success")
    return {"db_path": path, "run_id": run_id, "tmp": tmp_path}


class TestFromJsonl:
    def test_rewrites_the_pretty_json(self, cli, run_env):
        jsonl = artifacts.items_jsonl_path(run_env["tmp"], run_env["run_id"])
        artifacts.append_items([], jsonl)
        jsonl.write_text(
            json.dumps({
                "schemaVersion": "1.0",
                "entityType": "product",
                "externalId": "sku-1",
                "externalUrl": "https://site.test/p/sku-1",
                "product": {"name": "Refrigerador A"},
                "offer": {"price": "1349990.00"},
            }) + "\n",
            encoding="utf-8",
        )

        code = cli.main([
            "--run-id", str(run_env["run_id"]),
            "--items-dir", str(run_env["tmp"]),
            "--db", run_env["db_path"],
        ])

        assert code == 0
        payload = json.loads(
            artifacts.items_json_path(run_env["tmp"], run_env["run_id"])
            .read_text(encoding="utf-8"))
        assert payload["runId"] == run_env["run_id"]
        assert payload["source"]["storeId"] == 4
        assert payload["items"][0]["externalId"] == "sku-1"

    def test_missing_jsonl_points_to_from_db(self, cli, run_env):
        with pytest.raises(SystemExit) as caught:
            cli.main([
                "--run-id", str(run_env["run_id"]),
                "--items-dir", str(run_env["tmp"] / "nope"),
                "--db", run_env["db_path"],
            ])

        assert "--from-db" in str(caught.value)

    def test_jsonl_alone_is_enough(self, cli, tmp_path):
        """Un JSONL copiado sin la base sigue produciendo el JSON legible."""
        items_dir = tmp_path / "items"
        jsonl = artifacts.items_jsonl_path(items_dir, 5)
        artifacts.append_items([], jsonl)
        jsonl.write_text(json.dumps({
            "schemaVersion": "1.0",
            "entityType": "product",
            "externalId": "sku-9",
            "externalUrl": "https://site.test/p/sku-9",
            "product": {"name": "Refrigerador Z"},
            "offer": {"price": "999.00"},
            "source": {"name": "Tienda", "baseUrl": "https://site.test/",
                       "storeId": 3},
        }) + "\n", encoding="utf-8")

        assert cli.main([
            "--run-id", "5",
            "--items-dir", str(items_dir),
            "--db", str(tmp_path / "missing.db"),
        ]) == 0

        payload = json.loads(
            artifacts.items_json_path(items_dir, 5).read_text(encoding="utf-8"))
        assert payload["status"] == "unknown"
        assert payload["source"]["storeId"] == 3
        assert payload["items"][0]["externalId"] == "sku-9"


class TestFromDb:
    def _seed(self, run_env):
        html = (
            CARD.format(sku="sku-1", name="Refrigerador A", price="$ 1.349.990")
            + CARD.format(sku="sku-2", name="Refrigerador B", price="$ 599.990")
        )
        db.save_scraped_data(
            run_env["db_path"], run_env["run_id"],
            ScrapeResult(None, "https://site.test/", "webpage", html))

    def test_replays_with_the_bundle_stored_in_the_run(self, cli, run_env):
        self._seed(run_env)
        target = run_env["tmp"] / "replay.json"

        assert cli.main([
            "--run-id", str(run_env["run_id"]),
            "--from-db", "--out", str(target),
            "--db", run_env["db_path"],
        ]) == 0

        payload = json.loads(target.read_text(encoding="utf-8"))
        assert payload["status"] == "replay"
        assert [item["externalId"] for item in payload["items"]] == [
            "sku-1", "sku-2"]

    def test_requires_a_bundle_when_the_run_has_none(self, cli, run_env):
        with closing(db.connect(run_env["db_path"])) as conn, conn:
            conn.execute("UPDATE SCRAPER_CONFIG SET parser_config = ?",
                         (json.dumps({"entity_type": "webpage"}),))

        with pytest.raises(SystemExit) as caught:
            cli.main([
                "--run-id", str(run_env["run_id"]),
                "--from-db", "--db", run_env["db_path"],
            ])

        assert "--config" in str(caught.value)
