# Plantilla del contexto

Este archivo describe cómo se serializa la comparación antes de enviarla al
modelo. El backend arma el JSON desde PostgreSQL; el frontend nunca lo envía.

## Regla de oro

El contexto contiene **solo** los productos comparados (2 a 5, misma categoría).
Se validan contra `context/allowed-fields.json` y
`context/comparison-context.schema.json` antes de llamar al proveedor. Si un
campo no está en la allowlist, se descarta.

## Orden estable

1. `category.name`
2. `products[]` en el orden en que el usuario los comparó
3. `attributes[]` en `display_order` de la categoría
4. `specifications[]` en `display_order` de la categoría

El mismo orden se mantiene en `values[]` de cada fila: el índice `i` de un valor
corresponde a `products[i]`. Un producto sin ese dato lleva `null`.

## Forma

```json
{
  "category": { "name": "Refrigeradores" },
  "products": [
    {
      "name": "Refrigerador Samsung No Frost",
      "brand": "Samsung",
      "model": "RT38K",
      "bestPrice": "549990",
      "bestStore": "Falabella",
      "offerCount": 3,
      "lastUpdated": "2026-11-20T23:01:31Z"
    }
  ],
  "attributes": [
    {
      "label": "Capacidad",
      "unit": "L",
      "values": [380, 550, null],
      "differs": true
    }
  ],
  "specifications": [
    {
      "name": "Eficiencia energética",
      "unit": null,
      "values": ["A++", "A+", null],
      "differs": true
    }
  ]
}
```

## Reglas de serialización

- `bestPrice` es el menor precio **total** (precio + envío) en pesos chilenos,
  como cadena sin separadores ni símbolo. Si no hay ofertas disponibles, `null`.
- `offerCount` cuenta solo ofertas disponibles de tiendas activas.
- `attributes[].values` va tipado según el atributo: número para `integer` y
  `decimal`, booleano para `boolean`, cadena para `string` y `enum`. El tipo, la
  clave del atributo y el UUID del producto no se envían: el modelo no los usa.
- Una especificación que repite un atributo (mismo nombre que su etiqueta o
  clave, y el atributo tiene el dato en los mismos productos) no se envía.
- `differs` se calcula comparando todos los valores, incluidos los `null`.
- No se incluye `imageUrl`, descripciones largas ni URLs de tienda: no aportan a
  la decisión y consumen tokens.
- Si una fila no tiene ningún valor, no se envía.
- El JSON va compacto (sin indentación) dentro de `<productos>…</productos>`.
- El JSON no supera `context.maxBytes` de `config/limits.json` (8192). Si se
  pasa, se quitan primero las filas con `differs` en false y después las
  últimas filas, especificaciones antes que atributos; productos y categoría no
  se tocan. El backend lo registra como `assistant_context_trimmed`.
- La pregunta va recortada, sin HTML y dentro de `<pregunta>…</pregunta>`.
