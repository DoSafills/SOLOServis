# Errores y pendientes para Linear

Hallazgos de la revisión del backend y del sistema completo hecha el
**2026-11-22** sobre `origin/develop` (`4cc526e`), con el Compose local
levantado y el verificador `scripts/testing/contratos.py` corriendo contra el
gateway en `http://localhost:8443`.

Cada entrada trae el título listo para Linear, el área, la evidencia, el
impacto y el criterio de aceptación. El dueño se asigna según `AGENTS.md` §6:
frontend → Cristian; APIs del backend y Kubernetes → Marcelo; scraper y
crawler → Kevin; lo demás → Ricardo.

## Resumen

| # | Título | Área | Prioridad | Dueño |
| --- | --- | --- | --- | --- |
| G1 | El gateway bloquea las escrituras del catálogo y responde 403 HTML | Gateway / K8s | Media | Marcelo |
| G2 | `GET /health` por el gateway devuelve la SPA con 200 | Gateway / K8s | Media | Marcelo |
| G3 | Los prefijos privados devuelven la SPA con 200 en vez de 404 problem+json | Gateway / K8s | Baja | Marcelo |
| G4 | El 429 del gateway responde `instance` vacío | Gateway | Baja | Marcelo |
| D1 | Compose no aplica migraciones nuevas a un volumen existente | Base de datos | Alta | Marcelo |
| D2 | El CI no corre los tests de integración de Go | Base de datos / CI | Media | Marcelo |
| B1 | `GET /users` es un stub con contenido falso | Backend | Baja | Marcelo |
| B2 | Los POST de `features` responden 201 sin `Location` | Backend | Baja | Marcelo |
| F1 | El frontend no tiene tests | Frontend | Media | Cristian |
| F2 | `DynamicProductForm` no está montado en ninguna pantalla | Frontend | Baja | Cristian |
| F3 | `fronted/package-lock.json` convive con `pnpm-lock.yaml` | Frontend | Baja | Cristian |
| S1 | Mercado Libre bloqueado por anti-bots | Scraper | Media | Kevin |
| L1 | Restos sin uso: `neon.ts`, `backend/src/database/*.ts` | Limpieza | Baja | Ricardo |

---

## Gateway y Kubernetes

### G1. El gateway bloquea las escrituras del catálogo y responde 403 HTML

- **Evidencia:** `docker/frontend/default.conf.template:49-62` aplica
  `limit_except GET { deny all; }` a `/products`, `/stores` y `/services`. El
  frontend tiene las escrituras en `fronted/src/services/api/product-write.ts`
  (`POST /products`, `POST /products/duplicate-candidates`,
  `PUT /products/{id}`).
- **Comprobado:** `POST /products` por el gateway devuelve `403` con
  `text/html` de nginx (`contratos.py`, aviso "POST /products sin token").
- **Impacto:** la API de administración del catálogo queda inutilizable desde el
  navegador. Hoy ninguna pantalla la usa (`DynamicProductForm` no está montado,
  ver F2), pero el contrato dice que un admin puede escribir y el error que ve
  el cliente no es problem+json, así que el frontend no puede mostrar el
  `detail`.
- **Criterio de aceptación:** las escrituras del catálogo llegan al backend y
  responden 401/403/201 con problem+json o JSON del estándar; un usuario admin
  puede crear y editar un producto por el gateway. El preflight `OPTIONS` de
  esas rutas no se bloquea.

### G2. `GET /health` por el gateway devuelve la SPA con 200

- **Evidencia:** `default.conf.template` no tiene `location = /health`; la
  petición cae en `location /` (`try_files ... /index.html`).
- **Comprobado:** `GET /health` por el gateway responde `200 text/html`
  (`contratos.py`, aviso).
- **Impacto:** un monitor externo que use `/health` ve 200 aunque el proceso
  esté caído o sin base. Los probes de Kubernetes van directo al pod y no se
  ven afectados.
- **Criterio de aceptación:** `/health` por el gateway responde el estado real
  (proxy a un dominio o 404 problem+json), y queda documentado cuál es el
  endpoint de salud público.

