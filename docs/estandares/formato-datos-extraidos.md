# Formato común de datos extraídos

| Campo | Valor |
| --- | --- |
| Alcance | Crawler / scraper (`crawler/`) |
| Implementación de referencia | `crawler/src/normalized.py` |
| Esquema legible por máquina | `crawler/schema/normalized-item.schema.json` |
| Contrato de destino | Ingestion API, TAL-77 (`backend/README.md` §12.7) |
| Versión del formato | `1.0` |
| Estado | Normativo para todo parser nuevo o modificado |

Este documento define el registro canónico que el scraper produce para un
producto, **independiente de la tienda**. Cualquier tienda nueva se incorpora
como configuración (`parser_config`), no como código ni como formato paralelo.

---

## 1. Problema y decisión

Cada tienda publica la misma información con otra forma: Falabella deja el
identificador en el atributo `id` de la tarjeta y el enlace vacío; Ripley usa un
`href` relativo y muestra tres precios; Mercado Libre anida el precio en
`andes-money-amount`. Un extractor por tienda produciría un formato por tienda,
y el consumidor tendría que conocer las tiendas.

**Decisión:** separar tres cosas que suelen mezclarse.

| Concepto | Dónde vive | Por qué |
| --- | --- | --- |
| *De dónde salió* (tienda, página, ejecución) | `source` + `provenance` | Es un hecho de la ejecución, no del producto. |
| *Qué producto es* | `product` | Claves naturales que sobreviven al cambio de tienda. |
| *Qué se ofrece y a cuánto* | `offer` | Depende de la tienda y del momento. |

El registro resultante es un **sobre** (`NormalizedItem`) con un payload que
coincide exactamente con el contrato de Ingestion API. El sobre agrega versión y
trazabilidad; el payload se envía tal cual.

### Consecuencias normativas

1. **La tienda nunca está dentro de `product` ni de `offer`.** Se asigna una vez
   por ejecución (`POST /ingestion/runs` con `storeId`). Un registro puede
   moverse entre tiendas sin editarse.
2. **No se emiten claves foráneas.** El ítem lleva `sku`, `brand` y
   `categorySlug`; los `id` de `product`, `store`, `brand` y `category` los
   resuelve el servidor.
3. **Los fallos se acumulan, no se lanzan.** Un atributo ilegible no cuesta la
   tarjeta: se omite, se anota en `provenance.errors` y la validación decide.
4. **Un dato dudoso se reporta, no se rellena.** Si no hay `categorySlug`
   resoluble o la tarjeta no tiene enlace, el ítem queda marcado; nunca se
   inventa una categoría ni una URL de producto.

---

## 2. El sobre

```json
{
  "schemaVersion": "1.0",
  "entityType": "product",
  "externalId": "testId-pod-14873869",
  "externalUrl": "https://www.falabella.com/falabella-cl/search?Ntt=refrigerador",
  "source": {
    "name": "Falabella Chile",
    "baseUrl": "https://www.falabella.com/falabella-cl"
  },
  "product": { "...": "§3" },
  "offer": { "...": "§4" },
  "provenance": { "...": "§5" }
}
```

| Campo | Tipo | Obligatorio | Regla |
| --- | --- | --- | --- |
| `schemaVersion` | string | sí | Constante `"1.0"` (§8). |
| `entityType` | string | sí | Constante `"product"`. `"service"` se rechaza hasta que exista contrato de ingesta. |
| `externalId` | string | no | ≤ 255. Identificador propio de la tienda, si lo publica. Se guarda en `scraped_data.external_id`. |
| `externalUrl` | string | sí | URL `http(s)` absoluta, ≤ 2048. Es la página del producto o, si la tarjeta no la expone, la del listado (con nota en `provenance.errors`). |
| `source` | objeto | no | §2.1. |
| `product` | objeto | sí | §3. |
| `offer` | objeto | sí | §4. |
| `provenance` | objeto | sí | §5. |

`to_dict()` omite los campos opcionales vacíos; no emite `null`. Un campo
presente pero vacío y un campo ausente son equivalentes para el consumidor.

### 2.1 `source`

| Campo | Tipo | Obligatorio | Regla |
| --- | --- | --- | --- |
| `name` | string | sí | ≤ 150. Debe coincidir con `data_source.name` de la ejecución. |
| `baseUrl` | string | sí | URL `http(s)` absoluta. Origen de las URLs relativas. |
| `storeId` | integer | no | ≥ 1. Tienda de la ejecución; se documenta aquí, no en el ítem. |

