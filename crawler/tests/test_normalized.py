import json
from datetime import UTC, datetime
from pathlib import Path

import pytest
from jsonschema import Draft202012Validator, FormatChecker
from scrapling.parser import Adaptor

from normalized import (
    MONEY_RE,
    SCHEMA_VERSION,
    BundleExtractor,
    BundleSpec,
    NormalizedItem,
    OfferPayload,
    ProductPayload,
    SourceInfo,
    export_jsonl,
    to_slug,
)

ROOT = Path(__file__).resolve().parent.parent
EXAMPLES = ROOT / "examples"
SCHEMA_PATH = ROOT / "schema" / "normalized-item.schema.json"

FIXED_NOW = datetime(2026, 11, 2, 12, 0, tzinfo=UTC)
FALABELLA_URL = "https://www.falabella.com/falabella-cl/search?Ntt=refrigerador"
RIPLEY_URL = "https://simple.ripley.cl/search/refrigerador"

# Trimmed from the live listing (2 Nov 2026); keeps the real class names.
FALABELLA_CARD = """
<a class="pod pod-4_GRID pod-link" href="" id="testId-pod-14873869"
   data-pod="catalyst-pod">
  <div class="pod-head">
    <picture>
      <img src="https://media.falabella.com/falabellaCL/14873869_1/width=240"
           alt="SAMSUNG - Refrigerador Side by Side de 608 Litros">
    </picture>
  </div>
  <div class="pod-details">
    <div>
      <b class="title1 secondary jsx-233704000 bold">SAMSUNG</b>
      <b class="copy2 primary jsx-233704000 normal">Refrigerador Side by Side
        de 608 Litros Family Hub</b>
    </div>
  </div>
  <div class="pod-summary">
    <div class="prices prices-4_GRID">
      <ol class="pod-prices">
        <li class="prices-0">
          <div class="cmr-icon-container">
            <span class="copy10 primary medium">$  1.349.990</span>
            <div class="discount-badge"><span class="copy5">-43%</span></div>
          </div>
        </li>
        <li class="prices-1">
          <div class="cmr-icon-container">
            <span class="copy3 primary medium crossed">$  2.349.990</span>
          </div>
        </li>
      </ol>
    </div>
  </div>
</a>
"""

RIPLEY_CARD = """
<a class="product-link"
   href="/refrigerador-french-door-420l-no-frost-inverter?color_80=gris&s=mdco">
  <div class="product-item-desktop-vertical-wrap">
    <article class="product-item--wrapper">
      <div class="product-item--media-wrapper">
        <div class="product-image">
          <img class="product-image-img"
               src="https://rimage.ripley.cl/home.ripley/Attachment/MKP/5799/MPM10003483822/full_image-1"
               alt="REFRIGERADOR FRENCH DOOR 420L - 1">
        </div>
      </div>
      <span class="product-item--brand">ELECTROLUX</span>
      <p class="product-item--name">REFRIGERADOR FRENCH DOOR 420L NO FROST</p>
      <div class="product-item--price">
        <div class="product-price-wrapper">
          <div class="product-price-old-price-container">
            <span class="product-price-old-price">$799.990</span>
          </div>
          <div class="product-price-price-container">
            <span class="product-price-price">$599.990</span>
            <span class="product-price-discount">-25%</span>
          </div>
        </div>
      </div>
    </article>
  </div>
</a>
"""


def load_config(name: str = "falabella-cl.json") -> dict:
    return json.loads((EXAMPLES / name).read_text(encoding="utf-8"))


def build_spec(name: str = "falabella-cl.json") -> BundleSpec:
    return BundleSpec.from_parser_config(load_config(name))


def extract(html: str, name: str = "falabella-cl.json",
            base_url: str = FALABELLA_URL) -> list[NormalizedItem]:
    spec = build_spec(name)
    extractor = BundleExtractor(
        spec, run_id=7, config_id=3, clock=lambda: FIXED_NOW)
    return extractor.extract_all(Adaptor(html), base_url)


def simple_spec(**overrides) -> BundleSpec:
    config = {
        "entity_type": "product",
        "root": ".card",
        "product": {"fields": {"name": ".title"}},
        "offer": {"fields": {"price": ".price"}},
        "category_slug": "refrigeradores",
    }
    config.update(overrides)
    return BundleSpec.from_parser_config(config)


