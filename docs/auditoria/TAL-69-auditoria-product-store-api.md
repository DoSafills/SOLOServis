# TAL-69 — Auditoría de Product API y Store API

| Campo | Valor |
| --- | --- |
| Tarea | TAL-69: Auditar Product API y Store API (endpoints reales, operaciones incompletas, dependencias) |
| Fecha | 2026-10-18 |
| Código auditado | `origin/feature/platform-apis-and-product-comparison` @ `2738771` (2026-10-07) |
| Tipo | Solo lectura y documentación. No se modificó código. |

> **Importante: base auditada.** `develop` (`3b4018b`, 2026-10-02) **no** contiene `k8s/`, `scripts/k8s/deploy.ps1`, `docker/frontend/nginx.conf`, `cmd/product-api` ni `cmd/store-api` y solo tiene migraciones hasta la 005. `develop` y la rama auditada divergen en 14/20 commits. Por acuerdo con el solicitante, se auditó la rama `feature/platform-apis-and-product-comparison`, que es la que coincide con el contexto conocido. **Todas las referencias `archivo:línea` de este informe corresponden al commit `2738771`, no a `develop`.**

---

## 1. Resumen ejecutivo

1. Las dos APIs son binarios Go separados (`cmd/product-api`, `cmd/store-api`) dentro del **mismo módulo** `backend/` (chi v5 + pgx v5 + sqlc) y comparten **una sola base PostgreSQL**. No hay llamadas HTTP entre ellas.
2. **Product API**: 6 rutas (health, list, detail, create, update, soft-delete). La lectura funciona. La escritura está **incompleta**: no tiene autenticación, no gestiona atributos, todos los errores de BD se reportan como 404 o 500 y las respuestas de update/delete tienen otra forma (snake_case, con el ID interno).
3. **Store API**: 3 rutas, todas de solo lectura (health, list, detail). No hay CRUD de tiendas ni de ofertas. `reviewCount` siempre vale 0.
4. El esquema ya tiene **categorías jerárquicas y atributos dinámicos tipo EAV** (`product_category_specification` + `product_specification_value`), pero **no existe ningún endpoint para administrarlos ni validación** de tipos o de campos obligatorios.
5. Pricing, Search, Comparison e Ingestion **ya existen en el código** (`internal/features`), pero **no se despliegan en k8s**, sus endpoints de escritura no tienen autenticación y no tienen tests. TAL-74 a TAL-77 son más bien "adaptar" que "crear".
6. **Hay 0 tests** en todo el backend. `go build` y `go vet` pasan. Los formatos de error son inconsistentes (texto plano vs JSON) y los errores internos no se registran en ningún log.
7. Hay riesgos en las migraciones: los números 006, 007 y 008 están duplicados, el orden depende del locale y los archivos con `BEGIN/COMMIT` anulan el `--single-transaction` del runner.

---

## 2. Product API

### 2.1 Stack y estructura

| Aspecto | Valor | Evidencia |
| --- | --- | --- |
| Lenguaje | Go 1.27.1 | `backend/go.mod:3` |
| Router | `github.com/go-chi/chi/v5` v5.3.2 | `backend/go.mod:6`, `backend/internal/http/router.go:42-59` |
| Driver BD | `jackc/pgx/v5` (pool `pgxpool`, MaxConns 10) | `backend/go.mod:7`, `backend/internal/database/postgres.go:11-28` |
| Acceso a datos | sqlc v1.31.1 (código generado) + 1 SQL manual | `backend/sqlc.yaml:1-12`, `backend/internal/database/generated/db.go:3`, `backend/internal/products/repository.go:53-56` |
| Config | `godotenv` + env vars `PORT`, `DATABASE_URL`, `FRONTEND_URL` | `backend/internal/config/config.go:15-38` |
| Entrypoint | `cmd/product-api/main.go` → `apphttp.NewProductRouter` | `backend/cmd/product-api/main.go:30` |
| Puerto | 8081 (env `PORT`; si falta, default 8080) | `k8s/base/product-api/deployment.yaml:42-43`, `docker/compose.yml:28`, `backend/internal/config/config.go:19-21` |
| Capas | `handler.go` (HTTP) → `repository.go` → `generated/*.sql.go`; DTOs en `products/dto/` | `backend/internal/products/` |

Notas:
- `cmd/server/main.go` sigue existiendo: es un monolito que monta products, services y stores en un solo puerto (`backend/internal/http/router.go:138-168`). No se despliega en k8s ni en compose.
- `backend/src/database/*.ts` son restos de NestJS que no se usan desde Go.

### 2.2 Inventario de endpoints reales

Rutas registradas en `backend/internal/http/router.go:42-59`. Handlers en `backend/internal/products/handler.go`.

