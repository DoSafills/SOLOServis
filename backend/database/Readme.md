# Cómo probar la base de datos localmente

1. Desde la raiz del proyecto, levanta PostgreSQL y aplica automaticamente todas las migraciones a un volumen nuevo:
   docker compose -f docker/compose.yml up -d postgres

2. Para una base que ya existe, aplica solo la nueva migracion de catalogo:
   docker compose -f docker/compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U postgres -d soloservis -f /docker-entrypoint-initdb.d/005_demo_catalog.sql

3. Verifica productos y tiendas:
   docker compose -f docker/compose.yml exec -T postgres psql -U postgres -d soloservis -c "SELECT COUNT(*) FROM product WHERE sku LIKE 'DEMO-%';"
   docker compose -f docker/compose.yml exec -T postgres psql -U postgres -d soloservis -c "SELECT name FROM store ORDER BY name;"