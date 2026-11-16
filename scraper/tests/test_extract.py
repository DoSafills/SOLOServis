"""Extraction against real markup fixtures for each shipped store."""

from __future__ import annotations

import re

import pytest
from conftest import fixture_html

from scraper.extract.engine import extract_page
from scraper.extract.transforms import apply_regex, coerce
from scraper.models import Source


def _extract(spec, html):
    source = Source(name=spec.name, base_url=spec.base_url, store_id=spec.store_id)
    return extract_page(
        spec,
        html,
        page_url=spec.listing.url,
        source=source,
        run_id=7,
        config_id=3,
        scraped_at="2026-11-03T00:00:00Z",
    )


def test_falabella_reads_template_url_and_money(spec_by_slug):
    spec = spec_by_slug["falabella-cl"]
    result = _extract(spec, fixture_html("falabella-cl"))
    assert len(result.items) == 2
    assert not result.rejected

    first = result.items[0]
    assert first.external_id == "FAL-1001"
    assert first.external_url == "https://www.falabella.com/falabella-cl/product/FAL-1001"
    assert first.product.name == "Refrigerador No Frost 300 L"
    assert first.product.brand == "Samsung"
    assert first.product.category_slug == "refrigeradores"
    assert first.product.image_url.startswith("https://cdn.falabella.com/")
    assert first.offer.price == "549990.00"
    assert first.offer.list_price == "699990.00"
    assert first.offer.currency == "CLP"
    assert first.offer.available is True
    assert first.provenance.run_id == 7
    # La categoría "refrigeradores" exige capacidad_litros: sale del nombre.
    assert first.product.attributes == {"capacidad_litros": 300}

    second = result.items[1]
    assert second.offer.available is False
    assert second.product.attributes == {"capacidad_litros": 500}


def test_ripley_absolutizes_links(spec_by_slug):
    spec = spec_by_slug["ripley-cl"]
    result = _extract(spec, fixture_html("ripley-cl"))
    assert len(result.items) == 2
    first = result.items[0]
    assert first.external_id == "RIP-2001"
    assert first.external_url == "https://simple.ripley.cl/product/refrigerador-samsung-300l"
    assert first.offer.price == "549990.00"
    assert first.product.image_url == "https://simple.ripley.cl/images/RIP-2001.jpg"
    assert first.product.attributes == {"capacidad_litros": 300}
    assert result.items[1].product.attributes == {"capacidad_litros": 500}
    assert result.items[1].offer.available is False


def test_mercadolibre_reads_price_fraction(spec_by_slug):
    spec = spec_by_slug["mercadolibre-cl"]
    result = _extract(spec, fixture_html("mercadolibre-cl"))
    assert len(result.items) == 1
    item = result.items[0]
    assert item.external_id == "MLC-3001"
    assert item.external_url.startswith("https://articulo.mercadolibre.cl/")
    assert item.offer.price == "549990.00"
    assert item.offer.list_price == "699990.00"
    assert item.product.attributes == {"capacidad_litros": 300}


def test_required_capacity_is_rejected_when_missing(spec_by_slug):
    for store in ("falabella-cl", "ripley-cl", "mercadolibre-cl"):
        spec = spec_by_slug[store]
        html = re.sub(
            r"\d{2,4}\s*(?:lts?|litros?|l)\b",
            "",
            fixture_html(store),
            flags=re.IGNORECASE,
        )
        result = _extract(spec, html)

        assert result.items == []
        assert result.rejected
        assert all(
            any("product.attributes.capacidad_litros" in reason for reason in rejected.reasons)
            for rejected in result.rejected
        )


def test_last_units_means_available_for_all_stores(spec_by_slug):
    for store in ("falabella-cl", "ripley-cl", "mercadolibre-cl"):
        spec = spec_by_slug[store]
        html = fixture_html(store).replace("Disponible", "Últimas unidades")
        result = _extract(spec, html)

        assert result.items[0].offer.available is True


def test_string_coercion_normalizes_unicode_and_whitespace():
    value, error = coerce("  Cafe\u0301\tNo   Frost  ", "string")

    assert error is None
    assert value == "Café No Frost"


@pytest.mark.parametrize("store", ("falabella-cl", "ripley-cl", "mercadolibre-cl"))
@pytest.mark.parametrize(
    ("title", "expected"),
    (
        ("OFERTA: Refrigerador No Frost 300 L", "Refrigerador No Frost 300 L"),
        ("(Nuevo) Refrigerador No Frost 300 L", "Refrigerador No Frost 300 L"),
        ("NUEVO Refrigerador No Frost 300 L", "Refrigerador No Frost 300 L"),
        ("Refrigerador No Frost 300 L - ENVÍO GRATIS", "Refrigerador No Frost 300 L"),
        ("Refrigerador Nuevo Frost 300 L", "Refrigerador Nuevo Frost 300 L"),
        ("Nuevo Refrigerador No Frost 300 L", "Nuevo Refrigerador No Frost 300 L"),
    ),
)
def test_promotional_terms_are_removed_only_from_explicit_edges(
    spec_by_slug, store, title, expected
):
    spec = spec_by_slug[store]
    html = fixture_html(store).replace("Refrigerador No Frost 300 L", title, 1)

    result = _extract(spec, html)

    assert not result.rejected
    assert result.items[0].product.name == expected


