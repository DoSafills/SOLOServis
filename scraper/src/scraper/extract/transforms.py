"""Value transforms and type coercion.

A spec reads a raw string out of the DOM; this module turns it into the type
the ingestion contract expects. Coercion never guesses: when a value cannot be
read unambiguously it returns an error and the caller decides whether that
costs the field or the whole item.
"""

from __future__ import annotations

import re
import unicodedata
from decimal import Decimal, InvalidOperation
from functools import lru_cache
from urllib.parse import urljoin, urlparse, urlunparse

from scraper.models import money_text

WHITESPACE_RE = re.compile(r"\s+")
NUMBER_NOISE_RE = re.compile(r"[^\d,.\-+]")
MONEY_TOKEN_RE = re.compile(r"(?<![\d.,])[+-]?\d+(?:[.,]\d+)*(?![\d.,])")
CURRENCY_RE = re.compile(r"\b([A-Za-z]{3})(?![A-Za-z])")
SLUG_RE = re.compile(r"^[a-z0-9-]{1,64}$")
SLUG_STRIP_RE = re.compile(r"[^a-z0-9]+")

TRUE_WORDS = frozenset(
    {
        "1",
        "true",
        "t",
        "yes",
        "y",
        "si",
        "sí",
        "available",
        "disponible",
        "activo",
        "active",
        "en stock",
        "in stock",
        "instock",
        "últimas unidades",
        "ultimas unidades",
    }
)
FALSE_WORDS = frozenset(
    {
        "0",
        "false",
        "f",
        "no",
        "n",
        "agotado",
        "sin stock",
        "out of stock",
        "outofstock",
        "unavailable",
        "indisponible",
        "inactivo",
        "disabled",
    }
)

CONDITION_ALIASES: dict[str, str] = {
    "nuevo": "new",
    "nueva": "new",
    "new": "new",
    "usado": "used",
    "usada": "used",
    "used": "used",
    "segunda mano": "used",
    "reacondicionado": "refurbished",
    "reacondicionada": "refurbished",
    "refurbished": "refurbished",
    "remate": "refurbished",
}

ALLOWED_SCHEMES = frozenset({"http", "https"})

PROMOTIONAL_EDGE_TERMS = (
    "ENVÍO GRATIS",
    "ENVIO GRATIS",
    "DESPACHO GRATIS",
    "OFERTA",
    "NUEVO",
)
_PROMOTIONAL_TERM_PATTERN = "|".join(
    re.escape(term) for term in sorted(PROMOTIONAL_EDGE_TERMS, key=len, reverse=True)
)
_PROMOTIONAL_PREFIX_PAREN_RE = re.compile(
    rf"^\(\s*(?P<term>{_PROMOTIONAL_TERM_PATTERN})\s*\)\s*", re.IGNORECASE
)
_PROMOTIONAL_SUFFIX_PAREN_RE = re.compile(
    rf"\s*\(\s*(?P<term>{_PROMOTIONAL_TERM_PATTERN})\s*\)$", re.IGNORECASE
)
_PROMOTIONAL_PREFIX_RE = re.compile(
    rf"^(?P<term>{_PROMOTIONAL_TERM_PATTERN})(?P<separator>\s*[-:]\s*|\s+)",
    re.IGNORECASE,
)
_PROMOTIONAL_SUFFIX_RE = re.compile(
    rf"(?P<separator>\s*[-:]\s*|\s+)(?P<term>{_PROMOTIONAL_TERM_PATTERN})$",
    re.IGNORECASE,
)


def collapse_ws(text: str) -> str:
    """Normalize Unicode to NFC, collapse whitespace and trim."""
    normalized = unicodedata.normalize("NFC", text)
    return WHITESPACE_RE.sub(" ", normalized).strip()


def strip_promotional_edges(text: str) -> str:
    """Remove configured promotional terms only from explicit title edges."""
    result = collapse_ws(text)
    while result:
        match = _PROMOTIONAL_PREFIX_PAREN_RE.match(result)
        if match:
            result = result[match.end() :].strip()
            continue

        match = _PROMOTIONAL_PREFIX_RE.match(result)
        if match and (
            match.group("term").isupper()
            or "-" in match.group("separator")
            or ":" in match.group("separator")
        ):
            result = result[match.end() :].strip()
            continue

        match = _PROMOTIONAL_SUFFIX_PAREN_RE.search(result)
        if match:
            result = result[: match.start()].strip()
            continue

        match = _PROMOTIONAL_SUFFIX_RE.search(result)
        if match and (
            match.group("term").isupper()
            or "-" in match.group("separator")
            or ":" in match.group("separator")
        ):
            result = result[: match.start()].strip()
            continue

        return result
    return result


