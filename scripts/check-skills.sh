#!/bin/sh
# Verifica que los skills de .opencode y .claude no hayan divergido.
#
# El contenido canónico vive en .opencode/skills/ y .claude/skills/ es su
# espejo, porque cada agente lee de su propio directorio. Dos copias derivan:
# este script es el que lo impide.
#
#   ./scripts/check-skills.sh          # falla si difieren
#   ./scripts/check-skills.sh --sync   # copia .opencode -> .claude
set -u

RAIZ=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
ORIGEN="$RAIZ/.opencode/skills"
ESPEJO="$RAIZ/.claude/skills"

if [ "${1:-}" = "--sync" ]; then
  rm -rf "$ESPEJO"
  mkdir -p "$ESPEJO"
  cp -R "$ORIGEN"/. "$ESPEJO"/
  echo "Sincronizado: $ESPEJO"
  exit 0
fi

if [ ! -d "$ORIGEN" ]; then
  echo "ERROR: no existe $ORIGEN" >&2
  exit 1
fi
if [ ! -d "$ESPEJO" ]; then
  echo "ERROR: no existe $ESPEJO (corre $0 --sync)" >&2
  exit 1
fi

if ! diff -r "$ORIGEN" "$ESPEJO"; then
  echo "ERROR: los skills divergieron. Corre $0 --sync y revisa el diff." >&2
  exit 1
fi

total=$(find "$ORIGEN" -name SKILL.md | wc -l | tr -d ' ')
echo "OK: $total skills sincronizados entre .opencode y .claude"
