"""Extraction against real markup fixtures for each shipped store."""

from __future__ import annotations

from conftest import fixture_html

from scraper.extract.engine import extract_page
from scraper.extract.transforms import apply_regex
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
