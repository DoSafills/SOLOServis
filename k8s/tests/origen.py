"""Ventana "origen" del visor de llamadas.

Corre los escenarios de prueba contra el despliegue real y, por cada llamada,
muestra lo que envía, lo que le vuelve y si coincide con lo esperado. Todas las
llamadas pasan por el sidecar `destino` (127.0.0.1:9000), que muestra en su
propia ventana lo que recibió cada servicio.

  - TAL-165 (escenarios.py): solicitudes con distintos parámetros, por el
    gateway y directo, con y sin token.
  - TAL-166 (escenarios.py): entradas válidas, inválidas, incompletas y
    límite, y la forma de cada salida.
  - TAL-167 (flujo.py): el flujo scraper -> ingestion -> catálogo -> gateway
    con los datos de la última corrida real.

Es de solo lectura: las únicas escrituras son pedidos que la API rechaza al
decodificar o validar, antes de llegar a la base. El token de servicio viene
del Secret soloservis-scraper (TAL-199) y nunca se escribe en la salida.

Variables: TICKETS (por defecto, todos los de ESCENARIOS), PAUSA (segundos entre
llamadas, para poder leer las ventanas), SCRAPER_TOKEN (opcional).
Sale con código 1 si alguna verificación falla.
"""

from __future__ import annotations

import importlib
import json
import os
import re
import sys
import time
import urllib.error
import urllib.request
from concurrent.futures import ThreadPoolExecutor
from dataclasses import dataclass, field
from collections.abc import Callable

PROXY = os.environ.get("PROXY", "http://127.0.0.1:9000")
TOKEN = os.environ.get("SCRAPER_TOKEN", "")
PAUSA = float(os.environ.get("PAUSA", "0.4"))
EXTRACTO = int(os.environ.get("EXTRACTO", "600"))
TROZO_REGISTRO = 8000  # largo de cada línea @@@REGISTRO (el runtime corta las líneas de más de 16 KB)

OK, FALLA, AVISO, OMITIDA = "OK", "FALLA", "AVISO", "OMITIDA"

# Ticket -> (módulo, función) con sus escenarios. Cada función recibe (Visor, contexto).
ESCENARIOS = {
    "165": ("escenarios", "solicitudes"),
    "166": ("escenarios", "entradas_y_salidas"),
    "167": ("flujo", "flujo_completo"),
}

# URL que se muestra para cada servicio lógico (la que usaría el origen real).
URLS = {"frontend": "http://frontend:80", "comercial": "http://comercial:8080", "usuarios": "http://usuarios:8080",
        "ingestion": "http://ingestion:8080", "backend": "http://backend:8080", "scraper": "http://scraper-api:8095"}
# Servicios del backend Go: devuelven X-Request-ID y problem+json.
GO = {"comercial", "usuarios", "ingestion", "backend"}

DETALLES_INTERNOS = re.compile(r"pgx|SQLSTATE|sql:|panic|goroutine|runtime error|\.go:\d+|stack trace", re.I)
# UTF-8 leído con otra página de códigos (cp850 o Windows-1252). Va con escapes para que el
# archivo no traiga esos caracteres: el deploy los busca en lo que aplica (TAL-183).
MOJIBAKE = re.compile("[\u00c2\u00c3][\u0080-\u00bf]|\u251c|\u252c|\u00e2\u20ac")
CAMEL = re.compile(r"^[a-z][a-zA-Z0-9]*$")


@dataclass
class Caso:
    """Una llamada de prueba y lo que se espera de ella.

    `ruta` admite {pid}, {pid2}, {oid}, {sid}, {svc}: se completan con datos
    reales leídos en la preparación. `espera` es el código HTTP esperado (o una
    tupla de códigos válidos). `token` es None, "servicio" o "invalido".
    `comprueba` recibe (respuesta, contexto) y devuelve un texto con el problema,
    o None si está bien."""

    ticket: str
    slug: str
    nombre: str
    servicio: str
    ruta: str
    espera: int | tuple[int, ...] = 200
    metodo: str = "GET"
    body: object = None  # dict -> JSON; str o bytes -> tal cual
    tipo_body: str = "application/json"
    token: str | None = None
    codigo: str = ""  # code esperado del problem+json
    forma: str = "json"  # json, lista, problem, html o texto
    total: bool = False  # exige X-Total-Count numérico
    contiene: str = ""
    comprueba: Callable | None = None
    igual_a: str = ""  # id de otra llamada cuya respuesta debe ser igual (sin request_id)
    salidas: bool = True  # aplica las comprobaciones generales de salida


