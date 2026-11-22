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
| `internal/categories/` | Categorías, atributos dinámicos y validador de atributos. |
| `internal/products/`, `internal/stores/`, `internal/pricing/`, `internal/search/`, `internal/comparison/` | Paquetes propios de Product, Store, Pricing, Search y Comparison API. |
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

Search API y Comparison API (productos) ya no llaman a Product API ni a Pricing API: consultan la base directamente (ver sección 25).

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
| `INTERNAL_API_TOKEN` | Token para operaciones internas autorizadas (mínimo 32 caracteres; si es más corto se ignora). |
| `AI_PROVIDER` | Proveedor del asistente de comparación (`auto` por defecto: Gemini con `AI_API_KEY`). En el clúster es `compatible` (DeepSeek). |
| `AI_MODEL` | Modelo del asistente (`gemini-3.5-flash` por defecto; en el clúster `deepseek-chat`). |
| `AI_API_KEY` | Clave del proveedor. **Solo backend.** Sin ella `auto` responde 503 `ai_unavailable`: la ruta no genera texto propio. En el clúster vive en el Secret `soloservis-ai`. |
| `AI_BASE_URL` | Opcional; URL base del proveedor compatible OpenAI (`https://api.deepseek.com/v1` en el clúster). Cambiarla basta para apuntar a otro endpoint. |
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

Una migración nueva debe incorporarse tanto al directorio de migraciones como al manifiesto. El runner rechaza cualquier `.sql` que no figure en `migration_order.txt`.

El mismo runner admite una base remota (por ejemplo Neon) mediante `DATABASE_URL`. En PowerShell, configura `NEON_DATABASE_URL` desde tu gestor de secretos y ejecuta desde la raíz del repositorio:

```powershell
$env:DATABASE_URL = $env:NEON_DATABASE_URL
if (-not $env:DATABASE_URL) { throw "NEON_DATABASE_URL no está configurada." }
docker compose -f docker/compose.yml run --rm --no-deps -e DATABASE_URL postgres bash /docker-entrypoint-initdb.d/001_migrations.sh
```

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

Estos comandos son un atajo para bases ya inicializadas; el método soportado sigue siendo el runner.

## 10.4 Base remota (Neon)

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
| Auth API | Registro, login, refresh, logout y cierre de todas las sesiones. |
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

## 13.3 Límite de solicitudes (TAL-201)

Todos los límites responden **429** `rate_limited` en formato problem+json y con la cabecera `Retry-After` en segundos (`httpx.WriteRateLimited`).

| Capa | Qué limita | Valor | Dónde |
| --- | --- | --- | --- |
| Gateway (nginx) | `POST /auth/login` y `/auth/register` por IP | 20 por minuto, ráfaga de 10 | `docker/frontend/default.conf.template` |
| Gateway (nginx) | Resto de las APIs por IP | 30 por segundo, ráfaga de 60 | `docker/frontend/proxy.conf` |
| Auth API | Logins fallidos por email (un email inexistente también cuenta) | 10 en 15 minutos; un login correcto reinicia la cuenta | `internal/features/auth.go` |
| Auth API | Cálculos de PBKDF2 simultáneos (login y registro) | 2 a la vez; espera máxima de 2 s | `internal/features/auth.go` |
| Search API | Búsquedas guardadas por usuario | 50; la siguiente responde 422 `name/max` | `internal/features/catalog.go` |
| Search API | Historial de búsqueda por usuario | 100; al insertar se borran las más antiguas | `internal/features/catalog.go` |

- Los contadores de la Auth API viven en memoria (`internal/ratelimit`): se reinician con el pod y suponen **una réplica** del dominio `usuarios`. Con más réplicas cada una contaría por su lado.
- El gateway toma la IP del cliente de `X-Forwarded-For` solo si el salto anterior es de red privada (`real_ip_recursive`). Si el proxy de la plataforma no reenvía la IP real, todos los clientes comparten un cupo; por eso los límites del gateway son holgados y la Auth API cuenta además por email.
- Llamar directo a `usuarios:8080` desde dentro del clúster se salta el gateway, pero no los límites de la Auth API.

## 13.4 Sesiones (TAL-200)

- El access token dura 15 minutos y la sesión 30 días **desde el login o el registro**. `POST /auth/refresh` rota los dos tokens, pero la sesión nueva hereda el vencimiento de la anterior: refrescar no la extiende. Si a la sesión le quedan menos de 15 minutos, el access token vence con ella (`expiresIn` lo refleja).
- Cada login, registro o refresh borra las sesiones vencidas del usuario y deja como máximo 10 vigentes; salen primero las que llevan más tiempo sin rotarse.
- `POST /auth/logout` cierra la sesión del bearer o del `refreshToken` del body. `POST /auth/logout-all` (requiere sesión) cierra todas las del usuario y responde 204.
- Una cuenta desactivada no puede validar ni refrescar sus tokens (401).
- El login responde el mismo 401 «Credenciales inválidas.» y calcula PBKDF2 igual si el email no existe o la cuenta está inactiva, para no revelar qué cuentas existen. El registro mantiene el 409 de email duplicado; lo limita el gateway (20 por minuto por IP).
- Las contraseñas usan `crypto/pbkdf2` con el mismo formato `pbkdf2-sha256$<iteraciones>$<sal>$<hash>`: los hashes guardados siguen sirviendo.
- Las reseñas de `/reviews/*` muestran `author` y `authorVerified`, como `GET /products/{id}/reviews`, y no el `userId` interno.

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

Cuando un ítem coincide con un producto ya catalogado (por SKU, marca + modelo o tienda + URL de la oferta), Ingestion solo actualiza la oferta: el catálogo lo cura el admin. La única excepción es la imagen (TAL-243): si el producto no tiene ninguna imagen activa y el ítem trae `product.imageUrl`, se guarda como imagen principal. Nunca reemplaza una imagen existente ni reactiva una URL que el admin desactivó.

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
| Product (ilustraciones) | `GET /categories/artworks/{key}`: SVG de la categoría, desde la tabla `category_artworks` (migración `018_category_artworks.sql`) |
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

El contrato detallado de Categorías, Productos, Store, Pricing, Search y Comparison se encuentra en la sección 25.

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

## 16.4 Modo anterior (un contenedor por API)

`docker/compose.yml` levanta la arquitectura consolidada (TAL-109), con una sola imagen (`docker/all-apis/Dockerfile`, que compila `cmd/all-apis`) para los tres dominios. `API_DOMAIN` decide qué APIs registra cada proceso.

| Servicio | `API_DOMAIN` | Puertos en la máquina local |
|---|---|---|
| `comercial` | `comercial` | 8081, 8082, 8083, 8085, 8086, 8087, 8089, 8090, 8091, 8092 y 8093 |
| `usuarios` | `usuarios` | 8084 y 8088 |
| `ingestion` | `ingestion` | 8094 |

