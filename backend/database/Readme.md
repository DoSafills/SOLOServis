# Cómo probar la base de datos localmente

1. Desde la raiz del proyecto, levanta PostgreSQL y aplica automaticamente todas las migraciones a un volumen nuevo:
   docker compose -f docker/compose.yml up -d postgres

2. Para una base que ya existe, aplica las cargas adicionales de servicios:
   docker compose -f docker/compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U postgres -d soloservis -f /docker-entrypoint-initdb.d/006_seed_servicios_fibra.sql
   docker compose -f docker/compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U postgres -d soloservis -f /docker-entrypoint-initdb.d/007_seed_security_education_services.sql

3. Verifica productos, tiendas y servicios:
   docker compose -f docker/compose.yml exec -T postgres psql -U postgres -d soloservis -c "SELECT COUNT(*) FROM product WHERE sku LIKE 'DEMO-%';"
   docker compose -f docker/compose.yml exec -T postgres psql -U postgres -d soloservis -c "SELECT name FROM store ORDER BY name;"
   docker compose -f docker/compose.yml exec -T postgres psql -U postgres -d soloservis -c "SELECT category.name AS category, COUNT(*) FROM service JOIN service_category category ON category.id = service.category_id GROUP BY category.name ORDER BY category.name;"

Los precios cargados por `006_seed_servicios_fibra.sql` son referenciales según
la ficha proporcionada y deben verificarse con cada proveedor. Las velocidades
de subida no confirmadas se ingresan como estimaciones simétricas y quedan
marcadas explícitamente como no verificadas. Las imágenes, URL de contratación,
costos de instalación y contratos no informados se dejan vacíos.

La migración `007_seed_security_education_services.sql` agrega servicios VPN,
antivirus y educación online. Sus precios y algunas especificaciones son
referenciales; las monedas y frecuencias originales se conservan (EUR/USD,
mensual/anual/pago único), sin convertirlas a CLP.