@dataclass
class Respuesta:
    """Lo que volvió de una llamada: código, headers (en minúsculas), body y tiempo."""

    estado: int
    headers: dict[str, str]
    body: bytes
    ms: float

    @property
    def texto(self) -> str:
        """Body decodificado como UTF-8."""
        return self.body.decode("utf-8", "replace")

    @property
    def tipo(self) -> str:
        """Content-Type sin parámetros (charset)."""
        return self.headers.get("content-type", "").split(";")[0].strip()

    def json(self):
        """Body como JSON, o None si no lo es."""
        try:
            return json.loads(self.body)
        except ValueError:
            return None


@dataclass
class Resultado:
    """Una llamada hecha, con su veredicto. Es lo que va al reporte."""

    id: str
    ticket: str
    nombre: str
    metodo: str
    url: str
    headers: dict[str, str]
    body: str
    espera: str
    estado: str = OMITIDA
    motivos: list[str] = field(default_factory=list)
    obtenido: str = ""
    extracto: str = ""
    ms: float = 0.0


def una_linea(texto: str, largo: int) -> str:
    """Junta el texto en una línea y lo corta a `largo` caracteres."""
    texto = " ".join(texto.split())
    return texto[:largo] + (f"… (+{len(texto) - largo} caracteres)" if len(texto) > largo else "")


def sin_request_id(texto: str) -> str:
    """Quita el request_id de un body para comparar respuestas de pedidos distintos."""
    return re.sub(r'"request_id"\s*:\s*"[^"]*"', "", texto)


