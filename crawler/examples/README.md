# Ejemplos de `parser_config` para el formato común

Cada archivo es un `parser_config` completo y válido para `normalized.BundleSpec`
(ver `src/normalized.py`). Se construyen sin red en
`tests/test_normalized.py::TestRealCards::test_examples_build_without_errors`;
los selectores de Falabella y Ripley se verificaron contra el HTML servido por
las tiendas el **2 de noviembre de 2026**.

| Archivo | Tienda | Verificado en vivo | Estado |
|---|---|---|---|
| `falabella-cl.json` | Falabella Chile | sí | funciona |
| `ripley-cl.json` | Ripley Chile | sí | funciona |
| `mercadolibre-cl.json` | Mercado Libre Chile | no | basado en el markup documentado |

---

## Falabella Chile

Página inspeccionada: `https://www.falabella.com/falabella-cl/search?Ntt=refrigerador`

28 tarjetas, servidas en el HTML inicial (SSR). La tarjeta es el propio `<a>`:

```html
<a class="jsx-… pod pod-4_GRID pod-link"
   href="" id="testId-pod-14873869" data-pod="catalyst-pod">
  <div class="pod-head">
    <picture><img src="https://media.falabella.com/falabellaCL/14873869_1/…" alt="SAMSUNG - …"></picture>
  </div>
  <div class="pod-details">
    <b class="title1 secondary …">SAMSUNG</b>
    <b class="copy2 primary …">Refrigerador Side by Side de 608 Litros Family Hub</b>
  </div>
  <div class="pod-summary">
    <div class="prices">
      <ol class="pod-prices">
        <li class="prices-0"><span class="copy10 primary medium">$  1.349.990</span> …</li>
        <li class="prices-1"><span class="copy3 primary medium crossed">$  2.349.990</span></li>
      </ol>
    </div>
  </div>
</a>
```

Observaciones que condicionan el spec:

* **`href` llega vacío.** El enlace real lo inyecta JavaScript. El contrato
  cae a la URL del listado y deja la nota `externalUrl: the card has no href…`
  en `provenance.errors`; no se pierde la tarjeta.
* **El identificador está en la tarjeta, no en un descendiente**:
  `id="testId-pod-14873869"`. Por eso `external_id` se declara solo con
  `{"attribute": "id"}` (selector vacío = lee el propio nodo).
* **Los precios traen separador de miles repetido** (`1.349.990`). El
  transform `price` lo resuelve: todos los grupos de tres dígitos son miles.
* El precio "de lista" es `li.prices-1`; el vigente, `li.prices-0`.
* **La página repite algunas tarjetas.** En el listado inspeccionado, 28
  tarjetas corresponden a 25 `externalId` distintos (tres pods aparecen dos
  veces). El bundle no deduplica: emite lo que ve. El crawler sí deduplica por
  `externalId` (fallback `externalUrl`) antes de enviar, así que la repetición
  no llega a Ingestion.
* Las clases `jsx-…` son hashes de Emotion y cambian con cada despliegue; el
  spec usa solo las clases semánticas (`pod-details`, `copy2 primary`, …).

## Ripley Chile

Página inspeccionada: `https://simple.ripley.cl/search/refrigerador`

57 tarjetas, SSR. La tarjeta es `a.product-link` con `href` relativo:

```html
<a class="product-link" href="/refrigerador-french-door-420l…?color_80=gris&…">
  <div class="product-image-wrapper">
    <img src="…placeholder.webp">
    <img class="product-image-img" src="https://rimage.ripley.cl/home.ripley/Attachment/MKP/5799/MPM10003483822/full_image-1">
  </div>
  <span class="product-item--brand">ELECTROLUX</span>
  <p class="product-item--name">REFRIGERADOR FRENCH DOOR 420L NO FROST INVERTER DQ79SU INOX</p>
  <div class="product-item--price">
    <div class="product-price-old-price-container"><span class="product-price-old-price">$799.990</span></div>
    <div class="product-price-price-container"><span class="product-price-price">$599.990</span></div>
    <div class="product-price-ripley-price-container"><span class="product-price-ripley-price">$589.990</span></div>
  </div>
</a>
```

Observaciones que condicionan el spec:

* **Hay tres precios.** `product-price-old-price` (lista),
  `product-price-price` (vigente) y `product-price-ripley-price` (precio con
  tarjeta Ripley). El ejemplo usa el vigente como `price` y el antiguo como
  `list_price`; el precio con tarjeta se ignora a propósito: depende del medio
  de pago, no del producto.
* **La imagen viene precedida de un placeholder** dentro de un carrusel. El
  selector `.product-image-img` apunta a la imagen real; un `img` genérico
  tomaría el placeholder.
* `href` es relativo y con query de tracking. `externalUrl` se absolutiza
  contra la URL de la página, sin recortar la query (es la URL que la tienda
  usa para el mismo producto).

## Mercado Libre Chile

Página inspeccionada: `https://www.mercadolibre.cl/` y
`https://listado.mercadolibre.cl/refrigeradores`.

**No se pudo verificar en vivo.** El sitio redirige cualquier listado a
`/gz/account-verification` desde esta IP (muro anti-bot), y la portada solo
sirve tarjetas de *categorías* (`div.andes-card.card-grid-item`), no de
productos. El ejemplo se basa en el markup público de las tarjetas
`poly-card` (`li.ui-search-layout__item` → `a.poly-component__title`,
`.poly-price__current .andes-money-amount`, `img.poly-component__picture`).

Antes de usarlo en producción hay que ejecutarlo contra una sesión que supere
el muro (proxy residencial o `StealthyFetcher`) y ajustar los selectores a lo
que realmente llegue. Está marcado como no verificado a propósito.

## Disponibilidad (TAL-86)

El contrato acepta `offer.available` (booleano) y `offer.stock` (entero), pero
ninguno de los ejemplos los declara todavía: la señal no está verificada.

* **Ripley**: el JSON-LD de la página trae `"availability":
  "https://schema.org/InStock"` en las 57 tarjetas del listado inspeccionado.
  Es la fuente más fiable, pero no se observó ninguna tarjeta `OutOfStock`, así
  que el selector y su transformación quedan pendientes de comprobar.
* **Falabella**: la hoja de estilos define `.prices.isOutOfStock`, pero ninguna
  de las 28 tarjetas del listado la usa y no hay datos schema.org de
  disponibilidad. El listado inspeccionado era todo disponible; no se puede
  afirmar que la clase aparezca cuando el producto se agota.
* **Mercado Libre**: sin verificar por el muro anti-bot.

Mientras no exista una tarjeta agotada real, `available` conserva su default
`true`. No debe interpretarse como "hay stock": es "no se observó lo
contrario". El seguimiento vive en TAL-86; cuando se capture un listado con
productos agotados, se agrega el selector a `offer.fields`, una tarjeta de
prueba en `tests/test_normalized.py` y se actualiza esta nota.

## Tiendas descartadas para los ejemplos

| Tienda | Qué pasó |
|---|---|
| Lider (`lider.cl`) | Redirige a `/blocked` ("Robot or human?"). Muro anti-bot. |
| Paris (`paris.cl`) | El HTML inicial trae 4,6 MB, pero las tarjetas de producto se renderizan en cliente; `[data-testid*="product"]` solo matchea el botón de búsqueda. Requiere fetch con navegador. |
| PC Factory (`pcfactory.cl`) | Responde 200, pero el listado no expone tarjetas con selectores estables en el HTML inicial. |

Ninguna de estas tres está en el alcance del contrato; quedan documentadas para
que la próxima persona no repita la exploración.

## Cómo probar un ejemplo

```python
import asyncio, json, sys
sys.path.insert(0, "src")

from scraper import Scraper
from normalized import BundleExtractor, BundleSpec, export_jsonl

async def main():
    spec = BundleSpec.from_parser_config(
        json.load(open("examples/ripley-cl.json")))
    outcome = await Scraper().fetch(
        "https://simple.ripley.cl/search/refrigerador")
    items = BundleExtractor(spec).extract_all(
        outcome.document, outcome.final_url)
    export_jsonl(items, "out/ripley.jsonl")
    for item in items:
        if not item.is_valid:
            print(item.external_url, [e.to_dict() for e in item.validate()])

asyncio.run(main())
```

Los ejemplos son datos, no código: agregar una tienda es agregar un JSON.
