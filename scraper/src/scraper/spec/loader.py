"""Loading, hashing and resolving store specs."""

from __future__ import annotations

import json
from pathlib import Path

from scraper.errors import AmbiguousSpecError, MissingSpecError, SpecError
from scraper.models import StoreSpec
from scraper.spec.schema import parse_spec

SPEC_FILE = "spec.json"


def load_spec(path: str | Path) -> StoreSpec:
    """Read and validate one spec file."""
    target = Path(path)
    try:
        raw = json.loads(target.read_text(encoding="utf-8"))
    except FileNotFoundError as exc:
        raise SpecError(str(target), "file does not exist") from exc
    except json.JSONDecodeError as exc:
        raise SpecError(str(target), f"invalid JSON: {exc}") from exc
    return parse_spec(raw, target)


def load_specs(directory: str | Path) -> list[StoreSpec]:
    """Load every ``<slug>/spec.json`` under ``directory``.

    Duplicate slugs and overlapping match rules are rejected here, before any
    crawl starts: an ambiguous URL would otherwise pick a spec by accident.
    """
    root = Path(directory)
    if not root.is_dir():
        raise SpecError(str(root), "stores directory does not exist")
    paths = sorted(root.glob(f"*/{SPEC_FILE}"))
    if not paths:
        raise SpecError(str(root), f"no */{SPEC_FILE} files found")

    specs = [load_spec(path) for path in paths]
    seen: dict[str, Path] = {}
    for spec in specs:
        if spec.slug in seen:
            raise SpecError(
                str(spec.path), f"duplicate store slug {spec.slug!r} (also in {seen[spec.slug]})"
            )
        seen[spec.slug] = spec.path or Path(spec.slug)

    for index, first in enumerate(specs):
        for second in specs[index + 1 :]:
            if _overlaps(first, second):
                raise AmbiguousSpecError(
                    str(second.path),
                    f"match rules overlap with {first.slug!r}; "
                    "narrow hosts/pathPrefixes so every URL has one owner",
                )
    return specs


def _overlaps(first: StoreSpec, second: StoreSpec) -> bool:
    hosts = {host.lower() for host in first.match.hosts} & {
        host.lower() for host in second.match.hosts
    }
    if not hosts:
        return False
    first_prefixes = first.match.path_prefixes
    second_prefixes = second.match.path_prefixes
    if not first_prefixes or not second_prefixes:
        return True
    for left in first_prefixes:
        for right in second_prefixes:
            if left.startswith(right) or right.startswith(left):
                return True
    return False


def find_spec(specs: list[StoreSpec], url: str) -> StoreSpec:
    """Return the single spec that owns ``url``.

    Zero matches is a configuration error, not a reason to crawl blindly; more
    than one match is a spec bug caught at load time but re-checked here.
    """
    matches = [spec for spec in specs if spec.match.matches(url)]
    if not matches:
        raise MissingSpecError(
            url,
            "no store spec matches this URL; create "
            f"stores/<slug>/{SPEC_FILE} with a matching 'match' block",
        )
    if len(matches) > 1:
        slugs = ", ".join(sorted(spec.slug for spec in matches))
        raise AmbiguousSpecError(url, f"matched by several specs: {slugs}")
    return matches[0]
