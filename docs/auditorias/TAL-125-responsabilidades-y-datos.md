# TAL-125 — Separar responsabilidades y datos entre las APIs

| Campo | Valor |
| --- | --- |
| Tarea | TAL-125: Corregir los hallazgos de TAL-105 y TAL-95 sobre quién valida la sesión, quién es dueño de cada dato y el código sin uso |
| Fecha | 2026-11-01 |
| Código | rama `feature/TAL-125-separar-responsabilidades` |
| Despliegue | clúster UCT, topología `dominios`, imágenes `soloservis-backend:bf2a897` y `soloservis-frontend:bf2a897` |
| Antes | [TAL-105](TAL-105-rutas-y-dependencias.md) (hallazgos H1–H10), [TAL-95](TAL-95-inventario-despliegue.md) (D1–D4), [TAL-133](TAL-133-integracion-postgres.md) (pruebas con PostgreSQL) |

---

## 1. Resumen

1. **Cada usuario solo lee y modifica lo suyo (H1).** review, watchlist, notification y las comparaciones de servicios confiaban en el `userId` que manda el cliente: bastaba con cambiarlo para leer o borrar los datos de otro. Ahora el usuario sale del token de sesión, como ya hacían cart y favorites.
2. **Crear notificaciones es solo interno (H2).** `POST /notifications` no pedía token. Ahora exige `INTERNAL_API_TOKEN` (lo envían pricing e ingestion) o una sesión admin.
3. **Una sola forma de validar la sesión (H4):** `authz.RequireSession`, en las 14 APIs.
4. **Un solo formato de error y de log (H3):** las 6 APIs de features usan los middlewares TAL-78 y responden `problem+json`, como las otras 8.
5. **Cada dato tiene un dueño documentado (H5)**, `/health` dice qué pod contesta (H6/D1), se borró el código sin uso (H7) y todos los `main` arrancan con `runner.Run` (H10).
6. **Resultado:** los tests con PostgreSQL pasan, incluido uno nuevo que intenta acceder a los datos de otro usuario. Con el código anterior ese test falla. En el clúster, el pod de test pasó **29 de 29** verificaciones.

---

## 2. H1 y H2: el usuario sale del token

**Antes:** el handler leía el usuario de lo que mandaba el cliente.

```
GET /watchlist?userId=7          → la watchlist del usuario 7, sin token
DELETE /reviews/product:12?userId=7 → borra la reseña 12 del usuario 7
GET /comparisons/services/3      → la comparación 3, sea de quien sea
```

**Ahora:** la ruta exige `Authorization: Bearer <token>`. `authz.RequireSession` busca el hash del token en `user_session` y deja el usuario en el contexto, y el handler usa ese usuario (`features.sessionUserID`). El `userId` del body o del query se ignora. Los inputs lo siguen aceptando porque el frontend lo envía y el decoder rechaza campos desconocidos.

| Ruta | Antes | Ahora |
| --- | --- | --- |
| `GET /reviews/{product,store,service}/{id}` | pública | pública (sin cambios) |
| `POST /reviews/...`, `PUT /reviews/{id}`, `DELETE /reviews/{id}` | `userId` del cliente | sesión; el autor sale del token; la reseña de otro → 404 |
| `GET/POST/PUT/DELETE /watchlist…` | `userId` del cliente | sesión |
| `GET /notifications`, `/unread`, `PUT …/read`, `/read-all` | `userId` del cliente | sesión |
| `POST /notifications` | sin protección | `ADMIN`: `INTERNAL_API_TOKEN` o sesión admin; usuario común → 403 |
| `/comparisons/services` (5 rutas) | `userId` del cliente al crear; leer y borrar no revisaban el dueño | sesión, y cada consulta filtra `user_id`; la comparación de otro → 404 |

Responder **404** (no 403) a lo que es de otro usuario no revela si el recurso existe.

**Prueba:** `TestIntegracion_AccesoCruzado` registra a Ana y a Beto. Beto, con su token y mandando `userId` de Ana, intenta leer, editar o borrar la reseña, la watchlist, las notificaciones y la comparación de servicios de Ana. Todo responde 404 o una lista vacía, y la base no cambia. Con el código anterior el mismo test falla: la reseña de Ana quedó a nombre de Beto y Beto podía modificarla.

**Frontend:** no usa watchlist ni notifications, y las comparaciones de servicios ya enviaban el token. No cambió ninguna pantalla.

---

## 3. H4: una sola validación de sesión

| Antes | Dónde | Ahora |
| --- | --- | --- |
| `authz.RequireSession` | comparison (productos) | sin cambios |
| `features.requireSession` (copia, con otro formato de error) | favorites, cart, search (historial) | `authz.RequireSession` |
| Validación a mano dentro del handler | `GET /users/{id}` | `authz.RequireSession` + el id pedido debe ser el del token (otro → 404) |

Comercial sigue leyendo `user_session` y `user_account` para validar el token: es la única dependencia de datos entre dominios, y es de solo lectura. Si cae el pod `usuarios`, las sesiones abiertas siguen funcionando en `comercial`.

---

## 4. H3: un solo formato de error

Las 6 APIs de features (auth, favorites, cart, review, watchlist, notification) ahora tienen los mismos middlewares que las otras 8: `RequestID → AccessLog → Recover`. Responden los errores en `problem+json` (RFC 9457):

```json
{"type":"about:blank","title":"Unauthorized","status":401,"detail":"Se requiere un token de acceso válido (Authorization: Bearer <token>).","instance":"/watchlist","code":"unauthorized","request_id":"3f2c…"}
```

