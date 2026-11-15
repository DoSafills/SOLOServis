# SOLOServis

Plataforma web para **buscar, comparar y consultar productos y servicios** de distintas tiendas y proveedores de Chile. Toma como referencia a SoloTodo y amplía el enfoque a servicios.

Permite:

- Buscar y filtrar productos y servicios.
- Comparar alternativas y sus atributos.
- Consultar precios, su historial y las tiendas o proveedores que los ofrecen.
- Guardar favoritos, armar un carrito y ver valoraciones.

Un scraper trae ofertas reales de las tiendas y las carga al catálogo.

> **Si usas un agente de IA en este repo**, sus instrucciones están en [`AGENTS.md`](AGENTS.md): stack real, convenciones y pendientes.

---

## Estado (noviembre 2026)

El proyecto está **desplegado y funcionando** en el clúster Kubernetes de la UCT.

| Qué | Estado |
| --- | --- |
| Backend: 14 APIs en Go, agrupadas en 3 procesos de dominio | Hecho |
| Estándar de errores, validación y tests de API (TAL-78) | Hecho |
| Frontend: catálogo, búsqueda, comparación, tiendas, servicios, favoritos y carrito | Hecho, en mejora visual |
| PostgreSQL 18 con migraciones versionadas | Hecho |
| Scraper v2 entregando productos de Falabella y Ripley de punta a punta | Hecho |
| Despliegue en Kubernetes validado (29 de 29 verificaciones) | Hecho |
| Seguimiento de precios y notificaciones | API lista; falta la pantalla en el frontend |
| Reportes de testing, CI y tests del frontend | Pendiente |
| Consultas con IA / NLP | Pendiente |

Evidencia del último despliegue: [`docs/auditorias/TAL-110-despliegue-y-validacion.md`](docs/auditorias/TAL-110-despliegue-y-validacion.md).

---

## Arquitectura

```text
Navegador
   │  HTTPS
   ▼
Ingress ──► frontend (nginx + React)        ← único componente publicado
               │  enruta cada ruta pública a su dominio
               ├──► comercial :8080   product, store, service, pricing, search, comparison,
               │                      favorites, cart, review, watchlist, notification
               └──► usuarios  :8080   auth, user

scraper-api ──► ingestion :8080 ──► PostgreSQL 18 ◄── comercial, usuarios
(Falabella, Ripley)   (token de servicio)
```

- **Las 14 APIs corren en 3 procesos** (`comercial`, `usuarios`, `ingestion`). Los tres usan la misma imagen; la variable `API_DOMAIN` decide qué APIs sirve cada uno.
- **El API Gateway es el nginx del frontend:** el navegador solo conoce un origen.
- **El scraper no toca la base:** entrega los productos a la Ingestion API, que valida y guarda.

---

## Tecnologías

### En uso

| Área | Tecnología |
| --- | --- |
| Backend | Go 1.27, `chi` (router HTTP), `pgx` (driver PostgreSQL), `sqlc` (consultas tipadas) |
| API | REST con JSON; errores en formato RFC 9457 (`application/problem+json`) |
| Base de datos | PostgreSQL 18, migraciones SQL versionadas |
| Frontend | React 19, TypeScript, Vite 8, Tailwind CSS 4, `recharts` |
| Scraping | Python 3.12+, Scrapling, FastAPI (API de control), SQLite para el estado del scraper |
| Contenedores | Docker y Docker Compose |
| Orquestación | Kubernetes con Kustomize (clúster UCT) |
| Autenticación | Sesiones con tokens opacos, contraseñas con PBKDF2-SHA256 y token de servicio entre componentes |
| Calidad | `go vet` y `go test`; ESLint y `oxfmt` en el frontend; `pytest` y `ruff` en el scraper |
| Gestor de paquetes | `pnpm` |

### Previstas, todavía sin implementar

