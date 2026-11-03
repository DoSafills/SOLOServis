# TAL-133 — Integración de las APIs con PostgreSQL

| Campo | Valor |
| --- | --- |
| Tarea | TAL-133: Validar que todas las APIs leen y escriben sus datos en PostgreSQL y manejan bien los errores de conexión |
| Fecha | 2026-11-01 |
| Base de prueba | PostgreSQL 18.6 en Docker, limpia, con las 20 migraciones de `migration_order.txt` |
| Código | rama `feature/TAL-133-integracion-postgres` |
| Tests | `backend/internal/integration/` (`postgres_test.go` y `unavailable_test.go`) |

---

## 1. Resumen

1. **Las 14 APIs leen y escriben sus tablas.** Los tests levantan los 3 dominios (`comercial`, `usuarios`, `ingestion`) como servidores HTTP reales sobre la misma base, igual que los pods de Kubernetes. Para cada API verifican la respuesta HTTP **y la fila en PostgreSQL**.
2. **Las llamadas entre APIs viajan por HTTP de verdad:** cart → product y pricing, review → product, watchlist → pricing, y la única que cruza de dominio, ingestion → notification.
3. **Se corrigieron 2 defectos** que estos tests encontraron:
   - **Con la base caída, varias APIs respondían códigos engañosos:** 401 (el frontend cierra la sesión), 404, 409 ("el email ya está registrado") o 400. Ahora las 14 responden **503**.
   - **La sesión nacía vencida** si el proceso Go no estaba en UTC: el vencimiento se calculaba con la hora local de Go y se guardaba en una columna `TIMESTAMP` sin zona. Ahora lo calcula la base con `NOW()`.
4. **Resultado:** `go test ./...` con `TEST_DATABASE_URL` pasa en los 12 paquetes; los tests no dejan datos en la base.

---

## 2. Cómo se prueba

`newEnv` (en `postgres_test.go`) arma el mismo despliegue que el clúster:

- Un `httptest.Server` por dominio, con `allapis.NewDomainRouter`: cada uno sirve solo las rutas de su dominio.
- Las variables `*_API_URL` las resuelve `allapis.InternalURLs`, como `cmd/all-apis`: las APIs llamadas viven en `comercial`.
- Un solo pool de PostgreSQL compartido, como los 3 pods comparten el Service `postgres`.
- Cada test crea sus datos con un sufijo único (`tal133-<nanos>`) y los borra al terminar: los usuarios con `ON DELETE CASCADE` (sesiones, favoritos, carrito, reseñas, watchlist, notificaciones) y las tiendas con sus ofertas e historial.

---

## 3. Lectura y escritura por API

| Dominio | API | Operación probada | Tabla verificada | Llamada HTTP a otra API |
| --- | --- | --- | --- | --- |
| usuarios | auth | register, login (bien y mal), email duplicado → 409, validate, refresh (rota la sesión), logout | `user_account` (contraseña PBKDF2, nunca en claro), `user_session` (solo hashes SHA-256 de los tokens) | — |
| usuarios | user | `GET /users/{id}` con el token del mismo usuario; sin token → 401 | `user_account`, `user_session` | — |
| comercial | product | `GET /products/{id}` | `product` | — |
| comercial | store | `GET /stores/{id}` | `store` | — |
| comercial | service | `GET /services` | `service` | — |
| comercial | search | `GET /search/products`; `POST /search/history` con sesión | `search_query` | — |
| comercial | comparison | `GET /comparisons/compatible/{id}` | `product` | — |
| comercial | favorites | agregar, listar y quitar con sesión; sin sesión → 401 | `product_user_interest.is_favorite` | → product |
| comercial | cart | agregar 2 unidades, contar, vaciar | `cart_item` | → product y → pricing |
| comercial | review | crear, listar y borrar | `product_review` | → product |
| comercial | watchlist | agregar con precio objetivo y listar | `product_user_interest.target_price` | → pricing |
| comercial | notification | crear, listar y marcar todas como leídas | `notification.read_at` | — |
| comercial | pricing | crear oferta y cambiar el precio con `INTERNAL_API_TOKEN`; sin token → 401 | `product_offer`, y 2 filas en `product_price_history` que escribe el trigger | — |
| ingestion | ingestion | abrir ejecución, 2 lotes (600.000 y luego 450.000) y cerrar; sin token → 401 | `scrape_run`, `scraped_data`, `data_source`, `product_offer`, `product_price_history` | → notification (comercial) |

**Recorrido ingestion → notification** (`TestIntegracion_IngestionNotificaAComercial`):
1. Un usuario pone en su watchlist el refrigerador LG GM53 con objetivo de 500.000.
2. El scraper informa una oferta nueva a 600.000: no hay precio anterior, no se avisa (0 notificaciones).
3. El scraper informa 450.000: el precio cruza el objetivo, ingestion llama por HTTP a `POST /notifications` de comercial y queda 1 notificación `price_drop` para ese usuario.

---

## 4. Errores de conexión

`TestBaseCaida_CadaAPIResponde503` arma los 3 dominios con un pool que apunta a un puerto cerrado y prueba 24 rutas que cubren las 14 APIs, `/health` incluido. Exige 503, que no haya pánico y que la respuesta no muestre el error interno (`127.0.0.1`, `dial`, `failed to connect`). El error real sí queda en el log del servidor, con su `request_id`.

