# Scraper — mapeo de tiendas, volúmenes y tablas

Runbook para el riesgo más caro del scraper: **una oferta asignada a la tienda
equivocada**. También aclara dos fuentes de confusión al diagnosticar: el
volumen del scraper y la tabla `ingestion_run`.

## 1. Por qué importa el `storeId`

Cada spec declara un `store_id` numérico:

| Tienda | `spec.json` | `store_id` |
|---|---|---|
| Falabella | `scraper/stores/falabella-cl/spec.json` | 1 |
| Ripley | `scraper/stores/ripley-cl/spec.json` | 4 |
| Mercado Libre | `scraper/stores/mercadolibre-cl/spec.json` | 6 |

Ese número viaja en `POST /ingestion/runs` (`storeId`) y el backend lo usa para
resolver `data_source.store_id`. Si no coinciden, las ofertas se guardan bajo
otra tienda. En `TAL-110` §8 ya ocurrió: ofertas de Ripley quedaron como Paris
hasta corregir 2→4 y 3→6.

Los IDs **no son arbitrarios**: salen de `backend/migrations/005_demo_catalog.sql`,
que inserta las tiendas sin `ORDER BY`. En otra base el orden puede cambiar.

## 2. Verificación

```bash
cd scraper
PYTHONPATH=src ./venv/bin/python -m scraper stores
```

```
falabella-cl         Falabella Chile          storeId=1 ...
ripley-cl            Ripley Chile             storeId=4 ...
mercadolibre-cl      Mercado Libre Chile      storeId=6 ...
```

Contra PostgreSQL (solo lectura):

```bash
kubectl -n <namespace> exec postgres-0 -- psql -U postgres -d soloservis -c \
  "select id, name, store_id from data_source order by id;"
```

El `id` de Tienda debe coincidir con el `storeId` del spec. `store_id` en
`data_source` es la columna que Ingestion actualiza al abrir un run.

## 3. Cómo realinear

Si los IDs no coinciden:

1. Detén el scheduler: `POST /tasks/stop` (frena el pool y los runs abiertos).
2. Confirma el ID real de la tienda en `data_source`.
3. Ajusta el `store_id` del spec **y** su fixture si cambia.
4. `scraper validate` debe pasar; corre una tienda y revisa `itemsSent`.
5. Verifica en PostgreSQL que las ofertas nuevas caigan en la tienda correcta:

```sql
select ds.name, count(*)
from product_offer po
join data_source ds on ds.id = po.store_id
group by ds.name order by 2 desc;
```

**Nunca** edites IDs a mano en producción sin detener el scheduler: un run en
vuelo puede abrir con el ID viejo.

## 4. Volúmenes antiguos

El SQLite del scraper guarda `DATA_SOURCE.store_id` por fuente. Si el PVC
sobrevive a un cambio de IDs, la fuente vieja conserva el número anterior y
`POST /runs` responde **409** (`store mismatch`).

Dos salidas:

- alinear filas en `DATA_SOURCE` del SQLite (o recrear el `SCRAPER_CONFIG`), o
- empezar con un PVC limpio si no hay historial que conservar.

El 409 es intencional: evita seguir escribiendo en la tienda equivocada.

## 5. `ingestion_run` no es la tabla de corridas

En PostgreSQL existen `scrape_run` e `ingestion_run`. La segunda está **vacía**
y el código no la usa: Ingestion crea y cierra corridas en `scrape_run` desde
`backend/internal/ingestion/repository.go`.

Para diagnosticar usa:

```sql
select id, status, items_sent, ingestion_run_id, error_message, finished_at
from scrape_run order by id desc limit 10;
```

`ingestion_run` queda como residuo del diseño anterior. No la uses como fuente
de verdad y no la elimines sin revisar migraciones.

## 6. Retención del PVC

El PVC `scraper-data` es de 1 Gi y no tiene política de limpieza. Cada corrida
guarda HTML de páginas y rechazos:

- `out/runs/<id>/pages/*.html`
- `out/rejected/run-<id>.jsonl`

Una corrida de Ripley deja del orden de 6–7 MB. Con cuatro corridas diarias el
volumen se llena en semanas. Antes de eso:

```bash
# ejemplo: conservar 7 días
kubectl -n <namespace> exec deploy/scraper-api -- \
  sh -c 'find /data/runs -maxdepth 1 -type d -mtime +7 -exec rm -rf {} +'
kubectl -n <namespace> exec deploy/scraper-api -- \
  sh -c 'find /data/rejected -type f -mtime +7 -delete'
```

El catálogo (`/data/catalog`) **no** se purga: es el resultado que se compara
entre corridas.
