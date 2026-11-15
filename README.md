# SOLOServis

**Plataforma de búsqueda, comparación y seguimiento de productos y servicios.**

SOLOServis es una plataforma web orientada a centralizar información de productos, servicios, tiendas y proveedores, permitiendo a los usuarios buscar alternativas, comparar características y precios, y facilitar sus decisiones de compra o contratación.

El proyecto busca ofrecer una experiencia de consulta unificada mediante la integración de diferentes fuentes de información, procesamiento automatizado de datos y servicios independientes organizados por dominios funcionales.

---

## 1. Objetivo del proyecto

Desarrollar una plataforma que permita:

- Buscar productos y servicios desde un catálogo centralizado.
- Consultar información y características de diferentes alternativas.
- Comparar productos y servicios.
- Consultar precios y ofertas de distintas tiendas.
- Identificar tiendas y proveedores.
- Gestionar favoritos, carrito y listas de seguimiento.
- Administrar usuarios y sesiones.
- Obtener y actualizar información mediante procesos automatizados de extracción de datos.
- Facilitar el seguimiento de variaciones de precios.

La arquitectura está diseñada para permitir la incorporación de nuevas funcionalidades, fuentes de información y servicios.

---

## 2. Arquitectura del sistema

SOLOServis utiliza una arquitectura distribuida por dominios funcionales, donde el frontend consume servicios HTTP y el backend administra la lógica de negocio y el acceso a PostgreSQL.

El sistema se compone de cinco elementos principales:

1. **Frontend:** interfaz de usuario y consumo de APIs.
2. **Backend:** lógica de negocio y servicios HTTP.
3. **Base de datos:** almacenamiento relacional de la información.
4. **Scraper:** extracción, procesamiento e integración de información externa.
5. **Infraestructura:** construcción, ejecución y despliegue de los servicios.

### Diagrama general

```text
                       USUARIO
                          |
                          v
              +------------------------+
              |       FRONTEND         |
              |   React + TypeScript   |
              |         Vite           |
              +-----------+------------+
                          |
                      HTTP / REST
                          |
                          v
              +------------------------+
              |      NGINX / INGRESS   |
              |   Enrutamiento por     |
              |        rutas           |
              +-----------+------------+
                          |
            +-------------+-------------+
            |             |             |
            v             v             v
     +------------+ +------------+ +------------+
     | COMERCIAL  | |  USUARIOS  | | INGESTION  |
     |            | |            | |            |
     | 11 APIs    | |  2 APIs    | |  1 API     |
     +------+-----+ +------+-----+ +------+-----+
            |             |              ^
            |             |              |
            v             v              |
       +---------------------------------+
       |           POSTGRESQL            |
       |     Base de datos principal     |
       +---------------------------------+
                                         |
                                         |
                              +----------+----------+
                              |       SCRAPER       |
                              |    Python / API     |
                              | Extracción de datos |
                              +---------------------+
```

El diagrama representa las relaciones funcionales principales. Los servicios de backend comparten una instancia de PostgreSQL y el scraper integra información mediante el dominio de Ingestion.

### Distribución de los dominios

Las 14 APIs del backend se organizan en tres dominios:

| Dominio | APIs | Responsabilidad |
|---|---|---|
| Comercial | Product, Store, Service, Pricing, Search, Comparison, Favorites, Cart, Review, Watchlist, Notification | Catálogo, consultas, precios, comparaciones e interacción comercial. |
| Usuarios | Auth, User | Autenticación, sesiones y administración de usuarios. |
| Ingestion | Ingestion | Recepción e integración de información procesada por el scraper. |

Los tres dominios utilizan la misma imagen del backend Go (`cmd/all-apis`), diferenciándose mediante la variable de entorno `API_DOMAIN`.

También existe una topología alternativa denominada `consolidado`, que ejecuta las 14 APIs dentro de un único Deployment.

---

## 3. Tecnologías utilizadas

