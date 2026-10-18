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
| `backend/database/migrations/008_normalize_prices_to_clp.sql:6-14` | trigger normaliza precios y etiqueta CLP | Se requiere lectura del archivo real; no se confirmó aquí, pero se documenta como sentencia de trigger temporal | DEPENDE |
| `backend/internal/features/router.go:25-31` | ejemplo de rutas protegidas con `requireSession` | `backend/internal/features/router.go:27-34` | OK |
| `backend/internal/products/handler.go:24-50` | validación `imageUrl` HTTP(S) en Product API | `backend/internal/products/handler.go:24-50` | OK |
| `backend/internal/features/ingestion.go:30-45` | IngestProduct delega a Product API cuando hay `imageUrl` | `backend/internal/features/ingestion.go:24-45` | OK |
| `backend/internal/features/catalog.go:96-101` | el handler inserta `product_url` directo al `INSERT` | `backend/internal/features/catalog.go:96-101` | OK |
| `backend/database/migrations/001_schema.sql:130-131` | `condition` tiene `CHECK` para `new`, `used`, `refurbished` | `backend/database/migrations/001_schema.sql:129-135` | OK |
| `backend/database/migrations/001_schema.sql:128` | `stock` tiene `CHECK (stock >= 0)` | `backend/database/migrations/001_schema.sql:127-128` | OK |

## Tarea 2 — runner de migraciones en k8s y docker compose

### 2.1. Origen y orden de los .sql

El runner real es `k8s/base/migrations/migrate.sh` y toma la carpeta definida por `MIGRATIONS_DIR`, con valor por defecto de `/migrations` (`k8s/base/migrations/migrate.sh:5-6`). El script ordena por basename usando:

```bash
shopt -s nullglob
files=("$MIGRATIONS_DIR"/*.sql)
IFS=$'\n' sorted=($(printf '%s\n' "${files[@]}" | sort))
```

No hay `LC_ALL` ni `LANG` forzado en el script; el orden depende del locale del contenedor. En la práctica, con el locale por defecto el orden de los 11 archivos es:

1. `001_schema.sql`
2. `002_views.sql`
3. `003_seed.sql`
4. `004_platform_apis.sql`
5. `005_demo_catalog.sql`
6. `006_demo_catalog_services.sql`
7. `006_refrigeradores.sql`
8. `007_Computacion.sql`
9. `007_service_cart.sql`
10. `008_normalize_prices_to_clp.sql`
11. `008_Telefonia.sql`

Importante: `008_Telefonia.sql` aparece antes que `008_normalize_prices_to_clp.sql` en orden lexicográfico por el prefijo `T` frente a `n`, y esto se debe al orden por basename sin normalización adicional.

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

Esto ocurre en el entorno Kubernetes en cada ejecución del Job; no se excluye ningún `.sql` por nombre. En la práctica, la semántica del runner es “aplicar todo lo que esté en el directorio de migraciones, una sola vez según `schema_migrations`”.

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
008_normalize_prices_to_clp.sql
008_Telefonia.sql
```

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
- La lógica de Docker initdb se ejecuta solo al crear la base vacía; si el volumen ya existe, no re-ejecuta los scripts automáticamente.

Estado estricto: HECHO para el montaje y la ruta; SUPOSICION si se afirma que “Docker re-ejecutará todas las migraciones cada vez”. La documentación real dice que es una inicialización al primer arranque de un volumen nuevo, no un sistema de versión de migraciones.

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