- Dentro de un dominio las llamadas entre APIs van a `127.0.0.1:8080` (lo configura el binario). La única llamada entre dominios es de `ingestion` a notification: `NOTIFICATION_API_URL=http://comercial:8080`.
- El modo anterior, con un contenedor por API, se levanta con `docker compose -f docker/compose.microservicios.yml up -d --build` y usa los 14 `docker/<api>/Dockerfile`.
- No levantar los dos modos a la vez: comparten puertos y el contenedor de PostgreSQL.

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
TEST_DATABASE_URL="postgres://usuario:password@localhost:5432/soloservis_test" go test -p 1 ./...
```

En PowerShell:

```powershell
$env:TEST_DATABASE_URL="postgres://usuario:password@localhost:5432/soloservis_test"
go test -p 1 ./...
```

`-p 1` corre los paquetes en secuencia. Es necesario con base de datos: todos los paquetes comparten la misma base y en paralelo se pisan los datos, lo que produce fallas intermitentes que no son reales.

Las pruebas que necesitan PostgreSQL se omiten cuando `TEST_DATABASE_URL` no está definida.

Se debe utilizar una base de pruebas aislada, nunca la base de datos de producción.

Para levantar una base de pruebas desechable:

```bash
docker run -d --name soloservis-test-pg -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=soloservis -p 55432:5432 postgres:18
# aplicar las migraciones en el orden de backend/database/migration_order.txt, igual que k8s/base/migrations/migrate.sh
cd backend && TEST_DATABASE_URL=postgres://postgres:postgres@localhost:55432/soloservis go test -run Integration ./internal/categories/
```

### Integración por dominio (TAL-133, `internal/integration/`)

Levanta `comercial`, `usuarios` e `ingestion` como servidores HTTP sobre la misma base y verifica que las 14 APIs leen y escriben sus tablas, incluidas las llamadas entre APIs.

- `TestBaseCaida_*` no necesita base: comprueba que con PostgreSQL caído todas responden 503.
- `TestIntegracion_AccesoCruzado` (TAL-125) comprueba que un usuario no puede leer ni modificar las reseñas, la watchlist, las notificaciones ni las comparaciones de otro aunque mande su `userId`.

Detalle en `docs/auditorias/TAL-133-integracion-postgres.md` y `docs/auditorias/TAL-125-responsabilidades-y-datos.md`.

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

Consultar categorías:

```bash
curl http://localhost:8081/categories
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

En Kubernetes, el nginx del frontend (`docker/frontend/default.conf.template`) publica `/categories` y `/product-categories` solo para GET, junto con `/products`. Nginx bloquea desde internet todo lo que no sea GET (`limit_except GET`). Pricing API no se publica en nginx: solo la usan las otras APIs por la red interna.

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

# 25. Referencia detallada de endpoints (TAL-70 a TAL-76)

Esta sección conserva el contrato vigente de las APIs reescritas con el estándar TAL-78. Según la sección 22, cuando se reorganice la documentación cada subsección puede moverse a su propio archivo en `docs/`.

---

## 25.1 Endpoints `/categories` (TAL-71) y validación de atributos (TAL-72)

Categorías unificadas de productos y servicios con sus **definiciones de atributos dinámicos**. Son la base de la validación de atributos (TAL-72), los filtros (TAL-75, `filterable`) y la comparación (TAL-76, `comparable`).

- Servicio: **Product API** (`cmd/product-api`, puerto 8081). Código en `internal/categories`.
- Solo lectura: `POST`/`PUT`/`PATCH`/`DELETE` responden `405`. No hay endpoints de escritura (requerirían autenticación).
- Errores con el estándar TAL-78 (`application/problem+json`, ver `docs/estandares/TAL-78-estandar-api.md`).
- Incluye el validador de atributos (TAL-72) que usan los endpoints de escritura de productos y servicios (ver más abajo).
- Solo se exponen categorías con `active = TRUE`.

### Esquema (migraciones `012_categories.sql`, `013_categories_seed.sql`, `018_category_artworks.sql` y `019_reconcile_refrigeradores_category.sql`)

Se numeran 012 y 013 porque 009–011 son de TAL-67/68. Están registradas al final de `backend/database/migration_order.txt`: el runner rechaza cualquier `.sql` que no figure en ese manifiesto.

| Tabla | Columnas principales | Restricciones en BD |
| --- | --- | --- |
| `categories` | `slug`, `name`, `item_type` (`product`/`service`), `description`, `active`, `legacy_product_category_id`, `legacy_service_category_id`, timestamps | `slug` UNIQUE + CHECK `^[a-z0-9-]{1,64}$`; CHECK de `item_type`; enlace legado UNIQUE, FK `ON DELETE SET NULL` y del mismo tipo de ítem |
| `category_attributes` | `key`, `label`, `data_type`, `unit`, `required`, `comparable`, `filterable`, `min_value`, `max_value`, `max_length`, `allowed_values`, `display_order` | FK a `categories` `ON DELETE CASCADE`; UNIQUE `(category_id, key)`; CHECK `key ^[a-z][a-z0-9_]{0,63}$`; `data_type` ∈ `string`, `integer`, `decimal`, `boolean`, `enum`; `allowed_values` solo y obligatorio en `enum`; `min`/`max` solo numéricos (enteros en `integer`, `min <= max`); `max_length` solo `string` (1–255); `unit` solo numéricos |
| `category_artworks` | `artwork_key`, `svg_content`, `updated_at` | Clave primaria; contiene los SVG estáticos de las categorías |

Las tablas antiguas (`product_category`, `service_category` y sus `*_specification`) se mantienen. La migración 019 vincula `refrigeradores` con la subcategoría `Refrigeradores` bajo `Electrodomésticos`, mueve allí los productos y sus valores de especificación, y deja inactiva la raíz antigua `Refrigeracion`.

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

### Errores

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

Error de base de datos:

```json
{
  "type": "about:blank",
  "title": "Internal Server Error",
  "status": 500,
  "detail": "Ocurrió un error interno. Repórtalo indicando el request_id.",
  "instance": "/categories/notebooks",
  "code": "internal_error",
  "request_id": "ejemplo-500"
}
```

### Validación de atributos (TAL-72)

`categories.ValidateAttributes(defs, input)` (`internal/categories/validate.go`) valida los atributos de un producto o servicio contra las definiciones de su categoría **antes de guardarlo**. Es una función pura: no consulta la base (recibe las definiciones que ya devolvió `GetBySlug`), así que la reutilizan Product API (TAL-70) e Ingestion API (TAL-77).

El request lleva los atributos como un objeto `key → valor JSON nativo`, decodificado como `map[string]json.RawMessage`:

```json
{
  "attributes": {
    "capacidad_litros": 350,
    "eficiencia_energetica": "A+",
    "no_frost": true,
    "peso_kg": 65.5,
    "color": "Gris"
  }
}
```

Los valores se interpretan en la unidad de la definición (`L`, `kg`…); la unidad no se envía.

| `dataType` | Acepta | Rechaza (`errors[].code`) | Se guarda como |
| --- | --- | --- | --- |
| `integer` | número JSON entero (`350`, `350.0`, `3.5e2`) hasta 2^53−1 | `"350"`, `true`, `35.5` → `invalid_type`; fuera de rango → `min` / `max` | `"350"` |
| `decimal` | número JSON finito | string, booleano, objeto → `invalid_type`; `min` / `max` | `"65.5"` (sin ceros finales) |
| `boolean` | `true` / `false` | `"true"`, `1` → `invalid_type` | `"true"` / `"false"` |
| `enum` | string exacto de `allowedValues` (distingue mayúsculas; se quitan espacios en los extremos) | otro valor → `one_of`; no-string → `invalid_type` | el valor |
| `string` | string (se quitan espacios en los extremos) | no-string → `invalid_type`; más de `maxLength` caracteres (255 si no se define) → `max_length` | el texto |

Reglas generales:

- **Sin conversiones implícitas**: `"350"` no es un entero. Así un error del cliente no se guarda como dato válido.
- **Obligatorios**: un atributo `required` ausente, `null` o con texto vacío → `required`. Un opcional `null` o vacío se omite.
- **Keys desconocidas**: una key que no existe en la categoría → `unknown_field` (un error de tipeo no se pierde en silencio).
- **Rangos inclusivos**: `min` y `max` admiten el valor límite.
- **Errores** con `field: "attributes.<key>"` y en orden determinista: definiciones por `displayOrder` y luego keys desconocidas en orden alfabético.
- El resultado es una lista de `Value{AttributeID, Key, DataType, Text}`: `AttributeID` es la FK a `category_attributes.id` y `Text` cabe en las columnas `VARCHAR(255)` de valores.

Uso previsto en el handler (TAL-70). Los errores son reglas de negocio sobre un JSON ya decodificado, por eso responden **422**:

```go
values, fieldErrors := categories.ValidateAttributes(category.Attributes, req.Attributes)
if len(fieldErrors) > 0 {
	httpx.WriteValidation(w, r, fieldErrors)
	return
}
```

