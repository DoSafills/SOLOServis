# Auditoría TAL-127 — consultas y transacciones de PostgreSQL

**Fecha:** 2026-10-30  
**Alcance:** análisis estático de backend Go/pgx, SQL de sqlc y migraciones PostgreSQL. No se conectó a Neon ni a otra base de datos; no se modificó ningún archivo del repositorio.

## Resumen

- **Stack:** Go 1.27.1, `pgx/v5` 5.10 y consultas generadas con sqlc; pool `pgxpool`.
- Se hicieron búsquedas estáticas sobre el código Go del backend y revisión dirigida de comparaciones, catálogo, tiendas, servicios, carrito/favoritos, autenticación, búsquedas, precios e ingestión. En `backend/internal` se contaron **176 llamadas directas** a `Query`, `QueryRow` o `Exec` y **9 puntos `Begin`** en cinco archivos de cuatro paquetes (excluyendo archivos de test). El manifiesto declara 22 migraciones: coincide con los 22 `.sql` encontrados, sin nombres repetidos, faltantes ni migraciones sin listar.
- **Hallazgos:** 2 Altos, 12 Medios y 1 Bajo. No se confirmó SQL injection ni `SELECT *` en consultas de producción Go revisadas. Los valores dinámicos se pasan como parámetros; las piezas dinámicas observadas son identificadores/órdenes provenientes de listas internas permitidas.
- El detalle de `StoreProduct` confirma el hallazgo TAL111-01: SQL obtiene el ID real de oferta, pero el DTO de tienda no lo devuelve.

## Hallazgos