| Componente | Tecnología | Función |
|---|---|---|
| Frontend | React | Construcción de la interfaz. |
| Lenguaje frontend | TypeScript | Tipado y mantenimiento del código. |
| Build frontend | Vite | Desarrollo y construcción de la aplicación. |
| Estilos | Tailwind CSS | Diseño y estilos de interfaz. |
| Backend | Go | Lógica de negocio y servicios HTTP. |
| Router HTTP | Chi | Enrutamiento de solicitudes. |
| Acceso a datos | pgx | Conexión y comunicación con PostgreSQL. |
| Generación SQL | sqlc | Generación de código Go a partir de consultas SQL. |
| Base de datos | PostgreSQL | Persistencia de información principal. |
| Scraper | Python | Extracción y procesamiento de información. |
| Framework de scraping | Scrapling | Obtención y extracción de contenido web. |
| API del scraper | FastAPI | Control y administración de ejecuciones. |
| Almacenamiento scraper | SQLite / JSON | Registro de ejecuciones y catálogos. |
| Contenedores | Docker | Construcción y ejecución de servicios. |
| Orquestación | Docker Compose | Administración del entorno local. |
| Orquestación de producción | Kubernetes | Despliegue y administración de servicios. |
| Servidor frontend | Nginx | Servir la aplicación y enrutar solicitudes. |

---

## 4. Estructura del repositorio

```text
SOLOServis/
│
├── fronted/
│   ├── src/
│   │   ├── components/
│   │   ├── pages/
│   │   ├── services/
│   │   ├── hooks/
│   │   ├── types/
│   │   └── assets/
│   ├── public/
│   └── package.json
│
├── backend/
│   ├── cmd/
│   │   └── all-apis/
│   ├── internal/
│   │   ├── allapis/
│   │   ├── config/
│   │   ├── database/
│   │   ├── server/
│   │   └── ...
│   ├── database/
│   │   ├── migrations/
│   │   ├── queries/
│   │   └── migration_order.txt
│   ├── .env.example
│   ├── go.mod
│   └── README.md
│
├── scraper/
│   ├── stores/
│   ├── src/
│   │   └── scraper/
│   ├── tests/
│   ├── pyproject.toml
│   └── README.md
│
├── docker/
│   ├── all-apis/
│   ├── frontend/
│   ├── scraper/
│   ├── compose.yml
│   └── compose.microservicios.yml
│
├── k8s/
│   ├── base/
│   ├── components/
│   ├── overlays/
│   ├── tests/
│   └── README.md
│
├── scripts/
│   └── k8s/
│       ├── deploy.sh
│       └── deploy.ps1
│
├── docs/
│
├── package.json
├── pnpm-lock.yaml
├── .gitignore
└── README.md
```

**Importante:** el directorio del frontend se denomina actualmente `fronted/` en el repositorio. Se conserva ese nombre para utilizar las rutas existentes.

La estructura puede evolucionar conforme se incorporen nuevas funcionalidades y documentación.

---

## 5. Componentes del sistema

### 5.1 Frontend

El frontend es responsable de presentar la información y permitir la interacción de los usuarios con la plataforma.

Tecnologías principales:

- React.
- TypeScript.
- Vite.
- Tailwind CSS.
- Recharts.

Responsabilidades:

- Presentación del catálogo.
- Navegación entre secciones.
- Búsqueda y aplicación de filtros.
- Consulta de productos y servicios.
- Visualización y comparación de precios.
- Gestión de favoritos y carrito.
- Comunicación con las APIs mediante solicitudes HTTP.

Documentación: `fronted/` y sus archivos de implementación.

### 5.2 Backend

El backend concentra la lógica de negocio y expone las APIs necesarias para el funcionamiento de la plataforma.

Está desarrollado en Go y utiliza Chi para el enrutamiento HTTP, pgx para la conexión con PostgreSQL y sqlc para generar código de acceso a datos.

Responsabilidades:

- Administración del catálogo.
- Gestión de tiendas y servicios.
- Consulta y procesamiento de precios.
- Búsqueda y comparación.
- Autenticación y gestión de usuarios.
- Favoritos, carrito y seguimiento.
- Administración de reseñas.
- Recepción de información externa.

Los servicios utilizan una configuración compartida y ejecutan la validación de conexión con PostgreSQL al iniciar.

Documentación: [`backend/README.md`](backend/README.md).

### 5.3 Base de datos

PostgreSQL es la base de datos principal de SOLOServis.

Se utiliza para almacenar información relacionada con:

- Usuarios y sesiones.
- Productos y categorías.
- Marcas y especificaciones.
- Tiendas y proveedores.
- Ofertas y precios.
- Historial de precios.
- Reseñas.
- Información de seguimiento e integración.

