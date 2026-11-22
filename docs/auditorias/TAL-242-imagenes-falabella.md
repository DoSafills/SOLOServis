# TAL-242 — Imágenes de Falabella: esqueletos, no carga diferida

| Campo | Valor |
| --- | --- |
| Fecha | 2026-11-19 |
| Issue | #120 |
| Alcance | `scraper/extract/`, `scraper/spec/schema.py`, `stores/falabella-cl/spec.json`, README |
| Estado previo | 8 de 27 ítems de Falabella llegaban con `product.imageUrl`; los 56 de Ripley, con imagen |

## 1. La hipótesis del issue era incorrecta

El issue suponía carga diferida por atributo: la URL real en `data-src`,
`srcset` o similar, con `src` de respaldo. Se descargó el listado real
(`/falabella-cl/search?Ntt=refrigerador`, 28 tarjetas) y se inspeccionó:

| Comprobación | Resultado |
| --- | --- |
| Tarjetas con `<img>` en `pod-head` | 8 de 28 |
| Tarjetas con `<div class="image-skeleton">` | 20 de 28 |
| Ocurrencias de `data-src` en todo el documento | **0** |
| `srcset` sin `src` | 0 |

Las 20 tarjetas restantes **no tienen `<img>`**: el HTML del servidor sólo
pinta la imagen de las tarjetas del viewport inicial y deja un esqueleto que
hidrata JavaScript. El scraper usa HTTP plano (`AsyncFetcher`, sin navegador),
así que ese JavaScript nunca corre. Ningún cambio de atributo en el spec podía
resolverlo.

También se descartó la página de detalle (`spec.detail`): las URLs de producto
responden **403 de Cloudflare**, con y sin slug.

## 2. Dónde sí están los datos

El mismo HTML publica un `<script type="application/ld+json">` con un
`ItemList` de 56 entradas (52 ids únicos), cada una con:

| Campo | Ejemplo |
| --- | --- |
| `url` | `https://www.falabella.com/falabella-cl/product/15267403/refrigerador-…` |
| `image` | `https://media.falabella.com/falabellaCL/15267403_1/public` |

Las URLs de `image` responden **200** con `image/jpeg` o `image/webp`.

### La clave de emparejamiento no es el `externalId`

| Clave de la tarjeta | Coincide con el JSON-LD |
| --- | --- |
| `id="testId-pod-<n>"` (el `externalId` actual) | 14 de 28 |
| `data-key="<n>"` | **28 de 28** |

En la mitad de las tarjetas el `id` que renderiza Falabella no es el id
canónico del producto. Emparejar por `externalId` habría dado **imágenes
equivocadas** en 14 casos, no imágenes faltantes: habría sido peor que el bug.

## 3. Solución: `item.supplements`

Capacidad nueva y genérica del motor, no un parche por tienda: un campo que la
tarjeta no trae se rellena desde los datos estructurados de la página, una vez
por página y sin sobrescribir nunca lo que el HTML visible sí trae.

```json
"supplements": {
  "externalUrl": {
    "from": "jsonld",
    "matchBy": { "attribute": "data-key" },
    "valuePath": "url"
  },
  "product.imageUrl": {
    "from": "jsonld",
    "matchBy": { "attribute": "data-key" },
    "valuePath": "image"
  }
}
```

Reglas, todas validadas al cargar el spec o cubiertas por test:

- El destino es `externalUrl` o un campo ya declarado en `item.fields`.
- `from` sólo admite `jsonld`.
- La clave de unión son los primeros dígitos de `matchBy` y del `/product/<id>/`
  de la entrada: `data-key="15267403"` e `id="testId-pod-15267403"` encuentran
  lo mismo.
- Nunca sobrescribe: si la tarjeta trae `<img>`, gana la tarjeta.
- `externalUrl` tiene una sola fuente: o `item.externalUrl` o un suplemento,
  no ambos.
- JSON-LD ilegible → advertencia en `run.json`, no falla la corrida.
- Sólo se aceptan cadenas; una ruta ausente es "sin valor", nunca una suposición.

## 4. El `externalUrl` roto, en el mismo cambio

El spec construía `…/product/{externalId}`, es decir
`…/product/testId-pod-15267403`: una URL que no existe. Además, por el desfase
`id`/`data-key`, en 14 tarjetas apuntaba a otro producto. Ahora la URL canónica
sale del `url` del JSON-LD, con cobertura 28 de 28.

El template se eliminó en vez de dejarlo como respaldo: un respaldo que produce
una URL inválida no es un respaldo. Un ítem que no se puede emparejar se
rechaza con `externalUrl: required value not found`, que es la conducta
declarada del motor ("spec o nada").

## 5. Verificación

Motor contra el HTML real descargado (sin red, sin base):

```text
tarjetas=28 items=28 rechazadas=0
items con imageUrl=28/28
warnings: []
  testId-pod-144883806 | https://media.falabella.com/sodimacCL/7762216_001/…/fit=pad
     url: https://www.falabella.com/falabella-cl/product/144883805/refrigerador-…-mdrb241fge50m
```

Suites:

```text
$ ./venv/bin/python -m pytest
167 passed in 1.10s
$ ./venv/bin/ruff check src tests
All checks passed!
$ PYTHONPATH=src ./venv/bin/python -m scraper validate
3 spec(s) valid
```

Pruebas nuevas:

| Archivo | Qué fija |
| --- | --- |
| `tests/test_jsonld.py` | Índice por id, varios scripts, `@graph`, JSON roto como advertencia, scripts escapados, `matchKey` con y sin prefijo, `valueAt` con listas |
| `tests/test_extract.py` | Esqueleto rellenado desde JSON-LD, imagen de la tarjeta con prioridad, rechazo sin JSON-LD, JSON roto no rompe la corrida |
| `tests/test_spec.py` | `externalUrl` con una sola fuente, `from` soportado, `matchBy`/`valuePath` obligatorios y validados, destino declarado, claves desconocidas |

El fixture de Falabella ahora replica el caso real: una tarjeta con `<img>` y
una con esqueleto, más el bloque JSON-LD.

## 6. Límites declarados

- **No se completan los productos ya existentes.** Eso es TAL-243 (ingestion):
  un ítem que coincide con un producto existente sólo actualiza la oferta.
- **El `externalId` sigue siendo `testId-pod-<n>`.** Cambiarlo al id canónico
  duplicaría los 27 productos ya cargados, porque ingestion deduplica por
  `externalId`. Queda como decisión aparte.
- **Depende de que Falabella publique el `ItemList`.** Si lo quita, los ítems se
  rechazan por falta de URL en vez de entrar con enlaces falsos.
- **Sólo JSON-LD.** Microdatos o un JSON propio exigirían otro `from`.
