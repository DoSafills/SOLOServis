# Cómo probar la base de datos localmente

1. Clona/actualiza el repo y entra a esta carpeta:
   cd backend/db/migrations

2. Levanta PostgreSQL con Docker:
   sudo docker run --name soloservis-db -e POSTGRES_PASSWORD=admin123 -e POSTGRES_DB=soloservis -p 5432:5432 -d postgres:16

3. Copia los scripts al contenedor:
   sudo docker cp 001_schema.sql soloservis-db:/001_schema.sql
   sudo docker cp 002_views.sql soloservis-db:/002_views.sql
   sudo docker cp 003_seed.sql soloservis-db:/003_seed.sql

4. Ejecútalos en orden:
   sudo docker exec -it soloservis-db psql -U postgres -d soloservis -f /001_schema.sql
   sudo docker exec -it soloservis-db psql -U postgres -d soloservis -f /002_views.sql
   sudo docker exec -it soloservis-db psql -U postgres -d soloservis -f /003_seed.sql

5. Verifica:
   sudo docker exec -it soloservis-db psql -U postgres -d soloservis -c "SELECT * FROM product;"