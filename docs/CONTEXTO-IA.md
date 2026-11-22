# Contexto para agentes de IA — SOLOServis

> **Lee este archivo primero.** Resume qué es el proyecto, cómo se comunican sus
> componentes y qué partes de la documentación no deben tomarse al pie de la
> letra. No reemplaza a `AGENTS.md` ni a los skills de `.opencode/skills/`:
> los complementa con el estado verificado del repositorio.
>
> Última verificación: **2026-11-20**. Si el código contradice este documento,
> gana el código: corrige este archivo en el mismo cambio.

---

## 1. Qué es SOLOServis

Plataforma web chilena para buscar, comparar y seguir productos y servicios de
distintas tiendas y proveedores (referencia: SoloTodo, ampliado a servicios).
Un scraper obtiene ofertas reales, las entrega a la API de Ingestion y estas
terminan en PostgreSQL para que el frontend las muestre.

Proyecto del curso Taller de Integración III (UCT). Las tareas viven en Linear
con el identificador `TAL-XX`.

**Estado general:** funcional y desplegado en el clúster UCT. El scraper v2
entrega productos reales de Falabella y Ripley. Mercado Libre está bloqueado
por anti-bots y se deja fallando a propósito.

**Foco de trabajo actual (2026-11-17):** auditoría de consistencia entre los
README y el código. Este documento nace de esa revisión. Las correcciones
documentales detectadas están en la sección 12.

---

## 2. Reglas que no se rompen

1. **Propuesta antes de código.** Mostrar diseño y esperar aprobación. Después,
   mostrar el diff antes del commit.
2. **Una rama por tarea**, creada desde `develop`. Nunca commit directo en
   `develop` ni en `main`.
3. **No hacer push ni merge.** El push y el PR los hace la persona. El PR va
   contra `develop`.
4. **Commits sin atribución de IA.** Nada de `Co-Authored-By` ni
   "Generated with…".
5. **Migraciones aplicadas no se editan.** Todo cambio va en un archivo nuevo y
   se registra en `migration_order.txt`.
6. **SQL siempre parametrizado** (`$1`, `$2`…). Nunca concatenar valores.
7. **Secretos:** no imprimir ni escribir tokens o contraseñas en git. Los
   Secrets se nombran, no se muestran. No leer kubeconfig ni credenciales.
8. **Reportar lo real.** Si un test falla o no se ejecutó, decirlo con la salida.
9. **Responder en español.**

---

## 3. Stack verificado

| Área | Tecnología |
| --- | --- |
| Backend | Go **1.27.1** (`backend/go.mod`), `chi` v5, `pgx` v5, `sqlc`, `godotenv` |
| Base de datos | PostgreSQL **18** |
| Frontend | React 19, TypeScript 5.9, Vite 8, Tailwind CSS 4, `recharts`; gestor `pnpm` 11.24.0 |
| Calidad frontend | `oxfmt`, ESLint 10. `pnpm check` = formato + lint + build + audit |
| Scraper v2 (`scraper/`) | Python ≥ 3.12, Scrapling, FastAPI + uvicorn, `httpx`, SQLite; `pytest` y `ruff` |
| Crawler v1 (`crawler/`) | Python ≥ 3.12, Scrapling, asyncio, SQLite. **Legado: no se despliega** |
| Contenedores | Docker multi-etapa, sin root, Docker Compose |
| Orquestación | Kubernetes + Kustomize (clúster UCT), `deploy.ps1` / `deploy.sh` |
| Autenticación | Sesiones con token opaco (se guarda SHA-256), PBKDF2-SHA256, token de servicio `INTERNAL_API_TOKEN` |
| Asistente de comparación (IA) | Gemini (`gemini-3.5-flash`) en `backend/internal/assistant/`; contexto cerrado a 2–5 productos, guardrails, caché en memoria y auditoría en `ai_comparison_query`. `auto` usa Gemini con `AI_API_KEY`; sin clave responde 503 `ai_unavailable` y no inventa texto; `off` desactiva la ruta. |

