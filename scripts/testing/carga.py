#!/usr/bin/env python3
"""Prueba de carga con usuarios simultáneos (TAL-206) y consumo de recursos (TAL-207).

Simula usuarios que navegan el sitio como lo hace el frontend: lista de
productos (a veces filtrada por categoría), categorías, búsqueda, detalle de un
producto y tiendas, con una espera al azar entre páginas. La carga sube por
escalones (por defecto 5, 10 y 20 usuarios, 2 minutos cada uno) y, mientras
corre, `recursos.py` mide la CPU y la memoria de cada pod.

Por escalón y por ruta reporta latencias p50/p95/p99, errores por código y
throughput, y el consumo de cada pod en cada fase (reposo, escalones, pausas).

Frenos de seguridad (el clúster UCT es compartido):
  - Tope global de pedidos por segundo (`--max-rps`, 15 por defecto).
  - Corta la prueba si en los últimos 15 s los errores pasan del 20 % o el p95
    supera 5 s, o si algún pod se reinicia.

Cada pedido lleva `X-Request-ID: carga-<corrida>-<n>`, así errores.py separa
este tráfico del de usuarios reales.

Objetivos:
  - Por defecto, el host público del Ingress (lo lee de kubectl). En UCT el
    certificado es el autofirmado de la plataforma: hay que pasar
    `--sin-verificar-tls`, igual que se acepta el aviso en el navegador.
  - `--local`: levanta contenedores desechables `soloservis-carga-*`
    (PostgreSQL con las migraciones, comercial, usuarios y el gateway nginx) con
    los mismos limits de CPU y memoria que en el clúster, y los borra al final.
    Ahí se pueden usar `--login`, `--directo` (pega a comercial sin pasar por el
    gateway) y `--sin-frenos`.

Solo la librería estándar de Python 3.9+.

    python scripts/testing/carga.py --sin-verificar-tls                 # UCT: 5, 10 y 20 usuarios
    python scripts/testing/carga.py --local --login                     # mismo recorrido en local
    python scripts/testing/carga.py --local --directo --escalones 20,50,100 --espera 0 --max-rps 0

Deja el reporte en docs/reportes/carga/ y los datos crudos en tmp/reportes-carga/.
Sale con código 1 si la prueba se cortó por un freno o hubo errores 5xx, y con 2
si no se pudo preparar.
"""

from __future__ import annotations

import argparse
import datetime as dt
import http.client
import json
import math
import random
import re
import secrets
import ssl
import subprocess
import sys
import threading
import time
import urllib.parse
from collections import Counter, defaultdict
from dataclasses import dataclass, field
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from recursos import (  # noqa: E402
    ClusterError, Monitor, evaluar, fmt_cpu, fmt_mem, fmt_pct, tabla_contenedores, tabla_cuota,
)
from salud import kget  # noqa: E402

ROOT = Path(__file__).resolve().parents[2]
RUN_ID = secrets.token_hex(6)
REQUEST_ID_PREFIX = f"carga-{RUN_ID}"
TIMEOUT = 15  # segundos por pedido; pasado eso cuenta como error
VENTANA_FRENO = 15  # segundos que miran los frenos
FRENO_ERRORES = 0.20
FRENO_P95_MS = 5000
FRENO_MIN_PEDIDOS = 20

# Entorno local: contenedores desechables con nombre y puertos propios.
LOCAL_PREFIJO = "soloservis-carga-"
LOCAL_RED = "soloservis-carga"
LOCAL_PUERTO_GATEWAY = 18443
LOCAL_PUERTO_COMERCIAL = 18080
# El backend deja 10 sesiones vigentes por usuario (TAL-200): con más usuarios
# virtuales en una sola cuenta, los logins nuevos invalidan los tokens de otros.
USUARIOS_POR_CUENTA = 5
# Los mismos limits que en el clúster (k8s/base y k8s/components/dominios).
LOCAL_LIMITS = {
    "postgres": ("0.25", "512m"),
    "comercial": ("0.3", "256m"),
    "usuarios": ("0.2", "128m"),
    "frontend": ("0.1", "128m"),
}


# ---------------------------------------------------------------- cliente HTTP

@dataclass
class Pedido:
    """Resultado de un pedido: cuándo, en qué escalón, a qué ruta, código y latencia."""
    t: float
    escalon: int
    ruta: str
    codigo: int  # 0 si no hubo respuesta
    ms: float
    error: str = ""
    path: str = ""  # solo en los que fallaron (4xx, 5xx o sin respuesta), para saber cuál fue


