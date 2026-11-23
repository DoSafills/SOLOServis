#!/usr/bin/env python3
"""Health check del despliegue en el cluster (TAL-164).

Revisa en una sola pasada que SOLOServis este disponible y sano:

  - Pods y Deployments: todos listos, sin CrashLoopBackOff ni reinicios.
  - APIs: /health de cada proceso (comercial, usuarios e ingestion, o backend
    en la topologia consolidada) con la base conectada; scraper-api y frontend.
  - Gateway: lee las reglas location -> proxy_pass del nginx desplegado y, por
    cada prefijo, compara la respuesta por el gateway con la del upstream
    configurado y con la de los otros procesos.
  - Acceso publico: el Ingress responde por el host publico.
  - PostgreSQL: responde y tiene aplicadas todas las migraciones del manifiesto.
  - Cuota del namespace y volumenes.

A diferencia del pod de test (fase 10 del deploy), se puede correr en cualquier
momento: por ejemplo, cuando alguien avisa que el sitio no funciona.

Es de solo lectura: kubectl get/logs, GET por HTTP y una sesion READ ONLY en
PostgreSQL. No usa el token de servicio. Usa el kubectl y el namespace del
contexto actual. Solo la libreria estandar de Python 3.9+.

    python scripts/testing/salud.py
    python scripts/testing/salud.py --detalle        # cada pedido: esperado y obtenido
    python scripts/testing/salud.py --md salud.md    # evidencia en Markdown
    python scripts/testing/reporte.py --salud        # dentro del reporte de testing

Sale con codigo 1 si alguna comprobacion falla.
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import re
import secrets
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
# Cada pedido lleva X-Request-ID `salud-<corrida>-<n>`: así errores.py (TAL-219)
# separa en los logs el tráfico de este script del de usuarios reales.
REQUEST_ID_PREFIX = f"salud-{secrets.token_hex(8)}"

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


# Cada pedido que hace el script, con lo esperado y lo obtenido (--detalle).
PROBES: list[dict[str, str]] = []


def probe(section: str, request: str, expected: str, got: str, sample: str, ok: bool | str) -> None:
    """Registra un pedido. `ok` es True/False, o directamente OK, FAIL o WARN."""
    sample = " ".join(sample.split())
    status = ok if isinstance(ok, str) else (OK if ok else FAIL)
    PROBES.append({"seccion": section, "pedido": request, "esperado": expected, "obtenido": got,
                   "extracto": sample[:100] + ("…" if len(sample) > 100 else ""), "estado": status})


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
    """Lista los objetos de un tipo (pods, jobs…) del namespace actual; devuelve sus `items`."""
    return json.loads(kubectl(["get", kind, "-o", "json"]))["items"]


def sql(query: str) -> list[dict]:
    """Consulta de solo lectura en postgres-0; devuelve las filas como dicts."""
    script = ("SET SESSION CHARACTERISTICS AS TRANSACTION READ ONLY;\n"
              f"SELECT coalesce(json_agg(t), '[]'::json) FROM ({query}) t;\n")
    out = kubectl(["exec", "-i", "postgres-0", "-c", "postgres", "--", "sh", "-c",
                   'psql -X -q -At -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"'], stdin=script)
    return json.loads(out.strip())


def http_get(urls: list[str]) -> dict[str, tuple[int, str, str]]:
    """GET de URLs internas desde el pod del frontend, en un solo exec.

    Devuelve por URL (código, cuerpo, Content-Type)."""
    lines = []
    for i, url in enumerate(urls):
        lines.append(f"echo '@@@BEGIN {i}'")
        lines.append(f"curl -s -m 15 -H 'X-Request-ID: {REQUEST_ID_PREFIX}-{i}' "
                     f"-w '\\n@@@STATUS %{{http_code}} %{{content_type}}\\n' '{url}'")
    out = kubectl(["exec", "-i", "deploy/frontend", "--", "sh", "-s"], stdin="\n".join(lines) + "\n",
                  timeout=60 + 15 * len(urls))
    result: dict[str, tuple[int, str, str]] = {}
    for chunk in out.split("@@@BEGIN ")[1:]:
        head, _, rest = chunk.partition("\n")
        body, _, status = rest.rpartition("@@@STATUS ")
        code, _, ctype = status.strip().partition(" ")
        result[urls[int(head)]] = (int(code or 0), body.rstrip("\n"), ctype.split(";")[0].strip())
    return result


NGINX_CONF = "/etc/nginx/conf.d/default.conf"
LOCATION = re.compile(r"location\s+~\s+\^/\(?([a-z|-]+)\)?\(/\.\*\)\?\$\s*\{")
PROXY = re.compile(r"proxy_pass\s+https?://([^;/\s]+)")
METHODS = re.compile(r"limit_except\s+([A-Z ]+?)\s*\{")
REQUEST_ID = re.compile(r'"(request_?[iI]d)"\s*:\s*"[^"]*"')


def gateway_routes() -> list[tuple[str, str, str]]:
    """Reglas (prefijo, upstream, métodos) del nginx que corre en el pod del frontend."""
    conf = kubectl(["exec", "deploy/frontend", "--", "cat", NGINX_CONF])
    # Un bloque va hasta el siguiente `location`: puede tener llaves anidadas
    # (limit_except GET { deny all; }).
    heads = list(re.finditer(r"location\s", conf))
    routes = []
    for i, head in enumerate(heads):
        block = conf[head.start(): heads[i + 1].start() if i + 1 < len(heads) else len(conf)]
        match, upstream = LOCATION.match(block), PROXY.search(block)
        if match and upstream:
            methods = METHODS.search(block)
            allowed = " ".join(methods.group(1).split()) if methods else "todos"
            routes += [(prefix, upstream.group(1), allowed) for prefix in match.group(1).split("|")]
    return routes


def fingerprint(response: tuple[int, str, str]) -> tuple[int, str, str]:
    """Respuesta comparable entre pedidos: sin el request_id, que cambia en cada uno."""
    code, body, ctype = response
    return code, ctype, REQUEST_ID.sub("", body)


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
    """Convierte una fecha ISO de Kubernetes en «hace N h» o «hace N d»; devuelve «?» si no hay fecha."""
    if not timestamp:
        return "?"
    delta = dt.datetime.now(dt.timezone.utc) - dt.datetime.fromisoformat(timestamp.replace("Z", "+00:00"))
    hours = delta.total_seconds() / 3600
    return f"hace {hours:.1f} h" if hours < 48 else f"hace {hours / 24:.0f} d"


# ---------------------------------------------------------------- comprobaciones

def check_pods() -> tuple[list[Check], list[str], str]:
    """Revisa pods, reinicios, Deployments y Jobs.

    Devuelve las comprobaciones, los nombres de los Services (para saber la
    topología) y el namespace."""
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
    probe(s, "kubectl get pods (sin los de Jobs)", "todos Ready, sin esperas ni reinicios",
          f"{len(running) - len(not_ready)} de {len(running)} Ready, {len(restarted)} con reinicios",
          "; ".join(crashing), not (crashing or not_ready))
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
    probe(s, "kubectl get deployments", "readyReplicas = replicas en todos",
          f"{len(deployments) - len(behind)} de {len(deployments)}", "; ".join(behind), not behind)
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
    """Pide el /health de cada proceso, la app y cada prefijo del gateway.

    Recibe los Services del namespace y devuelve las comprobaciones de APIs y
    del enrutamiento de nginx."""
    apis = [name for name in API_SERVICES if name in services]
    targets = {f"http://{name}:8080/health": name for name in apis}
    targets["http://scraper-api:8095/health"] = "scraper-api"
    targets["http://frontend:80/healthz"] = "frontend"
    app = "http://frontend:80/"
    # Las rutas se leen de la configuración desplegada, no de este script. Cada
    # prefijo se pide por el gateway y directo a cada proceso de API.
    routes = gateway_routes()
    route_urls = [f"http://{host}/{prefix}" for prefix, _, _ in routes
                  for host in ["frontend:80"] + [f"{a}:8080" for a in apis]]
    responses = http_get(list(targets) + [app] + list(dict.fromkeys(route_urls)))

    checks: list[Check] = []
    for url, name in targets.items():
        code, body, _ = responses[url]
        problem = ""
        needs_db = name in API_SERVICES
        if code != 200:
            problem = f"HTTP {code}"
        elif needs_db and '"database":"connected"' not in body.replace(" ", ""):
            problem = "la base no está conectada"
        probe(SECTIONS[1], f"GET {url}", "200" + (' con "database":"connected"' if needs_db else ""),
              str(code), body, not problem)
        checks.append(Check(SECTIONS[1], f"{name} /health" if name != "frontend" else "frontend /healthz",
                            FAIL if problem else OK, f"{problem + ': ' if problem else ''}{body[:90]}".strip()))
    code, body, ctype = responses[app]
    app_ok = code == 200 and 'id="root"' in body
    probe(SECTIONS[2], f"GET {app}", '200, HTML con `id="root"`', f"{code} {ctype}", body, app_ok)
    checks.append(Check(SECTIONS[2], "App del frontend", OK if app_ok else FAIL, f"`/` → HTTP {code} {ctype}"))

    # Una ruta llega a su dominio si la respuesta del gateway es igual a la del
    # upstream configurado (código, Content-Type y cuerpo sin request_id). Se
    # distingue si además es distinta de la de los otros procesos.
    rows, wrong, ambiguous = [], [], []
    for prefix, upstream, methods in routes:
        via = responses[f"http://frontend:80/{prefix}"]
        direct = responses.get(f"http://{upstream}/{prefix}")
        others = [a for a in apis if f"{a}:8080" != upstream]
        same_as = [a for a in others if fingerprint(responses[f"http://{a}:8080/{prefix}"]) == fingerprint(via)]
        reached = direct is not None and fingerprint(direct) == fingerprint(via)
        detail = (f"/{prefix} → {upstream} (config, métodos: {methods}): gateway {via[0]} {via[2]}"
                  + (f" = directo {direct[0]} {direct[2]}" if reached else
                     f" ≠ directo {direct[0]} {direct[2]}" if direct else " (upstream no es un proceso de API)")
                  + "".join(f"; {a} {responses[f'http://{a}:8080/{prefix}'][0]} "
                            f"{responses[f'http://{a}:8080/{prefix}'][2]}" for a in others))
        rows.append(detail)
        if not reached:
            wrong.append(detail + (f" — la respuesta del gateway es la de {', '.join(same_as)}" if same_as else
                                   " — el gateway respondió la app (nginx no enruta el prefijo)"
                                   if via[2] == "text/html" else ""))
        elif same_as:
            ambiguous.append(detail + f" (igual que {', '.join(same_as)}: no se distingue el dominio)")
        probe(SECTIONS[2], f"GET http://frontend:80/{prefix}",
              f"igual a {upstream}" + (f", distinta de {', '.join(others)}" if others else ""),
              f"{via[0]} {via[2]}" + (", igual al upstream" if reached else ", distinta del upstream")
              + (f", igual a {', '.join(same_as)}" if same_as else ""),
              via[1], FAIL if not reached else WARN if same_as else OK)
    status = FAIL if wrong or not routes else WARN if ambiguous else OK
    summary = (f"{len(routes) - len(wrong)} de {len(routes)} prefijos de `{NGINX_CONF}` llegan al upstream "
               "que dice la config" if routes else f"no se encontraron reglas `proxy_pass` en {NGINX_CONF}")
    checks.append(Check(SECTIONS[2], "Enrutamiento del gateway (nginx del frontend)", status, summary,
                        items=wrong + ambiguous + [r for r in rows if not any(x.startswith(r) for x in wrong + ambiguous)]))
    return checks


def check_public() -> list[Check]:
    """Pide /healthz por el host público del Ingress desde este equipo; devuelve una comprobación."""
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
        request = urllib.request.Request(url, headers={"X-Request-ID": f"{REQUEST_ID_PREFIX}-publico"})
        with urllib.request.urlopen(request, timeout=20, context=context) as resp:
            code = resp.status
    except urllib.error.HTTPError as exc:
        code = exc.code
    except (urllib.error.URLError, OSError) as exc:
        probe(s, f"GET {url} (desde este equipo)", "200", "sin respuesta", str(getattr(exc, "reason", exc)), False)
        return [Check(s, "Acceso público", FAIL, f"{url}: {getattr(exc, 'reason', exc)}")]
    ms = (time.monotonic() - started) * 1000
    probe(s, f"GET {url} (desde este equipo)", "200", f"{code} en {ms:.0f} ms", "", code == 200)
    return [Check(s, "Acceso público", OK if code == 200 else FAIL,
                  f"{url} → HTTP {code} en {ms:.0f} ms (certificado de la plataforma, no de confianza)")]


def check_postgres() -> list[Check]:
    """Comprueba que PostgreSQL responde y compara schema_migrations con migration_order.txt."""
    s = SECTIONS[3]
    rows = sql("SELECT version, (SELECT count(*) FROM product WHERE active) AS productos FROM schema_migrations")
    applied = {r["version"] for r in rows}
    manifest = [line.strip() for line in (ROOT / "backend/database/migration_order.txt").read_text(encoding="utf-8")
                .splitlines() if line.strip().endswith(".sql")]
    missing = [m for m in manifest if m not in applied]
    extra = sorted(applied - set(manifest))
    products = rows[0]["productos"] if rows else 0
    probe(s, "SQL: SELECT version FROM schema_migrations (READ ONLY)",
          f"las {len(manifest)} del manifiesto", f"{len(applied)} aplicadas, {len(missing)} faltan",
          ", ".join(missing), not missing)
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
    """Compara el uso de la cuota del namespace con el umbral y revisa que los PVC estén Bound."""
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
        probe(s, f"kubectl get resourcequota {quota['metadata']['name']}",
              f"cada recurso bajo el {QUOTA_WARN:.0%}", f"{len(high)} sobre el umbral", "; ".join(high),
              WARN if high else OK)
        checks.append(Check(s, f"Cuota `{quota['metadata']['name']}`", WARN if high else OK,
                            "; ".join(rows), items=high))
    pvcs = kget("pvc")
    unbound = [f"{p['metadata']['name']}: {p['status'].get('phase')}"
               for p in pvcs if p["status"].get("phase") != "Bound"]
    probe(s, "kubectl get pvc", "todos Bound", f"{len(pvcs) - len(unbound)} de {len(pvcs)}",
          "; ".join(unbound), not unbound)
    checks.append(Check(s, "Volúmenes (PVC)", FAIL if unbound else OK,
                        "todos en Bound" if not unbound else f"{len(unbound)} sin enlazar", items=unbound))
    return checks


# ---------------------------------------------------------------- salida

def print_report(checks: list[Check], header: dict[str, str], detail: bool) -> None:
    """Muestra en la terminal el encabezado, las comprobaciones por sección y, con `detail`, cada pedido."""
    for key, value in header.items():
        print(f"{key}: {value}")
    for section in SECTIONS:
        print(f"\n== {section}")
        for c in (c for c in checks if c.section == section):
            print(f"  [{c.status}] {c.name}: {c.detail}")
            for item in c.items:
                print(f"      - {item}")
    if detail:
        print("\n== Pedidos realizados")
        for p in PROBES:
            print(f"  [{p['estado']}] {p['pedido']}\n      esperado: {p['esperado']} | obtenido: {p['obtenido']}"
                  + (f" | {p['extracto']}" if p["extracto"] else ""))
    fails = sum(c.status == FAIL for c in checks)
    warns = sum(c.status == WARN for c in checks)
    print(f"\nResultado: {len(checks) - fails - warns} OK, {fails} fallas, {warns} avisos")


def probes_markdown() -> list[str]:
    """Arma la tabla Markdown con cada pedido hecho (lo esperado y lo obtenido)."""
    cell = lambda text: text.replace("|", "\\|")  # noqa: E731
    lines = ["## Pedidos realizados", "",
             "Cada consulta que hizo el script, con lo esperado y lo obtenido.", "",
             "| Estado | Pedido | Esperado | Obtenido | Extracto de la respuesta |", "| --- | --- | --- | --- | --- |"]
    lines += [f"| {ICON[p['estado']]} | `{cell(p['pedido'])}` | {cell(p['esperado'])} | {cell(p['obtenido'])} | "
              f"{('`' + cell(p['extracto']) + '`') if p['extracto'] else ''} |" for p in PROBES]
    return lines + [""]


def write_markdown(path: Path, checks: list[Check], header: dict[str, str], detail: bool) -> None:
    """Escribe en `path` el resultado en Markdown; con `detail` suma la tabla de pedidos."""
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
    if detail:
        lines += probes_markdown()
    path.write_text("\n".join(lines), encoding="utf-8")


def main() -> int:
    """Lee los argumentos, corre las comprobaciones y escribe --md/--json.

    Devuelve 0 si todo pasa, 1 si alguna falla y 2 si no se pudo consultar el clúster."""
    parser = argparse.ArgumentParser(description="Health check del despliegue de SOLOServis en el clúster.")
    parser.add_argument("--md", type=Path, help="guarda el resultado en Markdown")
    parser.add_argument("--detalle", action="store_true",
                        help="muestra cada pedido hecho, con lo esperado y lo obtenido")
    parser.add_argument("--json", type=Path, help="guarda el resultado en JSON (lo usa reporte.py --salud)")
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
    print_report(checks, header, args.detalle)
    if args.md:
        write_markdown(args.md, checks, header, args.detalle)
        print(f"Markdown: {args.md}")
    if args.json:
        args.json.write_text(json.dumps({"header": header, "checks": [c.__dict__ for c in checks],
                                         "probes": PROBES}, ensure_ascii=False, indent=2), encoding="utf-8")
    return 1 if any(c.status == FAIL for c in checks) else 0


if __name__ == "__main__":
    sys.exit(main())
