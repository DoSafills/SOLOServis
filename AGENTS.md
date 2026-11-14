# SOLOServis — instrucciones para agentes de IA

Léelo completo antes de modificar el proyecto. Vale para cualquier agente (Claude Code, Copilot, Cursor, Codex). `fronted/AGENTS.md` y `crawler/AGENTS.md` agregan reglas propias de esas carpetas.

**Fuente de verdad:** el código y este archivo. El `README.md` de la raíz está desactualizado (ver sección 3): no lo uses para decidir qué tecnología existe.

---

## 1. Reglas que no se rompen

1. **Propuesta antes de código.** Muestra el diseño (esquema, endpoints, contratos, archivos a tocar) y espera aprobación. Después muestra el diff antes del commit.
2. **Una rama por tarea**, creada desde `develop`. Nunca hagas commit directo en `develop` ni en `main`.
3. **No hagas push ni merge.** El push y el PR los hace la persona. El PR va contra **`develop`** (no `main`).
4. **Commits sin atribución de IA:** nada de `Co-Authored-By` ni "Generated with…".
5. **Migraciones:** las aplicadas no se editan. Todo cambio va en un archivo nuevo.
6. **SQL siempre parametrizado** (`$1`, `$2`…). Nunca concatenes valores.
7. **Secretos:** nunca imprimas ni escribas en git tokens o contraseñas. Los Secrets se nombran, no se muestran. No leas `~/.docker/config.json` ni el kubeconfig.
8. **Reporta lo real:** si un test falla o no se pudo correr, dilo con la salida. No des por validado lo que no ejecutaste.
9. **Responde en español.**

---

## 2. Qué es

Plataforma web para buscar y comparar **productos y servicios** entre tiendas y proveedores de Chile (referencia: SoloTodo, ampliado a servicios). Un scraper trae ofertas reales de las tiendas y las carga al catálogo.

Proyecto del curso Taller de Integración III (UCT). Las tareas viven en Linear con el identificador `TAL-XX`.

---

## 3. Stack

### Implementado (verificado en el código)

| Área | Tecnología |
| --- | --- |
| Backend | Go 1.27, `chi` v5 (router), `pgx` v5 (PostgreSQL), `sqlc` (consultas generadas en `internal/database/generated`), `godotenv` |
| Base de datos | PostgreSQL 18 |
| Frontend | React 19, TypeScript 5.9, Vite 8, Tailwind CSS 4, `recharts`; gestor `pnpm` |
| Calidad frontend | `oxfmt` (formato), ESLint 10 |
| Scraper v2 (`scraper/`) | Python ≥ 3.12, Scrapling, FastAPI + uvicorn (API de control), `httpx`, SQLite para su estado; `pytest` y `ruff` |
| Crawler v1 (`crawler/`) | Python ≥ 3.12, Scrapling, asyncio, SQLite |
| Contenedores | Docker (imágenes multi-etapa, sin root), Docker Compose |
| Orquestación | Kubernetes con Kustomize (clúster UCT), scripts `deploy.ps1` / `deploy.sh` |
| Autenticación | Sesiones con tokens opacos (se guarda su SHA-256), contraseñas con PBKDF2-SHA256; token de servicio `INTERNAL_API_TOKEN` |
| Herramientas git | husky + lint-staged |

### Declarado en el README pero **no implementado**

No hay rastro en el código de: **Redis**, **Scrapy**, **Playwright** (directo), **TanStack Query**, **Prisma**, "Go Workers" ni JSONL como almacenamiento. No las uses ni las des por existentes. Agregar cualquiera es una decisión de arquitectura: proponla primero.

### Previsto, todavía sin implementar

| Qué | Estado |
| --- | --- |
| IA / NLP para consultas | Solo existe la tabla `ai_query_process`; no hay código |
| Caché (Redis) | Declarada en el README; sin implementar |
| CI (GitHub Actions) | No existe `.github/workflows` |
| Tests del frontend | No hay ninguno; solo `pnpm check` |
| Reportes de testing (TAL-168) | Propuesto, sin implementar |

---

## 4. Estructura del repo

```
backend/            API en Go
  cmd/all-apis/       binario único; API_DOMAIN elige qué APIs sirve
  cmd/<api>/          un binario por API (solo para el modo microservicios)
  internal/<api>/     handler, repositorio y tests de cada API
  internal/httpx/     errores problem+json, DecodeJSON, Validator, logs, request id
  internal/authz/     sesiones, rol admin y token de servicio
  internal/allapis/   dominios y URLs internas entre APIs
  database/           migrations/, migration_order.txt, queries/ (sqlc)
fronted/            frontend React (la carpeta se llama "fronted", sin la n)
scraper/            scraper v2: specs por tienda en stores/<slug>/spec.json
crawler/            crawler v1
docker/             Dockerfiles, compose.yml (3 dominios) y compose.microservicios.yml
k8s/                base/, components/{dominios,consolidado,espera-postgres}, tests/
scripts/k8s/        deploy.ps1, deploy.sh
docs/               estandares/, auditorias/, database/, requirements/
```

