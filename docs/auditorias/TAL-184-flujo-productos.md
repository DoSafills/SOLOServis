# TAL-184 — Validación del flujo de productos

| Campo | Valor |
| --- | --- |
| Tarea | TAL-184: comprobar el proceso completo desde la extracción hasta la visualización y consulta de productos |
| Fecha | 2026-11-18 (corridas del scraper de las 03:00) |
| Despliegue | clúster UCT, namespace `student-marcelo-vidal-22843887e32d`, topología `dominios`, tag `471c467` |
| Script | `scripts/testing/flujo_productos.py` |
| Relacionado | [TAL-183](TAL-183-pruebas-integrales.md) (pruebas integrales), TAL-169 (identidad de productos), TAL-241 (seed), TAL-242 (imágenes de Falabella) |

---

## 1. Resumen

1. **El flujo funciona de punta a punta:** los 83 ítems que extrajo el scraper el 18-11 están en el catálogo con el mismo precio, y los 80 productos se consultan bien por el gateway (detalle, historial y búsqueda).
2. **0 fallas en dos días seguidos:** las corridas del 17-11 y del 18-11 dieron 12 OK, 0 fallas y 2 avisos.
3. **Sin duplicados nuevos:** cada corrida suma solo publicaciones nuevas (6 el 17-11, 3 el 18-11) y registra cambios de precio reales.
4. **Dos avisos ya registrados en tickets:** 18 productos de Falabella sin imagen (TAL-242) y el Midea duplicado con un producto de demostración (TAL-241).

## 2. Cómo se valida

`flujo_productos.py` compara lo que extrajo el scraper con lo que el sitio muestra, ítem por ítem, en 4 etapas:

| Etapa | Qué comprueba | Fuente |
| --- | --- | --- |
| 1. Extracción | El CronJob disparó y su Job terminó; `scraper-api` responde; cada tienda tiene una corrida `success` en las últimas 26 h, sin ítems rechazados | `kubectl`, `scrape_run` |
| 2. Ingesta | Cada ítem extraído (`scraped_data`) tiene su oferta (misma tienda y URL), con el mismo precio, actualizada en esa corrida y con el producto activo; ninguna URL con dos productos scrapeados | PostgreSQL (`READ ONLY`) |
| 3. Consulta | `GET /products/{id}` trae esa oferta con ese precio; el historial del detalle (`priceHistory`) termina en ese precio; `GET /search/products?q=` encuentra el producto por su nombre | HTTP por el gateway |
| 4. Visualización | La página del producto responde con la app; el producto tiene imagen de la tienda | HTTP por el gateway |

```bash
python scripts/testing/flujo_productos.py                # muestra de 5 productos
python scripts/testing/flujo_productos.py --todos        # todos los productos de las corridas
python scripts/testing/flujo_productos.py --todos --md flujo.md
```

Es de solo lectura y no usa el token de servicio. El historial se consulta desde el detalle del producto, igual que el frontend: la API de precios (`/offers`) es interna y el gateway no la publica.

## 3. Resultado (2026-11-18, `--todos`)

**✅ El flujo funciona de punta a punta: 12 OK, 0 fallas, 2 avisos.**

| Etapa | Estado | Detalle |
| --- | --- | --- |
| Extracción | ✅ | CronJob activo (`0 3 * * *`); Job `scraper-trigger-29916360` terminado: Falabella, Mercado Libre y Ripley pedidas (HTTP 202) |
| | ✅ | `scraper-api /health` → 200 |
| | ✅ | Corrida 21 (Falabella): 26 ítems, 26 procesados, 0 fallidos |
| | ✅ | Corrida 22 (Ripley): 57 ítems, 57 procesados, 0 fallidos |
| Ingesta | ✅ | Falabella: 26 de 26 ítems con su oferta, el mismo precio y actualizada en la corrida |
| | ✅ | Ripley: 57 de 57 ítems en 54 ofertas (3 refrigeradores aparecen dos veces en el listado de la tienda, con la misma URL base y el mismo precio) |
| | ✅ | Ninguna URL con dos productos scrapeados |
| | ⚠️ | El Midea MDRS710 de Ripley comparte URL con un producto de demostración (productos 29 y 86): TAL-241 |
| Consulta | ✅ | Detalle: 80 de 80 con la oferta y el precio extraídos |
| | ✅ | Historial del detalle: 80 de 80 terminan en el precio actual |
| | ✅ | Búsqueda por nombre: 80 de 80 encontrados |
| Visualización | ✅ | Página del producto: 80 de 80 responden con la app |
| | ⚠️ | 62 de 80 con imagen: los 18 sin imagen son de Falabella (TAL-242) |

Mercado Libre recibe el pedido, pero no genera corrida: se queda en su verificación anti-bots, como está documentado. No se intenta saltar.

### El catálogo entre corridas

| | Tras la limpieza (021) | Tras el 17-11 | Tras el 18-11 |
| --- | --- | --- | --- |
| Productos | 134 | 140 (+6) | 143 (+3) |
| Ofertas | 174 | 180 (+6) | 183 (+3) |
| Historial de precios | 620 | 636 (+16) | 661 (+25) |

- Los productos nuevos son publicaciones nuevas en las tiendas, con URLs que no estaban en la base. Antes de TAL-169, cada corrida duplicaba unos 80.
- El 18-11, el historial sumó 3 registros por el precio inicial de las ofertas nuevas y 22 por cambios de precio en ofertas existentes. El trigger de la migración 010 solo registra cuando el precio cambia.

La corrida del 17-11 dio el mismo resultado: 12 OK, 0 fallas y los mismos 2 avisos, con 79 de 79 productos consultados.

## 4. Pruebas de que detecta fallas

Contra el clúster, pasándole datos alterados a propósito:

| Dato alterado | Resultado |
| --- | --- |
| Precio distinto del que tiene la oferta | ❌ FALLA en el detalle y en el historial ("el detalle muestra 159.990") |
| Oferta que no existe en el producto | ❌ FALLA en el detalle ("no trae la oferta") y en el historial |
| Producto inexistente | ❌ FALLA en el detalle (HTTP 404) y en la búsqueda |

## 5. Hallazgos

- **18 de los 27 productos de Falabella llegan sin imagen** desde la primera corrida: el spec solo obtiene la imagen de los primeros 7–8 del listado, probablemente por la carga diferida de la tienda. Además, ingestion no completa la imagen de un producto que ya existe. Queda en TAL-242.
- **Repetidos en el listado de Ripley:** algunos refrigeradores aparecen dos veces (posiciones distintas, misma URL base). Ingestion los une en una sola oferta, que es lo correcto; el script lo informa sin marcarlo como falla.
- **Documento de TAL-183 corregido:** decía que `/offers/92/price-history` respondía 200 por el gateway. Ese 200 era la página HTML de la app: el gateway no publica `/offers`.

## 6. Limitaciones

- La página del producto responde 200 aunque el producto no exista: la app se arma en el navegador. Esa comprobación solo confirma que la app se sirve; los datos los valida el detalle.
- Si el pod del Job de disparo ya se borró, el script usa el estado del Job en vez de su log.
- Necesita `kubectl` con acceso al namespace y una corrida reciente: no va al CI.
