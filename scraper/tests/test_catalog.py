"""Catalog files are stable, atomic and diff-friendly."""

from __future__ import annotations

import json

from scraper.output.catalog import CHANGES_FILE, INDEX_FILE, product_filename, write_catalog


def _item(external_id: str, name: str, price: str) -> dict:
    return {
        "schemaVersion": "1.0",
        "entityType": "product",
        "externalId": external_id,
        "externalUrl": f"https://example.test/p/{external_id}",
        "product": {"name": name, "categorySlug": "refrigeradores"},
        "offer": {"price": price, "currency": "CLP"},
    }


def test_first_write_adds_files_and_index(tmp_path):
    items = [_item("A", "Uno", "1000.00"), _item("B", "Dos", "2000.00")]
    result = write_catalog(tmp_path, store="demo", run_id=1, items=items)
    assert (result.added, result.updated, result.unchanged, result.removed) == (2, 0, 0, 0)
    directory = tmp_path / "demo"
    index = json.loads((directory / INDEX_FILE).read_text(encoding="utf-8"))
    assert index["count"] == 2
    assert index["runId"] == 1
    for item in items:
        path = directory / product_filename(item)
        assert path.exists()
        assert json.loads(path.read_text(encoding="utf-8"))["externalId"] == item["externalId"]


def test_unchanged_items_are_not_rewritten(tmp_path):
    items = [_item("A", "Uno", "1000.00")]
    write_catalog(tmp_path, store="demo", run_id=1, items=items)
    path = tmp_path / "demo" / product_filename(items[0])
    before = path.stat().st_mtime_ns
    result = write_catalog(tmp_path, store="demo", run_id=2, items=items, previous={"A": items[0]})
    assert result.unchanged == 1
    assert path.stat().st_mtime_ns == before


def test_changed_item_is_updated(tmp_path):
    old = _item("A", "Uno", "1000.00")
    new = _item("A", "Uno", "900.00")
    write_catalog(tmp_path, store="demo", run_id=1, items=[old])
    result = write_catalog(tmp_path, store="demo", run_id=2, items=[new], previous={"A": old})
    assert result.updated == 1
    path = tmp_path / "demo" / product_filename(new)
    assert json.loads(path.read_text(encoding="utf-8"))["offer"]["price"] == "900.00"


def test_complete_run_removes_missing_products(tmp_path):
    old = _item("A", "Uno", "1000.00")
    write_catalog(tmp_path, store="demo", run_id=1, items=[old])
    result = write_catalog(tmp_path, store="demo", run_id=2, items=[], previous={"A": old})
    assert result.removed == 1
    assert not (tmp_path / "demo" / product_filename(old)).exists()


def test_incomplete_run_never_deletes(tmp_path):
    old = _item("A", "Uno", "1000.00")
    write_catalog(tmp_path, store="demo", run_id=1, items=[old])
    result = write_catalog(
        tmp_path, store="demo", run_id=2, items=[], previous={"A": old}, complete=False
    )
    assert result.removed == 0
    assert (tmp_path / "demo" / product_filename(old)).exists()


def test_changes_log_is_append_only(tmp_path):
    old = _item("A", "Uno", "1000.00")
    write_catalog(tmp_path, store="demo", run_id=1, items=[old])
    write_catalog(
        tmp_path,
        store="demo",
        run_id=2,
        items=[_item("A", "Uno", "900.00")],
        previous={"A": old},
    )
    lines = (tmp_path / "demo" / CHANGES_FILE).read_text(encoding="utf-8").splitlines()
    kinds = [json.loads(line)["type"] for line in lines]
    assert kinds == ["added", "updated"]