# ---------------------------------------------------------------------------
# Real storefronts
# ---------------------------------------------------------------------------


class TestRealCards:
    def test_falabella_card_becomes_one_valid_item(self):
        items = extract(FALABELLA_CARD)
        assert len(items) == 1
        item = items[0]
        assert item.product.name == (
            "Refrigerador Side by Side de 608 Litros Family Hub")
        assert item.product.brand == "SAMSUNG"
        assert item.product.category_slug == "refrigeradores"
        assert item.product.image_url.startswith(
            "https://media.falabella.com/")
        assert item.offer.price == "1349990.00"
        assert item.offer.list_price == "2349990.00"
        assert item.offer.currency == "CLP"
        assert item.offer.condition == "new"
        assert item.offer.available is True
        assert item.offer.shipping_free is False
        assert item.external_id == "testId-pod-14873869"
        assert item.is_valid, [e.to_dict() for e in item.validate()]

    def test_falabella_card_without_href_falls_back_and_reports_it(self):
        item = extract(FALABELLA_CARD)[0]
        assert item.external_url == FALABELLA_URL
        assert any("falling back to the listing page" in e
                   for e in item.provenance.errors)

    def test_ripley_card_uses_the_relative_href_as_external_url(self):
        item = extract(RIPLEY_CARD, "ripley-cl.json", RIPLEY_URL)[0]
        assert item.product.name == "REFRIGERADOR FRENCH DOOR 420L NO FROST"
        assert item.product.brand == "ELECTROLUX"
        assert item.offer.price == "599990.00"
        assert item.offer.list_price == "799990.00"
        assert item.external_url.startswith(
            "https://simple.ripley.cl/refrigerador-french-door-420l")
        assert item.is_valid, [e.to_dict() for e in item.validate()]

    def test_source_metadata_is_run_level_not_item_level(self):
        item = extract(RIPLEY_CARD, "ripley-cl.json", RIPLEY_URL)[0]
        assert item.source is not None
        assert item.source.name == "Ripley Chile"
        payload = item.to_dict()
        assert payload["source"] == {
            "name": "Ripley Chile", "baseUrl": "https://simple.ripley.cl"}
        assert "storeId" not in json.dumps(payload["product"])
        assert "storeId" not in json.dumps(payload["offer"])

    def test_examples_build_without_errors(self):
        for path in sorted(EXAMPLES.glob("*.json")):
            spec = BundleSpec.from_parser_config(
                json.loads(path.read_text(encoding="utf-8")))
            assert spec.root
            assert spec.product.table == "product"
            assert spec.offer.table == "product_offer"


# ---------------------------------------------------------------------------
# Wire contract
# ---------------------------------------------------------------------------