**No existe** (aunque aparezca en documentos viejos): Redis, Scrapy, Playwright
directo, TanStack Query, Prisma, NestJS, "Go Workers", JSONL como
almacenamiento principal, IA/NLP para búsquedas (solo existe el asistente de
comparación), CI en GitHub Actions, tests de frontend, OpenAPI, pantallas de
watchlist/notificaciones.

---

## 4. Mapa del repositorio

```text
backend/
  cmd/all-apis/        binario único; API_DOMAIN elige qué APIs sirve
  cmd/<api>/           un binario por API (solo modo microservicios)
  internal/<api>/      handler, repositorio y tests de cada API
  internal/httpx/      problem+json, DecodeJSON, Validator, logs, request id
  internal/authz/      sesiones, rol admin y token de servicio
  internal/allapis/    dominios, prefijos y URLs internas
  database/            migrations/, migration_order.txt, queries/ (sqlc)
fronted/               frontend React (la carpeta se llama "fronted", sin la n)
assistant/             asistente de comparación: prompts, límites, contexto y sync de assets
scraper/               scraper v2: stores/<slug>/spec.json
crawler/               crawler v1, legado; no está en compose ni en k8s
docker/                Dockerfiles, compose.yml, compose.microservicios.yml
k8s/                   base/, components/{dominios,consolidado,espera-postgres}, tests/
scripts/k8s/           deploy.ps1, deploy.sh
scripts/linear/        linear.py (necesita LINEAR_API_KEY)
scripts/testing/       reporte.py (reporte de todas las suites)
docs/                  estandares/, auditorias/, database/, requirements/
```

**Restos conocidos:** `backend/src/database/*.ts` (TypeScript sin uso),
`fronted/package-lock.json` junto a `pnpm-lock.yaml`, y directorios
`backend/cmd/{products-api,services-api,stores-api,server}` que duplican a los
singular y no se usan. No construir nada nuevo sobre ellos.

---

## 5. Arquitectura: 14 APIs en 3 dominios

Una sola imagen (`docker/all-apis/Dockerfile`, binario `cmd/all-apis`). Solo
cambia `API_DOMAIN`:

| Dominio | APIs | Proceso |
| --- | --- | --- |
| `comercial` | product, store, service, pricing, search, comparison, favorites, cart, review, watchlist, notification (11) | puerto 8080 interno |
| `usuarios` | auth, user (2) | puerto 8080 interno |
| `ingestion` | ingestion (1) | puerto 8080 interno |

- Con `API_DOMAIN=""` se registran las 14 APIs en un solo proceso (modo
  consolidado).
- Cada proceso expone `GET /health` con el nombre del dominio y
  `"database":"connected"`.
- Prefijos por API (`backend/internal/allapis/router.go`): `/products`,
  `/product-categories`, `/categories`, `/stores`, `/services`, `/users`,
  `/offers`, `/search`, `/comparisons`, `/ingestion`, `/auth`, `/cart`,
  `/favorites`, `/notifications`, `/reviews`, `/watchlist`.
- **El API Gateway es el nginx del frontend.** El Ingress de UCT solo admite el
  path `/`; nginx enruta por prefijo a cada dominio.

### Puertos locales (Docker Compose)

| Componente | Puerto |
| --- | ---: |
| PostgreSQL | 5432 |
| Product / Store / Service | 8081 / 8082 / 8083 |
| User / Pricing | 8084 / 8085 |
| Search / Comparison | 8086 / 8087 |
| Auth / Favorites / Cart | 8088 / 8089 / 8090 |
| Review / Watchlist / Notification | 8091 / 8092 / 8093 |
| Ingestion | 8094 |
| Scraper API | 8095 |

Todos los puertos de APIs apuntan al puerto 8080 del proceso de su dominio.

---

## 6. Comunicación entre componentes

```text
Navegador
   │ HTTPS (solo path /)
   ▼
Ingress UCT ──► frontend (nginx)
                   ├── /products, /categories, /stores, /services ──► comercial
                   ├── /auth ──────────────────────────────────────► usuarios
                   └── /favorites, /cart, /search, /comparisons,
                       /reviews, /watchlist ───────────────────────► comercial
                                                                      │
                         scraper-api (:8095, interno) ──► ingestion ──┤
                                                                      ▼
                                                                 PostgreSQL
```

