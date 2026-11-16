---
name: database-migrations
description: Use when adding or changing SQL under backend/database/, editing migration_order.txt, touching sqlc queries, or when a migration seems not to run. Covers ordering, checksums, transaction rules and sqlc regeneration.
---

# Base de datos y migraciones

PostgreSQL 18. Las migraciones viven en `backend/database/migrations/` y el SQL
de sqlc en `backend/database/queries/`. Reglas generales en el `AGENTS.md` de
la raíz (§8).

## La regla que se olvida

**El orden lo da `backend/database/migration_order.txt`, no el nombre.** Hay
prefijos repetidos (`004_`, `005_`, `006_`, `007_`, `016_`). Un archivo nuevo
se agrega **al final de ese índice**.

Si el archivo existe pero no está en el índice, **nunca se aplica**. Pasó con
`021_dedupe_scraped_products.sql`: estaba en la carpeta y no en el índice.
Antes de dar por hecha una migración, comprueba:

```bash
cd backend/database
ls migrations/*.sql | wc -l          # archivos
grep -vc '^#' migration_order.txt    # entradas del índice
```

Los dos números deben coincidir.

## Reglas

1. **Una migración aplicada no se edita.** El runner guarda su checksum en
   `schema_migrations`; cambiarla rompe la verificación. Todo cambio va en un
   archivo nuevo al final del índice.
2. **El runner no envuelve en transacción.** Si la migración debe ser completa
   o nada, pon `BEGIN`/`COMMIT` o un solo bloque `DO`.
3. **No dependas de IDs fijos.** Los de `store` cambian según el entorno.
4. **SQL parametrizado siempre.** Las migraciones son estáticas; las consultas
   de aplicación usan `$1`, `$2`…
5. **`sqlc` no se edita a mano.** `internal/database/generated/` sale de
   `database/queries/` con:

```bash
cd backend && sqlc generate     # usa sqlc.yaml
```

## Cómo se escribe una migración

```
backend/database/migrations/022_descripcion_corta.sql
```

1. Cabecera con la tarea y el objetivo en una línea.
2. Idempotente cuando sea posible (`IF NOT EXISTS`, `CREATE OR REPLACE`).
3. Agregar el nombre al final de `migration_order.txt`.
4. Probar contra una base limpia, no sólo contra la de desarrollo.
5. Si toca datos existentes, migración aparte de la de esquema.

## Verificación

```bash
kubectl kustomize k8s/overlays/uct           # el Job de migraciones compila
cd backend && go build ./... && go test ./...
```

Con `TEST_DATABASE_URL` apuntando a un PostgreSQL migrado, los tests de
integración comprueban el esquema real. Sin la variable se omiten: **no
confundas "omitido" con "pasó"**.

## Antes de abrir el PR

- [ ] Archivo nuevo, nunca edición de uno aplicado.
- [ ] Nombre agregado al final de `migration_order.txt`.
- [ ] Conteo de archivos == conteo de entradas del índice.
- [ ] `sqlc generate` ejecutado si cambió una consulta.
- [ ] `go build ./...` y `go test ./...` en verde.
- [ ] La migración corre sobre una base limpia, no sólo la de siempre.

## Anti-patrones

- Renumerar migraciones ya aplicadas.
- Meter esquema y datos en el mismo archivo.
- Asumir que el orden alfabético funciona.
- Editar `internal/database/generated/`.