class TestIngestionProjection:
    def test_to_ingestion_only_emits_contract_keys(self):
        item = extract(FALABELLA_CARD)[0]
        payload = item.to_ingestion()
        assert set(payload) == {"externalId", "product", "offer"}
        assert set(payload["product"]) <= {
            "sku", "name", "brand", "model", "categorySlug", "description",
            "imageUrl", "attributes"}
        assert set(payload["offer"]) <= {
            "price", "listPrice", "currency", "shippingCost", "shippingFree",
            "available", "stock", "condition", "productUrl"}
        assert "schemaVersion" not in json.dumps(payload)
        assert "provenance" not in json.dumps(payload)

    def test_envelope_carries_version_and_provenance(self):
        item = extract(RIPLEY_CARD, "ripley-cl.json", RIPLEY_URL)[0]
        payload = item.to_dict()
        assert payload["schemaVersion"] == SCHEMA_VERSION == "1.0"
        assert payload["entityType"] == "product"
        assert payload["provenance"]["scrapedAt"] == "2026-11-02T12:00:00Z"
        assert payload["provenance"]["runId"] == 7
        assert payload["provenance"]["configId"] == 3
        assert payload["provenance"]["pageUrl"] == RIPLEY_URL
        assert payload["provenance"]["references"] == {"brand": "ELECTROLUX"}

    def test_money_is_text_never_float(self):
        item = extract(RIPLEY_CARD, "ripley-cl.json", RIPLEY_URL)[0]
        payload = item.to_dict()
        assert payload["offer"]["price"] == "599990.00"
        assert isinstance(payload["offer"]["price"], str)
        assert MONEY_RE.match(payload["offer"]["price"])

    def test_json_is_deterministic_and_utf8(self):
        item = extract(FALABELLA_CARD)[0]
        first = item.to_json()
        assert first == item.to_json()
        assert '"schemaVersion": "1.0"' in first
        assert "SAMSUNG" in first

    def test_from_replay_json_round_trips_canonical_item(self):
        item = _item(source=SourceInfo(
            name="Tienda", base_url="https://example.test", store_id=17))

        restored = NormalizedItem.from_replay_json(item.to_json())

        assert restored.to_dict() == item.to_dict()

    def test_from_replay_json_requires_positive_store_id(self):
        item = _item(source=SourceInfo(
            name="Tienda", base_url="https://example.test", store_id=17))
        payload = item.to_dict()
        payload["source"]["storeId"] = 0

        with pytest.raises(ValueError, match="source.storeId"):
            NormalizedItem.from_replay_json(json.dumps(payload))

    def test_schema_describes_the_emitted_envelope(self):
        schema = json.loads(SCHEMA_PATH.read_text(encoding="utf-8"))
        item = extract(FALABELLA_CARD)[0]
        payload = item.to_dict()
        assert set(schema["required"]) <= set(payload)
        assert schema["properties"]["schemaVersion"]["const"] == SCHEMA_VERSION
        assert schema["$defs"]["money"]["pattern"] == MONEY_RE.pattern

    def test_schema_accepts_integer_and_decimal_attributes(self):
        schema = json.loads(SCHEMA_PATH.read_text(encoding="utf-8"))
        validator = Draft202012Validator(schema, format_checker=FormatChecker())
        payload = extract(FALABELLA_CARD)[0].to_dict()
        payload["product"]["attributes"] = {
            "capacidad_litros": 350,
            "peso_kg": 65.5,
        }

        assert list(validator.iter_errors(payload)) == []

    def test_schema_rejects_stock_above_int32(self):
        schema = json.loads(SCHEMA_PATH.read_text(encoding="utf-8"))
        validator = Draft202012Validator(schema, format_checker=FormatChecker())
        payload = extract(FALABELLA_CARD)[0].to_dict()
        payload["offer"]["stock"] = 2147483648

        assert any(
            list(error.path) == ["offer", "stock"]
            for error in validator.iter_errors(payload))

    @pytest.mark.parametrize(("html", "config", "url"), [
        (FALABELLA_CARD, "falabella-cl.json", FALABELLA_URL),
        (RIPLEY_CARD, "ripley-cl.json", RIPLEY_URL),
    ])
    def test_emitted_record_validates_against_the_json_schema(
            self, html, config, url):
        schema = json.loads(SCHEMA_PATH.read_text(encoding="utf-8"))
        validator = Draft202012Validator(schema, format_checker=FormatChecker())
        payload = extract(html, config, url)[0].to_dict()
        assert list(validator.iter_errors(payload)) == []

    @pytest.mark.parametrize("path", [
        ("externalUrl",),
        ("source", "baseUrl"),
        ("product", "imageUrl"),
        ("offer", "productUrl"),
        ("provenance", "pageUrl"),
    ])
    def test_schema_http_url_requires_authority(self, path):
        schema = json.loads(SCHEMA_PATH.read_text(encoding="utf-8"))
        validator = Draft202012Validator(schema, format_checker=FormatChecker())
        payload = extract(FALABELLA_CARD)[0].to_dict()
        target = payload
        for key in path[:-1]:
            target = target.setdefault(key, {})
        target[path[-1]] = "https://?q=1"

        assert any(
            list(error.path) == list(path)
            for error in validator.iter_errors(payload))

    def test_schema_rejects_a_record_without_a_name(self):
        schema = json.loads(SCHEMA_PATH.read_text(encoding="utf-8"))
        validator = Draft202012Validator(schema, format_checker=FormatChecker())
        payload = extract(FALABELLA_CARD)[0].to_dict()
        payload["product"]["name"] = ""
        assert list(validator.iter_errors(payload))


# ---------------------------------------------------------------------------
# Validation
# ---------------------------------------------------------------------------


