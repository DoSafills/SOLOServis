#!/usr/bin/env python3
"""Verifica el contrato HTTP del sistema completo a través del gateway.

Comprueba, en una sola pasada y sin escribir datos, lo que el navegador ve:

  - estructura: cada prefijo público responde con JSON, no con la SPA;
  - códigos: 200, 400, 401, 404, 405 y 422 donde corresponde;
  - forma de las respuestas: los errores son problem+json de TAL-78 (con
    code y request_id), los listados traen X-Total-Count y los bodies tienen
    las claves que el frontend lee.

Separa los hallazgos en dos niveles:

  - ``contrato``: debe cumplirse; si falla, el script sale con código 1.
  - ``aviso``: desviación conocida, registrada en docs/reportes/
    errores-pendientes.md; se informa pero no rompe la corrida.

Uso (con el Compose local levantado, que publica el gateway en 8443):

    python scripts/testing/contratos.py
    python scripts/testing/contratos.py --detalle
    python scripts/testing/contratos.py --md docs/reportes/contratos.md
    python scripts/testing/contratos.py --base-url http://localhost:8443

Solo usa la librería estándar de Python 3.9+.
"""

from __future__ import annotations

import argparse
import json
import sys
import urllib.error
import urllib.request
from dataclasses import dataclass, field
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
ICON = {"OK": "✅", "FALLA": "❌", "AVISO": "⚠️"}
PROBLEM = "application/problem+json"
PROBLEM_KEYS = ("type", "title", "status", "detail", "instance", "code", "request_id")


@dataclass
class Check:
    """Un pedido con lo que se espera de su respuesta.

    ``estados`` son los códigos aceptados; ``contenido`` el prefijo del
    Content-Type; ``code`` el code del problem+json; ``claves`` las claves
    obligatorias del JSON; ``cabeceras`` las cabeceras que deben venir.
    ``severidad`` es "contrato" (falla la corrida) o "aviso" (solo informa).
    """

    nombre: str
    metodo: str
    ruta: str
    estados: tuple[int, ...]
    contenido: str | None = None
    code: str | None = None
    claves: tuple[str, ...] = ()
    cabeceras: tuple[str, ...] = ()
    cuerpo: str | None = None
    severidad: str = "contrato"
    nota: str = ""


@dataclass
class Result:
    """Resultado de un Check: estado, problemas encontrados y respuesta cruda."""

    check: Check
    estado: int | None = None
    contenido: str = ""
    problemas: list[str] = field(default_factory=list)

    @property
    def veredicto(self) -> str:
        """Devuelve OK, FALLA o AVISO según la severidad y los problemas."""
        if not self.problemas:
            return "OK"
        return "FALLA" if self.check.severidad == "contrato" else "AVISO"


