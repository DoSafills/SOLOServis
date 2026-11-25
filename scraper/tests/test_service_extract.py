"""Servicios (issue #191): spec, extracción y contrato de ingestion.

Los casos usan el HTML real de Entel guardado en ``fixtures/entel-cl`` para no
depender de la red. El spec de servicio no tiene tienda, usa ``service.*`` y
publica el proveedor en el ítem.
"""

from __future__ import annotations

import json

import pytest
from conftest import FIXTURES, spec_raw

from scraper.errors import SpecError
from scraper.extract.engine import extract_page
from scraper.models import (
    TARGET_SERVICE,
    Item,
    Product,
    Provider,
    Service,
    ServiceOffer,
    Source,
)
from scraper.spec.schema import parse_spec
from scraper.validate import validate_item


def _service_spec():
    return parse_spec(spec_raw("entel-cl"), "inline")


def _extract():
    spec = _service_spec()
    html = (FIXTURES / "entel-cl" / "listing.html").read_text(encoding="utf-8")
    return extract_page(
        spec,
        html,
        page_url=spec.listing.url,
        source=Source(name=spec.name, base_url=spec.base_url, store_id=None),
    )


def test_service_spec_has_no_store_and_declares_provider():
    spec = _service_spec()
    assert spec.target_type == TARGET_SERVICE
    assert spec.store_id == 0
    assert spec.provider_slug == "entel"
    assert spec.provider_name == "Entel"
    assert spec.field("service.name") is not None
    assert spec.field("offer.price") is not None


def test_service_listing_produces_service_items():
    result = _extract()
    assert len(result.items) == 2
    assert not result.rejected

    first = result.items[0]
    assert first.entity_type == TARGET_SERVICE
    assert first.external_id == "internet-fibra-600-megas"
    assert first.provider.slug == "entel"
    assert first.service.name == "Fibra 600"
    assert first.service.category_slug == "internet-fibra-optica"
    assert first.service.attributes == {
        "velocidad_bajada_mbps": 600,
        "velocidad_subida_mbps": 600,
    }
    assert first.service_offer.price == "15990.00"
    assert first.service_offer.currency == "CLP"
    assert first.service_offer.billing_period == "monthly"
    assert first.service_offer.contract_period == "12 meses"
    assert first.service_offer.available is True
    # El fragmento con el id se normaliza fuera: la URL es la de la página.
    assert first.external_url == "https://www.entel.cl/hogar/internet"


def test_service_item_is_valid_and_serializes_the_service_contract():
    item = _extract().items[0]
    assert validate_item(item) == []

    body = item.to_ingestion()
    assert "product" not in body
    assert body["externalId"] == "internet-fibra-600-megas"
    assert body["provider"] == {"slug": "entel", "name": "Entel"}
    assert body["service"]["name"] == "Fibra 600"
    assert body["service"]["attributes"]["velocidad_bajada_mbps"] == 600
    assert body["offer"]["price"] == "15990.00"
    assert body["offer"]["billingPeriod"] == "monthly"

    stored = json.loads(item.to_json())
    assert stored["entityType"] == "service"
    assert stored["service"]["categorySlug"] == "internet-fibra-optica"


def test_service_item_validates_price_and_provider():
    item = Item(
        product=Product(),
        offer=ServiceOffer(),
        entity_type=TARGET_SERVICE,
        external_id="plan-1",
        external_url="https://entel.cl/hogar/internet",
        provider=Provider(slug="entel", name="Entel"),
        service=Service(name="Fibra 600", category_slug="internet-fibra-optica"),
        service_offer=ServiceOffer(price="15990.00"),
    )
    assert validate_item(item) == []

    item.service_offer.price = "no-es-precio"
    item.provider.slug = "Entel SA"
    errors = {f"{error.field}/{error.code}" for error in validate_item(item)}
    assert "offer.price/format" in errors
    assert "provider.slug/format" in errors


def test_service_spec_rejects_product_fields():
    raw = spec_raw("entel-cl")
    raw["item"]["fields"]["product.name"] = {"selector": "h3", "type": "string", "required": True}
    with pytest.raises(SpecError, match="unknown field"):
        parse_spec(raw, "inline")


def test_service_spec_requires_provider_block():
    raw = spec_raw("entel-cl")
    del raw["provider"]
    with pytest.raises(SpecError, match="provider"):
        parse_spec(raw, "inline")


def test_product_spec_rejects_provider_block():
    raw = spec_raw("falabella-cl")
    raw["schemaVersion"] = "1.2"
    raw["provider"] = {"slug": "x", "name": "X"}
    with pytest.raises(SpecError, match="only valid"):
        parse_spec(raw, "inline")
