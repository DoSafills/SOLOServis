"""The spec is the only source of truth; these tests keep it strict."""

from __future__ import annotations

import json

import pytest
from conftest import STORES, spec_raw

from scraper.errors import AmbiguousSpecError, SpecError
from scraper.spec.loader import load_spec, load_specs
from scraper.spec.schema import parse_spec


def test_all_shipped_specs_load(specs):
    slugs = {spec.slug for spec in specs}
    assert slugs == {
        "falabella-cl",
        "ripley-cl",
        "mercadolibre-cl",
        "entel-cl",
        "movistar-cl",
        "gtd-cl",
    }
    for spec in specs:
        assert spec.digest
        if spec.target_type == "service":
            assert spec.field("service.name").required is True
            assert spec.field("offer.price").required is True
            assert spec.field("service.categorySlug") is not None
            assert spec.provider_slug
        else:
            assert spec.field("product.name").required is True
            assert spec.field("offer.price").required is True
            assert spec.field("product.categorySlug") is not None


def test_unknown_top_level_key_is_rejected():
    raw = spec_raw("falabella-cl")
    raw["selectors"] = {}
    with pytest.raises(SpecError, match="unknown key"):
        parse_spec(raw, "inline")


def test_missing_required_contract_field_is_rejected():
    raw = spec_raw("falabella-cl")
    del raw["item"]["fields"]["offer.price"]
    with pytest.raises(SpecError, match="offer.price"):
        parse_spec(raw, "inline")


def test_required_field_must_be_marked_required():
    raw = spec_raw("falabella-cl")
    raw["item"]["fields"]["offer.price"]["required"] = False
    with pytest.raises(SpecError, match="required"):
        parse_spec(raw, "inline")


def test_template_placeholder_must_exist():
    raw = spec_raw("falabella-cl")
    del raw["item"]["supplements"]["externalUrl"]
    raw["item"]["externalUrl"] = {"template": "https://example.test/{nope}"}
    with pytest.raises(SpecError, match="unknown placeholder"):
        parse_spec(raw, "inline")


def test_external_url_needs_a_source():
    raw = spec_raw("falabella-cl")
    del raw["item"]["supplements"]["externalUrl"]
    with pytest.raises(SpecError, match="externalUrl"):
        parse_spec(raw, "inline")


def test_external_url_cannot_be_declared_twice():
    raw = spec_raw("falabella-cl")
    raw["item"]["externalUrl"] = {"attribute": "href"}
    with pytest.raises(SpecError, match="not both"):
        parse_spec(raw, "inline")


def test_supplement_requires_a_supported_source():
    raw = spec_raw("falabella-cl")
    raw["item"]["supplements"]["product.imageUrl"]["from"] = "microdata"
    with pytest.raises(SpecError, match="from"):
        parse_spec(raw, "inline")


def test_supplement_requires_match_by_and_value_path():
    raw = spec_raw("falabella-cl")
    del raw["item"]["supplements"]["product.imageUrl"]["matchBy"]
    with pytest.raises(SpecError, match="matchBy"):
        parse_spec(raw, "inline")

    raw = spec_raw("falabella-cl")
    raw["item"]["supplements"]["product.imageUrl"]["valuePath"] = "image;alert(1)"
    with pytest.raises(SpecError, match="valuePath"):
        parse_spec(raw, "inline")


def test_supplement_target_must_be_a_declared_field():
    raw = spec_raw("falabella-cl")
    raw["item"]["supplements"]["product.sku"] = {
        "from": "jsonld",
        "matchBy": {"attribute": "data-key"},
        "valuePath": "sku",
    }
    with pytest.raises(SpecError, match="not a declared field"):
        parse_spec(raw, "inline")


def test_supplement_rejects_unknown_keys():
    raw = spec_raw("falabella-cl")
    raw["item"]["supplements"]["product.imageUrl"]["selector"] = "img"
    with pytest.raises(SpecError, match="unknown key"):
        parse_spec(raw, "inline")


def test_template_cannot_reference_another_template():
    raw = spec_raw("falabella-cl")
    raw["item"]["fields"]["product.sku"] = {
        "template": "{product.name}",
        "type": "string",
    }
    raw["item"]["fields"]["product.model"] = {
        "template": "{product.sku}",
        "type": "string",
    }
    with pytest.raises(SpecError, match="unknown placeholder"):
        parse_spec(raw, "inline")


def test_selector_and_value_are_mutually_exclusive():
    raw = spec_raw("falabella-cl")
    raw["item"]["fields"]["product.name"]["value"] = "fixed"
    with pytest.raises(SpecError, match="exactly one"):
        parse_spec(raw, "inline")


def test_overlapping_specs_are_rejected(tmp_path):
    for slug in ("falabella-cl", "ripley-cl"):
        raw = spec_raw(slug)
        raw["match"] = {"hosts": ["example.test"], "pathPrefixes": ["/"]}
        directory = tmp_path / slug
        directory.mkdir()
        (directory / "spec.json").write_text(json.dumps(raw), encoding="utf-8")
    with pytest.raises(AmbiguousSpecError):
        load_specs(tmp_path)


def test_missing_file_is_a_spec_error(tmp_path):
    with pytest.raises(SpecError, match="does not exist"):
        load_spec(tmp_path / "nope" / "spec.json")


def test_shipped_specs_are_the_only_ones():
    # Guard against an accidental spec being added without updating the docs.
    found = sorted(path.parent.name for path in STORES.glob("*/spec.json"))
    assert found == [
        "entel-cl",
        "falabella-cl",
        "gtd-cl",
        "mercadolibre-cl",
        "movistar-cl",
        "ripley-cl",
    ]


def test_regex_must_compile():
    raw = spec_raw("falabella-cl")
    raw["item"]["fields"]["product.attributes.capacidad_litros"]["regex"] = r"(\d+"
    with pytest.raises(SpecError, match="invalid regular expression"):
        parse_spec(raw, "inline")


def test_regex_needs_a_value_read_from_the_page():
    raw = spec_raw("falabella-cl")
    raw["item"]["fields"]["product.categorySlug"]["regex"] = "refri"
    with pytest.raises(SpecError, match="regex"):
        parse_spec(raw, "inline")


def test_store_ids_are_unique_and_pinned(specs):
    # Ingestion maps offers by store id, so a duplicate or a silent change
    # would put products in the wrong store. Los specs de servicio no tienen
    # tienda (storeId 0) y se validan aparte.
    mapping = {
        spec.slug: spec.store_id
        for spec in specs
        if spec.target_type != "service"
    }
    assert mapping == {
        "falabella-cl": 1,
        "ripley-cl": 4,
        "mercadolibre-cl": 6,
    }
    assert len(set(mapping.values())) == len(mapping)
    for spec in specs:
        if spec.target_type == "service":
            assert spec.store_id == 0
            assert spec.provider_slug