| Status | `code` |
| --- | --- |
| 400 | `invalid_parameter` |
| 401 | `unauthorized` |
| 403 | `forbidden` |
| 404 | `not_found` |
| 409 | `conflict` |
| 503 (base caída) | `service_unavailable` |
| 500 | `internal_error`; el error real queda en el log con el `request_id` |

El texto de cada mensaje no cambió; cambió el formato. El frontend lee ahora `detail` (`fronted/src/services/api/personalization.ts`), así que mensajes como "invalid credentials" o "email is already registered" se siguen mostrando.

---

## 5. H5: dueño de cada columna de `product_user_interest`

La tabla tiene una fila por (usuario, producto) y la comparten dos APIs. Se revisó que ninguna borra la fila de la otra: cada una solo apaga su marca con `UPDATE … = FALSE`. Quitar un favorito no borra la watchlist ni al revés.

| Columna | Dueña | Quién más la lee |
| --- | --- | --- |
| `is_favorite` | favorites-api | — |
| `target_price` | watchlist-api | pricing e ingestion, para avisar `price_drop` |
| `notify_price_drop` | watchlist-api | pricing e ingestion |

La migración nueva `016_product_user_interest_owners.sql` lo deja escrito en la base con `COMMENT ON` (se ve con `\d+ product_user_interest`). No cambia datos ni estructura. No se separó la tabla porque eso obliga a migrar datos y no resuelve un error real.

---

## 6. Limpieza (H6, H7, H10, D2, D4)

| # | Antes | Ahora |
| --- | --- | --- |
| H6 / D1 | `/health` de comercial e ingestion decía `"service":"soloservis-api"` | cada API dice su nombre (`product-api`, …) y cada pod su dominio: `comercial`, `usuarios`, `ingestion` (`all-apis` en el consolidado) |
| H7 | `cmd/server`, `cmd/products-api`, `cmd/stores-api`, `cmd/services-api`, `internal/server`, `config.LoadForPort`, `docker/api/Dockerfile` sin uso | borrados |
| H7 | el compose pasaba `PRODUCT_API_URL`/`PRICING_API_URL` a search y comparison, que no llaman por HTTP | quitadas; además `notification-api` recibe `INTERNAL_API_TOKEN` para validar H2 |
| H10 | product, store, service y user tenían su propio `main` (copia de `runner.Run`) | los 14 `main` usan `runner.Run` |
| D2 | `FRONTEND_URL` = `http://…` aunque la app se sirve por `https://…` | el deploy usa `https` por defecto (`-IngressScheme`) |
| D4 | el log de arranque decía `running on http://localhost:8080` | `Dominio comercial escuchando en :8080` |

---

## 7. Resultados

### Tests (PostgreSQL 18 en Docker, 21 migraciones)

```
TestIntegracion_Usuarios                      PASS
TestIntegracion_Comercial                     PASS  (8 subtests, ahora con sesión en review, watchlist y notification)
TestIntegracion_IngestionNotificaAComercial   PASS  (ingestion → notification con INTERNAL_API_TOKEN)
TestIntegracion_AccesoCruzado                 PASS  (4 subtests: review, watchlist, notification, comparación de servicios)
TestIntegracion_HealthPorDominio              PASS
TestBaseCaida_CadaAPIResponde503              PASS  (26 rutas, 14 APIs)
TestBaseCaida_ProblemJSON                     PASS  (estándar y features: 503, code y request_id)
```

`TEST_DATABASE_URL=... go test ./... -p 1`: los 12 paquetes con tests pasan. Frontend: formato, lint y build sin errores.

### Clúster

Re-despliegue del 2026-11-01 con `deploy.ps1 -Modo uct -FromPhase 3 -SkipCleanup` (sin borrar datos):

| Verificación | Resultado |
| --- | --- |
| Migraciones | 21 de 21 en `schema_migrations`; los comentarios de la 016 están en `product_user_interest` |
| Pods | `comercial`, `usuarios`, `ingestion` y `frontend` con la imagen `bf2a897`, `Running`, 0 reinicios |
| `/health` de cada pod | `{"status":"ok","service":"comercial","database":"connected"}` (y `usuarios`, `ingestion`) |
| Log de arranque | `Dominio comercial escuchando en :8080` |
| `FRONTEND_URL` | `https://soloservis-marcelo-vidal.dev.censei.cl` |
| `INTERNAL_API_TOKEN` en `comercial` (valida `POST /notifications`) e `ingestion` (lo envía) | definido en ambos (Secret `soloservis-internal`) |
| Pod de test (Fase 10) | **29 OK, 0 fallas**: las 10 rutas protegidas responden 401 sin token, aunque manden `userId` |
| Fase 9 (resiliencia) | `postgres-0` reiniciado; los datos persisten (21 migraciones) |
| Desde internet: `GET $BASE/watchlist?userId=1` | 401 `problem+json` con `request_id` |
| Desde internet: `POST $BASE/reviews/product/{id}` sin token | 401 |

---

## 8. Cómo repetirlo

```bash
cd backend && TEST_DATABASE_URL=postgres://postgres:postgres@localhost:55432/soloservis go test ./internal/integration/ -run 'AccesoCruzado|HealthPorDominio' -v
```

```powershell
.\scripts\k8s\deploy.ps1 -Modo uct -Registry docker.io/<usuario-docker> -FromPhase 3 -SkipCleanup
```

```bash
kubectl logs job/soloservis-smoke-test
```

La base de prueba se arma como en [TAL-133 §7](TAL-133-integracion-postgres.md#7-cómo-repetirlo).
