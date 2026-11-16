# TAL-222 — Sistema de consultas predefinidas

| Campo | Valor |
| --- | --- |
| Fecha | 2026-11-16 |
| Alcance | `backend/internal/search`, `internal/http/router.go`, `backend/README.md` |
| Estado previo | Sólo existía `/search/popular-products` (12 productos por popularidad) como búsqueda "de sistema" |

## 1. Decisión

Dos endpoints nuevos sobre el catálogo de presets:

| Método | Ruta | Respuesta |
| --- | --- | --- |
| GET | `/search/presets` | `{"presets":[…]}` con slug, título, descripción y filtros |
| GET | `/search/presets/{slug}` | Igual que `/search/products` (arreglo) + `X-Total-Count` |

Un slug desconocido responde **404** `problem+json` con `code: preset_not_found`.
`limit` y `offset` se aceptan; el resto de los filtros los define el preset.

No se extendió `/search/products` con `?preset=`: esa ruta ya valida `q`,
`category`, `brand`, `store`, `available`, `sort`, atributos dinámicos y
paginación. Añadir un modo condicional habría vuelto la validación dependiente
de otro parámetro y habría dejado al cliente sin forma de listar los presets.

## 2. Por qué el catálogo vive en código

| Criterio | Código (`presets.go`) | Tabla en PostgreSQL |
| --- | --- | --- |
| Cambiar un preset | PR con diff revisable | CRUD, permisos y migración |
| Rollback | revierte con el código | la fila queda |
| Validación | test de CI | nadie la garantiza |
| Coste inicial | cero migraciones | tabla + endpoints de escritura |
| Edición sin deploy | no | sí, pero nadie lo pidió |

Si en el futuro se necesitan presets editables, se siembran desde este catálogo
a una tabla **sin cambiar el contrato de la API**.

No se usó `search_query`: esa tabla es el **historial** de búsquedas
(`raw_query`, `user_id`, filtros) y confundirla con configuración sería un error
de modelo.

## 3. Portabilidad

Los presets sólo declaran:

- `q` (texto, hasta 5 palabras)
- `category` (slug, validado con `categories.ValidSlug`)
- `minPrice` / `maxPrice` (formato `NUMERIC(12,2)`)
- `available`
- `sort` (`name`, `price_asc`, `price_desc`, `brand_asc`, `brand_desc`, `newest`)
- `limit`

**Nunca `storeId` ni `brandId`.** `AGENTS.md` §8 lo advierte: los IDs de tienda
cambian según el entorno. Un preset con IDs fijos funcionaría en local y daría
resultados vacíos en el clúster.

## 4. Catálogo

| Slug | Título | Filtros | Aliases (para TAL-226) |
| --- | --- | --- | --- |
| `refrigeradores` | Refrigeradores | `category=refrigeradores`, `price_asc` | refrigerador, nevera, heladera, frigorifico, frio |
| `refrigeradores-economicos` | Refrigeradores económicos | `category`, `maxPrice=400000` | refrigerador barato, nevera economica |
| `notebooks` | Notebooks | `category=notebooks`, `price_asc` | notebook, laptop, portatil |
| `notebooks-economicos` | Notebooks económicos | `category`, `maxPrice=550000` | notebook barato, laptop economica |
| `televisores` | Televisores | `category=televisores`, `price_asc` | televisor, tv, pantalla, smart tv |
| `precios-bajos` | Precios bajos | `maxPrice=100000`, 12 | barato, economico, oferta |
| `novedades` | Novedades | `newest`, 12 | novedad, nuevo, reciente |

Las tres categorías son las sembradas en `013_categories_seed.sql`. Un test
falla si un preset apunta a otra que no exista.

## 5. Validación

`validatePresets` revisa el catálogo completo y un test lo ejecuta:

| Regla | Ejemplo que falla |
| --- | --- |
| Slug con formato `^[a-z0-9-]{1,64}$` y único | `Notebooks`, `x` repetido |
| Título y descripción no vacíos | `" "` |
| Categoría con formato válido | `Refrigeradores` |
| `sort` dentro de `orderBySQL` | `caro` |
| Precios con `moneyPattern` y `min ≤ max` | `100.000`, min 900 > max 100 |
| `limit` entre 0 y 100 | 101 |
| `q` hasta 5 palabras | 6 palabras |

## 6. Pruebas

`backend/internal/search/presets_test.go` (9 funciones, 10 subcasos):

| Test | Qué fija |
| --- | --- |
| `TestPresets_CatalogoValido` | El catálogo completo pasa la validación y tiene ≥6 presets |
| `TestPresets_CategoriasSembradas` | Cada categoría usada existe en las semillas |
| `TestPresets_Respuesta` | JSON completo y orden editorial (primero `refrigeradores`) |
| `TestPresetProducts_EjecutaElPreset` | `category`, `maxPrice`, `sort` y `limit` llegan al repositorio; `X-Total-Count` |
| `TestPresetProducts_UsaSuPropioLimite` | Sin `limit` del cliente, se usa el del preset |
| `TestPresetProducts_PaginacionInvalida` | `?limit=0` → 400 `limit/min` sin tocar la base |
| `TestPresetProducts_SlugDesconocido` | 404 `preset_not_found` sin tocar la base |
| `TestPresetProducts_ErrorInternoNoFiltraDetalles` | 500 genérico, detalle sólo al log |
| `TestValidatePresets_RechazaDefinicionesInvalidas` | Las nueve reglas de la tabla anterior |

## 7. Verificación

```text
$ go build ./... && go vet ./... && go test ./...
ok  	github.com/DoSafills/SOLOServis/backend/internal/search
```

Los handlers se probaron con `fakeSearcher`, sin base de datos.

## 8. Límites declarados

- **Sólo productos.** `SearchProducts` no cubre servicios; un preset de
  servicios exigiría otro repositorio.
- **Sin filtros por atributo.** El catálogo no declara `attr.*`; añadirlos exige
  definir la categoría y validarlos contra sus definiciones.
- **Sin frontend.** Las pantallas son otra tarea.
- **`novedades` usa `p.created_at`**, no la fecha de la oferta: es "lo último que
  ingresó al catálogo", y así se describe.

## 9. Siguiente

TAL-226 reutiliza este catálogo con `GET /search/alternatives?q=` y los
`Aliases`, que ya viajan en la estructura pero no se exponen por la API.