La estructura y evolución del esquema se administran mediante migraciones SQL.

El orden de ejecución se define en:

```text
backend/database/migration_order.txt
```

Las migraciones deben respetar el orden establecido y deben registrarse en este archivo cuando se incorporen nuevas versiones del esquema.

### 5.4 Scraper

El scraper es el componente encargado de obtener información desde diferentes tiendas y fuentes externas.

La implementación actual utiliza una arquitectura basada en especificaciones versionadas por tienda.

Responsabilidades:

- Extraer información de productos.
- Procesar precios y características.
- Validar la estructura de la información obtenida.
- Registrar ejecuciones.
- Mantener catálogos por tienda.
- Programar tareas recurrentes.
- Enviar información procesada al backend mediante Ingestion.

El scraper utiliza SQLite para registrar ejecuciones y almacenar su información operativa, además de archivos JSON para los catálogos.

Su API de control utiliza el puerto `8095` y permite administrar ejecuciones y tareas.

Documentación: [`scraper/README.md`](scraper/README.md).

### 5.5 Infraestructura

La infraestructura permite ejecutar los distintos componentes de manera independiente y reproducible.

Se utilizan:

- Docker para construir imágenes.
- Docker Compose para el entorno local.
- Nginx para servir el frontend y enrutar las solicitudes.
- Kubernetes para administrar el despliegue.
- PersistentVolumeClaims para persistencia de datos.
- Jobs para ejecutar migraciones y pruebas.
- Ingress para exponer la aplicación.

Documentación: [`k8s/README.md`](k8s/README.md).

---

## 6. Instalación y ejecución local

### Requisitos

Antes de comenzar se recomienda contar con:

| Herramienta | Versión / requisito |
|---|---|
| Git | Instalado y configurado. |
| Node.js | 24 para el entorno frontend. |
| pnpm | 11.24.0 para el frontend. |
| Go | 1.27.1 según `backend/go.mod`. |
| Python | 3.12 o superior para el scraper. |
| Docker | Docker Desktop o Docker Engine. |
| Docker Compose | Disponible mediante Docker Compose V2. |

### 6.1 Clonar el repositorio

```bash
git clone https://github.com/DoSafills/SOLOServis.git

cd SOLOServis
```

### 6.2 Levantar servicios de backend, base de datos y scraper

Desde la raíz del proyecto:

```bash
docker compose -f docker/compose.yml up -d --build
```

Este comando inicia:

- PostgreSQL.
- Dominio Comercial.
- Dominio Usuarios.
- Dominio Ingestion.
- Scraper.

La primera inicialización de PostgreSQL utiliza el volumen de datos y el procedimiento de migraciones configurado para el entorno.

Comprobar el estado:

```bash
docker compose -f docker/compose.yml ps
```

Consultar los registros:

```bash
docker compose -f docker/compose.yml logs -f
```

Detener los servicios:

```bash
docker compose -f docker/compose.yml down
```

**Importante:** `docker compose down -v` elimina también los volúmenes y puede borrar los datos persistidos de PostgreSQL y del scraper. No utilizarlo cuando se necesite conservar información.

### 6.3 Ejecutar el frontend

Desde otra terminal:

```bash
cd fronted
```

Instalar dependencias:

```bash
pnpm install --frozen-lockfile
```

Ejecutar el entorno de desarrollo:

```bash
pnpm dev
```

Vite expondrá la dirección local indicada en la terminal, normalmente:

```text
http://localhost:5173
```

Para conectar el frontend de desarrollo con los servicios de Docker Compose, crear `fronted/.env.local` con las siguientes direcciones:

```env
VITE_PRODUCTS_API_URL=http://localhost:8081
VITE_STORES_API_URL=http://localhost:8082
VITE_SERVICES_API_URL=http://localhost:8083
VITE_SEARCH_API_URL=http://localhost:8086
VITE_COMPARISON_API_URL=http://localhost:8087
VITE_AUTH_API_URL=http://localhost:8088
VITE_FAVORITES_API_URL=http://localhost:8089
VITE_CART_API_URL=http://localhost:8090
```

Para este modo de ejecución, el `FRONTEND_URL` de los servicios Go debe coincidir con el origen del frontend, es decir, `http://localhost:5173`. El archivo `docker/compose.yml` utiliza inicialmente `http://localhost:8443`, por lo que debe ajustarse para trabajar directamente con Vite.

