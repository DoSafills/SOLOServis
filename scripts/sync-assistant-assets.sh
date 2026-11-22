#!/usr/bin/env bash
# Sincroniza la configuración de assistant/ (fuente de verdad) con la copia que
# embebe Go en backend/internal/assistant/assets/.
#
#   ./scripts/sync-assistant-assets.sh          copia
#   ./scripts/sync-assistant-assets.sh --check  falla si están desincronizados
#
# Go no puede hacer //go:embed de una carpeta fuera del módulo backend/, así que
# la copia versionada es la que termina en el binario. CI corre --check.
set -eu

root="$(cd "$(dirname "$0")/.." && pwd)"
src="$root/assistant"
dst="$root/backend/internal/assistant/assets"
dirs="config prompts context"

copy_into() {
    target="$1"
    for dir in $dirs; do
        mkdir -p "$target/$dir"
        cp -R "$src/$dir/." "$target/$dir/"
    done
}

if [ "${1:-}" = "--check" ]; then
    tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"' EXIT
    copy_into "$tmp"
    if diff -r "$tmp" "$dst" >/dev/null 2>&1; then
        echo "assistant/assets sincronizado"
        exit 0
    fi
    echo "assistant/assets desincronizado; corre ./scripts/sync-assistant-assets.sh" >&2
    diff -r "$tmp" "$dst" || true
    exit 1
fi

rm -rf "$dst"
mkdir -p "$dst"
copy_into "$dst"
echo "Copiado assistant/{config,prompts,context} -> backend/internal/assistant/assets"
