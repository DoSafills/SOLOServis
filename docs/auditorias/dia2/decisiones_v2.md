# Decisiones de diseño — versión corregida (propuesta)

## Actualización de contexto

- Rama de trabajo revisada: `feature/offer-url-index-design`.
- Commit observado al inicio de esta revisión: `5aa528f avances de auditoria`; difiere del `2738771` indicado en el contexto original.
- Estado al inicio: `git status --short` vacío.
- Esta versión es una propuesta de decisión provisional; no reemplaza la confirmación del equipo ni la validación final en base real.

## Tabla de decisiones

| Tema | Opción elegida / propuesta | Evidencia clave | Afirmación de decisión | Cita real y verificación | Estado |
|---|---|---|---|---|---|
| Historial al borrar una oferta | Desactivar la oferta y conservar el historial, en vez de borrar físicamente la fila. Requiere incorporar una columna `active` a `product_offer` y decidir si la desactivación es un cambio de esquema adicional. | `backend/database/migrations/001_schema.sql:118-145` | La estructura actual no incluye `active` ni ninguna lógica de soft-delete en `product_offer`; el borrado real hoy es con `ON DELETE CASCADE` de `product_price_history`. | `001_schema.sql:118-145` muestra `UNIQUE (product_id, store_id)` y `product_price_history`; no existe `active` en `product_offer`. | propuesta |
| POST /offers | Mantener la propuesta de exigir precio y distinguir ausencia de 0; si el requisito es “precio obligatorio”, el tipo debe ser `*float64` en la entrada, no `float64`. | `backend/internal/features/catalog.go:14-25`; `backend/internal/features/catalog.go:71-75` | `Price` es `float64`, y la validación actual solo verifica `< 0`; eso no distingue campo omitido de `0`. | `catalog.go:14-25` define `Price float64` y `ListPrice *float64`; `catalog.go:71-75` valida `input.Price < 0`. | propuesta |
| Convención de nombres de migración | **PROPUESTA**, dependiente de la respuesta del equipo sobre las migraciones numeradas pendientes: usar un prefijo lowercase que ordene después de los `008_*` actuales, por ejemplo `008z_offer_url_index.sql`. El nombre final requiere confirmación del equipo. | `k8s/base/migrations/migrate.sh:31-58` | El runner ordena `.sql` por basename y usa `schema_migrations.version = basename`. El nombre debe respetar el orden del runner y evitar colisiones entre ramas. | `migrate.sh:31-58` lista los `.sql`, los ordena con `sort`, y `basename` se usa como `version`; el orden de nombres con mayúsculas/minúsculas depende del locale no fijado. | propuesta |
| Valor del índice propuesto | Hoy ninguna consulta del backend busca por `product_url` de forma explícita, por lo que el índice no aporta rendimiento inmediato hasta que el código lo use. | `backend/database/queries/products.sql:81-100`; `backend/database/queries/stores.sql:81-100`; `backend/internal/features/catalog.go:96-101` | El código entrega y guarda `product_url`, pero no hay un patrón de búsqueda por URL de oferta en el backend que justifique el índice solo por el campo. | `products.sql:81-100` y `stores.sql:81-100` consultan `po.product_url` en listados, pero no por filtro en `WHERE ... product_url = ...`. | propuesta |
| Moneda y trigger CLP | Mantener la regla temporal de CLP como decisión de negocio y documentarla explícitamente como temporal; no tratarla como conversión real de moneda. | `backend/database/migrations/008_normalize_prices_to_clp.sql` | El trigger fuerza la etiqueta a CLP en oferta y servicio; el README del backend describe la regla como temporal. | `backend/README.md:1117-1124` documenta `009_normalize_prices_to_clp.sql` y `010_seed_all_services.sql`; la lógica exacta del trigger debe revisarse en el SQL real. | propuesta |
| Autenticación en rutas de escritura | Las rutas de ingestión deben requerir autenticación o una identidad autorizada explícita; no asumir que el frontend o CORS entrega esa seguridad. | `backend/internal/features/router.go:95-98`; `backend/internal/features/handler.go:24-42`; `backend/internal/features/auth.go:138-143` | `requireSession` valida `Authorization: Bearer ...`; las rutas de ingestion no usan ese middleware. | `router.go:95-98` muestra `r.Post("/ingestion/*")` sin `r.Use(h.requireSession)`; `handler.go:24-42` y `auth.go:138-143` muestran la validación real. | propuesta |
| Identidad de tienda | Usar `store_id` interno estable como identidad relacional del lazo oferta-tienda; no basarse solo en `store.name` como clave de negocio. | `backend/database/migrations/001_schema.sql:118-145`; `backend/internal/features/ingestion.go:64-82` | El esquema usa `store_id` y la ingesta valida el store con la API de stores antes de crear la oferta. | `001_schema.sql:118-145` define `store_id INTEGER NOT NULL REFERENCES store(id)`; `ingestion.go:64-82` valida el store remoto. | propuesta |
| Separar esquema y datos demo | Mantener esquema y seeds separados; el runner actual aplica todos los `.sql` sin distinguir seeds de esquema. Se recomienda claridad explícita si el equipo quiere demo opcional. | `k8s/base/migrations/migrate.sh:31-58`; `backend/database/migrations/003_seed.sql`; `backend/database/migrations/005_demo_catalog.sql` | El runner aplica todos los `.sql` del directorio de migraciones y los registra en `schema_migrations`; no hay separación de “esquema” y “demo” en el motor actual. | `migrate.sh:31-58` itera sobre `"$MIGRATIONS_DIR"/*.sql` y aplica cada uno. | propuesta |
| Duplicado de oferta | Rechazar, actualizar o crear otra fila debe decidirse con la política de negocio; el esquema actual no define un `UNIQUE` sobre URL ni sobre `(product_id, store_id, product_url)`. | `backend/database/migrations/001_schema.sql:118-145` | El único `UNIQUE` actual es sobre `(product_id, store_id)`. | `001_schema.sql:118-145` contiene `UNIQUE (product_id, store_id)` y no una clave única por URL. | propuesta |
| SKU y colisiones | Las colisiones de SKU deben responder `409` consistentemente en todos los flujos de creación y de ingesta, pero la estrategia de manejo real depende del backend y no se puede inferir solo del esquema. | `backend/database/migrations/001_schema.sql:56-70`; `backend/internal/products/handler.go:24-50`; `backend/internal/features/ingestion.go:24-45` | El esquema tiene restricción de SKU en `product`; la creación directa ya valida `imageUrl` y la ingesta delega en Product API cuando hay imagen. | `001_schema.sql` define `product.sku` y `handler.go:24-50` valida url http(s); `ingestion.go:24-45` delega en Product API. | propuesta |
| URL de oferta y normalización | Definir explícitamente qué normalización de URL se acepta (espacios, mayúsculas, query funcional, `utm_*`, `gclid`, `fbclid`, barra final, etc.). El código actual no normaliza `product_url` antes de guardar. | `backend/internal/features/catalog.go:96-101` | El `INSERT` guarda `input.ProductURL` tal cual llega. No aparece normalización fuera del tipo/validación. | `catalog.go:96-101` insert: `... product_url)` con `input.ProductURL` tal cual. | propuesta |
| Ingesta con tienda inexistente / producto inexistente | La política de rechazo es un punto de decisión del equipo; el código actual delega en las APIs y responde como error de gateway si la validación falla. | `backend/internal/features/ingestion.go:64-82` | `IngestOffer` valida `storeId` con Store API y delega la creación de oferta a Pricing API. | `ingestion.go:64-82` muestra `remoteJSON(... /stores/{id})` y `/offers`. | propuesta |
| Revisión del historial y `ON DELETE CASCADE` | El historial actual se borra si se borra la oferta; la decisión requiere marcar si el historial debe sobrevivir a la eliminación lógica de una oferta o si la eliminación física es aceptable. | `backend/database/migrations/001_schema.sql:137-143` | `product_price_history` tiene `ON DELETE CASCADE` desde `product_offer`. | `001_schema.sql:137-143` confirma `product_offer_id INTEGER NOT NULL REFERENCES product_offer(id) ON DELETE CASCADE`. | propuesta |
| Detección de `user_email_verified` | No se encontró evidencia en esta rama del campo `user_email_verified`; la decisión requiere confirmación del equipo sobre si se incorporó en otra rama o se descartó. | `backend/database/migrations/001_schema.sql` (revisión general) | La rama revisada no muestra un campo `user_email_verified` en `user_account` dentro del esquema actual. | Hay que confirmar en la rama `develop` o en una rama de referencia; no se encontró evidencia en esta revisión. | propuesta |
| Rama de referencia | El equipo debe confirmar la rama de referencia. En esta revisión, la rama activa es `feature/offer-url-index-design`; el commit observado no coincide con el indicado en el contexto original. | `git branch --show-current`; `git log -1 --oneline` | Rama: `feature/offer-url-index-design`; commit: `5aa528f avances de auditoria`. | Verificación inicial de esta sesión; el contexto original citaba `2738771`. | propuesta |

