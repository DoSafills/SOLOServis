-- Repara los textos del seed que quedaron con las tildes dañadas en el clúster.
--
-- deploy.ps1 pasaba los .sql por `kubectl kustomize` en PowerShell 5.1, que leía
-- esa salida UTF-8 con la página de códigos 850 de la consola. Cada letra con
-- tilde ocupa 2 bytes en UTF-8 y se guardó como 2 caracteres de cp850:
-- "ó" (C3 B3) quedó como "├│", "í" (C3 AD) como "├¡" y "é" (C3 A9) como "├®".
-- El arreglo de deploy.ps1 (PR #110) evita que vuelva a pasar; esta migración
-- repara lo que ya se guardó. La 018 corrigió solo los dos nombres que
-- bloqueaban la 019.
--
-- Alcance:
--   - Todas las columnas text, varchar y jsonb del esquema public. Qué columnas
--     quedaron dañadas depende de qué migraciones aplicó el script viejo en cada
--     entorno: en el clúster fueron 21; en una base que se arme desde cero con
--     el script viejo son más. En las tablas de usuarios solo hay filas de
--     demostración del seed 003 (el usuario de example.com, dos reseñas y una
--     búsqueda): lo que se escribe desde la web no pasa por PowerShell.
--   - Solo las filas con "├" o "┬", el primer carácter de toda secuencia dañada
--     (bytes C3 y C2 en cp850). Un texto real no los usa; en una base sana no
--     cambia nada.
--   - PostgreSQL no trae la codificación cp850 y no se puede revertir con
--     convert_from: se usa una tabla de equivalencias explícita, calculada
--     codificando cada letra en UTF-8 y leyéndola como cp850.
--   - Si el valor corregido choca con un índice único (el valor correcto ya
--     existe), la fila queda como está y se informa con un NOTICE.
--
-- Todo va en un solo bloque: o se aplica completo o no cambia nada. Es
-- idempotente: una segunda pasada no encuentra filas que corregir.
DO $$
DECLARE
    -- Secuencia dañada (cp850) y la letra correcta, en el mismo orden.
    danadas text[] := ARRAY[
        U&'\251C\00ED', U&'\251C\00AE', U&'\251C\00A1', U&'\251C\2502', U&'\251C\2551',
        U&'\251C\00FC', U&'\251C\00EB', U&'\251C\00EC', U&'\251C\00F4', U&'\251C\00DC',
        U&'\251C\2592', U&'\251C\00E6', U&'\251C\255D', U&'\251C\00A3',
        U&'\252C\2510', U&'\252C\00ED', U&'\252C\2591', U&'\252C\2551', U&'\252C\00AC'];
    correctas text[] := ARRAY[
        U&'\00E1', U&'\00E9', U&'\00ED', U&'\00F3', U&'\00FA',   -- á é í ó ú
        U&'\00C1', U&'\00C9', U&'\00CD', U&'\00D3', U&'\00DA',   -- Á É Í Ó Ú
        U&'\00F1', U&'\00D1', U&'\00FC', U&'\00DC',              -- ñ Ñ ü Ü
        U&'\00BF', U&'\00A1', U&'\00B0', U&'\00BA', U&'\00AA'];  -- ¿ ¡ ° º ª
    patron CONSTANT text := U&'[\251C\252C]';
    col record;
    fila record;
    nuevo text;
    corregidas integer := 0;
    omitidas integer := 0;
    pendientes integer := 0;
    n integer;
BEGIN
    FOR col IN
        SELECT c.table_name AS tabla, c.column_name AS columna, c.udt_name AS tipo
        FROM information_schema.columns c
        JOIN information_schema.tables t
          ON t.table_schema = c.table_schema AND t.table_name = c.table_name
        WHERE c.table_schema = 'public'
          AND t.table_type = 'BASE TABLE'
          AND c.table_name <> 'schema_migrations'
          AND c.udt_name IN ('text', 'varchar', 'jsonb')
          AND c.is_generated = 'NEVER'
        ORDER BY c.table_name, c.column_name
    LOOP
        FOR fila IN EXECUTE format(
            'SELECT ctid AS fila_ctid, %I::text AS valor FROM %I WHERE %I::text ~ $1',
            col.columna, col.tabla, col.columna) USING patron
        LOOP
            nuevo := fila.valor;
            FOR i IN 1 .. array_length(danadas, 1) LOOP
                nuevo := replace(nuevo, danadas[i], correctas[i]);
            END LOOP;
            CONTINUE WHEN nuevo = fila.valor;

            BEGIN
                -- El cast vuelve al tipo de la columna (text, varchar o jsonb).
                EXECUTE format('UPDATE %I SET %I = $1::%I WHERE ctid = $2', col.tabla, col.columna, col.tipo)
                    USING nuevo, fila.fila_ctid;
                corregidas := corregidas + 1;
            EXCEPTION WHEN unique_violation THEN
                omitidas := omitidas + 1;
                RAISE NOTICE 'fix_seed_encoding: %.% = "%" ya existe; la fila queda sin cambios',
                    col.tabla, col.columna, nuevo;
            END;
        END LOOP;

        EXECUTE format('SELECT count(*) FROM %I WHERE %I::text ~ $1', col.tabla, col.columna)
            INTO n USING patron;
        pendientes := pendientes + n;
    END LOOP;

    RAISE NOTICE 'fix_seed_encoding: % valores corregidos, % omitidos por duplicado, % todavía con caracteres dañados',
        corregidas, omitidas, pendientes;
END;
$$;