def _item(**overrides) -> NormalizedItem:
    item = NormalizedItem(
        product=ProductPayload(name="Refrigerador", category_slug="refrigeradores"),
        offer=OfferPayload(price="549990.00"),
        external_url="https://example.test/p/1",
    )
    for key, value in overrides.items():
        setattr(item, key, value)
    return item


class TestValidation:
    def test_valid_item_has_no_errors(self):
        assert _item().validate() == []
        assert _item().is_valid

    def test_name_is_required(self):
        item = _item(product=ProductPayload(name="", category_slug="refrigeradores"))
        assert any(e.field == "product.name" and e.code == "required"
                   for e in item.validate())

    def test_name_over_255_is_reported(self):
        item = _item(product=ProductPayload(
            name="X" * 256, category_slug="refrigeradores"))
        assert any(e.field == "product.name" and e.code == "max_length"
                   for e in item.validate())

    def test_category_slug_is_required_and_checked(self):
        missing = _item(product=ProductPayload(name="TV"))
        assert any(e.field == "product.categorySlug" and e.code == "required"
                   for e in missing.validate())

        bad = _item(product=ProductPayload(
            name="TV", category_slug="Refrigeradores"))
        assert any(e.field == "product.categorySlug" and e.code == "format"
                   for e in bad.validate())

    def test_price_is_required(self):
        item = _item(offer=OfferPayload(price=""))
        assert any(e.field == "offer.price" and e.code == "required"
                   for e in item.validate())

    @pytest.mark.parametrize(
        ("price", "code"),
        [("0", "min"), ("0.40", "integer"), ("549990.50", "integer")],
    )
    def test_price_must_be_positive_integer_clp(self, price, code):
        item = _item(offer=OfferPayload(price=price))

        assert any(e.field == "offer.price" and e.code == code
                   for e in item.validate())

    def test_price_must_fit_numeric_12_2(self):
        item = _item(offer=OfferPayload(price="12345678901.00"))
        assert any(e.field == "offer.price" and e.code == "format"
                   for e in item.validate())

    def test_currency_and_condition_are_checked(self):
        item = _item(offer=OfferPayload(
            price="1.00", currency="clp", condition="seminuevo"))
        fields = {(e.field, e.code) for e in item.validate()}
        assert ("offer.currency", "format") in fields
        assert ("offer.condition", "one_of") in fields

    def test_negative_stock_is_reported(self):
        item = _item(offer=OfferPayload(price="1.00", stock=-1))
        assert any(e.field == "offer.stock" and e.code == "min"
                   for e in item.validate())

    def test_external_id_over_255_is_reported(self):
        item = _item(external_id="X" * 256)
        assert any(e.field == "externalId" and e.code == "max_length"
                   for e in item.validate())

    def test_product_url_must_be_absolute_http(self):
        item = _item(offer=OfferPayload(
            price="1.00", product_url="/relativo"))
        assert any(e.field == "offer.productUrl" and e.code == "format"
                   for e in item.validate())

    def test_source_is_validated_when_present(self):
        item = _item(source=SourceInfo(name="", base_url="ftp://x"))
        fields = {(e.field, e.code) for e in item.validate()}
        assert ("source.name", "required") in fields
        assert ("source.baseUrl", "format") in fields


# ---------------------------------------------------------------------------
# Attributes
# ---------------------------------------------------------------------------