## Preguntas reales del correo (versión corregida)

1. ¿Existen las migraciones 009 y 010 en el equipo de alguien, o nunca se crearon?
2. ¿`user_email_verified` existe solo en `develop`: se incorpora o se descarta?
3. ¿Cómo se aplican hoy las migraciones: solo `initdb.d` de Docker o con una herramienta?
4. ¿Autorizan una consulta de solo lectura a una base real (o un dump / base de desarrollo)? ¿Cuál?
5. ¿Ante una oferta existente en la ingesta: rechazar, actualizar (`upsert`) o crear otra?
6. ¿Identidad de tienda (store.name no es único): por nombre, dominio u otro?
7. ¿El historial debe sobrevivir al borrado de la oferta (hoy `ON DELETE CASCADE`)?
8. ¿Se corrigen las 4 ofertas demo sin historial (IDs 2, 4, 5, 6)?
9. ¿Colisión de SKU o UNIQUE: responder `409` en todos los flujos?
10. ¿`POST /offers` debe devolver `201` y un price ausente debe ser error?
11. ¿Se unifica la ingesta de productos en un solo camino (`ingestion.go:55-59` es código inalcanzable)?
12. ¿Las rutas POST/PUT/DELETE de productos, ofertas e ingestion deben exigir autenticación y rol? ¿Hay un gateway que ya las protege?
13. ¿Se separan migraciones de esquema y datos demo? ¿Se adopta una herramienta de migraciones versionadas?
14. ¿Convención de numeración y nombres para la próxima migración?
15. ¿Cuál es la rama de referencia del proyecto?

