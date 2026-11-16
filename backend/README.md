# SOLOServis — Backend

Backend de **SOLOServis**, encargado de administrar la lógica de negocio, procesar solicitudes HTTP, gestionar la información de la plataforma y comunicarse con PostgreSQL.

Está desarrollado en **Go**, utilizando Chi para el enrutamiento HTTP, pgx para la conexión con PostgreSQL y sqlc para generar código Go a partir de consultas SQL.

El backend está organizado en 14 APIs, agrupadas en tres dominios funcionales: Comercial, Usuarios e Ingestion. Esta separación permite mantener responsabilidades independientes y facilita el despliegue mediante diferentes topologías.

---

# 1. Objetivo y responsabilidades

El backend tiene como objetivo centralizar la lógica de negocio y proporcionar los servicios necesarios para el funcionamiento de SOLOServis.

Sus principales responsabilidades son:

- Gestionar productos y sus características.
- Administrar tiendas y proveedores.
- Consultar precios, ofertas e historial de precios.
- Gestionar categorías y atributos dinámicos.
- Procesar búsquedas y comparaciones.
- Administrar usuarios, sesiones y autenticación.
- Gestionar favoritos, carrito y listas de seguimiento.
- Administrar reseñas y notificaciones.
- Recibir información proveniente del scraper.
- Validar y persistir información en PostgreSQL.
- Comunicar diferentes APIs cuando una funcionalidad depende de otros servicios.

El backend actúa como intermediario entre el frontend, la base de datos y los procesos externos de integración de información.

---

# 2. Arquitectura general

El backend utiliza una arquitectura organizada por dominios y responsabilidades.

Las solicitudes llegan mediante HTTP, son procesadas por la API correspondiente y, cuando es necesario, se realizan consultas a PostgreSQL o solicitudes a otros servicios internos.

## 2.1 Diagrama de arquitectura

```text
                      FRONTEND
                 React + TypeScript
                         |
                         | HTTP / JSON
                         v
                 NGINX / INGRESS
                         |
                         v
              +----------------------+
              |      BACKEND         |
              |                      |
              |    14 APIs Go        |
              +----------+-----------+
                         |
          +--------------+--------------+
          |              |              |
          v              v              v
    +-----------+  +-----------+  +-----------+
    | COMERCIAL |  | USUARIOS  |  | INGESTION |
    |           |  |           |  |           |
    | 11 APIs   |  | 2 APIs    |  | 1 API     |
    +-----+-----+  +-----+-----+  +-----+-----+
          |              |              |
          +--------------+--------------+
                         |
                         v
                  +-------------+
                  | PostgreSQL  |
                  | soloservis  |
                  +-------------+
                         ^
                         |
                    Ingestion API
                         ^
                         |
                   +-----------+
                   |  SCRAPER  |
                   |  Python   |
                   +-----------+
```

El diagrama representa las relaciones principales. Los servicios de backend utilizan una instancia compartida de PostgreSQL y el scraper entrega información mediante Ingestion API.

## 2.2 Flujo de una solicitud

Ejemplo: consulta de productos.

```text
Frontend
   |
   | GET /products
   v
NGINX
   |
   v
Product API
   |
   v
Handler
   |
   v
Repository
   |
   v
sqlc / pgx
   |
   v
PostgreSQL
   |
   v
Resultado de consulta
   |
   v
DTO
   |
   v
JSON Response
   |
   v
Frontend
```

El frontend nunca debe conectarse directamente a PostgreSQL.

Toda operación de consulta o modificación de información debe pasar por los servicios correspondientes.

---

# 3. Tecnologías utilizadas

| Tecnología | Función |
|---|---|
| Go | Lenguaje principal del backend. |
| Chi | Router HTTP para administrar rutas y middleware. |
| PostgreSQL | Base de datos relacional. |
| pgx/v5 | Driver y administración de conexiones PostgreSQL. |
| pgxpool | Pool de conexiones a la base de datos. |
| sqlc | Generación de código Go a partir de consultas SQL. |
| godotenv | Carga de variables de entorno desde `.env`. |
| Docker | Contenedorización de servicios. |
| Docker Compose | Ejecución local de los servicios. |
| Kubernetes | Orquestación de servicios en el clúster UCT. |

---

# 4. Estructura del proyecto

La estructura actual se organiza de la siguiente manera:

```text
backend/
│
├── cmd/
│   ├── all-apis/
│   ├── auth-api/
│   ├── cart-api/
│   ├── comparison-api/
│   ├── favorites-api/
│   ├── ingestion-api/
│   ├── notification-api/
│   ├── pricing-api/
│   ├── product-api/
│   ├── review-api/
│   ├── search-api/
│   ├── service-api/
│   ├── store-api/
│   ├── user-api/
│   └── watchlist-api/
│
├── internal/
│   ├── allapis/
│   ├── apiclient/
│   ├── apiutil/
│   ├── auth/
│   ├── authz/
│   ├── cart/
│   ├── categories/
│   ├── comparison/
│   ├── config/
│   ├── database/
│   │   └── generated/
│   ├── favorites/
│   ├── features/
│   ├── http/
│   ├── httpx/
│   ├── ingestion/
│   ├── integration/
│   ├── notifications/
│   ├── pricing/
│   ├── products/
│   ├── reviews/
│   ├── runner/
│   ├── search/
│   ├── server/
│   ├── services/
│   ├── stores/
│   └── watchlist/
│
├── database/
│   ├── migrations/
│   ├── queries/
│   ├── migration_order.txt
│   └── Readme.md
│
├── .env.example
├── go.mod
├── go.sum
├── sqlc.yaml
└── README.md
```

