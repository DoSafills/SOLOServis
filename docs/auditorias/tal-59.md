# TAL-59 — Auditoría del esquema, consultas y migraciones

**Ejecución:** 2026-10-23

**Alcance:** revisión estática de las siete migraciones y las tres fuentes sqlc;
ejecución aislada en PostgreSQL 16.15. No se consultó ni modificó ninguna base
de datos del proyecto.

## Alcance y método reproducible

1. Se creó `tal_audit` desde cero en el contenedor independiente
   `solo_tal59_tmp_20261023`, imagen `postgres:16`, con
   `--tmpfs /var/lib/postgresql/data:rw,size=512m`, sin volumen ni puerto
   publicado. El comando ejecutado fue:

   ```powershell
   docker run -d --name solo_tal59_tmp_20261023 `
     --tmpfs /var/lib/postgresql/data:rw,size=512m `
     -e POSTGRES_PASSWORD=disposable-audit-password `
     -e POSTGRES_DB=tal_audit postgres:16
   docker exec solo_tal59_tmp_20261023 pg_isready -U postgres -d tal_audit
   docker exec solo_tal59_tmp_20261023 psql -X -U postgres -d tal_audit `
     -c "SELECT current_database(), version();"
   ```

   La salida real informó `tal_audit` y PostgreSQL 16.15 (Debian
   16.15-1.pgdg13+2).

2. Para cada archivo, en el orden de nombre `001` a `007`, se ejecutó:

   ```powershell
   docker cp backend\database\migrations\<archivo>.sql `
     solo_tal59_tmp_20261023:/tmp/migration.sql
   docker exec solo_tal59_tmp_20261023 psql -X -v ON_ERROR_STOP=1 `
     -U postgres -d tal_audit -f /tmp/migration.sql
   ```

   Todos los comandos terminaron con `exit=0`; el resultado por archivo se
   registra abajo. La ejecución completa de Q01–Q20 está en
   [resultados_reejecucion.txt](./dia2/resultados_reejecucion.txt).

3. Se cargaron filas sintéticas y acotadas desde
   [fixtures_reejecucion.sql](./dia2/fixtures_reejecucion.sql) y se ejecutó el
   archivo de diagnóstico completo:

   ```powershell
   docker cp docs\auditorias\dia2\fixtures_reejecucion.sql `
     solo_tal59_tmp_20261023:/tmp/fixtures.sql
   docker exec solo_tal59_tmp_20261023 psql -X -v ON_ERROR_STOP=1 `
     -U postgres -d tal_audit -f /tmp/fixtures.sql
   docker cp docs\auditorias\dia2\consultas.sql `
     solo_tal59_tmp_20261023:/tmp/consultas.sql
   docker exec solo_tal59_tmp_20261023 psql -X -A -v ON_ERROR_STOP=1 `
     -U postgres -d tal_audit -f /tmp/consultas.sql
   ```

   La carga y `consultas.sql` terminaron con `exit=0`. Q01–Q20 aparecen en la
   salida guardada en formato psql no alineado (`-A`); por ejemplo, Q01 detectó
   dos ofertas sintéticas de igual
   URL en la tienda 1, Q13 detectó dos ofertas sin historial y Q20 encontró
   un grupo duplicado para `(store_id, product_url)`. Q19 contó 3 productos,
   4 ofertas, 2 tiendas, 3 filas de historial y 3 valores de especificación.
   Estos conteos describen exclusivamente el fixture, no datos reales.

4. Se ejecutó `EXPLAIN (ANALYZE, BUFFERS)` en 12 consultas representativas de
   los tres archivos sqlc. Las salidas están en
   [explain_reejecucion.txt](./dia2/explain_reejecucion.txt). La muestra es
   demasiado pequeña para inferir rendimiento de producción: PostgreSQL elige
   varios `Seq Scan` aunque existan índices utilizables.

