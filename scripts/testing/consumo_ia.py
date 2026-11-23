#!/usr/bin/env python3
"""Reporte de consumo del asistente de IA (TAL-225).

Lee `ai_comparison_query`, la tabla de auditoría que llena el asistente en cada
consulta (migración 029), y responde las preguntas que importan para cuidar la
cuota del proveedor:

  - cuántas consultas hubo y cuántas se resolvieron con la caché;
  - cuántos tokens de entrada y salida se gastaron;
  - qué latencia se pagó y cuál fue el p95;
  - quién consumió (usuario o anónimo), por proveedor y modelo;
  - qué errores se repitieron y qué preguntas vuelven.

Es de solo lectura: abre una sesión READ ONLY y no escribe nada. Solo usa la
librería estándar de Python 3.9+, igual que el resto de `scripts/testing/`.

    python scripts/testing/consumo_ia.py                  # PostgreSQL de docker compose
    python scripts/testing/consumo_ia.py --cluster        # PostgreSQL del clúster (kubectl)
    python scripts/testing/consumo_ia.py --dias 30        # ventana de 30 días
    python scripts/testing/consumo_ia.py --json           # salida para procesar

Sale con código 1 si no puede consultar la base o la migración 029 no está
aplicada.
"""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
from pathlib import Path

# Raíz del repo: scripts/testing/consumo_ia.py -> tres niveles arriba.
REPO_ROOT = Path(__file__).resolve().parents[2]
SQL_FILE = REPO_ROOT / "docs" / "sql" / "consumo_ia.sql"

# Comando psql dentro del contenedor o del pod. Usa las variables de entorno que
# PostgreSQL ya tiene, así no se repiten credenciales en el script.
PSQL = 'psql -X -q -At -v ON_ERROR_STOP=1 -v dias={dias} -U "$POSTGRES_USER" -d "$POSTGRES_DB"'

# Orden en que se muestran los bloques del reporte.
SECTIONS = ["resumen", "por_dia", "por_usuario", "por_modelo", "errores", "repetidas"]


def load_queries(path: Path) -> dict[str, str]:
    """Separa docs/sql/consumo_ia.sql en bloques por su marcador `-- name: x`.

    Recibe la ruta del archivo y devuelve un diccionario nombre -> SQL. Ignora
    comentarios sueltos y líneas vacías de cada bloque, para que el texto que se
    envía a psql sea solo la consulta.
    """
    queries: dict[str, str] = {}
    current: str | None = None
    lines: list[str] = []

    def flush() -> None:
        """Cierra el bloque en curso: guarda su SQL si tenía contenido.

        Quita el punto y coma final porque la consulta se envuelve en un
        `SELECT ... FROM ( ... ) t` y ahí un `;` rompe la sintaxis.
        """
        nonlocal lines
        if current and lines:
            queries[current] = "\n".join(lines).strip().rstrip(";").rstrip()
        lines = []

    for raw in path.read_text(encoding="utf-8").splitlines():
        stripped = raw.strip()
        if stripped.startswith("-- name:"):
            flush()
            current = stripped.split(":", 1)[1].strip()
            continue
        if current is None or stripped.startswith("--"):
            continue
        lines.append(raw)
    flush()
    return queries


def build_command(args: argparse.Namespace) -> list[str]:
    """Arma el comando que ejecuta psql, según el destino elegido.

    Recibe los argumentos ya parseados y devuelve la lista de argumentos para
    subprocess. `--cluster` usa el pod `postgres-0` del namespace actual o del
    indicado; sin él, el contenedor de docker compose.
    """
    shell = ["sh", "-c", PSQL.format(dias=args.dias)]
    if args.cluster:
        command = ["kubectl"]
        if args.namespace:
            command += ["-n", args.namespace]
        return command + ["exec", "-i", "postgres-0", "-c", "postgres", "--"] + shell
    # docker exec no acepta `--` antes del comando: lo toma como el ejecutable.
    return ["docker", "exec", "-i", args.docker] + shell


def run_query(sql: str, args: argparse.Namespace) -> list[dict]:
    """Ejecuta una consulta y devuelve sus filas como diccionarios.

    Envuelve el SQL en un `json_agg` para no tener que parsear texto con
    separadores, y abre la sesión en READ ONLY. Recibe la consulta y los
    argumentos; devuelve la lista de filas o lanza SystemExit con el error de
    psql ya interpretado.
    """
    script = (
        "SET SESSION CHARACTERISTICS AS TRANSACTION READ ONLY;\n"
        f"SELECT coalesce(json_agg(t), '[]'::json) FROM (\n{sql}\n) t;\n"
    )
    try:
        proc = subprocess.run(
            build_command(args),
            input=script,
            capture_output=True,
            text=True,
            timeout=60,
        )
    except FileNotFoundError:
        target = "kubectl" if args.cluster else "docker"
        sys.exit(f"ERROR: no se encontró {target} en el PATH.")
    except subprocess.TimeoutExpired:
        sys.exit("ERROR: la consulta tardó más de 60 segundos.")

    if proc.returncode != 0:
        stderr = (proc.stderr or "").strip()
        if 'relation "ai_comparison_query" does not exist' in stderr:
            sys.exit("ERROR: falta la tabla ai_comparison_query. Aplica la migración 029.")
        sys.exit(f"ERROR: psql falló:\n{stderr}")

    output = (proc.stdout or "").strip()
    if not output:
        return []
    return json.loads(output)


