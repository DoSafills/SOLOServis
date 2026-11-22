# TAL-226 — Respuestas alternativas sin IA

| Campo | Valor |
| --- | --- |
| Fecha | 2026-11-16 |
| Alcance | `backend/internal/search/alternatives.go`, router, README |
| Depende de | TAL-222 (catálogo de presets) |

## 1. Contrato

`GET /search/alternatives?q=<texto>&limit=&offset=`

```json
{
  "query": "notebook para estudiar",
  "source": "preset",
  "matched": { "slug": "notebooks", "title": "Notebooks", "description": "…" },
  "presets": [ { "slug": "notebooks-economicos", … } ],
  "products": [ … ]
}
```

| Campo | Significado |
| --- | --- |
| `source` | `"preset"` si se respondió con una consulta curada; `"text"` si fue búsqueda libre |
| `matched` | El preset usado, o `null` cuando `source` es `"text"` |
| `presets` | Hasta tres sugerencias, sin el preset ya usado; nunca `null` |
| `products` | Resultados reales; nunca `null`. Total en `X-Total-Count` |

Errores: 400 `problem+json` si falta `q`, supera 200 caracteres o 5 palabras, o
la paginación es inválida. 500 genérico con el detalle sólo en el log.

## 2. Cómo empareja

1. **Normalizar**: minúsculas, sin tildes ni ñ, sólo letras y dígitos.
2. **Quitar stopwords**: `para`, `con`, `quiero`, `algo`, `mejor`… si la consulta
   queda vacía, no se adivina nada.
3. **Singular conservador**: `notebooks` → `notebook`, `televisores` → `televisor`.
4. **Puntuar**: proporción de tokens de la consulta presentes en el vocabulario
   del preset (slug + título + `Aliases`).
5. **Umbral 0.5**: si el mejor no lo alcanza, se responde con búsqueda libre y se
   sugieren hasta tres presets con algo de coincidencia.
6. **Desempate**: orden editorial del catálogo, no alfabético.

Ejemplos reales:

| Consulta | Resultado |
| --- | --- |
| `refrigerador barato` | `preset` → `refrigeradores-economicos` (2/2) |
| `neveras` | `preset` → `refrigeradores` (alias + plural) |
| `notebook para estudiar` | `preset` → `notebooks` (1/2 = 0.5) |
| `notebook con pantalla 4k` | `text`, sugerencias: notebooks, notebooks-económicos, televisores (1/3 cada uno) |
| `taladro percutor` | `text`, sin sugerencias |

## 3. Decisiones

| Decisión | Motivo |
| --- | --- |
| **No se genera texto** | Sin prosa generada no hay información inventada que revisar. `source` explica el resultado; todo lo demás son datos |
| **No se llama a un LLM** | La tarea es explícitamente "sin IA"; un modelo añadiría latencia, coste y una superficie de error que hoy no se puede operar |
| **Umbral 0.5 y no fuzzy** | Distancia de edición produce falsos positivos ("nevera" → "nevería"). El solapamiento de tokens es predecible y explicable |
| **La paginación del cliente manda** | Si envía `limit`, se respeta; si no, vale el del preset (novedades y precios-bajos piden 12) |
| **`presets` y `products` siempre arreglos** | Un `null` obliga al cliente a defenderse; la API no lo produce |

## 4. Pruebas

`backend/internal/search/alternatives_test.go` (8 funciones):

| Test | Qué fija |
| --- | --- |
| `TestTokens_NormalizaAcentosPluralesYStopwords` | Normalización: tildes, plurales, stopwords, `4K` |
| `TestAlternatives_EmparejaElPresetMasEspecifico` | "refrigerador barato" gana al preset genérico; filtros y `X-Total-Count` |
| `TestAlternatives_AceptaAcentoYPlural` | "neveras" → refrigeradores |
| `TestAlternatives_SinCoincidenciaUsaBusquedaLibre` | `source=text`, `matched=null`, palabras intactas, `presets` no es `null` |
| `TestAlternatives_SugierePresetsDebajoDelUmbral` | 1/3 no alcanza el umbral; tres sugerencias en orden editorial |
| `TestAlternatives_RespetaLosLimites` | `limit` del cliente vs. límite del preset |
| `TestAlternatives_ValidaLaConsulta` | Sin `q`, 6 palabras y `limit=0` → 400 sin tocar la base |
| `TestAlternatives_ErrorInternoNoFiltraDetalles` | 500 genérico, detalle sólo en el log |

## 5. Verificación

```text
$ go build ./... && go vet ./... && go test ./...
ok  	github.com/DoSafills/SOLOServis/backend/internal/search
ok  	github.com/DoSafills/SOLOServis/backend/internal/http
```

## 6. Límites declarados

- **Sólo productos**, igual que TAL-222.
- **Sin sinónimos que no estén en `Aliases`**: ampliar el vocabulario es editar
  el preset, no el código.
- **Sin puntuación por marca o categoría**: el emparejamiento es léxico, no
  entiende "el más barato que un iPad".
- **Sin frontend.**

## 7. Relación con las tareas de IA

Este endpoint es la alternativa determinista. Cuando exista proveedor de IA
(TAL-175 en adelante), puede consumir este contrato como respaldo: si el modelo
falla o tarda, el cliente ya tiene `source: "text"` con sugerencias reales. El
contrato no tiene que cambiar.