def checks() -> list[Check]:
    """Devuelve la lista de comprobaciones del contrato público."""
    uuid_ausente = "00000000-0000-0000-0000-000000000000"
    return [
        # --- Gateway y SPA -------------------------------------------------
        Check("El gateway responde su probe", "GET", "/healthz", (200,),
              "text/plain", cuerpo="ok"),
        Check("La SPA se sirve en la raíz", "GET", "/", (200,), "text/html"),
        Check("Una ruta del frontend devuelve la SPA", "GET", "/catalogo", (200,), "text/html"),

        # --- Catálogo (público, solo lectura) ------------------------------
        Check("GET /products devuelve un arreglo", "GET", "/products?limit=2", (200,),
              "application/json", claves=("id", "name")),
        Check("GET /products trae X-Total-Count", "GET", "/products?limit=2", (200,),
              "application/json", cabeceras=("X-Total-Count",)),
        Check("GET /products con id no UUID es 400", "GET", "/products/no-es-uuid", (400,),
              PROBLEM, code="invalid_parameter"),
        Check("GET /products con UUID ausente es 404", "GET", f"/products/{uuid_ausente}", (404,),
              PROBLEM, code="not_found"),
        Check("GET /products con limit inválido es 400", "GET", "/products?limit=0", (400,),
              PROBLEM, code="invalid_parameter"),
        Check("GET /product-categories responde", "GET", "/product-categories", (200,),
              "application/json"),
        Check("GET /categories devuelve items y total", "GET", "/categories", (200,),
              "application/json", claves=("items", "total", "limit", "offset")),
        Check("GET /categories con slug inválido es 400", "GET", "/categories/NO_VALIDO", (400,),
              PROBLEM, code="invalid_parameter"),
        Check("GET /stores responde", "GET", "/stores", (200,), "application/json"),
        Check("GET /services responde", "GET", "/services", (200,), "application/json"),

        # --- Búsqueda ------------------------------------------------------
        Check("GET /search/products responde", "GET", "/search/products?q=tv&limit=2", (200,),
              "application/json"),
        Check("DELETE /search/products es 405 con Allow", "DELETE", "/search/products", (405,),
              PROBLEM, code="method_not_allowed", cabeceras=("Allow",)),

        # --- Sesión --------------------------------------------------------
        Check("GET /auth/validate sin token es 401", "GET", "/auth/validate", (401,),
              PROBLEM, code="unauthorized"),
        Check("POST /auth/login con JSON malformado es 400", "POST", "/auth/login", (400,),
              PROBLEM, code="malformed_json", cuerpo="{",
              cabeceras=("Content-Type",)),
        Check("POST /auth/login con campo desconocido es 400", "POST", "/auth/login", (400,),
              PROBLEM, code="unknown_field", cuerpo='{"email":"a@b.cl","password":"x","extra":1}'),
        Check("POST /auth/login sin datos es 422", "POST", "/auth/login", (422,),
              PROBLEM, code="validation_failed", cuerpo="{}"),

        # --- APIs con sesión: sin token deben ser 401 ----------------------
        Check("GET /cart sin sesión es 401", "GET", "/cart", (401,), PROBLEM, code="unauthorized"),
        Check("GET /favorites sin sesión es 401", "GET", "/favorites", (401,), PROBLEM,
              code="unauthorized"),
        Check("GET /watchlist sin sesión es 401", "GET", "/watchlist", (401,), PROBLEM,
              code="unauthorized"),
        Check("GET /reviews/product/1 es público", "GET", "/reviews/product/1", (200,),
              "application/json"),
        Check("OPTIONS /cart/items responde 204", "OPTIONS", "/cart/items", (204,)),

        # --- Desviaciones conocidas (docs/reportes/errores-pendientes.md) --
        Check("POST /products sin token debería ser problem+json", "POST", "/products", (401, 403),
              PROBLEM, severidad="aviso",
              nota="Hoy nginx responde 403 text/html: el gateway bloquea todo lo que no sea GET."),
        Check("GET /health por el gateway debería ser 404 problem+json", "GET", "/health", (404,),
              PROBLEM, severidad="aviso",
              nota="Hoy cae en la SPA y responde index.html con 200."),
        Check("GET /offers no se publica (404 problem+json)", "GET", "/offers/product/1", (404,),
              PROBLEM, severidad="aviso",
              nota="Pricing no se publica: hoy el gateway devuelve la SPA con 200."),
        Check("GET /users no se publica (404 problem+json)", "GET", "/users", (404,),
              PROBLEM, severidad="aviso",
              nota="User API no se publica: hoy el gateway devuelve la SPA con 200."),
        Check("GET /ingestion/runs no se publica (404 problem+json)", "GET", "/ingestion/runs",
              (404,), PROBLEM, severidad="aviso",
              nota="Ingestion no se publica: hoy el gateway devuelve la SPA con 200."),
        Check("GET /notifications no se publica (404 problem+json)", "GET", "/notifications",
              (404,), PROBLEM, severidad="aviso",
              nota="Notification no se publica y el frontend no la usa: hoy devuelve la SPA con 200."),
    ]


def pedir(base: str, check: Check, timeout: float) -> tuple[int, dict[str, str], bytes]:
    """Hace el pedido del check y devuelve (status, cabeceras, cuerpo).

    Lanza urllib.error.URLError solo si no hay conexión con el gateway; los
    códigos 4xx y 5xx llegan como respuesta normal porque son lo que se está
    comprobando.
    """
    url = base.rstrip("/") + check.ruta
    data = check.cuerpo.encode("utf-8") if check.cuerpo is not None else None
    request = urllib.request.Request(url, data=data, method=check.metodo)
    if data is not None:
        request.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            return response.status, dict(response.headers), response.read()
    except urllib.error.HTTPError as error:
        return error.code, dict(error.headers), error.read()


def revisar(check: Check, estado: int, cabeceras: dict[str, str], cuerpo: bytes) -> list[str]:
    """Compara la respuesta con lo esperado y devuelve la lista de problemas."""
    problemas: list[str] = []
    if estado not in check.estados:
        problemas.append(f"status {estado}, quiero {'/'.join(map(str, check.estados))}")

    contenido = cabeceras.get("Content-Type", "")
    if check.contenido and not contenido.startswith(check.contenido):
        problemas.append(f"Content-Type {contenido or '(vacío)'}, quiero {check.contenido}")

    for cabecera in check.cabeceras:
        if not cabeceras.get(cabecera):
            problemas.append(f"falta la cabecera {cabecera}")

    texto = cuerpo.decode("utf-8", errors="replace")
    if check.code or (check.contenido == PROBLEM and texto.strip()):
        problemas += revisar_problem(check, estado, texto)
    elif check.claves:
        problemas += revisar_json(check, texto)
    return problemas