### Llamadas HTTP entre APIs

Declaradas en `backend/internal/allapis/domains.go` (`Calls`):

| Quien llama | API requerida |
| --- | --- |
| favorites | product |
| cart | product, pricing |
| review | product, store, service |
| watchlist | pricing |
| pricing | notification |
| ingestion | notification |

- Dentro del mismo dominio se llama a `127.0.0.1:8080`.
- La **única llamada entre dominios** es `ingestion → notification`:
  `NOTIFICATION_API_URL=http://comercial:8080`. Si falta, ingestion **no
  arranca** y dice qué variable falta.
- `comercial` valida sesiones leyendo `user_session` en la base compartida: es
  dependencia por base, no por HTTP.
- `search` y `comparison` ya no llaman a Product/Pricing: consultan PostgreSQL.

### Variables de entorno del backend

| Variable | Función |
| --- | --- |
| `PORT` | Puerto HTTP (8080 por defecto) |
| `DATABASE_URL` | Conexión a PostgreSQL |
| `FRONTEND_URL` | Origen permitido por CORS |
| `API_DOMAIN` | Dominio a servir (`comercial`, `usuarios`, `ingestion` o vacío) |
| `GATEWAY_URL` | URL común de respaldo para llamadas internas |
| `PRODUCT_API_URL`, `STORE_API_URL`, `SERVICE_API_URL`, `PRICING_API_URL`, `NOTIFICATION_API_URL` | URL específica de cada API llamada |
| `INTERNAL_API_TOKEN` | Token de servicio (mínimo 32 caracteres; si es más corto se ignora) |
| `AI_PROVIDER` | Proveedor del asistente de comparación (`auto` por defecto: Gemini con `AI_API_KEY`) |
| `AI_MODEL` | Modelo del asistente (`gemini-3.5-flash` por defecto) |
| `AI_API_KEY` | Clave del proveedor, solo en el backend. Sin ella `auto` responde 503 `ai_unavailable` y no arma respuestas propias; con `off` el asistente queda desactivado |
| `AI_BASE_URL` | Opcional; URL base alternativa del proveedor (pruebas o proxy propio) |
| `LOG_LEVEL` | Nivel de log |

---

## 7. Flujo del scraper v2

```text
scraper-api (FastAPI :8095)
   │ POST /ingestion/runs            abre una ejecución remota con storeId
   │ POST /ingestion/runs/{id}/items lotes de 1..100 ítems
   │ POST /ingestion/runs/{id}/finish success | failed
   ▼
Ingestion API ──► PostgreSQL (product, product_offer, price_history, scrape_run…)
                └─► Notification API si baja un precio
```

- **Spec-first:** una tienda es `scraper/stores/<slug>/spec.json`; no hay
  selectores de respaldo. Spec inválido = error antes de la primera petición.
- Tiendas y `storeId` actuales:
  - `falabella-cl` → 1 (funciona)
  - `ripley-cl` → 4 (funciona)
  - `mercadolibre-cl` → 6 (bloqueado por anti-bots)
- El scraper **no** tiene credenciales de PostgreSQL. Solo habla HTTP con
  Ingestion usando `INGESTION_API_TOKEN` (o `INTERNAL_API_TOKEN`).
- La ingesta se activa solo si hay `store_id`, token de ≥ 32 caracteres y un
  spec válido. Si falla, el crawl local termina igual y los ítems quedan en
  SQLite y en `out/rejected/` para replay.
- API de control: `/health`, `/ready`, `/stores`, `/runs`, `/runs/{id}/stop`,
  `/tasks` (tareas recurrentes en JSON sobre el PVC, no en tabla).
- **Crawler v1 (`crawler/`) es legado.** Ni Compose ni Kubernetes lo
  referencian. Su README describe el diseño original y no debe usarse como guía.

---

## 8. Frontend

- Se ejecuta con `pnpm dev` en **`http://localhost:8443`**
  (`fronted/vite.config.ts`), no en 5173.