**Antes y después** (el mismo test contra el código anterior):

| Ruta con la base caída | Antes | Ahora | Por qué importaba |
| --- | --- | --- | --- |
| `GET /favorites`, `GET /cart` (con sesión) | 401 | 503 | el frontend interpreta 401 como sesión vencida y cierra la sesión |
| `POST /auth/login`, `GET /auth/validate` | 401 | 503 | ídem: "credenciales inválidas" con la base caída |
| `POST /auth/register` | 409 | 503 | decía "el email ya está registrado" |
| `GET /users/{id}`, `GET /reviews/product/{id}`, `GET /services/{id}` | 404 | 503 | decía que el recurso no existe |
| `POST /notifications` | 400 | 503 | culpaba al cliente |
| APIs del estándar TAL-78 (`/products`, `/stores`, `/offers`, `/search`, `/ingestion/runs`, ...) | 500 | 503 `problem+json`, `code: "service_unavailable"` | distingue "base no disponible" de un bug |
| `GET /services` | 500 en texto plano | 503 `problem+json` | ahora usa el formato del estándar |

**Cómo se detecta** (`httpx.IsUnavailable`): pgx envuelve el error de red en `pgconn.ConnectError` sin ocultarlo, así que basta con `errors.As(err, *net.Error)` (conexión rechazada, DNS, red cortada) o `context.DeadlineExceeded`. Así `httpx` sigue dependiendo solo de la biblioteca estándar.

| Familia | Dónde | Respuesta con la base caída |
| --- | --- | --- |
| Estándar TAL-78 | `httpx.WriteInternal` (un solo lugar) | 503 `problem+json`, `code: "service_unavailable"` |
| features (auth, cart, favorites, review, watchlist, notification, búsquedas guardadas) | `features.dbError` en 50 lugares | al cerrar TAL-133: 503 `{"error":"database unavailable"}`. Desde TAL-125 (H3): 503 `problem+json`, `code: "service_unavailable"`, como las demás |
| services | `httpx.WriteInternal` | 503 `problem+json` |
| user | `httpx.WriteInternal` | 503 `problem+json` |

Fuera de la base caída nada cambia: un error que no es de conexión sigue respondiendo lo mismo que antes (404 si no existe, 409 si el email está repetido, 500 si es un bug).

**Recuperación:** cuando la base vuelve, el pool abre conexiones nuevas solo. Se comprobó en el clúster (Fase 9 del deploy, TAL-103): al reiniciar `postgres-0`, los 3 pods responden 503 en `/health`, salen de rotación sin reiniciarse y vuelven solos.

---

## 5. Defecto de zona horaria en las sesiones

`issueTokens` (`internal/features/auth.go`) calculaba el vencimiento con `time.Now()` en Go y lo guardaba en `user_session.access_expires_at`, de tipo `TIMESTAMP` sin zona. pgx guarda la hora local del proceso; la validación compara con `NOW()` de la base, que está en UTC. Con Go en -03:00 el token nacía vencido hace 3 horas: **login y registro devolvían un token que nunca servía**.

En el clúster no se notaba porque los pods y la base están en UTC, pero fallaba al correr el backend con `go run` en Windows contra la base de Docker. Ahora el `INSERT` usa `NOW() + INTERVAL '15 minutes'` y `NOW() + INTERVAL '30 days'`: el mismo reloj que valida.

---

## 6. Resultados

```
TestIntegracion_Usuarios                            PASS
TestIntegracion_Comercial                           PASS  (8 subtests)
TestIntegracion_IngestionNotificaAComercial         PASS
TestBaseCaida_CadaAPIResponde503                    PASS  (24 rutas, 14 APIs)
TestBaseCaida_ProblemJSON                           PASS
```

`TEST_DATABASE_URL=... go test ./... -p 1`: los 12 paquetes con tests pasan (`allapis`, `authz`, `categories`, `comparison`, `http`, `httpx`, `ingestion`, `integration`, `pricing`, `products`, `search`, `stores`). Después de la corrida no queda ningún usuario, tienda, fuente ni búsqueda de prueba en la base.

---

## 7. Cómo repetirlo

```bash
docker run -d --name soloservis-test-pg -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=soloservis -p 55432:5432 postgres:18
```

```bash
cd backend && for f in $(tr -d '\r' < database/migration_order.txt | grep '\.sql$'); do docker exec -i soloservis-test-pg psql -q -v ON_ERROR_STOP=1 -U postgres -d soloservis < database/migrations/$f; done
```

```bash
cd backend && TEST_DATABASE_URL=postgres://postgres:postgres@localhost:55432/soloservis go test ./internal/integration/ -v
```

Los tests de errores de conexión (`TestBaseCaida_*`) no necesitan Docker: corren siempre con `go test ./...`.

---

## 8. Pendiente

Nada. Ambos pendientes se cerraron el 2026-11-01:
- El clúster UCT corre estas correcciones desde el re-despliegue de TAL-125 (imagen `bf2a897`, 21 migraciones, pod de test 29 de 29).
- Las 6 APIs de features responden `problem+json` desde TAL-125 (H3). Los tests de este documento se actualizaron con TAL-125: review, watchlist y notifications ahora con sesión, y `TestBaseCaida_*` cubre 26 rutas y exige `request_id` también en features. Ver [TAL-125](TAL-125-responsabilidades-y-datos.md).