class Cliente:
    """Conexión HTTP persistente de un usuario virtual, como la de un navegador (keep-alive)."""

    def __init__(self, base: urllib.parse.SplitResult, tls: ssl.SSLContext | None):
        """`base` es la URL del objetivo; `tls` el contexto para https (None en http)."""
        self.base = base
        self.tls = tls
        self.conn: http.client.HTTPConnection | None = None

    def _conectar(self) -> http.client.HTTPConnection:
        """Abre la conexión si no hay una abierta y la devuelve."""
        if self.conn is None:
            if self.base.scheme == "https":
                self.conn = http.client.HTTPSConnection(self.base.netloc, timeout=TIMEOUT, context=self.tls)
            else:
                self.conn = http.client.HTTPConnection(self.base.netloc, timeout=TIMEOUT)
        return self.conn

    def pedir(self, metodo: str, path: str, request_id: str, body: dict | None = None,
              token: str | None = None) -> tuple[int, bytes, float, str]:
        """Hace un pedido y lee la respuesta completa.

        Devuelve (código, cuerpo, milisegundos, error); código 0 si no hubo respuesta."""
        headers = {"X-Request-ID": request_id, "Accept": "application/json", "User-Agent": "soloservis-carga"}
        data = None
        if body is not None:
            data = json.dumps(body).encode("utf-8")
            headers["Content-Type"] = "application/json"
        if token:
            headers["Authorization"] = f"Bearer {token}"
        started = time.perf_counter()
        try:
            conn = self._conectar()
            conn.request(metodo, path, body=data, headers=headers)
            resp = conn.getresponse()
            content = resp.read()
            if resp.getheader("Connection", "").lower() == "close":
                self.cerrar()
            return resp.status, content, (time.perf_counter() - started) * 1000, ""
        except (OSError, http.client.HTTPException) as exc:
            self.cerrar()
            kind = "timeout" if isinstance(exc, TimeoutError) or "timed out" in str(exc) else type(exc).__name__
            return 0, b"", (time.perf_counter() - started) * 1000, kind

    def cerrar(self) -> None:
        """Cierra la conexión; el próximo pedido abre otra."""
        if self.conn is not None:
            self.conn.close()
            self.conn = None


class Limitador:
    """Tope global de pedidos por segundo compartido por todos los usuarios (cubeta de fichas)."""

    def __init__(self, rps: float):
        """`rps` 0 o negativo desactiva el tope."""
        self.rps = rps
        self.lock = threading.Lock()
        self.siguiente = time.monotonic()

    def esperar(self, stop: threading.Event) -> bool:
        """Espera el turno del próximo pedido; devuelve False si se pidió detener."""
        if self.rps <= 0:
            return not stop.is_set()
        with self.lock:
            now = time.monotonic()
            turno = max(self.siguiente, now)
            self.siguiente = turno + 1 / self.rps
        while not stop.is_set():
            falta = turno - time.monotonic()
            if falta <= 0:
                return True
            time.sleep(min(falta, 0.2))
        return False


# ---------------------------------------------------------------- la prueba

@dataclass
class Semilla:
    """Datos reales del catálogo para armar pedidos variados (categorias son ids de product_category)."""
    productos: list[str]
    palabras: list[str]
    categorias: list[str]
    total: int


@dataclass
class Prueba:
    """Estado compartido de una corrida: pedidos, fases, frenos y parámetros."""
    base: urllib.parse.SplitResult
    tls: ssl.SSLContext | None
    semilla: Semilla
    espera: tuple[float, float]
    limitador: Limitador
    cuentas: list[dict] = field(default_factory=list)  # para --login; vacío sin login
    pedidos: list[Pedido] = field(default_factory=list)
    fases: list[tuple[str, float, float]] = field(default_factory=list)
    freno: str = ""
    lock: threading.Lock = field(default_factory=threading.Lock)
    contador: int = 0

    def request_id(self) -> str:
        """Siguiente X-Request-ID de la corrida."""
        with self.lock:
            self.contador += 1
            return f"{REQUEST_ID_PREFIX}-{self.contador}"

    def registrar(self, pedido: Pedido) -> None:
        """Guarda el resultado de un pedido."""
        with self.lock:
            self.pedidos.append(pedido)


def preparar_semilla(base: urllib.parse.SplitResult, tls: ssl.SSLContext | None) -> Semilla:
    """Lee productos y categorías reales del objetivo (no cuentan en la medición)."""
    cli = Cliente(base, tls)
    code, body, _, err = cli.pedir("GET", "/products?limit=100", f"{REQUEST_ID_PREFIX}-semilla-1")
    if code != 200:
        raise ClusterError(f"GET /products respondió {code or err}")
    productos = json.loads(body)
    # Los filtros salen del árbol de product_category, como en el frontend: /categories
    # también trae categorías de servicios, que /products?category rechaza con 400.
    code, body, _, err = cli.pedir("GET", "/product-categories", f"{REQUEST_ID_PREFIX}-semilla-2")
    categorias = [str(c["id"]) for c in json.loads(body)] if code == 200 else []
    cli.cerrar()
    if not productos:
        raise ClusterError("el catálogo está vacío: no hay productos para consultar")
    palabras = sorted({w.lower() for p in productos for w in re.findall(r"[A-Za-zÁÉÍÓÚáéíóúñÑ]{4,}", p["name"])[:2]})
    return Semilla([p["id"] for p in productos], palabras or ["producto"], categorias, len(productos))