- En desarrollo, Vite tiene **proxy por prefijo** hacia los puertos locales:
  `/products`, `/product-categories`, `/categories`, `/stores`, `/auth`,
  `/services`, `/search`, `/comparisons`, `/favorites`, `/cart`, `/reviews`,
  `/watchlist`.
- Las URLs se pueden sobrescribir con `VITE_PRODUCTS_API_URL`,
  `VITE_STORES_API_URL`, `VITE_SERVICES_API_URL`, `VITE_SEARCH_API_URL`,
  `VITE_AUTH_API_URL`, `VITE_FAVORITES_API_URL`, `VITE_CART_API_URL` y
  `VITE_COMPARISON_API_URL`. Vacías = mismo origen.
- El frontend **no** se conecta a PostgreSQL.
- Páginas actuales: home, products, search, services, stores y user. Usa auth,
  favorites, cart, comparison y search. **No** hay pantallas de watchlist ni de
  notificaciones.
- Los errores del backend llegan como `application/problem+json`: decidir con
  `code` y mostrar `detail`.
- Antes de entregar: `cd fronted && pnpm check`.

---

## 9. Base de datos y migraciones

- Migraciones en `backend/database/migrations/`.
- **El orden lo da `backend/database/migration_order.txt`**, no el nombre del
  archivo (hay prefijos repetidos).
- El runner es `k8s/base/migrations/migrate.sh`; registra nombre y checksum en
  `schema_migrations`. Editar una migración aplicada rompe el checksum.
- El runner **no** envuelve cada archivo en una transacción: si una migración
  debe ser atómica, lleva su propio `BEGIN`/`COMMIT` o un solo bloque `DO`.
- Una base antigua sin `schema_migrations` se rechaza: requiere baseline
  verificado.
- Consultas sqlc en `backend/database/queries/` (`products.sql`, `services.sql`,
  `stores.sql`). Regenerar con `cd backend && sqlc generate`. No editar
  `internal/database/generated/` a mano.

---

## 10. Kubernetes (UCT)

- Manifiestos en `k8s/`; `k8s/overlays/` es generado por el script y está
  ignorado por git.
- Dos topologías:
  - `dominios` (por defecto): pods `comercial`, `usuarios`, `ingestion`,
    `frontend`, `postgres` y `scraper-api`.
  - `consolidado` (respaldo): `backend` (14 APIs), `frontend`, `postgres` y
    `scraper-api`.
- Servicios reales: 7 en `dominios` (incluye `postgres-headless`), 5 en
  `consolidado`. Los Jobs de migración y test suman pods mientras corren.
- Cuota del namespace: **12 pods, 10 Services, 3 PVC, 1 CPU / 1 G de requests**.
  El LimitRange exige mínimo **25m y 64Mi** por contenedor, también en Jobs.
- Pods sin root, `readOnlyRootFilesystem`, sin capabilities (Pod Security
  `restricted`).
- Secrets: `postgres-secret` y `soloservis-internal`. Los crea la fase 4; no
  están en git.
- Despliegue: `scripts/k8s/deploy.ps1 -Modo uct -Registry docker.io/<usuario>`
  (o `deploy.sh`). Fases 0–10. Redeploy sin borrar datos:
  `-FromPhase 3 -SkipCleanup`.
- `deploy.ps1` debe quedar en **ASCII** (PowerShell 5.1); `deploy.sh`, en bash
  3.2.
- El pod de test (`k8s/tests/smoke-test.sh`) ejecuta 29 comprobaciones de APIs
  en `dominios` + hasta 6 del scraper = 35. Solo hace lecturas.

---

## 11. Comandos de verificación

```bash
cd backend && go build ./... && go vet ./... && go test ./...
cd fronted && pnpm check
cd scraper && python -m pytest -q && ruff check src tests
kubectl kustomize k8s/overlays/uct
docker compose -f docker/compose.yml config --quiet
docker compose -f docker/compose.microservicios.yml config --quiet
python scripts/testing/reporte.py
```