### Responsabilidades de las carpetas principales

| Directorio | Responsabilidad |
|---|---|
| `cmd/` | Puntos de entrada de las aplicaciones y APIs. |
| `internal/allapis/` | Registro, distribución y agrupación de las APIs. |
| `internal/config/` | Lectura y administración de configuración. |
| `internal/database/` | Conexión con PostgreSQL y código generado. |
| `internal/http/` | Routers y registro de rutas HTTP. |
| `internal/httpx/` | Utilidades HTTP, validación, errores y middleware. |
| `internal/auth/` | Funcionalidades de autenticación. |
| `internal/authz/` | Autorización y validación de sesiones. |
| `internal/apiclient/` | Comunicación HTTP entre APIs. |
| `internal/features/` | Funcionalidades compartidas y módulos de dominio. |
| `database/migrations/` | Versiones de cambios del esquema de PostgreSQL. |
| `database/queries/` | Consultas SQL utilizadas por sqlc. |

**Importante:** las funcionalidades no necesariamente tienen un único paquete por API. Algunos módulos se encuentran agrupados en `internal/features` o comparten implementaciones y utilidades.

Antes de crear una carpeta nueva, se debe revisar si la funcionalidad ya pertenece a un módulo existente.

---

# 5. APIs y dominios funcionales

El backend cuenta con 14 APIs.

Para facilitar su administración, se agrupan en tres dominios.

## 5.1 Dominio Comercial

Contiene las funcionalidades relacionadas con el catálogo, precios y operaciones comerciales.

| API | Responsabilidad | Ruta principal |
|---|---|---|
| Product API | Productos, detalles, características y categorías. | `/products` |
| Store API | Tiendas y sus ofertas. | `/stores` |
| Service API | Servicios y proveedores. | `/services` |
| Pricing API | Precios, ofertas e historial de precios. | `/offers` |
| Search API | Búsqueda, filtros y facetas. | `/search` |
| Comparison API | Comparación de productos y servicios. | `/comparisons` |
| Favorites API | Favoritos de usuarios. | `/favorites` |
| Cart API | Administración del carrito. | `/cart` |
| Review API | Reseñas de productos, tiendas y servicios. | `/reviews` |
| Watchlist API | Seguimiento de productos y precios objetivo. | `/watchlist` |
| Notification API | Administración de notificaciones. | `/notifications` |

## 5.2 Dominio Usuarios

Contiene las funcionalidades relacionadas con las cuentas y sesiones.

| API | Responsabilidad | Ruta principal |
|---|---|---|
| Auth API | Registro, login, refresh, logout y validación de sesiones. | `/auth` |
| User API | Información y perfil de usuarios. | `/users` |

## 5.3 Dominio Ingestion

Responsable de recibir y procesar información externa.

| API | Responsabilidad | Ruta principal |
|---|---|---|
| Ingestion API | Recepción, validación y persistencia de información del scraper. | `/ingestion` |

La API de Ingestion permite integrar información obtenida externamente sin que el scraper tenga acceso directo a PostgreSQL.

---

# 6. Ejecución y distribución de las APIs

El proyecto utiliza un punto de entrada principal:

```text
cmd/all-apis/main.go
```

Este ejecutable permite iniciar las 14 APIs o solamente las correspondientes a un dominio, utilizando la variable de entorno `API_DOMAIN`.

## 6.1 Topología por dominios

Es la topología utilizada por Docker Compose y Kubernetes.

```text
API_DOMAIN=comercial
API_DOMAIN=usuarios
API_DOMAIN=ingestion
```

Cada dominio se ejecuta como un proceso independiente, utilizando la misma imagen y el mismo binario.

| Proceso | APIs |
|---|---:|
| comercial | 11 |
| usuarios | 2 |
| ingestion | 1 |
| Total | 14 |

Cada proceso escucha internamente en el puerto `8080`.

## 6.2 Topología consolidada

Cuando la variable `API_DOMAIN` está vacía, el proceso registra las 14 APIs.

```text
API_DOMAIN=""
```

Esta alternativa permite ejecutar todas las APIs dentro de un único proceso.

Es útil para desarrollo, pruebas y entornos donde se dispone de recursos limitados.

## 6.3 Funcionamiento del router consolidado

El archivo:

```text
internal/allapis/router.go
```

asigna cada prefijo de ruta a su API correspondiente.

Por ejemplo:

```text
/products      -> Product API
/stores        -> Store API
/services      -> Service API
/offers        -> Pricing API
/search        -> Search API
/comparisons   -> Comparison API
/auth          -> Auth API
/cart          -> Cart API
/favorites     -> Favorites API
/reviews       -> Review API
/watchlist     -> Watchlist API
/notifications -> Notification API
/ingestion     -> Ingestion API
/users         -> User API
```

Las rutas de una API no incluida en un determinado dominio responden con `404`.