```json
{
  "type": "about:blank",
  "title": "Unprocessable Entity",
  "status": 422,
  "detail": "La solicitud no cumple las reglas de validación.",
  "instance": "/products",
  "code": "validation_failed",
  "request_id": "3f9c2a7e5b0d4c1a8e6f2b9d0a7c4e1f",
  "errors": [
    { "field": "attributes.velocidad_bajada_mbps", "code": "invalid_type", "message": "attributes.velocidad_bajada_mbps debe ser un número entero" },
    { "field": "attributes.velocidad_subida_mbps", "code": "required", "message": "attributes.velocidad_subida_mbps es obligatorio" },
    { "field": "attributes.tipo_contrato", "code": "one_of", "message": "attributes.tipo_contrato debe ser uno de: Sin permanencia, 12 meses, 24 meses" },
    { "field": "attributes.precio", "code": "unknown_field", "message": "attributes.precio no está definido para esta categoría" }
  ]
}
```

### Exposición por nginx

En Kubernetes, el nginx del frontend (`docker/frontend/default.conf.template`) publica `/categories` y `/product-categories` solo para GET, junto con `/products`. Ver `k8s/README.md`.

### Tests

- Unitarios, sin BD (`internal/categories/handler_test.go`, `internal/http/router_test.go`): `cd backend && go test ./...`.
- Integración: migraciones sobre base limpia, seeds, CHECK y repositorio (`internal/categories/repository_integration_test.go`). Se omiten si no existe `TEST_DATABASE_URL` (ver sección 19.3).

---

## 25.2 Productos con categoría y atributos dinámicos (TAL-70)

Desde TAL-70, Product API crea, consulta, edita y lista productos usando la categoría del modelo nuevo (`categories`, TAL-71) y sus atributos validados por TAL-72. Esta sección es el contrato vigente.

| Método | Ruta | Autorización | Éxito |
| --- | --- | --- | --- |
| GET | `/products?category=&limit=&offset=` | pública | 200, arreglo + `X-Total-Count` |
| GET | `/products/{publicID}` | pública | 200 |
| POST | `/products` | **admin** o token de servicio | 201 + `Location`, misma forma que el GET |
| PUT | `/products/{publicID}` | **admin** o token de servicio | 200, misma forma que el GET |
| DELETE | `/products/{publicID}` | **admin** o token de servicio | 204 (baja lógica) |
| GET | `/products/{publicID}/reviews` | pública | 200 |
| GET | `/product-categories` | pública | 200 (árbol de `product_category`; transitorio hasta TAL-51) |

### Modelo de datos (`014_product_attributes.sql`)

- `product.category_ref` → `categories(id)` (`ON DELETE RESTRICT`). Se cargó desde `categories.legacy_product_category_id`. `product.category_id` (`product_category`) **no cambia**: lo siguen usando Search, Comparison y el frontend.
- `product_attribute_value (product_id, category_id, attribute_id, value, value_numeric)`, PK `(product_id, attribute_id)`:
  - `value`: forma canónica del validador (`"350"`, `"65.5"`, `"true"`, `"A+"`).
  - `value_numeric`: solo para `integer`/`decimal`, para los filtros por rango de TAL-75.
  - **FK compuestas**: `(product_id, category_id) → product(id, category_ref)` y `(attribute_id, category_id) → category_attributes(id, category_id)`. La base de datos impide guardar un atributo de otra categoría, cambiar la categoría de un producto sin reemplazar sus valores y borrar un atributo que tenga valores (`RESTRICT`).
- Los productos existentes conservan sus especificaciones antiguas en `specs`; `attributes` se llena al crear o editar con este contrato.

### Autorización de escrituras (`internal/authz`)

`POST`, `PUT` y `DELETE` exigen `Authorization: Bearer <token>`:

- **Sesión de Auth API** de un usuario con `role = 'admin'` (tabla `user_session`, se compara el SHA-256 del token).
- **Token de servicio** `INTERNAL_API_TOKEN` (mínimo 32 caracteres; si es más corto se ignora). Ingestion API lo reenvía automáticamente (`features.remoteJSON`). En `docker/compose.yml` hay un valor de desarrollo que se reemplaza con la variable de entorno `INTERNAL_API_TOKEN`.

| Caso | Status | `code` |
| --- | --- | --- |
| Sin header, esquema distinto de Bearer o token desconocido/vencido | 401 (+ `WWW-Authenticate: Bearer`) | `unauthorized` |
| Usuario autenticado sin rol admin | 403 | `forbidden` |

Nginx sigue bloqueando desde internet todo lo que no sea GET (`limit_except GET`).

### Body de `POST /products` y `PUT /products/{publicID}`

```json
{
  "categorySlug": "refrigeradores",
  "brandId": 2,
  "name": "Refrigerador LG 350L No Frost",
  "model": "GT35",
  "sku": "LG-GT35",
  "description": "Refrigerador no frost de 350 litros",
  "imageUrl": "https://example.test/lg.png",
  "attributes": {
    "capacidad_litros": 350,
    "eficiencia_energetica": "A++",
    "no_frost": true,
    "peso_kg": 65.5,
    "color": "Gris"
  }
}
```

| Campo | Regla |
| --- | --- |
| `categorySlug` / `categoryId` | **Exactamente uno**. `categorySlug` es el modelo nuevo (recomendado). `categoryId` (id de `product_category`) se mantiene para Ingestion API hasta TAL-77; si esa categoría tiene equivalente nueva, se aplican sus atributos. La categoría debe ser de tipo `product`, estar activa y tener equivalente en `product_category`. |
| `name` | obligatorio, máximo 255 caracteres |
| `model` / `sku` / `description` | opcionales; máximo 150 / 100 / 5000 caracteres. `sku` es único. |
| `brandId` | opcional, mayor que 0 y existente |
| `imageUrl` | **obligatorio en POST**, URL http(s). En PUT no se acepta (las imágenes no se editan con PUT). |
| `attributes` | se valida con `categories.ValidateAttributes` (sección 25.1): tipos sin conversión implícita, obligatorios, rangos, largos, enum y keys desconocidas. |

**PUT reemplaza el producto completo**: un campo opcional omitido queda vacío y los atributos se reemplazan por los enviados.

Orden de validación (TAL-78 §3): id de ruta (400) → decodificación (400/413/415) → reglas de campos (422) → categoría (422) → atributos (422) → base de datos (404/409/422/500). Todo se guarda en **una transacción**: producto, imagen y atributos.

### Respuestas

`POST /products` (201, con `Location: /products/{publicID}`) y `PUT` (200) devuelven el mismo cuerpo que `GET /products/{publicID}`. Al formato anterior se agregan `categorySlug` (`null` si la categoría antigua no tiene equivalente) y `attributes` con valores tipados. Desde TAL-237 también viajan `updatedAt` (producto) y `lastUpdated` (cada oferta), ambos en RFC 3339 UTC: sin ellos la ficha no podía decir cuándo se verificó el precio. Respuesta real del `PUT` con el body de arriba:

```http
PUT /products/f670f409-4430-4758-b587-6b05153eb5e4  →  HTTP/1.1 200 OK
```

```json
{
  "id": "f670f409-4430-4758-b587-6b05153eb5e4",
  "name": "Refrigerador LG 350L No Frost",
  "brand": "LG",
  "model": "GT35",
  "categoryId": 2,
  "category": "Refrigeradores",
  "description": "Refrigerador no frost de 350 litros",
  "rating": 0,
  "reviewCount": 0,
  "specs": {},
  "images": [{ "url": "https://example.test/lg.png", "altText": "Refrigerador LG 350L", "sortOrder": 0 }],
  "offers": [],
  "priceHistory": [],
  "offerPriceHistory": [],
  "categorySlug": "refrigeradores",
  "attributes": {
    "capacidad_litros": 350,
    "color": "Gris",
    "eficiencia_energetica": "A++",
    "no_frost": true,
    "peso_kg": 65.5
  },
  "updatedAt": "2026-11-22T15:43:49Z"
}
```

