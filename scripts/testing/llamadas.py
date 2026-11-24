#!/usr/bin/env python3
"""Corre el visor de llamadas en el clúster y deja la evidencia (TAL-165).

El visor corre en el pod de test (Job soloservis-smoke-test, k8s/tests), junto
al smoke test de la Fase 10: el contenedor `origen` hace las llamadas de prueba
y el sidecar `destino` registra lo que recibe cada servicio. Este script:

  1. recrea el Job del pod de test con los tickets y la pausa pedidos;
  2. con --ventanas abre dos terminales, una con `kubectl logs -f` de cada
     contenedor (en Windows, dos PowerShell; en otro sistema imprime los
     comandos);
  3. espera a que termine y junta los dos registros con lo que anotaron los
     pods reales: el AccessLog del backend (por X-Request-ID) y el log del
     nginx del frontend (por el User-Agent, que lleva el mismo id);
  4. escribe un Markdown con cada llamada lado a lado: lo que envió el origen
     y lo que recibió el destino.

Solo lectura: el Job no escribe en la base y este script solo hace kubectl
apply/delete del Job, get y logs. El reporte cubre el visor; el resultado del
smoke test queda en `kubectl logs job/soloservis-smoke-test -c smoke-test`.
Usa el kubectl y el namespace del contexto actual. Solo la librería estándar de Python 3.9+.

    python scripts/testing/llamadas.py --ventanas
    python scripts/testing/llamadas.py --tickets 166 --pausa 1
    python scripts/testing/llamadas.py --solo-reporte      # reporte del Job que ya corrió

Sale con código 1 si alguna verificación falla y con 2 si no se pudo consultar el clúster.
"""

from __future__ import annotations

import argparse
import datetime as dt
import html
import json
import re
import subprocess
import sys
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
KUSTOMIZE = ROOT / "k8s/tests"
JOB = "soloservis-smoke-test"
API_DEPLOYMENTS = ("comercial", "usuarios", "ingestion", "backend")
TRANSIENT = ("Unable to connect", "TLS handshake timeout", "no such host", "i/o timeout",
             "connection reset", "forcibly closed", "error reading from error stream", "EOF")
ICON = {"OK": "✅", "FALLA": "❌", "AVISO": "⚠️", "OMITIDA": "⏭️"}


class ClusterError(RuntimeError):
    """kubectl falló por algo que no es un corte de red pasajero."""


def kubectl(args: list[str], stdin: str | None = None, timeout: int = 120, check: bool = True) -> str:
    """Ejecuta kubectl y devuelve su stdout.

    Reintenta los cortes de red del clúster. Con check=False devuelve "" en vez
    de lanzar ClusterError cuando kubectl falla."""
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
    if not check:
        return ""
    raise ClusterError(f"kubectl {' '.join(args[:3])}: {last.splitlines()[-1] if last else 'sin salida'}")


def manifiesto(tickets: str | None, pausa: float) -> str:
    """Arma el YAML del Job con kustomize y le pone los tickets y la pausa pedidos.

    Recibe la lista de tickets ("165,166"), o None para dejar los del Job, y la
    pausa en segundos; devuelve el YAML."""
    yaml = kubectl(["kustomize", str(KUSTOMIZE)])
    for nombre, valor in (("TICKETS", tickets), ("PAUSA", str(pausa))):
        if valor is None:
            continue
        # kustomize deja el valor con o sin comillas: se reemplaza la línea entera.
        yaml, n = re.subn(rf"(- name: {nombre}\r?\n\s+value: ).*", lambda m, v=valor: f'{m.group(1)}"{v}"', yaml)
        if n != 1:
            raise ClusterError(f"no encontré la variable {nombre} en el Job de {KUSTOMIZE}")
    return yaml


def lanzar(tickets: str | None, pausa: float) -> None:
    """Borra el Job del pod de test (los Jobs son inmutables) y aplica el nuevo.

    Recibe los tickets ("165,166", o None para los del Job) y la pausa en
    segundos. No devuelve nada; lanza ClusterError si kubectl falla."""
    kubectl(["delete", "job", JOB, "--ignore-not-found", "--wait=true", "--timeout=120s"])
    print(kubectl(["apply", "-f", "-"], stdin=manifiesto(tickets, pausa)).strip())


def pod_del_job() -> dict | None:
    """Devuelve el pod del Job (como dict de la API) o None si todavía no existe."""
    pods = json.loads(kubectl(["get", "pods", "-l", f"job-name={JOB}", "-o", "json"]))["items"]
    return max(pods, key=lambda p: p["metadata"]["creationTimestamp"]) if pods else None