Estado de todas las decisiones: propuesta, pendiente de confirmación del equipo.

## Observaciones de evidencia (HECHO / SUPOSICIÓN)

### HECHO

- `backend/database/migrations/001_schema.sql` define `product_offer` con `UNIQUE (product_id, store_id)` y `product_url` como `TEXT`.
- `backend/internal/features/router.go` protege `search-api`, `favorites-api` y `cart-api` con `requireSession`, y deja `ingestion-api` sin ese middleware.
- `backend/internal/features/handler.go:24-42` implementa `requireSession` leyendo el header `Authorization` y validando un token Bearer.
- `backend/internal/features/auth.go:138-143` implementa `bearerToken` con `Bearer ` prefix extraction.
- `k8s/base/migrations/migrate.sh` ordena todos los `.sql` por `basename`, usa `schema_migrations` como tabla de versiones y aplica cada archivo en transacción con `--single-transaction` y `ON_ERROR_STOP=1`.
- `docker/compose.yml` monta `../backend/database/migrations` en `/docker-entrypoint-initdb.d:ro` y usa un volumen nombrado `postgres_data`.
- `backend/README.md:1117-1124` menciona 009/010 y la ejecución manual de migraciones por archivo.

### SUPOSICIÓN

- La diferencia entre “frente a una base ya inicializada” y “volumen nuevo” puede requerir validación en una base real para afirmar la estrategia de reaplicación con total seguridad.
- La propuesta `008z_offer_url_index.sql` sigue el orden lexicográfico actual, pero la rama real del equipo puede usar otra conveniencia, por lo que debe confirmarse antes de escribirla como convención definitiva.
- El valor de rendimiento del índice sobre `product_url` no está probado aún; solo se infiere que no se usa en el código actual por consultas con `WHERE product_url = ...`.
- El campo `active` para desactivar ofertas aún no existe: se propone como cambio de esquema adicional, no como requisito ya aceptado por la rama.

## Conclusión

La evidencia actual soporta que:

- existen `UNIQUE` y índices actuales para `product_offer` pero no uno sobre `product_url`;
- `requireSession` existe y se aplica solo a rutas autenticadas;
- el runner de K8s versiona SQL por nombre de archivo y evita re-aplicar archivos ya ejecutados;
- Docker compose usa `initdb.d` y no es un sistema versionado de migraciones;
- la propuesta de índice requiere decidir si es un cambio de esquema adicional y si el equipo quiere mantener `product_url` como atributo o como parte de la identidad de una oferta.

Todo esto permanece en estado de propuesta, pendiente de la confirmación del equipo.

## Decisiones aprobadas 19-10-2026

### Implementación y commits por fase

| Fase | Resultado | Hash del commit |
|---|---|---|
| A | Conflictos de unicidad responden 409, creación de ofertas responde 201, precio obligatorio y `is_promotional` en historial. | `d2d8251bfc204f5540eb5c7d118585286c46c863` |
| B | Normalización de `product_url`, URL vacía como `NULL`, pruebas de casos. | `61e437dd83e1a8f6366ffdaace38b0f36efd1071` |
| C | Ingesta de productos delegada al Product API; `imageUrl` opcional. | `fd266bdc44580ff9a23928e8b64758c68a64c154` (checkpoint que incluye el trabajo de C) |
| D | Autenticación/rol admin se pospuso por decisión del usuario. Las escrituras continúan públicas y se dejaron comentarios TODO; no se implementó reenvío de Authorization. | Sin commit |
| E | Índices de identidad de oferta, tienda y categoría; pruebas de caracterización. Cambios pendientes de commit. | Sin commit |
| F | Renumeración, separación de seeds, backfill, runner y README. Equivalencia verificada en base desechable; cambios pendientes de commit. | Sin commit |
| G | Protección de `deploy.ps1` no implementada. | Sin commit |