5. Se obtuvo el esquema final con:

   ```powershell
   docker exec solo_tal59_tmp_20261023 pg_dump --schema-only `
     --no-owner --no-privileges -U postgres -d tal_audit
   ```

   Terminó con `exit=0` y produjo 3.022 líneas de DDL. Además se consultaron
   `pg_constraint` y `pg_indexes` en `tal_audit`; las definiciones reales
   relevantes se resumen en «Comparación del esquema».

La fuente de migraciones es
[backend/database/migrations/](../../backend/database/migrations/); sqlc apunta
a `database/migrations` y `database/queries` en
[sqlc.yaml](../../backend/sqlc.yaml#L5-L6). El alcance de esta ejecución es el
esquema reconstruido, no la configuración ni el contenido de una instancia
desplegada.

## Hallazgos por migración

| Archivo | Cambio y resultado de ejecución | Idempotencia | Riesgos, reversibilidad y bloqueo |
|---|---|---|---|
| [001_schema.sql](../../backend/database/migrations/001_schema.sql#L1-L20) | `exit=0`. Crea extensión, enum, tablas del catálogo, APIs base y sus índices; incluye categorías, productos, tiendas, ofertas e historial. Las entidades se declaran, por ejemplo, en líneas 46–70 y 104–143; los índices, en 457–469. | **No.** La extensión sí usa `IF NOT EXISTS` (L8), pero el enum y las tablas se crean sin esa guarda (L16, L22 en adelante). | No tiene un `down` reversible incorporado. En una base existente, el DDL puede esperar por locks y fallar parcialmente si se ejecuta sin transacción. El borrado de una categoría con productos se restringe; borrar producto o tienda elimina ofertas y, por cascada, historial (L59, L120–121, L139). |
| [002_views.sql](../../backend/database/migrations/002_views.sql#L1-L24) | `exit=0`. Crea/reemplaza las vistas `product_rating_summary` y `service_rating_summary`, agregando rating y conteo desde las reseñas. | **Sí, para la misma definición:** usa `CREATE OR REPLACE VIEW`. | No cambia filas base y se puede revertir con `DROP VIEW`, aunque la vista depende de tablas y cambios incompatibles de columnas pueden bloquear la sustitución. El comentario de `001_schema.sql` L65 dice «ver 003_views.sql», pero el archivo real es `002_views.sql`. |
| [003_platform_apis.sql](../../backend/database/migrations/003_platform_apis.sql#L1-L69) | `exit=0`. Agrega búsqueda guardada, sesión, carrito, reseñas de tienda, notificaciones e ingesta, más índices. | **Parcialmente sí:** todas las tablas e índices usan `IF NOT EXISTS`; repetir sobre una instalación idéntica no vuelve a crearlos. No repara una tabla preexistente de estructura distinta. | El borrado de cuentas elimina sesiones/carritos/notificaciones/reseñas vinculados por `ON DELETE CASCADE` (L3, L13, L23, L36, L48). La reversión requiere borrar estructuras y datos; no hay `down`. |
| [004_service_cart.sql](../../backend/database/migrations/004_service_cart.sql#L1-L11) | `exit=0`. Permite `NULL` en producto/oferta del carrito, agrega FK `service_offer_id`, una restricción XOR de producto/servicio y un índice parcial. | **No.** El `ALTER TABLE` agrega columna y constraint sin `IF NOT EXISTS` (L1–7); el índice sí tiene guarda (L10). | Toca `cart_item` y puede tomar lock; si la tabla contiene combinaciones que incumplen la nueva CHECK, la migración falla después de modificar columnas si no se aplica en transacción. Borrar oferta de servicio borra el carrito asociado por cascada. El rollback de la columna perdería las referencias a ofertas de servicio. |
| [005_normalize_prices_to_clp.sql](../../backend/database/migrations/005_normalize_prices_to_clp.sql#L1-L42) | `exit=0`. Define funciones y triggers que fuerzan `currency='CLP'` en ofertas de productos/servicios; ejecuta dos `UPDATE`. En esta base recién creada, ambos reportaron `UPDATE 0`. | **Sí, en resultado:** funciones `OR REPLACE`, `DROP TRIGGER IF EXISTS` seguido de `CREATE TRIGGER`; otra ejecución deja la etiqueta en CLP. | El `UPDATE` puede bloquear/reescribir muchas filas en una base poblada. Conserva el número, no convierte el importe; al sobrescribir la moneda original no es posible reconstruirla solo desde esa fila. La reversión del trigger no restaura las etiquetas anteriores. |
| [006_offer_identity_and_url_index.sql](../../backend/database/migrations/006_offer_identity_and_url_index.sql#L1-L10) | `exit=0`. Elimina la constraint previa `(product_id, store_id)` si existe y crea índice único sobre `(product_id, store_id, COALESCE(product_url, ''))`, además del índice parcial de búsqueda por tienda/URL. | **Sí, para nombres y definiciones esperados:** `DROP ... IF EXISTS` e índices `IF NOT EXISTS`; una relación de igual nombre con definición incorrecta no se corrige automáticamente. | Crear índices únicos puede bloquear escrituras y fallar si hay duplicados. Si falla tras eliminar la constraint anterior y no hay transacción envolvente, queda un estado parcial. El cambio de identidad afecta a seeds y consumidores que asumían una sola oferta por producto/tienda. La expresión solo reemplaza `NULL` por `''`; no recorta espacios, no cambia mayúsculas ni elimina parámetros de tracking. |
| [007_store_and_category_name_unique.sql](../../backend/database/migrations/007_store_and_category_name_unique.sql#L1-L42) | `exit=0`. Valida duplicados y crea índices únicos para tienda `LOWER(BTRIM(name))` y categoría `(COALESCE(parent_category_id,0), LOWER(BTRIM(name)))`. | **Sí, para un estado válido:** chequeos repetibles e índices `IF NOT EXISTS`; no es exitosa si los datos existentes violan la nueva unicidad. | No borra filas, pero puede detenerse ante duplicados. Los índices únicos toman locks y el segundo índice puede fallar después de que el de tiendas ya se creó si no hay transacción de migración. Son reversibles eliminando los índices; los nombres existentes no se normalizan en los datos. |

La cardinalidad de `product_price_history` respecto de oferta es uno-a-muchos,
y su FK es `ON DELETE CASCADE` en
[001_schema.sql](../../backend/database/migrations/001_schema.sql#L137-L143).
Por ello, eliminar físicamente una oferta **borra sus puntos históricos**; borrar
un producto o una tienda también llega a ese historial mediante la cascada de
`product_offer`. Es un riesgo alto para **TAL-65** si ese ticket requiere
conservar trazabilidad al retirar ofertas.

## Hallazgos por consulta sqlc

Los nombres de consulta y tablas se verificaron en
[products.sql](../../backend/database/queries/products.sql),
[stores.sql](../../backend/database/queries/stores.sql) y
[services.sql](../../backend/database/queries/services.sql). Las consultas de
lectura referenciadas en la tabla se ejecutaron con parámetros sintéticos
equivalentes. Índices citados son los comprobados en `pg_indexes` tras las siete
migraciones. Los planes con las consultas completas están en
[explain_reejecucion.txt](./dia2/explain_reejecucion.txt).

### `products.sql` — 13 consultas

| Consulta (líneas) | Tablas/vistas | Índices que la respaldan | Hallazgo / riesgo |
|---|---|---|---|
| `GetProductByID` (L1–15) | `product` | PK `product_pkey` | Búsqueda por PK; no filtra `active`, decisión de visibilidad no verificable en este SQL. |
| `GetProductByPublicID` (L18–32) | `product` | UNIQUE `product_public_id_key` | Búsqueda selectiva por UUID; tampoco filtra `active`. |
| `GetProductDetailByPublicID` (L36–57) | `product`, `brand`, `product_category`, vista `product_rating_summary` (`product_review`) | UNIQUE de UUID, PK de brand/categoría; el índice de reviews es `idx_product_review_product` | Join único; no filtra `p.active`. La vista agrega reseñas en lectura. |
| `ListProducts` (L60–82) | `product`, `brand`, `product_category`, `product_rating_summary` (`product_review`) | PKs de joins; `idx_product_category`/`idx_product_brand` respaldan las FKs, no el filtro `active` | Sin límite/paginación; ordena por `p.id` (PK), pero no hay índice específico parcial para activos. `EXPLAIN` con fixture pequeño eligió scans secuenciales. |
| `ListProductOffers` (L85–104) | `product_offer`, `store` | `idx_product_offer_product` o prefijo de `uq_product_offer_product_store_url`; PK store | `EXPLAIN` eligió el índice único de expresión para producto y luego ordenó por precio. No existe índice `(product_id, price)`; no filtra `available`. |
| `ListProductImages` (L107–118) | `product_image` | Solo PK `id`; no hay índice por `product_id` | Filtro por producto/activo y orden por `sort_order,id` sin índice compuesto; `EXPLAIN` mostró `Seq Scan` y `Sort`. |
| `CreateProduct` (L121–150) | `product` | PK/UUID/SKU únicos; PK de categoría/brand para validar FKs | Inserción con restricciones; no aplica `active` porque es una mutación, no búsqueda. |
| `UpdateProduct` (L152–175) | `product` | PK `id`, unique SKU; índices de FKs | `UPDATE` por PK; no filtra `active`. |
| `DeactivateProduct` (L177–192) | `product` | PK `id` | Actualización por PK; no es borrado físico, así que no dispara cascadas de ofertas/historial. |
| `UpdateProductByPublicID` (L195–218) | `product` | UNIQUE `product_public_id_key`, unique SKU | Filtra también `active=true`; indexa UUID. |
| `DeactivateProductByPublicID` (L221–237) | `product` | UNIQUE `product_public_id_key` | Filtra también `active=true`; indexa UUID. |
| `ListProductSpecifications` (L241–249) | `product_specification_value`, `product_category_specification` | UNIQUE `(product_id,specification_id)` da prefijo por producto; PK de especificación | `EXPLAIN` usó ambos índices y ordenó por `display_order,id`; no hay índice de orden que cubra esa combinación. |
| `ListProductPriceHistory` (L252–263) | `product_price_history`, `product_offer` | `idx_product_price_history_offer`, `idx_product_offer_product`/índice único de oferta | `EXPLAIN` hizo scan de la tabla de historial en el fixture diminuto. El índice por `product_offer_id` ayuda al join, pero no cubre orden global por `recorded_at,id`; sin límite puede devolver historial completo. |

**N+1 confirmado fuera del SQL aislado:** [products/handler.go](../../backend/internal/products/handler.go#L79-L110)
itera los productos y, dentro del loop, consulta imágenes, ofertas y
especificaciones (tres lecturas por producto además de la lista). Esto hace que
el endpoint de listado sea `1 + 3N` consultas. El detalle de producto ejecuta
cuatro consultas adicionales por producto, pero una sola entidad por request.

### `stores.sql` — 4 consultas

| Consulta (líneas) | Tablas | Índices que la respaldan | Hallazgo / riesgo |
|---|---|---|---|
| `ListStores` (L1–32) | `store`, `product_offer` | PK `store_pkey`; `idx_product_offer_store` | Filtra tiendas activas y ofertas disponibles; agrupa y cuenta productos distintos, sin paginación. No une `product`, por lo que también puede contar ofertas disponibles de productos inactivos. El índice por tienda no cubre conjuntamente `available` y `product_id`. |
| `GetStoreByID` (L34–68) | `store`, `product_offer`, `product` | PK store/product; `idx_product_offer_store` | Hallazgo de corrección: el `LEFT JOIN product p ... AND p.active=true` no participa en `COUNT(DISTINCT po.product_id)`, que cuenta el ID de la oferta aunque el producto esté inactivo. Ver L47–55. |
| `ListStoreProducts` (L71–118) | `product_offer`, `product`, `brand`, `product_category`, `product_image` | `idx_product_offer_store`; PKs de producto/brand/categoría; `product_image` no tiene índice por producto | Sin paginación; orden por `p.id`. La imagen usa subconsulta correlacionada con scan/sort sobre `product_image` por producto; `EXPLAIN` mostró `SubPlan` y `Seq Scan`. |
| `ListStoreLocations` (L121–138) | `store_location`, `location` | PK location; no existe índice en `store_location(store_id)` | `EXPLAIN` mostró `Seq Scan` de store_location. El filtro de tienda y activo puede degradarse con volumen. |

### `services.sql` — 5 consultas

| Consulta (líneas) | Tablas/vistas | Índices que la respaldan | Hallazgo / riesgo |
|---|---|---|---|
| `ListServices` (L1–24) | `service`, `service_category` (padre e hija), `service_rating_summary` (`service_review`) | PKs de joins; `idx_service_category` para FK, `idx_service_review_service` para agregado | No pagina resultados; `active` y orden `id` no tienen índice específico parcial. En fixture pequeño se eligieron scans secuenciales. |
| `GetServiceByPublicID` (L26–49) | `service`, categorías, `service_rating_summary` (`service_review`) | UNIQUE `service_public_id_key`; PKs de joins | Lookup selectivo; sí filtra `active=true`. |
| `ListServiceSpecifications` (L51–59) | `service_specification_value`, `service_category_specification` | UNIQUE `(service_id,specification_id)` y PK de especificación | Acceso indexado por servicio; sort por `display_order,id` no queda cubierto. |
| `ListServiceOffers` (L61–83) | `service_offer`, `provider` | UNIQUE `(service_id,provider_id)`/`idx_service_offer_service`; PK provider | Filtra disponibilidad y provider activo, pero ordena por precio sin índice compuesto. |
| `ListServicePriceHistory` (L85–95) | `service_price_history`, `service_offer` | `idx_service_offer_service`, `idx_service_price_history_offer` | El plan incluye ambos índices; el scan del índice de historial no se ejecutó porque el fixture no contiene ofertas de servicio. El orden por fecha/id requiere sort y no hay índice temporal por historial. |

**N+1 confirmado:** [services/handler.go](../../backend/internal/services/handler.go#L31-L50) consulta ofertas,
especificaciones e historial dentro del loop de cada servicio (3N lecturas más
la lista). La llamada de detalle L76–88 hace tres consultas para un servicio.
El [handler de tienda](../../backend/internal/stores/handler.go#L35-L65) llama
lista de productos y ubicaciones una vez cada una por request; no recorre
productos para consultar cada uno.

## Resolución de la contradicción documental

- El `git blame` de [plan_pruebas.md](./dia2/plan_pruebas.md#L41-L49) atribuye
  las líneas 41–42 al commit `5aa528f` del **18-10-2026 20:03 -03:00**: eran un
  estado preliminar que decía que Docker no estaba disponible y que el paso se
  omitió.
- [decisiones_v2.md](./dia2/decisiones_v2.md#L113-L115), añadido en el commit
  `6f37a84` del **20-10-2026**, y
  [resultados_dia2.txt](./dia2/resultados_dia2.txt#L21-L28) reportan que los
  casos SQL sí se ejecutaron el 18-10-2026.
- Se corrigió el estado preliminar en las líneas 41–49 de `plan_pruebas.md`:
  ahora identifica esas frases como obsoletas y apunta a los registros
  posteriores. La reejecución de este informe en 23-10-2026 es evidencia nueva,
  real y reproducible; **no prueba de forma independiente la fecha ni los
  comandos históricos del 18-10-2026**. Para el estado final del Día 2, la
  versión de decisiones y resultados es la más reciente y consistente; la
  ejecución histórica exacta queda respaldada solo por esos documentos.

## Comparación del esquema final con el DDL y documentos

El `pg_dump --schema-only` de la base recién migrada confirmó las cinco tablas,
sus PK/FK y los índices esperados. `pg_constraint` mostró:

- `product.category_id → product_category.id ON DELETE RESTRICT`;
- `product_category.parent_category_id → product_category.id ON DELETE SET NULL`;
- `product_offer.product_id → product.id ON DELETE CASCADE`;
- `product_offer.store_id → store.id ON DELETE CASCADE`;
- `product_price_history.product_offer_id → product_offer.id ON DELETE CASCADE`.

`pg_indexes` confirmó `uq_product_offer_product_store_url` sobre
`(product_id, store_id, COALESCE(product_url, ''))`,
`idx_product_offer_store_url_nonempty`,
`uq_store_name_normalized` y `uq_product_category_parent_name`. Por tanto, el
esquema final **no** conserva el `UNIQUE (product_id, store_id)` inicial de
[001_schema.sql](../../backend/database/migrations/001_schema.sql#L118-L143):
lo quita `006` y lo sustituye por unicidad de URL exacta por producto y tienda,
con `NULL` tratado como cadena vacía. No es normalización de la URL completa.

El PDF [MER normalizado.pdf](../database/MER%20normalizado.pdf) se pudo leer con
`pypdf` en un entorno virtual temporal. El texto extraído de su página 1 incluye
`PRODUCT`, `PRODUCT_CATEGORY`, `STORE`, `PRODUCT_OFFER` y
`PRODUCT_PRICE_HISTORY`, con IDs PK/FK correspondientes y marcas de relación
`1:N`; por lo tanto coincide en las relaciones principales representadas aquí.
El diagrama PDF marca `derived_average_rating` y `derived_review_count` como
“remove from storage”; el esquema actual no almacena esas columnas y las
calcula mediante la vista de `002_views.sql`, así que coincide con esa decisión.
El PDF no especifica el nuevo índice único por URL de `006`, ni su expresión
`COALESCE`; esa regla solo queda definida en el DDL vigente.

## Riesgos priorizados

| Prioridad | Riesgo | Evidencia |
|---|---|---|
| **Alto** | Borrar físicamente una oferta elimina todo su historial; borrar producto o tienda también lo elimina indirectamente. Riesgo de pérdida de trazabilidad relevante para TAL-65. | FK de historial y ofertas en [001_schema.sql](../../backend/database/migrations/001_schema.sql#L118-L143); FK confirmada por `pg_constraint`. |
| **Alto** | El listado de productos ejecuta tres queries adicionales por producto; el listado de servicios hace otras tres por servicio. El coste de round trips crece linealmente con resultados. | [products/handler.go](../../backend/internal/products/handler.go#L79-L110), [services/handler.go](../../backend/internal/services/handler.go#L31-L50). |
| **Medio** | Migraciones `001` y `004` no son repetibles; `006` y `007` pueden dejar cambios parciales si se ejecutan sin una transacción y falla una operación posterior. | DDL citado por migración en la tabla; `004` L1–10, `006` L2–9, `007` L1–42. |
| **Medio** | Listados sin `LIMIT`/paginación y ordenamientos sin índice compuesto; faltan índices de acceso por `product_image.product_id` y `store_location.store_id`. Los planes de fixture no predicen producción. | SQL y planes referenciados en las tablas; `pg_indexes` no muestra esos índices. |
| **Medio** | La migración de CLP reemplaza etiquetas de moneda sin convertir el importe; una vez sobrescrita la moneda original no se recupera desde esas filas. | [005_normalize_prices_to_clp.sql](../../backend/database/migrations/005_normalize_prices_to_clp.sql#L1-L42). |
| **Bajo** | `ListStores` cuenta ofertas disponibles sin filtrar si el producto está activo; `GetStoreByID` une solo productos activos, pero su `COUNT(po.product_id)` aún cuenta ofertas cuyo producto está inactivo. | [stores.sql](../../backend/database/queries/stores.sql#L14-L19), [L47-L55](../../backend/database/queries/stores.sql#L47-L55). |
| **Bajo** | El comentario de `001_schema.sql` cita `003_views.sql`, aunque la migración actual de vistas es `002_views.sql`. | [001_schema.sql](../../backend/database/migrations/001_schema.sql#L64-L66), [002_views.sql](../../backend/database/migrations/002_views.sql). |

## Conclusión

La secuencia `001`–`007` se aplicó sin errores a una base PostgreSQL desechable
y vacía; las 20 consultas de diagnóstico y los 12 `EXPLAIN` terminaron sin
error con un fixture sintético. Se verificó el resultado con `pg_dump
--schema-only`, `pg_constraint`, `pg_indexes` y lectura del PDF MER. La auditoría
no acredita el estado de ninguna base real o desplegada ni extrapola tiempos de
ejecución de una muestra diminuta.