def esperar_arranque(limite: int) -> str:
    """Espera a que el contenedor `origen` arranque y devuelve el nombre del pod.

    Lanza ClusterError si no arranca en `limite` segundos o si el pod queda en
    un estado de espera con error (imagen que no baja, configuración)."""
    fin = time.monotonic() + limite
    while time.monotonic() < fin:
        pod = pod_del_job()
        if pod:
            estados = pod["status"].get("containerStatuses", []) + pod["status"].get("initContainerStatuses", [])
            for estado in estados:
                razon = (estado.get("state", {}).get("waiting") or {}).get("reason", "")
                if razon in ("ErrImagePull", "ImagePullBackOff", "CreateContainerConfigError", "InvalidImageName"):
                    raise ClusterError(f"el contenedor {estado['name']} no arranca: {razon}")
            origen = next((e for e in estados if e["name"] == "origen"), None)
            if origen and ("running" in origen.get("state", {}) or "terminated" in origen.get("state", {})):
                return pod["metadata"]["name"]
        time.sleep(2)
    raise ClusterError(f"el contenedor origen no arrancó en {limite} s")


def abrir_ventanas() -> None:
    """Abre una terminal por contenedor con `kubectl logs -f` (Windows) o imprime los comandos."""
    comandos = {c: f"kubectl logs -f job/{JOB} -c {c}" for c in ("origen", "destino")}
    if sys.platform == "win32":
        for contenedor, comando in comandos.items():
            titulo = f"Visor de llamadas - {contenedor.upper()}"
            script = f"$Host.UI.RawUI.WindowTitle = '{titulo}'; [Console]::OutputEncoding = " \
                     f"[Text.Encoding]::UTF8; {comando}"
            subprocess.Popen(["powershell", "-NoExit", "-Command", script],
                             creationflags=subprocess.CREATE_NEW_CONSOLE)
        print("Abrí dos ventanas de PowerShell: ORIGEN y DESTINO.")
    else:
        print("Abre dos terminales y corre en cada una:")
        for comando in comandos.values():
            print(f"  {comando}")


def esperar_fin(limite: int) -> bool:
    """Espera a que el Job termine. Devuelve True si terminó bien (Complete), False si falló."""
    fin = time.monotonic() + limite
    while time.monotonic() < fin:
        job = json.loads(kubectl(["get", "job", JOB, "-o", "json"]))
        for cond in job["status"].get("conditions", []):
            if cond["status"] == "True" and cond["type"] in ("Complete", "Failed"):
                return cond["type"] == "Complete"
        time.sleep(5)
    raise ClusterError(f"el Job no terminó en {limite} s")


def leer_registro(log: str) -> dict:
    """Junta las líneas @@@REGISTRO n/N del log de origen y devuelve el JSON que forman."""
    trozos: dict[int, str] = {}
    total = 0
    for linea in log.splitlines():
        m = re.match(r"@@@REGISTRO (\d+)/(\d+) (.*)$", linea)
        if m:
            trozos[int(m.group(1))] = m.group(3)
            total = int(m.group(2))
    if not total or len(trozos) != total:
        raise ClusterError(f"el log de origen no trae el registro completo ({len(trozos)} de {total} partes); "
                           "¿el Job terminó antes de tiempo?")
    return json.loads("".join(trozos[n] for n in range(1, total + 1)))


def registros_reales(ids: set[str], desde_s: int) -> dict[str, list[str]]:
    """Busca cada X-Request-ID en los logs de los pods reales.

    Lee el AccessLog JSON del backend y el log del nginx del frontend de los
    últimos `desde_s` segundos. Devuelve, por id, las líneas encontradas ya
    resumidas ("comercial: GET /products 400 en 1.2 ms")."""
    encontrados: dict[str, list[str]] = {}
    desplegados = kubectl(["get", "deployments", "-o", "jsonpath={.items[*].metadata.name}"]).split()
    for nombre in [d for d in API_DEPLOYMENTS if d in desplegados]:
        for linea in kubectl(["logs", f"deploy/{nombre}", f"--since={desde_s}s"], check=False).splitlines():
            try:
                fila = json.loads(linea)
            except ValueError:
                continue
            if fila.get("request_id") in ids:
                encontrados.setdefault(fila["request_id"], []).append(
                    f"{nombre}: {fila.get('method')} {fila.get('path')} {fila.get('status')} "
                    f"en {fila.get('duration_ms')} ms")
    if "frontend" in desplegados:
        for linea in kubectl(["logs", "deploy/frontend", f"--since={desde_s}s"], check=False).splitlines():
            m = re.search(r'"([A-Z]+) (\S+) HTTP/[\d.]+" (\d{3}) .*"visor-llamadas ([A-Za-z0-9-]+)"', linea)
            if m and m.group(4) in ids:
                encontrados.setdefault(m.group(4), []).append(
                    f"nginx (gateway): {m.group(1)} {m.group(2)} {m.group(3)}")
    return encontrados