| Método | Ruta | Handler | Parámetros | Body | Respuesta | Códigos | Estado |
| --- | --- | --- | --- | --- | --- | --- | --- |
| GET | `/health` | `healthHandler` `router.go:170-191` | — | — | `{status, service:"soloservis-api", database}` | 200, 503 | Completo |
| GET | `/products` | `List` `handler.go:63-113` | ninguno (sin paginación ni filtros) | — | `[]ProductListItem` (`dto/product.go:13-25`): id (UUID), name, brand, model, category, description, rating, reviewCount, specs (map), images, offers | 200, 500 | Completo (con problemas de rendimiento) |
| GET | `/products/{publicID}` | `GetByPublicID` `handler.go:115-206` | `publicID` UUID | — | `ProductDetail` (`dto/product_detail.go:7-21`) = lo de List + priceHistory + offerPriceHistory | 200, 400, 404, 500 | Completo (con bugs menores) |
| POST | `/products` | `Create` `handler.go:24-53` | — | `{categoryId*, brandId?, name*, model?, sku?, description?, imageUrl*}` (`dto/product_request.go:8-16`); rechaza campos desconocidos | `{id, name, imageUrl}` (`dto/product.go:7-11`) | 201, 400, 500 | **Incompleto** |
| PUT | `/products/{publicID}` | `Update` `handler.go:208-240` | `publicID` UUID | `{categoryId, brandId?, name, model?, sku?, description?}` (`dto/product_request.go:18-25`) | fila `generated.Product` cruda (snake_case, incluye `id` interno) | 200, 400, 404 | **Incompleto** |
| DELETE | `/products/{publicID}` | `Deactivate` `handler.go:242-264` | `publicID` UUID | — | fila `generated.Product` cruda | 200, 400, 404 | **Incompleto** |

Todas las rutas pasan por el middleware CORS (`router.go:25-40`); `OPTIONS` responde 204 (`router.go:32-35`).

Endpoints declarados en la documentación que no coinciden con el código:
- `backend/README.md:409-415` dice que "actualmente existen" solo `GET /health`, `GET /products` y `GET /products/{publicID}`. Está desactualizado, porque POST/PUT/DELETE sí existen (`router.go:54-56`).
- No se encontró documentación que declare endpoints de Product API inexistentes.

### 2.3 Estado de cada endpoint (justificación)

**GET `/products` — Completo, con problemas**
- Hace N+1 consultas: por cada producto ejecuta 3 consultas adicionales (imágenes, ofertas y specs) (`handler.go:75`, `handler.go:85`, `handler.go:95`). Con el pool de 10 conexiones (`postgres.go:17`), el costo crece linealmente con el catálogo.
- No tiene paginación, límite ni filtros (`handler.go:64`; la query `ListProducts` en `database/queries/products.sql:60-82` no tiene `LIMIT`).
- Incluye ofertas no disponibles y de tiendas inactivas, porque `ListProductOffers` no filtra por `available` ni por `s.active` (`products.sql:85-104`).
- Si falla el encode, el error se ignora en silencio (`handler.go:110-112`).

**GET `/products/{publicID}` — Completo, con bugs menores**
- Cualquier error de BD (no solo "no encontrado") se reporta como 404 (`handler.go:125-129`).
- Devuelve productos desactivados, porque `GetProductDetailByPublicID` no filtra `p.active` (`products.sql:36-57`). En cambio, List sí lo hace (`products.sql:81`) y Update/Deactivate también (`products.sql:206`, `products.sql:227`).
- `priceHistory` mezcla el historial de **todas** las ofertas del producto sin indicar la tienda (`products.sql:252-261`, `handler.go:175-199`).
- Si falla el encode, el error se ignora (`handler.go:203-205`).

**POST `/products` — Incompleto**
- No tiene autenticación ni autorización (`router.go:54`).
- No acepta atributos/especificaciones ni varias imágenes (solo `imageUrl`) (`dto/product_request.go:8-16`, `repository.go:53-56`).
- Una FK inválida (`categoryId` o `brandId` inexistente) o un `sku` duplicado (UNIQUE en `001_schema.sql:63`) devuelve **500** en vez de 400/409/422 (`handler.go:43-47`).
- No valida largos, aunque `name` es `VARCHAR(255)` (`001_schema.sql:61`); si se excede, la BD falla y se responde 500.
- La respuesta es mínima (`{id,name,imageUrl}`) y tiene otra forma que la de GET (`dto/product.go:7-11`).
- Lo positivo: es transaccional (`repository.go:25-29`, `repository.go:61`) y valida `imageUrl` http(s) (`handler.go:55-61`).

**PUT `/products/{publicID}` — Incompleto**
- No valida nada después de decodificar: un `name` vacío o `categoryId=0` pasan al SQL (`handler.go:223-231`).
- Es un reemplazo total: los campos omitidos quedan en NULL/0 (`dto/product_request.go:38-48`).
- **Todos** los errores (FK, UNIQUE, BD caída) se reportan como 404 "product not found" (`handler.go:232-235`).
- No puede actualizar imágenes ni atributos (`dto/product_request.go:18-25`).
- Devuelve `generated.Product` crudo, con claves `public_id`, `category_id`, etc. y el `id` serial interno (`handler.go:239`, `generated/models.go:97-109`), lo que no coincide con el contrato camelCase de GET.
- Si falla el encode, el error se ignora (`handler.go:239`).