---

## 5. Arquitectura

**14 APIs en 3 procesos de dominio**, con una sola imagen (`docker/all-apis/Dockerfile`); solo cambia `API_DOMAIN`:

| Dominio | APIs |
| --- | --- |
| `comercial` | product, store, service, pricing, search, comparison, favorites, cart, review, watchlist, notification |
| `usuarios` | auth, user |
| `ingestion` | ingestion |

- Cada proceso escucha en `8080` y expone `GET /health` (nombre del dominio y ping a PostgreSQL).
- Las llamadas entre APIs del mismo dominio van a `127.0.0.1`. La única entre dominios es `ingestion → notification` (`NOTIFICATION_API_URL=http://comercial:8080`).
- **El API Gateway es el nginx del frontend**: enruta cada ruta pública al dominio que corresponde. Solo el frontend se publica.
- **Flujo de datos del scraper:** `scraper-api` extrae → `POST /ingestion/runs`, `/items`, `/finish` en `http://ingestion:8080` con el token de servicio → PostgreSQL → sitio. El scraper **no** tiene credenciales de la base.
- Cada spec declara el `storeId` de su tienda; debe coincidir con la tabla `store` y con el SQLite del scraper.

---

## 6. Flujo de trabajo

| Tema | Convención |
| --- | --- |
| Ramas | `feature/TAL-XX-descripcion`, `fix/…`, `docs/…`, `chore/…`, `test/TAL-XX-…`, desde `develop` |
| Commits | Pequeños, `tipo(TAL-XX): descripción` en español (`feat`, `fix`, `docs`, `test`, `chore`). Sin tarea: `tipo(área): …` |
| PR | Contra `develop`. Descripción breve: qué cambia, cómo se validó, riesgos. No repitas el README |
| Antes de afirmar que algo está en `develop` | Verifícalo (`git merge-base --is-ancestor`): ya hubo PR que quedaron en `main` por error |
| Checkout de la persona | No cambies de rama sin avisar. No toques sus contenedores de Docker (`soloservis_*`) |
| Documentación de una tarea | Se escribe después del código y del redeploy, con evidencia real del clúster |

---

## 7. Backend: estándar de API

Norma completa: `docs/estandares/TAL-78-estandar-api.md`. Lo esencial:

- **Errores:** RFC 9457 (`application/problem+json`) con `code` estable y `request_id`. Usa `httpx.WriteProblem`, `WriteValidation`, `WriteDecodeError`, `WriteInternal`. Nada de `http.Error` nuevo.
- **Body:** `httpx.DecodeJSON` (rechaza campos desconocidos, límite de tamaño, `Content-Type`).
- **400 vs 422:** falla al decodificar → 400; decodifica pero incumple reglas → 422 con `errors[]` por campo (`httpx.Validator`).
- **Errores de base:** `pgx.ErrNoRows` → 404, `23505` → 409, `23503`/`23514` → 422, el resto → 500. Nunca 404 ni 500 como comodín.
- **Éxito:** 201 con `Location` en POST, 204 en DELETE; POST/PUT responden con la misma forma que el GET.
- **Sin detalles internos** en las respuestas (SQL, stack, tablas). Los 5xx se registran con `request_id`.
- **Handlers contra una interfaz** de repositorio, para poder simular errores en los tests.
- **Escrituras protegidas:** sesión, rol `admin` o token de servicio (`internal/authz`). El `userId` del cliente no se confía: se usa el de la sesión.
- **Mensajes para el usuario en español; JSON en camelCase.**

**Tests (Go):** junto al código, table-driven con `t.Run`, `net/http/httptest`, solo la librería estándar. Casos mínimos por endpoint: feliz, entrada inválida (400 y 422), inexistente (404) y error interno simulado (500).

---

## 8. Base de datos

- Las migraciones están en `backend/database/migrations/` y **el orden lo da `migration_order.txt`**, no el nombre del archivo (hay prefijos repetidos como `005_` y `016_`). Una migración nueva se agrega al final de ese archivo.
- Sin `BEGIN`/`COMMIT` propios: el Job las aplica en una transacción y registra su checksum en `schema_migrations`. Editar una aplicada rompe el checksum.
- No dependas de IDs autogenerados fijos: los de `store` cambian según el entorno.
- Si cambias una consulta de `database/queries/`, regenera con `sqlc`.

---

## 9. Frontend (`fronted/`)

- `pnpm` (no `npm`). Servidor de desarrollo: `pnpm dev`, puerto **8443**.
- Antes de entregar: `pnpm check` (formato, lint, build y auditoría).
- Las URLs de las APIs vienen de `VITE_*_API_URL` (`src/services/api/config.ts`); en Kubernetes quedan vacías y todo pasa por el mismo origen.
- Los errores del backend llegan como problem+json: decide con `code`, muestra `detail`.
- Estilos con Tailwind v4. Más reglas en `fronted/AGENTS.md`.