| Tecnología | Para qué |
| --- | --- |
| IA / NLP | Interpretar consultas en lenguaje natural (hoy solo existe la tabla `ai_query_process`) |
| Caché (Redis) | Acelerar lecturas frecuentes del catálogo |
| CI (GitHub Actions) | Correr tests y chequeos en cada Pull Request |
| Tests del frontend | Hoy el frontend solo tiene formato, lint y build |
| Documentación OpenAPI | Publicar los endpoints; hoy están descritos en `backend/README.md` |

### Excluidas

Por las restricciones del proyecto, **PHP y Java** no forman parte del stack.

---

## Estructura del repositorio

```text
backend/            API en Go
  cmd/all-apis/       binario único (API_DOMAIN elige el dominio)
  cmd/<api>/          un binario por API (modo microservicios)
  internal/           una carpeta por API, más httpx, authz y allapis
  database/           migraciones SQL, su orden y las consultas de sqlc
fronted/            frontend en React (la carpeta se llama "fronted")
scraper/            scraper v2: una especificación JSON por tienda
crawler/            crawler v1
docker/             Dockerfiles y archivos de Compose
k8s/                manifiestos de Kubernetes y pod de test
scripts/            despliegue (k8s/) y entorno de pruebas local (local-test/)
docs/               estándares, auditorías, modelo de datos y requisitos
```

---

## Cómo ejecutar

### Requisitos

- Git
- Docker Desktop (incluye Docker Compose)
- Node.js (probado con la versión 24) y `pnpm`, para el frontend
- Go 1.27, solo si vas a correr el backend sin Docker
- Python 3.12 o superior, solo si vas a trabajar en el scraper

### 1. Backend y base de datos con Docker Compose

```bash
docker compose -f docker/compose.yml up -d --build
```

Levanta PostgreSQL (con las migraciones aplicadas), los 3 procesos de dominio y el scraper. Cada API queda en su puerto de siempre:

| Puerto | API | Puerto | API |
| --- | --- | --- | --- |
| 8081 | product | 8088 | auth |
| 8082 | store | 8089 | favorites |
| 8083 | service | 8090 | cart |
| 8084 | user | 8091 | review |
| 8085 | pricing | 8092 | watchlist |
| 8086 | search | 8093 | notification |
| 8087 | comparison | 8094 | ingestion |

El scraper queda en el 8095 y PostgreSQL en el 5432. Para comprobar: `http://localhost:8081/health`.

El modo anterior, con un contenedor por API, sigue disponible en `docker/compose.microservicios.yml`. No levantes los dos a la vez.

### 2. Frontend

```bash
cd fronted
pnpm install
pnpm dev
```

Abre `http://localhost:8443`. El frontend necesita saber dónde está cada API: crea `fronted/.env.local` (no se sube a git) con:

```env
VITE_PRODUCT_API_URL=http://localhost:8081
VITE_STORE_API_URL=http://localhost:8082
VITE_SERVICE_API_URL=http://localhost:8083
VITE_SEARCH_API_URL=http://localhost:8086
VITE_COMPARISON_API_URL=http://localhost:8087
VITE_AUTH_API_URL=http://localhost:8088
VITE_FAVORITES_API_URL=http://localhost:8089
VITE_CART_API_URL=http://localhost:8090
```

### 3. Backend sin Docker (opcional)

```bash
cd backend
cp .env.example .env
go run ./cmd/all-apis
```

En `.env` ajusta `DATABASE_URL` y deja `FRONTEND_URL=http://localhost:8443`, que es el origen que el backend acepta por CORS. Sin `API_DOMAIN`, el proceso sirve las 14 APIs en un solo puerto.

### 4. Kubernetes (clúster UCT)

```powershell
.\scripts\k8s\deploy.ps1 -Modo uct -Registry docker.io/<usuario-docker>
```

```bash
./scripts/k8s/deploy.sh --modo uct --registry docker.io/<usuario-docker>
```

El script despliega por fases, verifica cada una y termina con un pod de test. Kubeconfig, Docker Hub, redeploy y solución de problemas en [`k8s/README.md`](k8s/README.md).

---

## Tests y calidad