**DELETE `/products/{publicID}` — Incompleto**
- Hace soft-delete (`active=false`) (`products.sql:221-239`), pero cualquier error se reporta como 404 (`handler.go:256-259`).
- Devuelve 200 con la fila cruda en vez de 204 (`handler.go:261-263`).
- No desactiva las ofertas asociadas: `GetStoreByID` las sigue contando (ver §3.3).

**Código muerto detectado**
- `CreateProductRequest.ToParams()` no se usa (`dto/product_request.go:27-36`).
- `Repository.GetByPublicID` no se usa (`repository.go:76-78`).
- Las queries sqlc `GetProductByID`, `UpdateProduct`, `DeactivateProduct` y `GetProductByPublicID` no tienen consumidores (`products.sql:1-32`, `products.sql:152-194`).

### 2.4 Modelo de datos (Product API)

Tablas que usa Product API, según `backend/database/migrations/001_schema.sql`:

| Tabla | Columnas relevantes | Uso en código |
| --- | --- | --- |
| `product` (`001:56-70`) | id SERIAL, public_id UUID UNIQUE, category_id FK NOT NULL, brand_id FK, name VARCHAR(255), model, sku UNIQUE, description, active, created_at, updated_at | CRUD (`products.sql`) |
| `product_category` (`001:46-54`) | id, **parent_category_id** (jerarquía), name, description, active | Solo JOIN para el nombre (`products.sql:55`, `products.sql:79`) |
| `brand` (`001:38-44`) | id, name UNIQUE, logo_url, website_url, active | JOIN para el nombre |
| `product_image` (`001:72-79`) | product_id, image_url, alt_text, sort_order, active | Lectura + 1 insert en Create |
| `product_category_specification` (`001:81-90`) | category_id, name, **data_type VARCHAR(50)**, unit, **required**, **comparable**, display_order | Solo lectura (nombre para el map `specs`) |
| `product_specification_value` (`001:92-98`) | product_id, specification_id, **value VARCHAR(255)**, UNIQUE(product_id, specification_id) | Solo lectura |
| `product_offer` (`001:118-135`) | product_id, store_id, price NUMERIC(12,2), list_price, currency CHAR(3), shipping_cost, shipping_free, available, stock, condition (CHECK new/used/refurbished), product_url, last_updated, UNIQUE(product_id, store_id) | Solo lectura |
| `product_price_history` (`001:137-143`) | product_offer_id, price, is_promotional, recorded_at | Solo lectura |
| vista `product_rating_summary` (`002_views.sql:7-14`) | product_id, derived_average_rating, derived_review_count | rating/reviewCount |

**Categorías y atributos: ¿qué existe?**
- Las categorías son jerárquicas (`parent_category_id`, `001:48`). En los seeds hay subcategorías (`007_Computacion.sql:35`).
- Los atributos dinámicos siguen un modelo **EAV**: las definiciones por categoría están en `product_category_specification` (tipo, unidad, obligatorio, comparable, orden) y los valores en `product_specification_value`.
- Los valores se guardan **siempre como texto** (`value VARCHAR(255)`, `001:96`). `data_type` es texto libre, sin CHECK ni enum (`001:85`). En los seeds aparecen `number` (28), `string` (17) y `boolean` (1).
- No hay restricción UNIQUE `(category_id, name)` en las definiciones, ni una tabla de opciones o enumeraciones, ni rangos min/max.
- No se valida que un valor pertenezca a una spec de **la misma categoría** que el producto.
- No hay atributos en JSONB para productos. El único JSONB está en tablas de búsqueda/IA/scraping (`001:395`, `001:423`, `001:449`) y en `004_platform_apis.sql` (`saved_search.filters`, `notification.data`, `ingestion_run.details`).

**¿El código coincide con el esquema?**
- Las queries de Product API son coherentes con `001_schema.sql`: sqlc compila y `go build`/`go vet` pasan (ver §4.4).
- `generated/models.go` está **desactualizado** respecto a `004_platform_apis.sql`: no tiene `SavedSearch`, `UserSession`, `CartItem`, `StoreReview`, `Notification` ni `IngestionRun` (lista de tipos en `generated/models.go:14-430`). Esto no afecta a Product/Store, porque no usan esas tablas, pero indica que sqlc no se regeneró después de la 004. `sqlc` no está instalado localmente, así que no se pudo verificar la regeneración.

### 2.5 Dependencias (Product API)