Los commits de A, B y C son los hashes históricos de esta rama; no se crearon commits para D-G en esta serie de trabajo.

### Supuestos aprobados para las decisiones 11-15

- **11, identidad de oferta:** identidad por `(product_id, store_id, product_url normalizada)`. La URL ausente se representa como `NULL` en código y como cadena vacía en la expresión única; por tanto, se permite una sola oferta sin URL por producto y tienda. Se añade un índice único sobre `(product_id, store_id, COALESCE(product_url, ''))` y un índice no único parcial sobre `(store_id, product_url)` para URLs no vacías. No se reescriben filas existentes.
- **12, nombres normalizados:** tienda única por `LOWER(BTRIM(name))`; categoría única por `(COALESCE(parent_category_id, 0), LOWER(BTRIM(name)))`. Los bloques de validación de duplicados detienen la migración con una excepción y su conteo.
- **13, transacciones:** los SQL de migración no deben incluir transacciones explícitas de nivel superior; el runner aplica cada archivo y registra su basename en `schema_migrations` en una única transacción.
- **14, numeración:** nombres en minúsculas y prefijos consecutivos sin duplicados. Migraciones finales: `001_schema.sql`, `002_views.sql`, `003_platform_apis.sql`, `004_service_cart.sql`, `005_normalize_prices_to_clp.sql`, `006_offer_identity_and_url_index.sql`, `007_store_and_category_name_unique.sql`.
- **15, seeds:** los datos iniciales y de ejemplo se separan de las migraciones y se cargan manualmente en orden. Seeds finales: `001_seed.sql`, `002_demo_catalog.sql`, `003_demo_catalog_services.sql`, `004_refrigeradores.sql`, `005_computacion.sql`, `006_telefonia.sql`, `007_history_backfill.sql`. La carga indicada es `psql -v ON_ERROR_STOP=1 -1 -f`; el backfill inserta historial solo para ofertas sin historial.

Los nombres de archivos anteriores son los adoptados en F-2. Los cuatro seeds que tenían `ON CONFLICT (product_id, store_id)` se ajustaron para inferir el índice de expresión de la nueva identidad de oferta.

### Correcciones de citas de Fase 0

- `001_schema.sql` tiene 469 líneas, no aproximadamente 330. La restricción `UNIQUE (product_id, store_id)` estaba en la línea 134; las líneas 459-460 corresponden a los índices `idx_product_offer_product` e `idx_product_offer_store`, no a esa restricción.
- `requireSession` se implementa en `backend/internal/features/handler.go:24-45`. La cita `backend/internal/features/router.go:25-31` no identifica esa implementación: allí se registran rutas de búsqueda, incluida la aplicación del middleware en la línea 29.
- Los nombres `009_normalize_prices_to_clp.sql` y `010_seed_all_services.sql` que aparecían en el README eran incorrectos para los archivos existentes; los nombres aprobados están en el inventario de la decisión 14.

### Pruebas y equivalencia

Los casos SQL del plan de pruebas **sí se ejecutaron el 18-10-2026 en una base desechable**. Los casos de API seguían sin ejecutar hasta las pruebas de caracterización del Día 3. No se creó `plan_pruebas_v2.md`.

SQL de migración de versiones antiguas para las tres migraciones renombradas:

```sql
UPDATE schema_migrations
SET version = CASE version
    WHEN '004_platform_apis.sql' THEN '003_platform_apis.sql'
    WHEN '007_service_cart.sql' THEN '004_service_cart.sql'
    WHEN '008_normalize_prices_to_clp.sql' THEN '005_normalize_prices_to_clp.sql'
    ELSE version
END
WHERE version IN (
    '004_platform_apis.sql',
    '007_service_cart.sql',
    '008_normalize_prices_to_clp.sql'
);
```

**Advertencia:** ejecutar el mapeo **antes de desplegar el runner nuevo en una base existente**; de lo contrario, reaplicaría archivos renombrados. El SQL se verificó únicamente en una base desechable, no en otra base.

### Pendientes no implementados

- Crear el Secret real fuera del repositorio; no subir credenciales al repo.
- Separar usuarios/permisos de base de datos por componente.
- Configurar TLS.
- Definir backups, StorageClass y `reclaimPolicy` del PVC.
- Validar especificaciones de producto (`data_type`, `required` y categoría).
- Evaluar Flyway.
- Reconciliar `develop` con `platform-apis` y resolver `user_email_verified`.
- Kubernetes despliega solo `product-api` y `store-api`; los cambios de ingesta y autenticación están en Compose, no en Kubernetes.
- No hay CI de pruebas.
- Persiste el riesgo de que clientes o integraciones asuman una sola oferta por producto y tienda, sin considerar la URL como parte de la identidad.