El contrato HTTP de los servicios se mantiene independientemente de la topología utilizada.

---

# 7. Comunicación entre APIs

Algunas funcionalidades necesitan consultar información de otros servicios.

Para mantener esta comunicación se utiliza:

```text
internal/apiclient/
```

Este cliente centraliza las solicitudes HTTP y la transferencia de información JSON.

## 7.1 Dependencias actuales

| API que realiza la solicitud | API requerida | Motivo |
|---|---|---|
| Favorites | Product | Consultar información de productos. |
| Cart | Product | Validar productos asociados. |
| Cart | Pricing | Consultar ofertas y precios. |
| Review | Product | Validar productos. |
| Review | Store | Validar tiendas. |
| Review | Service | Validar servicios. |
| Watchlist | Pricing | Consultar información de precios. |
| Pricing | Notification | Generar notificaciones. |
| Ingestion | Notification | Notificar cambios de precios. |

Estas dependencias deben actualizarse cuando se agreguen nuevas llamadas entre APIs.

## 7.2 Configuración de las conexiones

Las URLs se resuelven mediante variables de entorno.

| Variable | Servicio |
|---|---|
| `PRODUCT_API_URL` | Product API. |
| `STORE_API_URL` | Store API. |
| `SERVICE_API_URL` | Service API. |
| `PRICING_API_URL` | Pricing API. |
| `NOTIFICATION_API_URL` | Notification API. |
| `GATEWAY_URL` | URL general de respaldo. |

El orden de resolución es:

```text
1. URL específica de la API.
          |
          v
2. GATEWAY_URL.
          |
          v
3. URL local predeterminada.
```

Cuando dos APIs se encuentran dentro del mismo proceso, `all-apis` configura automáticamente la comunicación hacia ese proceso.

Cuando una API necesita acceder a otro dominio, debe contar con la URL específica o con `GATEWAY_URL`.

Por ejemplo, Ingestion necesita acceder a Notification mediante:

```env
NOTIFICATION_API_URL=http://comercial:8080
```

en el entorno de Docker Compose.

---

# 8. Configuración del backend

La configuración se administra principalmente mediante:

```text
internal/config/config.go
```

Las variables pueden obtenerse del entorno de ejecución o del archivo `.env`.

## 8.1 Variables de entorno

| Variable | Función |
|---|---|
| `PORT` | Puerto de ejecución HTTP. |
| `DATABASE_URL` | Cadena de conexión a PostgreSQL. |
| `FRONTEND_URL` | Origen autorizado para CORS. |
| `API_DOMAIN` | Dominio que ejecutará el proceso. |
| `GATEWAY_URL` | URL común para llamadas entre APIs. |
| `PRODUCT_API_URL` | Dirección interna de Product API. |
| `STORE_API_URL` | Dirección interna de Store API. |
| `SERVICE_API_URL` | Dirección interna de Service API. |
| `PRICING_API_URL` | Dirección interna de Pricing API. |
| `NOTIFICATION_API_URL` | Dirección interna de Notification API. |
| `INTERNAL_API_TOKEN` | Token para operaciones internas autorizadas. |
| `LOG_LEVEL` | Nivel de registro de eventos. |

## 8.2 Archivo `.env`

Crear el archivo a partir de:

```text
.env.example
```

Ejemplo para desarrollo local:

```env
PORT=8080
FRONTEND_URL=http://localhost:5173
DATABASE_URL=postgres://postgres:postgres@localhost:5432/soloservis

API_DOMAIN=
GATEWAY_URL=
INTERNAL_API_TOKEN=
LOG_LEVEL=
```

La URL de ejemplo de PostgreSQL debe coincidir con las credenciales y el puerto del entorno local.

El archivo `.env` no debe subirse al repositorio cuando contenga credenciales reales.

---

# 9. Base de datos PostgreSQL

PostgreSQL es la base de datos principal del backend.

Su responsabilidad es almacenar la información persistente utilizada por las APIs.

Entre sus principales entidades se encuentran:

```text
Usuarios
Sesiones
Productos
Categorías
Atributos
Marcas
Tiendas
Servicios
Ofertas
Precios
Historial de precios
Favoritos
Carritos
Reseñas
Comparaciones
Notificaciones
Datos de integración
```

## 9.1 Conexión

La conexión se administra desde:

```text
internal/database/postgres.go
```

Se utiliza:

```go
pgxpool.Pool
```

para administrar un conjunto reutilizable de conexiones.

Configuración actual del pool:

| Parámetro | Valor |
|---|---:|
| Máximo de conexiones | 10 |
| Mínimo de conexiones | 2 |
| Duración máxima | 1 hora |
| Tiempo máximo inactivo | 30 minutos |
| Timeout de conexión | 5 segundos |
| Statement timeout | 30 segundos |

Al iniciar el backend se verifica la conexión mediante `Ping`.

Si PostgreSQL no está disponible, el proceso no continúa su ejecución.

---

# 10. Migraciones de base de datos

Las migraciones SQL se encuentran en:

```text
database/migrations/
```

El orden oficial de aplicación se mantiene en:

```text
database/migration_order.txt
```

Este archivo define explícitamente qué migraciones deben ejecutarse y en qué orden.

No se debe depender del orden alfabético ni del número de prefijo para determinar la secuencia.