- **Base de datos**: la misma PostgreSQL que el resto de APIs (`k8s/base/product-api/deployment.yaml:54-55`, `docker/compose.yml:29`).
- **Variables de entorno**: `PORT`, `DATABASE_URL`, `FRONTEND_URL` (`config.go:18-31`). Si faltan, usa defaults con credenciales `postgres:postgres@localhost` (`config.go:25`). En k8s también se inyectan `POSTGRES_USER/PASSWORD/DB` desde un Secret (`deployment.yaml:44-52`).
- **Librerías**: chi v5.3.2, pgx v5.10.0, godotenv v1.5.1 (`go.mod:5-9`).
- **No llama a otros servicios.**
- **Quién la consume**:
  - El frontend: `GET /products` y `GET /products/{id}` (`fronted/src/Services/api/api.ts:135`, `api.ts:140`).
  - Search API, que hace `GET /products` (`internal/features/catalog.go:200`).
  - Comparison API, que hace `GET /products/{id}` (`internal/features/clients.go:52-55`, `comparisons.go:44`).
  - Ingestion API, que hace `POST /products` (`internal/features/ingestion.go:40`).
- **Exposición**: nginx solo deja pasar GET desde internet hacia `/products` (`docker/frontend/nginx.conf:22-31`, `limit_except GET` en `:24`). En docker compose el puerto 8081 se publica directo en el host, sin ese filtro (`docker/compose.yml:31-32`).

---

## 3. Store API

### 3.1 Stack y estructura

| Aspecto | Valor | Evidencia |
| --- | --- | --- |
| Lenguaje, router, driver, sqlc | Igual que Product API (mismo módulo `backend/`) | `backend/go.mod`, `backend/internal/stores/repository.go:10-18` |
| Entrypoint | `cmd/store-api/main.go` → `apphttp.NewStoreRouter` | `backend/cmd/store-api/main.go:30` |
| Puerto | 8082 | `k8s/base/store-api/deployment.yaml:42-43`, `docker/compose.yml:49` |
| Capas | `handler.go` → `repository.go` → `generated/stores.sql.go`; DTOs en `stores/dto/store.go` | `backend/internal/stores/` |

Corrección al contexto conocido: Store API **no es un servicio con código aparte**. Es otro binario del mismo módulo Go, con su propia imagen Docker (`docker/store-api/Dockerfile:13`) y su propio Deployment en k8s.

### 3.2 Inventario de endpoints reales

Rutas en `backend/internal/http/router.go:77-91`. Handlers en `backend/internal/stores/handler.go`.

| Método | Ruta | Handler | Parámetros | Body | Respuesta | Códigos | Estado |
| --- | --- | --- | --- | --- | --- | --- | --- |
| GET | `/health` | `healthHandler` `router.go:170-191` | — | — | igual que Product (`service:"soloservis-api"`) | 200, 503 | Completo |
| GET | `/stores` | `List` `handler.go:20-34` | ninguno (sin paginación) | — | `[]Store` (`dto/store.go:10-21`): id (string de un int), name, logo, rating, reviewCount, productCount, reputation, dispatchTime, conditions, website | 200, 500 | Completo, con bug (`reviewCount`) |
| GET | `/stores/{id}` | `GetByID` `handler.go:36-77` | `id` entero int32 | — | `StoreDetail` (`dto/store.go:23-27`) = Store + products[] (`dto/store.go:29-46`, con la oferta de esa tienda) + locations[] | 200, 400, 404, 500 | Completo, con bugs |

No hay POST, PUT ni DELETE de tiendas. Tampoco hay endpoints de ofertas en Store API.

Documentación vs código: `backend/README.md:1102` lista "Store API | 8082 | `/stores`", lo que coincide. No se encontraron endpoints de Store declarados que no existan.

### 3.3 Estado de cada endpoint (justificación)

**GET `/stores` — Completo, con bug**
- `reviewCount` siempre vale 0: `FromStore` nunca lo asigna (`dto/store.go:59-71`), aunque la tabla `store_review` existe (`004_platform_apis.sql:34`). El frontend lo muestra (`fronted/src/Services/api/api.ts:88`).
- `productCount` cuenta ofertas disponibles sin filtrar productos inactivos (`stores.sql:14-19`).
- No hay paginación (`stores.sql:1-32`).
- Si falla el encode, el error se ignora (`handler.go:33`).

**GET `/stores/{id}` — Completo, con bugs**
- Cualquier error de BD se reporta como 404 (`handler.go:43-47`).
- `product_count` incluye productos inactivos: el `LEFT JOIN product p ... AND p.active` no filtra el `COUNT(DISTINCT po.product_id)` (`stores.sql:47-54`). En cambio, `ListStoreProducts` sí filtra `p.active` (`stores.sql:117`), así que el contador y la lista pueden no coincidir.
- Hace 3 consultas secuenciales (store, products, locations) (`handler.go:43`, `handler.go:49`, `handler.go:55`), lo cual es aceptable.
- Si falla el encode, el error se ignora (`handler.go:76`).

### 3.4 Modelo de datos (Store API)

| Tabla | Columnas relevantes | Uso |
| --- | --- | --- |
| `store` (`001:104-116`) | id SERIAL, name, website_url, logo_url, rating NUMERIC(3,2), reputation, shipping_information, general_conditions, active, timestamps | Lectura |
| `product_offer` (`001:118-135`) | ver §2.4 | Lectura (JOIN) |
| `product`, `brand`, `product_category`, `product_image` | ver §2.4 | Lectura (JOIN), son tablas del dominio Product |
| `store_location` (`001:351-358`) + `location` (`001:341-349`) | address, postal_code, country, region, city, commune, lat/long | Lectura |
| `store_review` (`004_platform_apis.sql:34-44`) | user_id, store_id, rating, title, content | **No se usa** en Store API (la usa Review API) |