Cada punto de `priceHistory` y `offerPriceHistory` conserva `date` y `price`, y desde TAL-238 indica de qué oferta y tienda es, la moneda de la oferta y la hora del registro (UTC, RFC 3339):

```json
{ "date": "2026-11-16", "price": "189990.00", "offerId": 69, "storeId": 4, "currency": "CLP", "recordedAt": "2026-11-16T06:00:12.345Z" }
```

`GET /products` mantiene el **cuerpo como arreglo** (lo consumen Search API y el frontend) con los mismos campos por producto, más `categorySlug` y `attributes`:

| Parámetro | Regla |
| --- | --- |
| `category` | id numérico de `product_category` (incluye subcategorías) **o** slug de `categories` |
| `limit` | 1–100, por defecto 100 (el catálogo actual tiene 38 productos) |
| `offset` | ≥ 0, por defecto 0 |

El total sin paginar va en el header `X-Total-Count`. El listado usa 6 consultas fijas (conteo, página, imágenes, ofertas, specs y atributos), sin N+1. Un producto dado de baja no aparece en el listado y su detalle responde 404.

### Errores

```json
{
  "type": "about:blank",
  "title": "Unprocessable Entity",
  "status": 422,
  "detail": "La solicitud no cumple las reglas de validación.",
  "instance": "/products",
  "code": "validation_failed",
  "request_id": "ejemplo-tal70",
  "errors": [
    { "field": "attributes.capacidad_litros", "code": "invalid_type", "message": "attributes.capacidad_litros debe ser un número entero" },
    { "field": "attributes.eficiencia_energetica", "code": "one_of", "message": "attributes.eficiencia_energetica debe ser uno de: A+++, A++, A+, A, B, C, D" }
  ]
}
```

| Caso | Status | `code` / `errors[]` |
| --- | --- | --- |
| `publicID` no es UUID | 400 | `invalid_parameter` / `publicID` `format` |
| `category`, `limit` u `offset` inválidos | 400 | `invalid_parameter` / `format`, `min`, `max`, `invalid_type` |
| JSON malformado, campo desconocido, tipo incorrecto | 400 | `malformed_json`, `unknown_field`, `invalid_type` |
| `Content-Type` distinto de JSON | 415 | `unsupported_media_type` |
| Sin token / sin rol admin | 401 / 403 | `unauthorized` / `forbidden` |
| Reglas de campos | 422 | `name` `required`/`max_length`, `categorySlug` `required`/`format`, `categoryId` `one_of`/`min`, `imageUrl` `required`/`format`, `brandId` `min` |
| Categoría inexistente, de servicios o sin equivalente | 422 | `categorySlug`/`categoryId` `not_found` o `one_of` |
| Atributos inválidos | 422 | `attributes.<key>` con `required`, `invalid_type`, `min`, `max`, `max_length`, `one_of`, `unknown_field` |
| Marca inexistente | 422 | `brandId` `not_found` |
| SKU duplicado | 409 | `conflict` |
| Producto inexistente o dado de baja | 404 | `not_found` |
| Error de base de datos | 500 | `internal_error` (el detalle solo va al log con el `request_id`) |

```http
GET /products?category=Refri_Geradores&limit=500
```

```json
{
  "type": "about:blank",
  "title": "Bad Request",
  "status": 400,
  "detail": "Los parámetros de la consulta no son válidos.",
  "instance": "/products",
  "code": "invalid_parameter",
  "request_id": "ejemplo-tal70",
  "errors": [
    { "field": "category", "code": "format", "message": "category debe ser un id numérico o un slug de categoría" },
    { "field": "limit", "code": "max", "message": "limit no puede superar 100" }
  ]
}
```

### Tests

- Unitarios sin BD: `internal/products/handler_test.go`, `internal/authz/authz_test.go`, `internal/http/router_test.go`.
- Integración con `TEST_DATABASE_URL`: `internal/products/catalog_integration_test.go` (ciclo de vida, filtros, paginación y FK compuestas) e `internal/http/router_integration_test.go` (router real con sesiones admin/cliente y token de servicio).

---

## 25.3 Store API: tiendas y ofertas por producto (TAL-73)

Store API (`cmd/store-api`, puerto 8082, código en `internal/stores`) es **solo lectura**. Nginx ya enruta `/stores` (solo GET), así que los endpoints nuevos quedan expuestos sin cambios de infraestructura.

| Método | Ruta | Respuesta |
| --- | --- | --- |
| GET | `/stores?limit=&offset=` | arreglo de tiendas + `X-Total-Count` |
| GET | `/stores/{id}` | tienda con `products` (ofertas disponibles de productos activos) y `locations` |
| GET | `/stores/offers?product={publicID}&available=&limit=&offset=` | **ofertas de un producto en todas las tiendas**, cada una con su tienda resumida |
| GET | `/stores/{id}/offers?available=&limit=&offset=` | ofertas de una tienda (disponibles o no) |

Límite con Pricing API (TAL-74): Store API **lee** ofertas agrupadas por tienda; Pricing **registra** precios y su historial.

### Parámetros

| Parámetro | Regla |
| --- | --- |
| `product` | obligatorio en `/stores/offers`; UUID de un producto activo |
| `available` | opcional, `true` o `false` (sin él, todas) |
| `limit` | 1–100, por defecto 100 |
| `offset` | ≥ 0, por defecto 0 |
| `{id}` | entero positivo |

### Reglas

- Solo se consideran **tiendas activas** y **productos activos**.
- `GET /stores/offers` ordena **disponibles primero, luego por precio ascendente** y nombre de tienda: lo que necesita el comparador.
- `GET /stores/{id}/offers` ordena disponibles primero y luego por nombre de producto.
- Una tienda puede tener **varias ofertas del mismo producto** (la unicidad es producto + tienda + URL normalizada, TAL-63/64).
- `productCount` cuenta productos **activos** con oferta disponible y vale lo mismo en el listado y en el detalle (antes el detalle contaba productos inactivos).
- `reviewCount` sale de `store_review` (antes siempre valía 0). `rating` sigue siendo el valor guardado en `store.rating`.
- Montos como string, igual que en Product API. `lastUpdated` en RFC 3339 (UTC).

### Ejemplos (respuestas reales)

```http
GET /stores/offers?product=14159cb1-e5d2-4fa7-88c7-a70f29d233f5&limit=2
X-Total-Count: 3
```

```json
[
  {
    "offerId": 64,
    "productId": "14159cb1-e5d2-4fa7-88c7-a70f29d233f5",
    "productName": "Refrigerador Midea Side by Side No Frost 555 L",
    "price": "332991.00",
    "listPrice": "",
    "currency": "CLP",
    "shippingCost": "0",
    "shippingFree": true,
    "available": true,
    "stock": null,
    "condition": "new",
    "productUrl": "https://simple.ripley.cl/refrigerador-side-by-side-midea-mdrs710fge50in-555l-no-frost-gris-2000406824253p?color_80=Gris",
    "lastUpdated": "2026-10-25T21:44:42Z",
    "store": {
      "id": "4",
      "name": "Ripley",
      "logo": "https://placehold.co/128x128?text=Ripley",
      "rating": 4.1,
      "reviewCount": 0,
      "website": "https://simple.ripley.cl"
    }
  },
  {
    "offerId": 60,
    "productId": "14159cb1-e5d2-4fa7-88c7-a70f29d233f5",
    "productName": "Refrigerador Midea Side by Side No Frost 555 L",
    "price": "379990.00",
    "listPrice": "",
    "currency": "CLP",
    "shippingCost": "8990.00",
    "shippingFree": false,
    "available": true,
    "stock": null,
    "condition": "new",
    "productUrl": "https://www.falabella.com/falabella-cl/product/80526705/refrigerador-side-by-side-no-frost-555-litros-mdrs710fge50in-midea/80526705",
    "lastUpdated": "2026-10-25T21:44:42Z",
    "store": {
      "id": "1",
      "name": "Falabella",
      "logo": "https://www.falabella.com/static/site/common/Falabella-Chile.jpg",
      "rating": 4.2,
      "reviewCount": 0,
      "website": "https://falabella.com"
    }
  }
]
```

