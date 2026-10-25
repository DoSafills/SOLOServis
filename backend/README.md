# SoloServis — Backend

Backend de la plataforma **SoloServis**, encargado de exponer la API HTTP, acceder a PostgreSQL y entregar los datos de productos al frontend.

Actualmente el backend está implementado en **Go**, utilizando **Chi** como router, **pgx/v5** para PostgreSQL y **sqlc** para generar el código de acceso a datos a partir de consultas SQL.

---

## 1. Arquitectura

La comunicación principal del sistema sigue este flujo:

```text
┌──────────────────────┐
│      Frontend        │
│   React + TypeScript │
└──────────┬───────────┘
           │
           │ HTTP / JSON
           ▼
┌──────────────────────┐
│      Go API          │
│       + Chi          │
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│       Handler        │
│ recibe la petición   │
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│     Repository       │
│ acceso a los datos   │
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│        sqlc          │
│ código generado      │
│ desde SQL             │
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│       pgx/v5         │
│ driver PostgreSQL    │
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│     PostgreSQL       │
│       soloservis     │
└──────────────────────┘
```

### Flujo de una petición

Por ejemplo, cuando el frontend solicita:

```http
GET /products
```

ocurre lo siguiente:

```text
Frontend
   │
   │ GET /products
   ▼
Chi Router
   │
   ▼
Product Handler
   │
   ▼
Product Repository
   │
   ▼
sqlc → ListProducts()
   │
   ▼
pgx/v5
   │
   ▼
PostgreSQL
   │
   ▼
datos SQL
   │
   ▼
Repository
   │
   ▼
Handler
   │
   ▼
DTO
   │
   ▼
JSON
   │
   ▼
Frontend
```

El frontend **no se conecta directamente a PostgreSQL**.

Toda comunicación con la base de datos pasa por el backend.

---

# 2. Tecnologías

| Tecnología | Función                           |
| ---------- | --------------------------------- |
| Go         | Lenguaje del backend              |
| Chi        | Router HTTP                       |
| PostgreSQL | Base de datos                     |
| pgx/v5     | Conexión y driver PostgreSQL      |
| sqlc       | Generación de código Go desde SQL |
| godotenv   | Carga de variables desde `.env`   |
| Docker     | Ejecución de PostgreSQL           |

---

# 3. Estructura del backend

```text
backend/
│
├── .env
├── .env.example
├── .gitignore
├── go.mod
├── go.sum
├── sqlc.yaml
│
├── cmd/
│   └── server/
│       └── main.go
│
├── database/
│   ├── Readme.md
│   │
│   ├── migrations/
│   │   ├── 001_schema.sql
│   │   ├── 002_views.sql
│   │   └── 003_seed.sql
│   │
│   └── queries/
│       └── products.sql
│
└── internal/
    │
    ├── config/
    │   └── config.go
    │
    ├── database/
    │   ├── postgres.go
    │   │
    │   └── generated/
    │       ├── db.go
    │       ├── models.go
    │       └── products.sql.go
    │
    ├── http/
    │   └── router.go
    │
    └── products/
        ├── handler.go
        ├── repository.go
        │
        └── dto/
            ├── offer.go
            ├── product.go
            └── product_detail.go
```

---

# 4. `cmd/server/main.go`

Es el punto de entrada de la aplicación.

Su responsabilidad es:

1. Cargar la configuración.
2. Crear el pool de conexiones PostgreSQL.
3. Comprobar la conexión.
4. Crear el router.
5. Iniciar el servidor HTTP.

Flujo:

```text
main.go
   │
   ├── config.Load()
   │
   ├── database.NewPostgresPool()
   │
   ├── database.Ping()
   │
   ├── http.NewRouter()
   │
   └── ListenAndServe()
```

El servidor actualmente utiliza:

```text
http://localhost:8080
```

---

# 5. Configuración

La configuración se encuentra en:

```text
internal/config/config.go
```

Se utilizan variables de entorno.

## `.env`

El archivo `.env` es local y **no debe subirse al repositorio** si contiene credenciales reales.

Ejemplo:

```env
PORT=8080
DATABASE_URL=postgres://postgres:postgres@localhost:5432/soloservis
```

También existe:

```text
.env.example
```

Este archivo sirve como referencia para crear el `.env`.

---

# 6. PostgreSQL

PostgreSQL es la base de datos principal del backend.

Actualmente se ejecuta mediante Docker.

El contenedor utilizado durante desarrollo es:

```text
soloservis_postgres
```

