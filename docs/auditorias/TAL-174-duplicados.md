# TAL-174 — Control de datos duplicados

| Campo | Valor |
| --- | --- |
| Fecha | 2026-11-16 |
| Alcance | `backend/database/migration_order.txt`, guardia de migraciones en Go |
| Estado previo | Tres capas de control; dos funcionaban y la tercera nunca se ejecutaba |

## 1. Las tres capas

Un producto puede duplicarse en tres momentos distintos, y cada uno tiene su
control:

| Capa | Dónde | Qué evita | Estado |
| --- | --- | --- | --- |
| 1. Dentro de una corrida | `scraper/src/scraper/pipeline.py` → `_dedupe()` | El mismo `externalId` extraído dos veces (listado + detalle, paginación repetida) | Funciona. Test: `test_pipeline_paginates_dedupes_and_persists` |
| 2. Al ingresar | `backend/internal/ingestion` → `FindProduct` | Crear un producto nuevo cuando ya existe: por SKU, por marca + modelo o por tienda + URL normalizada (TAL-169) | Funciona. Test: `TestAddItems_SinSkuNiModeloSeReconocePorTiendaYURL` |
| 3. Los que ya existen | `021_dedupe_scraped_products.sql` | Los duplicados que la capa 2 dejó antes de existir | **No se ejecutaba** |

Además, la tabla `product_offer` tiene `UNIQUE (product_id, store_id)`: el mismo
producto no puede tener dos ofertas de la misma tienda.

## 2. El hueco

`021_dedupe_scraped_products.sql` estaba en `backend/database/migrations/` pero
**no figuraba en `migration_order.txt`**. El runner aplica exactamente lo que
dice ese archivo, así que la limpieza nunca corrió: los duplicados históricos
siguen en la base y cada deploy los ignoraba en silencio.

El índice tenía 24 entradas para 25 archivos.

## 3. Cambios

| Archivo | Cambio |
| --- | --- |
| `backend/database/migration_order.txt` | Agrega `021_dedupe_scraped_products.sql` al final, con el motivo escrito al lado |
| `backend/internal/database/migrations_test.go` | Dos pruebas que convierten el olvido en fallo de CI |

### Las pruebas

| Test | Qué fija |
| --- | --- |
| `TestMigrationOrderListsEveryFileOnce` | Cada `.sql` del directorio está en el índice, y cada entrada del índice existe. También rechaza una entrada duplicada |
| `TestDedupeCleanupRunsAfterTheSchemaItNeeds` | La limpieza va después de `009` (de donde toma `normalize_product_offer_url`) y de `018`; no exige ser la última |

La guardia se probó quitando la línea a mano: el test falla con
`existe en ../../database/migrations pero no está en migration_order.txt: nunca
se aplicaría`. Restaurada, pasa.

## 4. Qué hace la migración

Es de TAL-169 y se aplica sin cambios; se documenta para que se sepa qué se
está ejecutando:

- Sólo toca productos **sin SKU y sin modelo** con **exactamente una oferta**.
- Agrupa por `store_id` + URL de la oferta sin parámetros ni fragmento.
- Conserva el producto más antiguo y le pasa el precio y la disponibilidad más
  recientes de la copia.
- Traspasa el historial de precios, omitiendo registros que ya existían.
- **No borra** una copia que tenga datos de usuario: seguimiento, reseña,
  carrito o comparación.
- Va en un solo bloque `DO`: se aplica completa o no cambia nada. Es
  idempotente: sin duplicados no hace nada.

## 5. Verificación

```text
$ go build ./... && go vet ./... && go test ./...
ok  	github.com/DoSafills/SOLOServis/backend/internal/database

$ go test ./internal/database/ -run "Migration|Dedupe" -v
--- PASS: TestMigrationOrderListsEveryFileOnce
--- PASS: TestDedupeCleanupRunsAfterTheSchemaItNeeds
```

## 6. Frontera con otras tareas

| Tarea | Dueño | Relación |
| --- | --- | --- |
| TAL-169 | Marcelo | Escribió la migración y la identidad por URL; aquí sólo se hace que se ejecute |
| TAL-227 | Cristian | Normalización de productos. No se tocó: es su archivo y su alcance |
| TAL-228 | Cristian | Identificación de productos equivalentes. Criterio de negocio distinto: equivalencia entre productos, no duplicados de la misma oferta |

Esta tarea **no** añade lógica de deduplicación nueva: la que existe es
suficiente y ya está probada. El defecto era de ejecución, no de diseño.

## 7. Pendiente y riesgos

| Tema | Estado |
| --- | --- |
| Aplicar la migración en el clúster | Requiere redeploy; la Fase 5 del `deploy.sh` la aplicará. No se ejecutó aquí |
| Duplicados con datos de usuario | No se borran por diseño: quedan y necesitan revisión manual si aparecen |
| Evidencia de cuántos duplicados había | No se pudo medir sin acceso a PostgreSQL. El `RAISE NOTICE` de la migración lo dirá en el deploy |
| Regresión del índice | Cerrada por `TestMigrationOrderListsEveryFileOnce` |

## 8. Cómo reproducir

```bash
cd backend
go test ./internal/database/ -run "Migration|Dedupe" -v
grep -c '^[^#]' database/migration_order.txt   # 25
ls database/migrations/*.sql | wc -l           # 25
```