def recorrido(semilla: Semilla, rnd: random.Random) -> list[tuple[str, str]]:
    """Una visita al sitio: (ruta agrupada, path real) en el orden en que la haría el frontend."""
    lista = f"/products?limit=20&offset={rnd.choice([0, 0, 20, 40])}"
    if semilla.categorias and rnd.random() < 0.5:
        lista = f"/products?limit=20&category={urllib.parse.quote(rnd.choice(semilla.categorias))}"
        etiqueta = "/products?category"
    else:
        etiqueta = "/products"
    q = urllib.parse.quote(rnd.choice(semilla.palabras))
    return [
        (etiqueta, lista),
        ("/categories", "/categories?itemType=product&limit=100"),
        ("/search/products", f"/search/products?q={q}&limit=20"),
        ("/products/{id}", f"/products/{rnd.choice(semilla.productos)}"),
        ("/stores", "/stores"),
    ]


def usuario(prueba: Prueba, escalon: int, n: int, stop: threading.Event) -> None:
    """Un usuario virtual: inicia sesión si corresponde y navega hasta que termine el escalón."""
    rnd = random.Random(f"{RUN_ID}-{escalon}-{n}")
    cli = Cliente(prueba.base, prueba.tls)
    # Arranque escalonado: no llegan todos en el mismo milisegundo.
    if stop.wait(rnd.uniform(0, max(prueba.espera[1], 1))):
        return
    token = None

    def pedir(ruta: str, metodo: str, path: str, body: dict | None = None) -> tuple[int, bytes]:
        """Hace un pedido respetando el tope global y lo registra."""
        if not prueba.limitador.esperar(stop):
            return -1, b""
        code, content, ms, err = cli.pedir(metodo, path, prueba.request_id(), body, token)
        fallido = code == 0 or code >= 400
        prueba.registrar(Pedido(time.time(), escalon, ruta, code, ms, err, path if fallido else ""))
        return code, content

    if prueba.cuentas:
        cuenta = prueba.cuentas[n % len(prueba.cuentas)]
        for _ in range(10):
            code, content = pedir(LOGIN, "POST", "/auth/login", cuenta)
            if code == 200:
                token = json.loads(content).get("accessToken")
            # Ante un 429 espera lo que pide Retry-After (3 s en el gateway) y reintenta.
            if code != 429 or stop.wait(3 + rnd.uniform(0, 2)):
                break
    while not stop.is_set():
        pasos = recorrido(prueba.semilla, rnd)
        if token:
            pasos.append(("/auth/validate", "/auth/validate"))
        for ruta, path in pasos:
            if pedir(ruta, "GET", path)[0] == -1:
                break
            if prueba.espera[1] > 0 and stop.wait(rnd.uniform(*prueba.espera)):
                break
    cli.cerrar()


def percentil(valores: list[float], p: float) -> float | None:
    """Percentil p (0-100) por el método del rango más cercano; None si no hay valores."""
    if not valores:
        return None
    ordenados = sorted(valores)
    k = max(0, min(len(ordenados) - 1, math.ceil(p / 100 * len(ordenados)) - 1))
    return ordenados[k]


LOGIN = "POST /auth/login"


def es_error(p: Pedido) -> bool:
    """Un pedido es error si no hubo respuesta, si es 5xx o si lo limitaron (429).

    El 429 del login no cuenta: todos los usuarios virtuales salen de la misma IP
    y el gateway permite 20 logins por minuto por IP (TAL-201), así que limitarlos
    es lo esperado. El usuario reintenta como lo haría una persona."""
    return p.codigo == 0 or (p.codigo == 429 and p.ruta != LOGIN) or p.codigo >= 500


def revisar_frenos(prueba: Prueba, monitor: Monitor | None, ultimo_reinicio: list[float]) -> str:
    """Revisa los frenos sobre los últimos segundos; devuelve el motivo si hay que cortar."""
    desde = time.time() - VENTANA_FRENO
    with prueba.lock:
        recientes = [p for p in prueba.pedidos[-5000:] if p.t >= desde]
    if len(recientes) >= FRENO_MIN_PEDIDOS:
        errores = sum(es_error(p) for p in recientes)
        if errores / len(recientes) > FRENO_ERRORES:
            codigos = Counter(p.codigo or p.error for p in recientes if es_error(p))
            return (f"errores {errores}/{len(recientes)} ({errores / len(recientes):.0%}) en {VENTANA_FRENO} s: "
                    + ", ".join(f"{c}×{n}" for c, n in codigos.most_common()))
        p95 = percentil([p.ms for p in recientes], 95)
        if p95 and p95 > FRENO_P95_MS:
            return f"p95 {p95:.0f} ms en los últimos {VENTANA_FRENO} s (máximo {FRENO_P95_MS} ms)"
    if monitor and time.time() - ultimo_reinicio[0] >= 30:
        ultimo_reinicio[0] = time.time()
        reinicios = monitor.reinicios()
        if reinicios:
            return "reinicio de pods: " + "; ".join(reinicios)
    return ""


