#!/usr/bin/env python3
"""Consumo de CPU y memoria de cada pod contra sus requests, limits y la cuota (TAL-207).

En el clúster UCT `kubectl top` no funciona (el rol no puede leer
metrics.k8s.io), así que el consumo se lee del cgroup v2 de cada contenedor:

  - CPU: `cpu.stat` (usage_usec) entre dos muestras, en milicores, y el
    porcentaje de periodos en que el kernel frenó el contenedor por pasar su
    limit (throttling).
  - Memoria: working set (`memory.current` menos `inactive_file`, lo mismo que
    mira Kubernetes para un OOMKill) y el pico desde que arrancó el contenedor
    (`memory.peak`).

Se abre un solo `kubectl exec` por pod para toda la medición: un bucle acotado
que imprime una muestra cada `--intervalo` segundos. Además suma la cuota del
namespace y los reinicios u OOMKilled que ocurran mientras mide.

Es de solo lectura: kubectl get y exec con `cat` y `grep` dentro de cada pod.
Usa el kubectl y el namespace del contexto actual. Con `--docker PREFIJO` mide
contenedores locales de Docker en vez del clúster (lo usa `carga.py --local`).
Solo la librería estándar de Python 3.9+.

    python scripts/testing/recursos.py                 # 30 s, una muestra cada 5 s
    python scripts/testing/recursos.py --duracion 120
    python scripts/testing/recursos.py --md recursos.md
    python scripts/testing/reporte.py --recursos       # dentro del reporte de testing

Sale con código 1 si un contenedor pasa el 90 % de su limit de memoria o se
reinicia mientras mide, y con 2 si no se pudo consultar el clúster.
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import math
import re
import subprocess
import sys
import threading
import time
from dataclasses import dataclass, field
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from salud import ClusterError, kget, quantity  # noqa: E402

MEM_FALLA = 0.9  # working set sobre el limit que ya es una falla (cerca del OOMKill)
CPU_AVISO = 0.9  # CPU media sobre el limit: el contenedor está saturado
THROTTLE_AVISO = 0.25  # periodos frenados por el limit de CPU
# Con pocos periodos (contenedor casi ocioso) el porcentaje es ruido: un solo
# pico de un GC ya es el 50 %. Solo se avisa desde este número de periodos.
THROTTLE_MIN_PERIODOS = 100
CUOTA_AVISO = 0.8

# Corre dentro del contenedor con `sh -s`. Lee los archivos con `read`, que es
# interno del shell: la única llamada a otro programa es `sleep`, así la
# medición casi no suma CPU al contenedor (está en el mismo cgroup). Funciona
# en todas las imágenes (busybox, Debian y python-slim).
SAMPLER = r"""
G=/sys/fs/cgroup; n=__N__; i=0
while [ $i -lt $n ]; do
  read up _ < /proc/uptime
  u=0; p=0; t=0; f=0; k=0
  while read key val; do
    case $key in usage_usec) u=$val;; nr_periods) p=$val;; nr_throttled) t=$val;; esac
  done < $G/cpu.stat
  while read key val; do
    case $key in inactive_file) f=$val;; esac
  done < $G/memory.stat
  read c < $G/memory.current
  [ -r $G/memory.peak ] && read k < $G/memory.peak
  echo "@@@S $up $u $p $t $c $k $f"
  i=$((i+1)); sleep __INT__