---

## 10. Scraper (`scraper/`)

- **Spec-first:** una tienda nueva es un `stores/<slug>/spec.json`, no código. Documentación del formato en `scraper/README.md`.
- Los campos son rutas del contrato de ingestion (`product.name`, `offer.price`, `product.attributes.<clave>`). La Ingestion API **rechaza campos desconocidos** y exige los atributos obligatorios de la categoría (por ejemplo `capacidad_litros` en `refrigeradores`).
- Contrato de datos: `docs/estandares/formato-datos-extraidos.md`.
- **No intentes saltar protecciones anti-bots.** Mercado Libre redirige al scraper a su verificación de cuenta: se deja fallando.
- Antes de entregar: `pytest` y `ruff check`.

---

## 11. Docker y Kubernetes

- **Local:** `docker compose -f docker/compose.yml up -d --build` (PostgreSQL, 3 dominios y scraper). Publica los puertos históricos 8081–8094 de cada API. El modo de 14 contenedores está en `docker/compose.microservicios.yml`; no levantes los dos a la vez.
- **Clúster UCT:** `scripts/k8s/deploy.ps1 -Modo uct -Registry docker.io/<usuario>`; redeploy sin borrar datos con `-FromPhase 3 -SkipCleanup`. Guía: `k8s/README.md`.
- `deploy.ps1` debe quedar en **ASCII** (PowerShell 5.1) y `deploy.sh` compatible con **bash 3.2**. Si cambias uno, cambia el otro.
- **Cuota del namespace:** 12 pods, 10 Services, 3 PVC, 1 CPU / 1G de requests. El LimitRange exige **mínimo 25m y 64Mi por contenedor**, también en Jobs y CronJobs.
- Los pods corren sin root, con `readOnlyRootFilesystem` y sin capabilities (Pod Security `restricted`).
- Un Service inyecta variables `<NOMBRE>_PORT=tcp://…` en los pods: si chocan con una variable de la app, usa `enableServiceLinks: false`.
- `k8s/overlays/` es generado y está ignorado por git.
- Los Secrets `postgres-secret` y `soloservis-internal` los crea o reutiliza la fase 4 del script; no están en git.

---

## 12. Comandos de verificación

```bash
cd backend && go build ./... && go vet ./... && go test ./...
cd fronted && pnpm check
cd scraper && python -m pytest -q && ruff check src tests
kubectl kustomize k8s/overlays/uct
docker compose -f docker/compose.yml config --quiet
```

- Los tests de integración de Go necesitan `TEST_DATABASE_URL` apuntando a un PostgreSQL 18 con las migraciones aplicadas en el orden de `migration_order.txt`; sin la variable se omiten.
- El pod de test del clúster (`k8s/tests`, fase 10 del deploy) hace 29 verificaciones.

---

## 13. Estado al 2026-11-13

Esta sección envejece rápido: confirma con `git log`, `gh pr list` y Linear antes de darla por cierta.

**Hecho y desplegado en UCT:**
- 14 APIs consolidadas en 3 dominios, con el estándar de errores TAL-78.
- PostgreSQL con secretos fuera de git y espera a la base antes de arrancar las APIs (TAL-98).
- Docker Compose con los 3 procesos (TAL-109).
- Despliegue validado, 29 de 29 en el pod de test (TAL-110). Evidencia: `docs/auditorias/TAL-110-despliegue-y-validacion.md`.
- Scraper v2 entregando productos reales de Falabella y Ripley de punta a punta.

**Pendiente:**
- Sprint 3, centrado en testing: TAL-168 (reportes de testing) y varios PR de pruebas abiertos.
- Sin CI y sin tests en el frontend.
- Mercado Libre bloqueado por anti-bots; evaluar su API oficial.
- Que ingestion resuelva la tienda por nombre en vez de por ID.
- El README de la raíz no refleja el stack ni la arquitectura actuales.
- `lint-staged` de la raíz apunta a rutas que no existen (`backend/**/*.ts`, `frontend/`): el pre-commit no formatea nada.
- Restos sin uso: `backend/src/database/*.ts` y `fronted/package-lock.json` junto al `pnpm-lock.yaml`.

---

## 14. Documentos de referencia

| Tema | Archivo |
| --- | --- |
| Estándar de API | `docs/estandares/TAL-78-estandar-api.md` |
| Contrato de datos del scraper | `docs/estandares/formato-datos-extraidos.md` |
| Despliegue en Kubernetes | `k8s/README.md` |
| Inventario y evidencia del clúster | `docs/auditorias/TAL-95-inventario-despliegue.md`, `TAL-110-despliegue-y-validacion.md` |
| Rutas y dependencias entre APIs | `docs/auditorias/TAL-105-rutas-y-dependencias.md` |
| Modelo de datos | `docs/database/modelo-relacional.md` |
| Scraper | `scraper/README.md` |
