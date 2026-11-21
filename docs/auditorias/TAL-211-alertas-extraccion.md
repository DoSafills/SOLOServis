# TAL-211 — Alertas de fallos de extracción

| Campo | Valor |
| --- | --- |
| Fecha | 2026-11-21 |
| Alcance | `backend/internal/ingestion/`, `backend/internal/features/`, `scraper/src/scraper/` |
| Migraciones | Ninguna: la tabla `notification` ya existía |
| Idea | Una tienda que deja de entregar datos se avisa sola, sin abrir el historial |

## 1. El problema

El scraper registra por corrida cuántas tarjetas vio, cuántas envió y cuántas
rechazó, pero nada de eso llegaba al backend: `POST /ingestion/runs/{id}/finish`
solo aceptaba `status` y `errorMessage`. Una tienda podía pasar días enviando
cero ítems y la única señal era un `run.json` que nadie abría.

## 2. Dónde se decide

En ingestion, al cerrar la corrida. Es el único dominio autorizado a llamar a
notification (`NOTIFICATION_API_URL=http://comercial:8080`), así que el scraper
no gana una conexión ni credenciales nuevas y la regla vive donde también vive
la tabla de usuarios.

## 3. Contrato (retrocompatible)

`POST /ingestion/runs/{id}/finish` acepta cuatro contadores opcionales:

```json
{
  "status": "success",
  "itemsFound": 30,
  "itemsSent": 28,
  "itemsRejected": 2,
  "pagesFailed": 0
}
```

- Si no vienen, no se evalúa nada: un scraper anterior cierra la corrida igual.
- Un valor negativo o mayor que `MaxCounterValue` (1 000 000) es **422** con el
  campo culpable (`itemsSent/min`, `itemsFound/max`), nunca 500.

## 4. Regla de degradación

Con `itemsFound > 0`:

| Condición | Razón que se informa |
| --- | --- |
| `itemsSent = 0` | «no se envió ningún ítem a ingestion» (+ páginas caídas si las hubo) |
| `itemsRejected / itemsFound > 0.30` | «N de M tarjetas rechazadas (X% supera el umbral)» |
| `itemsSent < 0.5 × mediana` de las últimas 5 corridas exitosas | «envió N ítems contra una mediana de M» |

Detalles que se decidieron a propósito:

- **Una página caída de veinte no alerta** si el catálogo llegó completo. Sólo se
  nombra cuando no se envió nada.
- **Sin historial no hay caída**: en la primera corrida de una tienda la mediana
  es cero y no se compara.
- **La mediana toma el menor de los dos centrales** con historial par, para
  quedarse con un valor que existió de verdad.
- Los umbrales se ajustan sin recompilar:
  `INGESTION_DEGRADED_REJECT_RATIO`, `INGESTION_DEGRADED_DROP_RATIO`,
  `INGESTION_DEGRADED_BASELINE_RUNS`. Un valor inválido se ignora: una variable
  mal escrita no puede apagar las alertas.

## 5. Dónde llega el aviso

`POST /notifications/admins` (nuevo, bajo `RequireAdmin`, que ya acepta el token
de servicio): inserta una fila por cada usuario `admin` y responde
`201 {"created": n}`. El destinatario lo resuelve Notification API, que es la
dueña de la tabla de cuentas.

El aviso queda con `type = scraper_degraded`, título
«La tienda {fuente} dejó de entregar productos», mensaje con las razones y
`data` con corrida, tienda, contadores y estado.

Si Notification API falla, se registra
`ingestion_degradation_notification_failed` y el cierre responde **200** igual:
perder una alerta no puede tumbar una ingesta.

## 6. Cambios por archivo

| Archivo | Cambio |
| --- | --- |
| `backend/internal/ingestion/model.go` | `FinishRequest` con contadores opcionales; umbrales por defecto; `NotificationTypeDegraded` |
| `backend/internal/ingestion/degrade.go` | `Counters`, `Thresholds`, `EvaluateDegradation`, `median`, lectura del entorno |
| `backend/internal/ingestion/notifier.go` | `Notifier` y `HTTPNotifier` contra `POST /notifications/admins` |
| `backend/internal/ingestion/handler.go` | Validación de contadores, evaluación al cerrar y aviso tolerante a fallos |
| `backend/internal/ingestion/repository.go` | `RecentFound`: `records_found` de las últimas corridas exitosas de la tienda |
| `backend/internal/features/reviews_watch_notifications.go` | `CreateAdminNotification` |
| `backend/internal/features/router.go` | Ruta `POST /notifications/admins` |
| `backend/internal/http/router.go` | Ingestion recibe `NewHTTPNotifier()` |
| `scraper/src/scraper/ingestion.py` | `finish_run(..., counters=...)` con los nombres del contrato |
| `scraper/src/scraper/pipeline.py` | Cuenta `itemsRejected` y adjunta los cuatro contadores al cerrar |

## 7. Verificación

```text
$ go build ./...            → sin errores
$ go vet ./...              → sin observaciones
$ go test ./... -count=1    → 18 paquetes ok, 0 FAIL
$ pytest                    → 152 passed (150 en develop + 2 nuevos)
$ ruff check src tests      → All checks passed!
```

Pruebas nuevas:

| Archivo | Qué fija |
| --- | --- |
| `backend/internal/ingestion/degrade_test.go` | Cada umbral, las razones acumuladas, la mediana con par/impar/vacío y la lectura del entorno (válida e inválida) |
| `backend/internal/ingestion/handler_test.go` | Aviso cuando hay degradación, silencio en corrida sana, payload viejo sin contadores, 422 por contador inválido, 200 aunque notification falle y sin tienda |
| `scraper/tests/test_ingestion.py` | El cierre envía los cuatro contadores y no cambia el cuerpo cuando no se pasan |
| `scraper/tests/test_e2e_ingestion.py` | El pipeline real adjunta `itemsFound/itemsSent/itemsRejected/pagesFailed` |

## 8. Límites declarados

- **No se guarda un estado nuevo.** `scrape_run.status` tiene un `CHECK` con
  cuatro valores; la degradación se evalúa al cerrar y vive en el aviso, no en
  la fila. Agregar un estado exigiría migración y otra tarea.
- **El canal es la base.** Un webhook a Discord/Slack sería una segunda fuente
  de fallo; si se pide, se agrega sobre el mismo `Notifier`.
- **El orden de despliegue importa.** `httpx.DecodeJSON` rechaza campos
  desconocidos: el backend debe ir antes o junto al scraper. En el clúster salen
  en la misma fase, así que no hay ventana.
- **No notifica si la fuente no tiene tienda**: no hay historial que comparar y
  la corrida se cierra igual.