def correr_escalon(prueba: Prueba, escalon: int, usuarios: int, segundos: float,
                   monitor: Monitor | None, frenos: bool) -> None:
    """Corre `usuarios` usuarios durante `segundos`, revisando los frenos cada 5 s."""
    stop = threading.Event()
    hilos = [threading.Thread(target=usuario, args=(prueba, escalon, i, stop), daemon=True) for i in range(usuarios)]
    inicio = time.time()
    for hilo in hilos:
        hilo.start()
    ultimo_reinicio = [time.time()]
    while time.time() - inicio < segundos:
        time.sleep(max(0.0, min(5.0, segundos - (time.time() - inicio))))
        if frenos:
            motivo = revisar_frenos(prueba, monitor, ultimo_reinicio)
            if motivo:
                prueba.freno = f"escalón de {usuarios} usuarios: {motivo}"
                break
        hechos = sum(1 for p in prueba.pedidos if p.escalon == escalon)
        print(f"  {usuarios} usuarios: {time.time() - inicio:4.0f}/{segundos:.0f} s, {hechos} pedidos", flush=True)
    stop.set()
    for hilo in hilos:
        hilo.join(timeout=TIMEOUT + 5)
    prueba.fases.append((f"{usuarios} usuarios", inicio, time.time()))


def pausa(prueba: Prueba, nombre: str, segundos: float) -> None:
    """Fase sin carga (reposo antes, entre escalones y después), para comparar el consumo."""
    if segundos <= 0:
        return
    print(f"  {nombre}: {segundos:.0f} s sin carga", flush=True)
    inicio = time.time()
    time.sleep(segundos)
    prueba.fases.append((nombre, inicio, time.time()))


# ---------------------------------------------------------------- entorno local

def docker(args: list[str], timeout: int = 300, stdin: bytes | None = None) -> str:
    """Ejecuta docker y devuelve stdout; lanza ClusterError con el error si falla."""
    try:
        proc = subprocess.run(["docker", *args], input=stdin, check=False, capture_output=True, timeout=timeout)
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise ClusterError(f"docker {args[0]}: {exc}") from exc
    if proc.returncode != 0:
        lines = proc.stderr.decode("utf-8", "replace").strip().splitlines() or ["sin salida"]
        raise ClusterError(f"docker {' '.join(args[:2])}: {lines[-1]}")
    return proc.stdout.decode("utf-8", "replace")


def imagen_local(nombre: str, pedida: str | None) -> str:
    """Imagen local a usar: la pedida, la del commit actual o la más reciente de ese repositorio."""
    if pedida:
        return pedida
    tags = docker(["images", "--format", "{{.Repository}}:{{.Tag}}", f"shell000/soloservis-{nombre}"]).split()
    if not tags:
        raise ClusterError(f"no hay imagen local shell000/soloservis-{nombre}; pásala con --imagen-{nombre}")
    head = subprocess.run(["git", "rev-parse", "--short", "HEAD"], cwd=ROOT, check=False, capture_output=True,
                          text=True).stdout.strip()
    return next((t for t in tags if t.endswith(":" + head)), tags[0])


def limpiar_local() -> None:
    """Borra los contenedores y la red desechables de la prueba local (solo los soloservis-carga-*)."""
    for name in LOCAL_LIMITS:
        subprocess.run(["docker", "rm", "-f", LOCAL_PREFIJO + name], check=False, capture_output=True)
    subprocess.run(["docker", "network", "rm", LOCAL_RED], check=False, capture_output=True)