El puerto de PostgreSQL es:

```text
5432
```

La aplicación Go se conecta mediante:

```text
DATABASE_URL
```

Ejemplo:

```text
postgres://postgres:postgres@localhost:5432/soloservis
```

---

# 7. Inicialización de la base de datos

Las estructuras de la base de datos están en:

```text
database/migrations/
```

Actualmente:

```text
001_schema.sql
002_views.sql
003_seed.sql
004_platform_apis.sql
005_demo_catalog.sql
006_demo_catalog_services.sql
006_refrigeradores.sql
007_Computacion.sql
007_service_cart.sql
008_normalize_prices_to_clp.sql
008_Telefonia.sql
012_categories.sql
013_categories_seed.sql
```

## `001_schema.sql`

Contiene la estructura principal de la base de datos.

Entre las entidades utilizadas por el módulo de productos se encuentran:

```text
brand
product_category
product
product_image
product_category_specification
product_specification_value
store
product_offer
product_price_history
product_review
```

## `002_views.sql`

Contiene vistas derivadas de la información almacenada.

Actualmente existe:

```text
product_rating_summary
```

Esta vista calcula:

```text
promedio de rating
cantidad de reviews
```

a partir de `product_review`.

El rating no se mantiene como un valor duplicado dentro de `product`.

## `003_seed.sql`

Contiene datos iniciales para desarrollo.

Actualmente existen productos de prueba como:

```text
Refrigerador Samsung No Frost
Refrigerador LG Side by Side
```

También existen tiendas, ofertas, imágenes y reviews utilizadas para comprobar el flujo completo.

## `005_demo_catalog.sql`

Agrega de forma repetible 22 productos de 8 categorias y 5 tiendas adicionales, con marcas, especificaciones, imagenes de placeholder, ofertas en varias tiendas e historial de precios. Incluye stock agotado, envio pagado/gratis y productos nuevos, usados o reacondicionados para probar busqueda, filtros, comparaciones, detalle de tiendas y precios. Docker lo ejecuta automaticamente en una base nueva. Para una base ya inicializada, ejecutar desde la raiz del repositorio:

```powershell
docker compose -f docker/compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U postgres -d soloservis -f /docker-entrypoint-initdb.d/005_demo_catalog.sql
```

---

# 8. Conexión con PostgreSQL

La conexión se encuentra en:

```text
internal/database/postgres.go
```

La función principal es:

```go
NewPostgresPool(databaseURL string)
```

Esta crea un:

```text
pgxpool.Pool
```

El pool permite reutilizar conexiones a PostgreSQL en lugar de crear una conexión nueva para cada petición.

La configuración actual establece límites para el pool:

```text
MaxConns       = 10
MinConns       = 2
MaxConnLifetime = 1 hora
MaxConnIdleTime = 30 minutos
```

Antes de iniciar el servidor se ejecuta un `Ping` a PostgreSQL.

Si PostgreSQL no está disponible, el backend termina indicando el error.

---

# 9. Router HTTP

El router se encuentra en:

```text
internal/http/router.go
```

Actualmente existen las siguientes rutas:

```http
GET /health
GET /products
GET /products/{publicID}
```

---

# 10. Endpoint `/health`

Permite comprobar que:

1. El servidor Go está funcionando.
2. El backend puede comunicarse con PostgreSQL.

Petición:

```http
GET http://localhost:8080/health
```

Respuesta exitosa:

```json
{
  "status": "ok",
  "service": "soloservis-api",
  "database": "connected"
}
```

Si PostgreSQL no responde:

```json
{
  "status": "error",
  "service": "soloservis-api",
  "database": "unavailable"
}
```

---

# 11. Endpoint `/products`

Petición:

```http
GET http://localhost:8080/products
```

Esta ruta devuelve el listado de productos activos.

El flujo es:

```text
GET /products
      ↓
ProductHandler.List()
      ↓
Repository.List()
      ↓
sqlc.ListProducts()
      ↓
PostgreSQL
```

El resultado se transforma a un DTO antes de enviarse al frontend.

Ejemplo:

```json
[
  {
    "id": "b87a37e7-905a-494f-9930-2d0278cffcd5",
    "name": "Refrigerador Samsung No Frost",
    "brand": "Samsung",
    "model": "RT38K",
    "category": "Refrigeradores",
    "description": "Refrigerador no frost de 380 litros",
    "rating": 4.5,
    "reviewCount": 2
  }
]
```

---

# 12. Endpoint `/products/{publicID}`

