# Evaluaciones del asistente

`cases.json` fija el comportamiento esperado sin depender del proveedor:

- `expect: "answer"`: el asistente responde sobre la comparación.
- `expect: "refusal"`: responde exactamente la frase de `refusalExpected`, que
  debe coincidir con la de `prompts/system.es.md`.

## Qué se puede verificar sin llamar al modelo

1. **Esquema del contexto**: `context/example-comparison.json` valida contra
   `context/comparison-context.schema.json`.
2. **Allowlist**: ninguna clave fuera de `context/allowed-fields.json`.
3. **Frase de rechazo**: `refusalExpected` es idéntica a la del prompt.
4. **Límites**: pregunta ≤ 300 caracteres, productos 2–5, misma categoría.

Estas comprobaciones son deterministas y deben correr en CI. No necesitan
`AI_API_KEY`.

## Qué necesita el proveedor

Los casos de `answer` y `refusal` se evalúan contra el modelo real, con el
contexto de `contextFixture`. Como los modelos gratuitos cambian y no son
deterministas:

- se corre a mano antes de cambiar el prompt o el modelo, no en cada commit;
- se acepta variación de redacción, pero **no** de alcance: un caso `refusal`
  que responde contenido de compras es una falla;
- se registra el modelo evaluado en `config/ai.config.json`.

## Cómo evaluar

El backend ya expone `POST /comparisons/assistant`. Para evaluar:

1. Levantar el backend con `AI_API_KEY` configurada.
2. Enviar cada caso con los `productIds` del fixture y la `question`.
3. Comparar la respuesta: para `refusal`, igualdad exacta; para `answer`,
   presencia de `mustIncludeAny` y ausencia de `mustNotIncludeAny`.

No se incluyen respuestas "correctas" completas a propósito: el asistente puede
redactar distinto, pero el alcance y los datos citados no pueden cambiar.
