"""Ventana "destino" del visor de llamadas (TAL-165).

Proxy HTTP que corre como sidecar en el pod de pruebas. El contenedor
`origen` le habla a http://127.0.0.1:9000/<servicio>/<ruta> y este proceso:

  1. registra lo que le llegó: método, ruta, headers y body;
  2. lo reenvía sin cambios al Service real de ese servicio;
  3. registra lo que respondió el servicio y se lo devuelve a `origen`.

Así `kubectl logs -f ... -c destino` muestra, llamada por llamada, lo que
recibe el destino, y `-c origen` lo que se envió. Las dos ventanas se
correlacionan por el X-Request-ID, que va al principio de cada bloque.

Solo reenvía a los Services de la lista SERVICIOS (no es un proxy abierto) y
nunca escribe el valor del header Authorization: muestra que viene y su largo.
Solo la librería estándar de Python.
"""

from __future__ import annotations

import json
import os
import sys
import threading
import time
import urllib.error
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

PUERTO = int(os.environ.get("PUERTO_VISOR", "9000"))
EXTRACTO = int(os.environ.get("EXTRACTO", "600"))  # caracteres de body por bloque en la ventana
EXTRACTO_REGISTRO = 2000  # caracteres de body por llamada en /__registro

# Nombre lógico -> URL base del Service. "backend" es la topología consolidada.
SERVICIOS = {
    "frontend": "http://frontend:80",
    "comercial": "http://comercial:8080",
    "usuarios": "http://usuarios:8080",
    "ingestion": "http://ingestion:8080",
    "backend": "http://backend:8080",
    "scraper": "http://scraper-api:8095",
}
# Para probar fuera del clúster (con kubectl port-forward): {"comercial": "http://127.0.0.1:18080", ...}
SERVICIOS.update(json.loads(os.environ.get("SERVICIOS_JSON", "{}")))

# Headers que no se reenvían: los de conexión los maneja cada salto.
SALTO = {"connection", "keep-alive", "proxy-connection", "transfer-encoding", "upgrade", "te", "trailer", "host",
         "content-length"}
# Headers que se muestran en la ventana (el resto se cuenta, no se lista).
VISIBLES = ("x-request-id", "content-type", "content-length", "authorization", "user-agent", "accept")

REGISTRO: list[dict] = []
CANDADO = threading.Lock()  # print y REGISTRO se comparten entre hilos


class SinRedirecciones(urllib.request.HTTPRedirectHandler):
    """Hace que urllib devuelva los 3xx tal cual en vez de seguirlos."""

    def redirect_request(self, req, fp, code, msg, headers, newurl):  # noqa: D102
        return None


ABRIDOR = urllib.request.build_opener(SinRedirecciones)


def ocultar(nombre: str, valor: str) -> str:
    """Devuelve el valor de un header listo para mostrar.

    Recibe el nombre y el valor del header. Para Authorization devuelve solo el
    esquema y el largo del token (nunca el token); para el resto, el valor."""
    if nombre.lower() != "authorization":
        return valor
    esquema, _, token = valor.partition(" ")
    return f"{esquema} **** ({len(token)} caracteres)" if token else "****"


def extracto(datos: bytes, largo: int) -> str:
    """Convierte un body en texto de una línea, cortado a `largo` caracteres.

    Recibe los bytes del body; devuelve "(vacío)" si no hay body."""
    if not datos:
        return "(vacío)"
    texto = " ".join(datos.decode("utf-8", "replace").split())
    return texto[:largo] + (f"… (+{len(texto) - largo} caracteres)" if len(texto) > largo else "")


def bloque(lineas: list[str]) -> None:
    """Escribe un bloque de líneas en la ventana sin que se mezcle con otro hilo."""
    with CANDADO:
        print("\n".join(lineas), flush=True)