Después de modificar las variables de entorno de Compose, recrear los servicios correspondientes.

---

## 7. Verificación y calidad del código

La validación del proyecto debe contemplar los diferentes componentes.

### Frontend

Desde `fronted/`:

```bash
pnpm check
```

Este comando ejecuta:

- Verificación de formato.
- ESLint.
- Build de producción.
- Auditoría de dependencias.

También se pueden ejecutar individualmente:

```bash
pnpm format:check
pnpm lint
pnpm build
pnpm audit
```

### Backend

Desde `backend/`:

```bash
go test ./...
```

Las pruebas que requieren una base de datos deben contar con su configuración de pruebas correspondiente.

### Scraper

Desde `scraper/`, instalar las dependencias de desarrollo y ejecutar:

```bash
python -m pytest
```

También se puede validar la estructura de las especificaciones de las tiendas:

```bash
python -m scraper validate
```

---

## 8. Despliegue en Kubernetes

SOLOServis cuenta con scripts de despliegue para el clúster universitario UCT.

La topología predeterminada es `dominios`:

| Deployment | Responsabilidad |
|---|---|
| comercial | 11 APIs comerciales. |
| usuarios | Autenticación y usuarios. |
| ingestion | Recepción de información del scraper. |
| frontend | Aplicación web y Nginx. |
| postgres | Base de datos con almacenamiento persistente. |
| scraper-api | Servicio de extracción y control de tareas. |

Además, se utiliza un CronJob para disparar ejecuciones del scraper cuando corresponde.

### 8.1 Despliegue desde PowerShell

```powershell
.\scripts\k8s\deploy.ps1 -Modo uct -Registry docker.io/<usuario-docker>
```

### 8.2 Despliegue desde Bash

```bash
./scripts/k8s/deploy.sh --modo uct --registry docker.io/<usuario-docker>
```

Los scripts realizan las siguientes operaciones:

1. Verificación de herramientas y acceso al clúster.
2. Preparación del almacenamiento y configuración.
3. Construcción y publicación de imágenes Docker.
4. Configuración de credenciales y despliegue de PostgreSQL.
5. Ejecución de migraciones.
6. Despliegue de APIs y scraper.
7. Despliegue del frontend.
8. Configuración del Ingress.
9. Pruebas de persistencia y recuperación de servicios.
10. Ejecución de pruebas de integración mediante el pod de test.

### 8.3 Topología alternativa

También existe una topología denominada `consolidado`, donde las 14 APIs se ejecutan en un único Deployment llamado `backend`.

PowerShell:

```powershell
.\scripts\k8s\deploy.ps1 -Modo uct -Topologia consolidado -Registry docker.io/<usuario-docker>
```

Bash:

```bash
./scripts/k8s/deploy.sh --modo uct --topologia consolidado --registry docker.io/<usuario-docker>
```

Esta alternativa permite reducir el consumo de recursos del clúster.

### 8.4 Documentación de despliegue

Para conocer los requisitos, configuración de kubeconfig, parámetros, fases, pruebas y solución de problemas, consultar:

[`k8s/README.md`](k8s/README.md).

---

## 9. Seguridad y configuración

Las configuraciones sensibles deben manejarse mediante variables de entorno y Kubernetes Secrets.

Entre las variables utilizadas se encuentran:

| Variable | Función |
|---|---|
| `DATABASE_URL` | Dirección de conexión a PostgreSQL. |
| `PORT` | Puerto de ejecución del servicio. |
| `FRONTEND_URL` | Origen autorizado para solicitudes del frontend. |
| `API_DOMAIN` | Dominio de APIs que ejecuta el proceso Go. |
| `GATEWAY_URL` | URL común para llamadas internas entre APIs. |
| `INTERNAL_API_TOKEN` | Autorización de operaciones internas. |
| `SCRAPER_API_TOKEN` | Autorización de la API de control del scraper. |
| `INGESTION_API_URL` | Dirección del servicio de integración. |

Los archivos `.env` y kubeconfig no deben incorporarse al repositorio.

El proyecto dispone de archivos `.env.example` como referencia de configuración.

No se deben publicar contraseñas, tokens, credenciales de Docker Hub ni archivos de acceso al clúster.

---

## 10. Flujo de desarrollo