`source` es informativo para auditoría: **no** forma parte del body de ingesta.

---

## 3. `product`

| Campo | Tipo | Obligatorio | Límite | Origen en el spec |
| --- | --- | --- | --- | --- |
| `name` | string | sí | 255 | `product.fields.name` |
| `sku` | string | no | 100 | `product.fields.sku` |
| `model` | string | no | 150 | `product.fields.model` |
| `brand` | string | no | 150 | `product.references.brand` |
| `categorySlug` | string | sí¹ | `^[a-z0-9-]{1,64}$` | `category_slug` |
| `description` | string | no | 5000 | `product.fields.description` |
| `imageUrl` | string | no | URL `http(s)` absoluta | `images.fields.image_url` |
| `attributes` | objeto | no | claves `^[a-z][a-z0-9_]{0,63}$` | `attributes` |

¹ Obligatorio para el scraper: sin categoría el producto nuevo no se puede
crear. El backend solo lo exige cuando el producto no existe, pero el scraper no
puede saberlo, así que lo trata como obligatorio.

Reglas:

* **`brand` va en `references`, no en `fields`.** Es una clave natural que la
  ingesta resuelve contra `brand.name`; el extractor la expone en
  `row.references` y el bundle la mueve a `product.brand`.
* **`sku` es el de la tienda.** La ingesta busca primero por `sku` y, si no
  coincide, por marca + modelo; por eso un SKU de tienda no duplica el catálogo.
* **`attributes` se valida contra la categoría en el backend.** Una clave que no
  esté definida para esa categoría produce `unknown_field`; el scraper debe
  emitir solo atributos que existan en el catálogo (§6).

---

## 4. `offer`

| Campo | Tipo | Obligatorio | Por defecto | Regla |
| --- | --- | --- | --- | --- |
| `price` | string | sí | — | `^\d{1,10}(\.\d{1,2})?$` |
| `listPrice` | string | no | — | mismo patrón que `price` |
| `currency` | string | sí | `CLP` | `^[A-Z]{3}$` |
| `shippingCost` | string | sí | `"0.00"` | mismo patrón de monto |
| `shippingFree` | boolean | sí | `false` | — |
| `available` | boolean | sí | `true` | — |
| `stock` | integer | no | — | ≥ 0 |
| `condition` | string | sí | `new` | `new` \| `used` \| `refurbished` |
| `productUrl` | string | no | — | URL `http(s)` absoluta, ≤ 2048 |

² El backend no compara `listPrice` con `price`. Si el "precio de lista"
scrapeado es menor que el vigente, la tienda está mostrando otra cosa (cuota,
precio con tarjeta) y conviene no declararlo como `list_price` en el spec.

### Los montos viajan como texto

`price`, `listPrice` y `shippingCost` se serializan como **string decimal**
(`"1349990.00"`), nunca como número JSON. Motivo: `NUMERIC(12,2)` es exacto y un
`float64` de JavaScript o Python pierde centavos antes de llegar a PostgreSQL.
Go acepta un número entrecomillado al deserializar en `json.Number`, así que el
contrato de Ingestion se cumple igual.

El scraper normaliza siempre a **dos decimales** (`_money_text` cuantiza). Un
precio `"599990"` sale como `"599990.00"`.

---

## 5. `provenance`

| Campo | Tipo | Obligatorio | Regla |
| --- | --- | --- | --- |
| `scrapedAt` | string | sí | ISO 8601 UTC con `Z` (`2026-11-02T12:00:00Z`). |
| `pageUrl` | string | no | URL de la página de la que se extrajo el ítem. |
| `runId` | integer | no | `scrape_run.id` cuando se conoce. |
| `configId` | integer | no | `scraper_config.id` cuando se conoce. |
| `references` | objeto | no | Claves naturales vistas en la página (`{"brand": "SAMSUNG"}`), antes de que la ingesta resuelva. |
| `errors` | array\<string\> | no | Fallos de extracción y notas, en el orden en que ocurrieron. |

`errors` es diagnóstico, no contrato: su texto puede cambiar. Lo estable es
`validate()`, que devuelve errores tipados con el mismo vocabulario que la
Ingestion API (`field`, `code`, `message`).

---

## 6. Configuración: `BundleSpec`