def celda(texto: str) -> str:
    """Prepara un texto para una celda de tabla Markdown: escapa HTML, | y saltos de línea."""
    return html.escape(str(texto), quote=False).replace("|", "&#124;").replace("\n", "<br>")


def escribir_markdown(ruta: Path, datos: dict, reales: dict[str, list[str]], cabecera: dict[str, str]) -> None:
    """Escribe el reporte: resumen, tabla de llamadas y el detalle lado a lado de cada una."""
    origen, destino = datos["origen"], datos["destino"]
    por_id = {}
    for fila in destino:
        por_id.setdefault(fila.get("id"), []).append(fila)
    lineas = ["# Visor de llamadas", "", "| Campo | Valor |", "| --- | --- |"]
    lineas += [f"| {k} | {v} |" for k, v in cabecera.items()]
    lineas += ["", "## Resultado por ticket", "", "| Ticket | OK | Fallas | Avisos | Omitidas |",
               "| --- | --- | --- | --- | --- |"]
    for ticket in sorted({r["ticket"] for r in origen}):
        filas = [r for r in origen if r["ticket"] == ticket]
        cuenta = [sum(r["estado"] == e for r in filas) for e in ("OK", "FALLA", "AVISO", "OMITIDA")]
        lineas.append(f"| TAL-{ticket} | " + " | ".join(map(str, cuenta)) + " |")
    problemas = [r for r in origen if r["estado"] in ("FALLA", "AVISO")]
    if problemas:
        lineas += ["", "## Fallas y avisos", ""]
        lineas += [f"- {ICON[r['estado']]} `{r['id']}` {r['nombre']}: {'; '.join(r['motivos'])}" for r in problemas]
    lineas += ["", "## Llamadas", "",
               "Origen: lo que envió y recibió el pod de pruebas. Destino: lo que registró el sidecar al recibir "
               "el pedido y al reenviarlo. Pod real: la línea del AccessLog del backend o del nginx con el mismo id.",
               "", "| Estado | ID | Caso | Esperado | Origen recibió | Destino registró | Pod real |",
               "| --- | --- | --- | --- | --- | --- | --- |"]
    for r in origen:
        recibido = por_id.get(r["id"], [])
        dest = "<br>".join(f"{f['metodo']} {f['ruta']} → {f.get('estado', '?')}" for f in recibido)
        lineas.append(f"| {ICON.get(r['estado'], '')} | `{r['id']}` | {celda(r['nombre'])} | {celda(r['espera'])} | "
                      f"{celda(r['obtenido'])} | {celda(dest)} | {celda('<br>'.join(reales.get(r['id'], [])))} |"
                      .replace("&lt;br&gt;", "<br>"))
    lineas += ["", "## Detalle lado a lado", ""]
    for r in origen:
        if r["url"] == "(verificación cruzada)" or r["estado"] == "OMITIDA":
            continue
        lineas += [f"<details><summary>{ICON.get(r['estado'], '')} <code>{html.escape(r['id'])}</code> "
                   f"{html.escape(r['nombre'])}</summary>", "",
                   "| | Origen (envía) | Destino (recibe) |", "| --- | --- | --- |"]
        recibido = por_id.get(r["id"], [{}])
        for f in recibido:
            headers_o = "<br>".join(f"{celda(k)}: {celda(v)}" for k, v in r["headers"].items())
            headers_d = "<br>".join(f"{celda(k)}: {celda(v)}" for k, v in f.get("headers", {}).items()
                                    if k.lower() not in ("host", "accept-encoding", "connection"))
            lineas += [f"| Pedido | <code>{celda(r['metodo'])} {celda(r['url'])}</code> | "
                       f"<code>{celda(f.get('metodo', ''))} {celda(f.get('ruta', ''))}</code> → "
                       f"{celda(f.get('servicio', ''))} |",
                       f"| Headers | {headers_o} | {headers_d} |",
                       f"| Body | <code>{celda(r['body'] or '(vacío)')}</code> | "
                       f"<code>{celda(f.get('body', ''))}</code> ({f.get('bytes', 0)} bytes) |",
                       f"| Respuesta | {celda(r['obtenido'])} en {r['ms']} ms | "
                       f"{celda(f.get('estado', ''))} {celda(f.get('tipo', ''))} en {f.get('ms', '?')} ms |"]
        lineas += [f"| Extracto | <code>{celda(r['extracto'][:600])}</code> | |",
                   f"| Esperado | {celda(r['espera'])} | "
                   f"{ICON.get(r['estado'], '')} {celda('; '.join(r['motivos']))} |",
                   f"| Pod real | | {celda('; '.join(reales.get(r['id'], [])) or '(sin línea con este id)')} |",
                   "", "</details>", ""]
    ruta.parent.mkdir(parents=True, exist_ok=True)
    ruta.write_text("\n".join(lineas), encoding="utf-8")