## 10.1 Funcionamiento

El proyecto utiliza un runner de migraciones ubicado en:

```text
k8s/base/migrations/migrate.sh
```

Este procedimiento:

1. Lee el manifiesto `migration_order.txt`.
2. Verifica que los archivos estén disponibles.
3. Comprueba las migraciones ya aplicadas.
4. Ejecuta solamente las migraciones pendientes.
5. Registra los nombres y checksums en `schema_migrations`.
6. Evita aplicar nuevamente migraciones previamente registradas.

Una migración nueva debe incorporarse tanto al directorio de migraciones como al manifiesto.

## 10.2 Reglas para crear migraciones

- No modificar migraciones que ya fueron aplicadas.
- No reutilizar el nombre de una migración existente.
- Mantener un orden explícito.
- Registrar cada archivo nuevo en `migration_order.txt`.
- Verificar las relaciones, restricciones y tipos de datos.
- Comprobar compatibilidad con los datos existentes.
- Probar la migración sobre una base de datos limpia.

**Importante:** una base antigua sin registro de migraciones requiere establecer un baseline verificado antes de utilizar el runner. No debe asumirse que su esquema coincide con todas las migraciones existentes.

## 10.3 Aplicación manual y bases remotas

Docker aplica el runner automáticamente al inicializar un volumen PostgreSQL vacío. Para un volumen existente con `schema_migrations`, se ejecuta el runner para aplicar las versiones pendientes. Una base antigua con esquema pero sin registro se rechaza intencionalmente: primero requiere establecer un baseline verificado; el runner no asume que migraciones o seeds se hayan aplicado.

Para aplicar manualmente migraciones puntuales sobre el contenedor local (por ejemplo, las de categorías de TAL-71), desde la raíz del repositorio en PowerShell:

```powershell
docker compose -f docker/compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U postgres -d soloservis -f /docker-entrypoint-initdb.d/007_service_cart.sql
docker compose -f docker/compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U postgres -d soloservis -f /docker-entrypoint-initdb.d/008_normalize_prices_to_clp.sql
docker compose -f docker/compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U postgres -d soloservis -f /docker-entrypoint-initdb.d/012_categories.sql
docker compose -f docker/compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U postgres -d soloservis -f /docker-entrypoint-initdb.d/013_categories_seed.sql
docker compose -f docker/compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U postgres -d soloservis -f /docker-entrypoint-initdb.d/018_category_artworks.sql
```

El mismo runner admite una base remota como Neon mediante `DATABASE_URL`. En PowerShell, configura `NEON_DATABASE_URL` desde tu gestor de secretos y ejecuta desde la raíz del repositorio:

```powershell
$env:DATABASE_URL = $env:NEON_DATABASE_URL
if (-not $env:DATABASE_URL) { throw "NEON_DATABASE_URL no está configurada." }
docker compose -f docker/compose.yml run --rm --no-deps -e DATABASE_URL postgres bash /docker-entrypoint-initdb.d/001_migrations.sh
```

Documentación complementaria:

```text
database/Readme.md
```

---

# 11. Acceso a datos y sqlc

El backend utiliza `sqlc` para generar código Go a partir de consultas SQL.

Configuración:

```text
sqlc.yaml
```

Consultas:

```text
database/queries/
```

Archivos generados:

```text
internal/database/generated/
```

Actualmente existen consultas relacionadas con:

```text
products.sql
services.sql
stores.sql
```

## 11.1 Flujo de generación

```text
Consultas SQL
      |
      v
   sqlc.yaml
      |
      v
  sqlc generate
      |
      v
Código Go generado
      |
      v
internal/database/generated/
      |
      v
Repositories
```

Para regenerar el código después de modificar las consultas:

```bash
cd backend
sqlc generate
```

Los archivos generados no deben modificarse manualmente.

---

# 12. Contrato HTTP y validaciones

Las APIs deben mantener contratos consistentes para facilitar la integración con el frontend y otros servicios.

El backend utiliza utilidades compartidas en:

```text
internal/httpx/
```

Entre sus responsabilidades se encuentran:

- Decodificación de JSON.
- Validación de parámetros.
- Validación de cuerpos HTTP.
- Construcción de respuestas de error.
- Registro de solicitudes.
- Recuperación ante errores de ejecución.

## 12.1 Middleware

Las APIs utilizan middleware compartido para:

1. Generar o propagar un identificador de solicitud (`request_id`).
2. Registrar información de acceso.
3. Recuperarse de errores de ejecución.
4. Configurar CORS.

## 12.2 Formato de errores

Se utiliza el estándar de errores basado en:

```text
application/problem+json
```

Ejemplo:

```json
{
  "type": "about:blank",
  "title": "Bad Request",
  "status": 400,
  "detail": "Los parámetros de la consulta no son válidos.",
  "instance": "/products",
  "code": "invalid_parameter",
  "request_id": "example-123",
  "errors": [
    {
      "field": "limit",
      "code": "max",
      "message": "El límite solicitado supera el máximo permitido."
    }
  ]
}
```

Los errores deben permitir identificar el problema sin exponer detalles internos de PostgreSQL u otras dependencias.

Documentación normativa:

```text
docs/estandares/TAL-78-estandar-api.md
```

---

