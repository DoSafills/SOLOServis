# Auditoría frontend: productos, tiendas y comparación

## Hallazgos

- `src/Services/api/products.ts` y `src/Services/api/products-client.ts` contienen dos adaptadores `toProduct` incompatibles. El segundo pierde `offers.offerId`, historial de precios y datos de oferta; debe retirarse o converger con el primero.
- `src/Services/api/api.ts` vuelve a implementar `getProductById`, `getStores` y `getStoreById`, mientras `products.ts` y `stores.ts` exponen variantes paralelas. Las páginas actuales consumen mayoritariamente `api.ts`, pero el detalle de producto aún consume `products.ts`.
- La UI de categorías usa `fallbackCategoryImages` de Unsplash cuando el backend no entrega imágenes. Es un fallback visual ficticio, no dato de catálogo.
- `backend/README.md` documenta `placehold.co` y dominios `example.test` en seeds/demos. No deben presentarse como productos, tiendas u ofertas reales en producción.
- La comparación de productos y servicios comparte patrón de carga, estados y tablas, pero no un componente genérico. Las reglas de compatibilidad están separadas correctamente por dominio.
- La tienda transforma productos de tienda a un `Product` artificial para reutilizar el carrito. Esta adaptación debe mantenerse aislada y documentada porque no representa un producto completo del catálogo.

## Decisiones para esta tarea

- Las definiciones de atributos provienen exclusivamente de `GET /categories/{slug}`.
- El nuevo formulario valida en cliente antes de llamar a `POST /products`: obligatorios, tipo, enum, largo, rango numérico y unidad mostrada.
- La API de escritura queda encapsulada en `src/Services/api/product-write.ts`; la UI no realiza llamadas HTTP directas.
