# Verificación de citas dudosas y resumen del runner

## Tarea 1 — citas dudosas (solo lectura)

### 1a) backend/database/migrations/001_schema.sql

Hecho: el archivo real tiene 469 líneas. La sección relevante es:

```sql
CREATE TABLE product_offer (
    id               SERIAL PRIMARY KEY,
    product_id       INTEGER NOT NULL REFERENCES product(id) ON DELETE CASCADE,
    store_id         INTEGER NOT NULL REFERENCES store(id) ON DELETE CASCADE,
    price            NUMERIC(12,2) NOT NULL CHECK (price >= 0),
    list_price       NUMERIC(12,2) CHECK (list_price >= 0),
    currency         CHAR(3) NOT NULL DEFAULT 'CLP',
    shipping_cost    NUMERIC(12,2) DEFAULT 0 CHECK (shipping_cost >= 0),
    shipping_free    BOOLEAN NOT NULL DEFAULT FALSE,
    available        BOOLEAN NOT NULL DEFAULT TRUE,
    stock            INTEGER CHECK (stock >= 0),
    -- DECISIÓN: valores estándar de condición en e-commerce
    condition        VARCHAR(20) NOT NULL DEFAULT 'new'
                       CHECK (condition IN ('new', 'used', 'refurbished')),
    product_url      TEXT,
    last_updated     TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (product_id, store_id)
);
```

Y la parte de índices reales es:

```sql
CREATE INDEX idx_product_category ON product(category_id);
CREATE INDEX idx_product_brand ON product(brand_id);
CREATE INDEX idx_product_offer_product ON product_offer(product_id);
CREATE INDEX idx_product_offer_store ON product_offer(store_id);
CREATE INDEX idx_product_price_history_offer ON product_price_history(product_offer_id);
```

Resultado:
- Cantidad total de líneas: 469.
- En la sección 118-145 existe `UNIQUE (product_id, store_id)`.
- En la sección 457-460 existen `idx_product_offer_product` e `idx_product_offer_store`.
- No existe un `CREATE INDEX idx_product_offer_*` adicional para `product_url` en ese archivo.
- Estado: OK para la afirmación “hay un UNIQUE en product_offer y hay índices por product_id/store_id”, NO COINCIDE para la afirmación de un índice específico sobre `product_url` o para cualquier afirmación que diga que el UNIQUE sea sobre `(product_id, store_id, product_url)`.

### 1b) backend/internal/features/router.go

Rutas reales en los rangos de archivo consultados:

```go
case "search-api":
    r.Get("/search/products", h.SearchProducts)
    r.Get("/search/categories", h.SearchCategories)
    r.Get("/search/brands", h.SearchBrands)
    r.Group(func(r chi.Router) {
        r.Use(h.requireSession)
        r.Post("/search/history", h.CreateSearchHistory)
        r.Get("/search/history", h.GetSearchHistory)
        r.Post("/search/saved", h.SaveSearch)
        r.Get("/search/saved", h.GetSavedSearches)
        r.Delete("/search/saved/{id}", h.DeleteSavedSearch)
    })
```

```go
case "favorites-api":
    r.Group(func(r chi.Router) {
        r.Use(h.requireSession)
        r.Get("/favorites", h.ListFavorites)
        r.Post("/favorites/{productId}", h.AddFavorite)
        r.Delete("/favorites/{productId}", h.DeleteFavorite)
        r.Get("/favorites/{productId}", h.GetFavorite)
        r.Get("/favorites/services", h.ListServiceFavorites)
        r.Post("/favorites/services/{serviceId}", h.AddServiceFavorite)
        r.Delete("/favorites/services/{serviceId}", h.DeleteServiceFavorite)
    })
```

```go
case "cart-api":
    r.Group(func(r chi.Router) {
        r.Use(h.requireSession)
        r.Get("/cart", h.GetCart)
        r.Post("/cart/items", h.AddCartItem)
        r.Put("/cart/items/{id}", h.UpdateCartItem)
        r.Delete("/cart/items/{id}", h.DeleteCartItem)
        r.Delete("/cart", h.ClearCart)
        r.Get("/cart/count", h.CartCount)
    })
```

```go
case "ingestion-api":
    r.Post("/ingestion/products", h.IngestProduct)
    r.Post("/ingestion/offers", h.IngestOffer)
    r.Post("/ingestion/sync", h.SyncIngestion)
```

Resultado:
- Las rutas dentro de grupos con `r.Use(h.requireSession)` son las de `search-api`, `favorites-api` y `cart-api`.
- Las rutas de `ingestion-api` no están protegidas por `requireSession` y quedan sin middleware de sesión.
- Estado: OK para la distinción entre rutas protegidas y rutas abiertas.

### 1c) requireSession y extracción del Bearer