# 13. Autenticación y autorización

El sistema utiliza Auth API para administrar la autenticación y las sesiones.

Las operaciones de usuario deben identificar al usuario a través de su sesión validada.

Paquetes principales:

```text
internal/auth/
internal/authz/
```

## 13.1 Responsabilidades

| Componente | Responsabilidad |
|---|---|
| Auth API | Registro, login, refresh y logout. |
| Authz | Validación de sesiones y permisos. |
| User API | Información asociada a los usuarios. |
| INTERNAL_API_TOKEN | Autorización de operaciones internas. |

## 13.2 Reglas de seguridad

- No confiar en el `userId` enviado por el frontend como mecanismo de autorización.
- El usuario de una operación debe obtenerse desde la sesión validada.
- Las operaciones protegidas requieren la autorización correspondiente.
- Las escrituras administrativas deben validar privilegios.
- Las operaciones internas deben utilizar el token correspondiente.
- Los errores no deben revelar credenciales ni información sensible.

Por ejemplo, enviar un `userId` diferente en una solicitud no debe permitir acceder a la información de otro usuario.

---

# 14. Integración con Ingestion y Scraper

El backend recibe información externa mediante Ingestion API.

El scraper no debe conectarse directamente a PostgreSQL.

El flujo de integración es:

```text
Scraper
   |
   | HTTP / JSON
   v
Ingestion API
   |
   v
Validación de información
   |
   v
Procesamiento
   |
   +------> Product
   |
   +------> Pricing
   |
   +------> Notification
   |
   v
PostgreSQL
```

La API de Ingestion administra ejecuciones de integración y procesa los productos y ofertas recibidos.

Las operaciones internas están protegidas por autorización administrativa o `INTERNAL_API_TOKEN`, según corresponda.

Documentación relacionada:

```text
scraper/README.md
```

---

# 15. Endpoints y servicios

Las APIs distribuyen sus rutas mediante prefijos.

| API | Prefijo |
|---|---|
| Product | `/products`, `/product-categories`, `/categories` |
| Store | `/stores` |
| Service | `/services` |
| User | `/users` |
| Pricing | `/offers` |
| Search | `/search` |
| Comparison | `/comparisons` |
| Ingestion | `/ingestion` |
| Auth | `/auth` |
| Cart | `/cart` |
| Favorites | `/favorites` |
| Notification | `/notifications` |
| Review | `/reviews` |
| Watchlist | `/watchlist` |

Cada API dispone de:

```http
GET /health
```

para verificar el estado del servicio y su conexión con PostgreSQL.

No todas las rutas tienen exposición pública. Algunas operaciones están reservadas para comunicación interna o requieren autenticación.

## 15.1 Endpoints `/categories` (TAL-71)

Categorías unificadas de productos y servicios con sus **definiciones de atributos dinámicos**. Son la base de la validación de atributos (TAL-72), los filtros (TAL-75, `filterable`) y la comparación (TAL-76, `comparable`).

- Servicio: **Product API** (`cmd/product-api`, puerto 8081). Código en `internal/categories`.
- Solo lectura: `POST`/`PUT`/`PATCH`/`DELETE` responden `405`. No hay endpoints de escritura (requerirían autenticación).
- Errores con el estándar TAL-78 (`application/problem+json`, ver `docs/estandares/TAL-78-estandar-api.md`).
- Incluye el validador de atributos (TAL-72) que usan los endpoints de escritura de productos y servicios.
- Solo se exponen categorías con `active = TRUE`.

### Esquema (migraciones `012_categories.sql`, `013_categories_seed.sql` y `018_category_artworks.sql`)

Se numeran 012 y 013 porque 009–011 son de TAL-67/68. Están registradas al final de `backend/database/migration_order.txt`: el runner rechaza cualquier `.sql` que no figure en ese manifiesto.

| Tabla | Columnas principales | Restricciones en BD |
| --- | --- | --- |
| `categories` | `slug`, `name`, `item_type` (`product`/`service`), `description`, `active`, `legacy_product_category_id`, `legacy_service_category_id`, timestamps | `slug` UNIQUE + CHECK `^[a-z0-9-]{1,64}$`; CHECK de `item_type`; enlace legado UNIQUE, FK `ON DELETE SET NULL` y del mismo tipo de ítem |
| `category_attributes` | `key`, `label`, `data_type`, `unit`, `required`, `comparable`, `filterable`, `min_value`, `max_value`, `max_length`, `allowed_values`, `display_order` | FK a `categories` `ON DELETE CASCADE`; UNIQUE `(category_id, key)`; CHECK `key ^[a-z][a-z0-9_]{0,63}$`; `data_type` ∈ `string`, `integer`, `decimal`, `boolean`, `enum`; `allowed_values` solo y obligatorio en `enum`; `min`/`max` solo numéricos (enteros en `integer`, `min <= max`); `max_length` solo `string` (1–255); `unit` solo numéricos |
| `category_artworks` | `artwork_key`, `svg_content`, `updated_at` | Clave primaria; contiene los SVG estáticos de las categorías |

Las tablas antiguas (`product_category`, `service_category` y sus `*_specification`) **no se modificaron**. Las columnas `legacy_*` enlazan cada categoría nueva con su equivalente antigua para migrar productos y servicios en TAL-70/72.