def levantar_local(img_backend: str, img_frontend: str, n_cuentas: int) -> list[dict]:
    """Levanta PostgreSQL con migraciones, comercial, usuarios y el gateway con los limits del clúster.

    Registra `n_cuentas` usuarios de prueba para --login y devuelve sus credenciales."""
    try:
        docker(["info", "--format", "{{.ServerVersion}}"], timeout=30)
    except ClusterError as exc:
        raise ClusterError("Docker no está corriendo (no lo inicio por mi cuenta)") from exc
    limpiar_local()
    password = secrets.token_hex(16)
    token = secrets.token_hex(24)
    docker(["network", "create", LOCAL_RED])

    def run(name: str, image: str, *extra: str) -> None:
        """Arranca un contenedor desechable con los limits del clúster para ese servicio."""
        cpus, mem = LOCAL_LIMITS[name]
        docker(["run", "-d", "--name", LOCAL_PREFIJO + name, "--network", LOCAL_RED, "--network-alias", name,
                "--cpus", cpus, "--memory", mem, *extra, image])

    print("  PostgreSQL 18 desechable…", flush=True)
    run("postgres", "postgres:18", "-e", f"POSTGRES_PASSWORD={password}", "-e", "POSTGRES_DB=soloservis",
        "--tmpfs", "/var/lib/postgresql")
    for _ in range(60):
        if subprocess.run(["docker", "exec", LOCAL_PREFIJO + "postgres", "pg_isready", "-U", "postgres"],
                          check=False, capture_output=True).returncode == 0:
            break
        time.sleep(1)
    else:
        raise ClusterError("PostgreSQL no quedó listo en 60 s")
    time.sleep(2)  # el entrypoint reinicia el servidor una vez tras crear la base
    migrations = ROOT / "backend" / "database" / "migrations"
    order = (ROOT / "backend" / "database" / "migration_order.txt").read_text(encoding="utf-8").splitlines()
    names = [n.strip() for n in order if n.strip().endswith(".sql")]
    print(f"  {len(names)} migraciones…", flush=True)
    for name in names:
        docker(["exec", "-i", LOCAL_PREFIJO + "postgres", "psql", "-X", "-q", "-v", "ON_ERROR_STOP=1",
                "--single-transaction", "-U", "postgres", "-d", "soloservis"],
               stdin=(migrations / name).read_bytes())

    db = f"postgres://postgres:{password}@postgres:5432/soloservis?sslmode=disable"
    for name in ("comercial", "usuarios"):
        extra = ["-e", f"API_DOMAIN={name}", "-e", "PORT=8080", "-e", f"DATABASE_URL={db}",
                 "-e", f"INTERNAL_API_TOKEN={token}", "-e", f"FRONTEND_URL=http://127.0.0.1:{LOCAL_PUERTO_GATEWAY}"]
        if name == "comercial":
            extra += ["-p", f"127.0.0.1:{LOCAL_PUERTO_COMERCIAL}:8080"]
        run(name, img_backend, *extra)
    upstreams = {k: "comercial:8080" for k in ("PRODUCT", "STORE", "SERVICE", "FAVORITES", "CART", "SEARCH",
                                               "COMPARISON", "REVIEW", "WATCHLIST")}
    # Igual que en k8s/components/dominios: auth y user van al dominio usuarios.
    upstreams["AUTH"] = upstreams["USER"] = "usuarios:8080"
    env = [x for k, v in upstreams.items() for x in ("-e", f"{k}_UPSTREAM={v}")]
    run("frontend", img_frontend, "-p", f"127.0.0.1:{LOCAL_PUERTO_GATEWAY}:8080", *env)

    base = urllib.parse.urlsplit(f"http://127.0.0.1:{LOCAL_PUERTO_GATEWAY}")
    cli = Cliente(base, None)
    for _ in range(60):
        code, _, _, _ = cli.pedir("GET", "/products?limit=1", f"{REQUEST_ID_PREFIX}-espera")
        if code == 200:
            break
        time.sleep(1)
    else:
        raise ClusterError("el gateway local no respondió /products en 60 s")
    cuentas = []
    for i in range(n_cuentas):
        cuenta = {"email": f"carga-{RUN_ID}-{i}@example.com", "password": secrets.token_urlsafe(18)}
        # El gateway admite 20 registros por minuto por IP: ante un 429 espera y reintenta.
        for _ in range(10):
            code, body, _, err = cli.pedir("POST", "/auth/register", f"{REQUEST_ID_PREFIX}-registro-{i}",
                                           {"name": "Usuario de carga", **cuenta})
            if code != 429:
                break
            time.sleep(3)
        if code != 201:
            raise ClusterError(f"no se pudo registrar el usuario de prueba: {code or err} {body[:200]!r}")
        cuentas.append(cuenta)
    cli.cerrar()
    return cuentas


# ---------------------------------------------------------------- reporte

def ms(value: float | None) -> str:
    """Milisegundos a texto; «—» si no hay valor."""
    return "—" if value is None else f"{value:.0f}"


def codigos(pedidos: list[Pedido]) -> str:
    """Conteo de códigos de respuesta, del más frecuente al menos ("200×950, 429×3")."""
    cuenta = Counter(str(p.codigo) if p.codigo else (p.error or "sin respuesta") for p in pedidos)
    return ", ".join(f"{c}×{n}" for c, n in cuenta.most_common())


def tabla_escalones(prueba: Prueba, escalones: list[int]) -> list[str]:
    """Una fila por escalón: pedidos, throughput, latencias, errores y códigos."""
    lines = ["| Usuarios | Pedidos | r/s | p50 ms | p95 ms | p99 ms | máx ms | Errores | Códigos |",
             "| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |"]
    duraciones = {nombre: fin - inicio for nombre, inicio, fin in prueba.fases}
    for i, usuarios in enumerate(escalones):
        ps = [p for p in prueba.pedidos if p.escalon == i]
        if not ps:
            continue
        lat = [p.ms for p in ps if p.codigo]
        seg = duraciones.get(f"{usuarios} usuarios") or 1
        err = sum(es_error(p) for p in ps)
        lines.append(f"| {usuarios} | {len(ps)} | {len(ps) / seg:.1f} | {ms(percentil(lat, 50))} | "
                     f"{ms(percentil(lat, 95))} | {ms(percentil(lat, 99))} | {ms(max(lat) if lat else None)} | "
                     f"{err} ({err / len(ps):.1%}) | {codigos(ps)} |")
    return lines


