# TAL-105 — Rutas y dependencias de los microservicios

| Campo | Valor |
| --- | --- |
| Tarea | TAL-105: Auditar routers, endpoints, middlewares, puertos, variables de entorno, conexiones a PostgreSQL y llamadas entre las APIs |
| Fecha | 2026-10-31 |
| Código auditado | `develop` @ `3e5e856` |
| Tipo | Solo lectura y documentación. No se modificó código. |
| Parte de | [TAL-95](TAL-95-inventario-microservicios.md), que ya cubre puertos, variables de entorno, arranque y health |

**Cómo se obtuvo el inventario de rutas:** se recorrió el router real de cada API con `chi.Walk`, la misma técnica del test `backend/internal/allapis/router_test.go`. Salen **101 rutas** en las 14 APIs: 87 de negocio y 14 `/health`.

---

## 1. Resumen

1. **Dos cadenas de middlewares.** Las 8 APIs del estándar TAL-78 aplican `RequestID → AccessLog → Recover → CORS`. Las 6 de `internal/features` (auth, cart, favorites, review, watchlist, notification) solo aplican CORS: no tienen request id, ni log de accesos, ni registro de los panics, y sus errores son `{"error": "..."}` en lugar de `problem+json`.
2. **Tres formas de validar la sesión** que hacen lo mismo: `authz.RequireSession`, `features.requireSession` y una consulta propia en user-api. Las tres leen `user_session` y `user_account` directamente. **Ninguna API llama a auth por HTTP.**
3. **🔴 Hallazgo de seguridad:** review, watchlist, notification y las comparaciones de servicios **identifican al usuario por un `userId` que manda el cliente**, sin validar el token. Cualquiera puede crear reseñas a nombre de otro, o leer y modificar la watchlist de otro. Review, watchlist y comparaciones son **públicas** en el nginx.
4. **Solo hay 8 llamadas HTTP entre APIs,** todas para verificar que algo existe o para notificar. Si una falla, la API responde **503** sin escribir nada. La excepción es la notificación, que se registra como warning y no frena la operación.
5. **El acoplamiento real es por datos:** varias APIs escriben las mismas tablas (`product_user_interest`, `product_offer`, `product`). Es el insumo de TAL-125.

---

## 2. Middlewares

| Cadena | APIs | Orden | Dónde |
| --- | --- | --- | --- |
| Estándar TAL-78 | product, store, service, user, pricing, search, comparison, ingestion | `httpx.RequestID` → `httpx.AccessLog` → `httpx.Recover` → CORS | `backend/internal/http/router.go:96-104`, `:132-136` |
| features | auth, cart, favorites, review, watchlist, notification | solo CORS | `backend/internal/features/router.go:10-14`, `:107` |

**Autorización por ruta:**

| Guarda | Qué exige | Dónde | Respuesta si falla |
| --- | --- | --- | --- |
| `ADMIN` = `authz.RequireAdmin` | sesión con rol `admin` **o** `INTERNAL_API_TOKEN` (comparación en tiempo constante, mínimo 32 caracteres) | `backend/internal/authz/authz.go:81-89` | 401 / 403 `problem+json` |
| `SESIÓN` = `authz.RequireSession` | token de acceso vigente; el usuario sale de la sesión | `authz.go:115` | 401 `problem+json` |
| `SESIÓN*` = `features.requireSession` | lo mismo, en el código de features | `backend/internal/features/handler.go:24-45` | 401 `{"error"}` |
| `userId` | **ninguna**: usa el `userId` del body o del query | `features/reviews_watch_notifications.go`, `features/service_personalization.go` | — |

Cómo se guarda la sesión (`backend/internal/features/auth.go`):
- Contraseña con PBKDF2-SHA256, 120.000 iteraciones y salt aleatorio (`:16`, `:161-165`).
- Al iniciar sesión se crean dos tokens aleatorios. En `user_session` solo se guarda su **hash SHA-256** (`:154-155`), nunca el token.
- Vigencia: acceso **15 minutos**, refresco **30 días** (`:80-81`).

---

## 3. Endpoints

Columna **Auth**: `—` = pública · `SESIÓN` / `SESIÓN*` / `ADMIN` = ver la sección 2 · `🔴 userId` = confía en el `userId` del cliente.
Columna **Exp.**: `pub` = la publica el nginx · `int` = solo desde dentro del clúster.

