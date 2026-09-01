# PostgreSQL

Este directorio contiene la configuración de Docker Compose para la base de datos PostgreSQL utilizada por SOLOServis en el entorno local de desarrollo.

## Requisitos

* Docker
* Docker Compose

## Configuración

El contenedor de PostgreSQL está configurado en `compose.yml` con los siguientes valores por defecto:

| Variable      | Valor                 |
| ------------- | --------------------- |
| Imagen        | `postgres:18`         |
| Contenedor    | `soloservis_postgres` |
| Base de datos | `soloservis`          |
| Usuario       | `postgres`            |
| Contraseña    | `postgres`            |
| Puerto        | `5432`                |

> Estas credenciales están destinadas únicamente al entorno de desarrollo local.

## Iniciar PostgreSQL

Desde este directorio:

```bash
docker compose up -d
```

Comprobar el estado del contenedor:

```bash
docker compose ps
```

Ver los logs de PostgreSQL:

```bash
docker compose logs -f postgres
```

El servidor estará listo cuando aparezca en los logs:

```text
database system is ready to accept connections
```

## Inicialización de la base de datos

El esquema de la base de datos se encuentra en:

```text
backend/db/migrations/
```

Los siguientes archivos SQL se ejecutan automáticamente cuando PostgreSQL inicializa una base de datos nueva:

```text
001_schema.sql
002_views.sql
003_seed.sql
```

Estos archivos se montan dentro del contenedor en:

```text
/docker-entrypoint-initdb.d/
```

La imagen oficial de PostgreSQL ejecuta los scripts en orden alfabético.

### Importante

Los scripts de inicialización se ejecutan **únicamente cuando el directorio de datos de PostgreSQL está vacío**.

Por lo tanto, modificar alguno de los archivos SQL no hará que PostgreSQL lo vuelva a ejecutar automáticamente sobre una base de datos existente.

## Verificar las tablas

Para listar las tablas creadas:

```bash
docker exec soloservis_postgres \
  psql -U postgres -d soloservis -c "\dt"
```

Para listar las vistas:

```bash
docker exec soloservis_postgres \
  psql -U postgres -d soloservis -c "\dv"
```

Para comprobar los datos insertados por el seed:

```bash
docker exec soloservis_postgres \
  psql -U postgres -d soloservis \
  -c "SELECT * FROM product LIMIT 5;"
```

## Acceder a PostgreSQL

Para abrir una consola `psql` dentro del contenedor:

```bash
docker exec -it soloservis_postgres \
  psql -U postgres -d soloservis
```

Una vez dentro de `psql`, algunos comandos útiles son:

```sql
\dt
\dv
\d product
\q
```

## Reiniciar la base de datos desde cero

Para eliminar completamente la base de datos local y volver a crearla ejecutando los scripts de inicialización:

```bash
docker compose down -v
docker compose up -d
```

> **Advertencia:** `docker compose down -v` elimina el volumen de PostgreSQL y, por lo tanto, todos los datos almacenados en él.

Después de iniciar nuevamente el servicio, se pueden revisar los logs:

```bash
docker compose logs -f postgres
```

Durante la inicialización deberían aparecer los siguientes scripts:

```text
running /docker-entrypoint-initdb.d/001_schema.sql
running /docker-entrypoint-initdb.d/002_views.sql
running /docker-entrypoint-initdb.d/003_seed.sql
```

## Detener PostgreSQL

Para detener el contenedor sin eliminar los datos:

```bash
docker compose down
```

El volumen de PostgreSQL se conservará y podrá utilizarse nuevamente al iniciar el servicio.

## Estructura

```text
SOLOServis2/
├── backend/
│   └── db/
│       └── migrations/
│           ├── 001_schema.sql
│           ├── 002_views.sql
│           ├── 003_seed.sql
│           └── Readme.md
│
└── docker/
    └── postgres/
        ├── compose.yml
        └── README.md
```

## Fuente del esquema

Los archivos SQL ubicados en:

```text
backend/db/migrations/
```

son la fuente principal del esquema de la base de datos.

Docker Compose monta directamente este directorio en:

```text
/docker-entrypoint-initdb.d/
```

para permitir la inicialización automática de una base de datos PostgreSQL nueva.

De esta manera, no es necesario mantener copias duplicadas de los archivos SQL dentro del directorio de Docker.
