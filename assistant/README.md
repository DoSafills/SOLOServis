# assistant/ — asistente de comparación

Carpeta hermana de `backend/`, `fronted/`, `scraper/` y `crawler/`. Guarda la
**configuración, el pre-contexto y las limitaciones** del asistente que responde
preguntas sobre los productos que el usuario está comparando.

**Estado:** conectado. El backend sirve `POST /comparisons/assistant` desde
`backend/internal/assistant/` y la ventana del frontend ya consume la respuesta
real. Con `AI_API_KEY` responde Gemini (`gemini-3.5-flash`); sin clave responde
503 `ai_unavailable` y **no** arma texto propio; `AI_PROVIDER=off` desactiva la
ruta sin romper el resto del sitio.

---

## 1. Qué hace el asistente

El usuario está comparando entre 2 y 5 productos de una misma categoría. El
asistente responde en español, en lenguaje natural, usando **solo** los datos de
esa comparación: precio, tienda, atributos y especificaciones.

Ejemplos válidos (las preguntas sugeridas del frontend):

- "¿Cuál tiene más capacidad?"
- "¿Cuál tiene mejores materiales?"
- "¿Qué diferencias técnicas importan?"
- "¿Cuál me recomiendas y por qué?"

Cualquier otro tema queda fuera de alcance.

---

## 2. Invariantes (no se negocian)

1. **El cliente solo manda IDs.** El frontend envía `productIds` + `question`;
   nunca datos de producto, precios ni atributos.
2. **El contexto se arma en el servidor** desde PostgreSQL, solo con los
   productos comparados (2–5, misma categoría).
3. **Nada de datos de usuario.** No se envían cuentas, sesiones, correos,
   historial de navegación ni el resto del catálogo.
4. **La IA no inventa.** Si un dato no está en el contexto, el asistente dice que
   no está disponible.
5. **La clave nunca sale del backend.** `AI_API_KEY` jamás en variables
   `VITE_*`: terminan en el bundle del navegador.
6. **La respuesta es siempre del modelo.** El backend no arma frases ni tiene
   respuestas de respaldo: sin proveedor responde 503 `ai_unavailable` y el
   frontend muestra el aviso con Reintentar. El test
   `TestAnswerIsOnlyTheModelText` lo fija.

---

## 3. Estructura

```
assistant/
├── config/
│   ├── ai.config.json    proveedor, modelo y parámetros de generación
│   ├── limits.json       límites de uso (entrada, salida, cuota, caché)
│   └── env.example       variables de entorno del backend
├── prompts/
│   ├── system.es.md      pre-contexto: rol, reglas y frase de rechazo
│   ├── context-template.md  cómo se serializa la matriz al modelo
│   └── suggested-questions.json  preguntas sugeridas del frontend
├── context/
│   ├── allowed-fields.json      allowlist/denylist de campos
│   ├── comparison-context.schema.json  JSON Schema del contexto
│   └── example-comparison.json  ejemplo válido con 3 productos
├── sql/
│   └── comparison-context.sql   consulta de referencia (solo lectura)
├── evals/
│   ├── cases.json        casos de aceptación y de rechazo
│   └── README.md         cómo se evalúan
└── docs/
    └── propuesta-asistente.md   propuesta original, con contrato y plan
```

La **frase exacta de rechazo** tiene una única fuente: `prompts/system.es.md`.
Si cambia, se cambia solo ahí; el backend la extrae del prompt embebido con
`RejectionPhrase()` y la usa cuando la respuesta no menciona los productos
comparados.

---

## 4. Alcance de los datos

`context/allowed-fields.json` es la lista cerrada. En resumen:

| Se envía | No se envía |
| --- | --- |
| `category.name` | `imageUrl` |
| `products[].name`, `brand`, `model` | datos de `user_account` |
| `products[].bestPrice`, `bestStore`, `offerCount`, `lastUpdated` | sesiones, tokens, correos |
| `attributes[]` (etiqueta, unidad, valores, `differs`) | historial de precios |
| `specifications[]` (nombre, unidad, valores, `differs`), sin las que repiten un atributo | reseñas |
| | UUID del producto, clave y tipo del atributo |
| | resto del catálogo |
| | credenciales o claves de API |

El JSON no pasa de `context.maxBytes` (`config/limits.json`, 8192 bytes); si
se pasa, el backend quita filas y lo registra como `assistant_context_trimmed`.

`context/comparison-context.schema.json` valida ese payload. Cualquier campo
fuera de la allowlist rompe la validación y se descarta antes de llamar al
proveedor.