def tabla_rutas(pedidos: list[Pedido]) -> list[str]:
    """Una fila por ruta con todos los escalones juntos."""
    lines = ["| Ruta | Pedidos | p50 ms | p95 ms | p99 ms | máx ms | Códigos |",
             "| --- | ---: | ---: | ---: | ---: | ---: | --- |"]
    por_ruta: dict[str, list[Pedido]] = defaultdict(list)
    for p in pedidos:
        por_ruta[p.ruta].append(p)
    for ruta, ps in sorted(por_ruta.items()):
        lat = [p.ms for p in ps if p.codigo]
        lines.append(f"| `{ruta}` | {len(ps)} | {ms(percentil(lat, 50))} | {ms(percentil(lat, 95))} | "
                     f"{ms(percentil(lat, 99))} | {ms(max(lat) if lat else None)} | {codigos(ps)} |")
    return lines


def tabla_fases(monitor: Monitor, prueba: Prueba) -> list[str]:
    """Consumo de cada contenedor en cada fase: una tabla por servicio, una fila por fase."""
    lines = []
    resumenes = [(nombre, monitor.resumen(inicio, fin)) for nombre, inicio, fin in prueba.fases]
    for cont in monitor.contenedores:
        titulo = (f"**{cont.servicio}** (CPU req {fmt_cpu(cont.req_cpu)} / lim {fmt_cpu(cont.lim_cpu)}; "
                  f"memoria req {fmt_mem(cont.req_mem)} / lim {fmt_mem(cont.lim_mem)})")
        lines += [titulo, "",
                  "| Fase | CPU media | CPU máx | Throttling | Memoria máx |", "| --- | ---: | ---: | ---: | ---: |"]
        for nombre, resumen in resumenes:
            r = resumen[cont.servicio]
            thr = "—" if r["throttling"] is None else f"{r['throttling']:.0%} de {r['periodos']}"
            lines.append(f"| {nombre} | {fmt_cpu(r['cpu_media'])}{fmt_pct(r['cpu_media'], cont.lim_cpu)} | "
                         f"{fmt_cpu(r['cpu_max'])}{fmt_pct(r['cpu_max'], cont.lim_cpu)} | {thr} | "
                         f"{fmt_mem(r['mem_max'])}{fmt_pct(r['mem_max'], cont.lim_mem)} |")
        lines.append("")
    return lines


def render(prueba: Prueba, monitor: Monitor | None, meta: dict[str, str], escalones: list[int],
           reinicios: list[str]) -> str:
    """Arma el reporte Markdown de la corrida."""
    ps = prueba.pedidos
    lat = [p.ms for p in ps if p.codigo]
    err = sum(es_error(p) for p in ps)
    resultado = f"cortada por un freno: {prueba.freno}" if prueba.freno else "completa"
    lines = [f"# Prueba de carga — {meta['Entorno']}", ""]
    lines += [f"- **{k}:** {v}" for k, v in meta.items()]
    total = (f"En total: {len(ps)} pedidos, p50 {ms(percentil(lat, 50))} ms, p95 {ms(percentil(lat, 95))} ms, "
             f"p99 {ms(percentil(lat, 99))} ms, {err} errores ({err / len(ps):.1%}).") if ps else "Sin pedidos."
    lines += [f"- **Resultado:** {resultado}", "", total, ""]
    lines += ["## Por escalón", ""] + tabla_escalones(prueba, escalones) + [""]
    lines += [("Errores: sin respuesta (timeout o conexión), 429 y 5xx. El 429 del login no cuenta como error: "
               "todos los usuarios virtuales salen de una IP y el gateway admite 20 logins por minuto por IP. "
               "Las latencias son desde este equipo e incluyen la red hasta el objetivo."), ""]
    lines += ["## Por ruta", ""] + tabla_rutas(ps) + [""]
    fallidos = Counter(f"{p.codigo or p.error} `{p.path}`" for p in ps if p.path)
    if fallidos:
        lines += ["Pedidos que no respondieron 2xx:", ""]
        lines += [f"- {n}× {k}" for k, n in fallidos.most_common(15)] + [""]
    if monitor:
        lines += ["## Consumo de recursos (TAL-207)", "",
                  f"Fuente: {monitor.a_dict()['fuente']}, cgroup v2, una muestra cada {monitor.intervalo:g} s.", ""]
        lines += ["### Toda la corrida", ""] + tabla_contenedores(monitor, monitor.resumen()) + [""]
        lines += ["### Por fase", ""] + tabla_fases(monitor, prueba)
        cuota = tabla_cuota(monitor, monitor.resumen())
        if cuota:
            lines += ["### Cuota del namespace", ""] + cuota + [""]
        icons = {"OK": "✅", "AVISO": "⚠️", "FALLA": "❌"}
        lines += ["### Evaluación", ""]
        lines += [f"- {icons[e]} **{s}:** {d}" for e, s, d in evaluar(monitor, monitor.resumen(), reinicios)]
        lines.append(f"- Reinicios u OOMKilled durante la prueba: {'; '.join(reinicios) if reinicios else 'ninguno'}")
        lines.append("")
    return "\n".join(lines)


