#!/bin/bash
# Aplica los SQL listados en MIGRATION_ORDER_FILE, una sola vez cada archivo.
# Registra cada archivo en la tabla schema_migrations (version + suma de verificacion).
set -euo pipefail

MIGRATIONS_DIR="${MIGRATIONS_DIR:-/migrations}"
MIGRATION_ORDER_FILE="${MIGRATION_ORDER_FILE:-${MIGRATIONS_DIR}/migration_order.txt}"

echo "Esperando PostgreSQL en ${PGHOST}:${PGPORT}..."
for _ in $(seq 1 90); do
  if pg_isready -q; then break; fi
  sleep 2
done
pg_isready

if [ ! -f "$MIGRATION_ORDER_FILE" ]; then
  echo "ERROR: no existe el manifiesto $MIGRATION_ORDER_FILE" >&2
  exit 1
fi

files=()
versions=()
declare -A seen_versions=()
while IFS= read -r version || [ -n "$version" ]; do
  version="${version%$'\r'}"
  [[ -z "${version//[[:space:]]/}" || "$version" =~ ^[[:space:]]*# ]] && continue
  if [[ "$version" != *.sql || "$version" == */* || ! "$version" =~ ^[A-Za-z0-9._-]+\.sql$ ]]; then
    echo "ERROR: entrada invalida en $MIGRATION_ORDER_FILE: $version" >&2
    exit 1
  fi
  if [ -n "${seen_versions[$version]:-}" ]; then
    echo "ERROR: version duplicada en $MIGRATION_ORDER_FILE: $version" >&2
    exit 1
  fi

  f="$MIGRATIONS_DIR/$version"
  if [ ! -f "$f" ]; then
    echo "ERROR: no existe el archivo listado: $f" >&2
    exit 1
  fi
  seen_versions["$version"]=1
  versions+=("$version")
  files+=("$f")
done < "$MIGRATION_ORDER_FILE"

if [ "${#files[@]}" -eq 0 ]; then
  echo "ERROR: el manifiesto no contiene scripts SQL" >&2
  exit 1
fi

shopt -s nullglob
for f in "$MIGRATIONS_DIR"/*.sql; do
  version="$(basename "$f")"
  if [ -z "${seen_versions[$version]:-}" ]; then
    echo "ERROR: falta una entrada para $version en $MIGRATION_ORDER_FILE" >&2
    exit 1
  fi
done

# Base con esquema previo pero sin registro: no se re-ejecuta 001 a ciegas.
has_schema="$(psql -tAc "SELECT to_regclass('public.user_account') IS NOT NULL")"
has_tracking_table="$(psql -tAc "SELECT to_regclass('public.schema_migrations') IS NOT NULL")"
if [ "$has_schema" = "t" ] && [ "$has_tracking_table" != "t" ]; then
  echo "ERROR: la base ya tiene esquema pero no tiene schema_migrations." >&2
  echo "Revisar manualmente antes de migrar (no se modifica nada)." >&2
  exit 1
fi

if [ "$has_tracking_table" = "t" ]; then
  applied="$(psql -tAc "SELECT count(*) FROM schema_migrations")"
  if [ "$has_schema" = "t" ] && [ "$applied" = "0" ]; then
    echo "ERROR: la base ya tiene esquema pero schema_migrations esta vacia." >&2
    echo "Revisar manualmente antes de migrar (no se modifica nada)." >&2
    exit 1
  fi
else
  psql -v ON_ERROR_STOP=1 -q -c "CREATE TABLE schema_migrations (
    version    TEXT PRIMARY KEY,
    checksum   TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
  );"
fi

for i in "${!files[@]}"; do
  f="${files[$i]}"
  version="${versions[$i]}"
  # Suma de verificacion sin CR: igual en copias Windows (CRLF) y Linux (LF).
  checksum="$(tr -d '\r' < "$f" | sha256sum | cut -d' ' -f1)"
  current="$(psql -tAc "SELECT checksum FROM schema_migrations WHERE version = '${version}'")"

  if [ -n "$current" ]; then
    if [ "$current" != "$checksum" ]; then
      echo "AVISO: ${version} cambio despues de aplicarse; no se re-ejecuta."
    fi
    echo "SKIP  ${version}"
    continue
  fi

  echo "APPLY ${version}"
  psql -q -v ON_ERROR_STOP=1 -f "$f"
  psql -q -v ON_ERROR_STOP=1 -v v="$version" -v checksum="$checksum" -f - <<'SQL'
INSERT INTO schema_migrations (version, checksum) VALUES (:'v', :'checksum');
SQL
done

echo "Migraciones aplicadas:"
for version in "${versions[@]}"; do
  psql -tAc "SELECT version || '  ' || applied_at FROM schema_migrations WHERE version = '${version}'"
done
