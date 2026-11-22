# TAL-247 — Asistente de comparación con proveedor intercambiable

**Estado:** integrado en `feat/backend` y verificado contra DeepSeek en local.
Sin desplegar: el redeploy está en pausa por decisión del equipo.

---

## 1. Punto de partida

TAL-247 pide «finalizar la integración del asistente de IA» usando la rama
`Feature/Ricardo-AI`. La rama traía el asistente completo: backend, frontend,
assets y documentación.

| Dato | Valor |
| --- | --- |
| Cabeza de la rama | `81f9890` (2026-11-20 23:13) |
| Base común con `develop` | `df66bff` |
| Commits de `develop` que le faltaban | 60 |
| Archivos | 47 |
| Líneas | 4 737 |

El proveedor ya era agnóstico por configuración (`AI_PROVIDER`, `AI_MODEL`,
`AI_BASE_URL`, `AI_API_KEY`), lo que cumple el requisito de no atar la IA a un
proveedor ni recompilar para cambiarlo.

## 2. Conflictos del merge

Dos, ambos por desfase de numeración y de códigos de error.

### 2.1 La migración chocaba con un número ya usado

La rama crea `022_ai_comparison_query.sql`, pero en `develop` el 022 es
`022_unify_product_category_tree.sql`. Aplicarla tal cual rompía el orden y el
checksum del runner.

Se renombró a **`029_ai_comparison_query.sql`** y se agregó al final de
`migration_order.txt`. La migración es idempotente (`CREATE TABLE IF NOT
EXISTS`) y no depende de las anteriores.

### 2.2 Dos códigos para el mismo 429

La rama sumaba `CodeTooManyRequests = "too_many_requests"`, mientras `develop`
ya tenía `CodeRateLimited = "rate_limited"` desde TAL-201, que es el que fija el
estándar TAL-78 e incluye `Retry-After`.

Se conservó `rate_limited` y se descartó el duplicado. El handler del asistente
usa ahora `httpx.WriteRateLimited`, y el limitador expone `RetryAfter(key)` para
calcular los segundos: si se agotó la ventana del minuto espera al acierto más
antiguo de esa ventana; si se agotó la del día, al más antiguo de las últimas
24 horas.

De la rama sólo sobrevive `CodeAIUnavailable = "ai_unavailable"`, que distingue
«el proveedor no respondió» de una caída de la base.

## 3. Wiring: dónde vive la clave

Decisión del equipo: **DeepSeek `deepseek-chat`**, por costo, usando el camino
`compatible` (formato `chat/completions` de OpenAI) que el código ya soportaba.

```env
AI_PROVIDER=compatible
AI_BASE_URL=https://api.deepseek.com/v1
AI_MODEL=deepseek-chat
AI_API_KEY=sk-...     # nunca en git
```

| Entorno | Dónde vive el valor | Quién lo crea |
| --- | --- | --- |
| Clúster UCT | Secret `soloservis-ai`, clave `AI_API_KEY` | Fase 4 de `deploy.sh` / `deploy.ps1` |
| Compose local | `docker/.env` | La persona |
| `go run` local | `backend/.env` | La persona |

La Fase 4 gana `ensure_ai_secret()` / `Ensure-AISecret`:

| Situación | Resultado |
| --- | --- |
| `soloservis-ai` existe | Se reutiliza; no se lee `AI_API_KEY` |
| No existe y hay `AI_API_KEY` | Se crea desde un archivo temporal con permisos `600`, borrado en el mismo paso |
| No existe y falta `AI_API_KEY` | Avisa y **continúa**: el asistente responde 503 y nada más se cae |

La clave nunca se pasa por la línea de comandos, así que no queda en el
historial del shell ni en la lista de procesos.

En `k8s/` sólo hay referencias, nunca el valor:

```yaml
- name: AI_API_KEY
  valueFrom:
    secretKeyRef: {name: soloservis-ai, key: AI_API_KEY, optional: true}
```

Con `optional: true`, si el Secret no existe la variable simplemente no está:
`NewProviderFromConfig` devuelve `nil` y la ruta responde `503 ai_unavailable`.

El wiring se hizo en las dos topologías (`dominios/comercial.yaml` y
`consolidado/backend.yaml`) para que el respaldo no quede sin asistente.

## 4. Verificación local

### 4.1. Estática

