#!/usr/bin/env python3
"""Health check del despliegue en el cluster (TAL-164).

Revisa en una sola pasada que SOLOServis este disponible y sano:

  - Pods y Deployments: todos listos, sin CrashLoopBackOff ni reinicios.
  - APIs: /health de cada proceso (comercial, usuarios e ingestion, o backend
    en la topologia consolidada) con la base conectada; scraper-api y frontend.
  - Gateway: una ruta de cada dominio publico pasa por el nginx del frontend.
  - Acceso publico: el Ingress responde por el host publico.
  - PostgreSQL: responde y tiene aplicadas todas las migraciones del manifiesto.
  - Cuota del namespace y volumenes.

A diferencia del pod de test (fase 10 del deploy), se puede correr en cualquier
momento: por ejemplo, cuando alguien avisa que el sitio no funciona.

Es de solo lectura: kubectl get/logs, GET por HTTP y una sesion READ ONLY en
PostgreSQL. No usa el token de servicio. Usa el kubectl y el namespace del
contexto actual. Solo la libreria estandar de Python 3.9+.

    python scripts/testing/salud.py
    python scripts/testing/salud.py --md salud.md    # evidencia en Markdown

Sale con codigo 1 si alguna comprobacion falla.
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import re
import ssl
import subprocess
import sys
import time
import urllib.error
import urllib.request
from dataclasses import dataclass, field
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
API_SERVICES = ("comercial", "usuarios", "ingestion", "backend")  # dominios o consolidado
QUOTA_WARN = 0.8
TRANSIENT = ("Unable to connect", "TLS handshake timeout", "no such host", "i/o timeout",
             "connection reset", "forcibly closed", "error reading from error stream", "EOF")

OK, FAIL, WARN = "OK", "FALLA", "AVISO"
ICON = {OK: "✅", FAIL: "❌", WARN: "⚠️"}
SECTIONS = ("Pods", "APIs", "Gateway y acceso público", "PostgreSQL", "Recursos")


@dataclass
class Check:
    section: str
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
            # stdin en bytes: en modo texto Windows convierte \n en \r\n.
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


def kget(kind: str) -> list[dict]:
    return json.loads(kubectl(["get", kind, "-o", "json"]))["items"]


def sql(query: str) -> list[dict]:
    """Consulta de solo lectura en postgres-0; devuelve las filas como dicts."""
    script = ("SET SESSION CHARACTERISTICS AS TRANSACTION READ ONLY;\n"
              f"SELECT coalesce(json_agg(t), '[]'::json) FROM ({query}) t;\n")
    out = kubectl(["exec", "-i", "postgres-0", "-c", "postgres", "--", "sh", "-c",
                   'psql -X -q -At -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"'], stdin=script)
    return json.loads(out.strip())


def http_get(urls: list[str]) -> dict[str, tuple[int, str]]:
    """GET de URLs internas desde el pod del frontend, en un solo exec."""
    lines = []
    for i, url in enumerate(urls):
        lines.append(f"echo '@@@BEGIN {i}'")
        lines.append(f"curl -s -m 15 -w '\\n@@@STATUS %{{http_code}}\\n' '{url}'")
    out = kubectl(["exec", "-i", "deploy/frontend", "--", "sh", "-s"], stdin="\n".join(lines) + "\n",
                  timeout=60 + 15 * len(urls))
    result: dict[str, tuple[int, str]] = {}
    for chunk in out.split("@@@BEGIN ")[1:]:
        head, _, rest = chunk.partition("\n")
        body, _, status = rest.rpartition("@@@STATUS ")
        result[urls[int(head)]] = (int(status.strip() or 0), body.rstrip("\n"))
    return result


QUANTITY = re.compile(r"^([0-9.]+)([a-zA-Z]*)$")
UNITS = {"": 1, "m": 1e-3, "k": 1e3, "M": 1e6, "G": 1e9, "T": 1e12,
         "Ki": 2**10, "Mi": 2**20, "Gi": 2**30, "Ti": 2**40}


def quantity(value: str) -> float:
    """Cantidad de Kubernetes ("275m", "704Mi", "1G") a número."""
    match = QUANTITY.match(str(value).strip())
    if not match or match.group(2) not in UNITS:
        raise ValueError(f"cantidad desconocida: {value}")
    return float(match.group(1)) * UNITS[match.group(2)]


def ago(timestamp: str | None) -> str:
    if not timestamp:
        return "?"
    delta = dt.datetime.now(dt.timezone.utc) - dt.datetime.fromisoformat(timestamp.replace("Z", "+00:00"))
    hours = delta.total_seconds() / 3600
    return f"hace {hours:.1f} h" if hours < 48 else f"hace {hours / 24:.0f} d"


# ---------------------------------------------------------------- comprobaciones

def check_pods() -> tuple[list[Check], list[str], str]:
    s = SECTIONS[0]
    checks: list[Check] = []
    pods = kget("pods")
    namespace = pods[0]["metadata"]["namespace"] if pods else "?"
    running = [p for p in pods if not any(o.get("kind") == "Job" for o in p["metadata"].get("ownerReferences", []))]
    not_ready, crashing, restarted = [], [], []
    for pod in running:
        name = pod["metadata"]["name"]
        ready = any(c["type"] == "Ready" and c["status"] == "True" for c in pod["status"].get("conditions", []))
        for cs in pod["status"].get("containerStatuses", []):
            waiting = (cs.get("state", {}).get("waiting") or {}).get("reason", "")
            if waiting:
                crashing.append(f"{name}: {waiting}")
            if cs.get("restartCount", 0):
                term = (cs.get("lastState", {}).get("terminated") or {})
                restarted.append(f"{name}: {cs['restartCount']} reinicios, el último {ago(term.get('finishedAt'))}"
                                 f" ({term.get('reason', '?')}, código {term.get('exitCode', '?')})")
        if not ready:
            not_ready.append(f"{name}: {pod['status'].get('phase')}")
    if crashing or not_ready:
        checks.append(Check(s, "Pods listos", FAIL, f"{len(running) - len(not_ready)} de {len(running)} listos",
                             items=crashing + not_ready))
    else:
        checks.append(Check(s, "Pods listos", OK, f"{len(running)} de {len(running)} listos: "
                             + ", ".join(sorted(p["metadata"]["name"].rsplit("-", 2)[0]
                                                if p["metadata"].get("ownerReferences", [{}])[0].get("kind") == "ReplicaSet"
                                                else p["metadata"]["name"] for p in running))))
    checks.append(Check(s, "Reinicios", WARN if restarted else OK,
                        f"{len(restarted)} pods con reinicios" if restarted else "ningún contenedor se reinició",
                        items=restarted))

    deployments = kget("deployments")
    behind = [f"{d['metadata']['name']}: {d['status'].get('readyReplicas', 0)} de {d['spec'].get('replicas', 1)}"
              for d in deployments if d["status"].get("readyReplicas", 0) < d["spec"].get("replicas", 1)]
    images = sorted({c["image"].rsplit(":", 1)[-1] for d in deployments
                     for c in d["spec"]["template"]["spec"]["containers"] if "soloservis" in c["image"]})
    checks.append(Check(s, "Deployments", FAIL if behind else OK,
                        f"{len(deployments) - len(behind)} de {len(deployments)} con todas sus réplicas listas; "
                        f"tag de imagen: {', '.join(images) or '?'}", items=behind))

    failed_jobs = [f"{j['metadata']['name']}: {j['status'].get('failed')} intentos fallidos"
                   for j in kget("jobs") if j["status"].get("failed") and not j["status"].get("succeeded")]
    checks.append(Check(s, "Jobs", WARN if failed_jobs else OK,
                        f"{len(failed_jobs)} Jobs fallidos" if failed_jobs else "ningún Job fallido",
                        items=failed_jobs))
    services = [svc["metadata"]["name"] for svc in kget("services")]
    return checks, services, namespace


def check_http(services: list[str]) -> list[Check]:
    apis = [name for name in API_SERVICES if name in services]
    targets = {f"http://{name}:8080/health": name for name in apis}
    targets["http://scraper-api:8095/health"] = "scraper-api"
    targets["http://frontend:80/healthz"] = "frontend"
    gateway = {
        "http://frontend:80/": ("app (frontend)", 200, 'id="root"'),
        "http://frontend:80/products?limit=1": ("/products → comercial", 200, '"id"'),
        "http://frontend:80/search/products?limit=1": ("/search → comercial", 200, '"id"'),
        "http://frontend:80/services?limit=1": ("/services → comercial", 200, ""),
        "http://frontend:80/auth/validate": ("/auth → usuarios (sin sesión: 401)", 401, ""),
    }
    responses = http_get(list(targets) + list(gateway))

    checks: list[Check] = []
    for url, name in targets.items():
        code, body = responses[url]
        problem = ""
        if code != 200:
            problem = f"HTTP {code}"
        elif name in API_SERVICES and '"database":"connected"' not in body.replace(" ", ""):
            problem = "la base no está conectada"
        checks.append(Check(SECTIONS[1], f"{name} /health" if name != "frontend" else "frontend /healthz",
                            FAIL if problem else OK, f"{problem + ': ' if problem else ''}{body[:90]}".strip()))
    wrong = []
    for url, (label, expected, needle) in gateway.items():
        code, body = responses[url]
        # Una ruta que nginx no enruta cae en la app y responde 200 con HTML:
        # las rutas de API tienen que devolver JSON.
        is_api = not url.endswith(":80/")
        if code != expected or (needle and needle not in body):
            wrong.append(f"{label}: HTTP {code}, se esperaba {expected}")
        elif is_api and body.lstrip().startswith("<"):
            wrong.append(f"{label}: respondió HTML de la app, no la API (nginx no enruta la ruta)")
    checks.append(Check(SECTIONS[2], "Rutas del gateway (nginx del frontend)", FAIL if wrong else OK,
                        f"{len(gateway) - len(wrong)} de {len(gateway)} rutas llegan a su dominio", items=wrong))
    return checks


def check_public() -> list[Check]:
    s = SECTIONS[2]
    ingresses = kget("ingress")
    hosts = [r["host"] for i in ingresses for r in i["spec"].get("rules", []) if r.get("host")]
    if not hosts:
        return [Check(s, "Acceso público", FAIL, "no hay Ingress con host")]
    host = hosts[0]
    # El certificado lo pone la plataforma y no es de confianza (k8s/README.md):
    # se acepta igual, como hace el navegador al aceptar el aviso.
    context = ssl.create_default_context()
    context.check_hostname = False
    context.verify_mode = ssl.CERT_NONE
    url = f"https://{host}/healthz"
    started = time.monotonic()
    try:
        with urllib.request.urlopen(url, timeout=20, context=context) as resp:
            code = resp.status
    except urllib.error.HTTPError as exc:
        code = exc.code
    except (urllib.error.URLError, OSError) as exc:
        return [Check(s, "Acceso público", FAIL, f"{url}: {getattr(exc, 'reason', exc)}")]
    ms = (time.monotonic() - started) * 1000
    return [Check(s, "Acceso público", OK if code == 200 else FAIL,
                  f"{url} → HTTP {code} en {ms:.0f} ms (certificado de la plataforma, no de confianza)")]


def check_postgres() -> list[Check]:
    s = SECTIONS[3]
    rows = sql("SELECT version, (SELECT count(*) FROM product WHERE active) AS productos FROM schema_migrations")
    applied = {r["version"] for r in rows}
    manifest = [line.strip() for line in (ROOT / "backend/database/migration_order.txt").read_text(encoding="utf-8")
                .splitlines() if line.strip().endswith(".sql")]
    missing = [m for m in manifest if m not in applied]
    extra = sorted(applied - set(manifest))
    products = rows[0]["productos"] if rows else 0
    checks = [Check(s, "PostgreSQL responde", OK, f"{products} productos activos")]
    if missing:
        checks.append(Check(s, "Migraciones", WARN,
                            f"{len(applied)} aplicadas; {len(missing)} del manifiesto local sin aplicar "
                            "(¿falta desplegar esta versión?)", items=missing))
    else:
        checks.append(Check(s, "Migraciones", OK, f"{len(manifest)} de {len(manifest)} del manifiesto aplicadas"))
    if extra:
        checks.append(Check(s, "Migraciones fuera del manifiesto local", WARN,
                            f"{len(extra)} aplicadas que este checkout no conoce", items=extra))
    return checks


def check_resources() -> list[Check]:
    s = SECTIONS[4]
    checks: list[Check] = []
    for quota in kget("resourcequota"):
        hard, used = quota["status"].get("hard", {}), quota["status"].get("used", {})
        rows, high = [], []
        for key in sorted(hard):
            try:
                ratio = quantity(used.get(key, "0")) / quantity(hard[key])
            except (ValueError, ZeroDivisionError):
                continue
            rows.append(f"{key} {used.get(key, '0')}/{hard[key]}")
            if ratio >= QUOTA_WARN:
                high.append(f"{key}: {used.get(key, '0')} de {hard[key]} ({ratio:.0%})")
        checks.append(Check(s, f"Cuota `{quota['metadata']['name']}`", WARN if high else OK,
                            "; ".join(rows), items=high))
    unbound = [f"{p['metadata']['name']}: {p['status'].get('phase')}"
               for p in kget("pvc") if p["status"].get("phase") != "Bound"]
    checks.append(Check(s, "Volúmenes (PVC)", FAIL if unbound else OK,
                        "todos en Bound" if not unbound else f"{len(unbound)} sin enlazar", items=unbound))
    return checks


# ---------------------------------------------------------------- salida

def print_report(checks: list[Check], header: dict[str, str]) -> None:
    for key, value in header.items():
        print(f"{key}: {value}")
    for section in SECTIONS:
        print(f"\n== {section}")
        for c in (c for c in checks if c.section == section):
            print(f"  [{c.status}] {c.name}: {c.detail}")
            for item in c.items:
                print(f"      - {item}")
    fails = sum(c.status == FAIL for c in checks)
    warns = sum(c.status == WARN for c in checks)
    print(f"\nResultado: {len(checks) - fails - warns} OK, {fails} fallas, {warns} avisos")


def write_markdown(path: Path, checks: list[Check], header: dict[str, str]) -> None:
    fails = sum(c.status == FAIL for c in checks)
    warns = sum(c.status == WARN for c in checks)
    lines = ["# Health check del despliegue", "", "| Campo | Valor |", "| --- | --- |"]
    lines += [f"| {k} | {v} |" for k, v in header.items()]
    lines += ["", "## Resultado", "",
              f"**{'✅ El despliegue está sano.' if not fails else '❌ Hay comprobaciones fallidas.'}** "
              f"{len(checks) - fails - warns} OK, {fails} fallas, {warns} avisos.", ""]
    for section in SECTIONS:
        lines += [f"## {section}", "", "| Estado | Comprobación | Detalle |", "| --- | --- | --- |"]
        section_checks = [c for c in checks if c.section == section]
        lines += [f"| {ICON[c.status]} {c.status} | {c.name} | {c.detail} |" for c in section_checks]
        for c in section_checks:
            if c.items:
                lines += ["", f"**{c.name}:**", ""] + [f"- {i}" for i in c.items]
        lines.append("")
    path.write_text("\n".join(lines), encoding="utf-8")


def main() -> int:
    parser = argparse.ArgumentParser(description="Health check del despliegue de SOLOServis en el clúster.")
    parser.add_argument("--md", type=Path, help="guarda el resultado en Markdown")
    args = parser.parse_args()
    for stream in (sys.stdout, sys.stderr):
        if hasattr(stream, "reconfigure"):
            stream.reconfigure(encoding="utf-8")

    started = dt.datetime.now().astimezone()
    try:
        checks, services, namespace = check_pods()
        checks += check_http(services)
        checks += check_public()
        checks += check_postgres()
        checks += check_resources()
    except ClusterError as exc:
        print(f"ERROR: no se pudo consultar el clúster: {exc}", file=sys.stderr)
        return 2

    header = {
        "Fecha": started.strftime("%Y-%m-%d %H:%M %z"),
        "Namespace": f"`{namespace}`",
        "Topología": "dominios" if "comercial" in services else "consolidado" if "backend" in services else "?",
        "Generado con": "`python scripts/testing/salud.py`",
    }
    print_report(checks, header)
    if args.md:
        write_markdown(args.md, checks, header)
        print(f"Markdown: {args.md}")
    return 1 if any(c.status == FAIL for c in checks) else 0


if __name__ == "__main__":
    sys.exit(main())