| ID | Severidad | Categoría | Archivo:línea | Problema | Riesgo | Arreglo mínimo propuesto |
|---|---|---|---|---|---|---|
| TAL127-01 | Alta | Integridad/propiedad de datos | `backend/internal/features/router.go:21-23,99-104`; `backend/internal/features/service_personalization.go:73-128,144-180,188-245` | Las rutas de comparaciones de servicios se registran sin middleware de sesión. La creación recibe `userId` del cuerpo; lectura, borrado y edición consultan por ID de comparación sin comprobar propietario. La tabla sí guarda `user_id` (`001_schema.sql:294-300`). | Una solicitud sin identidad autenticada puede acceder o modificar comparaciones que pertenecen a otras cuentas si conoce sus identificadores. | Aplicar el middleware de sesión, derivar el usuario de la sesión y añadir `user_id` a las condiciones de lectura/escritura/eliminación. |
| TAL127-02 | Alta | Integridad/propiedad de datos | `backend/internal/features/router.go:51-59`; `backend/internal/features/reviews_watch_notifications.go:28-110,128-155` | Las rutas de reseñas no exigen sesión; el backend acepta `userId` enviado por el cliente para insertar, actualizar o borrar reseñas. `UpdateReview` y `DeleteReview` usan ese valor como propietario. | Se pueden atribuir cambios de reseñas a otra cuenta; una falla o manipulación no queda limitada al usuario autenticado. | Proteger las mutaciones con sesión y usar el ID autenticado en lugar del `userId` del cuerpo/consulta. |
| TAL127-03 | Media | Consistencia relacional | `backend/internal/features/service_personalization.go:82-128`; `backend/database/migrations/001_schema.sql:294-309` | Al crear una comparación de servicios, solo se consulta la categoría del primer servicio y se insertan todos los servicios. No se comprueba que los demás compartan categoría; la tabla de ítems solo tiene FKs independientes y no lo impide. | Quedan comparaciones que mezclan categorías y no representan una comparación válida. | Validar en el servidor las categorías de todos los servicios antes de insertar, manteniendo las inserciones en la transacción. |
| TAL127-04 | Media | Concurrencia/transacción | `backend/internal/features/service_personalization.go:188-227`; `backend/database/migrations/001_schema.sql:303-309` | `AddServiceComparisonItem` cuenta ítems y luego inserta sin bloquear la fila de comparación. `CreateServiceComparison` tampoco limita el tamaño de la lista; la tabla no tiene una restricción de máximo de tres. | Solicitudes concurrentes pueden superar el límite de tres, y la ruta de creación puede excederlo directamente. | Aplicar el máximo también en creación y bloquear la fila padre con `SELECT ... FOR UPDATE` antes de contar e insertar. |
| TAL127-05 | Media | Transacción/recursos | `backend/internal/features/favorites_cart.go:140-164` | `AddCartItem` abre una transacción y, antes de insertar/confirmar, hace verificaciones HTTP al servicio de productos y al de precios. | La transacción retiene una conexión y su snapshot durante llamadas de red; latencia o caída de otro servicio puede consumir conexiones del pool y prolongar locks/recursos. | Hacer las llamadas externas antes de `Begin`; dentro de la transacción, limitarse a validar/guardar la información necesaria en PostgreSQL. |
| TAL127-06 | Media | Errores de transacción | `backend/internal/features/favorites_cart.go:180-185` | El error de `Commit` se combina con el error de `Exec` y ambos terminan como HTTP 400 sin registrar ni distinguir el fallo de base de datos. | Un fallo de persistencia se presenta como error del cliente, dificulta el diagnóstico y puede inducir reintentos incorrectos. | Separar los errores de `Exec` y `Commit`; registrar y responder el fallo de base de datos como error interno, verificando el error concreto de `Commit`. |
| TAL127-07 | Media | Inserciones/idempotencia | `backend/internal/features/favorites_cart.go:211-225` | Al añadir un servicio al carrito, `ON CONFLICT ... DO NOTHING` deja intacta una fila ya existente, pero la ruta responde `201 Created` con la cantidad solicitada. | El cliente cree que la cantidad solicitada quedó guardada aunque la base conserve la cantidad previa. | Usar `DO UPDATE` para sumar/actualizar cantidad y precio, o detectar `RowsAffected()==0` y responder explícitamente que el ítem ya existía. |
| TAL127-08 | Media | Contrato/esquema API | `backend/database/queries/stores.sql:71-118`; `backend/internal/stores/dto/store.go:27-45,88-107` | `ListStoreProducts` selecciona `po.id AS offer_id` y el modelo generado contiene `OfferID`, pero `StoreProduct` y `FromStoreProduct` no lo exponen. | El consumidor de productos de tienda no recibe el identificador real requerido para operar sobre la oferta/carrito; coincide con TAL111-01. | Añadir `offerId` al DTO y copiar `row.OfferID` en el mapper; mantener ese ID durante la adaptación del frontend. |
| TAL127-09 | Media | Rendimiento/paginación | `backend/internal/services/handler.go:22-49`; `backend/database/queries/services.sql:1-24,85-95` | `GET /services` carga todos los servicios y, por cada uno, ejecuta por separado consultas de ofertas, especificaciones e historial. La consulta de servicios e historial no tiene `LIMIT`/paginación. | El coste crece con servicios (al menos tres consultas adicionales por servicio), y los historiales completos aumentan memoria, transferencia y latencia. | Implementar paginación y cargar ofertas/especificaciones/historial por lotes para los IDs de la página; acotar/paginar historial. |
| TAL127-10 | Media | Rendimiento/paginación | `backend/internal/stores/repository.go:139-165`; `backend/database/queries/stores.sql:71-118` | El detalle de tienda carga todos sus productos/ofertas con `ListStoreProducts`, cuya consulta no tiene `LIMIT` ni paginación. | Una tienda con muchas ofertas puede devolver respuestas grandes y costosas. | Añadir paginación al detalle o una ruta paginada de productos/ofertas de la tienda. |
| TAL127-11 | Media | Atomicidad de ingestión | `backend/internal/ingestion/processor.go:63-111`; `backend/internal/products/catalog_repository.go:316-346`; `backend/internal/pricing/repository.go:121-149` | Para un producto nuevo, el procesador confirma `Catalog.Create` y después realiza el `UpsertOffer` en otra operación. Si el upsert falla, el ítem se informa fallido pero el producto creado queda persistido sin su oferta. | Puede quedar catálogo parcialmente aplicado para un ítem que el resultado reporta como fallido. | Compartir una transacción para crear producto/imagen/atributos y oferta, o definir una compensación explícita que no oculte el error. Las notificaciones externas deben seguir fuera de la transacción. |
| TAL127-12 | Media | Atomicidad de autenticación | `backend/internal/features/auth.go:53-79,91-108` | `Refresh` elimina el refresh token y luego `issueTokens` inserta la nueva sesión en una operación separada. Si falla la inserción, el token anterior ya quedó consumido. En `Register`, el usuario también se confirma antes de guardar su primera sesión. | Fallos intermedios pueden dejar una cuenta creada sin sesión o cerrar la sesión sin entregar una renovación válida. | Hacer la rotación del token en una transacción (eliminar token anterior e insertar el nuevo); revisar también el alta de usuario y sesión como unidad atómica. |
| TAL127-13 | Bajo | Manejo de errores | `backend/internal/features/auth.go:111-120` | `Logout` descarta el error de `DELETE user_session` y siempre devuelve 204. | Si falla la base, la sesión sigue válida aunque el cliente recibió confirmación de cierre. | Comprobar el error y responder/loguear el fallo; conservar 204 solo cuando la eliminación se completó o el token ya no existía, según el contrato elegido. |
| TAL127-14 | Media | Consistencia de métricas | `backend/internal/ingestion/processor.go:63-82` | Si falla `AddCounters`, solo se escribe un log y se devuelve el resultado del lote como si las métricas se hubieran actualizado. | `scrape_run` puede mostrar conteos que no coinciden con los resultados por ítem. | Propagar el fallo o incluir explícitamente un estado de persistencia de contadores en la respuesta; permitir reconciliar contadores desde los resultados guardados. |
| TAL127-15 | Media* | Retención de historial | `backend/database/migrations/001_schema.sql:118-143,282-288` | Eliminar una oferta elimina en cascada su historial de precios; eliminar producto/tienda puede borrar la oferta y, por transitividad, el historial. | Se pierde trazabilidad histórica si el negocio espera conservar precios de ofertas retiradas. | Confirmar la política de retención. Si el historial debe sobrevivir, usar baja lógica/archivado o separar una instantánea histórica de la oferta viva antes de cambiar la FK. |