El desarrollo se organiza mediante tareas, ramas y Pull Requests.

Flujo general:

```text
       LINEAR
          |
          v
    ASIGNACIÓN DE TAREA
          |
          v
     CREACIÓN DE RAMA
          |
          v
     IMPLEMENTACIÓN
          |
          v
     PRUEBAS LOCALES
          |
          v
     PULL REQUEST
          |
          v
      CODE REVIEW
          |
          v
      APROBACIÓN
          |
          v
        MERGE
          |
          v
       COMPLETADO
```

### Estrategia de ramas

```text
main
 |
 develop
 |
 +-- feature/*
 |
 +-- fix/*
 |
 +-- refactor/*
 |
 +-- docs/*
```

| Rama | Responsabilidad |
|---|---|
| `main` | Versiones estables del proyecto. |
| `develop` | Integración del desarrollo actual. |
| `feature/*` | Nuevas funcionalidades. |
| `fix/*` | Corrección de errores. |
| `refactor/*` | Mejoras estructurales del código. |
| `docs/*` | Creación y actualización de documentación. |

No se debe desarrollar directamente sobre `main`.

### Pull Requests

Toda modificación relevante debe integrarse mediante Pull Request.

Una Pull Request debe:

- Tener un título descriptivo.
- Explicar el problema que resuelve.
- Describir los cambios realizados.
- Estar asociada a la tarea correspondiente.
- Incluir las verificaciones realizadas.
- Contar con revisión de otro integrante.

El desarrollador que implementó una funcionalidad no debe aprobar su propia Pull Request.

---

## 11. Documentación técnica

La documentación del proyecto debe permitir comprender no solamente qué hace una funcionalidad, sino también:

- Por qué existe.
- Para qué se utiliza.
- Cómo funciona.
- Qué entradas y salidas maneja.
- Qué componentes intervienen.
- De qué otras funciones o servicios depende.
- Qué efectos produce en el sistema.
- Cómo verificar su correcto funcionamiento.

La documentación debe mantenerse separada por componente, siguiendo una estructura coherente con el código fuente.

### Documentación existente

| Documento | Contenido |
|---|---|
| [`backend/README.md`](backend/README.md) | Arquitectura, configuración y funcionamiento del backend. |
| [`scraper/README.md`](scraper/README.md) | Arquitectura, especificaciones de tiendas y funcionamiento del scraper. |
| [`k8s/README.md`](k8s/README.md) | Instalación, despliegue, operación y pruebas en Kubernetes. |

El directorio `docs/` centraliza documentación técnica, auditorías y material complementario del proyecto.

Toda funcionalidad nueva o modificación importante debe actualizar su documentación correspondiente.

---

## 12. Definition of Done

Una tarea puede considerarse terminada cuando cumple los siguientes criterios:

```text
[ ] Funcionalidad implementada.
[ ] Criterios de aceptación cumplidos.
[ ] Validaciones de entradas y salidas realizadas.
[ ] Pruebas correspondientes ejecutadas.
[ ] Sin errores críticos conocidos.
[ ] Revisión de código realizada.
[ ] Code Review aprobado.
[ ] Build y verificaciones correspondientes exitosos.
[ ] Documentación actualizada.
[ ] Pull Request integrada.
[ ] Tarea de linear actualizada.
```

---

## 13. Estado del proyecto

SOLOServis cuenta con frontend, backend, base de datos, scraper y mecanismos de despliegue mediante Docker y Kubernetes.

El desarrollo continúa mediante la incorporación de funcionalidades y mejoras sobre los componentes existentes.

Las líneas de trabajo incluyen:

- Mejoras de consistencia visual y experiencia de usuario.
- Validación y robustecimiento de las APIs.
- Mejoras de extracción y procesamiento de información.
- Pruebas de integración y despliegue.
- Mantenimiento de la base de datos.
- Mejoras de rendimiento y estabilidad.
- Integración de nuevas capacidades de automatización e inteligencia artificial.

---

## 14. Repositorio

**GitHub:** https://github.com/DoSafills/SOLOServis

**Documentación principal:** `README.md`

**Documentación de backend:** `backend/README.md`

**Documentación de scraper:** `scraper/README.md`

**Documentación de Kubernetes:** `k8s/README.md`

---

**SOLOServis — Plataforma de búsqueda, comparación y seguimiento de productos y servicios.**