Seeds: `refrigeradores`, `notebooks`, `televisores` (producto) e `internet-fibra-optica` (servicio), con atributos de los cinco `data_type`.

### `GET /categories/artworks/{key}`

Devuelve la ilustración SVG guardada en `category_artworks` con `Content-Type: image/svg+xml`. Las ilustraciones se cargan mediante la migración `018_category_artworks.sql`.

### `GET /categories`

| Parámetro | Tipo | Defecto | Regla |
| --- | --- | --- | --- |
| `itemType` | query | (todos) | `product` o `service` |
| `limit` | query | 20 | entero entre 1 y **100** |
| `offset` | query | 0 | entero ≥ 0 |

Orden: por `name`. Un `limit` mayor a 100 **no se recorta**: responde 400. El listado usa 2 consultas fijas (conteo y página), sin N+1.

```http
GET /categories?limit=2
```

```json
{
  "items": [
    {
      "slug": "internet-fibra-optica",
      "name": "Internet Fibra Óptica",
      "itemType": "service",
      "description": "Planes de internet hogar por fibra óptica",
      "attributeCount": 5
    },
    {
      "slug": "notebooks",
      "name": "Notebooks",
      "itemType": "product",
      "description": "Notebooks y computadores portátiles",
      "attributeCount": 6
    }
  ],
  "total": 4,
  "limit": 2,
  "offset": 0
}
```

Sin resultados (por ejemplo, `offset` mayor que `total`), `items` es `[]`.

### `GET /categories/{slug}`

El `slug` se valida contra `^[a-z0-9-]{1,64}$` **antes** de consultar la base. Los atributos vienen ordenados por `displayOrder`. Las restricciones que no aplican al `dataType` van como `null` (el contrato siempre trae todas las claves). El detalle usa 2 consultas fijas.

```http
GET /categories/internet-fibra-optica
```

```json
{
  "slug": "internet-fibra-optica",
  "name": "Internet Fibra Óptica",
  "itemType": "service",
  "description": "Planes de internet hogar por fibra óptica",
  "attributes": [
    { "key": "velocidad_bajada_mbps", "label": "Velocidad de bajada", "dataType": "integer", "unit": "Mbps", "required": true, "comparable": true, "filterable": true, "min": 1, "max": 10000, "maxLength": null, "allowedValues": null, "displayOrder": 1 },
    { "key": "velocidad_subida_mbps", "label": "Velocidad de subida", "dataType": "integer", "unit": "Mbps", "required": true, "comparable": true, "filterable": true, "min": 1, "max": 10000, "maxLength": null, "allowedValues": null, "displayOrder": 2 },
    { "key": "tipo_contrato", "label": "Tipo de contrato", "dataType": "enum", "unit": null, "required": false, "comparable": true, "filterable": true, "min": null, "max": null, "maxLength": null, "allowedValues": ["Sin permanencia", "12 meses", "24 meses"], "displayOrder": 3 },
    { "key": "ip_fija", "label": "IP fija", "dataType": "boolean", "unit": null, "required": false, "comparable": true, "filterable": true, "min": null, "max": null, "maxLength": null, "allowedValues": null, "displayOrder": 4 },
    { "key": "router_incluido", "label": "Router incluido", "dataType": "string", "unit": null, "required": false, "comparable": true, "filterable": false, "min": null, "max": null, "maxLength": 60, "allowedValues": null, "displayOrder": 5 }
  ]
}
```

### Errores de `/categories`

| Caso | Status | `code` | `errors[]` |
| --- | --- | --- | --- |
| `itemType` inválido | 400 | `invalid_parameter` | `itemType` / `one_of` |
| `limit` no entero / < 1 / > 100 | 400 | `invalid_parameter` | `limit` / `invalid_type`, `min`, `max` |
| `offset` no entero / < 0 | 400 | `invalid_parameter` | `offset` / `invalid_type`, `min` |
| `slug` con formato inválido | 400 | `invalid_parameter` | `slug` / `format` |
| Categoría inexistente o inactiva | 404 | `not_found` | — |
| Error de base de datos | 500 | `internal_error` | — (el detalle solo va al log, con el `request_id`) |
| Método distinto de GET | 405 | — (respuesta de chi en texto plano; pendiente de TAL-78) | — |

Los parámetros de query inválidos se reportan todos juntos:

```http
GET /categories?limit=1000000&itemType=foo
```

```json
{
  "type": "about:blank",
  "title": "Bad Request",
  "status": 400,
  "detail": "Los parámetros de la consulta no son válidos.",
  "instance": "/categories",
  "code": "invalid_parameter",
  "request_id": "ejemplo-123",
  "errors": [
    { "field": "itemType", "code": "one_of", "message": "itemType debe ser product o service" },
    { "field": "limit", "code": "max", "message": "limit no puede superar 100" }
  ]
}
```

```http
GET /categories/Refri_Geradores
```

```json
{
  "type": "about:blank",
  "title": "Bad Request",
  "status": 400,
  "detail": "El slug de la categoría no es válido.",
  "instance": "/categories/Refri_Geradores",
  "code": "invalid_parameter",
  "request_id": "ejemplo-123",
  "errors": [
    { "field": "slug", "code": "format", "message": "slug debe tener entre 1 y 64 caracteres: minúsculas, dígitos o guiones" }
  ]
}
```