def git(*args: str) -> str:
    """Salida de un comando git en la raíz del repositorio."""
    return subprocess.run(["git", *args], cwd=ROOT, check=False, capture_output=True, text=True).stdout.strip()


# ---------------------------------------------------------------- main

def main() -> int:
    """Prepara el objetivo, corre la carga midiendo recursos y escribe el reporte.

    Devuelve 0 si la prueba terminó sin frenos ni 5xx, 1 si no, y 2 si no se pudo preparar."""
    parser = argparse.ArgumentParser(description="Prueba de carga de SOLOServis con medición de recursos.")
    parser.add_argument("--url", help="URL del objetivo (por defecto, el host del Ingress del clúster)")
    parser.add_argument("--local", action="store_true", help="levanta contenedores desechables y prueba ahí")
    parser.add_argument("--directo", action="store_true", help="con --local, pega a comercial sin el gateway")
    parser.add_argument("--escalones", default="5,10,20", help="usuarios por escalón (5,10,20)")
    parser.add_argument("--duracion", type=float, default=120, help="segundos por escalón (120)")
    parser.add_argument("--pausa", type=float, default=30, help="segundos sin carga antes, entre y después (30)")
    parser.add_argument("--espera", default="1-3", help="espera entre páginas de un usuario, en s (1-3; 0 = sin espera)")
    parser.add_argument("--max-rps", type=float, default=15, help="tope global de pedidos por segundo (15; 0 = sin tope)")
    parser.add_argument("--login", action="store_true", help="con --local, cada usuario inicia sesión")
    parser.add_argument("--sin-frenos", action="store_true", help="con --local, no corta por errores ni latencia")
    parser.add_argument("--sin-recursos", action="store_true", help="no mide CPU ni memoria")
    parser.add_argument("--sin-verificar-tls", action="store_true",
                        help="acepta el certificado autofirmado de la plataforma (UCT)")
    parser.add_argument("--intervalo", type=float, default=5, help="segundos entre muestras de recursos (5)")
    parser.add_argument("--imagen-backend", help="imagen para --local (por defecto shell000/soloservis-backend)")
    parser.add_argument("--imagen-frontend", help="imagen para --local (por defecto shell000/soloservis-frontend)")
    parser.add_argument("--mantener", action="store_true", help="con --local, no borra los contenedores al final")
    parser.add_argument("--out", default="docs/reportes/carga", help="carpeta del reporte")
    args = parser.parse_args()
    for stream in (sys.stdout, sys.stderr):
        if hasattr(stream, "reconfigure"):
            stream.reconfigure(encoding="utf-8")

    if not args.local and (args.login or args.directo or args.sin_frenos):
        parser.error("--login, --directo y --sin-frenos solo se usan con --local")
    escalones = [int(x) for x in args.escalones.split(",") if x.strip()]
    lo, _, hi = args.espera.partition("-")
    espera = (float(lo), float(hi or lo))

    started = dt.datetime.now().astimezone()
    cuentas: list[dict] = []
    try:
        if args.local:
            img_backend = imagen_local("backend", args.imagen_backend)
            img_frontend = imagen_local("frontend", args.imagen_frontend)
            print(f"Entorno local: {img_backend}, {img_frontend}", flush=True)
            n_cuentas = -(-max(escalones) // USUARIOS_POR_CUENTA) if args.login else 1
            cuentas = levantar_local(img_backend, img_frontend, n_cuentas)
            url = f"http://127.0.0.1:{LOCAL_PUERTO_COMERCIAL if args.directo else LOCAL_PUERTO_GATEWAY}"
            entorno = "local, directo a comercial" if args.directo else "local, por el gateway"
        elif args.url:
            url, entorno = args.url, args.url
        else:
            hosts = [r["host"] for i in kget("ingress") for r in i["spec"].get("rules", []) if r.get("host")]
            if not hosts:
                raise ClusterError("no hay Ingress con host; pasa --url")
            url, entorno = f"https://{hosts[0]}", "clúster UCT"
        base = urllib.parse.urlsplit(url)
        tls = None
        if base.scheme == "https":
            tls = ssl.create_default_context()
            if args.sin_verificar_tls:
                tls.check_hostname = False
                tls.verify_mode = ssl.CERT_NONE
        semilla = preparar_semilla(base, tls)
    except (ClusterError, ssl.SSLError) as exc:
        print(f"ERROR: no se pudo preparar la prueba: {exc}", file=sys.stderr)
        if isinstance(exc, ssl.SSLError) or "CERTIFICATE" in str(exc):
            print("Si es el certificado autofirmado de la plataforma, usa --sin-verificar-tls.", file=sys.stderr)
        if args.local and not args.mantener:
            limpiar_local()
        return 2

    prueba = Prueba(base, tls, semilla, espera, Limitador(args.max_rps),
                    cuentas=cuentas if args.login else [])
    total = len(escalones) * (args.duracion + args.pausa) + args.pausa
    monitor = None
    reinicios: list[str] = []
    print(f"Objetivo: {url} ({entorno}); {semilla.total} productos y {len(semilla.categorias)} categorías de semilla")
    print(f"Escalones {escalones} × {args.duracion:g} s, espera {args.espera} s, tope "
          f"{args.max_rps:g} r/s, unos {total / 60:.1f} min", flush=True)
    try:
        if not args.sin_recursos:
            monitor = Monitor(args.intervalo, total + 120, LOCAL_PREFIJO if args.local else None)
            monitor.iniciar()
        pausa(prueba, "reposo antes", args.pausa)
        for i, usuarios in enumerate(escalones):
            print(f"Escalón {i + 1}/{len(escalones)}: {usuarios} usuarios", flush=True)
            correr_escalon(prueba, i, usuarios, args.duracion, monitor, not args.sin_frenos)
            if prueba.freno:
                print(f"FRENO: {prueba.freno}", flush=True)
                break
            pausa(prueba, "reposo después" if i == len(escalones) - 1 else f"pausa tras {usuarios}", args.pausa)
        if monitor:
            reinicios = monitor.reinicios()
    except KeyboardInterrupt:
        prueba.freno = "interrumpida a mano"
    except ClusterError as exc:
        print(f"ERROR durante la prueba: {exc}", file=sys.stderr)
        prueba.freno = f"error: {exc}"
    finally:
        if monitor:
            monitor.detener()
        if args.local and not args.mantener:
            limpiar_local()

    meta = {
        "Fecha": started.strftime("%Y-%m-%d %H:%M %z"),
        "Rama y commit": f"`{git('rev-parse', '--abbrev-ref', 'HEAD')}` @ `{git('rev-parse', '--short', 'HEAD')}`",
        "Entorno": entorno,
        "Objetivo": f"`{url}`" + (" (certificado de la plataforma sin verificar)" if args.sin_verificar_tls else ""),
        "Escalones": f"{', '.join(map(str, escalones))} usuarios, {args.duracion:g} s cada uno, "
                     f"{args.pausa:g} s de pausa entre escalones",
        "Usuario virtual": f"lista, categorías, búsqueda, detalle y tiendas, con {args.espera} s de espera entre "
                           "páginas" + (f", con login ({len(prueba.cuentas)} cuentas, una cada "
                                        f"{USUARIOS_POR_CUENTA} usuarios)" if prueba.cuentas else ""),
        "Tope global": f"{args.max_rps:g} r/s" if args.max_rps > 0 else "sin tope",
        "Frenos": "desactivados" if args.sin_frenos else
                  f"errores > {FRENO_ERRORES:.0%} o p95 > {FRENO_P95_MS} ms en {VENTANA_FRENO} s, o reinicio de pods",
        "Request ID": f"`{REQUEST_ID_PREFIX}-*`",
        "Generado con": "`python scripts/testing/carga.py " + " ".join(sys.argv[1:]) + "`",
    }
    text = render(prueba, monitor, meta, escalones, reinicios)
    slug = "uct" if entorno == "clúster UCT" else "local-directo" if args.directo else "local" if args.local else "url"
    stamp = started.strftime("%Y-%m-%d-%H%M")
    out_dir = ROOT / args.out
    out_dir.mkdir(parents=True, exist_ok=True)
    report = out_dir / f"{stamp}-{slug}.md"
    report.write_text(text + "\n", encoding="utf-8")
    raw_dir = ROOT / "tmp" / "reportes-carga"
    raw_dir.mkdir(parents=True, exist_ok=True)
    (raw_dir / f"{stamp}-{slug}.json").write_text(json.dumps({
        "meta": meta, "freno": prueba.freno, "fases": prueba.fases, "reinicios": reinicios,
        "pedidos": [p.__dict__ for p in prueba.pedidos],
        "recursos": monitor.a_dict() if monitor else None}, ensure_ascii=False), encoding="utf-8")
    print()
    print(text)
    print(f"Reporte: {report.relative_to(ROOT)}")
    hubo_5xx = any(p.codigo >= 500 for p in prueba.pedidos)
    return 1 if prueba.freno or hubo_5xx else 0


if __name__ == "__main__":
    sys.exit(main())