- `store.rating` es un valor estático guardado en la columna y no se deriva de `store_review`. En cambio, el rating de los productos sí se calcula con una vista (`002_views.sql:7-14`).
- El código de Store coincide con el esquema (compila con sqlc). `store` no usa UUID público y expone el ID serial (`dto/store.go:61`), a diferencia de `product` (UUID).

### 3.5 Dependencias (Store API)

- **No llama a Product API ni a ningún otro servicio**. El acoplamiento con Product es **a nivel de base de datos**: Store API lee directamente `product`, `product_image`, `brand` y `product_category` (`stores.sql:71-118`).
- La base de datos es la misma que la de Product. Las variables de entorno, las librerías y los defaults también son los mismos (`k8s/base/store-api/deployment.yaml:41-58`).
- **Quién la consume**:
  - El frontend: `GET /stores` y `GET /stores/{id}` (`fronted/src/Services/api/api.ts:176`, `api.ts:181`).
  - Ingestion API, que valida la tienda con `GET /stores/{id}` (`internal/features/ingestion.go:71-75`).
- **Exposición**: nginx solo deja pasar GET (`docker/frontend/nginx.conf:33-42`). En compose el puerto 8082 se publica directo (`docker/compose.yml:52-53`).

---

## 4. Estado transversal

### 4.1 Formato de errores — inconsistente
- Product y Store usan `http.Error`, que responde en **texto plano** (`text/plain`, mensaje + `\n`), por ejemplo en `products/handler.go:31` y `stores/handler.go:23`.
- El resto de APIs (features, user) responde **JSON** `{"error": "..."}` con `apiutil.WriteError` (`internal/apiutil/http.go:20-22`, `internal/features/handler.go:111-113`).
- El frontend tiene que manejar los dos formatos.
- Los códigos HTTP no distinguen los tipos de error: FK, UNIQUE o caída de BD se reportan como 404 o 500 (§2.3, §3.3).
- Lo positivo: los mensajes son genéricos y **no exponen detalles internos** al cliente.

### 4.2 Validación de entrada
- Se usa `DisallowUnknownFields` en Create/Update (`products/handler.go:28`, `:221`).
- La validación es mínima y solo existe en Create (`handler.go:38-41`). Update no valida (`handler.go:223-231`).
- No hay límites de longitud ni de tamaño del body (no se usa `http.MaxBytesReader` en ningún handler).
- No hay librería de validación.

### 4.3 Logging
- Solo hay logs de arranque y errores fatales (`cmd/product-api/main.go:22-38`, `cmd/store-api/main.go:22-38`).
- **No hay logging de requests ni de errores**: los errores de BD se descartan sin registrarse (ejemplo: `products/handler.go:44-46`).
- No se usa middleware de chi (logger, recoverer, request-id). El único `r.Use` es CORS (`internal/http/router.go:44`, `:79`).

### 4.4 Tests
- **0 archivos de test** en el repo (no hay `*_test.go`, `*.spec.*` ni `*.test.*` fuera de `node_modules`).
- Ejecutado sobre una copia del commit auditado, sin modificar el repo, con Go 1.27.1:
  - `go build ./...` → OK
  - `go vet ./...` → OK
  - `go test ./...` → exit 0, `[no test files]` en los 38 paquetes
- No hay tests de integración ni infraestructura de tests (no hay testcontainers ni BD de test).
- El hook pre-commit (`.husky/pre-commit` → `lint-staged`) apunta a `backend/**/*.ts` y `frontend/src/**` (`package.json:20`, `package.json:24`). Ninguna de esas rutas aplica: el backend es Go y la carpeta real es `fronted/`. Por eso el hook no valida nada en la práctica (deducido de las rutas; no se ejecutó el hook).

### 4.5 CORS
- El middleware es propio (`internal/http/router.go:25-40`): un solo origen tomado de `FRONTEND_URL`, métodos `GET, POST, PUT, DELETE, OPTIONS`, headers `Content-Type, Authorization`. No envía `Vary: Origin`.
- En k8s, `FRONTEND_URL=http://soloservis.local` (`k8s/base/kustomization.yaml:32`). No se encontró en `scripts/k8s/deploy.ps1` nada que lo sobrescriba. En la práctica no afecta, porque el navegador accede por el mismo origen a través de nginx (`nginx.conf:18-21`).
- En compose: `FRONTEND_URL=http://localhost:8443` (`docker/compose.yml:30`).

### 4.6 Migraciones e infraestructura (afecta a todas las tareas)
- **Hay dos mecanismos de migración distintos**:
  - k8s: Job con `migrate.sh` y la tabla `schema_migrations` (version + checksum) (`k8s/base/migrations/migrate.sh:15-19`).
  - Docker compose: `docker-entrypoint-initdb.d`, que corre solo en un volumen nuevo y **sin** `schema_migrations` (`docker/compose.yml:14`).
