# Día 2 — plan de pruebas para Día 3

**Estado: propuesta, pendiente de confirmación.** Este archivo describe
pruebas futuras; no ejecuté `go test`, `go build`, `sqlc generate` ni consultas
a una base de datos. En la inspección no aparecieron archivos `*_test.go` bajo
`backend/`.

## Casos mínimos de API

| Área | Entrada/acción | Resultado esperado | Evidencia actual |
|---|---|---|---|
| Crear oferta | Producto y tienda existentes, precio positivo, URL válida. | Una oferta y un registro inicial de historial; creación responde `201 Created` según la propuesta. | `backend/internal/features/catalog.go:71-110`; `backend/internal/features/router.go:18-22`; `backend/database/migrations/001_schema.sql:118-145`. |
| Crear oferta duplicada | Repetir producto+tienda con URL igual o distinta. | `409 Conflict` según la propuesta; no se crea otra fila ni se altera la existente. | UNIQUE actual `(product_id, store_id)`: `backend/database/migrations/001_schema.sql:118-134`; manejo actual de error: `backend/internal/features/catalog.go:96-106`. |
| Precio ausente | Enviar body sin propiedad `price`. | Rechazo de validación según la propuesta; no debe confundirse ausencia con valor `0`. | `offerInput.Price` es `float64` y la validación actual comprueba `< 0`: `backend/internal/features/catalog.go:14-25`, `71-75`. |
| URL vacía | Enviar `productUrl` vacío o con espacios. | Guardar `NULL` tras normalización según la propuesta; no guardar cadena vacía ni alterar una URL válida. | Columna nullable: `backend/database/migrations/001_schema.sql:132`; inserción actual pasa `input.ProductURL` directamente: `backend/internal/features/catalog.go:96-101`. |
| Condición inválida | Enviar `condition` fuera de `new`, `used`, `refurbished`. | Rechazo como dato inválido; sin filas parciales. | CHECK de columna: `backend/database/migrations/001_schema.sql:130-131`; valor por defecto del handler: `backend/internal/features/catalog.go:84-87`. |
| Stock negativo | Enviar `stock < 0`. | Rechazo de datos; no crear ni actualizar oferta. | CHECK de columna: `backend/database/migrations/001_schema.sql:128`; handler entrega el campo al INSERT/UPDATE: `backend/internal/features/catalog.go:96-101`, `146-151`. |
| Actualizar con precio cambiado | Actualizar oferta con precio distinto al anterior. | Actualiza precio y añade exactamente un punto de historial. | `backend/internal/features/catalog.go:120-165`; columna de historial: `backend/database/migrations/001_schema.sql:137-145`. |
| Actualizar sin cambio de precio | Cambiar stock/URL/condición sin cambiar el precio. | Actualiza la oferta sin añadir punto de historial de precio. | Condición `oldPrice != currentPrice`: `backend/internal/features/catalog.go:146-153`. |
| Promoción en historial | Enviar `is_promotional` al crear/actualizar según la propuesta. | Persistir el indicador esperado en el punto de historial; verificar también el valor por defecto actual (`FALSE`). | La columna existe: `backend/database/migrations/001_schema.sql:137-145`; el handler no la incluye al insertar historial: `backend/internal/features/catalog.go:102`, `152`. |
| Ingesta con tienda inexistente | POST `/ingestion/offers` con un `storeId` que Store API no reconoce. | Rechazo; no debe crearse oferta. Acordar si se conserva el actual `502` o se transforma a `404`. | `backend/internal/features/ingestion.go:64-82`; ruta: `backend/internal/features/router.go:95-98`. |
| Ingesta con producto inexistente | Oferta con `productId` inexistente. | Rechazo; no crear historial ni oferta. Revisar traducción actual del error aguas arriba. | Product ID se resuelve antes del INSERT: `backend/internal/features/catalog.go:78-82`; llamada desde ingestion: `backend/internal/features/ingestion.go:77-82`. |
| Ingesta duplicada | Enviar dos veces la misma identidad de oferta. | Segundo intento responde `409` según la propuesta; estado final sin cambios adicionales. | `backend/internal/features/ingestion.go:64-82`; UNIQUE: `backend/database/migrations/001_schema.sql:134`. |
| Colisión de SKU vía Product API | Crear producto con SKU existente. | Responder `409` según la propuesta, no error interno genérico. | SKU UNIQUE: `backend/database/migrations/001_schema.sql:56-70`; handler: `backend/internal/products/handler.go:24-50`. |
| Colisión de SKU, ingestion con imagen | Ingerir producto con `imageUrl` y SKU existente. | Respuesta de conflicto consistente con Product API; no crear segunda fila. | Delegación a Product API: `backend/internal/features/ingestion.go:30-45`; ruta de creación: `backend/internal/products/handler.go:24-50`. |
| Colisión de SKU, ingestion sin imagen | Ingerir producto sin `imageUrl` y SKU existente. | `409` según la propuesta, consistente con los otros dos flujos. | INSERT directo y manejo actual: `backend/internal/features/ingestion.go:46-61`; SKU UNIQUE: `backend/database/migrations/001_schema.sql:56-70`. |
| Normalización URL | Casos de espacios, URL vacía, `utm_*`, `gclid`, `fbclid`, query funcional, mayúsculas/minúsculas y barra final. | Normalizar solo según reglas aprobadas; no perder parámetros funcionales ni cambiar la identidad accidentalmente. | URL se guarda sin normalización en el handler: `backend/internal/features/catalog.go:96-101`; columna: `backend/database/migrations/001_schema.sql:132`. |
| Moneda | Enviar CLP y otra etiqueta de moneda a la oferta. | Verificar el comportamiento temporal y no afirmar conversión monetaria real. | Trigger fuerza etiqueta CLP: `backend/database/migrations/008_normalize_prices_to_clp.sql:6-14`, `26-30`, `38-42`. |
| Autenticación en escritura | Llamar rutas de ingestion sin credenciales y luego con identidad autorizada. | Sin credenciales: denegar; con credenciales: permitir según el mecanismo que apruebe el equipo. | Rutas sin middleware en el switch: `backend/internal/features/router.go:95-98`; ejemplo protegido: `backend/internal/features/router.go:25-31`. |