```http
GET /stores/1/offers?available=true&limit=1
X-Total-Count: 22
```

```json
[
  {
    "offerId": 13,
    "productId": "163ef4e4-529a-4a66-a263-076c3eddbb72",
    "productName": "Apple iPhone 15 128 GB",
    "price": "699990.00",
    "listPrice": "799990.00",
    "currency": "CLP",
    "shippingCost": "0",
    "shippingFree": true,
    "available": true,
    "stock": 10,
    "condition": "new",
    "productUrl": "https://demo.soloservis.cl/products/DEMO-APPLE-IPHONE15-128?store=falabella",
    "lastUpdated": "2026-10-25T21:44:41Z"
  }
]
```

En `/stores/{id}/offers` no se repite `store`: es la tienda de la ruta.

`GET /stores` conserva su forma (la consumen el frontend e Ingestion API):

```json
[
  {
    "id": "1",
    "name": "Falabella",
    "logo": "https://www.falabella.com/static/site/common/Falabella-Chile.jpg",
    "rating": 4.2,
    "reviewCount": 0,
    "productCount": 22,
    "reputation": "Buena",
    "dispatchTime": "Envío en 3-5 días hábiles",
    "conditions": "Garantía de 12 meses",
    "website": "https://falabella.com"
  }
]
```

### Errores

| Caso | Status | `code` / `errors[]` |
| --- | --- | --- |
| `{id}` no es un entero positivo | 400 | `invalid_parameter` / `id` `format` |
| Falta `product` o no es UUID | 400 | `invalid_parameter` / `product` `required` o `format` |
| `available`, `limit` u `offset` inválidos | 400 | `invalid_parameter` / `one_of`, `min`, `max`, `invalid_type` |
| Producto inexistente o inactivo | 404 | `not_found` |
| Tienda inexistente o inactiva | 404 | `not_found` |
| Error de base de datos | 500 | `internal_error` (antes se respondía 404 ante cualquier error) |
| Método distinto de GET | 405 | respuesta de chi |

```http
GET /stores/offers?product=abc&available=si
```

```json
{
  "type": "about:blank",
  "title": "Bad Request",
  "status": 400,
  "detail": "Los parámetros de la consulta no son válidos.",
  "instance": "/stores/offers",
  "code": "invalid_parameter",
  "request_id": "ejemplo-tal73",
  "errors": [
    { "field": "product", "code": "format", "message": "product debe ser un UUID" },
    { "field": "available", "code": "one_of", "message": "available debe ser true o false" }
  ]
}
```

### Tests

- Unitarios sin BD: `internal/stores/handler_test.go` e `internal/http/router_test.go` (rutas registradas, solo lectura).
- Integración con `TEST_DATABASE_URL`: `internal/stores/repository_integration_test.go` (orden y filtros de ofertas, tiendas y productos inactivos, contadores coherentes entre listado y detalle, `reviewCount` real).

---

## 25.4 Pricing API: precio actual e historial (TAL-74)

Pricing API (`cmd/pricing-api`, puerto 8085) tiene ahora su propio paquete, `internal/pricing`, con el estándar TAL-78 (antes vivía en `internal/features` con `apiutil`). Usa `apphttp.NewPricingRouter`: RequestID → AccessLog → Recover → CORS.

| Método | Ruta | Autorización | Respuesta |
| --- | --- | --- | --- |
| POST | `/offers` | admin o token de servicio | **registra el precio actual**: 201 + `Location` si la oferta es nueva, 200 si ya existía |
| PUT | `/offers/{id}` | admin o token de servicio | 200, reemplaza los datos de la oferta |
| GET | `/offers/{id}` | pública | oferta |
| GET | `/offers/product/{productId}` | pública | ofertas del producto (disponibles primero, luego por precio) |
| GET | `/offers/product/{productId}/best` | pública | **mejor precio actual**: oferta disponible con menor `totalPrice` |
| GET | `/offers/{id}/price-history?from=&to=&limit=&offset=` | pública | historial de la oferta + `X-Total-Count` |
| GET | `/offers/product/{productId}/price-history?from=&to=&limit=&offset=` | pública | historial del producto en todas sus tiendas + `X-Total-Count` |

`{productId}` acepta el UUID público o el id numérico (como antes). Límite con Store API (TAL-73): Store **lee** ofertas por tienda; Pricing **registra** precios y su historial.

### Cómo se registra el precio actual

- `POST /offers` hace **upsert** sobre el índice único de TAL-63/64 (producto + tienda + URL normalizada): el scraper/Ingestion puede reenviar la misma oferta y solo se actualiza.
- El **historial lo escribe el trigger** `record_product_offer_price_history` (migración 010) en el alta y cuando cambia el precio. Reenviar el mismo precio no agrega historial.
- Si el precio baja, se avisa a los usuarios cuyo precio objetivo quedó cruzado (`product_user_interest`) mediante Notification API (`NOTIFICATION_API_URL`). Un fallo del aviso queda en el log y no afecta la respuesta.
- La moneda la normaliza el trigger de la migración 008 (hoy todo se guarda como CLP).

### Body de `POST /offers` / `PUT /offers/{id}`

```json
{
  "productId": "cdb7b0a4-aaff-412d-9147-8b2a2465d6af",
  "storeId": 2,
  "price": 329990,
  "listPrice": 399990,
  "shippingCost": 0,
  "shippingFree": true,
  "available": true,
  "stock": 10,
  "condition": "new",
  "currency": "CLP",
  "productUrl": "https://example.test/midea-scraper"
}
```

`productId` y `storeId` solo van en POST (en PUT son `unknown_field`).

| Campo | Regla | Si se omite |
| --- | --- | --- |
| `price` | **obligatorio**, ≥ 0, hasta 10 enteros y 2 decimales | 422 `required` (antes se guardaba 0) |
| `listPrice` | ≥ 0, mismo formato | `null` |
| `shippingCost` | ≥ 0, mismo formato | `0` |
| `shippingFree` | booleano | `false` |
| `available` | booleano | `true` (antes un PUT sin este campo apagaba la oferta) |
| `stock` | ≥ 0 | `null` |
| `condition` | `new`, `used` o `refurbished` | `new` |
| `currency` | 3 letras mayúsculas | `CLP` |
| `productUrl` | URL http(s), máximo 2048 | `null` |

Los montos aceptan número JSON o string numérico y **no pasan por `float64`**: viajan como texto decimal a `NUMERIC(12,2)`. En las respuestas van como string con 2 decimales.

### Respuestas (reales)

`POST /offers` con un precio nuevo para una oferta existente → **200**:

```json
{
  "id": 125,
  "productId": "cdb7b0a4-aaff-412d-9147-8b2a2465d6af",
  "storeId": 2,
  "storeName": "Paris",
  "price": "329990.00",
  "listPrice": null,
  "currency": "CLP",
  "shippingCost": "0.00",
  "shippingFree": true,
  "totalPrice": "329990.00",
  "available": true,
  "stock": null,
  "condition": "new",
  "productUrl": "https://example.test/midea-scraper",
  "lastUpdated": "2026-10-25T22:14:13Z"
}
```

`totalPrice` = `price` + `shippingCost` (0 si `shippingFree`). `GET /offers/product/{productId}/best` devuelve la misma forma; sin ofertas disponibles responde 404.

`GET /offers/125/price-history` (`X-Total-Count: 2`), más reciente primero:

```json
[
  { "id": 173, "price": "329990.00", "isPromotional": false, "recordedAt": "2026-10-25T22:14:13.846796Z", "currency": "CLP" },
  { "id": 172, "price": "349990.00", "isPromotional": false, "recordedAt": "2026-10-25T22:14:04.554457Z", "currency": "CLP" }
]
```

`currency` es la moneda de la oferta (TAL-238): el historial no guarda una propia por registro.

`GET /offers/product/{productId}/price-history?limit=3` (`X-Total-Count: 5`) agrega la oferta y la tienda de cada punto:

```json
[
  { "id": 173, "price": "329990.00", "isPromotional": false, "recordedAt": "2026-10-25T22:14:13.846796Z", "currency": "CLP", "offerId": 125, "storeId": 2, "storeName": "Paris" },
  { "id": 172, "price": "349990.00", "isPromotional": false, "recordedAt": "2026-10-25T22:14:04.554457Z", "currency": "CLP", "offerId": 125, "storeId": 2, "storeName": "Paris" },
  { "id": 108, "price": "379990.00", "isPromotional": false, "recordedAt": "2026-10-25T22:11:55.34416Z", "currency": "CLP", "offerId": 66, "storeId": 7, "storeName": "Lider" }
]
```

Parámetros del historial: `from`/`to` en `YYYY-MM-DD` (inclusivas, `to` ≥ `from`), `limit` 1–500 (por defecto 100), `offset` ≥ 0.

### Errores

| Caso | Status | `code` / `errors[]` |
| --- | --- | --- |
| `{id}` o `{productId}` con formato inválido | 400 | `invalid_parameter` / `id` o `productId` `format` |
| `from`, `to`, `limit`, `offset` inválidos | 400 | `invalid_parameter` / `format`, `min`, `max`, `invalid_type` |
| JSON malformado, campo desconocido, tipo incorrecto | 400 | `malformed_json`, `unknown_field`, `invalid_type` |
| Sin token / sin rol admin | 401 / 403 | `unauthorized` / `forbidden` |
| Reglas de campos | 422 | `price`/`listPrice`/`shippingCost` `required`, `min`, `format`; `currency` `format`; `condition` `one_of`; `stock` `min`; `productUrl` `format`/`max_length`; `productId` `required`/`format`; `storeId` `min` |
| Producto o tienda inexistente (en el body) | 422 | `productId` / `storeId` `not_found` |
| Otra oferta con la misma URL (PUT) | 409 | `conflict` |
| Oferta inexistente (también su historial; antes respondía `[]`) | 404 | `not_found` |
| Producto inexistente o inactivo (ruta) | 404 | `not_found` |
| Error de base de datos | 500 | `internal_error` |

```json
{
  "type": "about:blank",
  "title": "Unprocessable Entity",
  "status": 422,
  "detail": "La solicitud no cumple las reglas de validación.",
  "instance": "/offers",
  "code": "validation_failed",
  "request_id": "ejemplo-tal74",
  "errors": [
    { "field": "price", "code": "min", "message": "price debe ser mayor o igual a 0" },
    { "field": "listPrice", "code": "format", "message": "listPrice debe ser un monto con hasta 10 enteros y 2 decimales" },
    { "field": "condition", "code": "one_of", "message": "condition debe ser new, used o refurbished" }
  ]
}
```

### Consumidores

Los nombres de los campos se mantienen. Cart lee `productId` de `GET /offers/{id}`; Comparison y Watchlist reenvían `GET /offers/product/{productId}` sin interpretarlo; Ingestion usa `POST /offers` y reenvía `INTERNAL_API_TOKEN` (configurado para `pricing-api` e `ingestion-api` en `docker/compose.yml`).

### Tests

- Unitarios sin BD: `internal/pricing/handler_test.go`, `internal/http/router_test.go`.
- Integración con `TEST_DATABASE_URL`: `internal/pricing/integration_test.go` (upsert, historial por trigger, PUT sin apagar ofertas, mejor precio, filtros, avisos de bajada). Reemplaza a los tests de caracterización de Pricing de `internal/features`, actualizados al comportamiento corregido.

### Pendiente (fuera de TAL-74)

- Pricing se despliega en k8s (como `pricing-api` en local o dentro de `backend` en UCT), pero no se publica en nginx: solo la usan las otras APIs por la red interna.
- `is_promotional` siempre es `false`: el trigger de la 010 no lo calcula.

---

## 25.5 Search API: texto, categoría y atributos filtrables (TAL-75)

Search API (`cmd/search-api`, puerto 8086) tiene ahora la búsqueda pública en su propio paquete, `internal/search`, con el estándar TAL-78 (`apphttp.NewSearchRouter`). El historial y las búsquedas guardadas (`/search/history`, `/search/saved`, requieren sesión) siguen en `internal/features` y se montan en el mismo router: es lo único que usa hoy el frontend. También cumplen el estándar TAL-78.

Ya no descarga `GET /products` de Product API: consulta la base directamente.

| Método | Ruta | Respuesta |
| --- | --- | --- |
| GET | `/search/products` | arreglo de productos + `X-Total-Count` |
| GET | `/search/facets` | total, rango de precios, marcas y atributos filtrables (para los filtros del frontend, TAL-57) |
| GET | `/search/categories?q=&itemType=` | categorías del modelo nuevo (`categories`) |
| GET | `/search/brands?q=` | marcas activas |
| GET | `/search/presets` | catálogo de consultas curadas (TAL-222) |
| GET | `/search/presets/{slug}` | ejecuta el preset: mismo formato que `/search/products` + `X-Total-Count` |
| GET | `/search/alternatives?q=` | respuesta determinista: preset si hay coincidencia, búsqueda libre si no (TAL-226) |

### Consultas predefinidas (TAL-222)

Siete búsquedas curadas en `internal/search/presets.go`, en orden editorial:

| Slug | Filtros |
| --- | --- |
| `refrigeradores` | categoría `refrigeradores`, `price_asc` |
| `refrigeradores-economicos` | categoría + `maxPrice=400000` |
| `notebooks` | categoría `notebooks`, `price_asc` |
| `notebooks-economicos` | categoría + `maxPrice=550000` |
| `televisores` | categoría `televisores`, `price_asc` |
| `precios-bajos` | `maxPrice=100000`, 12 resultados |
| `novedades` | `newest`, 12 resultados |

Los presets usan **slug de categoría**, texto, precio y orden: nunca `storeId`
ni `brandId`, porque esos IDs cambian según el entorno. `GET /search/presets/{slug}`
acepta `limit` y `offset`; el resto de los filtros los define el preset y no se
pueden sobrescribir. Un slug desconocido responde 404 con `code: preset_not_found`.

El catálogo se valida en un test (`presets_test.go`): slug único y con formato,
título y descripción, categoría sembrada, `sort` permitido, precios válidos y
`limit` en rango. Un preset mal escrito rompe CI, no producción.

### Respuestas alternativas sin IA (TAL-226)

`GET /search/alternatives?q=` empareja el texto con el catálogo y responde sin
generar prosa:

| Campo | Valor |
| --- | --- |
| `source` | `preset` si se usó una consulta curada; `text` si fue búsqueda libre |
| `matched` | El preset usado, o `null` |
| `presets` | Hasta tres sugerencias, sin el ya usado |
| `products` | Resultados reales; total en `X-Total-Count` |

La normalización quita tildes, stopwords y plurales (`notebooks` → `notebook`).
Se exige que al menos la mitad de los tokens de la consulta estén en el
vocabulario del preset; por debajo, se prefiere la búsqueda libre antes que
adivinar. No hay LLM ni texto generado: `source` explica lo que pasó.



### Parámetros de `/search/products` (y de `/search/facets`, sin `sort`/`limit`/`offset`)