Un bundle declara **una raíz** y las partes de la tarjeta. Las claves
desconocidas se rechazan: un typo no puede descartar media tarjeta en silencio.

```json
{
  "entity_type": "product",
  "root": "a.product-link",
  "source": { "name": "Ripley Chile", "base_url": "https://simple.ripley.cl" },
  "product": {
    "fields": { "name": ".product-item--name" },
    "references": { "brand": ".product-item--brand" }
  },
  "offer": {
    "fields": {
      "price": ".product-price-price",
      "list_price": ".product-price-old-price"
    }
  },
  "images": {
    "fields": {
      "image_url": { "selector": ".product-image-img", "mode": "attr", "attribute": "src" }
    }
  },
  "category_slug": { "value": "refrigeradores" }
}
```

| Clave | Obligatoria | Significado |
| --- | --- | --- |
| `entity_type` | no | Solo `product` (§8). |
| `root` | no | CSS de la tarjeta en un listado. Sin `root`, el documento es un ítem (página de detalle). |
| `product` | sí | `{fields, references}`; columnas de la tabla `product`. |
| `offer` | sí | `{fields}`; columnas de `product_offer`. |
| `images` | no | `{fields: {image_url, alt_text?, sort_order?}}`; se usa el primer match de `image_url`. |
| `external_id` | no | CSS string, u objeto `{selector?, mode?, attribute?}`. |
| `category_slug` | no | String literal, u objeto (§7). |
| `attributes` | no | Mapa `clave → especificación` (§7.2). |
| `source` | no | `{name, base_url, store_id?}`. |

`fields`, `references`, `mode`, `attribute`, `transform` y `default` se
comportan igual que en `extractor.py`; los límites de columna están transcritos
del esquema de producción, así que un `NUMERIC(12,2)` no acepta texto.

### 6.1 Identificador externo

`external_id` acepta dos formas:

```json
"external_id": ".product-item--code"
```

```json
"external_id": { "attribute": "id" }
```

La segunda lee un atributo del **propio nodo raíz**, sin selector. Es necesaria
cuando la tienda pone el identificador en la tarjeta: Falabella usa
`id="testId-pod-14873869"` y ningún selector descendiente puede verlo.

### 6.2 `category_slug`

```json
"category_slug": "refrigeradores"
```

```json
"category_slug": {
  "selector": ".breadcrumb",
  "alias": {
    "Electrodomésticos > Refrigeradores": "refrigeradores",
    "Hogar / Refrigeración": "refrigeradores"
  }
}
```

Resolución, en orden:

1. `value`, si está declarado. Se valida contra `^[a-z0-9-]{1,64}$`.
2. Texto del `selector`, buscado en `alias` (comparación sin mayúsculas, acentos
   ni espacios extra). Si no hay match exacto, se prueba con el último segmento
   del breadcrumb (`>`, `|`, `/`).
3. Sin alias: el texto se pliega con `to_slug` (NFKD, sin acentos, minúsculas,
   todo lo no alfanumérico a `-`, máximo 64).
4. Si nada de lo anterior produce un slug válido, `categorySlug` queda ausente y
   se agrega un error. **No se inventa una categoría.**

Un slug literal debe existir en el catálogo (`categories.slug`). El scraper no
crea categorías.

### 6.3 `attributes`

```json
"attributes": {
  "capacidad_litros":   { "selector": ".spec-capacity", "type": "integer" },
  "eficiencia_energetica": { "selector": ".energy", "type": "enum", "choices": ["A", "B", "C"] },
  "no_frost":           { "attribute": "data-nofrost", "type": "boolean" },
  "volumen_pies":       { "selector": ".cubic-feet", "type": "decimal" },
  "color":              { "selector": ".color", "type": "string", "max_length": 60 }
}
```

| `type` | Valor emitido | Regla |
| --- | --- | --- |
| `string` | string | ≤ 255 por defecto; `max_length` lo ajusta. |
| `integer` | number entero | Rechaza decimales (`12.5` → error). |
| `decimal` | number | Se emite como `float` JSON. |
| `boolean` | boolean | `true`/`false`, sin cadenas. |
| `enum` | string | Debe estar en `choices`; si no, se omite y se reporta. |

* El `type` es **obligatorio en la práctica**: sin él se asume `string`. El
  backend no convierte tipos, así que un atributo `integer` emitido como `"350"`
  falla con `invalid_type`.