Permite obtener el detalle completo de un producto.

Ejemplo:

```http
GET /products/b87a37e7-905a-494f-9930-2d0278cffcd5
```

El identificador público utilizado por la API es un UUID.

El flujo es:

```text
GET /products/{publicID}
          ↓
ProductHandler.GetByPublicID()
          ↓
validación UUID
          ↓
Repository.GetDetailByPublicID()
          ↓
sqlc
          ↓
PostgreSQL
```

Después se obtienen adicionalmente:

```text
imagenes
ofertas
tiendas
precios
stock
envío
```

y se construye el DTO final.

---

# 12.1. Endpoints `/categories` (TAL-71)

Categorías unificadas de productos y servicios con sus **definiciones de atributos dinámicos**. Son la base de la validación de atributos (TAL-72), los filtros (TAL-75, `filterable`) y la comparación (TAL-76, `comparable`).

- Servicio: **Product API** (`cmd/product-api`, puerto 8081). Código en `internal/categories`.
- Solo lectura: `POST`/`PUT`/`PATCH`/`DELETE` responden `405`. No hay endpoints de escritura (requerirían autenticación).
- Errores con el estándar TAL-78 (`application/problem+json`, ver `docs/estandares/TAL-78-estandar-api.md`).
- Incluye el validador de atributos (TAL-72) que usan los endpoints de escritura de productos y servicios (ver más abajo).
- Solo se exponen categorías con `active = TRUE`.

## Esquema (migraciones `012_categories.sql` y `013_categories_seed.sql`)