```http
GET /categories/no-existe
```

```json
{
  "type": "about:blank",
  "title": "Not Found",
  "status": 404,
  "detail": "La categoría no existe.",
  "instance": "/categories/no-existe",
  "code": "not_found",
  "request_id": "ejemplo-123"
}
```

---

# 16. Ejecución local

La forma recomendada de ejecutar el backend completo es mediante Docker Compose.

Desde la raíz del repositorio:

```bash
docker compose -f docker/compose.yml up -d --build
```

Este comando inicia:

- PostgreSQL.
- Dominio Comercial.
- Dominio Usuarios.
- Dominio Ingestion.
- Scraper.

El frontend se ejecuta independientemente desde `fronted/`.

## 16.1 Comprobar los servicios

```bash
docker compose -f docker/compose.yml ps
```

## 16.2 Consultar logs

```bash
docker compose -f docker/compose.yml logs -f
```

Consultar solamente Comercial:

```bash
docker compose -f docker/compose.yml logs -f comercial
```

Consultar solamente Usuarios:

```bash
docker compose -f docker/compose.yml logs -f usuarios
```

Consultar solamente Ingestion:

```bash
docker compose -f docker/compose.yml logs -f ingestion
```

## 16.3 Detener los servicios

```bash
docker compose -f docker/compose.yml down
```

No utilizar `down -v` cuando se necesite conservar los datos persistidos.

---

# 17. Ejecución directa con Go

También es posible ejecutar el backend directamente sin construir sus imágenes Docker.

Para ello, PostgreSQL debe estar funcionando y debe existir una configuración válida.

Desde la raíz del proyecto:

```bash
docker compose -f docker/compose.yml up -d postgres
```

Después, ingresar al directorio:

```bash
cd backend
```

Configurar `.env` con la conexión correspondiente a PostgreSQL local.

Ejecutar todas las APIs:

```bash
go run ./cmd/all-apis
```

Por defecto:

```text
PORT=8080
API_DOMAIN=""
```

Esto inicia las 14 APIs dentro de un único proceso.

## 17.1 Ejecutar un dominio específico

PowerShell:

```powershell
$env:API_DOMAIN="comercial"
$env:PORT="8080"
go run ./cmd/all-apis
```

Bash:

```bash
API_DOMAIN=comercial PORT=8080 go run ./cmd/all-apis
```

Los valores disponibles para `API_DOMAIN` son:

```text
comercial
usuarios
ingestion
```

Cuando se ejecutan dominios por separado, se deben configurar las URLs de las APIs que se encuentren en otros procesos.

Para ejecutar el sistema completo, Docker Compose es la alternativa recomendada.

---

# 18. Puertos utilizados

En la topología de Docker Compose, los procesos escuchan en `8080` dentro de la red interna y se publican los siguientes puertos históricos hacia el equipo local:

| Componente | Puerto local |
|---|---:|
| PostgreSQL | 5432 |
| Product API | 8081 |
| Store API | 8082 |
| Service API | 8083 |
| User API | 8084 |
| Pricing API | 8085 |
| Search API | 8086 |
| Comparison API | 8087 |
| Auth API | 8088 |
| Favorites API | 8089 |
| Cart API | 8090 |
| Review API | 8091 |
| Watchlist API | 8092 |
| Notification API | 8093 |
| Ingestion API | 8094 |

El scraper utiliza el puerto `8095`.

---

# 19. Pruebas y validación

Las modificaciones del backend deben validarse antes de integrarse a `develop`.

## 19.1 Compilación

Desde `backend/`:

```bash
go build ./...
```

Permite verificar que los paquetes del backend compilen correctamente.

## 19.2 Pruebas unitarias

```bash
go test ./...
```

Ejecuta las pruebas automatizadas disponibles.

## 19.3 Pruebas de integración

Algunas pruebas requieren una instancia PostgreSQL independiente y una base preparada con todas las migraciones.

Para estas pruebas se utiliza:

```env
TEST_DATABASE_URL=
```

Ejemplo en Bash:

```bash
TEST_DATABASE_URL="postgres://usuario:password@localhost:5432/soloservis_test" go test ./...
```

En PowerShell:

```powershell
$env:TEST_DATABASE_URL="postgres://usuario:password@localhost:5432/soloservis_test"
go test ./...
```

Las pruebas que necesitan PostgreSQL se omiten cuando `TEST_DATABASE_URL` no está definida.

Se debe utilizar una base de pruebas aislada, nunca la base de datos de producción.

## 19.4 Prueba de endpoints

Ejemplo:

```bash
curl http://localhost:8081/health
```

Consultar productos:

```bash
curl http://localhost:8081/products
```

Consultar tiendas:

```bash
curl http://localhost:8082/stores
```

Consultar servicios:

```bash
curl http://localhost:8083/services
```

---

# 20. Integración con Kubernetes

El backend puede desplegarse mediante dos topologías:

| Topología | Distribución |
|---|---|
| `dominios` | Tres Deployments de backend: comercial, usuarios e ingestion. |
| `consolidado` | Un Deployment de backend que ejecuta las 14 APIs. |

Los scripts de despliegue construyen y publican las imágenes, configuran los servicios y ejecutan las migraciones.

