# Cómo probar la base de datos localmente

1. Desde la raíz del proyecto, levanta PostgreSQL y aplica automáticamente las migraciones y seeds en el orden de `backend/database/migration_order.txt` a un volumen nuevo:
   docker compose -f docker/compose.yml up -d postgres

2. Para una base versionada existente, aplica las versiones pendientes mediante el mismo runner:
   docker compose -f docker/compose.yml exec -T postgres bash /docker-entrypoint-initdb.d/001_migrations.sh
   Las bases antiguas sin tabla `schema_migrations` se rechazan para evitar reaplicar
   cambios a ciegas; antes de migrarlas se debe preparar y verificar un baseline.

3. Verifica productos, tiendas y el límite de servicios activos por microcategoría:
   docker compose -f docker/compose.yml exec -T postgres psql -U postgres -d soloservis -c "SELECT COUNT(*) FROM product WHERE sku LIKE 'DEMO-%';"
   docker compose -f docker/compose.yml exec -T postgres psql -U postgres -d soloservis -c "SELECT name FROM store ORDER BY name;"
   docker compose -f docker/compose.yml exec -T postgres psql -U postgres -d soloservis -c "SELECT parent.name AS categoria, category.name AS microcategoria, COUNT(service.id) AS servicios_activos FROM service_category category JOIN service_category parent ON parent.id = category.parent_category_id LEFT JOIN service ON service.category_id = category.id AND service.active GROUP BY parent.name, category.name HAVING COUNT(service.id) > 3;"

# Cambiar entre PostgreSQL local y Neon

El interruptor cambia únicamente la conexión de las APIs. No copia ni sincroniza datos entre las bases.

```powershell
# PostgreSQL Docker local
.\scripts\switch-database.ps1 postgres

# Neon
# Primero configura DATABASE_URL en backend/.env.
.\scripts\switch-database.ps1 neon
```

Después del cambio, las sesiones que existían en la base anterior dejan de ser válidas. Borra la clave `soloservice.auth-session` desde la consola del navegador e inicia sesión nuevamente.

Antes de usar Neon, aplica las migraciones a ese destino. No se ejecutan automáticamente al cambiar:
aplica los archivos de `backend/database/migration_order.txt` en ese orden con `psql` y la URL de Neon,
verificando primero que no exista ya el esquema.