- `go test` sin `TEST_DATABASE_URL` omite los tests de integración.
- El frontend no tiene tests unitarios; `pnpm check` es la única barrera.
- Fallas conocidas del reporte de testing: 2 tests del crawler en Windows
  (SQLite sin cerrar) y `pnpm format:check` por archivos con CRLF.
- `go build ./...` es obligatorio antes de pedir un PR.

---

## 12. Discrepancias documentales verificadas

Estas son las que un agente debe corregir o tener en cuenta. **No repetirlas en
código nuevo.**

| # | Dónde | Qué dice | Realidad verificada |
| --- | --- | --- | --- |
| 1 | `README.md` (raíz), líneas ~399 y ~415; `backend/README.md` línea ~469; `backend/.env.example`; `backend/internal/config/config.go` | El frontend corre en `http://localhost:5173` y hay que cambiar `FRONTEND_URL` a ese puerto | Vite corre en **8443** (`fronted/vite.config.ts`, `fronted/README.md`, `docker/compose*.yml`). El ejemplo de 5173 deja CORS desalineado |
| 2 | `crawler/README.md` (~295), `k8s/README.md` (~613), `docs/estandares/formato-datos-extraidos.md` (~9 y ~454) | Remiten al contrato de Ingestion en `backend/README.md` **§12.7** | Esa sección **no existe**. El README del backend termina en §25.6 y tiene un comentario `PENDIENTE` que reconoce que faltan §25.7 y §25.8 |
| 3 | `backend/README.md` final | Termina a mitad de §25.6 (Comparison API) con un TODO sobre la rama TAL-227 | Falta el resto de Comparison API, historial/búsquedas guardadas y el estándar TAL-78 |
| 4 | `scraper/README.md` §4.1 | Ejemplo con `"storeId": 2` para Ripley | `scraper/stores/ripley-cl/spec.json` usa **4**; el propio README §3.1 también dice 4 |
| 5 | `scraper/README.md` §7 | "The Go backend proxies `/admin/scraper/*` to `scraper-api:8095`" | **No existe** ninguna ruta `/admin/scraper` en el backend. El scraper solo se alcanza por red interna o `port-forward` |
| 6 | `k8s/README.md` §1 vs §3.4 | §1: 6 pods y 7 Services en `dominios`; §3.4: 5 pods y 6 Services | Los manifiestos incluyen `scraper-api`: son **6 pods y 7 Services** en `dominios`, y **4 pods y 5 Services** en `consolidado`. La tabla de cuota omite el scraper |
| 7 | `docs/DiagramaSS/README.md` | Contiene un here-string de PowerShell (`@" ... "@ | Set-Content`) en vez de Markdown | El archivo está mal generado: se guardó el comando, no el contenido |
| 8 | `crawler/README.md` | Se presenta como parte vigente del sistema; dice que v1 y v2 conviven "hasta alcanzar paridad" | `crawler/AGENTS.md` lo declara **legado** y ni Compose ni k8s lo despliegan. El README raíz ni lo menciona |
| 9 | `README.md` (raíz) §13 | "El pod de test hace 29 verificaciones" | Son 29 de APIs **más hasta 6 del scraper** (35 en `dominios`). El README de k8s lo detalla bien |
| 10 | `backend/README.md` §11 | "Actualmente existen consultas: products.sql, services.sql, stores.sql" | Correcto, pero hay endpoints nuevos sin documentar (ver #11) |
| 11 | `backend/internal/http/router.go` | No aparecen en ningún README | Existen `POST /products/duplicate-candidates` (admin) y `GET /search/popular-products` |
| 12 | `README.md` (raíz) §4 | Estructura del frontend incluye `src/types/` y omite `crawler/` | No existe `fronted/src/types/`; sí existen `constants/`, `imports/`, `figma/`. `crawler/` existe y no figura |
| 13 | `README.md` (raíz) §9 | "El proyecto dispone de archivos `.env.example`" (plural) | Solo existe `backend/.env.example` |
| 14 | `backend/README.md` §10.1 y §10.4 | Repiten el mismo párrafo de Neon | Duplicación, no contradicción |

### Otras observaciones menores

- `fronted/README.md` documenta `pnpm lint` y `pnpm build`, pero el estándar del
  repo es `pnpm check` (incluye formato y auditoría).
- El diagrama del README raíz dice "NGINX / INGRESS con enrutamiento por rutas".
  En UCT el Ingress solo publica `/`; el enrutamiento por prefijo lo hace nginx
  dentro del pod `frontend`.
- `docs/auditorias/TAL-105-rutas-y-dependencias.md` sigue citando el puerto
  5173 como valor por defecto; es documentación histórica.
- `docs/auditorias/TAL-125-responsabilidades-y-datos.md` afirma que
  `cmd/{server,products-api,stores-api,services-api}` fueron borrados; los
  directorios todavía existen (los tres primeros con `main.go` duplicado).

---

## 13. Contrato mínimo de la API (TAL-78)

- Errores RFC 9457 (`application/problem+json`) con `code` estable y
  `request_id`.
- Usar `httpx.WriteProblem`, `WriteValidation`, `WriteDecodeError`,
  `WriteInternal`. Nada de `http.Error` nuevo.
- `httpx.DecodeJSON`: rechaza campos desconocidos, limita tamaño y exige
  `Content-Type`.
- **400** = falla al decodificar. **422** = decodifica pero incumple reglas
  (`httpx.Validator`, `errors[]` por campo).
- Errores de base: `pgx.ErrNoRows` → 404; `23505` → 409; `23503`/`23514` → 422;
  el resto → 500. Nunca 404/500 como comodín.
- Éxito: 201 + `Location` en POST; 204 en DELETE; POST/PUT devuelven la misma
  forma que el GET.
- Escrituras protegidas por sesión, rol `admin` o token de servicio. El
  `userId` del cliente **no** se confía: sale de la sesión.
- Mensajes al usuario en español; JSON en camelCase; sin detalles internos en
  las respuestas.
- Tests Go junto al código, table-driven con `t.Run`, `httptest` y solo la
  librería estándar. Casos mínimos: feliz, 400, 422, 404 y 500 simulado.

---

## 14. Fuentes de verdad

| Tema | Dónde mirar |
| --- | --- |
| Reglas del repo | `AGENTS.md`, `fronted/AGENTS.md`, `crawler/AGENTS.md` |
| Cómo trabajar por área | `.opencode/skills/` (espejo en `.claude/skills/`) |
| API y errores | `docs/estandares/TAL-78-estandar-api.md` |
| Contrato del scraper | `docs/estandares/formato-datos-extraidos.md` |
| Endpoints y contratos | `backend/README.md` (incompleto al final; ver sección 12) |
| Modelo de datos | `docs/database/modelo-relacional.md` |
| Despliegue | `k8s/README.md` |
| Scraper v2 | `scraper/README.md` |
| Tests | `scripts/testing/reporte.py` y `docs/reportes/testing/` |
| Asistente de comparación con IA (conectado a Gemini) | `assistant/README.md` y `assistant/docs/propuesta-asistente.md` |

**Regla práctica:** antes de afirmar que algo existe, verificar en el código
(`grep`, `router.go`, `domains.go`, `compose.yml`, `k8s/`). Antes de afirmar que
algo está en `develop`, usar `git merge-base --is-ancestor`.

---

## 15. Evidencia de la última verificación

| Comprobación | Resultado |
| --- | --- |
| `cd backend && go build ./...` | OK (2026-11-17) |
| `docker compose -f docker/compose.yml config --quiet` | OK |
| `docker compose -f docker/compose.microservicios.yml config --quiet` | OK |
| Conteo de servicios/Deployments de k8s contra `k8s/base` y `k8s/components` | 6 pods / 7 Services en `dominios`; 4 / 5 en `consolidado` (incluye scraper) |
| `scraper validate` / `pytest` / `ruff` | No ejecutado en esta revisión |
| `go test ./...` | No ejecutado en esta revisión |
| `pnpm check` | No ejecutado en esta revisión |

Si retomas el trabajo, empieza por corregir las discrepancias 1–7 de la
sección 12: son las que hoy pueden hacer perder tiempo o romper la comunicación
local entre frontend y backend.