\* Impacto condicionado a que el requisito de negocio sea conservar historial tras retirar una oferta.

## Inserciones, actualizaciones y eliminaciones

- Las mutaciones del catálogo y la mayoría de los borrados filtran por clave y/o propietario. `Create`/`Update` de producto guardan producto, imagen y atributos en una transacción (`catalog_repository.go:316-386`). `UpsertOffer` usa la identidad única de producto/tienda/URL y `ON CONFLICT` (`pricing/repository.go:121-149`).
- No se encontró un `UPDATE`/`DELETE` de producción sin filtro. `ClearCart` borra solo por `user_id`; la limpieza completa requiere confirmar que esta es la semántica prevista.
- Las excepciones de propiedad/atomicidad y el `DO NOTHING` del carrito se detallan en TAL127-01, 02, 07, 11 y 12.

## Consultas

- No se confirmó SQL injection: los filtros se parametrizan; el orden de búsqueda usa un mapa de órdenes constantes (`search/repository.go:86-91,133-155`). La construcción de tabla para reseñas se alimenta de un mapa fijo en `parseReviewID` (`reviews_watch_notifications.go:144-155`), no de entrada SQL libre.
- No se observó `SELECT *` en el SQL de producción Go inspeccionado. Los cursores que recorren filas usan `defer rows.Close()` y verifican `rows.Err()`; los métodos que usan `pgx.CollectRows` consumen mediante ese helper.
- Riesgos verificados de N+1 y lecturas sin límite: TAL127-09 y TAL127-10. Las consultas de historial de servicio también devuelven el historial completo.
- Los valores de dinero se almacenan como `NUMERIC(12,2)` en el esquema. Algunas lecturas intermedias de carrito escanean el precio a `float64` (`features/favorites_cart.go:147-152`); conviene evitar introducir más conversiones flotantes en cálculos monetarios y preferir representación decimal/texto. No se elevó a hallazgo independiente porque esta ruta guarda los valores de la columna con dos decimales y no se verificó un desvío de centavos con datos reales.

## Transacciones

- Se encontraron nueve llamadas `Begin` en cinco paquetes. Las rutas que las usan instalan `defer tx.Rollback(...)` tras `Begin`; creación de producto, comparaciones de producto e ingestión de ejecución verifican los errores de `Commit`.
- Problemas concretos: llamada HTTP dentro de la transacción del carrito (TAL127-05); límites de comparación de servicios sin bloqueo (TAL127-04); creación de producto/oferta y renovación de sesión no atómicas (TAL127-11/12); el error de commit del carrito queda clasificado como error de cliente (TAL127-06).
- La agregación por lote de ingestión es deliberadamente parcial entre ítems; la unidad que queda parcialmente aplicada es un ítem que crea un producto antes de fallar al crear su oferta (TAL127-11).

## Errores y recursos