Hecho real de `backend/internal/features/handler.go`:

```go
func (h *Handler) requireSession(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := bearerToken(r)
        if token == "" {
            apiutil.WriteError(w, http.StatusUnauthorized, "bearer token is required")
            return
        }
```

Hecho real de `backend/internal/features/auth.go`:

```go
func bearerToken(r *http.Request) string {
    value := r.Header.Get("Authorization")
    if len(value) > 7 && strings.EqualFold(value[:7], "Bearer ") {
        return strings.TrimSpace(value[7:])
    }
    return ""
}
```

Resultado:
- `requireSession` evalúa el header `Authorization` y exige un token Bearer no vacío.
- `bearerToken` recorta el prefijo `Bearer ` con `strings.EqualFold(value[:7], "Bearer ")` y devuelve el resto limpio.
- Estado: OK.

### 1d) Otras citas en decisiones.md y plan_pruebas.md

| Cita | Afirmación | Líneas reales | Estado |
|---|---|---|---|
| `backend/internal/features/catalog.go:14-25` | `offerInput` usa `Price float64` (y `ListPrice *float64` como el único campo opcional) | `backend/internal/features/catalog.go:14-25` | OK |
| `backend/internal/features/catalog.go:71-75` | validación actual exige `input.Price < 0` y no distingue ausencia de 0 | `backend/internal/features/catalog.go:67-75` | OK |
| `backend/internal/features/ingestion.go:64-82` | ingesta de oferta valida `storeId` remoto y delega en Pricing API | `backend/internal/features/ingestion.go:64-82` | OK |
| `backend/internal/features/router.go:95-98` | rutas `/ingestion/*` están abiertas y sin `requireSession` | `backend/internal/features/router.go:95-98` | OK |
| `backend/database/migrations/001_schema.sql:118-145` | producto de oferta tiene `UNIQUE (product_id, store_id)` y no hay `active` para product_offer | `backend/database/migrations/001_schema.sql:118-145` | OK |
| `backend/database/migrations/008_normalize_prices_to_clp.sql:6-14` | la lógica normaliza la etiqueta de moneda a CLP | `008_normalize_prices_to_clp.sql:6-14` define ambas funciones; `:26-30` y `:38-42` conectan los triggers | OK |
| `backend/internal/features/router.go:25-31` | ejemplo de rutas protegidas con `requireSession` | el grupo comienza en `router.go:27`, middleware en `:28`, las rutas siguen en `:29-33`; el rango más preciso es `:27-34` | NO COINCIDE (cita imprecisa; usar `:27-34`) |
| `backend/internal/products/handler.go:24-50` | validación `imageUrl` HTTP(S) en Product API | `backend/internal/products/handler.go:24-50` | OK |
| `backend/internal/features/ingestion.go:30-45` | IngestProduct delega a Product API cuando hay `imageUrl` | `backend/internal/features/ingestion.go:24-45` | OK |
| `backend/internal/features/catalog.go:96-101` | el handler inserta `product_url` directo al `INSERT` | `backend/internal/features/catalog.go:96-101` | OK |
| `backend/database/migrations/001_schema.sql:130-131` | `condition` tiene `CHECK` para `new`, `used`, `refurbished` | `backend/database/migrations/001_schema.sql:129-135` | OK |
| `backend/database/migrations/001_schema.sql:128` | `stock` tiene `CHECK (stock >= 0)` | `backend/database/migrations/001_schema.sql:127-128` | OK |
| `backend/database/migrations/001_schema.sql:56-70` | SKU tiene restricción UNIQUE | Dentro del rango citado se declara la restricción del SKU | OK |
| `backend/database/migrations/001_schema.sql:104-121` | la oferta referencia `store_id` y no determina identidad por nombre | `001_schema.sql:118-121` define la FK; el nombre visible no es la clave de oferta | OK |
| `backend/internal/features/catalog.go:71-110` | crear oferta resuelve producto, inserta oferta e historial | `catalog.go:71-110`; la respuesta de creación se arma después | OK |
| `backend/internal/features/catalog.go:78-82` | `ProductID` se resuelve antes de insertar | `catalog.go:78-82` | OK |
| `backend/internal/features/catalog.go:84-87` | asigna valores por defecto de moneda/condición | `catalog.go:84-87` | OK |
| `backend/internal/features/catalog.go:96-106` | inserta oferta y convierte fallo de inserción a error de petición | `catalog.go:96-106` | OK |
| `backend/internal/features/catalog.go:102`, `:152` | inserta historial sin enviar `is_promotional` | inserciones a `product_price_history` no incluyen esa columna | OK |
| `backend/internal/features/catalog.go:120-165` | actualiza oferta y agrega historial solo si cambia el precio | `catalog.go:120-165`; condición de cambio en `:146-153` | OK |
| `backend/internal/features/ingestion.go:23-62` | el camino con imagen delega en Product API y sin imagen hace INSERT directo | `ingestion.go:24-45` y `:46-62` | OK |
| `backend/internal/features/ingestion.go:46-61` | inserción directa para producto sin imagen | `ingestion.go:46-61` | OK |
| `backend/internal/features/ingestion.go:77-82` | valida tienda y delega la oferta a Pricing API | `ingestion.go:77-82` | OK |
| `backend/internal/products/handler.go:24-50` | valida `imageUrl` http(s) y crea producto | `products/handler.go:24-50` | OK |
| `backend/database/migrations/005_demo_catalog.sql:249-329` | carga datos de catálogo demo | El rango citado forma parte del INSERT/upsert de datos demo | OK |
| `backend/database/migrations/006_refrigeradores.sql:300-372` | carga productos/ofertas demo | El rango citado forma parte del INSERT/upsert de catálogo demo | OK |
| `backend/database/migrations/008_normalize_prices_to_clp.sql:6-14`, `:26-30`, `:38-42` | normaliza etiqueta CLP mediante funciones y triggers | Las funciones fuerzan `NEW.currency := 'CLP'`; ambos triggers cubren insert/update | OK |

