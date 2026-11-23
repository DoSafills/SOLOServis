# SOLOServis — instrucciones para agentes de IA

Léelo completo antes de modificar el proyecto. Vale para cualquier agente (Claude Code, Copilot, Cursor, Codex). `fronted/AGENTS.md` y `crawler/AGENTS.md` agregan reglas propias de esas carpetas.

**Fuente de verdad:** el código. Si este archivo o un README lo contradicen, gana el código: avisa y corrige el documento en el mismo cambio.

## Arnés de agentes

Este archivo es el mapa. El detalle operativo de cada área vive en skills, con disparador, comandos verificados y definición de término:

| Skill | Cuándo se usa |
| --- | --- |
| `backend-api` | Endpoints, handlers, repositorios y tests de Go |
| `scraper-spec` | Scraper, tiendas nuevas, extracción e ingesta |
| `frontend-ui` | React, Vite y Tailwind en `fronted/` |
| `database-migrations` | Esquema, `migration_order.txt` y sqlc |
| `k8s-deploy` | Despliegue, manifiestos y pod de test |
| `linear-workflow` | Estados, comentarios y enlaces de tareas TAL |
| `quality-gate` | Qué correr antes de un commit o PR, y fallas conocidas |

El contenido canónico está en `.opencode/skills/`; `.claude/skills/` es su espejo porque cada agente lee de su propio directorio. Si editas uno, sincroniza el otro:

```bash
./scripts/check-skills.sh --sync     # .opencode -> .claude
./scripts/check-skills.sh            # falla si divergieron
```

No repitas en un skill lo que ya dice este archivo: aquí va **qué** se hace y por qué; en el skill, **cómo** se hace en ese directorio.

---

## 1. Reglas que no se rompen

1. **Propuesta antes de código.** Muestra el diseño (esquema, endpoints, contratos, archivos a tocar) y espera aprobación. Después muestra el diff antes del commit.
2. **Una rama por dominio**, creada desde `develop` (§6). Nunca hagas commit directo en `develop` ni en `main`.
3. **Push y PR solo cuando la persona lo pide; nunca merge.** Sin un pedido explícito, deja los commits en la rama local y entrega el texto del PR para que la persona lo abra. El PR va contra **`develop`** (no `main`).
4. **Commits sin atribución de IA:** nada de `Co-Authored-By` ni "Generated with…".
5. **Migraciones:** las aplicadas no se editan. Todo cambio va en un archivo nuevo.
6. **SQL siempre parametrizado** (`$1`, `$2`…). Nunca concatenes valores.
7. **Secretos:** nunca imprimas ni escribas en git tokens o contraseñas. Los Secrets se nombran, no se muestran. No leas `~/.docker/config.json` ni el kubeconfig.
8. **Reporta lo real:** si un test falla o no se pudo correr, dilo con la salida. No des por validado lo que no ejecutaste.
9. **Responde en español.**
10. **Toda función se explica.** Cada función, método o handler que agregues o modifiques lleva un comentario que diga qué hace, qué recibe y qué devuelve, y por qué existe si no es obvio. En Go, comentario de documentación (`// NombreFuncion …`); en Python, docstring; en TypeScript, JSDoc (`/** … */`). Lo pidió el scrum master para todo el proyecto: si tocas un archivo con funciones sin explicar, explícalas en el mismo cambio.

---

## 2. Qué es

Plataforma web para buscar y comparar **productos y servicios** entre tiendas y proveedores de Chile (referencia: SoloTodo, ampliado a servicios). Un scraper trae ofertas reales de las tiendas y las carga al catálogo.

Proyecto del curso Taller de Integración III (UCT). Las tareas viven en Linear con el identificador `TAL-XX`; se pueden consultar desde la terminal con `scripts/linear/linear.py` (ver `scripts/linear/README.md`).

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
| Asistente de comparación (IA) | Proveedor agnóstico por configuración: `AI_PROVIDER`/`AI_MODEL`/`AI_BASE_URL`/`AI_API_KEY`, sin recompilar. En el clúster corre DeepSeek (`deepseek-chat`, camino `compatible`); Gemini y cualquier API compatible OpenAI entran cambiando esas variables. Código en `backend/internal/assistant/`; contexto cerrado a 2–5 productos, guardrails, caché en memoria y auditoría en `ai_comparison_query`. Sin clave responde 503 `ai_unavailable` (no hay respuestas de plantilla). Config y prompt en `assistant/`. |
| Herramientas git | husky + lint-staged |

