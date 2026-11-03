\echo '=== 1. TABLAS ==='
\dt

\echo '=== 2. TABLAS SIN CLAVE PRIMARIA ==='
SELECT c.relname AS tabla
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = 'public' AND c.relkind = 'r'
  AND NOT EXISTS (
    SELECT 1 FROM pg_constraint k
    WHERE k.conrelid = c.oid AND k.contype = 'p'
  );

\echo '=== 3. COLUMNAS DE PRECIO ==='
SELECT table_name, column_name, data_type
FROM information_schema.columns
WHERE table_schema = 'public'
  AND (column_name ILIKE '%price%' OR column_name ILIKE '%precio%');

\echo '=== 4. COLUMNAS DE FECHA ==='
SELECT table_name, column_name, data_type
FROM information_schema.columns
WHERE table_schema = 'public'
  AND data_type LIKE 'timestamp%'
ORDER BY data_type, table_name;

\echo '=== 5. INDICES ==='
SELECT tablename, indexname
FROM pg_indexes
WHERE schemaname = 'public'
ORDER BY tablename;

\echo '=== 6. FOREIGN KEYS SIN INDICE ==='
SELECT conrelid::regclass AS tabla, conname AS fk
FROM pg_constraint c
WHERE contype = 'f'
  AND NOT EXISTS (
    SELECT 1 FROM pg_index i
    WHERE i.indrelid = c.conrelid
      AND c.conkey[1] = i.indkey[0]
  );