## Tarea 2 — runner de migraciones en k8s y docker compose

### 2.1. Origen y orden de los .sql

El runner real es `k8s/base/migrations/migrate.sh` y toma la carpeta definida por `MIGRATIONS_DIR`, con valor por defecto de `/migrations` (`k8s/base/migrations/migrate.sh:5-6`). El script ordena por basename usando:

```bash
shopt -s nullglob
files=("$MIGRATIONS_DIR"/*.sql)
IFS=$'\n' sorted=($(printf '%s\n' "${files[@]}" | sort))
```

El script no fija `LC_ALL` ni `LANG`; `sort` usa el locale heredado por el proceso. Por eso no puede afirmarse el orden de los pares que difieren por mayúsculas/minúsculas sin conocer el locale efectivo del Job. Los nombres quedan así, con los pares sensibles a locale indicados:

1. `001_schema.sql`
2. `002_views.sql`
3. `003_seed.sql`
4. `004_platform_apis.sql`
5. `005_demo_catalog.sql`
6-7. `006_demo_catalog_services.sql` y `006_refrigeradores.sql` (orden relativo sensible a locale)
8-9. `007_Computacion.sql` y `007_service_cart.sql` (orden relativo sensible a locale)
10-11. `008_Telefonia.sql` y `008_normalize_prices_to_clp.sql` (orden relativo sensible a locale; en orden bytewise C/POSIX, Telefonia va primero)

El runner aplica exactamente el resultado de `sort` del entorno. Como no fija el locale y el Job no establece `LANG`/`LC_ALL`, el orden absoluto de esos pares no se puede deducir con certeza solo de estos archivos.

### 2.2. Tabla de versiones y archivos ya aplicados

El script crea `schema_migrations` y usa una fila por archivo con:

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
  version    TEXT PRIMARY KEY,
  checksum   TEXT NOT NULL,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Luego, cada archivo se identifica por `basename` del SQL y se usa como `version`:

```bash
version="$(basename "$f")"
checksum="$(tr -d '\r' < "$f" | sha256sum | cut -d' ' -f1)"
current="$(psql -tAc "SELECT checksum FROM schema_migrations WHERE version = '${version}'")"
```

Si el archivo ya existe en `schema_migrations`, el script lo salta y solo avisa si cambió el checksum. Si la base ya tiene esquema pero `schema_migrations` está vacío, el runner falla antes de tocar nada.

### 2.3. ¿Transacción por archivo? ¿ON_ERROR_STOP? ¿Detiene al primer error?

Sí: ejecuta cada archivo con `--single-transaction` y `-v ON_ERROR_STOP=1`:

```bash
psql -q -v ON_ERROR_STOP=1 --single-transaction \
  -f "$f" \
  -c "INSERT INTO schema_migrations (version, checksum) VALUES ('${version}', '${checksum}')"
```

Esto significa:
- una transacción por archivo SQL;
- si el archivo falla, la transacción se revierte;
- `ON_ERROR_STOP=1` hace que el comando de psql falle al primer error y detenga la ejecución del Job.

### 2.4. ¿Aplaza también seeds y datos demo?

Sí. El runner aplica todos los `.sql` bajo `/migrations`, incluyendo los que contienen seeds/datos demo, como `003_seed.sql`, `005_demo_catalog.sql`, `006_demo_catalog_services.sql`, `006_refrigeradores.sql`, `007_Computacion.sql`, `008_Telefonia.sql` y `008_normalize_prices_to_clp.sql`.

