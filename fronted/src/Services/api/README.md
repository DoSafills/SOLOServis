# API de Productos (contenedor separado de la BD)

Este proyecto es **solo la API**. Se conecta a tu contenedor de Postgres que ya existe — no crea uno nuevo.

## Estructura
```
api-productos/
├── docker-compose.yml     # Levanta SOLO la API
├── Dockerfile
├── .env.example
├── sql/
│   ├── schema.sql         # Corre esto una vez contra tu BD existente
│   └── seed.sql           # Datos iniciales, corre después del schema
├── src/                   # Backend (Express + TypeScript + pg)
│   ├── db.ts
│   ├── server.ts
│   └── routes/
│       ├── products.ts
│       ├── services.ts
│       └── stores.ts
├── frontend-src/
│   └── api.ts             # Reemplazo de tu mock en el frontend
└── test-api.sh
```

## Paso 1: Identificar tu contenedor y red de Postgres existente

```bash
docker ps
# anota el NOMBRE de tu contenedor de Postgres, ej: mi_postgres

docker inspect mi_postgres --format '{{json .NetworkSettings.Networks}}'
# esto te muestra el/los nombres de red a los que pertenece, ej: "mired_default"
```

## Paso 2: Crear las tablas y cargar los datos en tu BD existente

```bash
# Copia los scripts SQL dentro del contenedor
docker cp sql/schema.sql mi_postgres:/schema.sql
docker cp sql/seed.sql mi_postgres:/seed.sql

# Ejecútalos (ajusta usuario y nombre de la base de datos)
docker exec -it mi_postgres psql -U mi_usuario -d mi_base_de_datos -f /schema.sql
docker exec -it mi_postgres psql -U mi_usuario -d mi_base_de_datos -f /seed.sql
```

Verifica que quedó cargado:
```bash
docker exec -it mi_postgres psql -U mi_usuario -d mi_base_de_datos -c "SELECT COUNT(*) FROM products;"
# debería devolver 6
```

## Paso 3: Configurar la conexión de la API

```bash
cp .env.example .env
```

Edita `.env`:
```
DATABASE_URL=postgresql://mi_usuario:mi_password@mi_postgres:5432/mi_base_de_datos
PORT=4000
```

El host (`mi_postgres` en el ejemplo) es el **nombre del contenedor**, no `localhost` — así es como Docker resuelve nombres entre contenedores en la misma red.

## Paso 4: Editar docker-compose.yml con el nombre real de tu red

Abre `docker-compose.yml` y reemplaza:
```yaml
networks:
  db_network:
    external: true
    name: CAMBIA_ESTO_por_la_red_de_tu_contenedor_db
```
por el nombre real que obtuviste en el Paso 1 (ej. `mired_default`).

## Paso 5: Levantar la API

```bash
docker compose up -d --build
```

Prueba que responde:
```bash
curl http://localhost:4000/api/health
curl http://localhost:4000/api/products | jq
```

## Alternativa sin Docker para la API
Si prefieres correr la API directo en tu máquina (más rápido para desarrollar) y tu contenedor de Postgres ya publica el puerto 5432 al host:

```bash
cp .env.example .env
# En .env usa: DATABASE_URL=postgresql://usuario:password@localhost:5432/nombre_bd
npm install
npm run dev
```

## Endpoints disponibles
| Método | Ruta | Descripción |
|---|---|---|
| GET | `/api/products` | Lista productos. Filtros: `?category=` `?search=` |
| GET | `/api/products/:id` | Detalle de un producto (ofertas, specs, historial) |
| GET | `/api/services` | Lista servicios. Filtro: `?category=` |
| GET | `/api/services/:id` | Detalle de un servicio |
| GET | `/api/stores` | Lista tiendas |
| GET | `/api/stores/:id` | Detalle de una tienda |

Las respuestas usan el mismo shape que tus tipos `Product`, `Service`, `Store` del frontend.

## Conectar el frontend
Reemplaza tu archivo mock por `frontend-src/api.ts` (mismo patrón de siempre: copia el archivo a tu carpeta de datos). Las funciones puras (`formatPrice`, `getMinPrice`, `getMinOffer`, `getAvailableStoreCount`) son idénticas; lo que cambia es que `products`/`services`/`stores` ahora son funciones async (`getProducts()`, etc.) que hacen `fetch` a la API — necesitarás `useEffect`/`useState` o tu librería de data-fetching donde antes usabas los arreglos directos.

Define en tu frontend:
```
VITE_API_URL=http://localhost:4000/api
```

## Probar que todo funciona
```bash
./test-api.sh
```