class Visor:
    """Hace las llamadas a través del sidecar `destino`, las muestra y las evalúa."""

    def __init__(self, topologia: str):
        """Recibe la topología ("dominios" o "consolidado") para resolver cada servicio."""
        self.topologia = topologia
        self.resultados: list[Resultado] = []
        self.respuestas: dict[str, Respuesta] = {}
        self.contador: dict[str, int] = {}

    def servicio(self, nombre: str) -> str:
        """En la topología consolidada, comercial, usuarios e ingestion son el Service backend."""
        return "backend" if self.topologia == "consolidado" and nombre in ("comercial", "usuarios", "ingestion") \
            else nombre

    def nuevo_id(self, ticket: str, slug: str) -> str:
        """Arma el X-Request-ID de la llamada: t<ticket>-<nn>-<slug>, hasta 64 caracteres [A-Za-z0-9-]."""
        self.contador[ticket] = self.contador.get(ticket, 0) + 1
        limpio = re.sub(r"[^a-z0-9-]+", "-", slug.lower()).strip("-")
        return f"t{ticket}-{self.contador[ticket]:02d}-{limpio}"[:64].rstrip("-")

    def enviar(self, rid: str, metodo: str, servicio: str, ruta: str, body: bytes | None,
               headers: dict[str, str]) -> Respuesta | None:
        """Envía un pedido al sidecar, que lo reenvía al servicio.

        Devuelve la respuesta (también las 4xx y 5xx) o None si no hubo respuesta."""
        pedido = urllib.request.Request(f"{PROXY}/{servicio}{ruta}", data=body, method=metodo)
        for k, v in headers.items():
            pedido.add_header(k, v)
        inicio = time.monotonic()
        try:
            with urllib.request.urlopen(pedido, timeout=40) as resp:
                estado, h, datos = resp.status, resp.headers, resp.read()
        except urllib.error.HTTPError as err:
            estado, h, datos = err.code, err.headers, err.read()
        except (urllib.error.URLError, OSError) as err:
            print(f"  SIN RESPUESTA del visor: {getattr(err, 'reason', err)}", flush=True)
            return None
        return Respuesta(estado, {k.lower(): v for k, v in h.items()}, datos, (time.monotonic() - inicio) * 1000)

    def llamar(self, caso: Caso, ctx: dict) -> Respuesta | None:
        """Hace la llamada de un caso, la muestra en la ventana y registra su veredicto.

        Recibe el caso y el contexto con los ids reales. Devuelve la respuesta, o
        None si el caso se omitió (falta un dato o el token) o no hubo respuesta."""
        rid = self.nuevo_id(caso.ticket, caso.slug)
        servicio = self.servicio(caso.servicio)
        espera = caso.espera if isinstance(caso.espera, tuple) else (caso.espera,)
        espera_txt = " o ".join(map(str, espera)) + (f" {caso.forma}" if caso.forma != "json" else "") \
            + (f" code={caso.codigo}" if caso.codigo else "")
        try:
            ruta = caso.ruta.format(**ctx)
        except KeyError as falta:
            return self.omitir(rid, caso, servicio, caso.ruta, espera_txt, f"falta el dato {falta} de la preparación")
        headers = {"X-Request-ID": rid, "User-Agent": f"visor-llamadas {rid}", "Accept": "application/json"}
        if caso.token == "servicio":
            if not TOKEN:
                return self.omitir(rid, caso, servicio, ruta, espera_txt, "no hay token de servicio (Secret ausente)")
            headers["Authorization"] = f"Bearer {TOKEN}"
        elif caso.token == "invalido":
            headers["Authorization"] = "Bearer token-que-no-existe-0000"
        body = caso.body
        if isinstance(body, (dict, list)):
            body = json.dumps(body, ensure_ascii=False)
        if isinstance(body, str):
            body = body.encode("utf-8")
        if body is not None and caso.tipo_body:
            headers["Content-Type"] = caso.tipo_body

        visibles = {k: ("Bearer **** (" + str(len(TOKEN)) + " caracteres)" if k == "Authorization"
                        and caso.token == "servicio" else v) for k, v in headers.items() if k != "User-Agent"}
        url = URLS[servicio] + ruta
        print(f"\n━━ [{rid}] TAL-{caso.ticket} · {caso.nombre}", flush=True)
        print(f"  ENVÍA   {caso.metodo} {una_linea(url, 300)}")
        print("          " + " · ".join(f"{k}: {v}" for k, v in visibles.items()))
        if body is not None:
            print(f"          body ({len(body)} bytes): {una_linea(body.decode('utf-8', 'replace'), EXTRACTO)}")
        res = Resultado(rid, caso.ticket, caso.nombre, caso.metodo, url, visibles,
                        una_linea(body.decode("utf-8", "replace"), 2000) if body else "", espera_txt)
        resp = self.enviar(rid, caso.metodo, servicio, ruta, body, headers)
        if resp is None:
            res.estado, res.motivos = FALLA, ["sin respuesta"]
            self.cerrar(res)
            return None
        self.respuestas[rid] = resp
        res.ms, res.extracto = round(resp.ms, 1), una_linea(resp.texto, 2000)
        res.obtenido = f"{resp.estado} {resp.tipo}".strip()
        print(f"  RECIBE  {resp.estado} {resp.tipo or '(sin Content-Type)'} · {resp.ms:.0f} ms"
              + (f" · X-Request-ID: {resp.headers['x-request-id']}" if "x-request-id" in resp.headers else "")
              + (f" · X-Total-Count: {resp.headers['x-total-count']}" if "x-total-count" in resp.headers else ""))
        print(f"          {una_linea(resp.texto, EXTRACTO) or '(vacío)'}")
        print(f"  ESPERA  {espera_txt}")
        res.motivos, avisos = self.evaluar(caso, resp, ctx, espera, rid, servicio)
        res.estado = FALLA if res.motivos else AVISO if avisos else OK
        res.motivos += avisos
        self.cerrar(res)
        return resp

    def rafaga(self, caso: Caso, ctx: dict, n: int) -> None:
        """Envía `n` copias del GET de un caso al mismo tiempo y evalúa cada una.

        Además verifica que todas las respuestas sean iguales (sin el
        request_id). No devuelve nada: los resultados quedan en self.resultados."""
        servicio = self.servicio(caso.servicio)
        ruta = caso.ruta.format(**ctx)
        espera = caso.espera if isinstance(caso.espera, tuple) else (caso.espera,)
        ids = [self.nuevo_id(caso.ticket, f"{caso.slug}-{i + 1:02d}") for i in range(n)]
        url = URLS[servicio] + ruta
        print(f"\n━━ [{ids[0]} … {ids[-1]}] TAL-{caso.ticket} · {caso.nombre}", flush=True)
        print(f"  ENVÍA   {n} × GET {url} al mismo tiempo, cada uno con su X-Request-ID")

        def uno(rid: str) -> Respuesta | None:
            """Envía una copia del pedido con su propio X-Request-ID."""
            headers = {"X-Request-ID": rid, "User-Agent": f"visor-llamadas {rid}", "Accept": "application/json"}
            return self.enviar(rid, "GET", servicio, ruta, None, headers)

        with ThreadPoolExecutor(max_workers=n) as pool:
            respuestas = list(pool.map(uno, ids))
        cuerpos = set()
        for rid, resp in zip(ids, respuestas, strict=True):
            res = Resultado(rid, caso.ticket, f"{caso.nombre} ({rid[-2:]})", "GET", url,
                            {"X-Request-ID": rid}, "", str(espera[0]))
            if resp is None:
                res.estado, res.motivos = FALLA, ["sin respuesta"]
            else:
                self.respuestas[rid] = resp
                cuerpos.add(sin_request_id(resp.texto))
                res.obtenido, res.ms, res.extracto = f"{resp.estado} {resp.tipo}", round(resp.ms, 1), \
                    una_linea(resp.texto, 300)
                fallas, avisos = self.evaluar(caso, resp, ctx, espera, rid, servicio)
                res.estado, res.motivos = (FALLA if fallas else AVISO if avisos else OK), fallas + avisos
            print(f"  RECIBE  [{rid}] {res.obtenido or 'sin respuesta'} · {res.ms:.0f} ms  [{res.estado}]"
                  + (" " + "; ".join(res.motivos) if res.motivos else ""))
            self.resultados.append(res)
        self.verificar(caso.ticket, f"{caso.slug}-iguales", f"{caso.nombre}: todas las respuestas iguales",
                       None if len(cuerpos) == 1 else f"{len(cuerpos)} respuestas distintas")
        time.sleep(PAUSA)

    def omitir(self, rid: str, caso: Caso, servicio: str, ruta: str, espera: str, motivo: str) -> None:
        """Registra un caso que no se pudo correr y por qué. Devuelve None."""
        print(f"\n━━ [{rid}] TAL-{caso.ticket} · {caso.nombre}\n  OMITIDA: {motivo}", flush=True)
        self.resultados.append(Resultado(rid, caso.ticket, caso.nombre, caso.metodo, URLS[servicio] + ruta, {}, "",
                                         espera, OMITIDA, [motivo]))
        return None

    def cerrar(self, res: Resultado) -> None:
        """Muestra el veredicto de una llamada, la guarda y hace la pausa entre llamadas."""
        print(f"  [{res.estado}]" + (" " + "; ".join(res.motivos) if res.motivos else ""), flush=True)
        self.resultados.append(res)
        time.sleep(PAUSA)

    def verificar(self, ticket: str, slug: str, nombre: str, problema: str | None, aviso: bool = False) -> None:
        """Registra una verificación que cruza varias llamadas (sin pedido propio).

        `problema` es None si está bien; si no, el motivo. Con aviso=True un
        problema cuenta como AVISO en vez de FALLA."""
        rid = self.nuevo_id(ticket, slug)
        estado = OK if problema is None else AVISO if aviso else FALLA
        print(f"\n━━ [{rid}] TAL-{ticket} · {nombre}\n  [{estado}]" + (f" {problema}" if problema else ""), flush=True)
        self.resultados.append(Resultado(rid, ticket, nombre, "—", "(verificación cruzada)", {}, "", "coincide",
                                         estado, [problema] if problema else []))

    def evaluar(self, caso: Caso, resp: Respuesta, ctx: dict, espera: tuple[int, ...], rid: str,
                servicio: str) -> tuple[list[str], list[str]]:
        """Compara la respuesta con lo esperado.

        Devuelve (fallas, avisos): listas de motivos; vacías si todo coincide."""
        fallas: list[str] = []
        avisos: list[str] = []
        datos = resp.json()
        if resp.estado not in espera:
            fallas.append(f"código {resp.estado}, quiero {' o '.join(map(str, espera))}")
        if caso.forma == "lista" and not isinstance(datos, list):
            fallas.append("el body no es un arreglo JSON")
        elif caso.forma == "json" and resp.estado < 300 and datos is None:
            fallas.append("el body no es JSON")
        elif caso.forma == "problem" and resp.tipo != "application/problem+json":
            fallas.append(f"Content-Type {resp.tipo or 'vacío'}, quiero application/problem+json")
        elif caso.forma == "html" and "text/html" not in resp.tipo:
            fallas.append(f"Content-Type {resp.tipo or 'vacío'}, quiero text/html")
        if caso.codigo and (not isinstance(datos, dict) or datos.get("code") != caso.codigo):
            fallas.append(f"code {datos.get('code') if isinstance(datos, dict) else '(sin code)'}, "
                          f"quiero {caso.codigo}")
        if caso.total and not resp.headers.get("x-total-count", "").isdigit():
            fallas.append("falta X-Total-Count numérico")
        if caso.contiene and caso.contiene.format(**ctx) not in resp.texto:
            fallas.append(f"la respuesta no contiene {caso.contiene.format(**ctx)!r}")
        if caso.igual_a:
            otra = self.respuestas.get(self.buscar_id(caso.igual_a))
            if otra is None:
                fallas.append(f"no está la respuesta de {caso.igual_a} para comparar")
            elif (otra.estado, otra.tipo, sin_request_id(otra.texto)) != \
                    (resp.estado, resp.tipo, sin_request_id(resp.texto)):
                fallas.append(f"distinta de {caso.igual_a} ({otra.estado} {otra.tipo})")
        if caso.comprueba:
            problema = caso.comprueba(resp, ctx)
            if problema:
                fallas.append(problema)
        if caso.salidas:
            f, a = self.salidas(resp, datos, rid, servicio)
            fallas += f
            avisos += a
        return fallas, avisos

    def salidas(self, resp: Respuesta, datos, rid: str, servicio: str) -> tuple[list[str], list[str]]:
        """Comprobaciones de salida que valen para toda respuesta de la API.

        Devuelve (fallas, avisos)."""
        fallas: list[str] = []
        avisos: list[str] = []
        es_api = servicio in GO or (servicio == "frontend" and resp.tipo in ("application/json",
                                                                              "application/problem+json"))
        if es_api and resp.headers.get("x-request-id") != rid:
            fallas.append(f"X-Request-ID de vuelta {resp.headers.get('x-request-id', '(no vino)')!r}, quiero {rid!r}")
        if resp.tipo == "application/problem+json":
            faltan = [k for k in ("type", "title", "status", "detail", "code", "request_id")
                      if not isinstance(datos, dict) or k not in datos]
            if faltan:
                fallas.append(f"problem+json sin {', '.join(faltan)}")
            elif datos["status"] != resp.estado or datos["request_id"] != rid:
                fallas.append("status o request_id del problem+json no coinciden con la respuesta")
        if resp.estado >= 400 and DETALLES_INTERNOS.search(resp.texto):
            fallas.append("la respuesta deja ver detalles internos")
        if resp.tipo == "application/json" and resp.estado < 300:
            objeto = datos[0] if isinstance(datos, list) and datos else datos
            malas = [k for k in objeto if not CAMEL.match(k)] if isinstance(objeto, dict) else []
            if malas:
                fallas.append(f"claves que no son camelCase: {', '.join(malas[:5])}")
        if MOJIBAKE.search(resp.texto):
            avisos.append("texto mal codificado en la respuesta (p. ej. «"
                          + MOJIBAKE.search(resp.texto).group(0) + "»)")
        return fallas, avisos

    def buscar_id(self, prefijo: str) -> str:
        """Devuelve el id completo de la última llamada cuyo slug termina en `prefijo`."""
        for res in reversed(self.resultados):
            if res.id.endswith(prefijo):
                return res.id
        return ""