### **No** implementado (figuró en versiones antiguas del README)

No hay rastro en el código de: **Redis**, **Scrapy**, **Playwright** (directo), **TanStack Query**, **Prisma**, **NestJS**, "Go Workers" ni JSONL como almacenamiento. No las uses ni las des por existentes. Agregar cualquiera es una decisión de arquitectura: proponla primero.

### Previsto, todavía sin implementar

| Qué | Estado |
| --- | --- |
| Caché (Redis) | Prevista; sin implementar |
| Documentación OpenAPI | Prevista; hoy los endpoints están en `backend/README.md` |
| Pantallas de seguimiento de precios y notificaciones | Las APIs `watchlist` y `notification` existen; el frontend no las usa |
| Tests del frontend | No hay ninguno; solo `pnpm check` |
| Visor de llamadas en el pod de test | Pedido por el scrum master: un sistema de pruebas que recree el flujo de datos (scraper → ingestion → catálogo → gateway) y el uso de las APIs desde el pod de test, y que muestre cada llamada en **dos ventanas**: en una, lo que envía el origen; en la otra, lo que recibe el destino. Parte de `scripts/testing/salud.py` (TAL-164) y `scripts/testing/flujo_productos.py` (TAL-184), que ya comparan lo enviado con lo recibido. Diseño pendiente: proponlo antes de implementarlo |

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
assistant/          asistente de comparación: config, prompt y allowlist (fuente de assets)
docker/             Dockerfiles, compose.yml (3 dominios) y compose.microservicios.yml
k8s/                base/, components/{dominios,consolidado,espera-postgres}, tests/
scripts/k8s/        deploy.ps1, deploy.sh
scripts/linear/     linear.py: CLI de Linear (necesita LINEAR_API_KEY)
scripts/testing/    reporte.py: reporte de testing de todas las suites
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
| Ramas | Desde 2026-11-20, **una rama por dominio**, no por tarea: `feat/api`, `feat/backend`, `feat/frontend`, `feat/test` u otra según el área (por ejemplo `feat/scraper`, `feat/k8s`, `docs/…`). Se crean desde `develop`, que sigue siendo la rama de integración. El ticket va en el commit, no en el nombre de la rama. Antes de empezar, actualiza la rama con `git merge origin/develop`: la comparten varias tareas y se desfasa rápido |
| Commits | Pequeños, `tipo(TAL-XX): descripción` en español (`feat`, `fix`, `docs`, `test`, `chore`). Sin tarea: `tipo(área): …` |
| PR | Contra `develop`. Descripción breve: qué cambia, cómo se validó, riesgos. No repitas el README |
| Antes de afirmar que algo está en `develop` | Verifícalo (`git merge-base --is-ancestor`): ya hubo PR que quedaron en `main` por error |
| Checkout de la persona | No cambies de rama sin avisar. No toques sus contenedores de Docker (`soloservis_*`) |
| Documentación de una tarea | Se escribe después del código y del redeploy, con evidencia real del clúster |
| A quién va un ticket | Según el área del problema: frontend (`fronted/`) → Cristian; APIs del backend y Kubernetes → Marcelo; scraper y crawler → Kevin; lo demás → Ricardo (scrum master). Revisa en el código dónde está la causa antes de asignarlo |

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
- El runner (`k8s/base/migrations/migrate.sh`) **no** envuelve cada archivo en una transacción: una migración que deba aplicarse completa o nada lleva su propio `BEGIN`/`COMMIT` o un solo bloque `DO`. El runner registra el checksum en `schema_migrations`; editar una aplicada rompe el checksum.
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
- Las NetworkPolicy están en `k8s/components/red`. UCT no deja crearlas (el rol solo las lee), así que el deploy las omite; se aplican solo si `kubectl auth can-i create networkpolicies` responde `yes`.
- Los Secrets `postgres-secret`, `soloservis-internal` y `soloservis-ai` los crea o reutiliza la fase 4 del script; no están en git. `soloservis-ai` no se puede generar al azar: la fase 4 la toma de `AI_API_KEY` en el entorno del despliegue, y sin ella avisa y sigue (el asistente responde 503).

