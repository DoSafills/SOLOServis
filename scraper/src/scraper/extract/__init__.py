"""Spec-driven extraction."""

from scraper.extract.engine import ExtractionResult, RejectedItem, extract_page
from scraper.extract.transforms import (
    apply_transform,
    coerce,
    collapse_ws,
    normalize_url,
    parse_money,
    slugify,
)

__all__ = [
    "ExtractionResult",
    "RejectedItem",
    "apply_transform",
    "coerce",
    "collapse_ws",
    "extract_page",
    "normalize_url",
    "parse_money",
    "slugify",
]