```
go build ./...                    sin errores
go vet ./...                      sin observaciones
go test ./... -count=1            20 paquetes ok, 0 FAIL
bash scripts/check-assistant.sh   assets sincronizados + contrato ok
pnpm check (fronted/)             formato, lint, build y auditoría ok
kubectl kustomize overlays/uct    renderiza con AI_* en el pod
docker compose config --quiet     ok
bash -n scripts/k8s/deploy.sh     sintaxis ok
```

### 4.2. Prueba con el proveedor real (2026-11-22)

Con los cinco contenedores de `docker/compose.yml` arriba, DeepSeek configurado
por entorno y tres refrigeradores del catálogo real:

```http
POST http://localhost:8081/comparisons/assistant
```

```json
{
  "productIds": ["9d212279-…", "c835e09e-…", "8e1cf996-…"],
  "question": "¿Cuál conviene para una familia de 5 con presupuesto de 900 mil?"
}
```

`HTTP 200` en **2,27 s**:

```json
{
  "answer": "Con 900 mil tienes dos opciones: el LG French Door 530 L y el LG Side by Side, ambos a $899.990… Para una familia de 5, el Side by Side gana en capacidad y en eficiencia…",
  "sources": ["9d212279-…", "c835e09e-…", "8e1cf996-…"],
  "provider": "compatible",
  "model": "deepseek-chat",
  "cached": false
}
```

La respuesta menciona los productos comparados, usa solo datos de la matriz
(precio, litros, eficiencia) y declara lo que no tiene («No hay datos de
garantía ni materiales»), que es el guardrail esperado.

La auditoría quedó en PostgreSQL:

```
provider    | model          | prompt | output | latency | cached | success
compatible  | deepseek-chat  |    956 |    178 |   2207  | f      | t
compatible  | deepseek-chat  |        |        |         | t      | t
```

Repetir la misma pregunta devuelve `cached: true` y no consume cuota: la caché
en memoria de 24 h funciona antes de llamar al proveedor.

### 4.3. El resto del contrato, en el binario desplegado

| Caso | Resultado |
| --- | --- |
| Cuota agotada (11 pedidos en un minuto) | `429 rate_limited` con `Retry-After: 34` |
| Un solo producto | `422 validation_failed` con `errors[0].field = productIds` |
| Campo desconocido | `400 unknown_field` |
| Diez llamadas auditadas | 10 filas, 10 con `success`, 9 231 tokens en total |

Los tres primeros son el estándar TAL-78 respondido por el handler real, no por
un test unitario.

Prueba nueva: `TestDeepSeekProvider` arma el proveedor desde el entorno
(`AI_PROVIDER=compatible`, `AI_BASE_URL`, `AI_MODEL`, `AI_API_KEY`) contra un
servidor simulado y verifica que la llamada va a `/v1/chat/completions`, que la
clave viaja en `Authorization: Bearer`, que el modelo enviado es `deepseek-chat`
y que no se manda el campo `private` (exclusivo de Pollinations). Es la prueba
de que cambiar de proveedor es configuración y no código.

## 5. Límites y pendientes

- **La llamada real a DeepSeek se hizo y funciona** (2026-11-22, ver §4.2). Lo
  que sigue sin probar es el camino `gemini`, que sólo tiene cobertura de
  servidor simulado.
- **Nada está en el clúster**: el redeploy está en pausa por decisión del
  equipo, y siguen sin aplicar las migraciones 026, 027, 028 y ahora 029. En
  local se aplicaron solas al inicializar el volumen de PostgreSQL.
- **El pod de test no cubre el asistente**: las 35 verificaciones no incluyen
  esta ruta; agregarla exige decidir antes cómo se prueba sin gastar cuota.
- **`deploy.ps1` no se validó aquí**: no hay PowerShell en el equipo de
  desarrollo. El archivo quedó en ASCII y con sintaxis compatible con PS 5.1;
  la validación real ocurre en el primer despliegue desde Windows.
- **El frontend no se probó contra el contenedor**: `docker/compose.yml` no
  levanta el gateway; la tarjeta `ComparisonAssistantCard` se valida con
  `pnpm dev` contra `VITE_COMPARISON_API_URL`.

Queda pendiente para la evidencia del clúster: la respuesta del proveedor desde
el pod, la fila en `ai_comparison_query` y el `provider`/`model` que devuelve
`POST /comparisons/assistant` en UCT.