| Parámetro | Regla |
| --- | --- |
| `q` | hasta 5 palabras y 200 caracteres; **cada palabra** debe aparecer en nombre, modelo, SKU o marca. Sin distinguir tildes ni mayúsculas (`translate`, sin extensiones). `%` y `_` son literales. |
| `category` | slug de `categories` **o** id de `product_category` (incluye subcategorías) |
| `brand` | ids de marca separados por coma (hasta 20) |
| `minPrice` / `maxPrice` | sobre el **mejor precio total** (precio + envío) de las ofertas disponibles en tiendas activas; hasta 2 decimales; `minPrice` ≤ `maxPrice` |
| `available` | `true`: solo productos con al menos una oferta disponible |
| `attr.<key>=valor` | igualdad: `integer`/`decimal` numérica, `boolean` `true`/`false`, `enum` dentro de `allowedValues`, `string` sin distinguir mayúsculas |
| `attr.<key>.min` / `.max` | rango, solo atributos numéricos (usa `value_numeric` de TAL-70) |
| `sort` | `name` (por defecto, con la collation de la BD), `price_asc`, `price_desc` (sin precio al final), `newest` |
| `limit` / `offset` | 1–100 (por defecto 20) / ≥ 0 |

Los filtros `attr.*` exigen `category` con slug (los atributos son de la categoría) y solo aceptan atributos **`filterable`** (TAL-71). Todos los filtros se combinan con Y.

La consulta es una sola sentencia estática y parametrizada: palabras, marcas y filtros de atributos viajan como arreglos (`unnest`), sin concatenar entrada del usuario. Una búsqueda usa 3 consultas fijas (conteo, página, atributos); las facetas, 4.

### Ejemplos (respuestas reales)

```http
GET /search/products?q=refrigerador&category=refrigeradores&attr.capacidad_litros.min=500&attr.no_frost=true&sort=price_asc&limit=2
X-Total-Count: 2
```

```json
[
  {
    "id": "e666b310-fc3c-4bbb-b93b-fcc7696c3233",
    "name": "Refrigerador Midea Side by Side No Frost 555 L",
    "brand": "Midea",
    "model": "MDRS710FGE50IN",
    "category": "Refrigeradores",
    "categorySlug": "refrigeradores",
    "imageUrl": "https://rimage.ripley.cl/home.ripley/Attachment/WOP/1/2000406824253/full_image-2000406824253",
    "bestPrice": "332991.00",
    "offerCount": 3,
    "attributes": { "capacidad_litros": 555, "eficiencia_energetica": "A+", "no_frost": true }
  },
  {
    "id": "098d4ed6-ac0e-4bcd-bd8e-b885cc348c9b",
    "name": "Refrigerador LG French Door 530 L",
    "brand": "LG",
    "model": "GM53",
    "category": "Electrodomésticos",
    "subcategory": "Refrigeradores",
    "categorySlug": "refrigeradores",
    "imageUrl": "https://placehold.co/640x640?text=DEMO-LG-FRIDGE-530",
    "bestPrice": "899990.00",
    "offerCount": 2,
    "attributes": { "capacidad_litros": 530, "eficiencia_energetica": "A++", "no_frost": true }
  }
]
```

`bestPrice` es `null` si el producto no tiene ofertas disponibles; `offerCount` cuenta ofertas disponibles.

```http
GET /search/facets?category=refrigeradores
```

```json
{
  "total": 5,
  "price": { "min": "332991.00", "max": "1458980.00" },
  "brands": [
    { "id": 2, "name": "LG", "count": 2 },
    { "id": 10, "name": "Midea", "count": 2 },
    { "id": 1, "name": "Samsung", "count": 1 }
  ],
  "attributes": [
    { "key": "capacidad_litros", "label": "Capacidad", "dataType": "integer", "unit": "L", "min": 400, "max": 555 },
    { "key": "eficiencia_energetica", "label": "Eficiencia energética", "dataType": "enum", "unit": null,
      "values": [{ "value": "A+", "count": 2 }, { "value": "A++", "count": 1 }] },
    { "key": "no_frost", "label": "No Frost", "dataType": "boolean", "unit": null,
      "values": [{ "value": true, "count": 3 }] }
  ]
}
```

Las facetas aplican los mismos filtros que la búsqueda; solo incluyen atributos `filterable`. Los productos existentes no tienen atributos hasta que se editan (TAL-70 no migró las especificaciones antiguas): en el ejemplo, 3 de los 5 refrigeradores.

```http
GET /search/categories?q=OPTICA
```

```json
[{ "slug": "internet-fibra-optica", "name": "Internet Fibra Óptica", "itemType": "service" }]
```

### Errores

| Caso | Status | `code` / `errors[]` |
| --- | --- | --- |
| `q`, `category`, `brand`, precios, `available`, `sort`, `limit`, `offset` inválidos | 400 | `invalid_parameter` / `max_length`, `max`, `format`, `min`, `one_of`, `invalid_type` |
| `attr.*` sin `category` con slug | 400 | `category` `required` |
| Categoría inexistente | 400 | `category` `not_found` |
| Atributo desconocido / no filtrable | 400 | `attr.<key>` `unknown_field` / `not_filterable` |
| Valor que no cumple el tipo o el enum | 400 | `attr.<key>` `invalid_type` / `one_of` |
| Error de base de datos | 500 | `internal_error` |

```json
{
  "type": "about:blank",
  "title": "Bad Request",
  "status": 400,
  "detail": "Los parámetros de la consulta no son válidos.",
  "instance": "/search/products",
  "code": "invalid_parameter",
  "request_id": "ejemplo-tal75",
  "errors": [
    { "field": "attr.eficiencia_energetica", "code": "one_of", "message": "attr.eficiencia_energetica debe ser uno de: A+++, A++, A+, A, B, C, D" },
    { "field": "attr.no_existe", "code": "unknown_field", "message": "attr.no_existe no es un atributo de la categoría" },
    { "field": "attr.peso_kg", "code": "not_filterable", "message": "attr.peso_kg no es filtrable" }
  ]
}
```

### Índices

No se agregó un índice trigram: con el catálogo actual (38 productos) PostgreSQL recorre la tabla de todos modos, el texto combina columnas de `product` y `brand` (un índice sobre `product` no lo cubriría) y `pg_trgm`/`unaccent` exigen extensiones con privilegios en k8s. Revisar cuando el catálogo crezca (miles de productos): columna generada normalizada en `product` + índice GIN trigram.

### Tests

- Unitarios sin BD: `internal/search/handler_test.go`, `internal/http/router_test.go`.
- Integración con `TEST_DATABASE_URL`: `internal/search/integration_test.go` (texto sin tildes y con comodines literales, marca, precio total, disponibilidad, filtros por atributo, orden, paginación, facetas, categorías y marcas).

---

## 25.6 Comparison API: alternativas de una misma categoría (TAL-76)

Comparison API (`cmd/comparison-api`, puerto 8087) tiene la comparación de **productos** en su propio paquete, `internal/comparison`, con el estándar TAL-78 (`apphttp.NewComparisonRouter`). Las comparaciones de **servicios** (`/comparisons/services`) siguen en `internal/features` y se montan en el mismo router; también cumplen el estándar TAL-78.

Ya no llama a Product API ni a Pricing API: el mejor precio se calcula en SQL con el mismo criterio que Pricing y Search (precio + envío, ofertas disponibles en tiendas activas).

| Método | Ruta | Acceso | Respuesta |
| --- | --- | --- | --- |
| GET | `/comparisons/products/compare?ids=a,b,c` | pública | matriz de comparación (2 a 5 productos, sin guardar) |
| GET | `/comparisons/compatible/{productId}?limit=&offset=` | pública | alternativas de la misma categoría + `X-Total-Count` |
| POST | `/comparisons/assistant` | pública (rate limit) | respuesta del asistente de comparación (IA) |
| POST | `/comparisons/products` | sesión | 201 + `Location` con la comparación guardada |
| GET | `/comparisons/products?limit=&offset=` | sesión | mis comparaciones + `X-Total-Count` |
| GET | `/comparisons/products/{id}` | sesión (dueño) | comparación guardada con su matriz |
| PUT | `/comparisons/products/{id}` | sesión (dueño) | renombrar: `{"name": "..."}` |
| DELETE | `/comparisons/products/{id}` | sesión (dueño) | 204 |
| POST | `/comparisons/products/{id}/items` | sesión (dueño) | agregar `{"productId": "..."}` → 201 con la comparación |
| DELETE | `/comparisons/products/{id}/items/{productId}` | sesión (dueño) | 204 |