### product · 8081 · estándar · pub (solo GET)

| Método | Ruta | Auth |
| --- | --- | --- |
| GET | `/products` · `/products/{publicID}` · `/products/{publicID}/reviews` | — |
| POST · PUT · DELETE | `/products` · `/products/{publicID}` | ADMIN |
| GET | `/categories` · `/categories/{slug}` · `/product-categories` (transitoria, TAL-51) | — |

### store · 8082 · estándar · pub

| GET | `/stores` · `/stores/offers` · `/stores/{id}` · `/stores/{id}/offers` | — |
| --- | --- | --- |

### service · 8083 · estándar · pub

| GET | `/services` · `/services/{publicID}` | — |
| --- | --- | --- |

### user · 8084 · estándar · int

| Método | Ruta | Auth |
| --- | --- | --- |
| GET | `/users` | — (responde un texto fijo, `user-api-ready`; no lista usuarios) |
| GET | `/users/{id}` | token propio: el token debe ser del mismo usuario (`router.go:253-274`) |

### pricing · 8085 · estándar · int

| Método | Ruta | Auth |
| --- | --- | --- |
| GET | `/offers/{id}` · `/offers/{id}/price-history` · `/offers/product/{productId}` · `…/best` · `…/price-history` | — |
| POST · PUT | `/offers` · `/offers/{id}` | ADMIN |

### search · 8086 · estándar · pub

| Método | Ruta | Auth |
| --- | --- | --- |
| GET | `/search/products` · `/search/facets` · `/search/categories` · `/search/brands` | — |
| GET · POST | `/search/history` · `/search/saved` | SESIÓN* |
| DELETE | `/search/saved/{id}` | SESIÓN* |

### comparison · 8087 · estándar · pub

| Método | Ruta | Auth |
| --- | --- | --- |
| GET | `/comparisons/products/compare` · `/comparisons/compatible/{productId}` | — |
| GET · POST · PUT · DELETE | `/comparisons/products[/{id}[/items[/{productId}]]]` (6 rutas) | SESIÓN |
| GET · POST · DELETE | `/comparisons/services[/{id}[/items[/{serviceId}]]]` (5 rutas) | 🔴 userId al crear; leer, editar y borrar solo piden el `id` de la comparación, sin dueño (`features/service_personalization.go:144-150`) |

### auth · 8088 · features · pub (GET y POST)

| POST | `/auth/register` · `/auth/login` · `/auth/refresh` · `/auth/logout` | — |
| --- | --- | --- |
| GET | `/auth/validate` | token en el header |

### favorites · 8089 · features · pub

| GET · POST · DELETE | `/favorites` · `/favorites/{productId}` · `/favorites/services[/{serviceId}]` (7 rutas) | SESIÓN* |
| --- | --- | --- |

### cart · 8090 · features · pub

| GET · POST · PUT · DELETE | `/cart` · `/cart/count` · `/cart/items[/{id}]` (6 rutas) | SESIÓN* |
| --- | --- | --- |

### review · 8091 · features · pub

| Método | Ruta | Auth |
| --- | --- | --- |
| GET | `/reviews/product/{id}` · `/reviews/store/{id}` · `/reviews/service/{id}` | — |
| POST · PUT · DELETE | `/reviews/product/{id}` · `/reviews/store/{id}` · `/reviews/service/{id}` · `/reviews/{id}` | 🔴 userId |

### watchlist · 8092 · features · pub

| GET · POST · PUT · DELETE | `/watchlist` · `/watchlist/{productId}` (4 rutas) | 🔴 userId |
| --- | --- | --- |

### notification · 8093 · features · int

| Método | Ruta | Auth |
| --- | --- | --- |
| POST | `/notifications` | ninguna (pricing e ingestion envían el token, pero no se valida) |
| GET · PUT | `/notifications` · `/notifications/unread` · `/notifications/{id}/read` · `/notifications/read-all` | 🔴 userId |

### ingestion · 8094 · estándar · int

| POST · GET | `/ingestion/runs` · `/ingestion/runs/{id}` · `/ingestion/runs/{id}/items` · `/ingestion/runs/{id}/finish` | ADMIN |
| --- | --- | --- |