def slugify(text: str) -> str:
    """ASCII-fold, lowercase and hyphenate a label into a valid slug."""
    folded = unicodedata.normalize("NFKD", text)
    ascii_text = folded.encode("ascii", "ignore").decode("ascii").lower()
    slug = SLUG_STRIP_RE.sub("-", ascii_text).strip("-")
    return slug[:64].strip("-")


def normalize_url(raw: str, base_url: str | None = None) -> str | None:
    """Resolve, trim and validate a URL; ``None`` when it is not crawlable."""
    candidate = (raw or "").strip()
    if not candidate:
        return None
    if base_url:
        candidate = urljoin(base_url, candidate)
    try:
        parsed = urlparse(candidate)
    except ValueError:
        return None
    scheme = parsed.scheme.lower()
    if scheme not in ALLOWED_SCHEMES:
        return None
    host = parsed.hostname
    if not host:
        return None
    netloc = host.lower()
    port = parsed.port
    default_port = (scheme == "http" and port == 80) or (scheme == "https" and port == 443)
    if port is not None and not default_port:
        netloc = f"{netloc}:{port}"
    path = parsed.path or "/"
    if len(path) > 1 and path.endswith("/"):
        path = path.rstrip("/")
    return urlunparse((scheme, netloc, path, "", parsed.query, ""))


def parse_money(raw: str) -> Decimal | None:
    """Parse the first well-formed monetary amount in a text value."""
    amount, _ = _parse_money_value(raw)
    return amount


def _parse_money_value(raw: str) -> tuple[Decimal | None, str | None]:
    match = MONEY_TOKEN_RE.search(raw or "")
    if match is None:
        return None, f"could not read {raw!r} as money"

    token = match.group()
    sign = token[0] if token[0] in "+-" else ""
    number = token[1:] if sign else token
    has_dot, has_comma = "." in number, "," in number

    if has_dot and has_comma:
        decimal_separator = "." if number.rfind(".") > number.rfind(",") else ","
        grouping_separator = "," if decimal_separator == "." else "."
        integer_part, fraction = number.rsplit(decimal_separator, 1)
        if len(fraction) > 2:
            return None, f"more than two decimal places in amount {token!r}"
        if decimal_separator in integer_part:
            return None, f"invalid thousands grouping in amount {token!r}"
        if grouping_separator in integer_part:
            groups = integer_part.split(grouping_separator)
            if not _valid_thousands_groups(groups):
                return None, f"invalid thousands grouping in amount {token!r}"
            integer_part = "".join(groups)
        elif not integer_part.isdigit():
            return None, f"invalid thousands grouping in amount {token!r}"
        normalized = f"{sign}{integer_part}.{fraction}"
    elif has_dot or has_comma:
        separator = "," if has_comma else "."
        groups = number.split(separator)
        if len(groups) > 2:
            if not _valid_thousands_groups(groups):
                return None, f"invalid thousands grouping in amount {token!r}"
            normalized = sign + "".join(groups)
        else:
            integer_part, fraction = groups
            if len(fraction) == 3:
                # CLP has no sub-unit decimals; comma + 3 digits means thousands, as in v1.
                if not _valid_thousands_groups(groups):
                    return None, f"invalid thousands grouping in amount {token!r}"
                normalized = sign + "".join(groups)
            elif len(fraction) > 2:
                return None, f"more than two decimal places in amount {token!r}"
            else:
                normalized = f"{sign}{integer_part}.{fraction}"
    else:
        normalized = sign + number

    try:
        return Decimal(normalized), None
    except InvalidOperation:
        return None, f"could not read {raw!r} as money"


def _valid_thousands_groups(groups: list[str]) -> bool:
    return (
        bool(groups)
        and 1 <= len(groups[0]) <= 3
        and groups[0].isdigit()
        and all(len(group) == 3 and group.isdigit() for group in groups[1:])
    )


def _parse_number(raw: str) -> Decimal | None:
    """Keep the existing permissive parser for non-money numeric fields."""
    text = NUMBER_NOISE_RE.sub("", raw or "")
    if not text or text in {"-", "+", ".", ","}:
        return None
    has_dot, has_comma = "." in text, "," in text
    if has_dot and has_comma:
        text = (
            text.replace(".", "").replace(",", ".")
            if text.rfind(",") > text.rfind(".")
            else text.replace(",", "")
        )
    elif has_comma:
        text = _resolve_separator(text, ",")
    elif has_dot:
        text = _resolve_separator(text, ".")
    try:
        return Decimal(text)
    except InvalidOperation:
        return None