- **Números de migración duplicados**: `006_demo_catalog_services.sql` / `006_refrigeradores.sql`, `007_Computacion.sql` / `007_service_cart.sql` y `008_normalize_prices_to_clp.sql` / `008_Telefonia.sql`. El orden depende de `sort` y del locale (`migrate.sh:37`) o de `Sort-Object Name` (`scripts/k8s/deploy.ps1:206`). Por ejemplo, con locale C, `008_Telefonia` se ordena antes que `008_normalize`; con en_US, al revés. No se verificó el locale efectivo del contenedor.
- **La atomicidad declarada no se cumple** en las migraciones que traen su propio `BEGIN; … COMMIT;` (005, 006, 007_Computacion, 008: por ejemplo `008_normalize_prices_to_clp.sql:1` y `:46`). `migrate.sh:56-58` usa `--single-transaction` con `-f` y `-c INSERT`. La documentación de psql advierte que `--single-transaction` no tiene el efecto deseado si el script contiene `BEGIN/COMMIT`: el `COMMIT` interno cierra la transacción antes del `INSERT` en `schema_migrations`. Esto se basa en el comportamiento documentado de psql; no se ejecutó.
- Si una migración cambia después de aplicarse, solo se emite un aviso y no se detiene el proceso (`migrate.sh:47-49`).
- El README backend menciona `008_service_cart.sql`, `009_normalize_prices_to_clp.sql` y `010_seed_all_services.sql` (`backend/README.md:1117`, `:1122-1124`). Los nombres reales son `007_service_cart.sql` y `008_normalize_prices_to_clp.sql`, y `010_seed_all_services.sql` no existe.
- **Solo se despliegan en k8s** postgres, migraciones, product-api, store-api y frontend (`k8s/base/kustomization.yaml:15-26`). Nginx solo enruta `/products` y `/stores` (`nginx.conf:22-42`). El frontend se compila con todas las URLs `VITE_*` vacías, es decir, mismo origen (`docker/frontend/Dockerfile:19-34`), y llama además a Auth, Search, Favorites, Cart y Comparison (`fronted/src/Services/api/personalization.ts:4-8`). En k8s esas rutas caen en el fallback de la SPA (`nginx.conf:51-54`). Es probable que esas funciones no operen en k8s; **no se verificó en ejecución**.

---

## 5. Hallazgos de seguridad

| # | Severidad | Hallazgo | Evidencia | Notas |
| --- | --- | --- | --- | --- |
| S1 | **Alta** | Endpoints de escritura de Product API (POST/PUT/DELETE `/products`) **sin autenticación ni autorización** | `backend/internal/http/router.go:54-56` | Nginx los bloquea desde internet (`nginx.conf:24`), pero cualquier pod del namespace, el puerto publicado en compose (`docker/compose.yml:31-32`) e Ingestion API (`ingestion.go:40`) pueden escribir. El enum `user_role` ya define `admin` (`001_schema.sql:16`), pero no se usa. |
| S2 | **Alta** | Pricing API `POST /offers` y `PUT /offers/{id}`, e Ingestion API `POST /ingestion/*`, **sin autenticación** | `internal/features/router.go:21-22`, `:96-98`; `catalog.go:71`, `catalog.go:120`; `ingestion.go:23`, `:64`, `:89` | Hoy no se despliegan en k8s. Son la base de TAL-74 y TAL-77, así que el riesgo se hereda. |
| S3 | **Media** | Comparison API toma `userId` del body sin sesión (IDOR: se pueden crear comparaciones a nombre de otro usuario, y `GET /comparisons/products/{id}` no verifica al dueño) | `internal/features/comparisons.go:21`, `:87-103`; `router.go:35-47` (sin `requireSession`) | Fuera del alcance directo, pero afecta a TAL-76. |
| S4 | **Media** | Credenciales de BD hardcodeadas (`postgres/postgres`) en el default de config, en el Secret de k8s versionado en git y en compose | `backend/internal/config/config.go:25`; `k8s/base/postgres/secret.yaml:12-14`; `docker/compose.yml:7-8`, `:29` | El Secret aclara que son "de DESARROLLO" (`secret.yaml:1-2`), pero el script de despliegue aplica ese mismo Secret en el clúster compartido (no se verificó si deploy.ps1 lo reemplaza). |
| S5 | **Media** | `GET /products` sin paginación, con N+1 consultas y accesible desde internet: posible agotamiento del pool (10 conexiones) | `products/handler.go:63-106`; `postgres.go:17`; `nginx.conf:22-24` | Con el catálogo actual es manejable; escala mal. |
| S6 | **Baja** | Sin límite de tamaño de body ni de longitud de campos en Create/Update | `products/handler.go:27-30`, `:220-226` | Mitigado en k8s por el bloqueo de no-GET en nginx. |
| S7 | **Baja** | `ORDER BY` construido por concatenación en Search API | `internal/features/catalog.go:227` | El valor sale de una lista blanca (`catalog.go:210-217`), así que **no es inyectable**. Se documenta como patrón a vigilar. Product y Store usan solo queries parametrizadas (sqlc). Lo mismo aplica a `reviews_watch_notifications.go:138`, `:153` (lista blanca en `:166`). |
| S8 | **Baja** | `sslmode=disable` en la conexión a BD | `k8s/base/product-api/deployment.yaml:55`, `k8s/base/store-api/deployment.yaml:55` | El tráfico es interno del clúster. |
| S9 | **Baja** | Interpolación de nombre de archivo y checksum en SQL dentro de `migrate.sh` | `k8s/base/migrations/migrate.sh:44`, `:58` | Los nombres vienen del repo y no de un usuario. |
| S10 | **Baja** | Dependencia npm `"psql": "^0.0.1"` en el `package.json` raíz, sin uso aparente | `package.json:32` | **No verificado** qué paquete es. Conviene revisarlo por riesgo de typosquatting. |
| S11 | **Baja** | Se expone el ID serial interno en las respuestas de PUT/DELETE de producto y en Store | `products/handler.go:239`, `:263`; `stores/dto/store.go:61` | Permite enumerar recursos. |
| — | Positivo | No se exponen errores internos al cliente: los mensajes son genéricos | ej. `products/handler.go:45`, `apiutil/http.go:20-22` | Pero tampoco se registran en logs (§4.3). |