---

## 4. Conexiones a PostgreSQL

- **Una base para todas** (`DATABASE_URL`). Cada proceso abre un `pgxpool` de hasta **10 conexiones** (`backend/internal/database/postgres.go:17`) y hace ping al arrancar. Si el ping falla, el proceso termina (`backend/internal/runner/runner.go:26`).
- Con 14 procesos son hasta 140 conexiones. En el modo consolidado de UCT es un solo pool de 10.
- Los valores que llegan del usuario van siempre como parámetros (`$1, $2…`). Lo único que se arma concatenando texto son constantes del propio código y dos identificadores elegidos de una lista fija: la tabla de reseñas (`features/reviews_watch_notifications.go:166`) y el orden de la búsqueda (`search/repository.go:85,137`). No hay riesgo de inyección SQL.

Tablas por API: **E** = escribe, L = solo lee.

| API | Tablas propias (E) | Tablas de otros dominios (L) |
| --- | --- | --- |
| product | `product`, `product_attribute_value`, `product_image` | `product_category`, `categories`, `category_attributes`, `brand`, `store`, `product_offer`, `product_rating_summary` |
| store | — | `store`, `product`, `product_offer`, `store_review` |
| service | — | `service`, `service_category`, `service_offer`, `provider` (sqlc) |
| user | — | `user_account`, `user_session` |
| pricing | `product_offer` (el historial `product_price_history` lo escribe un trigger, ver abajo) | `product`, `store`, `product_user_interest` (watchers), `product_price_history` |
| search | `search_query`, `saved_search` | catálogo completo (`product`, `brand`, `categories`, atributos, ofertas, imágenes) |
| comparison | `product_comparison(_item)`, `service_comparison(_item)` | catálogo de productos y `service` |
| auth | `user_account`, `user_session` | — |
| favorites | `product_user_interest` (`is_favorite`), `service_user_interest` | `product`, `service` |
| cart | `cart_item` | `product`, `product_offer`, `service`, `service_offer`, `store`, `provider` |
| review | `product_review`, `store_review`, `service_review` | `product`, `service` |
| watchlist | `product_user_interest` (precio objetivo) | `product` |
| notification | `notification` | — |
| ingestion | `scrape_run`, `scraped_data`, `data_source`, `scraper_config`, `brand` + **`product` y `product_offer`, con los repositorios de product y pricing** | `store` |

Además, **todas las que tienen rutas con sesión leen `user_session` y `user_account`**.

**El historial de precios lo escribe la base, no una API.** El trigger `product_offer_price_history_after_change` (`backend/database/migrations/010_product_price_history_on_price_change.sql:32-33`) inserta en `product_price_history` cada vez que se crea una oferta o cambia su precio, venga el cambio de pricing o de ingestion.

**Tablas que escriben varias APIs:**

| Tabla | La escriben | Riesgo |
| --- | --- | --- |
| `product_user_interest` | favorites (`is_favorite`) y watchlist (precio objetivo); pricing la lee para avisar | tres APIs dependen del mismo esquema |
| `product_offer` | pricing e ingestion (con el mismo repositorio) | bajo: el código es el mismo |
| `product` | product e ingestion (con el mismo repositorio) | bajo: el código es el mismo |
| `user_account` / `user_session` | auth; las demás las leen | cualquier cambio de sesión afecta a 10 APIs |

---

## 5. Llamadas entre APIs

Todas usan `INTERNAL_API_TOKEN` como Bearer si está definido. Timeouts: 4 s en features (`features/clients.go:14`) y 3 s en el notificador (`pricing/notifier.go:46`).

| Origen → destino | Endpoint llamado | Para qué | Si falla | Evidencia |
| --- | --- | --- | --- | --- |
| favorites → product | `GET /products/{id}` | verificar que el producto existe | 503, no guarda | `features/favorites_cart.go:41` |
| cart → product | `GET /products/{id}` | verificar el producto | 503 | `favorites_cart.go:157` |
| cart → pricing | `GET /offers/{id}` | la oferta debe ser de ese producto | 503, o 409 si no coincide | `favorites_cart.go:164-170` |
| review → product | `GET /products/{id}` | verificar el producto | 503 | `reviews_watch_notifications.go:41` |
| review → store | `GET /stores/{id}` | verificar la tienda | 503 | `reviews_watch_notifications.go:69` |
| review → service | `GET /services/{id}` | verificar el servicio | 503 | `reviews_watch_notifications.go:98` |
| watchlist → pricing | `GET /offers/product/{id}` | el producto debe tener ofertas | 503 | `reviews_watch_notifications.go:199`, `:225` |
| pricing, ingestion → notification | `POST /notifications` | avisar bajas de precio a quienes lo siguen | warning en el log; **la oferta se guarda igual** | `pricing/handler.go:232-252`, `ingestion/processor.go:136` |