## Casos del borrador de migración

1. Aplicar `proposed_offer_url_index.sql`; esperar exit code `0`.
2. Aplicarlo por segunda vez; esperar exit code `0` por `IF NOT EXISTS`.
3. Verificar el índice parcial en `pg_indexes` y revisar con `EXPLAIN` una consulta por `(store_id, product_url)`.
4. Ejecutar su rollback; confirmar que el índice desaparece; reaplicar la migración.
5. En transacción que termine en `ROLLBACK`, probar que `proposed_unique_store_url_OPCIONAL.sql` aborta con mensaje claro ante URL duplicada por tienda; verificar que no quedaron filas de prueba.
6. En un conjunto sin duplicados, verificar que el índice único opcional se crea y que su rollback lo elimina.
7. Aplicar las migraciones originales en el orden explicitado en la instrucción de auditoría y ejecutar la copia de las 20 consultas; comparar con los conteos de referencia del Día 1.

**Nota de estado e historial:** esta nota fue escrita el 18-10-2026 antes de la
validación posterior. `resultados_dia2.txt` registra la ejecución SQL del Día 2
y `decisiones_v2.md` la confirma en su sección «Pruebas y equivalencia». Por
tanto, las frases «NO EJECUTADOS» y «se omitió el Paso D completo» de esta
versión son un estado preliminar obsoleto, no el resultado final del Día 2.
La reejecución reproducible del 23-10-2026 en una base desechable se registra
en `resultados_reejecucion.txt`; no demuestra por sí sola lo ocurrido el
18-10-2026.

## Estrategias posibles para integración

### PostgreSQL desechable con Docker y `--tmpfs`

- Iniciar solo el contenedor temporal autorizado, publicar el puerto en
  `127.0.0.1` y usar `--tmpfs` para que los datos no queden en un volumen
  persistente.
- Aplicar las migraciones y los borradores, ejecutar las comprobaciones y
  eliminar solo ese contenedor.
- **Ventajas:** sencillo de entender y cercano a una instancia PostgreSQL real.
- **Riesgos:** requiere daemon disponible; un error de nombre/puerto puede
  afectar otro recurso; si se omite `--tmpfs` o la limpieza, quedan datos
  locales. Se debe revisar el nombre y puerto antes de crear nada.

### Testcontainers

- Los tests Go crearían y destruirían una instancia efímera durante la suite.
- **Ventajas:** el ciclo de vida queda automatizado y los tests pueden ser
  repetibles en CI.
- **Riesgos:** requiere dependencia nueva y daemon/runtime compatible; agrega
  configuración y puede hacer los tests más lentos. No se debe agregar sin
  aprobación del equipo.

No se ejecutaron contenedores, tests de integración ni tests unitarios.