def _resolve_separator(text: str, separator: str) -> str:
    """Decide whether the separator groups thousands or marks decimals."""
    parts = text.split(separator)
    if len(parts) > 2:
        return text.replace(separator, "")
    head, _, tail = text.partition(separator)
    if len(tail) == 3 and head not in {"", "-", "+"}:
        return head + tail
    return text.replace(separator, ".") if separator == "," else text


def apply_transform(text: str, transform: str | None, page_url: str) -> str:
    """Apply the spec-level transform before type coercion."""
    if not transform:
        return text
    if transform == "trim":
        return text.strip()
    if transform == "collapse_ws":
        return collapse_ws(text)
    if transform == "strip_promo_edges":
        return strip_promotional_edges(text)
    if transform == "lower":
        return text.lower()
    if transform == "upper":
        return text.upper()
    if transform == "slugify":
        return slugify(text)
    if transform == "absolute_url":
        return normalize_url(text, page_url) or text
    raise ValueError(f"unknown transform {transform!r}")


@lru_cache(maxsize=256)
def _compiled(pattern: str) -> re.Pattern[str]:
    return re.compile(pattern, re.IGNORECASE)


def apply_regex(text: str, pattern: str | None) -> str:
    """Keep only the part of ``text`` that ``pattern`` matches.

    Returns the first capture group when the pattern has one, otherwise the
    whole match. No match yields ``""``: the value is absent, not an error, so
    an optional field is simply left out. Matching ignores case.
    """
    if not pattern:
        return text
    match = _compiled(pattern).search(text)
    if match is None:
        return ""
    return match.group(1) if match.re.groups else match.group(0)


def coerce(
    raw: str | None, field_type: str, *, currency: str = "CLP", page_url: str = ""
) -> tuple[object | None, str | None]:
    """Turn a raw string into ``field_type``; return ``(value, error)``.

    ``error`` is a human-readable reason, or ``None`` on success. An empty raw
    value is not an error here: absence is the caller's concern.
    """
    if raw is None:
        return None, None
    text = raw.strip()
    if not text:
        return None, None

    if field_type == "string":
        return collapse_ws(text), None

    if field_type == "money":
        amount, parse_error = _parse_money_value(text)
        if amount is None:
            return None, parse_error
        if amount < 0:
            return None, f"negative amount {text!r}"
        if amount >= Decimal("10000000000"):
            return None, f"amount {text!r} exceeds 10 integer digits"
        return money_text(amount), None

    if field_type == "integer":
        amount = _parse_number(text)
        if amount is None:
            return None, f"could not read {text!r} as an integer"
        if amount != amount.to_integral_value():
            return None, f"{text!r} is not a whole number"
        value = int(amount)
        if value < 0:
            return None, f"negative integer {text!r}"
        return value, None

    if field_type == "boolean":
        word = collapse_ws(text).lower()
        if word in TRUE_WORDS:
            return True, None
        if word in FALSE_WORDS:
            return False, None
        return None, f"could not read {text!r} as a boolean"

    if field_type == "url":
        url = normalize_url(text, page_url)
        if url is None:
            return None, f"{text!r} is not an absolute http(s) URL"
        return url, None

    if field_type == "slug":
        slug = collapse_ws(text).lower()
        if not SLUG_RE.match(slug):
            return None, f"{text!r} is not a valid slug (lowercase, digits, hyphens)"
        return slug, None

    return None, f"unsupported field type {field_type!r}"


def coerce_condition(raw: str | None) -> tuple[str | None, str | None]:
    """Map a localized condition label onto ``new|used|refurbished``."""
    if raw is None:
        return None, None
    word = collapse_ws(raw).lower()
    if not word:
        return None, None
    if word in CONDITION_ALIASES:
        return CONDITION_ALIASES[word], None
    if word in {"new", "used", "refurbished"}:
        return word, None
    return None, f"unknown condition {raw!r}; expected new, used or refurbished"


def coerce_currency(raw: str | None) -> tuple[str | None, str | None]:
    """Extract an ISO-4217 code from a label such as ``"CLP"`` or ``"$usd"``."""
    if raw is None:
        return None, None
    text = raw.strip()
    if not text:
        return None, None
    match = CURRENCY_RE.search(text)
    if match:
        return match.group(1).upper(), None
    if len(text) == 3 and text.isalpha():
        return text.upper(), None
    return None, f"could not read {text!r} as a currency code"