def as_number(value: object) -> int | float:
    """Convierte un valor de JSON en número, o 0 si viene nulo.

    Recibe cualquier valor y devuelve el número equivalente; sirve para sumar
    sin repetir la comprobación de None en cada cálculo.
    """
    if value is None:
        return 0
    return value


def format_int(value: object) -> str:
    """Formatea un entero con separador de miles para leerlo de un vistazo."""
    return f"{int(as_number(value)):,}".replace(",", ".")


def percentage(part: object, total: object) -> str:
    """Devuelve `part` sobre `total` como porcentaje con un decimal.

    Recibe dos números y devuelve el texto ya formateado; si el total es cero,
    devuelve un guion en vez de dividir por cero.
    """
    total_number = as_number(total)
    if not total_number:
        return "—"
    return f"{100 * as_number(part) / total_number:.1f}%"


def print_table(title: str, rows: list[dict]) -> None:
    """Imprime un bloque del reporte como tabla alineada.

    Recibe el título del bloque y sus filas; no devuelve nada. Las columnas se
    alinean a la izquierda con un ancho calculado sobre el contenido, y los
    valores numéricos se imprimen tal como vienen porque psql ya los redondea.
    """
    print()
    print(title)
    if not rows:
        print("  (sin datos en la ventana)")
        return

    headers = list(rows[0].keys())
    cells = [[str(row.get(header, "")) for header in headers] for row in rows]
    widths = [len(header) for header in headers]
    for row in cells:
        for index, cell in enumerate(row):
            widths[index] = max(widths[index], len(cell))

    header_line = "  ".join(header.ljust(widths[i]) for i, header in enumerate(headers))
    print("  " + header_line.rstrip())
    print("  " + "  ".join("-" * widths[i] for i in range(len(headers))))
    for row in cells:
        line = "  ".join(cell.ljust(widths[i]) for i, cell in enumerate(row))
        print("  " + line.rstrip())


def print_summary(summary: dict) -> None:
    """Imprime el bloque de totales con las proporciones ya calculadas.

    Recibe la única fila del bloque `resumen` y no devuelve nada. Separa el
    ahorro de la caché de lo que sí llegó al proveedor, que es la lectura que
    interesa cuando se mira la cuota.
    """
    total = as_number(summary.get("consultas"))
    cached = as_number(summary.get("desde_cache"))
    print()
    print("Resumen de la ventana")
    print(f"  consultas          {format_int(total)}")
    print(f"  al proveedor       {format_int(summary.get('al_proveedor'))}")
    print(f"  desde caché        {format_int(cached)}  ({percentage(cached, total)})")
    print(f"  errores            {format_int(summary.get('errores'))}")
    print(f"  tokens entrada     {format_int(summary.get('tokens_entrada'))}")
    print(f"  tokens salida      {format_int(summary.get('tokens_salida'))}")
    print(f"  latencia media     {summary.get('latencia_media_ms') or '—'} ms")
    print(f"  latencia p95       {summary.get('latencia_p95_ms') or '—'} ms")


def main() -> int:
    """Punto de entrada: parsea argumentos, ejecuta cada bloque y lo imprime.

    Devuelve 0 si todo salió bien y 1 si no hay nada que reportar. Con `--json`
    imprime el resultado crudo de cada bloque, sin formato, para procesarlo.
    """
    parser = argparse.ArgumentParser(
        description="Reporte de consumo del asistente de IA (solo lectura).",
    )
    parser.add_argument("--dias", type=int, default=7, help="ventana hacia atrás en días (por defecto 7)")
    parser.add_argument("--cluster", action="store_true", help="consultar el PostgreSQL del clúster con kubectl")
    parser.add_argument("-n", "--namespace", help="namespace del clúster (por defecto el actual)")
    parser.add_argument("--docker", default="soloservis_postgres", help="contenedor de PostgreSQL local")
    parser.add_argument("--json", action="store_true", help="salida JSON sin formato")
    args = parser.parse_args()

    if args.dias < 1:
        sys.exit("ERROR: --dias debe ser al menos 1.")
    if not SQL_FILE.exists():
        sys.exit(f"ERROR: no se encontró {SQL_FILE}.")

    queries = load_queries(SQL_FILE)
    missing = [name for name in SECTIONS if name not in queries]
    if missing:
        sys.exit(f"ERROR: faltan bloques en {SQL_FILE.name}: {', '.join(missing)}")

    results = {name: run_query(queries[name], args) for name in SECTIONS}

    if args.json:
        print(json.dumps(results, indent=2, ensure_ascii=False))
        return 0 if results["resumen"] else 1

    destino = "clúster" if args.cluster else f"docker ({args.docker})"
    print(f"Consumo del asistente de IA — últimos {args.dias} días — {destino}")

    summary = results["resumen"][0] if results["resumen"] else {}
    if not summary or not as_number(summary.get("consultas")):
        print()
        print("  No hay consultas registradas en la ventana. Nada que reportar.")
        return 1
    print_summary(summary)

    titles = {
        "por_dia": "Consumo por día",
        "por_usuario": "Consumo por usuario",
        "por_modelo": "Consumo por proveedor y modelo",
        "errores": "Errores agrupados",
        "repetidas": "Preguntas repetidas",
    }
    for name in SECTIONS[1:]:
        print_table(titles[name], results[name])

    print()
    return 0


if __name__ == "__main__":
    sys.exit(main())