### G3. Los prefijos privados devuelven la SPA con 200 en vez de 404 problem+json

- **Evidencia:** `/offers`, `/users`, `/notifications` e `/ingestion` no tienen
  location en el gateway (es intencional: solo red interna).
- **Comprobado:** los cuatro devuelven `200 text/html` con el `index.html` de la
  SPA (`contratos.py`, avisos).
- **Impacto:** un cliente que llame a una ruta no publicada recibe un 200 que
  parece éxito. Confunde a quien integra y a los monitores.
- **Criterio de aceptación:** esos prefijos responden 404 problem+json desde
  nginx, o se documenta explícitamente en `k8s/README.md` que devuelven la SPA.

### G4. El 429 del gateway responde `instance` vacío

- **Evidencia:** `docker/frontend/default.conf.template:71` y `:76` arman el
  problem+json con `"instance":""`.
- **Impacto:** menor; el frontend decide por `code`. Rompe la simetría con el
  backend, que manda la ruta en `instance`.
- **Criterio de aceptación:** `instance` trae `$request_uri` y el cuerpo sigue
  siendo problem+json válido.

---

## Base de datos

### D1. Compose no aplica migraciones nuevas a un volumen existente

- **Evidencia:** `docker/compose.yml` monta las migraciones y
  `k8s/base/migrations/migrate.sh` en `/docker-entrypoint-initdb.d`, que
  PostgreSQL solo ejecuta cuando el directorio de datos está vacío. En el
  volumen local no existía `schema_migrations` ni la columna `search_text` de
  la migración `028_product_search_text.sql`.
- **Comprobado:** `GET /search/products` devolvía `500` con
  `ERROR: column p.search_text does not exist (SQLSTATE 42703)` en el log de
  `comercial`; tras aplicar la 028 a mano respondió `200`. La tabla
  `schema_migrations` no existía, así que el runner tampoco puede verificar
  checksums.
- **Impacto:** después de actualizar `develop`, una base local existente queda
  con el esquema viejo y las APIs fallan con 500 sin una pista clara. Pasó en
  esta misma revisión.
- **Criterio de aceptación:** `docker compose up` deja la base al día (servicio
  `migrate` de un solo uso que corre el runner en cada arranque) o el README
  indica el paso exacto para migrar un volumen existente; `GET /search/products`
  responde 200 después de actualizar la rama. El volumen de desarrollo se
  recrea con `docker compose down -v` cuando cambia el esquema.

### D2. El CI no corre los tests de integración de Go

- **Evidencia:** `.github/workflows/ci.yml` corre `go test -race ./...` sin
  `TEST_DATABASE_URL`, y los tests de integración se omiten solos.
- **Impacto:** un PR con una migración puede quedar verde y romper el esquema en
  el despliegue. Ya está anotado en `AGENTS.md` §13.
- **Criterio de aceptación:** job con `services: postgres:18`, migraciones
  aplicadas con `k8s/base/migrations/migrate.sh` y
  `go test -p 1 ./...` con `TEST_DATABASE_URL`.

---

## Backend

### B1. `GET /users` es un stub con contenido falso

- **Evidencia:** `backend/internal/http/router.go`, `registerUserRoutes`
  responde 200 con `{"status":"user-api-ready"}` y sin `request_id`.
- **Comprobado:** lo usa `k8s/tests/llamadas/escenarios.py:148` como prueba de
  vida del proceso `usuarios`.
- **Impacto:** un 200 con contenido inventado se puede confundir con un
  endpoint real de usuarios.
- **Criterio de aceptación:** se define el contrato (por ejemplo
  `GET /users/me` con la sesión, o 404 problem+json) y se actualiza el escenario
  del visor de llamadas.

### B2. Los POST de `features` responden 201 sin `Location`

- **Evidencia:** `backend/internal/features/favorites_cart.go`,
  `reviews_watch_notifications.go`, `service_personalization.go` y `auth.go`
  responden 201 sin `Location`; `products`, `pricing`, `comparison` e
  `ingestion` sí lo ponen.