class TestAttributes:
    HTML = """
    <div class="card" data-nofrost="true">
      <span class="title">Refrigerador</span>
      <span class="price">$549.990</span>
      <span class="cap">400</span>
      <span class="eff">A+</span>
      <span class="litros">380.5</span>
    </div>
    """

    def _spec(self, attributes: dict) -> BundleSpec:
        return simple_spec(attributes=attributes)

    def _extract(self, attributes: dict) -> NormalizedItem:
        spec = self._spec(attributes)
        extractor = BundleExtractor(spec, clock=lambda: FIXED_NOW)
        return extractor.extract_all(Adaptor(self.HTML), "https://x.test")[0]

    def test_values_are_json_native(self):
        item = self._extract({
            "capacidad_litros": {"selector": ".cap", "type": "integer"},
            "eficiencia": {"selector": ".eff", "type": "string"},
            "volumen": {"selector": ".litros", "type": "decimal"},
            "no_frost": {"attribute": "data-nofrost", "type": "boolean"},
        })
        attributes = item.product.attributes
        assert attributes["capacidad_litros"] == 400
        assert isinstance(attributes["capacidad_litros"], int)
        assert attributes["eficiencia"] == "A+"
        assert attributes["volumen"] == 380.5
        assert attributes["no_frost"] is True
        payload = item.to_dict()["product"]["attributes"]
        assert isinstance(payload["capacidad_litros"], int)
        assert isinstance(payload["no_frost"], bool)

    def test_integer_rejects_a_quoted_decimal(self):
        item = self._extract({
            "capacidad_litros": {"selector": ".litros", "type": "integer"}})
        assert "capacidad_litros" not in item.product.attributes
        assert any("not a whole number" in e for e in item.provenance.errors)

    def test_enum_must_match_the_declared_choices(self):
        item = self._extract({
            "eficiencia": {"selector": ".eff", "type": "enum",
                           "choices": ["A", "B"]}})
        assert "eficiencia" not in item.product.attributes
        assert any("is not one of" in e for e in item.provenance.errors)

    def test_unknown_type_is_rejected_loudly(self):
        with pytest.raises(ValueError, match="unknown type"):
            simple_spec(attributes={"x": {"selector": ".x", "type": "money"}})

    def test_enum_without_choices_is_rejected(self):
        with pytest.raises(ValueError, match="needs choices"):
            simple_spec(attributes={"x": {"selector": ".x", "type": "enum"}})

    def test_invalid_key_is_rejected(self):
        with pytest.raises(ValueError, match="must match"):
            simple_spec(attributes={"Capacidad": {"selector": ".x"}})

    def test_string_over_255_is_reported(self):
        spec = simple_spec(attributes={"x": {"selector": ".title"}})
        extractor = BundleExtractor(spec, clock=lambda: FIXED_NOW)
        item = extractor.extract_all(
            Adaptor('<div class="card"><span class="title">' + "A" * 300 +
                    '</span><span class="price">1</span></div>'),
            "https://x.test")[0]
        assert any("over the 255 allowed" in e for e in item.provenance.errors)


# ---------------------------------------------------------------------------
# categorySlug
# ---------------------------------------------------------------------------


class TestCategorySlug:
    def test_alias_matches_accented_breadcrumbs(self):
        spec = simple_spec(category_slug={
            "selector": ".breadcrumb",
            "alias": {"Electrodomesticos > Refrigeradores": "refrigeradores"},
        })
        extractor = BundleExtractor(spec, clock=lambda: FIXED_NOW)
        item = extractor.extract_all(
            Adaptor('<div class="card"><span class="title">TV</span>'
                    '<span class="price">1</span>'
                    '<span class="breadcrumb">Electrodomésticos &gt; '
                    'Refrigeradores</span></div>'),
            "https://x.test")[0]
        assert item.product.category_slug == "refrigeradores"

    def test_alias_matches_only_the_most_specific_segment(self):
        spec = simple_spec(category_slug={
            "selector": ".breadcrumb",
            "alias": {"refrigeradores": "refrigeradores"},
        })
        extractor = BundleExtractor(spec, clock=lambda: FIXED_NOW)
        item = extractor.extract_all(
            Adaptor('<div class="card"><span class="title">TV</span>'
                    '<span class="price">1</span>'
                    '<span class="breadcrumb">Hogar / Refrigeradores</span></div>'),
            "https://x.test")[0]
        assert item.product.category_slug == "refrigeradores"

    def test_without_alias_the_label_is_folded(self):
        spec = simple_spec(category_slug={"selector": ".breadcrumb"})
        extractor = BundleExtractor(spec, clock=lambda: FIXED_NOW)
        item = extractor.extract_all(
            Adaptor('<div class="card"><span class="title">TV</span>'
                    '<span class="price">1</span>'
                    '<span class="breadcrumb">Línea Blanca / Refrigeración'
                    '</span></div>'),
            "https://x.test")[0]
        assert item.product.category_slug == "linea-blanca-refrigeracion"

    def test_missing_label_never_invents_a_slug(self):
        spec = simple_spec(category_slug={"selector": ".breadcrumb"})
        extractor = BundleExtractor(spec, clock=lambda: FIXED_NOW)
        item = extractor.extract_all(
            Adaptor('<div class="card"><span class="title">TV</span>'
                    '<span class="price">1</span></div>'),
            "https://x.test")[0]
        assert item.product.category_slug is None
        assert any("category_slug" in e for e in item.provenance.errors)
        assert any(e.field == "product.categorySlug"
                   for e in item.validate())

    def test_to_slug(self):
        assert to_slug("  Electrodomésticos > Refrigeración  ") == (
            "electrodomesticos-refrigeracion")
        assert to_slug("A+ / B+") == "a-b"
        assert to_slug("---") == ""
        assert len(to_slug("x" * 100)) == 64


