#!/usr/bin/env python3
"""Validacion del flujo de productos de punta a punta en el cluster (TAL-184).

Comprueba que lo que extrajo el scraper en su ultima corrida es lo que el sitio
muestra y deja consultar:

  1. Extraccion: el CronJob disparo, scraper-api responde y cada tienda tiene
     una corrida reciente, terminada y sin items fallidos.
  2. Ingesta: cada item extraido (scraped_data) tiene su oferta en el catalogo,
     con el mismo precio, actualizada en esa corrida y con el producto activo;
     y no hay dos productos con la misma URL en una tienda.
  3. Consulta: para una muestra, el detalle, el historial de precios y la
     busqueda por nombre responden por el gateway con ese mismo precio.
  4. Visualizacion: la pagina del producto responde con la app y el producto
     tiene imagen de la tienda.

Es de solo lectura: la sesion de PostgreSQL se abre en modo READ ONLY y las
consultas HTTP son GET por el gateway (el Service del frontend), desde el pod
del frontend. No usa el token de servicio. Usa el kubectl y el namespace del
contexto actual. Solo la libreria estandar de Python 3.9+.

    python scripts/testing/flujo_productos.py                # muestra de 5
    python scripts/testing/flujo_productos.py --todos        # todos los productos
    python scripts/testing/flujo_productos.py --md salida.md # evidencia en Markdown

Sale con codigo 1 si alguna comprobacion falla.
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import subprocess
import sys
import time
import urllib.parse
from dataclasses import dataclass, field
from decimal import Decimal, InvalidOperation
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
GATEWAY = "http://frontend:80"
SCRAPER_HEALTH = "http://scraper-api:8095/health"
CRONJOB = "scraper-trigger"
SEARCH_MAX_WORDS = 5  # search.MaxWords del backend
SEARCH_LIMIT = 100  # search.MaxLimit del backend
PLACEHOLDER_HOSTS = ("placehold.co", "example.com", "demo.soloservis.cl")
TRANSIENT = ("Unable to connect", "TLS handshake timeout", "no such host", "i/o timeout",
             "connection reset", "forcibly closed", "error reading from error stream", "EOF")

OK, FAIL, WARN = "OK", "FALLA", "AVISO"
ICON = {OK: "✅", FAIL: "❌", WARN: "⚠️"}
STAGES = ("1. Extracción", "2. Ingesta", "3. Consulta", "4. Visualización")


@dataclass
class Check:
    stage: str
    name: str
    status: str
    detail: str = ""
    items: list[str] = field(default_factory=list)


class ClusterError(RuntimeError):
    pass


# ---------------------------------------------------------------- kubectl

def kubectl(args: list[str], stdin: str | None = None, timeout: int = 120) -> str:
    """Ejecuta kubectl y devuelve stdout. Reintenta los cortes de red del cluster."""
    cmd = ["kubectl", "--request-timeout=30s", *args]
    last = ""
    for attempt in range(3):
        try:
            # En bytes: en modo texto Windows convierte los \n de stdin en \r\n
            # y el \r termina pegado a los argumentos del script remoto.
            proc = subprocess.run(cmd, input=stdin.encode("utf-8") if stdin is not None else None,
                                  capture_output=True, timeout=timeout)
        except subprocess.TimeoutExpired:
            last = f"tiempo agotado tras {timeout}s"
        except OSError as exc:
            raise ClusterError(f"no se pudo ejecutar kubectl: {exc}") from exc
        else:
            out = proc.stdout.decode("utf-8", "replace")
            if proc.returncode == 0:
                return out
            last = (proc.stderr.decode("utf-8", "replace") or out).strip()
            if not any(t in last for t in TRANSIENT):
                break
        time.sleep(3 * (attempt + 1))
    raise ClusterError(f"kubectl {' '.join(args[:3])}: {last.splitlines()[-1] if last else 'sin salida'}")


def sql(query: str) -> list[dict]:
    """Consulta de solo lectura en postgres-0; devuelve las filas como dicts."""
    script = ("SET SESSION CHARACTERISTICS AS TRANSACTION READ ONLY;\n"
              f"SELECT coalesce(json_agg(t), '[]'::json) FROM ({query}) t;\n")
    out = kubectl(["exec", "-i", "postgres-0", "-c", "postgres", "--", "sh", "-c",
                   'psql -X -q -At -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"'],
                  stdin=script)
    return json.loads(out.strip())  # json_agg separa los elementos con saltos de línea


def http_get(urls: list[str]) -> dict[str, tuple[int, str]]:
    """GET de varias URLs internas desde el pod del frontend, en un solo exec."""
    lines = []
    for i, url in enumerate(urls):
        if "'" in url:
            raise ValueError(f"URL con comilla: {url}")
        lines.append(f"echo '@@@BEGIN {i}'")
        lines.append(f"curl -s -m 20 -w '\\n@@@STATUS %{{http_code}}\\n' '{url}'")
    out = kubectl(["exec", "-i", "deploy/frontend", "--", "sh", "-s"], stdin="\n".join(lines) + "\n",
                  timeout=60 + 20 * len(urls))
    result: dict[str, tuple[int, str]] = {}
    for chunk in out.split("@@@BEGIN ")[1:]:
        head, _, rest = chunk.partition("\n")
        body, _, status = rest.rpartition("@@@STATUS ")
        code = int(status.strip() or 0)
        result[urls[int(head)]] = (code, body.rstrip("\n"))
    return result


def as_json(body: str):
    try:
        return json.loads(body)
    except ValueError:
        return None


def same_price(a, b) -> bool:
    try:
        return Decimal(str(a)) == Decimal(str(b))
    except (InvalidOperation, TypeError):
        return False


def money(value) -> str:
    try:
        return f"{Decimal(str(value)):,.0f}".replace(",", ".")
    except (InvalidOperation, TypeError):
        return str(value)


def parse_time(value: str) -> dt.datetime:
    return dt.datetime.fromisoformat(value.replace("Z", "+00:00"))


# ---------------------------------------------------------------- etapas

def stage_extraction(hours: int) -> tuple[list[Check], list[dict], str]:
    stage = STAGES[0]
    checks: list[Check] = []

    cron = json.loads(kubectl(["get", "cronjob", CRONJOB, "-o", "json"]))
    namespace = cron["metadata"]["namespace"]
    status = cron.get("status", {})
    last_ok = status.get("lastSuccessfulTime")
    now = dt.datetime.now(dt.timezone.utc)
    if cron["spec"].get("suspend"):
        checks.append(Check(stage, f"CronJob {CRONJOB}", FAIL, "está suspendido"))
    elif not last_ok or now - parse_time(last_ok) > dt.timedelta(hours=hours):
        checks.append(Check(stage, f"CronJob {CRONJOB}", FAIL,
                            f"sin ejecución exitosa en las últimas {hours} h (última: {last_ok or 'nunca'})"))
    else:
        checks.append(Check(stage, f"CronJob {CRONJOB}", OK,
                            f"activo, horario `{cron['spec']['schedule']}`, última ejecución exitosa {last_ok}"))

    jobs = json.loads(kubectl(["get", "jobs", "-o", "json"]))["items"]
    own = [j for j in jobs if any(o.get("name") == CRONJOB for o in j["metadata"].get("ownerReferences", []))]
    if own:
        job = max(own, key=lambda j: j["metadata"]["creationTimestamp"])
        name = job["metadata"]["name"]
        # Por nombre de pod: `logs job/<x>` espera un pod en ejecución. El pod
        # de un Job terminado puede no existir ya; entonces solo queda su estado.
        pods = json.loads(kubectl(["get", "pods", "-l", f"job-name={name}", "-o", "json"]))["items"]
        log = [line.strip() for pod in pods[-1:]
               for line in kubectl(["logs", pod["metadata"]["name"]]).splitlines() if line.strip()]
        bad = [line for line in log if "HTTP 2" not in line]
        succeeded = job.get("status", {}).get("succeeded", 0) >= 1
        detail = f"Job `{name}` {'terminado' if succeeded else 'sin terminar'}"
        detail += f", {len(log)} tiendas pedidas a scraper-api" if log else ", su pod ya no existe (sin log)"
        checks.append(Check(stage, "Disparo de las tiendas", FAIL if bad or not succeeded else OK,
                            detail, items=log))

    code, body = http_get([SCRAPER_HEALTH])[SCRAPER_HEALTH]
    checks.append(Check(stage, "scraper-api /health", OK if code == 200 else FAIL, f"HTTP {code} {body[:60]}"))

    runs = sql(f"""
        SELECT DISTINCT ON (ds.store_id)
               r.id, ds.store_id, s.name AS tienda, r.status, r.started_at, r.finished_at,
               r.records_found, r.records_processed, r.records_failed, r.error_message,
               round(extract(epoch FROM (now()::timestamp - r.started_at)) / 3600, 1) AS horas
        FROM scrape_run r
        JOIN scraper_config c ON c.id = r.scraper_config_id
        JOIN data_source ds ON ds.id = c.data_source_id
        JOIN store s ON s.id = ds.store_id
        ORDER BY ds.store_id, r.started_at DESC""")
    fresh = []
    for run in runs:
        name = f"Corrida {run['id']} ({run['tienda']})"
        detail = (f"inicio {run['started_at']} (hace {run['horas']} h), {run['records_found']} ítems, "
                  f"{run['records_processed']} procesados, {run['records_failed']} fallidos, estado {run['status']}")
        problems = []
        if float(run["horas"]) > hours:
            problems.append(f"más antigua que {hours} h")
        if run["status"] != "success" or not run["finished_at"]:
            problems.append(f"no terminó bien: {run['error_message'] or run['status']}")
        if run["records_failed"]:
            problems.append(f"{run['records_failed']} ítems rechazados por ingestion")
        checks.append(Check(stage, name, FAIL if problems else OK,
                            "; ".join(problems) + (" — " if problems else "") + detail))
        if float(run["horas"]) <= hours:
            fresh.append(run)
    if not runs:
        checks.append(Check(stage, "Corridas del scraper", FAIL, "no hay ninguna corrida registrada"))
    return checks, fresh, namespace


def stage_ingestion(runs: list[dict]) -> tuple[list[Check], list[dict]]:
    stage = STAGES[1]
    checks: list[Check] = []
    ingested: list[dict] = []
    for run in runs:
        run_id, store_id = int(run["id"]), int(run["store_id"])
        rows = sql(f"""
            WITH r AS (SELECT started_at FROM scrape_run WHERE id = {run_id}),
            items AS (
                SELECT sd.id, sd.raw_data #>> '{{product,name}}' AS nombre,
                       sd.raw_data #>> '{{offer,price}}' AS precio,
                       split_part(normalize_product_offer_url(sd.external_url), '?', 1) AS url_base
                FROM scraped_data sd WHERE sd.scrape_run_id = {run_id})
            SELECT i.id, i.nombre, i.precio, i.url_base,
                   o.offer_id, o.precio_oferta, o.actualizada, o.product_id, o.activo
            FROM items i CROSS JOIN r
            LEFT JOIN LATERAL (
                SELECT o.id AS offer_id, o.price::text AS precio_oferta,
                       o.last_updated >= r.started_at AS actualizada,
                       p.public_id::text AS product_id, p.active AS activo
                FROM product_offer o JOIN product p ON p.id = o.product_id
                WHERE o.store_id = {store_id}
                  AND split_part(normalize_product_offer_url(o.product_url), '?', 1) = i.url_base
                ORDER BY p.active DESC, p.id, o.id LIMIT 1) o ON true
            ORDER BY i.id""")
        missing = [r for r in rows if r["offer_id"] is None]
        inactive = [r for r in rows if r["offer_id"] is not None and not r["activo"]]
        price = [r for r in rows if r["offer_id"] is not None and not same_price(r["precio"], r["precio_oferta"])]
        stale = [r for r in rows if r["offer_id"] is not None and not r["actualizada"]]
        bases: dict[str, int] = {}
        for r in rows:
            bases[r["url_base"]] = bases.get(r["url_base"], 0) + 1
        repeated = sum(n - 1 for n in bases.values() if n > 1)
        details = ([f"sin oferta: {r['nombre']} ({r['url_base']})" for r in missing]
                   + [f"producto dado de baja: {r['nombre']}" for r in inactive]
                   + [f"precio extraído {money(r['precio'])} ≠ catálogo {money(r['precio_oferta'])}: {r['nombre']}"
                      for r in price]
                   + [f"oferta {r['offer_id']} no se actualizó en la corrida: {r['nombre']}" for r in stale])
        offers = len({r["offer_id"] for r in rows if r["offer_id"] is not None})
        summary = (f"{len(rows)} ítems extraídos → {offers} ofertas; "
                   f"{len(rows) - len(missing)} con oferta, {len(rows) - len(price) - len(missing)} con el mismo precio, "
                   f"{len(rows) - len(stale) - len(missing)} actualizadas en la corrida")
        if repeated:
            summary += f"; {repeated} ítems repetidos en el listado de la tienda (misma URL base)"
        checks.append(Check(stage, f"Corrida {run_id} ({run['tienda']}) → catálogo",
                            FAIL if details else OK, summary, items=details))
        seen = set()
        for r in rows:
            if r["offer_id"] is not None and r["activo"] and r["product_id"] not in seen:
                seen.add(r["product_id"])
                ingested.append({**r, "tienda": run["tienda"]})

    store_ids = ",".join(str(int(r["store_id"])) for r in runs) or "NULL"
    dups = sql(f"""
        SELECT s.name AS tienda, split_part(normalize_product_offer_url(o.product_url), '?', 1) AS url_base,
               count(*) AS productos,
               bool_or(p.sku IS NOT NULL OR NOT p.active) AS con_demo,
               string_agg(p.id::text || CASE WHEN p.sku IS NOT NULL THEN ' (demo)' ELSE '' END, ', ' ORDER BY p.id) AS ids
        FROM product_offer o JOIN product p ON p.id = o.product_id JOIN store s ON s.id = o.store_id
        WHERE o.store_id IN ({store_ids})
        GROUP BY 1, 2 HAVING count(DISTINCT p.id) > 1""")
    scraped_dups = [d for d in dups if not d["con_demo"]]
    demo_dups = [d for d in dups if d["con_demo"]]
    if scraped_dups:
        checks.append(Check(stage, "Productos duplicados por URL", FAIL,
                            f"{len(scraped_dups)} URLs con más de un producto scrapeado",
                            items=[f"{d['tienda']}: productos {d['ids']} — {d['url_base']}" for d in scraped_dups]))
    else:
        checks.append(Check(stage, "Productos duplicados por URL", OK, "ninguna URL con dos productos scrapeados"))
    if demo_dups:
        checks.append(Check(stage, "Duplicados con datos de demostración", WARN,
                            f"{len(demo_dups)} URLs compartidas con un producto del seed (ver TAL-241)",
                            items=[f"{d['tienda']}: productos {d['ids']} — {d['url_base']}" for d in demo_dups]))
    return checks, ingested


def pick_sample(products: list[dict], size: int | None) -> list[dict]:
    """Muestra repartida entre tiendas y a lo largo de la corrida (determinista)."""
    if size is None or size >= len(products):
        return products
    by_store: dict[str, list[dict]] = {}
    for p in products:
        by_store.setdefault(p["tienda"], []).append(p)
    stores = list(by_store.values())
    sample: list[dict] = []
    for i, items in enumerate(stores):
        quota = size // len(stores) + (1 if i < size % len(stores) else 0)
        step = len(items) / quota if quota else 0
        sample += [items[int(j * step)] for j in range(min(quota, len(items)))]
    rest = [p for p in products if p not in sample]
    return sample + rest[: size - len(sample)]


def search_words(name: str) -> list[str]:
    """Las palabras más largas del nombre (las más específicas: modelo, capacidad)."""
    words = [w for w in name.split() if len(w) > 1]
    longest = sorted(words, key=len, reverse=True)[:SEARCH_MAX_WORDS]
    return [w for w in words if w in longest][:SEARCH_MAX_WORDS]


def stage_query_and_view(sample: list[dict]) -> list[Check]:
    query, view = STAGES[2], STAGES[3]
    urls = []
    for p in sample:
        q = urllib.parse.quote(" ".join(search_words(p["nombre"])))
        # El historial se lee del detalle (priceHistory), como el frontend: la
        # API de precios (/offers) es interna y el gateway no la publica.
        p["urls"] = {
            "detalle": f"{GATEWAY}/products/{p['product_id']}",
            "busqueda": f"{GATEWAY}/search/products?q={q}&limit={SEARCH_LIMIT}",
            "pagina": f"{GATEWAY}/?page=product-detail&productId={p['product_id']}",
        }
        urls.extend(p["urls"].values())
    responses = http_get(urls)

    detail_bad, history_bad, search_bad, page_bad, no_image = [], [], [], [], []
    for p in sample:
        label = f"{p['nombre']} ({p['tienda']}, {money(p['precio_oferta'])})"
        code, body = responses[p["urls"]["detalle"]]
        data = as_json(body) if code == 200 else None
        offer = next((o for o in (data or {}).get("offers", []) if o.get("offerId") == p["offer_id"]), None)
        if data is None:
            detail_bad.append(f"HTTP {code}: {label}")
        elif offer is None:
            detail_bad.append(f"el detalle no trae la oferta {p['offer_id']}: {label}")
        elif not same_price(offer.get("price"), p["precio_oferta"]):
            detail_bad.append(f"el detalle muestra {money(offer.get('price'))}: {label}")

        points = [x for x in (data or {}).get("priceHistory") or [] if x.get("offerId") == p["offer_id"]]
        if data is not None and not points:
            history_bad.append(f"el historial del detalle no tiene puntos de la oferta {p['offer_id']}: {label}")
        elif points:
            last = max(points, key=lambda x: x.get("recordedAt") or x.get("date") or "")
            if not same_price(last.get("price"), p["precio_oferta"]):
                history_bad.append(f"el último punto es {money(last.get('price'))}: {label}")

        code, body = responses[p["urls"]["busqueda"]]
        found = as_json(body) if code == 200 else None
        if not isinstance(found, list):
            search_bad.append(f"HTTP {code}: {label}")
        elif not any(r.get("id") == p["product_id"] for r in found):
            search_bad.append(f"no aparece buscando «{' '.join(search_words(p['nombre']))}»: {label}")

        code, body = responses[p["urls"]["pagina"]]
        if code != 200 or 'id="root"' not in body:
            page_bad.append(f"HTTP {code}: {label}")
        images = (data or {}).get("images") or []
        image = (images[0] or {}).get("url", "") if images else ""
        host = urllib.parse.urlparse(image).hostname or ""
        if not image or any(host.endswith(h) for h in PLACEHOLDER_HOSTS):
            no_image.append(f"{'sin imagen' if not image else 'imagen de relleno ' + host}: {label}")

    n = len(sample)
    checks = [
        Check(query, "Detalle `GET /products/{id}`", FAIL if detail_bad else OK,
              f"{n - len(detail_bad)} de {n} con la oferta y el precio extraídos", items=detail_bad),
        Check(query, "Historial de precios del detalle (`priceHistory`)", FAIL if history_bad else OK,
              f"{n - len(history_bad)} de {n} terminan en el precio actual de su oferta", items=history_bad),
        Check(query, "Búsqueda `GET /search/products?q=`", FAIL if search_bad else OK,
              f"{n - len(search_bad)} de {n} encontrados por su nombre", items=search_bad),
        Check(view, "Página del producto `/?page=product-detail`", FAIL if page_bad else OK,
              f"{n - len(page_bad)} de {n} responden con la app", items=page_bad),
        Check(view, "Imagen de la tienda", WARN if no_image else OK,
              f"{n - len(no_image)} de {n} con imagen de la tienda", items=no_image),
    ]
    return checks


# ---------------------------------------------------------------- salida

def git_commit() -> str:
    try:
        return subprocess.run(["git", "rev-parse", "--short", "HEAD"], cwd=ROOT, capture_output=True,
                              text=True, check=True).stdout.strip()
    except (OSError, subprocess.CalledProcessError):
        return "?"


def print_report(checks: list[Check], header: dict[str, str]) -> None:
    for key, value in header.items():
        print(f"{key}: {value}")
    for stage in STAGES:
        print(f"\n== {stage}")
        for c in (c for c in checks if c.stage == stage):
            print(f"  [{c.status}] {c.name}: {c.detail}")
            for item in c.items[:15]:
                print(f"      - {item}")
            if len(c.items) > 15:
                print(f"      … y {len(c.items) - 15} más")
    fails = sum(c.status == FAIL for c in checks)
    warns = sum(c.status == WARN for c in checks)
    print(f"\nResultado: {len(checks) - fails - warns} OK, {fails} fallas, {warns} avisos")


def write_markdown(path: Path, checks: list[Check], header: dict[str, str]) -> None:
    fails = sum(c.status == FAIL for c in checks)
    warns = sum(c.status == WARN for c in checks)
    lines = ["# Validación del flujo de productos", "", "| Campo | Valor |", "| --- | --- |"]
    lines += [f"| {k} | {v} |" for k, v in header.items()]
    lines += ["", "## Resultado", "",
              f"**{'✅ El flujo funciona de punta a punta.' if not fails else '❌ Hay comprobaciones fallidas.'}** "
              f"{len(checks) - fails - warns} OK, {fails} fallas, {warns} avisos.", ""]
    for stage in STAGES:
        lines += [f"## {stage}", "", "| Estado | Comprobación | Detalle |", "| --- | --- | --- |"]
        stage_checks = [c for c in checks if c.stage == stage]
        lines += [f"| {ICON[c.status]} {c.status} | {c.name} | {c.detail} |" for c in stage_checks]
        for c in stage_checks:
            if c.items:
                lines += ["", f"**{c.name}:**", ""] + [f"- {i}" for i in c.items]
        lines.append("")
    path.write_text("\n".join(lines), encoding="utf-8")


def main() -> int:
    parser = argparse.ArgumentParser(description="Valida el flujo de productos de punta a punta en el cluster.")
    parser.add_argument("--muestra", type=int, default=5, help="productos a consultar por HTTP (por defecto 5)")
    parser.add_argument("--todos", action="store_true", help="consulta todos los productos de las corridas")
    parser.add_argument("--horas", type=int, default=26, help="antigüedad máxima de la corrida (por defecto 26)")
    parser.add_argument("--md", type=Path, help="guarda el resultado en Markdown")
    args = parser.parse_args()
    for stream in (sys.stdout, sys.stderr):
        if hasattr(stream, "reconfigure"):
            stream.reconfigure(encoding="utf-8")

    started = dt.datetime.now().astimezone()
    try:
        checks, runs, namespace = stage_extraction(args.horas)
        ingest_checks, products = stage_ingestion(runs)
        checks += ingest_checks
        sample = pick_sample(products, None if args.todos else args.muestra)
        if sample:
            checks += stage_query_and_view(sample)
        else:
            checks.append(Check(STAGES[2], "Muestra", FAIL, "no hay productos ingresados para consultar"))
    except ClusterError as exc:
        print(f"ERROR: no se pudo consultar el clúster: {exc}", file=sys.stderr)
        return 2

    header = {
        "Fecha": started.strftime("%Y-%m-%d %H:%M %z"),
        "Namespace": f"`{namespace}`",
        "Commit local": f"`{git_commit()}`",
        "Corridas revisadas": ", ".join(f"{r['id']} ({r['tienda']})" for r in runs) or "ninguna",
        "Productos consultados": f"{len(sample)} de {len(products)}" if sample else "0",
        "Generado con": "`python scripts/testing/flujo_productos.py`",
    }
    print_report(checks, header)
    if args.md:
        write_markdown(args.md, checks, header)
        print(f"Markdown: {args.md}")
    return 1 if any(c.status == FAIL for c in checks) else 0


if __name__ == "__main__":
    sys.exit(main())
