#!/usr/bin/env python3
"""Seguimiento de errores del despliegue en el cluster (TAL-219).

Junta los errores que deja SOLOServis en el cluster y los agrupa para ver
cuales se repiten y de donde vienen:

  - AccessLog de los procesos Go (comercial, usuarios, ingestion): pedidos con
    status >= 400, y las lineas WARN/ERROR con su request_id (WriteInternal,
    Recover, ingestion).
  - nginx del frontend (gateway): pedidos con status >= 400 y su log de errores.
  - scraper-api: pedidos >= 400, WARNING/ERROR y Tracebacks.
  - PostgreSQL: lineas ERROR/FATAL del log, corridas fallidas o con
    records_failed en scrape_run, y alertas scraper_degraded (TAL-211).
  - SQLite del scraper: sus corridas fallidas (una version del scraper anterior
    a TAL-172 no las reporta a ingestion, asi que solo quedan ahi).
  - Kubernetes: reinicios de contenedores, Jobs fallidos y eventos Warning.

Cada error se agrupa por origen, ruta (con los ids normalizados), code y causa.
Un grupo es recurrente si aparece --min veces o mas. Se muestra con su primer y
ultimo momento y un request_id (o id de corrida) de ejemplo.

Trafico de pruebas: lo que hace este equipo (pod de test, salud.py) tambien
deja 4xx. Se marca como prueba un pedido con X-Request-ID `salud-...` o
`carga-...` (los manda carga.py, TAL-206) o que cae
a menos de --ventana segundos de un pedido interno (curl/Wget sin
X-Forwarded-For) en el nginx o del Job de smoke test. Es una heuristica: un
error real en esa ventana tambien quedaria marcado como prueba.

Los logs de los pods solo cubren desde que arranco cada contenedor (y el
anterior, si se reinicio): un redeploy borra la historia. Las corridas y las
alertas vienen de la base y cubren --dias dias.

Es de solo lectura: kubectl get/logs/exec, una sesion READ ONLY en PostgreSQL y
el SQLite del scraper abierto con mode=ro. No usa el token de servicio. Solo la
libreria estandar de Python 3.9+.

    python scripts/testing/errores.py
    python scripts/testing/errores.py --md errores.md     # evidencia en Markdown
    python scripts/testing/errores.py --dias 30 --min 3
    python scripts/testing/reporte.py --errores           # dentro del reporte de testing

Sale con codigo 1 si hay errores graves recurrentes fuera de las pruebas (5xx,
lineas ERROR, reinicios, Jobs o corridas fallidas).
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import re
import sys
from dataclasses import dataclass
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from salud import API_SERVICES, ClusterError, kget, kubectl, sql  # noqa: E402

UTC = dt.timezone.utc
# X-Request-ID de nuestras pruebas: salud.py (TAL-164) y carga.py (TAL-206).
SALUD_PREFIX = ("salud-", "carga-")
TEST_JOB = "soloservis-smoke-test"

# Fuentes, en el orden en que se muestran.
LOGS, NGINX, SCRAPER, POSTGRES, SQLITE, K8S = ("log de pods", "nginx (gateway)", "scraper-api",
                                               "PostgreSQL", "SQLite del scraper", "Kubernetes")


@dataclass
class Event:
    """Un error observado, antes de agruparlo."""

    source: str
    origin: str
    what: str
    code: str
    cause: str
    when: dt.datetime | None
    example: str = ""
    grave: bool = False
    test: bool = False
    count: int = 1


@dataclass
class Group:
    """Errores iguales (misma fuente, origen, ruta, code y causa)."""

    source: str
    origin: str
    what: str
    code: str
    cause: str
    grave: bool
    count: int = 0
    tests: int = 0
    first: dt.datetime | None = None
    last: dt.datetime | None = None
    example: str = ""
    example_is_test: bool = True

    def add(self, event: Event) -> None:
        """Suma un evento: cuenta, ventana de tiempo y ejemplo.

        El ejemplo es la ocurrencia más reciente, prefiriendo una que no sea de pruebas."""
        self.count += event.count
        self.tests += event.count if event.test else 0
        newest = event.when is None or self.last is None or event.when >= self.last
        if event.when:
            self.first = min(self.first or event.when, event.when)
            self.last = max(self.last or event.when, event.when)
        if event.example and (not self.example or (self.example_is_test and not event.test)
                              or (newest and self.example_is_test == event.test)):
            self.example, self.example_is_test = event.example, event.test

    @property
    def real(self) -> int:
        """Ocurrencias que no son trafico de pruebas."""
        return self.count - self.tests


# ---------------------------------------------------------------- normalizacion

UUID = re.compile(r"[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}")
HEX_ID = re.compile(r"\b[0-9a-f]{16,}\b")
NUMBER = re.compile(r"\d+")


def norm_path(path: str) -> str:
    """Ruta sin query string, con los ids (UUID, numeros, hex) como {id}."""
    parts = []
    for part in path.split("?", 1)[0].split("/"):
        if UUID.fullmatch(part) or part.isdigit() or HEX_ID.fullmatch(part):
            part = "{id}"
        parts.append(part)
    return "/".join(parts) or "/"


def norm_text(text: str, limit: int = 160) -> str:
    """Mensaje comparable entre ocurrencias: sin UUID, ids ni numeros, en una linea."""
    text = UUID.sub("{uuid}", text)
    text = HEX_ID.sub("{id}", text)
    text = NUMBER.sub("N", text)
    text = " ".join(text.split())
    return text[:limit] + ("…" if len(text) > limit else "")


def parse_time(raw: str | None) -> dt.datetime | None:
    """Fecha RFC 3339 (con nanosegundos, como las de kubectl) o 'AAAA-MM-DD HH:MM:SS' a UTC."""
    if not raw:
        return None
    raw = str(raw).strip().replace(" ", "T", 1)
    match = re.match(r"(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d)(\.\d+)?(Z|[+-]\d\d:?\d\d)?", raw)
    if not match:
        return None
    frac = (match.group(2) or "")[:7]
    zone = match.group(3) or "+00:00"
    zone = "+00:00" if zone == "Z" else zone if ":" in zone else zone[:3] + ":" + zone[3:]
    return dt.datetime.fromisoformat(match.group(1) + frac + zone).astimezone(UTC)


def short(moment: dt.datetime | None) -> str:
    """Fecha corta en hora local, para las tablas."""
    return moment.astimezone().strftime("%Y-%m-%d %H:%M") if moment else "?"


# ---------------------------------------------------------------- logs de pods

NGINX_ACCESS = re.compile(r'^(\S+) - \S+ \[[^\]]+\] "(\S+) (\S+)[^"]*" (\d{3}) \d+ "[^"]*" "([^"]*)" "([^"]*)"')
NGINX_ERROR = re.compile(r"^\d{4}/\d\d/\d\d \d\d:\d\d:\d\d \[(warn|error|crit|alert|emerg)\] \d+#\d+: (?:\*\d+ )?(.*)")
UVICORN = re.compile(r'"(\S+) (\S+) HTTP/[\d.]+" (\d{3})')
PYLOG = re.compile(r"^\d{4}-\d\d-\d\d \d\d:\d\d:\d\d,\d+ (WARNING|ERROR|CRITICAL) (\S+): (.*)")
PGLOG = re.compile(r"UTC \[\d+\] (ERROR|FATAL|PANIC):\s+(.*)")


def app_name(pod: dict) -> str:
    """Nombre del Deployment o StatefulSet dueno del pod (comercial, postgres...)."""
    name = pod["metadata"]["name"]
    owner = (pod["metadata"].get("ownerReferences") or [{}])[0].get("kind")
    if owner == "ReplicaSet":
        return name.rsplit("-", 2)[0]
    if owner == "StatefulSet":
        return name.rsplit("-", 1)[0]
    return name


def parse_go(origin: str, stamp: dt.datetime | None, line: str) -> Event | None:
    """Linea JSON de slog: AccessLog con status >= 400 o un WARN/ERROR propio."""
    if not line.startswith("{"):
        return None
    try:
        entry = json.loads(line)
    except ValueError:
        return None
    request_id = entry.get("request_id", "")
    if entry.get("msg") == "http_request":
        status = int(entry.get("status", 0))
        if status < 400:
            return None
        return Event(LOGS, origin, f"{entry.get('method')} {norm_path(entry.get('path', ''))}", str(status),
                     "", stamp, request_id, grave=status >= 500, test=request_id.startswith(SALUD_PREFIX))
    level = entry.get("level", "")
    if level not in ("WARN", "ERROR"):
        return None
    where = f"{entry['method']} {norm_path(entry['path'])}" if entry.get("path") else "-"
    detail = entry.get("error") or entry.get("panic") or ""
    example = request_id or str(entry.get("run_id") or entry.get("runId") or "")
    return Event(LOGS, origin, where, str(entry.get("msg")), norm_text(str(detail)), stamp, example,
                 grave=level == "ERROR", test=request_id.startswith(SALUD_PREFIX))


def parse_nginx(stamp: dt.datetime | None, line: str, markers: list[dt.datetime]) -> Event | None:
    """Linea del nginx del frontend. Anota los pedidos internos como marcas de prueba."""
    match = NGINX_ACCESS.match(line)
    if match:
        _client, method, path, status, agent, forwarded = match.groups()
        internal = forwarded in ("-", "127.0.0.1") and agent.startswith(("curl/", "Wget"))
        if internal and stamp:
            markers.append(stamp)
        if int(status) < 400:
            return None
        return Event(NGINX, "frontend", f"{method} {norm_path(path)}", status, "", stamp,
                     grave=int(status) >= 500, test=internal)
    match = NGINX_ERROR.match(line)
    if match:
        level, message = match.groups()
        return Event(NGINX, "frontend", "-", f"nginx {level}", norm_text(message), stamp,
                     grave=level != "warn")
    return None


def parse_python(origin: str, stamp: dt.datetime | None, line: str) -> Event | None:
    """Linea de uvicorn (pedido >= 400) o de logging (WARNING/ERROR) del scraper."""
    match = PYLOG.match(line)
    if match:
        level, logger, message = match.groups()
        return Event(SCRAPER, origin, logger, level, norm_text(message), stamp, grave=level != "WARNING")
    if line.startswith("Traceback"):
        return Event(SCRAPER, origin, "-", "Traceback", "excepción sin capturar (ver el log del pod)", stamp,
                     grave=True)
    match = UVICORN.search(line)
    if match and int(match.group(3)) >= 400:
        method, path, status = match.groups()
        return Event(SCRAPER, origin, f"{method} {norm_path(path)}", status, "", stamp, grave=int(status) >= 500)
    return None


def parse_postgres(stamp: dt.datetime | None, line: str) -> Event | None:
    """Linea ERROR/FATAL/PANIC del log de PostgreSQL."""
    match = PGLOG.search(line)
    if not match:
        return None
    level, message = match.groups()
    return Event(POSTGRES, "postgres", "log", level, norm_text(message), stamp, grave=level != "ERROR")


def read_logs(pod: dict, container: str, previous: bool) -> list[tuple[dt.datetime | None, str]]:
    """Lineas (momento, texto) del contenedor. Si no hay log anterior devuelve []."""
    args = ["logs", pod["metadata"]["name"], "-c", container, "--timestamps"]
    if previous:
        args.append("--previous")
    try:
        out = kubectl(args, timeout=180)
    except ClusterError:
        if previous:
            return []
        raise
    lines = []
    for raw in out.splitlines():
        stamp, _, text = raw.partition(" ")
        lines.append((parse_time(stamp), text.strip()))
    return lines


def collect_pods(pods: list[dict], markers: list[dt.datetime]) -> tuple[list[Event], dict[str, dt.datetime]]:
    """Errores de los logs de cada pod de la app y de sus reinicios.

    Devuelve los eventos y, por pod, desde cuando cubren sus logs."""
    events: list[Event] = []
    since: dict[str, dt.datetime] = {}
    for pod in pods:
        owners = pod["metadata"].get("ownerReferences") or [{}]
        if owners[0].get("kind") == "Job":
            continue
        app = app_name(pod)
        statuses = {c["name"]: c for c in pod["status"].get("containerStatuses", [])}
        for container in pod["spec"]["containers"]:
            status = statuses.get(container["name"], {})
            restarts = status.get("restartCount", 0)
            if restarts:
                term = (status.get("lastState") or {}).get("terminated") or {}
                events.append(Event(K8S, app, f"contenedor {container['name']}", "reinicio",
                                    f"{term.get('reason', '?')} (código {term.get('exitCode', '?')})",
                                    parse_time(term.get("finishedAt")), pod["metadata"]["name"], grave=True,
                                    count=restarts))
            lines = read_logs(pod, container["name"], previous=False)
            if restarts:
                lines = read_logs(pod, container["name"], previous=True) + lines
            stamps = [s for s, _ in lines if s]
            if stamps:
                since[app] = min(stamps)
            for stamp, text in lines:
                if app in API_SERVICES:
                    event = parse_go(app, stamp, text)
                elif app == "frontend":
                    event = parse_nginx(stamp, text, markers)
                elif app == "postgres":
                    event = parse_postgres(stamp, text)
                else:
                    event = parse_python(app, stamp, text)
                if event:
                    events.append(event)
    return events, since


GRAVE_EVENTS = ("BackOff", "Failed", "FailedScheduling", "Unhealthy", "OOMKilling", "Evicted")


def collect_k8s(markers: list[dt.datetime]) -> list[Event]:
    """Jobs fallidos y eventos Warning. El Job de smoke test suma marcas de prueba."""
    events: list[Event] = []
    for job in kget("jobs"):
        status = job["status"]
        name = job["metadata"]["name"]
        if name.startswith(TEST_JOB):
            markers += [m for m in (parse_time(status.get("startTime")),
                                    parse_time(status.get("completionTime"))) if m]
        if status.get("failed") and not status.get("succeeded"):
            reason = next((c.get("message") or c.get("reason") for c in status.get("conditions", [])
                           if c.get("type") == "Failed"), "sin motivo")
            base = re.sub(r"-\d{6,}$", "", name)  # los Jobs de un CronJob llevan el minuto
            events.append(Event(K8S, base, "Job", "job fallido", norm_text(str(reason)),
                                parse_time(status.get("startTime")), name, grave=True))
    for item in kget("events"):
        if item.get("type") != "Warning":
            continue
        obj = item.get("involvedObject", {})
        when = parse_time(item.get("lastTimestamp") or item.get("eventTime")
                          or item["metadata"].get("creationTimestamp"))
        events.append(Event(K8S, re.sub(r"-[a-z0-9]{5,10}(-[a-z0-9]{5})?$", "", obj.get("name", "?")),
                            obj.get("kind", "?"), item.get("reason", "?"), norm_text(item.get("message", "")),
                            when, obj.get("name", ""), grave=item.get("reason") in GRAVE_EVENTS,
                            count=item.get("count") or 1))
    return events


# ---------------------------------------------------------------- corridas del scraper

def collect_runs(days: int) -> list[Event]:
    """Corridas fallidas o con ítems fallidos en scrape_run, y alertas scraper_degraded."""
    events: list[Event] = []
    rows = sql(f"""
        SELECT r.id, coalesce(ds.name, 'config ' || r.scraper_config_id) AS fuente, r.status,
               r.started_at, coalesce(r.records_found, 0) AS f, coalesce(r.records_processed, 0) AS p,
               coalesce(r.records_failed, 0) AS x, coalesce(r.error_message, '') AS error
        FROM scrape_run r
        LEFT JOIN scraper_config c ON c.id = r.scraper_config_id
        LEFT JOIN data_source ds ON ds.id = c.data_source_id
        WHERE (r.status = 'failed' OR coalesce(r.records_failed, 0) > 0)
          AND r.started_at >= now() - interval '{int(days)} days'
        ORDER BY r.id""")
    for row in rows:
        if row["status"] == "failed":
            cause = norm_text(row["error"]) or "sin error_message"
        elif row["p"] == 0:
            cause = "success con todos los ítems fallidos (records_processed = 0)"
        else:
            cause = "success con ítems fallidos (records_failed > 0)"
        events.append(Event(POSTGRES, row["fuente"], "scrape_run", row["status"], cause,
                            parse_time(row["started_at"]), f"corrida {row['id']}",
                            grave=row["status"] == "failed" or row["p"] == 0))
    alerts = sql(f"""
        SELECT id, created_at, coalesce(title, '') AS title FROM notification
        WHERE type = 'scraper_degraded' AND created_at >= now() - interval '{int(days)} days'
        ORDER BY id""")
    for row in alerts:
        events.append(Event(POSTGRES, "ingestion", "notification", "scraper_degraded", norm_text(row["title"]),
                            parse_time(row["created_at"]), f"notificación {row['id']}"))
    return events


# Corre dentro del pod scraper-api: lee su SQLite en modo solo lectura y
# devuelve las corridas con problemas en JSON. No escribe nada.
SQLITE_SCRIPT = r"""
import json, os, sqlite3
path = os.environ.get("SCRAPER_DB_PATH") or "/data/scraper.db"
con = sqlite3.connect(f"file:{path}?mode=ro", uri=True)
cols = {row[1].lower() for row in con.execute("PRAGMA table_info(SCRAPE_RUN)")}
extra = [c for c in ("ingestion_error", "items_failed", "ingestion_run_id") if c in cols]
query = ("SELECT r.id, ds.name, r.status, r.started_at, coalesce(r.error_message, '')"
         + "".join(f", r.{c}" for c in extra)
         + " FROM SCRAPE_RUN r JOIN SCRAPER_CONFIG c ON c.id = r.scraper_config_id"
         + " JOIN DATA_SOURCE ds ON ds.id = c.data_source_id ORDER BY r.id")