---

## 5. Cómo está conectado

Go no puede hacer `//go:embed` de una carpeta fuera del módulo `backend/`, así
que la fuente de verdad vive aquí y se copia al backend:

1. `scripts/sync-assistant-assets.sh` copia `config/`, `prompts/` y `context/` a
   `backend/internal/assistant/assets/`. El modo `--check` falla si están
   desincronizados (para CI).
2. `backend/internal/assistant/` embebe esa copia con `go:embed`, arma el
   contexto desde PostgreSQL, aplica los límites y llama al proveedor
   (`provider.go`), con timeout y reintentos.
3. Ruta `POST /comparisons/assistant` (Comparison API), pública y con rate limit
   por IP (10/min, 100/día). Valida el body con el estándar TAL-78:
   `400` al decodificar, `422` al incumplir reglas, `404` si un producto no
   existe, `429` `rate_limited` con `Retry-After` al agotar la cuota y `503`
   `ai_unavailable` con el asistente en `off` o si el proveedor falla.
4. Migración `029_ai_comparison_query.sql` para auditoría (pregunta, respuesta,
   proveedor, tokens, latencia, caché). Un fallo al auditar no rompe la
   respuesta.
5. `fronted/src/services/api/assistant.ts` y la ventana
   `ComparisonAssistantCard` en `ProductComparisonPage.tsx` consumen la ruta y
   muestran los estados de error sin afectar la comparación.
6. `scripts/check-assistant.sh` es el pre-commit del asistente: valida que los
   assets estén sincronizados y corre los tests que fijan el contrato (respuesta
   solo del modelo, contexto desde la matriz y 503 sin clave). `lint-staged` lo
   ejecuta cuando el commit toca `assistant/` o `backend/internal/assistant/`.

La caché es en memoria por hash de IDs ordenados + pregunta normalizada, TTL 24 h
(configurable en `config/limits.json`). No sustituye a Redis: el caché se pierde
al reiniciar el proceso.

### Proveedores

| `AI_PROVIDER` | Comportamiento |
| --- | --- |
| `auto` (por defecto) | Gemini si hay `AI_API_KEY`; sin clave, 503 `ai_unavailable` |
| `gemini` | Gemini; sin clave responde 503 |
| `pollinations` | API compatible OpenAI sin clave (≈1 respuesta/45 s por IP) |
| `compatible` | Cualquier API compatible OpenAI en `AI_BASE_URL` (Groq, Ollama, proxy) |
| `off` | Desactiva el asistente: 503 `ai_unavailable` |

El modelo de Gemini se fija con `AI_MODEL` (`gemini-3.5-flash` por defecto). Google
marca `gemini-3.5-flash` como deprecado y lo enruta a `gemini-3.6-flash`; para
fijar otro, cambia `AI_MODEL` (por ejemplo `gemini-3.6-flash` o
`gemini-3.5-flash-lite`). Para mover el endpoint sin tocar código, cambia
`AI_BASE_URL`: el formato de la petición sigue siendo el de Gemini.

**No hay respuestas de plantilla.** El proveedor local determinista se eliminó:
si no hay IA conectada, la ruta falla con 503 en vez de devolver texto armado por
el backend. Los datos que ve el modelo salen de `buildModelContext`, que solo
serializa la matriz de la comparación (productos, atributos y especificaciones).

---

## 6. Lo que NO va en esta carpeta

- **Claves o secretos.** Solo los nombres de las variables (`config/env.example`).
  En el clúster, `AI_API_KEY` vive en el Secret `soloservis-ai`.
- **Migraciones o consultas ejecutables.** `sql/comparison-context.sql` es
  referencia: las consultas reales van a `backend/database/queries/` y se
  regeneran con `sqlc`.
- **La tabla `ai_query_process`.** Es legado: depende de `search_query` y sirve
  para clasificar intención de búsqueda, no para comparar. No reutilizar.
- **Código del proveedor.** Los adaptadores viven en
  `backend/internal/assistant/provider.go`: Gemini y API compatible
  OpenAI (Pollinations/Groq/Ollama). Esta
  carpeta solo define configuración, prompt y límites.

---

## 7. Referencias

| Tema | Archivo |
| --- | --- |
| Propuesta completa, contrato y pasos | `docs/propuesta-asistente.md` |
| Estándar de errores | `docs/estandares/TAL-78-estandar-api.md` |
| Contrato de la matriz | `backend/internal/comparison/model.go` |
| Contexto general del repo | `docs/CONTEXTO-IA.md` |