---

## 6. Impacto en el backlog (TAL-70 a TAL-78)

| Tarea | Qué existe | Qué falta | Tipo |
| --- | --- | --- | --- |
| **TAL-70** Product CRUD con categoría y atributos dinámicos | CRUD básico con `categoryId` FK (`router.go:52-56`); soft-delete; lectura de specs como map (`handler.go:95-103`); tablas EAV (`001:81-98`) | Escribir atributos en create/update; actualizar imágenes; validación en Update; códigos HTTP correctos (400/409/422); respuestas uniformes en camelCase; PATCH o reemplazo parcial; filtrar `active` en el detalle; paginación y filtros en el listado; autenticación de escritura | **Adaptar** |
| **TAL-71** Endpoints de categorías y definiciones de atributos | Esquema completo: categorías jerárquicas (`001:46-54`) y definiciones con tipo, unidad, obligatorio, comparable y orden (`001:81-90`); solo hay lectura parcial `GET /search/categories` en Search API (`catalog.go:231-234`) | Todos los endpoints CRUD de categorías y specs en Product API; UNIQUE `(category_id, name)`; catálogo de `data_type` permitido; opciones para atributos tipo enum | **Crear desde cero** (endpoints) sobre un **esquema existente** que hay que ajustar |
| **TAL-72** Validación de atributos | Metadatos `data_type` y `required` en BD (`001:85-87`) | Toda la lógica: validar tipos (number/string/boolean), obligatorios, que la spec pertenezca a la categoría del producto, unidades y rangos. Hoy no hay nada en código; `value` es VARCHAR libre (`001:96`) | **Crear desde cero** |
| **TAL-73** Store API y ofertas por producto | Store API de solo lectura (`router.go:87-88`); las ofertas por producto se exponen en `GET /products/{id}` (`handler.go:137-165`) y en Pricing API `GET /offers/product/{productId}` (`catalog.go:28-42`, no desplegada) | CRUD de tiendas; decidir qué API es dueña de las ofertas (hoy hay solapamiento entre Product, Store y Pricing); `reviewCount` real; corregir `productCount`; paginación; autenticación | **Adaptar** (lectura) + **crear** (escritura de tiendas) |
| **TAL-74** Pricing (precio actual e historial) | Tabla `product_price_history` (`001:137-143`); historial en el detalle de producto (`handler.go:149-199`); Pricing API con GET/POST/PUT de ofertas, historial por oferta, inserción de historial cuando cambia el precio y notificación de baja de precio (`catalog.go:28-196`); trigger de moneda CLP (`008_normalize_prices_to_clp.sql:27-37`) | Desplegar en k8s y enrutar en nginx; autenticación de escritura; usar decimales en vez de `float64` para dinero (`catalog.go:17`); endpoint de "mejor precio actual" por producto; historial por tienda (hoy se mezcla en el detalle); tests | **Adaptar** |
| **TAL-75** Search | Search API `GET /search/products` con `q`, categoría, marca, rango de precio y orden, `LIMIT 100` (`catalog.go:198-229`); búsqueda de categorías y marcas; historial y búsquedas guardadas con sesión (`router.go:23-34`) | Paginación; índices (hoy usa `ILIKE '%…%'` sin índice trigram/FTS); quitar la descarga completa de `GET /products` que solo se usa como chequeo de disponibilidad (`catalog.go:200-203`); filtrar por atributos; despliegue en k8s. El frontend parece filtrar del lado del cliente (`fronted/src/pages/search/SearchResultsPage.tsx:74-106`, no verificado en detalle) | **Adaptar** |
| **TAL-76** Comparison | Comparison API completa en rutas (`router.go:35-47`); tablas `product_comparison(_item)` (`001:149-165`); regla de misma categoría (`comparisons.go:59-62`); specs `comparable` (`comparisons.go:96-98`); el frontend ya la consume (`personalization.ts:409-443`) | Autenticación y propiedad (S3); desplegar en k8s; tests; manejo de errores (cualquier fallo de Product API se reporta como 503, `comparisons.go:44-47`) | **Adaptar** |
| **TAL-77** Ingestion | Ingestion API: `POST /ingestion/products` (delega en Product API si viene `imageUrl`, si no inserta directo en BD), `POST /ingestion/offers` (valida la tienda vía Store API y delega en Pricing), `POST /ingestion/sync` (`ingestion.go:23-104`); tablas `data_source`, `scraper_config`, `scrape_run` y `scraped_data` sin uso (`001:406-451`) | `sync` es un **stub**: solo inserta `ingestion_run` con estado `accepted` y no procesa nada (`ingestion.go:89-105`); hay una rama muerta (`ingestion.go:55-60`, inalcanzable por el `return` de `:45`); dos caminos de escritura de productos (HTTP vs SQL directo); sin autenticación (S2); sin idempotencia ni dedupe; sin despliegue | **Adaptar** la API + **crear** la lógica real de sincronización |
| **TAL-78** Errores, logs y tests | Helper JSON `apiutil.WriteError` (`apiutil/http.go:20-22`), que Product y Store no usan; `go vet` pasa | Formato de error único; mapeo de errores pgx a códigos HTTP; logging estructurado de requests y errores; middleware recoverer/request-id; **tests unitarios y de integración (hoy 0)**; corregir el hook lint-staged (`package.json:20-24`); un único mecanismo de migraciones con numeración única | **Crear desde cero** |