def main() -> int:
    """Lanza el Job (salvo --solo-reporte), espera, junta los registros y escribe el reporte."""
    parser = argparse.ArgumentParser(description="Visor de llamadas de SOLOServis en el clúster.")
    parser.add_argument("--tickets", help="tickets a correr, separados por coma (por defecto, todos los del Job)")
    parser.add_argument("--pausa", type=float, default=0.4, help="segundos entre llamadas (para leer las ventanas)")
    parser.add_argument("--ventanas", action="store_true", help="abre una terminal por contenedor con kubectl logs -f")
    parser.add_argument("--solo-reporte", action="store_true", help="no relanza el Job: arma el reporte del último")
    parser.add_argument("--md", type=Path, help="ruta del reporte (por defecto docs/reportes/testing/llamadas-…md)")
    parser.add_argument("--timeout", type=int, default=600, help="segundos máximos de espera del Job")
    args = parser.parse_args()
    for stream in (sys.stdout, sys.stderr):
        if hasattr(stream, "reconfigure"):
            stream.reconfigure(encoding="utf-8")

    inicio = dt.datetime.now().astimezone()
    try:
        if not args.solo_reporte:
            lanzar(args.tickets, args.pausa)
            pod = esperar_arranque(180)
            print(f"Pod del visor: {pod}")
            if args.ventanas:
                abrir_ventanas()
            else:
                print(f"Para mirarlo en vivo: kubectl logs -f job/{JOB} -c origen   (y -c destino)")
            print("Esperando a que termine…")
            completo = esperar_fin(args.timeout)
        else:
            completo = True
        log = kubectl(["logs", f"job/{JOB}", "-c", "origen"])
        datos = leer_registro(log)
        pod = pod_del_job()
        creado = dt.datetime.fromisoformat(pod["metadata"]["creationTimestamp"].replace("Z", "+00:00")) if pod \
            else inicio
        desde = int((dt.datetime.now(dt.timezone.utc) - creado).total_seconds()) + 15
        ids = {r["id"] for r in datos["origen"]}
        reales = registros_reales(ids, desde)
        namespace = kubectl(["config", "view", "--minify", "-o", "jsonpath={..namespace}"], check=False).strip()
    except ClusterError as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 2

    resumen = next((linea for linea in log.splitlines() if linea.startswith("Resultado: ")), "Resultado: ?")
    print("\n" + "\n".join(linea for linea in log.splitlines()
                           if re.match(r"(TAL-\d+: |  \[(FALLA|AVISO)\] )", linea)))
    print(resumen)
    con_api = [r for r in datos["origen"] if r["obtenido"] and r["id"] in reales]
    print(f"Llamadas encontradas en los logs de los pods reales: {len(con_api)} de "
          f"{sum(1 for r in datos['origen'] if r['obtenido'])}")
    cabecera = {
        "Fecha": creado.astimezone().strftime("%Y-%m-%d %H:%M %z"),
        "Namespace": f"`{namespace or '?'}`",
        "Topología": datos.get("topologia", "?"),
        "Tickets": ", ".join(f"TAL-{t}" for t in sorted({r["ticket"] for r in datos["origen"]})),
        "Resultado": resumen.removeprefix("Resultado: "),
        "Job": f"`{JOB}` ({'terminó bien' if completo else 'terminó con fallas'})",
        "Generado con": "`python scripts/testing/llamadas.py`",
    }
    ruta = args.md or ROOT / "docs/reportes/testing" / f"llamadas-{creado.astimezone():%Y-%m-%d-%H%M}.md"
    escribir_markdown(ruta, datos, reales, cabecera)
    print(f"Reporte: {ruta}")
    return 1 if any(r["estado"] == "FALLA" for r in datos["origen"]) else 0


if __name__ == "__main__":
    sys.exit(main())