# ---------------------------------------------------------------------------
# Spec construction and extraction mechanics
# ---------------------------------------------------------------------------


class TestBundleSpec:
    def test_unknown_top_level_key_is_rejected(self):
        with pytest.raises(ValueError, match="unknown parser_config keys"):
            BundleSpec.from_parser_config({
                "entity_type": "product",
                "product": {"fields": {"name": ".t"}},
                "offer": {"fields": {"price": ".p"}},
                "feilds": {},
            })

    def test_product_and_offer_are_mandatory(self):
        with pytest.raises(ValueError, match="'product'"):
            BundleSpec.from_parser_config({"entity_type": "product"})
        with pytest.raises(ValueError, match="'product_offer'"):
            BundleSpec.from_parser_config({
                "product": {"fields": {"name": ".t"}}})

    def test_services_are_not_supported_yet(self):
        with pytest.raises(ValueError, match="not supported yet"):
            BundleSpec.from_parser_config({"entity_type": "service"})

    def test_unknown_column_is_still_rejected(self):
        with pytest.raises(ValueError, match="unknown column"):
            simple_spec(product={"fields": {"colour": ".c"}})

    def test_extract_all_returns_one_item_per_root(self):
        html = (FALABELLA_CARD + FALABELLA_CARD.replace("14873869", "80563776"))
        items = extract(html)
        assert len(items) == 2
        assert [i.external_id for i in items] == [
            "testId-pod-14873869", "testId-pod-80563776"]

    def test_without_root_it_returns_one_item(self):
        spec = BundleSpec.from_parser_config({
            "product": {"fields": {"name": ".title"}},
            "offer": {"fields": {"price": ".price"}},
            "category_slug": "refrigeradores",
        })
        document = Adaptor('<span class="title">TV</span>'
                           '<span class="price">1</span>')
        items = BundleExtractor(
            spec, clock=lambda: FIXED_NOW).extract_all(
                document, "https://x.test/p/1")
        assert len(items) == 1
        assert items[0].external_url == "https://x.test/p/1"

    def test_image_list_takes_the_first_match(self):
        spec = simple_spec(images={"fields": {
            "image_url": {"selector": "img", "mode": "list",
                          "attribute": "src"}}})
        document = Adaptor('<div class="card"><span class="title">TV</span>'
                           '<span class="price">1</span>'
                           '<img src="/a.jpg"><img src="/b.jpg"></div>')
        item = BundleExtractor(
            spec, clock=lambda: FIXED_NOW).extract_all(
                document, "https://x.test/p/1")[0]
        assert item.product.image_url == "https://x.test/a.jpg"

    def test_extraction_errors_are_prefixed_and_collected(self):
        spec = simple_spec(product={"fields": {"name": ".nope"}})
        document = Adaptor('<div class="card"><span class="price">1</span></div>')
        item = BundleExtractor(
            spec, clock=lambda: FIXED_NOW).extract_all(
                document, "https://x.test/p/1")[0]
        assert any(e.startswith("product.name:") for e in item.provenance.errors)
        assert not item.is_valid


# ---------------------------------------------------------------------------
# Export
# ---------------------------------------------------------------------------


class TestExport:
    def test_export_jsonl_appends_one_line_per_item(self, tmp_path):
        items = extract(FALABELLA_CARD)
        target = tmp_path / "out" / "items.jsonl"
        assert export_jsonl(items, target) == 1
        assert export_jsonl(items, target) == 1
        lines = target.read_text(encoding="utf-8").strip().splitlines()
        assert len(lines) == 2
        first = json.loads(lines[0])
        assert first["schemaVersion"] == SCHEMA_VERSION
        assert first["product"]["name"].startswith("Refrigerador")
