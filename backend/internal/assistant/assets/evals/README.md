# Evaluaciones del asistente

`cases.json` fija el comportamiento esperado sin depender del proveedor:

- `expect: "answer"`: el asistente responde sobre la comparación.
- `expect: "refusal"`: responde exactamente la frase de `refusalExpected`, que
  debe coincidir con la de `prompts/system.es.md`.

Los casos se ejecutan desde `backend/internal/assistant/evals_test.go`. El
archivo se copia al binario con `scripts/sync-assistant-assets.sh` (junto con
`config/`, `prompts/` y `context/`), así que `--check` falla si esta carpeta y la
copia embebida divergen.

## Capa 1 — sin llamar al modelo (corre siempre)

```bash
cd backend && go test ./internal/assistant/ -run TestEval -count=1
```

No necesita `AI_API_KEY` ni red, y es la que corre `scripts/check-assistant.sh`
y CI:

| Test | Qué fija |
| --- | --- |
| `TestEvalCasesAreWellFormed` | Versión de la suite, ids únicos, `expect` válido, `mustIncludeAny` presente en los casos de respuesta, pregunta dentro del límite real |
| `TestEvalRefusalPhraseMatchesPrompt` | `refusalExpected` es idéntica a la del prompt: una sola fuente de verdad |
| `TestEvalFixtureRespectsAllowlist` | El contexto de ejemplo tiene 2–5 productos, no usa claves fuera de `allowed-fields.json` y no incluye ningún campo prohibido |

## Capa 2 — contra el modelo real (opt-in)

```bash
cd backend
set -a && . ./.env && set +a          # AI_PROVIDER, AI_MODEL, AI_BASE_URL, AI_API_KEY
SOLOSERVIS_EVALS_PROVIDER=1 go test ./internal/assistant/ -run TestEvalCasesAgainstProvider -count=1 -v
```

Exige el flag **y** la clave a propósito: sin el flag, un `go test ./...` con la
clave en el entorno haría diez llamadas pagadas sin avisar. Cada corrida gasta
cuota (10 consultas, del orden de 12 mil tokens con `deepseek-chat`).

Qué se afirma por caso:

- `answer`: la respuesta no es la frase de rechazo, contiene al menos uno de
  `mustIncludeAny` y ninguno de `mustNotIncludeAny`.
- `refusal`: la respuesta es exactamente `refusalExpected`.

La comparación ignora mayúsculas y tildes, para no fallar por redacción.

## Criterio al corregir un caso

La suite mide alcance y datos citados, no redacción. Si al correr la capa 2 un
caso falla, primero se lee qué respondió el modelo:

- si el modelo **inventó** un dato o se salió de los productos comparados, el
  error está en el prompt y se corrige `prompts/system.es.md`;
- si el modelo dijo lo correcto con otras palabras, el caso estaba escrito
  demasiado estrecho y se amplía la aserción.

Eso pasó en la primera corrida real (2026-11-22): dos casos exigían una frase
literal y el modelo usaba una equivalente. Se amplió la aserción y, aparte, se
endureció la regla 5 del prompt para que un rechazo no venga acompañado de una
comparación.
