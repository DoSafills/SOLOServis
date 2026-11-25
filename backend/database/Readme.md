# Cómo probar la base de datos localmente

1. Desde la raíz del proyecto, levanta PostgreSQL y aplica automáticamente las migraciones y seeds en el orden de `backend/database/migration_order.txt` a un volumen nuevo:
   docker compose -f docker/compose.yml up -d postgres

2. Para una base versionada existente, aplica las versiones pendientes mediante el mismo runner:
   docker compose -f docker/compose.yml exec -T postgres bash /docker-entrypoint-initdb.d/001_migrations.sh
   Las bases antiguas sin tabla `schema_migrations` se rechazan para evitar reaplicar
   cambios a ciegas; antes de migrarlas se debe preparar y verificar un baseline.

3. Verifica que no queden productos del seed (TAL-241 los borra; los del scraper no traen SKU), las tiendas y el límite de servicios activos por microcategoría:
   docker compose -f docker/compose.yml exec -T postgres psql -U postgres -d soloservis -c "SELECT COUNT(*) FROM product WHERE sku IS NOT NULL;"
   docker compose -f docker/compose.yml exec -T postgres psql -U postgres -d soloservis -c "SELECT name FROM store ORDER BY name;"
   docker compose -f docker/compose.yml exec -T postgres psql -U postgres -d soloservis -c "SELECT parent.name AS categoria, category.name AS microcategoria, COUNT(service.id) AS servicios_activos FROM service_category category JOIN service_category parent ON parent.id = category.parent_category_id LEFT JOIN service ON service.category_id = category.id AND service.active GROUP BY parent.name, category.name HAVING COUNT(service.id) > 3;"

## Bases sin el runner (TAL-241)

`scripts/TAL-241-neon.sql` deja una base que no se migra con el runner (por ejemplo, la de pruebas en Neon) igual que tras 044 y 045: borra los productos, usuarios y marcas del seed y repara las tildes cp850. Primero revisa, en solo lectura, y después aplica en una transacción:

   psql "$DATABASE_URL" -f backend/database/scripts/TAL-241-neon.sql
   psql "$DATABASE_URL" -v aplicar=1 -f backend/database/scripts/TAL-241-neon.sql

Si cambias 044 o 045, copia su contenido entre los marcadores `-- >>>` y `-- <<<` del script y actualiza su checksum (`tr -d '\r' < archivo | sha256sum`): `go test ./internal/database/` falla si el script quedó desactualizado.
