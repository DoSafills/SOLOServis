#!/usr/bin/env python3
"""Reporte de testing de SOLOServis (TAL-168).

Corre las suites de pruebas del repositorio y registra resultados, errores y
estados en un reporte Markdown. Solo usa la libreria estandar de Python 3.9+.

    python scripts/testing/reporte.py                    # todas las suites
    python scripts/testing/reporte.py --suites backend   # solo una
    python scripts/testing/reporte.py --postgres-docker  # con tests de integracion
    python scripts/testing/reporte.py --smoke            # suma el pod de test de k8s

Sale con codigo 1 si alguna prueba falla, para poder usarlo en CI.
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import re
import shutil
import subprocess
import sys
import time
import xml.etree.ElementTree as ET
from dataclasses import dataclass, field
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SUITES = ("backend", "scraper", "crawler", "frontend")
MAX_ERROR_LINES = 25
PG_IMAGE = "postgres:18"
PG_CONTAINER = "soloservis_reporte_pg"
PG_PORT = "55432"

OK, FAIL, SKIPPED, NOT_RUN = "aprobada", "fallida", "omitida", "no ejecutada"
ICON = {OK: "✅", FAIL: "❌", SKIPPED: "⏭", NOT_RUN: "⚪"}


@dataclass
class Case:
    name: str
    status: str  # OK | FAIL | SKIPPED
    detail: str = ""


@dataclass
class Suite:
    key: str
    title: str
    command: str = ""
    status: str = NOT_RUN
    reason: str = ""
    seconds: float = 0.0
    cases: list[Case] = field(default_factory=list)

    def count(self, status: str) -> int:
        return sum(1 for case in self.cases if case.status == status)


def run(cmd: list[str], cwd: Path, env: dict[str, str] | None = None, timeout: int = 1800):
    """Ejecuta un comando y devuelve (codigo, salida). Nunca lanza por el codigo."""
    try:
        proc = subprocess.run(
            cmd,
            cwd=str(cwd),
            env=env,
            capture_output=True,
            text=True,
            encoding="utf-8",
            errors="replace",
            timeout=timeout,
            shell=False,
        )
        return proc.returncode, (proc.stdout or "") + (proc.stderr or "")
    except subprocess.TimeoutExpired:
        return 124, f"tiempo agotado tras {timeout}s"
    except OSError as exc:
        return 127, str(exc)


def which(name: str) -> str | None:
    return shutil.which(name)


def trim(text: str, limit: int = MAX_ERROR_LINES, tail: bool = False) -> str:
    """Recorta una salida larga. ``tail`` conserva el final (donde un comando deja el error)."""
    lines = [line.rstrip() for line in text.strip().splitlines() if line.strip()]
    if len(lines) > limit:
        note = f"… ({len(lines) - limit} líneas más en los datos crudos)"
        lines = [note] + lines[-limit:] if tail else lines[:limit] + [note]
    return "\n".join(lines)


# ----------------------------------------------------------------- backend


def suite_backend(raw_dir: Path, database_url: str | None) -> Suite:
    suite = Suite("backend", "Backend (Go)", "go test -json ./...")
    if not which("go"):
        suite.reason = "no se encontró `go` en el PATH"
        return suite
    env = dict(os.environ)
    if database_url:
        env["TEST_DATABASE_URL"] = database_url
    started = time.time()
    code, out = run(["go", "test", "-json", "-count=1", "./..."], ROOT / "backend", env)
    suite.seconds = time.time() - started
    (raw_dir / "backend-go-test.json").write_text(out, encoding="utf-8")

    outputs: dict[tuple[str, str], list[str]] = {}
    build_errors: list[str] = []
    for line in out.splitlines():
        try:
            event = json.loads(line)
        except ValueError:
            if line.strip():
                build_errors.append(line)
            continue
        package = event.get("Package", "").replace("github.com/DoSafills/SOLOServis/backend/", "")
        test = event.get("Test")
        action = event.get("Action")
        if action == "output":
            outputs.setdefault((package, test or ""), []).append(event.get("Output", ""))
            continue
        if test and action in ("pass", "fail", "skip"):
            name = f"{package}: {test}"
            text = "".join(outputs.get((package, test), []))
            if action == "pass":
                suite.cases.append(Case(name, OK))
            elif action == "skip":
                reason = re.search(r"\.go:\d+: (.+)", text)
                suite.cases.append(Case(name, SKIPPED, reason.group(1).strip() if reason else ""))
            else:
                suite.cases.append(Case(name, FAIL, trim(text)))
        elif not test and action == "fail":
            text = "".join(outputs.get((package, ""), []))
            if "[build failed]" in text or "setup failed" in text or not any(
                c.name.startswith(package + ":") for c in suite.cases
            ):
                suite.cases.append(Case(f"{package}: (el paquete no compiló o no arrancó)", FAIL, trim(text)))

    # Un subtest fallido marca tambien al padre: se deja solo la hoja.
    failed = {case.name for case in suite.cases if case.status == FAIL}
    suite.cases = [
        case
        for case in suite.cases
        if not (case.status == FAIL and any(other.startswith(case.name + "/") for other in failed))
    ]
    if build_errors and not suite.cases:
        suite.cases.append(Case("compilación", FAIL, trim("\n".join(build_errors))))
    suite.status = FAIL if suite.count(FAIL) or code != 0 else OK
    if code != 0 and not suite.count(FAIL):
        suite.cases.append(Case("go test", FAIL, trim("\n".join(build_errors) or out[-2000:])))
    return suite


# ------------------------------------------------------------------ pytest


def suite_pytest(key: str, title: str, folder: Path, python: str, raw_dir: Path) -> Suite:
    suite = Suite(key, title, "python -m pytest -q --junitxml=…")
    code, out = run([python, "-c", "import pytest, scrapling"], folder)
    if code != 0:
        suite.reason = (
            f"faltan dependencias en `{python}` (pytest o scrapling): "
            f"instala con `pip install -e \"./{folder.name}[dev]\"` o usa `--python`"
        )
        return suite
    xml_path = raw_dir / f"{key}-junit.xml"
    started = time.time()
    code, out = run(
        [python, "-m", "pytest", "-q", "-p", "no:cacheprovider", f"--junitxml={xml_path}"], folder
    )
    suite.seconds = time.time() - started
    (raw_dir / f"{key}-pytest.txt").write_text(out, encoding="utf-8")
    if not xml_path.exists():
        suite.status = FAIL
        suite.cases.append(Case("pytest", FAIL, trim(out)))
        return suite
    for case in ET.parse(xml_path).getroot().iter("testcase"):
        name = f"{case.get('classname', '')}::{case.get('name', '')}".replace("tests.", "", 1)
        failure = case.find("failure")
        if failure is None:
            failure = case.find("error")
        skipped = case.find("skipped")
        if failure is not None:
            suite.cases.append(Case(name, FAIL, trim(failure.get("message", "") + "\n" + (failure.text or ""))))
        elif skipped is not None:
            suite.cases.append(Case(name, SKIPPED, skipped.get("message", "")))
        else:
            suite.cases.append(Case(name, OK))
    suite.status = FAIL if suite.count(FAIL) or code not in (0, 5) else OK
    return suite


# ---------------------------------------------------------------- frontend


def suite_frontend(raw_dir: Path) -> Suite:
    suite = Suite("frontend", "Frontend (chequeos estáticos)", "pnpm format:check, lint, build, audit")
    pnpm = which("pnpm") or which("pnpm.cmd")
    folder = ROOT / "fronted"
    if not pnpm:
        suite.reason = "no se encontró `pnpm` en el PATH"
        return suite
    if not (folder / "node_modules").exists():
        suite.reason = "falta `fronted/node_modules`: corre `pnpm install` en `fronted/`"
        return suite
    started = time.time()
    log: list[str] = []
    for step in ("format:check", "lint", "build", "audit"):
        code, out = run([pnpm, "run", step], folder, timeout=900)
        log.append(f"### pnpm {step} (código {code})\n{out}")
        suite.cases.append(Case(f"pnpm {step}", OK if code == 0 else FAIL, "" if code == 0 else trim(out, tail=True)))
    suite.seconds = time.time() - started
    (raw_dir / "frontend-check.txt").write_text("\n\n".join(log), encoding="utf-8")
    suite.status = FAIL if suite.count(FAIL) else OK
    return suite


# ------------------------------------------------------------------- smoke


def suite_smoke(raw_dir: Path) -> Suite:
    suite = Suite("smoke", "Kubernetes (pod de test)", "kubectl logs job/soloservis-smoke-test")
    if not which("kubectl"):
        suite.reason = "no se encontró `kubectl` en el PATH"
        return suite
    code, out = run(["kubectl", "logs", "job/soloservis-smoke-test"], ROOT, timeout=60)
    (raw_dir / "smoke-test.txt").write_text(out, encoding="utf-8")
    if code != 0:
        suite.reason = (
            "el Job `soloservis-smoke-test` no existe en el namespace: lo crea la fase 10 del deploy"
            if "NotFound" in out
            else "no se pudo leer el Job `soloservis-smoke-test` (¿hay acceso al clúster?)"
        )
        return suite
    lines = out.splitlines()
    for index, line in enumerate(lines):
        match = re.match(r"^(OK|FALLA)\s+(.+?)\s{2,}(\d{3}.*)$", line.strip())
        if not match:
            continue
        state, name, rest = match.groups()
        detail = ""
        if state == "FALLA":
            extra = lines[index + 1].strip() if index + 1 < len(lines) else ""
            detail = trim(rest + ("\n" + extra if extra else ""))
        suite.cases.append(Case(" ".join(name.split()), OK if state == "OK" else FAIL, detail))
    if not suite.cases:
        suite.reason = "el log del Job no trae verificaciones"
        return suite
    suite.status = FAIL if suite.count(FAIL) else OK
    return suite


# ------------------------------------------------- PostgreSQL desechable


def start_postgres(raw_dir: Path) -> tuple[str | None, str]:
    """Levanta un PostgreSQL 18 desechable con las migraciones. Devuelve (url, motivo)."""
    if not which("docker"):
        return None, "no se encontró `docker`"
    code, _ = run(["docker", "info"], ROOT, timeout=30)
    if code != 0:
        return None, "Docker no está corriendo"
    run(["docker", "rm", "-f", PG_CONTAINER], ROOT, timeout=60)
    code, out = run(
        [
            "docker", "run", "-d", "--name", PG_CONTAINER,
            "-e", "POSTGRES_PASSWORD=reporte", "-e", "POSTGRES_DB=soloservis_test",
            "-p", f"127.0.0.1:{PG_PORT}:5432", "--tmpfs", "/var/lib/postgresql", PG_IMAGE,
        ],
        ROOT,
        timeout=300,
    )
    if code != 0:
        return None, "no se pudo iniciar el contenedor: " + trim(out, 3)
    for _ in range(60):
        code, _ = run(
            ["docker", "exec", PG_CONTAINER, "pg_isready", "-U", "postgres", "-d", "soloservis_test"],
            ROOT, timeout=15,
        )
        if code == 0:
            break
        time.sleep(1)
    else:
        return None, "PostgreSQL no quedó listo en 60 s"
    time.sleep(2)  # el entrypoint reinicia el servidor una vez tras crear la base

    migrations = ROOT / "backend" / "database" / "migrations"
    order = (ROOT / "backend" / "database" / "migration_order.txt").read_text(encoding="utf-8")
    log: list[str] = []
    for name in (line.strip() for line in order.splitlines()):
        if not name.endswith(".sql"):
            continue
        sql = (migrations / name).read_bytes()
        proc = subprocess.run(
            ["docker", "exec", "-i", PG_CONTAINER, "psql", "-X", "-q", "-v", "ON_ERROR_STOP=1",
             "--single-transaction", "-U", "postgres", "-d", "soloservis_test"],
            input=sql, capture_output=True,
        )
        log.append(f"{name}: código {proc.returncode}")
        if proc.returncode != 0:
            (raw_dir / "postgres-migraciones.txt").write_text("\n".join(log), encoding="utf-8")
            return None, f"falló la migración {name}: " + trim(proc.stderr.decode("utf-8", "replace"), 3)
    (raw_dir / "postgres-migraciones.txt").write_text("\n".join(log), encoding="utf-8")
    return f"postgres://postgres:reporte@127.0.0.1:{PG_PORT}/soloservis_test?sslmode=disable", ""


def stop_postgres() -> None:
    run(["docker", "rm", "-f", PG_CONTAINER], ROOT, timeout=60)


# ----------------------------------------------------------------- reporte


def version(cmd: list[str]) -> str:
    code, out = run(cmd, ROOT, timeout=30)
    return out.strip().splitlines()[0] if code == 0 and out.strip() else "no disponible"


def git(*args: str) -> str:
    code, out = run(["git", *args], ROOT, timeout=30)
    return out.strip() if code == 0 else "desconocido"


def duration(seconds: float) -> str:
    return f"{seconds:.1f} s" if seconds < 60 else f"{int(seconds // 60)} min {int(seconds % 60)} s"


def render(suites: list[Suite], meta: dict[str, str]) -> str:
    lines = ["# Reporte de testing", ""]
    lines += ["| Campo | Valor |", "| --- | --- |"]
    lines += [f"| {key} | {value} |" for key, value in meta.items()]
    lines.append("")

    total = {s: sum(suite.count(s) for suite in suites) for s in (OK, FAIL, SKIPPED)}
    failed_suites = [suite for suite in suites if suite.status == FAIL]
    not_run = [suite for suite in suites if suite.status == NOT_RUN]
    verdict = "❌ Hay pruebas fallidas" if failed_suites else "✅ Todas las pruebas ejecutadas pasaron"
    if not_run:
        verdict += f"; {len(not_run)} suite(s) no se ejecutaron"
    lines += ["## Resultado", "", f"**{verdict}.**", ""]
    lines += [
        f"- Pruebas ejecutadas: **{sum(total.values())}**",
        f"- Aprobadas: **{total[OK]}**",
        f"- Fallidas: **{total[FAIL]}**",
        f"- Omitidas: **{total[SKIPPED]}**",
        "",
    ]

    lines += ["## Resumen por suite", ""]
    lines += ["| Suite | Estado | Total | Aprobadas | Fallidas | Omitidas | Duración |",
              "| --- | --- | --- | --- | --- | --- | --- |"]
    for suite in suites:
        if suite.status == NOT_RUN:
            lines.append(f"| {suite.title} | {ICON[NOT_RUN]} {NOT_RUN} | – | – | – | – | – |")
        else:
            lines.append(
                f"| {suite.title} | {ICON[suite.status]} {suite.status} | {len(suite.cases)} | "
                f"{suite.count(OK)} | {suite.count(FAIL)} | {suite.count(SKIPPED)} | {duration(suite.seconds)} |"
            )
    lines.append("")

    if not_run:
        lines += ["## Suites no ejecutadas", "", "| Suite | Motivo |", "| --- | --- |"]
        lines += [f"| {suite.title} | {suite.reason} |" for suite in not_run]
        lines.append("")

    lines += ["## Errores", ""]
    failures = [(suite, case) for suite in suites for case in suite.cases if case.status == FAIL]
    if not failures:
        lines += ["No hubo pruebas fallidas.", ""]
    for suite, case in failures:
        lines += [f"### {ICON[FAIL]} {suite.title} — `{case.name}`", "", "```text",
                  case.detail or "(sin salida)", "```", ""]

    skipped = [(suite, case) for suite in suites for case in suite.cases if case.status == SKIPPED]
    lines += ["## Pruebas omitidas", ""]
    if not skipped:
        lines += ["No se omitió ninguna prueba.", ""]
    else:
        groups: dict[tuple[str, str], int] = {}
        for suite, case in skipped:
            groups[(suite.title, case.detail or "sin motivo declarado")] = (
                groups.get((suite.title, case.detail or "sin motivo declarado"), 0) + 1
            )
        lines += ["| Suite | Motivo | Pruebas |", "| --- | --- | --- |"]
        lines += [f"| {title} | {reason} | {count} |" for (title, reason), count in groups.items()]
        lines.append("")

    lines += ["## Detalle por suite", ""]
    for suite in suites:
        if suite.status == NOT_RUN:
            continue
        lines += [f"### {suite.title}", "", f"Comando: `{suite.command}`", ""]
        if suite.key == "frontend":
            lines += ["El frontend no tiene tests unitarios: se registran sus chequeos estáticos.", ""]
        if len(suite.cases) <= 40:
            lines += ["| Prueba | Estado |", "| --- | --- |"]
            lines += [f"| `{case.name}` | {ICON[case.status]} {case.status} |" for case in suite.cases]
        else:
            groups2: dict[str, dict[str, int]] = {}
            for case in suite.cases:
                group = case.name.split(":", 1)[0]
                groups2.setdefault(group, {OK: 0, FAIL: 0, SKIPPED: 0})[case.status] += 1
            lines += ["| Paquete o archivo | Aprobadas | Fallidas | Omitidas |", "| --- | --- | --- | --- |"]
            lines += [
                f"| `{group}` | {counts[OK]} | {counts[FAIL]} | {counts[SKIPPED]} |"
                for group, counts in sorted(groups2.items())
            ]
        lines.append("")
    return "\n".join(lines).rstrip() + "\n"


def main() -> int:
    parser = argparse.ArgumentParser(description="Genera el reporte de testing de SOLOServis.")
    parser.add_argument("--suites", default=",".join(SUITES),
                        help=f"suites separadas por coma (por defecto: {','.join(SUITES)})")
    parser.add_argument("--smoke", action="store_true", help="suma el pod de test de Kubernetes")
    parser.add_argument("--postgres-docker", action="store_true",
                        help="levanta un PostgreSQL 18 desechable para los tests de integración")
    parser.add_argument("--python", default=sys.executable,
                        help="intérprete con las dependencias del scraper y del crawler")
    parser.add_argument("--out", default="docs/reportes/testing", help="carpeta del reporte")
    args = parser.parse_args()
    for stream in (sys.stdout, sys.stderr):
        if hasattr(stream, "reconfigure"):
            stream.reconfigure(encoding="utf-8", errors="replace")

    chosen = [name.strip() for name in args.suites.split(",") if name.strip()]
    unknown = [name for name in chosen if name not in SUITES]
    if unknown:
        parser.error(f"suite desconocida: {', '.join(unknown)}")

    now = dt.datetime.now().astimezone()
    commit = git("rev-parse", "--short", "HEAD")
    stamp = f"{now:%Y-%m-%d-%H%M}-{commit}"
    raw_dir = ROOT / "tmp" / "reportes-testing" / stamp
    raw_dir.mkdir(parents=True, exist_ok=True)

    database_url = os.environ.get("TEST_DATABASE_URL")
    integration = "con `TEST_DATABASE_URL` del entorno" if database_url else "no (se omiten)"
    started_pg = False
    if "backend" in chosen and args.postgres_docker:
        print("PostgreSQL desechable: iniciando…", flush=True)
        database_url, why = start_postgres(raw_dir)
        started_pg = True
        integration = (f"sí, PostgreSQL desechable (`{PG_IMAGE}`)" if database_url
                       else f"no: {why}")

    suites: list[Suite] = []
    try:
        for name in chosen:
            print(f"Suite {name}: ejecutando…", flush=True)
            if name == "backend":
                suites.append(suite_backend(raw_dir, database_url))
            elif name == "scraper":
                suites.append(suite_pytest("scraper", "Scraper v2 (pytest)", ROOT / "scraper", args.python, raw_dir))
            elif name == "crawler":
                suites.append(suite_pytest("crawler", "Crawler v1 (pytest)", ROOT / "crawler", args.python, raw_dir))
            elif name == "frontend":
                suites.append(suite_frontend(raw_dir))
        if args.smoke:
            print("Suite smoke: leyendo el pod de test…", flush=True)
            suites.append(suite_smoke(raw_dir))
    finally:
        if started_pg:
            stop_postgres()

    dirty = " (con cambios sin commit)" if git("status", "--porcelain") else ""
    meta = {
        "Fecha": f"{now:%Y-%m-%d %H:%M %z}",
        "Rama": f"`{git('rev-parse', '--abbrev-ref', 'HEAD')}`",
        "Commit": f"`{commit}`{dirty}",
        "Suites pedidas": ", ".join(chosen + (["smoke"] if args.smoke else [])),
        "Tests de integración": integration,
        "Go": version(["go", "version"]),
        "Python": version([args.python, "--version"]),
        "Node": version(["node", "--version"]),
        "Generado con": "`python scripts/testing/reporte.py`",
    }
    out_dir = ROOT / args.out
    out_dir.mkdir(parents=True, exist_ok=True)
    report = out_dir / f"{stamp}.md"
    report.write_text(render(suites, meta), encoding="utf-8")

    for suite in suites:
        extra = f" ({suite.reason})" if suite.status == NOT_RUN else (
            f" {suite.count(OK)} ok, {suite.count(FAIL)} fallidas, {suite.count(SKIPPED)} omitidas")
        print(f"  {suite.title}: {suite.status}{extra}")
    print(f"Reporte: {report.relative_to(ROOT)}")
    print(f"Datos crudos: {raw_dir.relative_to(ROOT)}")
    return 1 if any(suite.status == FAIL for suite in suites) else 0


if __name__ == "__main__":
    sys.exit(main())
