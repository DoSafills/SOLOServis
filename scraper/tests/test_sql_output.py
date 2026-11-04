"""PostgreSQL import scripts reflect the production catalog schema."""

from __future__ import annotations

import pytest

from scraper.output.sql import write_import_sql


def _item() -> dict:
    return {
        "externalUrl": "https://example.test/product/1",
        "product": {
            "name": "Refrigerador O'Brien",
            "sku": "SKU-1",
            "model": "Model-1",
            "brand": "Brand",
            "categorySlug": "refrigeradores",
            "imageUrl": "https://example.test/image.jpg",
        },
        "offer": {
            "price": "123.45",
            "listPrice": "150.00",
            "currency": "CLP",
            "shippingCost": "0.00",
            "shippingFree": False,
            "available": True,
            "condition": "new",
            "productUrl": "https://example.test/product/1",
        },
    }


def test_writes_idempotent_postgres_import_sql(tmp_path):
    path = write_import_sql(
        tmp_path, run_id=7, store_slug="demo", store_id=2, items=[_item()]
    )
    sql = path.read_text(encoding="utf-8")

    assert path.name == "run-7-demo.sql"
    assert "BEGIN;" in sql and "COMMIT;" in sql
    assert "legacy_product_category_id" in sql
    assert "INSERT INTO product (" in sql
    assert "INSERT INTO product_image" in sql
    assert "INSERT INTO product_offer" in sql
    assert "normalize_product_offer_url(product_url)" in sql
    assert "'Refrigerador O''Brien'" in sql
    assert "123.45" in sql and "150.00" in sql


def test_rejects_invalid_price_instead_of_emitting_bad_sql(tmp_path):
    item = _item()
    item["offer"]["price"] = "not-a-price"

    with pytest.raises(ValueError, match="invalid price"):
        write_import_sql(tmp_path, run_id=1, store_slug="demo", store_id=2, items=[item])