### Propiedad (corrige S3 de TAL-69)

- El usuario sale del **token de sesión** de Auth API (`Authorization: Bearer <token>`, `authz.RequireSession`), nunca del body: enviar `userId` responde 400 `unknown_field`.
- Las consultas filtran por `user_id`: una comparación de otro usuario responde **404**, igual que una inexistente, para no revelar que existe.
- Sin token o con un token vencido: 401 `unauthorized`.

### Reglas

- Todos los productos deben ser de la **misma categoría** (`product.category_id`; equivale a la misma `category_ref`, porque el enlace con la categoría anterior es único).
- Entre 2 y 5 productos en `/compare`; entre 1 y 5 en una comparación guardada. Sin repetidos.
- Solo productos activos; un producto dado de baja deja de aparecer en las comparaciones guardadas.
- El nombre es opcional al crear (por defecto "Comparación de productos"), máximo 150 caracteres.

### La matriz

- `products`: columnas en el orden pedido, con `bestPrice` (precio total), `bestStore`, `offerCount` y
  `lastUpdated` (TAL-223): fecha de la última verificación de esa mejor oferta, RFC 3339 UTC, o
  `null` si el producto no tiene ofertas. Es lo que permite decir si el precio es de hoy.
- `attributes`: atributos **`comparable`** de la categoría (TAL-71), en `displayOrder`, con valores tipados alineados con `products` (`null` si el producto no lo tiene).
- `specifications`: especificaciones anteriores comparables (`product_category_specification`), como texto. Se incluyen porque los productos existentes todavía no tienen atributos (TAL-70 no los migró).
- Solo aparecen filas con al menos un valor. `differs` indica si los valores difieren (un `null` cuenta como distinto), para mostrar "solo diferencias".
- No se marca un "ganador" por atributo: no siempre más (o menos) es mejor. El precio menor sí se informa por producto.

```http
GET /comparisons/products/compare?ids=c5b9f86a-703b-4569-9902-0ccadfcdab60,bb0aa9e9-1ca2-4aff-bd18-f8fc01666b7c
```

```json
{
  "category": { "slug": "refrigeradores", "name": "Refrigeradores" },
  "products": [
    { "id": "c5b9f86a-703b-4569-9902-0ccadfcdab60", "name": "Refrigerador Midea Side by Side No Frost 555 L",
      "brand": "Midea", "model": "MDRS710FGE50IN",
      "imageUrl": "https://rimage.ripley.cl/home.ripley/Attachment/WOP/1/2000406824253/full_image-2000406824253",
      "bestPrice": "332991.00", "bestStore": "Ripley", "offerCount": 3,
      "lastUpdated": "2026-11-22T15:43:48Z" },
    { "id": "bb0aa9e9-1ca2-4aff-bd18-f8fc01666b7c", "name": "Refrigerador LG French Door 530 L",
      "brand": "LG", "model": "GM53", "imageUrl": "https://placehold.co/640x640?text=DEMO-LG-FRIDGE-530",
      "bestPrice": "899990.00", "bestStore": "Paris", "offerCount": 2,
      "lastUpdated": "2026-11-22T15:43:48Z" }
  ],
  "attributes": [
    { "key": "capacidad_litros", "label": "Capacidad", "dataType": "integer", "unit": "L", "values": [555, 530], "differs": true },
    { "key": "eficiencia_energetica", "label": "Eficiencia energética", "dataType": "enum", "unit": null, "values": ["D", "A+"], "differs": true },
    { "key": "no_frost", "label": "No Frost", "dataType": "boolean", "unit": null, "values": [true, true], "differs": false }
  ],
  "specifications": [
    { "name": "Capacidad", "unit": "L", "values": ["555", "530"], "differs": true },
    { "name": "Eficiencia energetica", "unit": null, "values": ["D", "A+"], "differs": true }
  ]
}
```

### Asistente de comparación (IA)

`POST /comparisons/assistant` responde en lenguaje natural una pregunta sobre los
**productos que el usuario está comparando**. Vive en `internal/assistant/` y su
configuración declarativa, el prompt y la allowlist de campos están en la carpeta
raíz `assistant/`, copiada a `internal/assistant/assets/` para `go:embed`
(`scripts/sync-assistant-assets.sh`; `--check` falla si divergen).

Proveedores (`AI_PROVIDER`): `auto` (por defecto) usa Gemini si existe
`AI_API_KEY`; sin clave la ruta responde 503 `ai_unavailable` y **no** arma una
respuesta de plantilla. `gemini` exige clave; `pollinations` usa el endpoint
gratuito sin clave (límite aproximado de 1 respuesta cada 45 s por IP);
`compatible` apunta a cualquier API compatible OpenAI con `AI_BASE_URL`; `off`
desactiva y responde 503. `AI_BASE_URL` también cambia el endpoint de Gemini sin
tocar código. El modelo se cambia con `AI_MODEL` (por defecto
`gemini-3.5-flash`).

El clúster usa DeepSeek con el camino `compatible`, porque es más barato y no
exige cambiar código:

```env
AI_PROVIDER=compatible
AI_BASE_URL=https://api.deepseek.com/v1
AI_MODEL=deepseek-chat
AI_API_KEY=sk-...   # Secret soloservis-ai
```

Cambiar a Gemini, Groq, Ollama o un proxy propio es cambiar esas variables y
reiniciar el pod: el binario es el mismo.

```http
POST /comparisons/assistant
Content-Type: application/json

{ "productIds": ["<uuid-a>", "<uuid-b>"], "question": "¿Cuál conviene para una familia de 5?" }
```

```json
{
  "answer": "Para una familia de 5, el ...",
  "sources": ["<uuid-a>", "<uuid-b>"],
  "provider": "gemini",
  "model": "gemini-3.5-flash",
  "cached": false
}
```

- El cliente manda **solo** `productIds` (2 a 5, misma categoría, sin repetidos) y
  `question` (1 a 300 caracteres, texto plano). El contexto (precio, tienda,
  fecha de verificación de la oferta, atributos y especificaciones) lo arma el
  servidor desde PostgreSQL con la matriz de `/compare`; nunca llegan imágenes,
  URLs, descripciones ni datos de usuario.
- Errores: `400` al decodificar, `422 validation_failed` al incumplir reglas
  (incluida `incompatible_category`), `404 not_found` si un producto no existe,
  `429 rate_limited` con `Retry-After` al superar 10/min o 100/día por IP y `503
  ai_unavailable` si el proveedor falla o el asistente está en `off`. Los
  detalles internos y la clave no aparecen en la respuesta.
- Guardrails de salida: texto plano, mención de al menos un producto comparado y
  largo máximo de 900 caracteres; si no se cumple, responde la frase fija de
  rechazo de `prompts/system.es.md`.
- Caché en memoria (24 h) por hash de IDs ordenados + pregunta normalizada. La
  auditoría va a `ai_comparison_query` (migración 029); un fallo al auditar no
  cambia la respuesta.

<!--
PENDIENTE: el texto original de la rama TAL-227 que se pegó termina aquí (a mitad de §12.6).
Antes de hacer commit, agregar desde la rama (git show TAL-227/normalizacion-de-productos:backend/README.md):
  - el resto de Comparison API (alternativas compatibles, errores, tests)
  - §12.7 y §12.8 (historial/búsquedas guardadas y estándar TAL-78), renumerándolos como 25.7 y 25.8
-->

---

**SOLOServis — Backend**