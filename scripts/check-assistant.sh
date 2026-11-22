#!/usr/bin/env bash
# Pre-commit del asistente de comparación.
#
# Fija el contrato que importa:
#   1. La copia embebida (backend/internal/assistant/assets/) está sincronizada
#      con assistant/, que es la fuente de verdad.
#   2. La respuesta de POST /comparisons/assistant es el texto del proveedor de
#      IA, sin plantillas ni respuestas armadas por el backend.
#   3. El contexto enviado al modelo sale de la matriz de la comparación
#      (productos, atributos y especificaciones), no de otro lado.
#   4. Sin AI_API_KEY la ruta responde 503 ai_unavailable y no filtra datos.
#
# Uso: ./scripts/check-assistant.sh
# Para saltarlo en una emergencia: SKIP_ASSISTANT_CHECK=1 git commit ...
set -eu

if [ "${SKIP_ASSISTANT_CHECK:-0}" = "1" ]; then
    echo "check-assistant: omitido por SKIP_ASSISTANT_CHECK=1" >&2
    exit 0
fi

root="$(cd "$(dirname "$0")/.." && pwd)"

echo "check-assistant: sincronizando assets"
bash "$root/scripts/sync-assistant-assets.sh" --check

echo "check-assistant: tests del contrato"
cd "$root/backend"
go test ./internal/assistant/... -count=1