El runner de Kubernetes no filtra por entorno: aplica los `.sql` del directorio montado cuando se ejecuta el Job, saltando los ya registrados. Docker initdb también monta la carpeta completa, pero la procesa al inicializar un directorio de datos vacío. Esto no demuestra que ambos mecanismos se ejecuten en todos los entornos.

### 2.5. Qué pasaría con `proposed_offer_url_index.sql`

Si se añadiera a la carpeta de migraciones como archivo nuevo, el runner lo ordenaría según su basename y lo ejecutaría después de los `008_*` actuales, porque el prefijo `proposed_...` queda alfabéticamente después de `8` (y no tiene un prefijo numérico). Si la base ya tenía aplicadas las 11 migraciones y el archivo no estaba registrado en `schema_migrations`, el runner lo aplicaría una sola vez al encontrarlo nuevo en la carpeta. Si ya está marcado en `schema_migrations`, lo saltaría.

### 2.6. Cantidad de archivos y filtro

El directorio real tiene 11 archivos SQL:

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
008_Telefonia.sql
008_normalize_prices_to_clp.sql
```

La pareja final es sensible al locale; la lista anterior muestra orden bytewise C/POSIX, no un orden garantizado para cualquier entorno.

El script usa `"$MIGRATIONS_DIR"/*.sql` y no filtra por patrón `^[0-9].*\.sql$` ni por un subconjunto específico. Solo toma todos los `.sql` del directorio y los ordena lexicográficamente por nombre.

### 2.7. Comparación con Docker compose

Hecho real de `docker/compose.yml`:

```yaml
services:
  postgres:
    image: postgres:18
    volumes:
      - postgres_data:/var/lib/postgresql
      - ../backend/database/migrations:/docker-entrypoint-initdb.d:ro
```

Esto significa:
- PostgreSQL usa la imagen `postgres:18`.
- Los SQL se montan en `/docker-entrypoint-initdb.d` en modo lectura (`:ro`).
- El volumen `postgres_data` persiste el almacenamiento del contenedor.

HECHO: imagen, volumen nombrado y montaje de solo lectura. SUPOSICIÓN basada en el comportamiento documentado de la imagen oficial PostgreSQL: initdb procesa los scripts al inicializar un directorio de datos vacío; el YAML no prueba el estado de un volumen existente ni que los scripts se reejecuten en cada arranque.

## README del backend: menciones a 009/010 y verificaciones

En `backend/README.md` se documentan claramente estas menciones:

- `backend/README.md:1117-1124` — documenta que `009_normalize_prices_to_clp.sql` etiqueta los precios existentes como CLP y que `010_seed_all_services.sql` concentra los seeds de servicios.
- `backend/README.md:1120-1124` — muestra el comando manual de ejecución por archivo en un volumen existente.
- `backend/README.md:359` — una verificación de SQL por `docker compose exec ... -f /docker-entrypoint-initdb.d/005_demo_catalog.sql`.

Esto confirma que el README del backend describe un flujo de migraciones manuales por archivo y la versión 009/010 como nombres de migración documentados, pero estos nombres no aparecen en el directorio actual de la rama vista (`backend/database/migrations/`), que termina en `008_*`.

## Conclusión del runner

La respuesta corta a “¿cómo se aplican hoy las migraciones?” es:
- Docker (compose): apoya una inicialización automática sobre un volumen nuevo con `.sql` en `/docker-entrypoint-initdb.d`, no un sistema versionado.
- Kubernetes: usa un Job único con `migrate.sh`, ordena `.sql` por basename, registra `schema_migrations`, calcula checksum, salta los ya aplicados y aplica cada SQL en una transacción con `ON_ERROR_STOP=1`.

Esto es una diferencia estructural importante: Docker es “initdb en un volumen nuevo”; K8s es “runner versionado por esquema”.

## Checklist Día 2

- [x] Citas dudosas verificadas, indicando el orden sensible al locale.
- [x] Runner de migraciones de k8s revisado estáticamente; no ejecutado.
- [x] Preguntas del correo incorporadas a `decisiones_v2.md`.
- [ ] Respuestas del equipo registradas — pendientes: no se recibieron respuestas.
- [!] Consulta a base real — bloqueada: no hay autorización.
- [!] Decisiones confirmadas — bloqueadas: permanecen como propuestas.

## Estado de verificación inicial

La comprobación inicial dio `git status --short` vacío y rama `feature/offer-url-index-design`. El commit observado fue `5aa528f avances de auditoria`, distinto de `2738771` indicado en el contexto anterior. La instrucción de detenerse se condicionaba a un status no vacío; como estaba limpio, la revisión continuó. Los archivos de esta entrega se guardan en `documentos/auditorias/dia2/` conforme a la ubicación que el usuario confirmó después.