def preparar(visor: Visor) -> dict:
    """Lee ids reales para armar las rutas: dos productos, una oferta, una tienda y un servicio.

    Devuelve el contexto (dict) que usan los casos; falta la clave si no se pudo leer."""
    ctx: dict = {}
    r = visor.llamar(Caso("165", "prep-productos", "Preparación: dos productos reales", "comercial",
                          "/products?limit=2", forma="lista"), ctx)
    productos = r.json() if r else None
    if isinstance(productos, list) and productos:
        ctx["pid"] = productos[0]["id"]
        ctx["pid2"] = productos[-1]["id"]
        ctx["pnombre"] = productos[0].get("name", "")
    r = visor.llamar(Caso("165", "prep-ofertas", "Preparación: ofertas del primer producto", "comercial",
                          "/offers/product/{pid}", forma="lista"), ctx)
    ofertas = r.json() if r else None
    if isinstance(ofertas, list) and ofertas:
        ctx["oid"] = ofertas[0]["id"]
    r = visor.llamar(Caso("165", "prep-tiendas", "Preparación: una tienda", "comercial", "/stores?limit=1",
                          forma="lista", total=True), ctx)
    tiendas = r.json() if r else None
    if isinstance(tiendas, list) and tiendas:
        ctx["sid"] = tiendas[0]["id"]
    r = visor.llamar(Caso("165", "prep-servicios", "Preparación: un servicio", "comercial", "/services",
                          forma="lista"), ctx)
    servicios = r.json() if r else None
    if isinstance(servicios, list) and servicios:
        ctx["svc"] = servicios[0]["id"]
    return ctx