- Consultas del backend reciben `context.Context` o `r.Context()`. El pool configura máximo 10, mínimo 2, vida máxima de 1 hora e idle máximo de 30 minutos (`internal/database/postgres.go:10-28`); el servidor cierra el pool al salir (`cmd/server/main.go` y `server/run.go`). No observé conexiones adquiridas manualmente que deban liberarse.
- Errores concretos a atender: logout ignora el error de DB (TAL127-13), fallo de contadores no cambia el resultado (TAL127-14) y commit de carrito no se clasifica/loguea correctamente (TAL127-06).
- La conversión de no-encontrado usa `pgx.ErrNoRows` en repositorios inspeccionados; no aplica `database/sql.ErrNoRows` al driver pgx.

## Esquema y migraciones

- `001_schema.sql` define claves primarias, FKs, `NOT NULL`, defaults, `UNIQUE` y `CHECK` en las entidades de catálogo y usuario; importes principales usan `NUMERIC(12,2)`. Las relaciones de historial con oferta usan `ON DELETE CASCADE` (TAL127-15).
- El manifest contiene todos los 22 archivos SQL actuales sin duplicados de nombre. Las migraciones `016_fk_indexes.sql` y `017_nm_uniqueness.sql` añaden índices para FKs y unicidad de asociaciones; el conteo estático no demuestra que esas migraciones ya se aplicaron en una instancia.
- Desajustes encontrados entre consulta, tipo generado y DTO de tienda: TAL127-08. Restricciones relacionales que la base no puede verificar para comparación de servicios: categoría común (TAL127-03) y máximo de ítems (TAL127-04).
- No se consultaron `information_schema`, `pg_constraint` ni `pg_indexes` en Neon. Por tanto, el estado real de la instancia, índices desplegados, duplicados históricos y planes de ejecución quedan sin verificar.

## Priorización y esfuerzo

1. **S:** TAL127-01 y TAL127-02 — imponer identidad/propiedad en mutaciones y lecturas de datos personales.
2. **S:** TAL127-03 y TAL127-04 — evitar comparaciones de servicio inválidas y límites inconsistentes/concurrentes.
3. **S:** TAL127-08 — devolver el `offerId` real para corregir el flujo de carrito desde tienda.
4. **M:** TAL127-05, 06, 07, 11, 12, 13 y 14 — corregir límites transaccionales y respuestas ante errores.
5. **M/L:** TAL127-09 y 10 — paginar y agrupar consultas; TAL127-15 depende de una decisión de retención y podría requerir migración.

## Archivos y componentes revisados

Se realizó búsqueda estática en el código Go del backend, incluidos comandos, paquetes internos y archivos de test como referencia (sin ejecutar tests). Lectura dirigida de:

- `backend/internal/features/router.go`, `auth.go`, `favorites_cart.go`, `service_personalization.go`, `reviews_watch_notifications.go`, `repository.go`
- `backend/internal/services/handler.go`, `repository.go`
- `backend/internal/stores/repository.go`, `dto/store.go`
- `backend/internal/products/catalog_repository.go`, `handler.go`, `repository.go`
- `backend/internal/comparison/repository.go`
- `backend/internal/ingestion/processor.go`, `repository.go`, `handler.go`
- `backend/internal/pricing/repository.go`, `notifier.go`
- `backend/internal/search/repository.go`
- `backend/internal/database/postgres.go`, `backend/internal/server/run.go`, `backend/cmd/server/main.go`
- `backend/database/queries/products.sql`, `services.sql`, `stores.sql`; migraciones `001_schema.sql`, `004_platform_apis.sql`, `007_service_cart.sql`, `014_product_attributes.sql`, `015_ingestion_data_source_store.sql`, `016_fk_indexes.sql`, `017_nm_uniqueness.sql`; `migration_order.txt`.

## No verificado / límites

- No se conectó a Neon ni a ninguna base. No se revisaron datos reales, existencia de duplicados, ejecución/aplicación efectiva de migraciones ni estado de constraints e índices desplegados.
- No se ejecutaron `go test`, `go vet` ni `go build`: la tarea pidió auditoría estática de solo lectura y existen pruebas de integración que podrían requerir una base. Los conteos de llamadas y migraciones son búsquedas estáticas, no telemetría de producción.
- No se ejecutaron `EXPLAIN ANALYZE` ni pruebas concurrentes; los problemas de paginación, límites y transacciones se deducen del código y del DDL revisados.