def revisar_problem(check: Check, estado: int, texto: str) -> list[str]:
    """Valida la forma del problem+json: claves, status y code esperados."""
    try:
        problem = json.loads(texto)
    except json.JSONDecodeError:
        return [f"el cuerpo no es JSON: {texto[:80]!r}"]
    if not isinstance(problem, dict):
        return ["el problem+json no es un objeto"]
    problemas = [f"falta {clave}" for clave in PROBLEM_KEYS if clave not in problem]
    if problem.get("status") != estado:
        problemas.append(f"problem.status {problem.get('status')}, quiero {estado}")
    if check.code and problem.get("code") != check.code:
        problemas.append(f"code {problem.get('code')!r}, quiero {check.code!r}")
    if not str(problem.get("request_id", "")).strip():
        problemas.append("request_id vacío")
    return problemas


def revisar_json(check: Check, texto: str) -> list[str]:
    """Valida que el body sea JSON con las claves esperadas.

    Si el body es un arreglo, las claves se buscan en el primer elemento
    (un arreglo vacío cumple: no hay nada que leer).
    """
    try:
        body = json.loads(texto)
    except json.JSONDecodeError:
        return [f"el cuerpo no es JSON: {texto[:80]!r}"]
    objetivo = body[0] if isinstance(body, list) and body else body
    if not isinstance(objetivo, dict):
        return []
    return [f"falta la clave {clave}" for clave in check.claves if clave not in objetivo]


def ejecutar(base: str, timeout: float) -> list[Result]:
    """Corre todos los checks contra base y devuelve sus resultados."""
    resultados: list[Result] = []
    for check in checks():
        result = Result(check)
        try:
            estado, cabeceras, cuerpo = pedir(base, check, timeout)
        except (urllib.error.URLError, OSError) as error:
            result.problemas.append(f"no se pudo conectar: {error}")
            resultados.append(result)
            continue
        result.estado = estado
        result.contenido = cabeceras.get("Content-Type", "")
        result.problemas = revisar(check, estado, cabeceras, cuerpo)
        resultados.append(result)
    return resultados


def render(resultados: list[Result]) -> str:
    """Devuelve la tabla de resultados en texto plano para la consola."""
    lineas = []
    for result in resultados:
        check = result.check
        detalle = f"{check.metodo} {check.ruta}"
        marca = ICON[result.veredicto]
        linea = f"{marca} {result.estado or '---':>3}  {detalle:<48} {check.nombre}"
        if result.problemas:
            linea += "  → " + "; ".join(result.problemas)
        lineas.append(linea)
    return "\n".join(lineas)


def render_md(resultados: list[Result], base: str) -> str:
    """Devuelve el reporte en Markdown, listo para guardar como evidencia."""
    total = {clave: sum(1 for r in resultados if r.veredicto == clave) for clave in ICON}
    lineas = [
        "# Contratos HTTP del gateway",
        "",
        f"- Gateway: `{base}`",
        f"- Resultado: {total['OK']} ok, {total['FALLA']} fallidas, {total['AVISO']} avisos",
        "",
        "| | Status | Pedido | Comprobación | Detalle |",
        "| --- | --- | --- | --- | --- |",
    ]
    for result in resultados:
        check = result.check
        detalle = "; ".join(result.problemas) or check.nota or ""
        lineas.append(
            f"| {ICON[result.veredicto]} | {result.estado or '---'} | "
            f"`{check.metodo} {check.ruta}` | {check.nombre} | {detalle} |"
        )
    lineas.append("")
    return "\n".join(lineas)


def main() -> int:
    """Punto de entrada: corre los checks, imprime el resultado y sale 0/1."""
    parser = argparse.ArgumentParser(description="Verifica el contrato HTTP del gateway.")
    parser.add_argument("--base-url", default="http://localhost:8443",
                        help="URL del gateway (por defecto http://localhost:8443)")
    parser.add_argument("--timeout", type=float, default=10.0, help="segundos por pedido")
    parser.add_argument("--detalle", action="store_true", help="muestra la nota de cada check")
    parser.add_argument("--md", help="guarda el reporte Markdown en esta ruta")
    args = parser.parse_args()

    try:
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    except AttributeError:  # pragma: no cover - consolas sin reconfigure
        pass

    resultados = ejecutar(args.base_url, args.timeout)
    print(render(resultados))
    if args.detalle:
        print()
        for result in resultados:
            if result.check.nota:
                print(f"· {result.check.metodo} {result.check.ruta}: {result.check.nota}")

    total = {clave: sum(1 for r in resultados if r.veredicto == clave) for clave in ICON}
    print(f"\n{total['OK']} ok, {total['FALLA']} fallidas, {total['AVISO']} avisos")

    if args.md:
        destino = Path(args.md)
        destino.parent.mkdir(parents=True, exist_ok=True)
        destino.write_text(render_md(resultados, args.base_url), encoding="utf-8")
        print(f"Reporte: {destino}")

    if all(result.estado is None for result in resultados):
        print("No hay gateway en esa URL: levántalo con docker compose -f docker/compose.yml up -d")
        return 2
    return 1 if total["FALLA"] else 0


if __name__ == "__main__":
    sys.exit(main())