def detectar_topologia() -> str:
    """Pregunta al visor por /health de comercial: si responde, la topología es dominios."""
    try:
        pedido = urllib.request.Request(f"{PROXY}/comercial/health", headers={"X-Request-ID": "visor-topologia"})
        with urllib.request.urlopen(pedido, timeout=10) as resp:
            return "dominios" if resp.status == 200 else "consolidado"
    except (urllib.error.URLError, OSError):
        return "consolidado"


def esperar_visor() -> None:
    """Espera a que el sidecar `destino` acepte conexiones (hasta 60 s)."""
    for _ in range(60):
        try:
            urllib.request.urlopen(f"{PROXY}/__listo", timeout=2).read()
            return
        except (urllib.error.URLError, OSError):
            time.sleep(1)
    print("AVISO: el visor no respondió en 60 s; se sigue igual", flush=True)


def publicar_registro(visor: Visor, topologia: str) -> None:
    """Escribe al final del log el registro de las dos ventanas en JSON, en líneas @@@REGISTRO.

    Lo lee scripts/testing/llamadas.py para armar el reporte lado a lado."""
    try:
        with urllib.request.urlopen(f"{PROXY}/__registro", timeout=10) as resp:
            destino = json.loads(resp.read())
    except (urllib.error.URLError, OSError, ValueError):
        destino = []
    texto = json.dumps({"topologia": topologia, "origen": [r.__dict__ for r in visor.resultados],
                        "destino": destino}, ensure_ascii=False)
    trozos = [texto[i:i + TROZO_REGISTRO] for i in range(0, len(texto), TROZO_REGISTRO)]
    for n, trozo in enumerate(trozos, 1):
        print(f"@@@REGISTRO {n}/{len(trozos)} {trozo}")