---

## 7. Preguntas abiertas para el equipo

1. **Rama base**: `develop` no tiene k8s, nginx, Product/Store API separadas ni las migraciones 004 a 008. ¿Cuándo se integra `feature/platform-apis-and-product-comparison` (o `Feat-Services`) en `develop`? ¿Las ramas de TAL-70 a TAL-78 deben salir de `develop` o de esa rama?
2. **Migraciones**: ¿renumeramos los duplicados 006, 007 y 008? Habría que coordinarlo con las bases que ya tienen `schema_migrations`, porque cambiar el nombre implica re-ejecutar. ¿Quitamos el `BEGIN/COMMIT` de los seeds o cambiamos el runner? ¿Compose debería usar el mismo runner que k8s?
3. **Dueño de las ofertas** (TAL-73/TAL-74): hoy hay ofertas en Product (lectura), Store (lectura por tienda) y Pricing (lectura y escritura). ¿Qué servicio es el dueño de `product_offer` y del historial?
4. **Autenticación de escritura**: ¿qué mecanismo se usa para CRUD de productos, categorías, tiendas y ofertas? Ya existen `user_session` y el rol `admin` en `user_role`. ¿Lo validaría cada API o habría un gateway?
5. **Modelo de atributos** (TAL-70 a TAL-72): ¿mantenemos EAV (`product_specification_value.value VARCHAR`) o migramos a JSONB con validación? ¿Qué catálogo cerrado de `data_type` usamos (number, string, boolean, enum)?
6. **Despliegue**: en k8s solo están Product y Store, pero el frontend llama a Auth, Search, Favorites, Cart y Comparison por el mismo origen y nginx no tiene esas rutas. ¿Es intencional para el MVP? Habría que verificarlo en el entorno real.
7. **sqlc**: `generated/models.go` no refleja la 004. ¿Quién regenera y con qué versión (v1.31.1)? ¿Las tablas de 004 se acceden siempre con SQL manual en `features`?
8. **Código legado**: ¿se puede eliminar `cmd/server` (monolito), `backend/src/database/*.ts` (NestJS), las queries sqlc sin uso y la dependencia `psql` del `package.json` raíz?
9. **Contrato de IDs**: los productos usan UUID público y las tiendas un entero serial. ¿Unificamos con un `public_id` UUID para `store`?
10. **Datos de seed**: no se verificó si los seeds cumplen las specs `required` de cada categoría. ¿Se valida al introducir TAL-72, o se aceptan los datos existentes como legado?

---

### Anexo: elementos no verificados
- Comportamiento en ejecución contra una BD real: no se levantó infraestructura, por indicación expresa.
- Locale efectivo del contenedor `postgres:18` del Job de migraciones y el orden resultante de los duplicados.
- Si `scripts/k8s/deploy.ps1` reemplaza el Secret de desarrollo o `FRONTEND_URL` en el overlay generado. El grep no encontró evidencia de que lo haga.
- Qué paquete es `psql@^0.0.1` en npm.
- Funcionamiento de Auth, Search, Favorites, Cart y Comparison desde el frontend desplegado en k8s.