@pytest.mark.parametrize("store", ("falabella-cl", "ripley-cl", "mercadolibre-cl"))
@pytest.mark.parametrize("brand", ("LG", "lg", "L.G."))
def test_brand_text_is_preserved_for_backend_resolution(spec_by_slug, store, brand):
    spec = spec_by_slug[store]
    html = fixture_html(store).replace("Samsung", brand, 1)

    result = _extract(spec, html)

    assert result.items[0].product.brand == brand


def test_last_units_boolean_aliases_are_available():
    assert coerce("Últimas unidades", "boolean")[0] is True
    assert coerce("ultimas unidades", "boolean")[0] is True


def test_missing_price_rejects_the_card(spec_by_slug):
    spec = spec_by_slug["ripley-cl"]
    html = fixture_html("ripley-cl").replace(
        '<span class="product-price-price">$549.990</span>', ""
    )
    result = _extract(spec, html)
    assert len(result.items) == 1
    assert len(result.rejected) == 1
    assert "offer.price" in " ".join(result.rejected[0].reasons)


def test_unknown_boolean_falls_back_to_the_declared_default(spec_by_slug):
    spec = spec_by_slug["ripley-cl"]
    html = fixture_html("ripley-cl").replace("Disponible", "Consultar disponibilidad")
    result = _extract(spec, html)
    assert result.items[0].offer.available is True


def test_no_root_match_yields_nothing(spec_by_slug):
    spec = spec_by_slug["falabella-cl"]
    result = _extract(spec, "<html><body><p>vacío</p></body></html>")
    assert result.items == []
    assert result.rejected == []
    assert result.cards == 0


def test_apply_regex_keeps_the_capture_group():
    litros = r"(\d{2,4})\s*(?:lts?|litros?|l)\b"
    assert apply_regex("REFRIGERADOR FRIO DIRECTO 262 LTS LRB-270SDIW", litros) == "262"
    assert apply_regex("Side by Side LG 664 Litros Smart Inverter", litros) == "664"
    assert apply_regex("Refrigerador No Frost 300 L", litros) == "300"
    # Sin grupo se devuelve la coincidencia completa; sin coincidencia, vacío.
    assert apply_regex("modelo LRB-270", r"lrb-\d+") == "LRB-270"
    assert apply_regex("Refrigerador sin capacidad", litros) == ""
    assert apply_regex("no se toca", None) == "no se toca"


# TAL-170 money parsing tests, kept separate from the TAL-173 cases above.
_FIRST_PRICE_TEXT = {
    "falabella-cl": "$549.990",
    "ripley-cl": "$549.990",
    "mercadolibre-cl": "549.990",
}


def _fixture_with_first_price(store: str, price: str) -> str:
    html = fixture_html(store)
    original = _FIRST_PRICE_TEXT[store]
    assert original in html
    return html.replace(original, price, 1)


@pytest.mark.parametrize("store", ("falabella-cl", "ripley-cl", "mercadolibre-cl"))
@pytest.mark.parametrize(
    ("raw_price", "expected"),
    (
        ("$549.990", "549990.00"),
        ("549.990,00", "549990.00"),
        ("1.299.990", "1299990.00"),
        ("10,999", "10999.00"),
        ("1,234", "1234.00"),
        ("1.299.990 por 12 cuotas", "1299990.00"),
    ),
)
def test_tal170_valid_money_formats_across_stores(
    spec_by_slug, store, raw_price, expected
):
    spec = spec_by_slug[store]
    result = _extract(spec, _fixture_with_first_price(store, raw_price))

    assert not result.rejected
    assert result.items[0].offer.price == expected


@pytest.mark.parametrize("store", ("falabella-cl", "ripley-cl", "mercadolibre-cl"))
@pytest.mark.parametrize(
    ("raw_price", "reason"),
    (
        ("1.2.3", "invalid thousands grouping"),
        ("549.990,555", "more than two decimal places"),
    ),
)
def test_tal170_invalid_money_is_rejected_with_reason(
    spec_by_slug, store, raw_price, reason
):
    spec = spec_by_slug[store]
    result = _extract(spec, _fixture_with_first_price(store, raw_price))

    assert len(result.rejected) == 1
    assert any(
        reason in error
        for error in result.rejected[0].reasons
        if error.startswith("offer.price:")
    )


def test_tal170_integer_coercion_remains_separate_from_money():
    assert coerce("300", "integer") == (300, None)
