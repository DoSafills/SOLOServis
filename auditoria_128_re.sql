\echo '=== FKs SIN INDICE (re-auditoria TAL-128) ==='
SELECT conrelid::regclass AS tabla, conname AS fk
FROM pg_constraint c
WHERE contype = 'f'
  AND NOT EXISTS (
    SELECT 1 FROM pg_index i
    WHERE i.indrelid = c.conrelid
      AND c.conkey[1] = i.indkey[0]
  );