def main() -> int:
    """Corre la preparación y los tickets pedidos en TICKETS; devuelve 1 si hubo fallas."""
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8")
    tickets = [t.strip() for t in os.environ.get("TICKETS", ",".join(ESCENARIOS)).split(",") if t.strip()]
    esperar_visor()
    topologia = detectar_topologia()
    print(f"Visor de llamadas, ventana ORIGEN. Topología: {topologia}. Tickets: {', '.join(tickets)}. "
          f"Token de servicio: {'sí' if TOKEN else 'no'}. Pausa: {PAUSA} s", flush=True)
    visor = Visor(topologia)
    ctx = preparar(visor)
    for ticket in tickets:
        print(f"\n══════════ TAL-{ticket} ══════════", flush=True)
        if ticket not in ESCENARIOS:
            print(f"Ticket desconocido: {ticket}")
            continue
        # Los escenarios van en archivos aparte del ConfigMap; se cargan al usarlos.
        modulo, funcion = ESCENARIOS[ticket]
        getattr(importlib.import_module(modulo), funcion)(visor, ctx)

    print("\n══════════ Resumen ══════════")
    for ticket in ["165"] + [t for t in tickets if t != "165"]:
        filas = [r for r in visor.resultados if r.ticket == ticket]
        cuenta = {e: sum(r.estado == e for r in filas) for e in (OK, FALLA, AVISO, OMITIDA)}
        print(f"TAL-{ticket}: {cuenta[OK]} OK, {cuenta[FALLA]} fallas, {cuenta[AVISO]} avisos, "
              f"{cuenta[OMITIDA]} omitidas")
        for r in filas:
            if r.estado in (FALLA, AVISO):
                print(f"  [{r.estado}] {r.id}: {'; '.join(r.motivos)}")
    total = {e: sum(r.estado == e for r in visor.resultados) for e in (OK, FALLA, AVISO, OMITIDA)}
    print(f"Resultado: {total[OK]} OK, {total[FALLA]} fallas, {total[AVISO]} avisos, {total[OMITIDA]} omitidas",
          flush=True)
    publicar_registro(visor, topologia)
    return 1 if total[FALLA] else 0


if __name__ == "__main__":
    sys.exit(main())