* `enum` exige `choices`; se rechaza en la construcción del spec si falta.
* La clave debe cumplir `^[a-z][a-z0-9_]{0,63}$`.
* **El atributo debe existir en la categoría.** La ingesta valida contra la
  definición de `category_attributes`; una clave desconocida produce
  `unknown_field` aunque el scraper la haya leído bien.

---

## 7. Validación

`NormalizedItem.validate()` aplica las reglas de Ingestion localmente, antes de
enviar nada. Devuelve una lista de `FieldError`:

```python
[{"field": "product.name", "code": "required",
  "message": "product.name es obligatorio"}]
```

| `code` | Significado |
| --- | --- |
| `required` | Falta un campo obligatorio. |
| `max_length` | Supera el largo de la columna. |
| `format` | URL, monto, moneda o slug con forma inválida. |
| `one_of` | Valor fuera del conjunto permitido (`condition`). |
| `min` | `stock` negativo. |

`is_valid` es el atajo booleano. Los errores de extracción (selector que no
matchea, atributo ilegible) van a `provenance.errors` como texto; los de
validación usan el vocabulario del backend.

### Qué NO hace la validación local

* No consulta el catálogo: no sabe si `categorySlug` existe ni si los atributos
  están declarados para esa categoría. Eso lo decide la ingesta.
* No deduplica ni compara contra la base.
* No garantiza que la URL responda.

Es un filtro barato para no enviar basura, no un reemplazo de la validación del
servidor.

---

## 8. Versionado

`schemaVersion` es la versión del **sobre**. Cambia cuando cambia la forma, no
cuando cambia una tienda.

| Cambio | Efecto |
| --- | --- |
| Agregar un campo opcional al sobre | `1.x` (compatible hacia atrás). |
| Agregar un campo obligatorio, quitar o renombrar uno | `2.0`; consumidores deben actualizarse. |
| Cambiar un límite heredado del backend | No cambia la versión: se sincroniza con el contrato de ingesta. |
| Cambiar selectores de una tienda | No cambia la versión: es configuración. |

`entityType` está fijado a `"product"`. `"service"` está reservado y se rechaza
en `BundleSpec.from_parser_config` hasta que exista un contrato de ingesta para
servicios; emitirlo hoy produciría registros que el backend no sabe procesar.

---

## 9. Relación con Ingestion API

`to_ingestion()` proyecta el sobre al body exacto de
`POST /ingestion/runs/{id}/items`:

```json
{
  "externalId": "testId-pod-14873869",
  "product": { "name": "…", "brand": "SAMSUNG", "categorySlug": "refrigeradores", "…": "…" },
  "offer": { "price": "1349990.00", "currency": "CLP", "condition": "new", "…": "…" }
}
```

* `externalId` se omite si no existe.
* `schemaVersion`, `entityType`, `source` y `provenance` **no se envían**:
  Ingestion rechaza campos desconocidos y esos datos son de ejecución o
  auditoría.
* El body se envía en lotes de 1 a 100 ítems; un ítem inválido no detiene el
  lote.

La correspondencia es 1:1 con `internal/ingestion/model.go`. Si ese contrato
cambia, este documento y `normalized.py` cambian en el mismo commit.

---

## 10. Ejemplos y pruebas

* `crawler/examples/falabella-cl.json` y `crawler/examples/ripley-cl.json`:
  configuraciones verificadas contra el HTML real (2 nov 2026).
* `crawler/examples/mercadolibre-cl.json`: markup documentado, **no verificado**
  (el sitio responde con muro anti-bot).
* `crawler/tests/test_normalized.py`: contrato, validación, atributos,
  categoría, exportación y tarjetas reales recortadas.
* `crawler/schema/normalized-item.schema.json`: JSON Schema 2020-12 del sobre.

```bash
cd crawler
./venv/bin/python -m pytest tests/test_normalized.py -q
./venv/bin/python -m ruff check src tests
```

---

## 11. Fuera de alcance

Este documento **no** define:

* El envío HTTP a Ingestion (cliente, reintentos, lotes): TAL-79/TAL-90.
* La persistencia local del registro: hoy `export_jsonl` escribe el sobre y la
  ingesta lo archiva en `scraped_data.raw_data`.
* El catálogo de categorías y atributos: TAL-71/TAL-72.
* La resolución de marca, SKU y producto: la hace el servidor.
* Tiendas que requieren navegador (Paris, Lider): el contrato es el mismo, pero
  el fetch cambia.
