"""Versioned, strict store specs."""

from scraper.spec.loader import find_spec, load_spec, load_specs
from scraper.spec.schema import SPEC_SCHEMA_VERSION, parse_spec

__all__ = ["SPEC_SCHEMA_VERSION", "find_spec", "load_spec", "load_specs", "parse_spec"]