- **Impacto:** inconsistencia con `AGENTS.md` §7 ("201 con `Location` en POST").
  Varios de esos recursos no tienen GET canónico (una reseña, una entrada de
  carrito), así que un `Location` apuntaría a una ruta que no existe.
- **Criterio de aceptación:** se decide la regla (solo los recursos con GET
  llevan `Location`) y se documenta en `backend/README.md`; los que la cumplan,
  la implementan con su test.

---

## Frontend

### F1. El frontend no tiene tests

- **Evidencia:** no hay suite de tests en `fronted/`; el CI corre formato, lint
  y build.
- **Impacto:** los cambios de UI se validan solo a ojo. Ya está en
  `AGENTS.md` §3.
- **Criterio de aceptación:** al menos las funciones puras (formateo de precios,
  mapeo de productos y rutas) con tests y un script `pnpm test` en el CI.

### F2. `DynamicProductForm` no está montado en ninguna pantalla

- **Evidencia:** `git grep DynamicProductForm -- fronted/src` devuelve solo su
  definición (`fronted/src/components/products/DynamicProductForm.tsx:34`).
- **Impacto:** es la única pantalla que escribiría productos y hoy es código
  muerto; además no se puede usar por G1.
- **Criterio de aceptación:** se monta en una ruta de administración o se
  elimina.

### F3. `fronted/package-lock.json` convive con `pnpm-lock.yaml`

- **Evidencia:** ambos archivos están versionados.
- **Impacto:** confunde sobre el gestor real (pnpm) y puede desincronizar
  dependencias.
- **Criterio de aceptación:** se borra `package-lock.json` y queda solo pnpm.

---

## Scraper

### S1. Mercado Libre bloqueado por anti-bots

- **Evidencia:** ya documentado en `AGENTS.md` §13; no se revisó en esta
  corrida.
- **Impacto:** esa tienda no entrega ofertas.
- **Criterio de aceptación:** evaluar la API oficial o dejar la tienda fuera del
  catálogo con una nota en `scraper/README.md`.

> El contrato scraper → ingestion no se volvió a verificar en esta corrida: el
> foco fue backend y gateway. Antes de tocar un `spec.json`, correr
> `pytest` y `ruff check` en `scraper/`.

---

## Limpieza

### L1. Restos sin uso

- `neon.ts` en la raíz (20 líneas, sin referencias en el código).
- `backend/src/database/database.module.ts` y `database.service.ts` (restos de
  NestJS).
- `lint-staged` de la raíz apunta a rutas que no existen.
- **Criterio de aceptación:** se borran o se mueven a `docs/`; `lint-staged`
  apunta a `backend/**/*.go` y `fronted/**/*.{ts,tsx}`.

---

## Corregido en esta revisión (no requiere ticket)

- **CORS unificado:** `internal/httpx/cors.go` reemplaza las dos copias de
  `internal/http` y `internal/features`. Ahora todas las APIs exponen
  `X-Total-Count` y no mandan `Access-Control-Allow-Origin` vacío cuando
  `FRONTEND_URL` no está definida. Tests en `internal/httpx/cors_test.go`.
- **Contrato gateway ↔ backend automatizado:**
  `backend/internal/allapis/gateway_test.go` falla si un prefijo público queda
  sin location, si una location no corresponde a ninguna API o si una
  `${*_UPSTREAM}` usada por la plantilla no tiene valor en la imagen.
- **Gateway en el Compose local:** `docker/compose.yml` agrega el servicio
  `frontend` en `8443`, con los mismos destinos que
  `k8s/components/dominios`. Antes el sistema completo solo se podía probar en
  el clúster.
- **Verificador de contratos:** `scripts/testing/contratos.py` comprueba
  estructura, códigos y forma de las respuestas por el gateway, y separa las
  desviaciones conocidas (avisos) de las fallas.
- **CI:** `go test -race`, `pnpm format:check` y un job que construye las
  imágenes sin publicarlas. Workflow `e2e.yml` (manual) que levanta el Compose y
  corre `contratos.py`.