Las verificaciones de despliegue incluyen:

- Estado de los Pods.
- Conexión con PostgreSQL.
- Ejecución de migraciones.
- Disponibilidad de los endpoints.
- Comunicación entre dominios.
- Persistencia de datos.
- Pruebas de integración mediante el pod de test.

Documentación completa:

```text
k8s/README.md
```

---

# 21. Convenciones para desarrollar nuevas funcionalidades

Toda funcionalidad nueva debe respetar la separación de responsabilidades del backend.

Antes de comenzar una implementación, se debe determinar:

1. A qué API pertenece.
2. Qué problema resuelve.
3. Qué datos necesita.
4. Qué dependencias tiene.
5. Qué entradas recibirá.
6. Qué respuestas debe entregar.
7. Qué validaciones necesita.
8. Qué pruebas deben realizarse.
9. Qué documentación debe actualizarse.

## 21.1 Separación de responsabilidades

| Componente | Responsabilidad |
|---|---|
| Router | Registrar las rutas y middlewares. |
| Handler | Procesar solicitudes HTTP y respuestas. |
| Repository | Consultar y modificar datos. |
| DTO | Definir estructuras de entrada y salida. |
| Database | Administrar conexiones y acceso a PostgreSQL. |
| API Client | Realizar solicitudes hacia otros servicios. |
| Authz | Controlar permisos y sesiones. |
| Validator | Validar datos recibidos. |

No se deben incorporar consultas SQL directamente dentro de los handlers cuando corresponde utilizar la capa de persistencia.

Tampoco se deben omitir las validaciones para resolver rápidamente una integración.

## 21.2 Incorporación de una nueva funcionalidad

Flujo recomendado:

```text
Definición del requerimiento
          |
          v
Identificación de API
          |
          v
Análisis de dependencias
          |
          v
Diseño de contrato HTTP
          |
          v
Diseño de persistencia
          |
          v
Implementación
          |
          v
Validaciones
          |
          v
Pruebas unitarias
          |
          v
Pruebas de integración
          |
          v
Documentación
          |
          v
Pull Request
```

## 21.3 Reglas de implementación

- Mantener la compatibilidad de los contratos existentes.
- Validar todos los parámetros recibidos.
- Evitar confiar en datos sensibles enviados por el cliente.
- No duplicar lógica que ya existe en otro módulo.
- Utilizar consultas parametrizadas.
- Evitar exponer errores internos de la base de datos.
- Manejar correctamente los errores de dependencias externas.
- Registrar errores relevantes con su `request_id`.
- Crear pruebas para los casos exitosos y fallidos.
- Actualizar la documentación de la funcionalidad.

---

# 22. Documentación técnica

La documentación debe permitir que cualquier integrante del equipo comprenda el funcionamiento de las APIs, sin depender exclusivamente del conocimiento del desarrollador que las implementó.

Cada funcionalidad importante debe documentar:

| Elemento | Descripción |
|---|---|
| Propósito | Por qué existe la funcionalidad. |
| Objetivo | Qué problema resuelve. |
| Funcionamiento | Cómo se ejecuta internamente. |
| Entradas | Parámetros, headers y cuerpos recibidos. |
| Salidas | Respuestas y estructuras JSON. |
| Validaciones | Condiciones y restricciones. |
| Dependencias | APIs, tablas o servicios involucrados. |
| Errores | Casos de error y códigos HTTP. |
| Pruebas | Cómo comprobar su funcionamiento. |

La documentación debe mantenerse organizada y separada por funcionalidad.

Los documentos generales del backend deben permanecer en:

```text
backend/README.md
```

La documentación detallada de cada módulo debe centralizarse en:

```text
docs/
```

Los estándares de API deben mantenerse en:

```text
docs/estandares/
```

---

# 23. Definition of Done

Una modificación del backend podrá considerarse terminada cuando cumpla los siguientes criterios:

```text
[ ] Requerimiento comprendido.
[ ] API responsable identificada.
[ ] Dependencias identificadas.
[ ] Entradas y salidas definidas.
[ ] Validaciones implementadas.
[ ] Persistencia correctamente implementada.
[ ] Manejo de errores implementado.
[ ] Pruebas unitarias realizadas.
[ ] Pruebas de integración realizadas cuando corresponda.
[ ] Compilación exitosa.
[ ] Código revisado.
[ ] Documentación actualizada.
[ ] Pull Request revisada.
[ ] Integración aprobada.
```

---

# 24. Estado del backend

El backend cuenta con una arquitectura de 14 APIs agrupadas por dominios funcionales, integración con PostgreSQL y mecanismos de ejecución tanto mediante Docker Compose como Kubernetes.

El desarrollo continúa mediante mejoras en:

- Validación de entradas y salidas.
- Robustecimiento de las operaciones de persistencia.
- Comunicación entre APIs.
- Integración con Scraper e Ingestion.
- Manejo de errores y consistencia de respuestas.
- Pruebas de integración.
- Optimización de consultas y rendimiento.
- Seguridad y control de sesiones.
- Mantenibilidad y documentación técnica.

Las nuevas funcionalidades deben incorporarse respetando la arquitectura existente y manteniendo actualizada la documentación correspondiente.

---

**SOLOServis — Backend**