rows = []
for row in con.execute(query):
    item = dict(zip(["id", "fuente", "status", "started_at", "error"] + extra, row))
    if item["status"] == "failed" or item.get("ingestion_error") or (item.get("items_failed") or 0) > 0:
        rows.append(item)
con.close()
print(json.dumps(rows))
"""


def collect_sqlite(days: int) -> tuple[list[Event], str]:
    """Corridas con problemas del SQLite del scraper. Devuelve (eventos, aviso)."""
    try:
        out = kubectl(["exec", "-i", "deploy/scraper-api", "--", "python", "-"], stdin=SQLITE_SCRIPT)
        rows = json.loads(out.strip().splitlines()[-1])
    except (ClusterError, ValueError, IndexError) as exc:
        return [], f"no se pudo leer el SQLite del scraper: {exc}"
    cutoff = dt.datetime.now(UTC) - dt.timedelta(days=days)
    events = []
    for row in rows:
        when = parse_time(row.get("started_at"))
        if when and when < cutoff:
            continue
        if row["status"] == "failed":
            code, cause = "failed", norm_text(row["error"]) or "sin error_message"
        elif row.get("ingestion_error"):
            code, cause = "ingestion", norm_text(row["ingestion_error"])
        else:
            code, cause = "items_failed", "ingestion rechazó ítems de la corrida"
        sent = "sí" if row.get("ingestion_run_id") else "no"
        events.append(Event(SQLITE, row["fuente"], f"SCRAPE_RUN (en scrape_run: {sent})", code, cause, when,
                            f"corrida {row['id']}", grave=code == "failed"))
    return events, ""


# ---------------------------------------------------------------- agrupacion

def mark_tests(events: list[Event], markers: list[dt.datetime], window: int) -> None:
    """Marca como prueba los errores HTTP que caen cerca de un pedido interno."""
    marks = sorted(markers)
    delta = dt.timedelta(seconds=window)
    for event in events:
        if event.test or event.when is None or event.source not in (LOGS, NGINX, SCRAPER):
            continue
        if not event.code.isdigit():
            continue
        if any(abs(event.when - m) <= delta for m in marks):
            event.test = True


def group(events: list[Event]) -> list[Group]:
    """Agrupa los eventos iguales y los ordena: graves primero, luego por cantidad real."""
    groups: dict[tuple, Group] = {}
    for event in events:
        key = (event.source, event.origin, event.what, event.code, event.cause)
        if key not in groups:
            groups[key] = Group(*key, grave=event.grave)
        groups[key].add(event)
    return sorted(groups.values(), key=lambda g: (not g.grave, -g.real, -g.count, g.origin, g.what))


def classify(groups: list[Group], minimum: int) -> tuple[list[Group], list[Group], list[Group]]:
    """Separa en recurrentes, aislados y trafico de pruebas (todas sus ocurrencias lo son)."""
    tests = [g for g in groups if g.real == 0]
    recurrent = [g for g in groups if g.real >= minimum]
    single = [g for g in groups if 0 < g.real < minimum]
    return recurrent, single, tests


# ---------------------------------------------------------------- salida

def row_text(g: Group) -> str:
    """Una linea de texto por grupo."""
    tests = f" (+{g.tests} de pruebas)" if g.tests and g.real else ""
    cause = f" — {g.cause}" if g.cause else ""
    return (f"{g.real if g.real else g.count:>4}x{tests} [{g.source}] {g.origin} | {g.what} | {g.code}{cause}\n"
            f"        {short(g.first)} → {short(g.last)}" + (f" | ejemplo: {g.example}" if g.example else ""))


def print_report(header: dict[str, str], recurrent: list[Group], single: list[Group], tests: list[Group],
                 notes: list[str]) -> None:
    """Salida en texto para la terminal."""
    for key, value in header.items():
        print(f"{key}: {value.replace('`', '')}")
    for note in notes:
        print(f"AVISO: {note}")
    for title, rows in (("Errores recurrentes", recurrent), ("Una sola vez", single),
                        ("Tráfico de pruebas (no cuenta)", tests)):
        print(f"\n== {title} ({len(rows)})")
        for g in rows:
            print(("  ! " if g.grave and rows is not tests else "    ") + row_text(g))
    grave = [g for g in recurrent if g.grave]
    print(f"\nResultado: {len(recurrent)} recurrentes ({len(grave)} graves), {len(single)} aislados, "
          f"{len(tests)} de pruebas")


def table(rows: list[Group], with_tests: bool = True) -> list[str]:
    """Tabla Markdown de grupos."""
    cell = lambda text: str(text).replace("|", "\\|")  # noqa: E731
    lines = ["| Veces | Fuente | Origen | Qué | Code | Causa | Primera | Última | Ejemplo |",
             "| --- | --- | --- | --- | --- | --- | --- | --- | --- |"]
    for g in rows:
        times = str(g.real or g.count) + (f" (+{g.tests} pruebas)" if with_tests and g.tests and g.real else "")
        lines.append(f"| {('**' + times + '**') if g.grave else times} | {g.source} | {cell(g.origin)} | "
                     f"`{cell(g.what)}` | `{cell(g.code)}` | {cell(g.cause)} | {short(g.first)} | "
                     f"{short(g.last)} | {('`' + cell(g.example) + '`') if g.example else ''} |")
    return lines


def write_markdown(path: Path, header: dict[str, str], recurrent: list[Group], single: list[Group],
                   tests: list[Group], notes: list[str]) -> None:
    """Evidencia en Markdown. Las filas en negrita son graves (5xx, ERROR, reinicios, corridas fallidas)."""
    lines = ["# Seguimiento de errores", "", "| Campo | Valor |", "| --- | --- |"]
    lines += [f"| {k} | {v} |" for k, v in header.items()]
    lines += [""] + [f"> Aviso: {n}" for n in notes] + ([""] if notes else [])
    grave = [g for g in recurrent if g.grave]
    summary = (f"{len(recurrent)} errores recurrentes ({len(grave)} graves), {len(single)} aislados y "
               f"{len(tests)} grupos de tráfico de pruebas. En negrita, los graves.")
    lines += ["## Resultado", "", summary, ""]
    for title, rows in (("Errores recurrentes", recurrent), ("Una sola vez", single)):
        lines += [f"## {title}", ""] + (table(rows) if rows else ["Ninguno."]) + [""]
    lines += ["## Tráfico de pruebas", "",
              "Errores que dejan el pod de test y `salud.py` (no cuentan como recurrentes).", ""]
    lines += (table(tests, with_tests=False) if tests else ["Ninguno."]) + [""]
    path.write_text("\n".join(lines), encoding="utf-8")


def main() -> int:
    """Junta, agrupa y muestra los errores; escribe --md y --json si se piden."""
    parser = argparse.ArgumentParser(description="Errores recurrentes del despliegue de SOLOServis y su origen.")
    parser.add_argument("--md", type=Path, help="guarda el resultado en Markdown")
    parser.add_argument("--json", type=Path, help="guarda el resultado en JSON (lo usa reporte.py --errores)")
    parser.add_argument("--dias", type=int, default=14, help="días de corridas y alertas a revisar (14)")
    parser.add_argument("--min", type=int, default=2, help="veces para considerar un error recurrente (2)")
    parser.add_argument("--ventana", type=int, default=120,
                        help="segundos alrededor de un pedido interno que se toman como prueba (120)")
    args = parser.parse_args()
    for stream in (sys.stdout, sys.stderr):
        if hasattr(stream, "reconfigure"):
            stream.reconfigure(encoding="utf-8")

    started = dt.datetime.now().astimezone()
    markers: list[dt.datetime] = []
    notes: list[str] = []
    try:
        pods = kget("pods")
        events, since = collect_pods(pods, markers)
        events += collect_k8s(markers)
        events += collect_runs(args.dias)
        sqlite_events, note = collect_sqlite(args.dias)
    except ClusterError as exc:
        print(f"ERROR: no se pudo consultar el clúster: {exc}", file=sys.stderr)
        return 2
    events += sqlite_events
    if note:
        notes.append(note)
    mark_tests(events, markers, args.ventana)
    recurrent, single, tests = classify(group(events), args.min)

    namespace = pods[0]["metadata"]["namespace"] if pods else "?"
    oldest = min(since.values()) if since else None
    header = {
        "Fecha": started.strftime("%Y-%m-%d %H:%M %z"),
        "Namespace": f"`{namespace}`",
        "Logs de pods desde": f"{short(oldest)} (arranque de los contenedores; un redeploy borra lo anterior)",
        "Corridas y alertas": f"últimos {args.dias} días",
        "Recurrente": f"{args.min} o más veces fuera de las pruebas",
        "Generado con": "`python scripts/testing/errores.py`",
    }
    print_report(header, recurrent, single, tests, notes)
    if args.md:
        write_markdown(args.md, header, recurrent, single, tests, notes)
        print(f"Markdown: {args.md}")
    if args.json:
        dump = lambda rows: [{**g.__dict__, "first": short(g.first), "last": short(g.last),  # noqa: E731
                              "real": g.real} for g in rows]
        args.json.write_text(json.dumps({"header": header, "notes": notes, "recurrent": dump(recurrent),
                                         "single": dump(single), "tests": dump(tests)},
                                        ensure_ascii=False, indent=2), encoding="utf-8")
    return 1 if any(g.grave for g in recurrent) else 0


if __name__ == "__main__":
    sys.exit(main())
