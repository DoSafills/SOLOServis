"""The item contract (TAL-79).

These rules mirror ``backend/internal/ingestion/model.go``. If the Go contract
changes, this module and ``docs/estandares/formato-datos-extraidos.md`` change
in the same commit. Keeping the check local means an invalid item is rejected
and counted *before* a batch is sent, instead of failing at the API.
"""

from __future__ import annotations

import re
from dataclasses import dataclass
from typing import Any
from urllib.parse import urlparse

from scraper.models import (
    CONDITIONS,
    MAX_BRAND,
    MAX_DESCRIPTION,
    MAX_EXTERNAL_ID,
    MAX_MODEL,
    MAX_NAME,
    MAX_SKU,
    MAX_SOURCE_NAME,
    MAX_URL,
    Item,
)

MONEY_RE = re.compile(r"^\d{1,10}(\.\d{1,2})?$")
SLUG_RE = re.compile(r"^[a-z0-9-]{1,64}$")
CURRENCY_RE = re.compile(r"^[A-Z]{3}$")
ATTRIBUTE_KEY_RE = re.compile(r"^[a-z][a-z0-9_]{0,63}$")
MAX_ATTRIBUTE_TEXT = 255


@dataclass(frozen=True, slots=True)
class FieldError:
    field: str
    code: str
    message: str

    def to_dict(self) -> dict[str, str]:
        return {"field": self.field, "code": self.code, "message": self.message}

    def __str__(self) -> str:
        return f"{self.field}: {self.message}"


def _required(errors: list[FieldError], field: str, value: Any) -> None:
    if value is None or (isinstance(value, str) and not value.strip()):
        errors.append(FieldError(field, "required", f"{field} es obligatorio"))


def _max_length(errors: list[FieldError], field: str, value: Any, limit: int) -> None:
    if value is not None and len(str(value)) > limit:
        errors.append(
            FieldError(field, "max_length", f"{field} supera {limit} caracteres")
        )


def _is_http_url(value: Any) -> bool:
    if not isinstance(value, str):
        return False
    parsed = urlparse(value)
    return parsed.scheme in {"http", "https"} and bool(parsed.netloc)


def _http_url(errors: list[FieldError], field: str, value: Any, limit: int | None = None) -> None:
    if not value:
        return
    if limit is not None:
        _max_length(errors, field, value, limit)
    if not _is_http_url(value):
        errors.append(
            FieldError(field, "format", f"{field} debe ser una URL http(s) absoluta")
        )


def _money(errors: list[FieldError], field: str, value: Any, required: bool = False) -> None:
    if not value:
        if required:
            errors.append(FieldError(field, "required", f"{field} es obligatorio"))
        return
    if not isinstance(value, str) or not MONEY_RE.match(value):
        errors.append(
            FieldError(
                field,
                "format",
                f"{field} debe ser un monto con hasta 10 enteros y 2 decimales",
            )
        )


def validate_item(item: Item) -> list[FieldError]:
    """Apply the ingestion rules locally, with the same field codes."""
    errors: list[FieldError] = []

    _required(errors, "externalUrl", item.external_url)
    _http_url(errors, "externalUrl", item.external_url)
    _max_length(errors, "externalId", item.external_id, MAX_EXTERNAL_ID)

    if item.source is not None:
        _required(errors, "source.name", item.source.name)
        _max_length(errors, "source.name", item.source.name, MAX_SOURCE_NAME)
        _required(errors, "source.baseUrl", item.source.base_url)
        _http_url(errors, "source.baseUrl", item.source.base_url)

    product = item.product
    _required(errors, "product.name", product.name)
    _max_length(errors, "product.name", product.name, MAX_NAME)
    _max_length(errors, "product.sku", product.sku, MAX_SKU)
    _max_length(errors, "product.model", product.model, MAX_MODEL)
    _max_length(errors, "product.brand", product.brand, MAX_BRAND)
    _max_length(errors, "product.description", product.description, MAX_DESCRIPTION)
    _http_url(errors, "product.imageUrl", product.image_url)

    _required(errors, "product.categorySlug", product.category_slug)
    if product.category_slug and not SLUG_RE.match(product.category_slug):
        errors.append(
            FieldError(
                "product.categorySlug",
                "format",
                "product.categorySlug debe tener entre 1 y 64 caracteres: "
                "minúsculas, dígitos o guiones",
            )
        )

    for key, value in product.attributes.items():
        field_name = f"product.attributes.{key}"
        if not ATTRIBUTE_KEY_RE.match(key):
            errors.append(
                FieldError(field_name, "format", f"{field_name} no es una clave válida")
            )
        if isinstance(value, str):
            _max_length(errors, field_name, value, MAX_ATTRIBUTE_TEXT)

    offer = item.offer
    _money(errors, "offer.price", offer.price, required=True)
    _money(errors, "offer.listPrice", offer.list_price)
    _money(errors, "offer.shippingCost", offer.shipping_cost)

    if not CURRENCY_RE.match(offer.currency or ""):
        errors.append(
            FieldError(
                "offer.currency",
                "format",
                "offer.currency debe tener exactamente tres letras mayúsculas",
            )
        )
    if offer.condition not in CONDITIONS:
        errors.append(
            FieldError(
                "offer.condition",
                "one_of",
                "offer.condition debe ser new, used o refurbished",
            )
        )
    if offer.stock is not None and offer.stock < 0:
        errors.append(
            FieldError("offer.stock", "min", "offer.stock debe ser mayor o igual a 0")
        )
    _http_url(errors, "offer.productUrl", offer.product_url, MAX_URL)

    return errors