Ninguna otra API llama por HTTP. Search y comparison reciben `*_API_URL` en compose, pero no las usan (ver TAL-95 §4).

```
favorites ─┐
cart ──────┼──► product
review ────┤
           ├──► store, service   (solo review)
cart ──────┴──► pricing ◄── watchlist
                pricing ───► notification ◄─── ingestion
```

---

## 6. Recorrido de los datos (para explicar la app)

**Leer el catálogo** (`GET /products`)
`navegador → nginx del frontend → product → PostgreSQL`. product lee `product` con sus ofertas, imágenes y atributos y responde JSON. No interviene ninguna otra API.

**Iniciar sesión** (`POST /auth/login`)
1. auth busca el usuario por email y verifica la contraseña PBKDF2.
2. Crea un token de acceso (15 min) y uno de refresco (30 días), y guarda **solo sus hashes** en `user_session`.
3. El frontend envía el token en `Authorization: Bearer …`. Cada API con sesión lo hashea y lo busca en `user_session`.

**Agregar al carrito** (`POST /cart/items`)
1. cart valida la sesión leyendo `user_session`.
2. Pregunta a product si el producto existe.
3. Pregunta a pricing si la oferta corresponde a ese producto.
4. Recién entonces inserta en `cart_item`. Si product o pricing no responden, devuelve 503 y no guarda nada.

**Ingesta del scraper** (`POST /ingestion/runs/{id}/items`)
1. El scraper se autentica con `INTERNAL_API_TOKEN`.
2. ingestion guarda el dato crudo en `scraped_data`.
3. Busca el producto por SKU, o por marca y modelo. Si no existe, lo crea con el repositorio de product; si existe, **no lo modifica**, porque el catálogo lo cura el admin. Después crea o actualiza la `product_offer` con el repositorio de pricing (`ingestion/processor.go:101-126`), y el trigger de la base registra el precio en `product_price_history`.
4. Si el precio bajó, busca en `product_user_interest` quién sigue ese producto y llama a notification, que inserta en `notification`.

---

## 7. Hallazgos y a qué tarea van

| # | Severidad | Hallazgo | Propuesta | Tarea |
| --- | --- | --- | --- | --- |
| H1 | 🔴 alta | review, watchlist, notification y comparaciones de servicios confían en el `userId` del cliente; las comparaciones de servicios ni siquiera revisan el dueño al leer o borrar | protegerlas con sesión y sacar el usuario del token, como ya hacen cart y favorites (`normalizeUserID`) | TAL-125 |
| H2 | 🟠 media | `POST /notifications` no exige token | protegerla con `ADMIN` (pricing e ingestion ya envían el token) | TAL-125 |
| H3 | 🟠 media | las 6 APIs de features no tienen los middlewares TAL-78 ni `problem+json` | montarlas con `useStandardMiddleware` | TAL-125 |
| H4 | 🟡 baja | 3 implementaciones de validación de sesión | usar solo `authz.RequireSession` | TAL-125 |
| H5 | 🟡 baja | `product_user_interest` la escriben favorites y watchlist | documentar quién es dueño de cada columna o separar la tabla | TAL-125 |
| H6 | 🟡 baja | `/health` del estándar dice `soloservis-api` en las 8 APIs | poner el nombre de cada API | TAL-125 |
| H7 | ⚪ info | variables `*_API_URL` sin uso en search y comparison; binarios antiguos sin uso | limpiar | TAL-125 |
| H8 | ⚪ info | las llamadas HTTP solo cruzan dentro de comercial, y de ingestion a notification | confirma la agrupación de TAL-106 y TAL-107 | TAL-106/107 |