Los números 009–011 quedan reservados para las migraciones de TAL-67/68 (PR #30). Si ese PR agrega `backend/database/migration_order.txt`, estas dos migraciones deben sumarse al final del manifiesto.

| Tabla | Columnas principales | Restricciones en BD |
| --- | --- | --- |
| `categories` | `slug`, `name`, `item_type` (`product`/`service`), `description`, `active`, `legacy_product_category_id`, `legacy_service_category_id`, timestamps | `slug` UNIQUE + CHECK `^[a-z0-9-]{1,64}$`; CHECK de `item_type`; enlace legado UNIQUE, FK `ON DELETE SET NULL` y del mismo tipo de ítem |
| `category_attributes` | `key`, `label`, `data_type`, `unit`, `required`, `comparable`, `filterable`, `min_value`, `max_value`, `max_length`, `allowed_values`, `display_order` | FK a `categories` `ON DELETE CASCADE`; UNIQUE `(category_id, key)`; CHECK `key ^[a-z][a-z0-9_]{0,63}$`; `data_type` ∈ `string`, `integer`, `decimal`, `boolean`, `enum`; `allowed_values` solo y obligatorio en `enum`; `min`/`max` solo numéricos (enteros en `integer`, `min <= max`); `max_length` solo `string` (1–255); `unit` solo numéricos |

Las tablas antiguas (`product_category`, `service_category` y sus `*_specification`) **no se modificaron**. Las columnas `legacy_*` enlazan cada categoría nueva con su equivalente antigua para migrar productos y servicios en TAL-70/72.

Seeds: `refrigeradores`, `notebooks`, `televisores` (producto) e `internet-fibra-optica` (servicio), con atributos de los cinco `data_type`.

## `GET /categories`

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

## `GET /categories/{slug}`

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

## Errores

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

Con la base de datos caída:

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

## Validación de atributos (TAL-72)

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

## Exposición por nginx (pendiente de aprobación)

Hoy `docker/frontend/nginx.conf` solo enruta `/products` y `/stores`, así que en k8s `/categories` cae en la SPA. Para exponerlo (solo GET) haría falta un bloque igual al de `/products`:

```nginx
location ~ ^/categories(/.*)?$ {
    limit_except GET { deny all; }
    proxy_pass http://product-api:8081;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_connect_timeout 5s;
    proxy_read_timeout 30s;
}
```

## Tests

- Unitarios, sin BD (`internal/categories/handler_test.go`, `internal/http/router_test.go`): `cd backend && go test ./...`.
- Integración: migraciones sobre base limpia, seeds, CHECK y repositorio (`internal/categories/repository_integration_test.go`). Se omiten si no existe `TEST_DATABASE_URL`:

```bash
docker run -d --name soloservis-test-pg -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=soloservis -p 55432:5432 postgres:18
# aplicar backend/database/migrations/*.sql en orden, igual que k8s/base/migrations/migrate.sh
cd backend && TEST_DATABASE_URL=postgres://postgres:postgres@localhost:55432/soloservis go test -run Integration ./internal/categories/
```

---


# 13. Handlers

Los handlers se encuentran en:

```text
internal/products/handler.go
```

El handler es responsable de la capa HTTP.

Sus responsabilidades son:

* recibir la petición;
* obtener parámetros;
* validar datos básicos;
* llamar al repository;
* transformar los resultados a DTO;
* devolver JSON;
* devolver códigos HTTP apropiados.

El handler **no debe contener consultas SQL**.

---

# 14. Repository

El repository está en:

```text
internal/products/repository.go
```

Su responsabilidad es comunicarse con la capa generada por sqlc.

Actualmente proporciona operaciones como:

```text
List()
GetByPublicID()
GetDetailByPublicID()
ListOffers()
ListImages()
```

El repository tampoco contiene SQL directamente.

Las consultas se encuentran en:

```text
database/queries/products.sql
```

---

# 15. sqlc

El proyecto utiliza `sqlc`.

Configuración:

```text
sqlc.yaml
```

Las consultas escritas manualmente se encuentran en:

```text
database/queries/
```

Actualmente:

```text
database/queries/products.sql
```

sqlc analiza las consultas SQL y genera código Go.

Los archivos generados se encuentran en:

```text
internal/database/generated/
```

Principalmente:

```text
db.go
models.go
products.sql.go
```

### Importante

Los archivos dentro de:

```text
internal/database/generated/
```

son archivos generados.

No se deben modificar manualmente.

Si se modifica una consulta SQL, se debe volver a ejecutar sqlc.

---

# 16. Consultas de productos

`database/queries/products.sql` contiene actualmente operaciones como:

```text
GetProductByID
GetProductByPublicID
GetProductDetailByPublicID
ListProducts
ListProductOffers
ListProductImages
CreateProduct
UpdateProduct
DeactivateProduct
```

Esto permite separar:

```text
SQL
↓
sqlc
↓
Go
```

en lugar de escribir SQL directamente dentro de los handlers.

---

# 17. DTOs

Los DTO se encuentran en:

```text
internal/products/dto/
```

Actualmente:

```text
product.go
product_detail.go
offer.go
```

Los DTO sirven como contrato entre el backend y el frontend.

Esto evita exponer directamente los modelos internos generados por sqlc.

Por ejemplo, el modelo de PostgreSQL puede contener tipos específicos de `pgtype`, mientras que el JSON público utiliza tipos simples:

```json
{
  "id": "...",
  "name": "...",
  "rating": 4.5
}
```

---

# 18. CORS

El frontend y backend funcionan actualmente en puertos diferentes.

Frontend:

```text
http://localhost:8443
```

Backend:

```text
http://localhost:8080
```

Por esta razón el backend configura CORS.

Actualmente permite:

```text
Origin:
http://localhost:8443
```

y los métodos:

```text
GET
OPTIONS
```

Esto permite que el navegador pueda realizar peticiones desde el frontend hacia la API.

---

# 19. ¿Qué debe estar encendido?

Para trabajar con el sistema completo durante desarrollo se necesitan tres componentes:

```text
┌────────────────────┐
│ PostgreSQL / Docker│
│       :5432        │
└─────────┬──────────┘
          │
          ▼
┌────────────────────┐
│      Go API        │
│       :8080        │
└─────────┬──────────┘
          │
          ▼
┌────────────────────┐
│      Frontend      │
│       :8443        │
└────────────────────┘
```

### 1. PostgreSQL

Debe estar ejecutándose en Docker.

Comprobar:

```powershell
docker ps
```

Debe aparecer el contenedor de PostgreSQL.

### 2. Backend

Desde:

```text
SoloServise/backend
```

ejecutar:

```powershell
go run ./cmd/server
```

Debe aparecer:

```text
PostgreSQL connection established
SoloServis API running on http://localhost:8080
```

### 3. Frontend

Desde:

```text
SoloServise/fronted
```

ejecutar:

```powershell
pnpm dev
```

El frontend utiliza la URL configurada en:

```text
fronted/.env
```

Actualmente:

```env
VITE_API_URL=http://localhost:8080
```

---

# 20. Comprobación rápida

Una vez iniciado PostgreSQL y el backend:

### Health

```powershell
Invoke-RestMethod http://localhost:8080/health
```

Debe mostrar:

```text
status service        database
------ -------        --------
ok     soloservi...   connected
```

### Productos

```powershell
Invoke-RestMethod http://localhost:8080/products
```

Debe devolver los productos almacenados en PostgreSQL.

### Producto individual

Utilizar el UUID devuelto por `/products`:

```powershell
Invoke-RestMethod "http://localhost:8080/products/{UUID}"
```

Debe devolver el detalle del producto.

---

# 21. Regenerar código de sqlc

Cuando se modifique:

```text
database/queries/
```

o el esquema utilizado por sqlc, se debe regenerar el código.

Desde `backend`:

```powershell
sqlc generate
```

Esto actualiza:

```text
internal/database/generated/
```

Después se recomienda comprobar:

```powershell
go build ./...
```

---

# 22. Comprobación del backend

Antes de considerar una modificación terminada:

```powershell
gofmt -w .
go build ./...
```

Si existen pruebas:

```powershell
go test ./...
```

La compilación debe terminar sin errores.

---

# 23. Separación de responsabilidades

La arquitectura actual busca mantener responsabilidades separadas.

### `main.go`

Arranque de la aplicación.

### `config`

Configuración y variables de entorno.

### `database`

Conexión con PostgreSQL.

### `http`

Rutas HTTP y middleware.

### `products/handler.go`

Recibe y responde peticiones HTTP.

### `products/repository.go`

Acceso a datos mediante sqlc.

### `products/dto/`

Define la representación pública de los datos.

### `database/queries/`

SQL escrito manualmente.

### `database/generated/`

Código generado automáticamente por sqlc.

---

# 24. Lo que NO hace actualmente el backend

El backend actual es una primera implementación funcional.

Todavía no están implementados todos los módulos de SoloServis.

Entre las funcionalidades pendientes se encuentran, entre otras:

```text
usuarios
autenticación
favoritos
comparaciones persistentes
servicios
proveedores
búsqueda avanzada
filtros avanzados
historial de precios expuesto completamente por API
reviews mediante API
administración
scraping
workers
caché Redis
IA para interpretación de búsquedas
```

Estas funcionalidades se incorporarán progresivamente.

---

# 25. Estado actual de la integración Frontend ↔ Backend

Actualmente existe un flujo funcional para productos:

```text
PostgreSQL
    ↓
Go
    ↓
Chi
    ↓
Product Handler
    ↓
Repository
    ↓
sqlc
    ↓
JSON
    ↓
Frontend React
```

El frontend actualmente consume datos reales de PostgreSQL para el listado de productos.

La migración desde `mockData` todavía es parcial.

### Actualmente conectado

```text
Home
 └── productos reales

Productos / búsqueda
 └── productos reales
```

### Todavía pendiente

```text
ProductDetailPage
 └── migrar desde mockData

Ofertas en ProductCard
 └── conectar con API

Imágenes en ProductCard
 └── conectar con API

Historial de precios
 └── conectar con API

Servicios
 └── todavía utiliza datos mock
```

Por lo tanto, **no se debe eliminar `mockData` todavía**.

Se eliminará progresivamente a medida que cada módulo tenga su correspondiente endpoint y consumo desde el frontend.

---

# 26. Regla importante para futuras modificaciones

Cuando se agregue una nueva funcionalidad al backend, mantener el flujo:

```text
SQL
 ↓
sqlc
 ↓
Repository
 ↓
Handler
 ↓
DTO
 ↓
JSON
```

Evitar:

```text
Handler
 ↓
SQL directo
```

y evitar:

```text
Frontend
 ↓
PostgreSQL
```

El frontend siempre debe comunicarse con PostgreSQL **a través de la API**.

---

# 27. Flujo completo de desarrollo

---

# 28. Microservicios de la plataforma

El backend ejecuta APIs independientes en el mismo módulo Go y comparte PostgreSQL. Cada comando de `cmd/` usa un router de dominio bajo `internal/` y el runner común para cargar configuración, comprobar la base e iniciar HTTP.

| Servicio | Puerto | Rutas principales |
| --- | ---: | --- |
| Product API | 8081 | `/products` |
| Store API | 8082 | `/stores` |
| Service API | 8083 | `/services` |
| User API | 8084 | `/users/{id}` |
| Pricing API | 8085 | `/offers`, `/offers/product/{productId}`, historial |
| Search API | 8086 | productos, categorías, marcas, historial y búsquedas guardadas |
| Comparison API | 8087 | comparaciones e items de productos y servicios; productos compatibles |
| Auth API | 8088 | registro, login, refresh, logout y validación |
| Favorites API | 8089 | favoritos de productos y servicios |
| Cart API | 8090 | carrito asociado a una oferta de tienda |
| Review API | 8091 | reseñas de producto, tienda y servicio |
| Watchlist API | 8092 | productos y precio objetivo |
| Notification API | 8093 | notificaciones y estado de lectura |
| Ingestion API | 8094 | productos, ofertas y ejecuciones de sync |

La navegación, el carrito local y las comparaciones no requieren iniciar sesión. La autenticación se realiza mediante Auth API (registro, inicio, renovación y cierre de sesión); favoritos, historial de búsqueda, búsquedas guardadas y carrito sincronizado requieren una sesión válida. El frontend conserva la sesión en `localStorage` y renueva el token de acceso al vencerlo. Favorites, Cart y Search APIs validan el token y usan el usuario de la sesión, sin aceptar que el cliente elija otro `userId`.

Las nuevas tablas se definen en `database/migrations/004_platform_apis.sql`. Docker ejecuta las migraciones automáticamente al inicializar un volumen PostgreSQL nuevo. La compatibilidad del carrito con ofertas de servicios se agrega en `database/migrations/007_service_cart.sql`. La migración `008_normalize_prices_to_clp.sql` etiqueta los precios existentes como CLP sin cambiar sus importes (conversión temporal 1:1) y deja triggers para normalizar la moneda de nuevas ofertas. Los valores que originalmente estaban en USD/EUR son referenciales y no representan una conversión real a pesos chilenos. Las categorías unificadas y sus definiciones de atributos (TAL-71) se crean en `012_categories.sql` y se cargan en `013_categories_seed.sql` (ver sección 12.1).

Para un volumen existente que ya tiene aplicadas las migraciones anteriores, aplica las migraciones pendientes una vez desde el root del proyecto:

```powershell
docker compose -f docker/compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U postgres -d soloservis -f /docker-entrypoint-initdb.d/007_service_cart.sql
docker compose -f docker/compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U postgres -d soloservis -f /docker-entrypoint-initdb.d/008_normalize_prices_to_clp.sql
docker compose -f docker/compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U postgres -d soloservis -f /docker-entrypoint-initdb.d/012_categories.sql
docker compose -f docker/compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U postgres -d soloservis -f /docker-entrypoint-initdb.d/013_categories_seed.sql
```

Las integraciones entre servicios usan las URLs internas definidas en `docker/compose.yml`: Search, Comparison, Favorites y Cart consultan Product API; Comparison y Cart consultan Pricing API; Review valida producto/tienda/servicio; Watchlist consulta Pricing; Pricing genera notificaciones mediante Notification API; e Ingestion delega en Product, Store y Pricing APIs. User API solo expone el perfil asociado al token de sesión autenticado.

Para compilar y ejecutar todo el stack:

```powershell
cd backend
gofmt -w .
go build ./cmd/...
go test ./...
cd ..
docker compose -f docker/compose.yml up -d --build
```

Cada API expone `GET /health`, que informa el estado de PostgreSQL.

Para una modificación de datos de productos:

```text
1. Modificar esquema si es necesario
        ↓
2. Modificar database/queries/*.sql
        ↓
3. Ejecutar sqlc generate
        ↓
4. Adaptar Repository
        ↓
5. Adaptar Handler
        ↓
6. Crear/adaptar DTO
        ↓
7. Exponer endpoint
        ↓
8. Consumir endpoint desde Frontend
        ↓
9. Probar API
        ↓
10. Probar interfaz
        ↓
11. go build ./...
        ↓
12. git commit
```

---

# 28. Puertos utilizados

| Componente | Puerto |
| ---------- | -----: |
| PostgreSQL | `5432` |
| Go API     | `8080` |
| Frontend   | `8443` |

---

# 29. Resumen

El backend de SoloServis funciona actualmente como una API REST escrita en Go.

La aplicación recibe peticiones HTTP desde el frontend, procesa las solicitudes mediante Chi, obtiene los datos utilizando repositories y sqlc, consulta PostgreSQL mediante pgx/v5 y devuelve respuestas JSON.

La separación principal es:

```text
Frontend
   ↓
HTTP
   ↓
Chi
   ↓
Handler
   ↓
Repository
   ↓
sqlc
   ↓
pgx/v5
   ↓
PostgreSQL
```

Esta estructura permite que cada capa tenga una responsabilidad concreta y facilita la expansión futura del sistema hacia servicios, usuarios, scraping, historial de precios, favoritos, comparaciones y otras funcionalidades.
