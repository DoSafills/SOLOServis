# TAL-247 — Asistente de comparación con proveedor intercambiable

**Estado:** integrado en `feat/backend`, sin desplegar. La llamada real al
proveedor queda pendiente de la clave y del redeploy, que está en pausa.

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

Prueba nueva: `TestDeepSeekProvider` arma el proveedor desde el entorno
(`AI_PROVIDER=compatible`, `AI_BASE_URL`, `AI_MODEL`, `AI_API_KEY`) contra un
servidor simulado y verifica que la llamada va a `/v1/chat/completions`, que la
clave viaja en `Authorization: Bearer`, que el modelo enviado es `deepseek-chat`
y que no se manda el campo `private` (exclusivo de Pollinations). Es la prueba
de que cambiar de proveedor es configuración y no código.

## 5. Límites y pendientes

- **La llamada real a DeepSeek no se hizo**: falta la clave. El camino está
  probado contra un servidor HTTP simulado, no contra la API.
- **Nada está en el clúster**: el redeploy está en pausa por decisión del
  equipo, y siguen sin aplicar las migraciones 026, 027, 028 y ahora 029.
- **El pod de test no cubre el asistente**: las 35 verificaciones no incluyen
  esta ruta; agregarla exige decidir antes cómo se prueba sin gastar cuota.
- **`deploy.ps1` no se validó aquí**: no hay PowerShell en el equipo de
  desarrollo. El archivo quedó en ASCII y con sintaxis compatible con PS 5.1;
  la validación real ocurre en el primer despliegue desde Windows.

Cuando haya clave y redeploy, esta auditoría se completa con la evidencia real:
respuesta del proveedor, fila en `ai_comparison_query` y el `provider`/`model`
que devuelve `POST /comparisons/assistant`.
