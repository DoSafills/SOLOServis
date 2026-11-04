\echo '=== 1. TODAS LAS FOREIGN KEYS (con ON DELETE / ON UPDATE) ==='
SELECT
  c.conrelid::regclass::text  AS tabla_hija,
  a.attname                   AS columna,
  c.confrelid::regclass::text AS tabla_padre,
  c.conname                   AS fk,
  CASE c.confdeltype WHEN 'a' THEN 'NO ACTION' WHEN 'r' THEN 'RESTRICT'
                     WHEN 'c' THEN 'CASCADE'   WHEN 'n' THEN 'SET NULL'
                     WHEN 'd' THEN 'SET DEFAULT' END AS on_delete,
  CASE c.confupdtype WHEN 'a' THEN 'NO ACTION' WHEN 'r' THEN 'RESTRICT'
                     WHEN 'c' THEN 'CASCADE'   WHEN 'n' THEN 'SET NULL'
                     WHEN 'd' THEN 'SET DEFAULT' END AS on_update
FROM pg_constraint c
JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = c.conkey[1]
WHERE c.contype = 'f' AND c.connamespace = 'public'::regnamespace
ORDER BY 1, 2;

\echo '=== 2. RESUMEN DE ON DELETE ==='
SELECT
  CASE confdeltype WHEN 'a' THEN 'NO ACTION' WHEN 'r' THEN 'RESTRICT'
                   WHEN 'c' THEN 'CASCADE'   WHEN 'n' THEN 'SET NULL'
                   WHEN 'd' THEN 'SET DEFAULT' END AS on_delete,
  count(*) AS cantidad
FROM pg_constraint
WHERE contype = 'f' AND connamespace = 'public'::regnamespace
GROUP BY 1 ORDER BY 2 DESC;

\echo '=== 3. COLUMNAS *_id SIN FOREIGN KEY ==='
SELECT c.table_name, c.column_name, c.data_type
FROM information_schema.columns c
JOIN information_schema.tables t
  ON t.table_name = c.table_name AND t.table_schema = c.table_schema
WHERE c.table_schema = 'public'
  AND t.table_type = 'BASE TABLE'
  AND c.column_name LIKE '%\_id' ESCAPE '\'
  AND NOT EXISTS (
    SELECT 1 FROM pg_constraint k
    JOIN pg_attribute a ON a.attrelid = k.conrelid AND a.attnum = ANY (k.conkey)
    WHERE k.contype = 'f'
      AND k.conrelid = (quote_ident(c.table_schema)||'.'||quote_ident(c.table_name))::regclass
      AND a.attname = c.column_name
  )
ORDER BY c.table_name, c.column_name;

\echo '=== 4. TABLAS INTERMEDIAS N:M ==='
SELECT c.conrelid::regclass::text AS tabla,
       count(*) AS num_fks,
       (SELECT count(*) FROM information_schema.columns ic
         WHERE ic.table_schema = 'public'
           AND ic.table_name = c.conrelid::regclass::text) AS num_columnas
FROM pg_constraint c
WHERE c.contype = 'f' AND c.connamespace = 'public'::regnamespace
GROUP BY c.conrelid
HAVING count(*) >= 2
ORDER BY 3, 1;

\echo '=== 5. N:M SIN UNIQUE/PK COMPUESTA ==='
SELECT DISTINCT c.conrelid::regclass::text AS tabla
FROM pg_constraint c
WHERE c.contype = 'f' AND c.connamespace = 'public'::regnamespace
  AND c.conrelid::regclass::text IN (
    'store_location','provider_location','product_comparison_item',
    'service_comparison_item','product_category_specification',
    'service_category_specification','product_specification_value',
    'service_specification_value','product_attribute_value'
  )
  AND NOT EXISTS (
    SELECT 1 FROM pg_constraint u
    WHERE u.conrelid = c.conrelid AND u.contype IN ('u','p')
      AND array_length(u.conkey,1) >= 2
  );

\echo '=== 6. CART_ITEM ==='
SELECT
  count(*) AS total,
  count(*) FILTER (WHERE offer_id IS NOT NULL AND service_offer_id IS NOT NULL) AS con_ambos,
  count(*) FILTER (WHERE offer_id IS NULL AND service_offer_id IS NULL) AS con_ninguno
FROM cart_item;

SELECT conname, pg_get_constraintdef(oid) AS definicion
FROM pg_constraint
WHERE conrelid = 'cart_item'::regclass AND contype = 'c';
