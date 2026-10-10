# Cómo probar la base de datos localmente

1. Desde la raiz del proyecto, levanta PostgreSQL y aplica automaticamente todas las migraciones a un volumen nuevo:
   docker compose -f docker/compose.yml up -d postgres

2. Para una base que ya existe, aplica la migracion de catalogo y la carga unificada de servicios:
   docker compose -f docker/compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U postgres -d soloservis -f /docker-entrypoint-initdb.d/005_demo_catalog.sql
   docker compose -f docker/compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U postgres -d soloservis -f /docker-entrypoint-initdb.d/006_demo_catalog_services.sql

3. Verifica productos, tiendas y el límite de servicios activos por microcategoría:
   docker compose -f docker/compose.yml exec -T postgres psql -U postgres -d soloservis -c "SELECT COUNT(*) FROM product WHERE sku LIKE 'DEMO-%';"
   docker compose -f docker/compose.yml exec -T postgres psql -U postgres -d soloservis -c "SELECT name FROM store ORDER BY name;"
   docker compose -f docker/compose.yml exec -T postgres psql -U postgres -d soloservis -c "SELECT parent.name AS categoria, category.name AS microcategoria, COUNT(service.id) AS servicios_activos FROM service_category category JOIN service_category parent ON parent.id = category.parent_category_id LEFT JOIN service ON service.category_id = category.id AND service.active GROUP BY parent.name, category.name HAVING COUNT(service.id) > 3;"