```bash
cd backend && go vet ./... && go test ./...
```

```bash
cd fronted && pnpm check
```

```bash
cd scraper && python -m pytest -q && ruff check src tests
```

- Los tests de integración del backend necesitan `TEST_DATABASE_URL` apuntando a un PostgreSQL 18 con las migraciones aplicadas; sin esa variable se omiten.
- `pnpm check` corre formato, lint, build y auditoría de dependencias.

---

## Flujo de desarrollo

```text
Tarea en Linear (TAL-XX) → rama → desarrollo → Pull Request → revisión → tests → merge
```

### Ramas

```text
main        versiones estables; no se desarrolla directo aquí
└── develop   integración del trabajo en curso; los PR apuntan aquí
     ├── feature/TAL-XX-descripcion
     ├── fix/…
     ├── test/…
     ├── docs/…
     └── chore/…
```

### Commits

Pequeños y con el formato `tipo(TAL-XX): descripción`, por ejemplo `fix(TAL-98): crear postgres-secret en el deploy`. Tipos: `feat`, `fix`, `test`, `docs`, `chore`, `refactor`.

### Pull Requests

Un PR debe:

- Apuntar a `develop` y relacionarse con una tarea de Linear.
- Tener un título descriptivo y decir qué cambia y cómo se validó.
- Pasar los tests y los chequeos de la parte que toca.
- Ser revisado por otro integrante: **quien implementó no aprueba su propio PR**.

### Definition of Done

```text
[ ] Funcionalidad implementada y criterios de aceptación cumplidos
[ ] Tests escritos y aprobados
[ ] Lint y build sin errores
[ ] Revisión aprobada por otro integrante
[ ] Pull Request integrada en develop
[ ] Documentación actualizada si cambió el comportamiento
[ ] Tarea de Linear actualizada
```

Para los endpoints del backend aplica además la lista de [`docs/estandares/TAL-78-estandar-api.md`](docs/estandares/TAL-78-estandar-api.md).

---

## Variables de entorno y secretos

- Los archivos `.env` no se suben al repositorio; `backend/.env.example` sirve de referencia.
- En Kubernetes, la contraseña de PostgreSQL y el token de servicio viven en Secrets que crea el script de despliegue. No están en git.
- Las credenciales de `docker/compose.yml` son de desarrollo, solo para uso local.

---

## Documentación

| Tema | Dónde |
| --- | --- |
| Backend: APIs, endpoints y configuración | [`backend/README.md`](backend/README.md) |
| Estándar de API (errores, validación, tests) | [`docs/estandares/TAL-78-estandar-api.md`](docs/estandares/TAL-78-estandar-api.md) |
| Formato de los datos extraídos por el scraper | [`docs/estandares/formato-datos-extraidos.md`](docs/estandares/formato-datos-extraidos.md) |
| Scraper | [`scraper/README.md`](scraper/README.md) |
| Despliegue en Kubernetes | [`k8s/README.md`](k8s/README.md) |
| Modelo de datos | [`docs/database/modelo-relacional.md`](docs/database/modelo-relacional.md) |
| Auditorías y evidencia de despliegue | [`docs/auditorias/`](docs/auditorias/) |
| Requisitos, casos de uso y arquitectura | [`docs/requirements/`](docs/requirements/), [`docs/use_case/`](docs/use_case/), [`docs/architecture/`](docs/architecture/) |

Cada cambio de comportamiento se documenta junto al código: el porqué y el para qué de lo que se agrega, y de qué depende.

---

## Equipo y gestión

Proyecto del curso Taller de Integración III (UCT), desarrollado por un equipo de 6 integrantes. Las tareas se planifican por Sprint y se gestionan en Linear.

---

## Próximos objetivos

1. Reportes de testing y CI en cada Pull Request.
2. Tests del frontend.
3. Más tiendas en el scraper y una alternativa para Mercado Libre, que hoy bloquea el scraping.
4. Consultas con IA / NLP.
5. Caché para las lecturas del catálogo.