---

## 12. Comandos de verificación

```bash
cd backend && go build ./... && go vet ./... && go test ./...
cd fronted && pnpm check
cd scraper && python -m pytest -q && ruff check src tests
kubectl kustomize k8s/overlays/uct
bash scripts/k8s/validate-manifests.sh
docker compose -f docker/compose.yml config --quiet
```

- Los tests de integración de Go necesitan `TEST_DATABASE_URL` apuntando a un PostgreSQL 18 con las migraciones aplicadas en el orden de `migration_order.txt`; sin la variable se omiten. Con la variable, corre `go test -p 1 ./...`: los paquetes comparten la base y en paralelo se pisan los datos.
- `scripts/k8s/validate-manifests.sh` valida los manifiestos contra el esquema de Kubernetes (kubeconform, modo estricto) sin tocar el clúster; `kubectl kustomize` solo arma el YAML y no detecta un campo mal indentado.
- El pod de test del clúster (`k8s/tests`, fase 10 del deploy) hace 35 verificaciones.
- **Health check (TAL-164):** `python scripts/testing/salud.py` revisa el despliegue en cualquier momento (pods, `/health`, gateway, host público, migraciones y cuota), sin escribir nada. Es lo primero que se corre cuando alguien avisa que el sitio no funciona. Con `--detalle` lista cada pedido con lo esperado y lo obtenido; `python scripts/testing/reporte.py --salud` lo suma al reporte de testing.
- **Reporte de testing (TAL-168):** `python scripts/testing/reporte.py` corre todas las suites y deja el resultado en `docs/reportes/testing/` (`--postgres-docker` suma los tests de integración). Córrelo antes de pedir un PR: `go build ./...` tiene que pasar siempre.

---

## 13. Estado al 2026-11-19

Esta sección envejece rápido: confirma con `git log`, `gh pr list` y Linear antes de darla por cierta.

**Hecho y desplegado en UCT:**
- 14 APIs consolidadas en 3 dominios, con el estándar de errores TAL-78.
- PostgreSQL con secretos fuera de git y espera a la base antes de arrancar las APIs (TAL-98).
- Docker Compose con los 3 procesos (TAL-109).
- Despliegue validado, 29 de 29 en el pod de test (TAL-110). Evidencia: `docs/auditorias/TAL-110-despliegue-y-validacion.md`.
- Scraper v2 entregando productos reales de Falabella y Ripley de punta a punta.
- Asistente de comparación (IA) conectado: `POST /comparisons/assistant` con DeepSeek (`deepseek-chat`) en el clúster, contexto cerrado, guardrails, caché y auditoría. La respuesta es siempre la del modelo: sin `AI_API_KEY` responde 503 `ai_unavailable` y el backend no arma texto por su cuenta. El proveedor se cambia por configuración (`AI_PROVIDER`, `AI_BASE_URL`, `AI_MODEL`) sin recompilar; el camino compatible-OpenAI se prueba con un servidor simulado.

**Pendiente:**
- Sprint 3, centrado en testing. El reporte de `scripts/testing/reporte.py` todavía muestra 2 tests del crawler que fallan en Windows (conexión SQLite sin cerrar).
- CI en `.github/workflows/ci.yml` (TAL-235) corre backend, scraper, frontend, skills, compose y manifiestos de Kubernetes, pero **no** los tests de integración de Go (no levanta PostgreSQL): un PR con migraciones puede quedar verde en CI y romper `TEST_DATABASE_URL`. Antes de mergear migraciones, córrelos a mano (§12). El frontend sigue sin tests.
- `pnpm format:check` ya pasa (no quedan archivos con CRLF), pero el CI todavía lo excluye: falta activarlo en `ci.yml`.
- El frontend no tiene pantallas para seguimiento de precios ni notificaciones.
- Mercado Libre bloqueado por anti-bots; evaluar su API oficial.
- Que ingestion resuelva la tienda por nombre en vez de por ID.
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
| Asistente de comparación (IA) | `assistant/README.md` |