done
echo "@@@FIN"
"""


@dataclass
class Muestra:
    """Una lectura del cgroup. `t` es la hora de este equipo al recibirla."""
    t: float
    uptime: float
    cpu_usec: int
    periodos: int
    frenados: int
    mem: int
    pico: int
    inactiva: int


@dataclass
class Contenedor:
    """Un pod (o contenedor de Docker) medido, con sus requests y limits."""
    nombre: str
    servicio: str
    req_cpu: float | None = None  # en núcleos
    lim_cpu: float | None = None
    req_mem: float | None = None  # en bytes
    lim_mem: float | None = None
    reinicios: int = 0
    muestras: list[Muestra] = field(default_factory=list)
    error: str = ""


def fmt_cpu(cores: float | None) -> str:
    """Núcleos a texto en milicores ("123m"); «—» si no hay valor."""
    return "—" if cores is None else f"{cores * 1000:.0f}m"


def fmt_mem(value: float | None) -> str:
    """Bytes a texto en MiB ("45Mi"); «—» si no hay valor."""
    return "—" if value is None else f"{value / 2**20:.0f}Mi"


def fmt_pct(part: float | None, total: float | None) -> str:
    """Porcentaje de `part` sobre `total`; vacío si falta alguno de los dos."""
    if part is None or not total:
        return ""
    return f" ({part / total:.0%})"


def servicio_de(pod: dict) -> str:
    """Nombre corto del servicio de un pod: la etiqueta `app` o el nombre sin el sufijo aleatorio."""
    labels = pod["metadata"].get("labels", {})
    for key in ("app.kubernetes.io/name", "app", "app.kubernetes.io/component"):
        if labels.get(key):
            return labels[key]
    return re.sub(r"(-[a-z0-9]{6,10})?-[a-z0-9]{5}$", "", pod["metadata"]["name"])


def _docker(args: list[str], timeout: int = 60) -> str:
    """Ejecuta docker y devuelve stdout; lanza ClusterError si falla."""
    try:
        proc = subprocess.run(["docker", *args], check=False, capture_output=True, timeout=timeout)
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise ClusterError(f"docker {args[0]}: {exc}") from exc
    if proc.returncode != 0:
        raise ClusterError(f"docker {args[0]}: {proc.stderr.decode('utf-8', 'replace').strip()}")
    return proc.stdout.decode("utf-8", "replace")


class Monitor:
    """Mide en paralelo todos los contenedores del objetivo mientras corre otra cosa.

    `iniciar()` descubre los contenedores y abre un exec por cada uno;
    `detener()` los cierra. Las muestras quedan en `contenedores`."""

    def __init__(self, intervalo: float = 5, duracion_max: float = 600, docker: str | None = None):
        """`docker` es el prefijo de nombre de los contenedores locales; None mide el clúster."""
        self.intervalo = intervalo
        self.duracion_max = duracion_max
        self.docker = docker
        self.contenedores: list[Contenedor] = []
        self.cuota: list[dict] = []
        self.inicio = 0.0
        self.fin = 0.0
        self._procs: list[subprocess.Popen] = []
        self._hilos: list[threading.Thread] = []

    # ------------------------------------------------------------ descubrir

    def _descubrir_k8s(self) -> None:
        """Pods en Running del namespace (sin los de Jobs), con sus requests, limits y la cuota."""
        for pod in kget("pods"):
            owners = [o.get("kind") for o in pod["metadata"].get("ownerReferences", [])]
            if pod["status"].get("phase") != "Running" or "Job" in owners:
                continue
            spec = pod["spec"]["containers"][0]
            res = spec.get("resources", {})
            req, lim = res.get("requests", {}), res.get("limits", {})
            status = next((s for s in pod["status"].get("containerStatuses", []) if s["name"] == spec["name"]), {})
            self.contenedores.append(Contenedor(
                nombre=pod["metadata"]["name"], servicio=servicio_de(pod),
                req_cpu=quantity(req["cpu"]) if "cpu" in req else None,
                lim_cpu=quantity(lim["cpu"]) if "cpu" in lim else None,
                req_mem=quantity(req["memory"]) if "memory" in req else None,
                lim_mem=quantity(lim["memory"]) if "memory" in lim else None,
                reinicios=status.get("restartCount", 0)))
        for quota in kget("resourcequota"):
            self.cuota.append({"nombre": quota["metadata"]["name"], "hard": quota["status"].get("hard", {}),
                               "used": quota["status"].get("used", {})})

    def _descubrir_docker(self) -> None:
        """Contenedores locales cuyo nombre empieza con el prefijo; los limits salen de docker inspect."""
        names = _docker(["ps", "--filter", f"name=^{self.docker}", "--format", "{{.Names}}"]).split()
        if not names:
            return
        for info in json.loads(_docker(["inspect", *names])):
            host = info["HostConfig"]
            self.contenedores.append(Contenedor(
                nombre=info["Name"].lstrip("/"), servicio=info["Name"].lstrip("/").removeprefix(self.docker),
                lim_cpu=host["NanoCpus"] / 1e9 if host.get("NanoCpus") else None,
                lim_mem=float(host["Memory"]) if host.get("Memory") else None,
                reinicios=info.get("RestartCount", 0)))

    # ------------------------------------------------------------ medir

    def iniciar(self) -> None:
        """Descubre los contenedores y arranca un exec de muestreo por cada uno."""
        if self.docker:
            self._descubrir_docker()
        else:
            self._descubrir_k8s()
        if not self.contenedores:
            raise ClusterError("no hay contenedores para medir")
        self.contenedores.sort(key=lambda c: c.servicio)
        n = math.ceil(self.duracion_max / self.intervalo) + 2
        script = SAMPLER.replace("__N__", str(n)).replace("__INT__", f"{self.intervalo:g}")
        self.inicio = time.time()
        for cont in self.contenedores:
            if self.docker:
                cmd = ["docker", "exec", "-i", cont.nombre, "sh", "-s"]
            else:
                cmd = ["kubectl", "exec", "-i", cont.nombre, "--", "sh", "-s"]
            proc = subprocess.Popen(cmd, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            # En bytes: en modo texto Windows convertiría los \n en \r\n.
            proc.stdin.write(script.encode("utf-8"))
            proc.stdin.close()
            hilo = threading.Thread(target=self._leer, args=(cont, proc), daemon=True)
            hilo.start()
            self._procs.append(proc)
            self._hilos.append(hilo)

    def _leer(self, cont: Contenedor, proc: subprocess.Popen) -> None:
        """Lee las muestras que imprime el bucle de un contenedor hasta que termina."""
        for raw in proc.stdout:
            line = raw.decode("utf-8", "replace").strip()
            if line.startswith("@@@S "):
                parts = line.split()[1:]
                try:
                    cont.muestras.append(Muestra(time.time(), float(parts[0]), *(int(x) for x in parts[1:])))
                except (ValueError, TypeError):
                    continue
        proc.wait()
        err = proc.stderr.read().decode("utf-8", "replace").strip()
        lines = [x for x in err.splitlines() if not x.startswith("Defaulted container")]
        if lines and not cont.muestras:
            cont.error = lines[-1]

    def detener(self) -> None:
        """Cierra los exec de muestreo (el bucle remoto termina solo al agotar sus vueltas)."""
        self.fin = time.time()
        for proc in self._procs:
            if proc.poll() is None:
                proc.terminate()
        for hilo in self._hilos:
            hilo.join(timeout=10)

    def esperar(self, segundos: float) -> None:
        """Mide durante `segundos` sin hacer nada más (uso independiente del script)."""
        time.sleep(segundos + 0.5)

    # ------------------------------------------------------------ reinicios

    def reinicios(self) -> list[str]:
        """Contenedores que se reiniciaron (o murieron por OOM) desde que empezó la medición."""
        nuevos = []
        if self.docker:
            names = [c.nombre for c in self.contenedores]
            try:
                infos = {i["Name"].lstrip("/"): i for i in json.loads(_docker(["inspect", *names]))}
            except ClusterError as exc:
                return [f"no se pudo revisar: {exc}"]
            for cont in self.contenedores:
                info = infos.get(cont.nombre)
                if info is None or not info["State"].get("Running"):
                    nuevos.append(f"{cont.servicio}: el contenedor no está corriendo")
                elif info["State"].get("OOMKilled") or info.get("RestartCount", 0) > cont.reinicios:
                    nuevos.append(f"{cont.servicio}: reinicio u OOMKilled")
            return nuevos
        try:
            pods = {p["metadata"]["name"]: p for p in kget("pods")}
        except ClusterError as exc:
            return [f"no se pudo revisar: {exc}"]
        for cont in self.contenedores:
            pod = pods.get(cont.nombre)
            if pod is None:
                nuevos.append(f"{cont.servicio}: el pod {cont.nombre} ya no existe")
                continue
            for status in pod["status"].get("containerStatuses", []):
                if status.get("restartCount", 0) > cont.reinicios:
                    reason = status.get("lastState", {}).get("terminated", {}).get("reason", "?")
                    nuevos.append(f"{cont.servicio}: {status['restartCount'] - cont.reinicios} reinicio(s), "
                                  f"último por {reason}")
        return nuevos

    # ------------------------------------------------------------ resumir

    def resumen(self, desde: float | None = None, hasta: float | None = None) -> dict[str, dict]:
        """Consumo de cada contenedor entre dos horas (por defecto, toda la medición).

        Devuelve por servicio: CPU media y máxima en núcleos, porcentaje de
        throttling, working set máximo y pico de memoria, y cuántas muestras usó."""
        desde = self.inicio if desde is None else desde
        hasta = (self.fin or time.time()) if hasta is None else hasta
        out = {}
        for cont in self.contenedores:
            cpus, ws = [], []
            usec = seg = periodos = frenados = 0
            for prev, cur in zip(cont.muestras, cont.muestras[1:]):
                if not desde <= cur.t <= hasta + 0.5:
                    continue
                dt_s = cur.uptime - prev.uptime
                d_usec = cur.cpu_usec - prev.cpu_usec
                if dt_s <= 0 or d_usec < 0:  # el contenedor se reinició entre las dos
                    continue
                cpus.append(d_usec / 1e6 / dt_s)
                usec, seg = usec + d_usec, seg + dt_s
                periodos += max(cur.periodos - prev.periodos, 0)
                frenados += max(cur.frenados - prev.frenados, 0)
                ws.append(cur.mem - cur.inactiva)
            ultima = cont.muestras[-1] if cont.muestras else None
            out[cont.servicio] = {
                "cpu_media": usec / 1e6 / seg if seg else None,
                "cpu_max": max(cpus) if cpus else None,
                "throttling": frenados / periodos if periodos else (0.0 if cpus else None),
                "mem_max": max(ws) if ws else None,
                "mem_pico": ultima.pico if ultima and ultima.pico else None,
                "periodos": periodos,
                "muestras": len(cpus),
            }
        return out

    def a_dict(self) -> dict:
        """Todo lo medido, para guardarlo en JSON."""
        return {
            "fuente": f"docker ({self.docker}*)" if self.docker else "kubernetes",
            "intervalo": self.intervalo, "inicio": self.inicio, "fin": self.fin, "cuota": self.cuota,
            "contenedores": [{**{k: v for k, v in c.__dict__.items() if k != "muestras"},
                              "muestras": [m.__dict__ for m in c.muestras]} for c in self.contenedores],
        }


# ---------------------------------------------------------------- evaluación

def evaluar(monitor: Monitor, resumen: dict[str, dict], reinicios: list[str]) -> list[tuple[str, str, str]]:
    """Juzga el consumo: devuelve (estado, servicio, detalle) con OK, AVISO o FALLA por contenedor."""
    out = []
    for cont in monitor.contenedores:
        r = resumen[cont.servicio]
        if r["muestras"] == 0:
            out.append(("AVISO", cont.servicio, f"sin muestras: {cont.error or 'el exec no devolvió datos'}"))
            continue
        problemas, avisos = [], []
        if cont.lim_mem and r["mem_max"] and r["mem_max"] >= MEM_FALLA * cont.lim_mem:
            problemas.append(f"memoria {fmt_mem(r['mem_max'])} de {fmt_mem(cont.lim_mem)} de limit")
        if cont.lim_cpu and r["cpu_media"] and r["cpu_media"] >= CPU_AVISO * cont.lim_cpu:
            avisos.append(f"CPU media {fmt_cpu(r['cpu_media'])} de {fmt_cpu(cont.lim_cpu)}: saturado")
        if r["throttling"] and r["throttling"] >= THROTTLE_AVISO and r["periodos"] >= THROTTLE_MIN_PERIODOS:
            avisos.append(f"frenado por el limit de CPU en el {r['throttling']:.0%} de los periodos")
        problemas += [x.split(": ", 1)[1] for x in reinicios if x.startswith(cont.servicio + ":")]
        detalle = (f"CPU {fmt_cpu(r['cpu_media'])} media, {fmt_cpu(r['cpu_max'])} máx "
                   f"(req {fmt_cpu(cont.req_cpu)}, lim {fmt_cpu(cont.lim_cpu)}); "
                   f"memoria {fmt_mem(r['mem_max'])} (req {fmt_mem(cont.req_mem)}, lim {fmt_mem(cont.lim_mem)})")
        if problemas:
            out.append(("FALLA", cont.servicio, detalle + " — " + "; ".join(problemas + avisos)))
        elif avisos:
            out.append(("AVISO", cont.servicio, detalle + " — " + "; ".join(avisos)))
        else:
            out.append(("OK", cont.servicio, detalle))
    return out


def tabla_contenedores(monitor: Monitor, resumen: dict[str, dict]) -> list[str]:
    """Tabla Markdown con el consumo de cada contenedor contra sus requests y limits."""
    lines = ["| Servicio | CPU media | CPU máx | Throttling | Req / lim CPU | Memoria máx | Pico | Req / lim memoria |",
             "| --- | ---: | ---: | ---: | --- | ---: | ---: | --- |"]
    for cont in monitor.contenedores:
        r = resumen[cont.servicio]
        thr = "—" if r["throttling"] is None else f"{r['throttling']:.0%} de {r['periodos']}"
        lines.append(
            f"| {cont.servicio} | {fmt_cpu(r['cpu_media'])}{fmt_pct(r['cpu_media'], cont.lim_cpu)} | "
            f"{fmt_cpu(r['cpu_max'])} | {thr} | {fmt_cpu(cont.req_cpu)} / {fmt_cpu(cont.lim_cpu)} | "
            f"{fmt_mem(r['mem_max'])}{fmt_pct(r['mem_max'], cont.lim_mem)} | {fmt_mem(r['mem_pico'])} | "
            f"{fmt_mem(cont.req_mem)} / {fmt_mem(cont.lim_mem)} |")
    lines.append("")
    lines.append("CPU media y memoria máx van con su porcentaje del limit. Throttling: periodos de 100 ms "
                 "en que el limit de CPU frenó al contenedor, sobre los periodos en que tuvo trabajo. "
                 "Memoria es el working set; el pico es desde que arrancó el contenedor.")
    return lines


def tabla_cuota(monitor: Monitor, resumen: dict[str, dict]) -> list[str]:
    """Cuota del namespace (lo reservado) junto al consumo real sumado de todos los pods."""
    if not monitor.cuota:
        return []
    cpu_real = sum(r["cpu_media"] or 0 for r in resumen.values())
    mem_real = sum(r["mem_max"] or 0 for r in resumen.values())
    lines = ["| Cuota | Usado | Límite | % |", "| --- | --- | --- | ---: |"]
    for quota in monitor.cuota:
        for key in ("requests.cpu", "limits.cpu", "requests.memory", "limits.memory", "pods"):
            if key in quota["hard"]:
                used, hard = quota["used"].get(key, "0"), quota["hard"][key]
                pct = quantity(used) / quantity(hard) if quantity(hard) else 0
                mark = " ⚠️" if pct >= CUOTA_AVISO else ""
                lines.append(f"| `{key}` | {used} | {hard} | {pct:.0%}{mark} |")
    hard = monitor.cuota[0]["hard"]
    lines.append("")
    lines.append(f"Consumo real sumado de los pods: CPU {fmt_cpu(cpu_real)}"
                 f"{fmt_pct(cpu_real, quantity(hard['requests.cpu']) if 'requests.cpu' in hard else None)} "
                 f"de los requests de la cuota, memoria {fmt_mem(mem_real)}"
                 f"{fmt_pct(mem_real, quantity(hard['requests.memory']) if 'requests.memory' in hard else None)}. "
                 "La cuota limita lo reservado (requests y limits), no lo consumido.")
    return lines


# ---------------------------------------------------------------- main

def main() -> int:
    """Mide durante --duracion segundos, imprime el consumo y escribe --md/--json.

    Devuelve 0 si todo está bien, 1 si hay una falla y 2 si no se pudo medir."""
    parser = argparse.ArgumentParser(description="Consumo de CPU y memoria de los pods de SOLOServis.")
    parser.add_argument("--duracion", type=float, default=30, help="segundos de medición (30)")
    parser.add_argument("--intervalo", type=float, default=5, help="segundos entre muestras (5)")
    parser.add_argument("--docker", metavar="PREFIJO", help="mide contenedores locales con ese prefijo")
    parser.add_argument("--md", type=Path, help="guarda el resultado en Markdown")
    parser.add_argument("--json", type=Path, help="guarda el resultado en JSON (lo usa reporte.py --recursos)")
    args = parser.parse_args()
    for stream in (sys.stdout, sys.stderr):
        if hasattr(stream, "reconfigure"):
            stream.reconfigure(encoding="utf-8")

    monitor = Monitor(args.intervalo, args.duracion + args.intervalo * 2, args.docker)
    started = dt.datetime.now().astimezone()
    try:
        monitor.iniciar()
        print(f"Midiendo {len(monitor.contenedores)} contenedores durante {args.duracion:g} s…", flush=True)
        monitor.esperar(args.duracion)
        reinicios = monitor.reinicios()
    except ClusterError as exc:
        print(f"ERROR: no se pudo medir: {exc}", file=sys.stderr)
        return 2
    finally:
        monitor.detener()

    resumen = monitor.resumen()
    juicio = evaluar(monitor, resumen, reinicios)
    header = {"Fecha": started.strftime("%Y-%m-%d %H:%M %z"),
              "Fuente": monitor.a_dict()["fuente"] + " (cgroup v2)",
              "Duración": f"{args.duracion:g} s, una muestra cada {args.intervalo:g} s",
              "Generado con": "`python scripts/testing/recursos.py`"}
    lines = ["# Consumo de recursos", ""] + [f"- **{k}:** {v}" for k, v in header.items()] + [""]
    lines += tabla_contenedores(monitor, resumen) + [""]
    cuota = tabla_cuota(monitor, resumen)
    if cuota:
        lines += ["## Cuota del namespace", ""] + cuota + [""]
    icons = {"OK": "✅", "AVISO": "⚠️", "FALLA": "❌"}
    lines += ["## Evaluación", ""] + [f"- {icons[e]} **{s}:** {d}" for e, s, d in juicio]
    if reinicios:
        lines += ["", "Reinicios durante la medición:", ""] + [f"- {x}" for x in reinicios]
    text = "\n".join(lines) + "\n"
    print(text)
    if args.md:
        args.md.write_text(text, encoding="utf-8")
        print(f"Markdown: {args.md}")
    if args.json:
        args.json.write_text(json.dumps({"header": header, "resumen": resumen, "evaluacion": juicio,
                                         "reinicios": reinicios, "medicion": monitor.a_dict()},
                                        ensure_ascii=False, indent=2), encoding="utf-8")
    return 1 if any(e == "FALLA" for e, _, _ in juicio) else 0


if __name__ == "__main__":
    sys.exit(main())
