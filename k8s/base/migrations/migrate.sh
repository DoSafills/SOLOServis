#!/bin/bash
# Aplica backend/database/migrations/*.sql en orden, una sola vez cada una.
# Registro en la tabla schema_migrations (version + checksum).
set -euo pipefail

MIGRATIONS_DIR="${MIGRATIONS_DIR:-/migrations}"

echo "Esperando PostgreSQL en ${PGHOST}:${PGPORT}..."
for _ in $(seq 1 90); do
  if pg_isready -q; then break; fi
  sleep 2
done
pg_isready

psql -v ON_ERROR_STOP=1 -q -c "CREATE TABLE IF NOT EXISTS schema_migrations (
  version    TEXT PRIMARY KEY,
  checksum   TEXT NOT NULL,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);"

# Base con esquema previo pero sin registro: no se re-ejecuta 001 a ciegas.
has_schema="$(psql -tAc "SELECT to_regclass('public.user_account') IS NOT NULL")"
applied="$(psql -tAc "SELECT count(*) FROM schema_migrations")"
if [ "$has_schema" = "t" ] && [ "$applied" = "0" ]; then
  echo "ERROR: la base ya tiene esquema pero schema_migrations esta vacia." >&2
  echo "Revisar manualmente antes de migrar (no se modifica nada)." >&2
  exit 1
fi

shopt -s nullglob
files=("$MIGRATIONS_DIR"/*.sql)
if [ "${#files[@]}" -eq 0 ]; then
  echo "ERROR: no hay archivos .sql en $MIGRATIONS_DIR" >&2
  exit 1
fi

IFS=$'\n' sorted=($(printf '%s\n' "${files[@]}" | sort))
unset IFS

for f in "${sorted[@]}"; do
  version="$(basename "$f")"
  # Checksum sin CR: igual en checkouts Windows (CRLF) y Linux (LF).
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
  # Archivo + registro en la misma transaccion (si falla, no queda a medias).
  psql -q -v ON_ERROR_STOP=1 --single-transaction \
    -f "$f" \
    -c "INSERT INTO schema_migrations (version, checksum) VALUES ('${version}', '${checksum}')"
done

echo "Migraciones aplicadas:"
psql -tAc "SELECT version || '  ' || applied_at FROM schema_migrations ORDER BY version"