class Visor(BaseHTTPRequestHandler):
    """Atiende cada pedido de `origen`: lo registra, lo reenvía y registra la respuesta."""

    protocol_version = "HTTP/1.1"
    server_version = "visor-destino"

    def log_message(self, formato, *args):  # noqa: D102 - el registro propio reemplaza al de http.server
        return

    def do_GET(self):  # noqa: N802, D102
        self.reenviar()

    do_POST = do_PUT = do_DELETE = do_PATCH = do_HEAD = do_GET  # noqa: N815

    def responder(self, estado: int, cuerpo: bytes, tipo: str = "application/json") -> None:
        """Responde directamente a `origen` (salud del visor, registro o error propio)."""
        self.send_response(estado)
        self.send_header("Content-Type", tipo)
        self.send_header("Content-Length", str(len(cuerpo)))
        self.end_headers()
        if self.command != "HEAD":
            self.wfile.write(cuerpo)

    def reenviar(self) -> None:
        """Registra el pedido, lo envía al Service real y devuelve su respuesta.

        La ruta llega como /<servicio>/<ruta original>. Si el servicio no está
        en SERVICIOS responde 502 sin reenviar nada."""
        if self.path == "/__listo":  # startupProbe del sidecar
            return self.responder(200, b"ok\n", "text/plain")
        if self.path == "/__registro":  # lo pide origen al final para el reporte
            with CANDADO:
                cuerpo = json.dumps(REGISTRO, ensure_ascii=False).encode("utf-8")
            return self.responder(200, cuerpo)

        _, servicio, resto = self.path.split("/", 2) if self.path.count("/") >= 2 else ("", self.path.strip("/"), "")
        ruta = "/" + resto
        largo = int(self.headers.get("Content-Length") or 0)
        body = self.rfile.read(largo) if largo else b""
        rid = self.headers.get("X-Request-ID", "(sin X-Request-ID)")
        headers = {k: v for k, v in self.headers.items()}
        base = SERVICIOS.get(servicio)
        lineas = [f"━━ [{rid}] llegó para {servicio}",
                  f"  RECIBIÓ   {self.command} {ruta}",
                  "            " + " · ".join(f"{k}: {ocultar(k, v)}" for k, v in headers.items()
                                            if k.lower() in VISIBLES),
                  f"            body ({len(body)} bytes): {extracto(body, EXTRACTO)}"]
        fila = {"id": rid, "servicio": servicio, "metodo": self.command, "ruta": ruta,
                "headers": {k: ocultar(k, v) for k, v in headers.items()},
                "bytes": len(body), "body": extracto(body, EXTRACTO_REGISTRO)}
        if base is None:
            lineas.append(f"  NO REENVÍA: «{servicio}» no es un servicio conocido")
            bloque(lineas)
            return self.responder(502, json.dumps({"error": f"servicio desconocido: {servicio}"}).encode())

        pedido = urllib.request.Request(base + ruta, data=body if body or self.command in ("POST", "PUT", "PATCH")
                                        else None, method=self.command)
        for k, v in headers.items():
            if k.lower() not in SALTO:
                pedido.add_header(k, v)
        inicio = time.monotonic()
        try:
            with ABRIDOR.open(pedido, timeout=30) as resp:
                estado, resp_headers, datos = resp.status, resp.headers, resp.read()
        except urllib.error.HTTPError as err:  # 4xx y 5xx también son respuestas válidas
            estado, resp_headers, datos = err.code, err.headers, err.read()
        except (urllib.error.URLError, OSError) as err:
            ms = (time.monotonic() - inicio) * 1000
            motivo = str(getattr(err, "reason", err))
            lineas.append(f"  SIN RESPUESTA de {base} tras {ms:.0f} ms: {motivo}")
            bloque(lineas)
            fila.update({"estado": 0, "ms": round(ms, 1), "error": motivo})
            with CANDADO:
                REGISTRO.append(fila)
            return self.responder(502, json.dumps({"error": f"sin respuesta de {servicio}: {motivo}"}).encode())
        ms = (time.monotonic() - inicio) * 1000
        tipo = resp_headers.get("Content-Type", "")
        lineas.append(f"  RESPONDIÓ {servicio} {estado} {tipo or '(sin Content-Type)'} · {ms:.0f} ms · "
                      f"X-Request-ID: {resp_headers.get('X-Request-ID', '(no vino)')}")
        bloque(lineas)
        fila.update({"estado": estado, "tipo": tipo, "ms": round(ms, 1),
                     "rid_respuesta": resp_headers.get("X-Request-ID", "")})
        with CANDADO:
            REGISTRO.append(fila)

        self.send_response(estado)
        for k, v in resp_headers.items():
            if k.lower() not in SALTO:
                self.send_header(k, v)
        self.send_header("Content-Length", str(len(datos)))
        self.end_headers()
        if self.command != "HEAD":
            self.wfile.write(datos)


def main() -> None:
    """Levanta el proxy en PUERTO_VISOR y atiende hasta que Kubernetes lo detiene."""
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8")
    print(f"Visor de llamadas, ventana DESTINO: escuchando en :{PUERTO}. Cada bloque es lo que llegó "
          "a un servicio y lo que respondió.", flush=True)
    ThreadingHTTPServer(("0.0.0.0", PUERTO), Visor).serve_forever()


if __name__ == "__main__":
    main()
