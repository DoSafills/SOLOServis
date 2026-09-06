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
