-- TAL-241: limpieza de los datos de demostración para una base que no usa el
-- runner de migraciones (por ejemplo, la base de pruebas en Neon). Hace lo
-- mismo que las migraciones 044_delete_demo_seed.sql y
-- 045_fix_seed_encoding_cp850.sql, que van copiadas sin cambios en la parte 2
-- (un test de Go lo comprueba).
--
-- Uso, con la URL de conexión en una variable de entorno (nunca en el archivo):
--   1. Revisión, solo lectura:  psql "$DATABASE_URL" -f backend/database/scripts/TAL-241-neon.sql
--   2. Aplicar:                 psql "$DATABASE_URL" -v aplicar=1 -f backend/database/scripts/TAL-241-neon.sql
-- Antes de aplicar, respalda la base (pg_dump o una rama de Neon). La parte 2
-- va en una sola transacción: si algo falla no cambia nada. Es idempotente: una
-- segunda pasada no borra ni corrige nada. Necesita psql 10 o superior.
--
-- Si la base tiene schema_migrations, la parte 2 registra 044 y 045 con el
-- checksum que calcula el runner, y una corrida posterior del runner las salta.
-- Si no la tiene, el runner se niega a migrar esa base (tiene esquema sin
-- registro) y no cambia nada; esta limpieza no lo afecta.
\set ON_ERROR_STOP on
\pset pager off

-- Parte 1: revisión. No escribe nada.
BEGIN TRANSACTION READ ONLY;

\echo '== Lo que se borraría'
WITH demo(sku) AS (VALUES
    ('SKU-SAMS-001'), ('SKU-LG-001'), ('SKU-SAMS-002'), ('SKU-LG-002'),
    ('DEMO-APPLE-AIRPODS-PRO2'), ('DEMO-APPLE-IPHONE15-128'), ('DEMO-ASUS-TUF-A15'),
    ('DEMO-BOSCH-WASHER-9'), ('DEMO-HP-PAVILION-15'), ('DEMO-JBL-FLIP6'),
    ('DEMO-LENOVO-IDEAPAD-16'), ('DEMO-LG-FRIDGE-530'), ('DEMO-LG-TV-OLED-65'),
    ('DEMO-MIDEA-FRIDGE-400'), ('DEMO-MOTO-EDGE50-256'), ('DEMO-NINTENDO-SWITCH-OLED'),
    ('DEMO-PHILIPS-AIRFRYER-62'), ('DEMO-SAMSUNG-S24-256'), ('DEMO-SAMSUNG-WASHER-11'),
    ('DEMO-SONY-HEADPHONES-XM5'), ('DEMO-SONY-PS5-SLIM'), ('DEMO-SONY-TV-55-4K'),
    ('DEMO-TCL-TV-QLED-50'), ('DEMO-XBOX-SERIES-X-1TB'), ('DEMO-XIAOMI-REDMI13-256'),
    ('DEMO-XIAOMI-ROBOT-VACUUM'),
    ('SKU-LEN-001'), ('SKU-APP-001'), ('SKU-XIA-001'),
    ('SAMSUNG-MKF9HNZ5C6'), ('LG-LM22SGPK'), ('MIDEA-MDRS710FGE50IN'),
    ('HP-VICTUS-15-FB3004LA'), ('ASUS-TUF-A15-FA506-R7-RTX3050'), ('LENOVO-LOQ-R7-RTX5060-16-512'),
    ('APPLE-IPHONE-16-128'), ('SAMSUNG-GALAXY-S26-ULTRA-256'), ('XIAOMI-17-ULTRA-512')),
marcas(name) AS (VALUES
    ('Apple'), ('ASUS'), ('Bosch'), ('Dell'), ('HP'), ('JBL'), ('Lenovo'), ('LG'), ('Microsoft'),
    ('Midea'), ('Motorola'), ('Nintendo'), ('Philips'), ('Samsung'), ('Sony'), ('TCL'), ('Xiaomi'))
SELECT
    (SELECT count(*) FROM product p JOIN demo d ON d.sku = p.sku) AS productos_demo,
    (SELECT count(*) FROM product p WHERE p.sku IS NULL OR p.sku NOT IN (SELECT sku FROM demo)) AS productos_que_quedan,
    (SELECT count(*) FROM product p WHERE p.sku IS NOT NULL AND p.sku NOT IN (SELECT sku FROM demo)) AS con_otro_sku,
    (SELECT count(*) FROM user_account WHERE password_hash LIKE 'hash\_demo\_%') AS usuarios_demo,
    (SELECT count(*) FROM user_account WHERE password_hash NOT LIKE 'hash\_demo\_%') AS usuarios_que_quedan,
    (SELECT count(*) FROM brand b JOIN marcas m ON m.name = b.name
      WHERE NOT EXISTS (SELECT 1 FROM product p WHERE p.brand_id = b.id
                          AND (p.sku IS NULL OR p.sku NOT IN (SELECT sku FROM demo)))) AS marcas_que_quedan_vacias;

\echo '== Requisitos: columnas, función de la 009, FK que bloquearían el borrado y migraciones registradas'
DO $$
DECLARE
    falta text;
    fk record;
    col record;
    n integer;
    total integer := 0;
BEGIN
    FOR falta IN
        SELECT r.tabla || '.' || r.columna
        FROM (VALUES ('product', 'sku'), ('product', 'brand_id'), ('product', 'created_at'),
                     ('product_offer', 'product_url'), ('product_offer', 'last_updated'),
                     ('product_offer', 'list_price'), ('product_offer', 'stock'),
                     ('product_price_history', 'recorded_at'), ('user_account', 'password_hash'),
                     ('brand', 'name')) AS r(tabla, columna)
        WHERE NOT EXISTS (SELECT 1 FROM information_schema.columns c
                          WHERE c.table_schema = 'public' AND c.table_name = r.tabla AND c.column_name = r.columna)
    LOOP
        RAISE NOTICE 'FALTA la columna %: la parte 2 va a fallar sin cambiar nada', falta;
    END LOOP;

    IF to_regprocedure('normalize_product_offer_url(text)') IS NULL THEN
        RAISE NOTICE 'AVISO: falta normalize_product_offer_url (migración 009): la parte 2 omite el rescate de precios';
    ELSE
        SELECT count(*) INTO n
        FROM product_offer demo
        JOIN product dp ON dp.id = demo.product_id AND dp.sku IS NOT NULL
        JOIN product_offer real_offer
          ON real_offer.store_id = demo.store_id
         AND split_part(normalize_product_offer_url(real_offer.product_url), '?', 1) =
             split_part(normalize_product_offer_url(demo.product_url), '?', 1)
        JOIN product rp ON rp.id = real_offer.product_id AND rp.sku IS NULL;
        RAISE NOTICE 'OK: normalize_product_offer_url existe; % ofertas con SKU comparten URL con un producto sin SKU (se rescatan)', n;
    END IF;

    -- Una FK sin ON DELETE CASCADE ni SET NULL hacia estas tablas hace fallar el borrado.
    FOR fk IN
        SELECT conrelid::regclass AS hijo, confrelid::regclass AS padre, conname
        FROM pg_constraint
        WHERE contype = 'f'
          AND confrelid IN ('product'::regclass, 'product_offer'::regclass, 'user_account'::regclass, 'brand'::regclass)
          AND confdeltype NOT IN ('c', 'n')
    LOOP
        RAISE NOTICE 'BLOQUEA: % -> % (%) no borra en cascada: la parte 2 va a fallar sin cambiar nada', fk.hijo, fk.padre, fk.conname;
    END LOOP;

    IF to_regclass('public.schema_migrations') IS NULL THEN
        RAISE NOTICE 'Sin schema_migrations: la parte 2 no registra 044 ni 045';
    ELSE
        EXECUTE 'SELECT count(*) FROM schema_migrations WHERE version = ANY ($1)'
            INTO n USING ARRAY['044_delete_demo_seed.sql', '045_fix_seed_encoding_cp850.sql'];
        RAISE NOTICE 'schema_migrations: % de 2 ya registradas (044 y 045)', n;
        -- Si falta un seed, el runner lo aplicaría después y volverían sus productos demo.
        FOR falta IN
            EXECUTE 'SELECT s.v FROM unnest($1::text[]) AS s(v)
                     WHERE NOT EXISTS (SELECT 1 FROM schema_migrations m WHERE m.version = s.v)'
            USING ARRAY['003_seed.sql', '005_demo_catalog.sql', '005_seed_catalogo.sql',
                        '006_refrigeradores.sql', '007_Computacion.sql', '008_Telefonia.sql']
        LOOP
            RAISE NOTICE 'AVISO: % no está en schema_migrations; si el runner la aplica después, vuelven sus productos demo', falta;
        END LOOP;
    END IF;

    -- Textos con tildes dañadas por cp850: lo que corrige 045.
    FOR col IN
        SELECT c.table_name AS tabla, c.column_name AS columna
        FROM information_schema.columns c
        JOIN information_schema.tables t ON t.table_schema = c.table_schema AND t.table_name = c.table_name
        WHERE c.table_schema = 'public' AND t.table_type = 'BASE TABLE' AND c.table_name <> 'schema_migrations'
          AND c.udt_name IN ('text', 'varchar', 'jsonb') AND c.is_generated = 'NEVER'
    LOOP
        EXECUTE format('SELECT count(*) FROM %I WHERE %I::text ~ $1', col.tabla, col.columna)
            INTO n USING U&'[\251C\252C]';
        IF n > 0 THEN
            RAISE NOTICE 'Tildes dañadas: %.% en % filas', col.tabla, col.columna, n;
            total := total + n;
        END IF;
    END LOOP;
    RAISE NOTICE 'Tildes dañadas: % valores en total', total;
END;
$$;

ROLLBACK;

\if :{?aplicar}
\echo '== Parte 2: aplicando 044 y 045 en una sola transacción'
BEGIN;

-- >>> 044_delete_demo_seed.sql
-- TAL-241: borra los productos, usuarios y marcas de demostración del seed sin
-- tocar lo que trae el scraper.
--
-- Qué se borra:
--   - Los 38 productos que siembran 003, 005_demo_catalog, 005_seed_catalogo,
--     006_refrigeradores, 007_Computacion y 008_Telefonia, identificados por su
--     SKU. Ingestion no inventa SKU y el scraper no los manda: un producto sin
--     SKU o con otro SKU no se toca. Las FK borran en cascada sus ofertas,
--     historial, imágenes, specs, atributos, reseñas, intereses y carrito.
--   - Los usuarios del seed 003, cuyo password_hash es "hash_demo_N": nadie
--     podía entrar con ellos. Sus reseñas, comparaciones e intereses se van en
--     cascada; search_query y ai_comparison_query quedan con user_id NULL.
--   - Las marcas del seed que quedan sin ningún producto, porque
--     GET /search/brands lista todas las marcas activas. Si el scraper trae
--     después un producto de esa marca, ingestion la vuelve a crear.
--
-- Rescate: el scraper reconoce un producto por la URL de su oferta y, si dos
-- productos comparten URL, elige el de menor id. Así actualizó la oferta de un
-- producto demo (el Midea MDRS710 de Ripley, que comparte URL con el producto
-- scrapeado). Antes de borrar, el historial que el scraper dejó en esa oferta
-- (posterior al alta del producto real) se copia a la oferta del producto real,
-- y esta toma el precio, la disponibilidad y la fecha más recientes. Sin
-- normalize_product_offer_url (migración 009) el rescate se omite con un aviso.
--
-- Va en un solo bloque: o se aplica completo o no cambia nada. Es idempotente:
-- una segunda pasada no encuentra filas que borrar. No tiene reversa: el
-- respaldo es el pg_dump previo al deploy.
DO $$
DECLARE
    -- SKU de los productos del seed, en el orden de las migraciones que los crean.
    skus_demo CONSTANT text[] := ARRAY[
        -- 003_seed
        'SKU-SAMS-001', 'SKU-LG-001', 'SKU-SAMS-002', 'SKU-LG-002',
        -- 005_demo_catalog
        'DEMO-APPLE-AIRPODS-PRO2', 'DEMO-APPLE-IPHONE15-128', 'DEMO-ASUS-TUF-A15',
        'DEMO-BOSCH-WASHER-9', 'DEMO-HP-PAVILION-15', 'DEMO-JBL-FLIP6',
        'DEMO-LENOVO-IDEAPAD-16', 'DEMO-LG-FRIDGE-530', 'DEMO-LG-TV-OLED-65',
        'DEMO-MIDEA-FRIDGE-400', 'DEMO-MOTO-EDGE50-256', 'DEMO-NINTENDO-SWITCH-OLED',
        'DEMO-PHILIPS-AIRFRYER-62', 'DEMO-SAMSUNG-S24-256', 'DEMO-SAMSUNG-WASHER-11',
        'DEMO-SONY-HEADPHONES-XM5', 'DEMO-SONY-PS5-SLIM', 'DEMO-SONY-TV-55-4K',
        'DEMO-TCL-TV-QLED-50', 'DEMO-XBOX-SERIES-X-1TB', 'DEMO-XIAOMI-REDMI13-256',
        'DEMO-XIAOMI-ROBOT-VACUUM',
        -- 005_seed_catalogo
        'SKU-LEN-001', 'SKU-APP-001', 'SKU-XIA-001',
        -- 006_refrigeradores
        'SAMSUNG-MKF9HNZ5C6', 'LG-LM22SGPK', 'MIDEA-MDRS710FGE50IN',
        -- 007_Computacion
        'HP-VICTUS-15-FB3004LA', 'ASUS-TUF-A15-FA506-R7-RTX3050', 'LENOVO-LOQ-R7-RTX5060-16-512',
        -- 008_Telefonia
        'APPLE-IPHONE-16-128', 'SAMSUNG-GALAXY-S26-ULTRA-256', 'XIAOMI-17-ULTRA-512'];
    -- Marcas que crean esos seeds. Solo se borran las que quedan sin productos.
    marcas_demo CONSTANT text[] := ARRAY[
        'Apple', 'ASUS', 'Bosch', 'Dell', 'HP', 'JBL', 'Lenovo', 'LG', 'Microsoft',
        'Midea', 'Motorola', 'Nintendo', 'Philips', 'Samsung', 'Sony', 'TCL', 'Xiaomi'];
    demo_ids integer[];
    ultimo_historial integer;
    historial_copiado integer := 0;
    ofertas_actualizadas integer := 0;
    productos_borrados integer;
    usuarios_borrados integer;
    marcas_borradas integer;
BEGIN
    demo_ids := ARRAY(SELECT id FROM product WHERE sku = ANY (skus_demo));

    IF to_regprocedure('normalize_product_offer_url(text)') IS NULL THEN
        RAISE NOTICE 'delete_demo_seed: falta normalize_product_offer_url (migración 009); se omite el rescate de ofertas';
    ELSE
        -- Cada oferta demo con la misma tienda y la misma URL (sin parámetros)
        -- que una oferta de un producto real, con el mismo desempate que ingestion.
        CREATE TEMP TABLE tal241_rescate ON COMMIT DROP AS
        SELECT DISTINCT ON (demo.id)
               demo.id AS demo_offer_id, real_offer.id AS real_offer_id, real_product.created_at AS desde
        FROM product_offer demo
        JOIN product_offer real_offer
          ON real_offer.store_id = demo.store_id
         AND split_part(normalize_product_offer_url(real_offer.product_url), '?', 1) =
             split_part(normalize_product_offer_url(demo.product_url), '?', 1)
        JOIN product real_product ON real_product.id = real_offer.product_id
        WHERE demo.product_id = ANY (demo_ids)
          AND real_product.id <> ALL (demo_ids)
        ORDER BY demo.id, real_product.active DESC, real_product.id, real_offer.id;

        -- El historial que el scraper dejó en la oferta demo después del alta
        -- del producto real; el del seed es anterior y se descarta.
        INSERT INTO product_price_history (product_offer_id, price, recorded_at)
        SELECT r.real_offer_id, h.price, h.recorded_at
        FROM tal241_rescate r
        JOIN product_price_history h ON h.product_offer_id = r.demo_offer_id
        WHERE h.recorded_at > r.desde
        ON CONFLICT DO NOTHING;
        GET DIAGNOSTICS historial_copiado = ROW_COUNT;

        -- La oferta real toma los datos de la demo si esta se actualizó después.
        -- El trigger de 010 anota el cambio de precio con la hora de esta
        -- migración; esa fila se quita porque el precio ya está en el historial
        -- copiado con la hora en que el scraper lo vio.
        SELECT COALESCE(max(id), 0) INTO ultimo_historial FROM product_price_history;
        UPDATE product_offer real_offer
        SET price = demo.price,
            list_price = demo.list_price,
            available = demo.available,
            stock = demo.stock,
            last_updated = demo.last_updated
        FROM (
            SELECT DISTINCT ON (r.real_offer_id) r.real_offer_id, o.*
            FROM tal241_rescate r
            JOIN product_offer o ON o.id = r.demo_offer_id
            ORDER BY r.real_offer_id, o.last_updated DESC
        ) demo
        WHERE real_offer.id = demo.real_offer_id
          AND demo.last_updated > real_offer.last_updated;
        GET DIAGNOSTICS ofertas_actualizadas = ROW_COUNT;
        DELETE FROM product_price_history
        WHERE id > ultimo_historial
          AND product_offer_id IN (SELECT real_offer_id FROM tal241_rescate);
    END IF;

    DELETE FROM product WHERE id = ANY (demo_ids);
    GET DIAGNOSTICS productos_borrados = ROW_COUNT;

    DELETE FROM user_account WHERE password_hash LIKE 'hash\_demo\_%';
    GET DIAGNOSTICS usuarios_borrados = ROW_COUNT;

    DELETE FROM brand b
    WHERE b.name = ANY (marcas_demo)
      AND NOT EXISTS (SELECT 1 FROM product p WHERE p.brand_id = b.id);
    GET DIAGNOSTICS marcas_borradas = ROW_COUNT;

    RAISE NOTICE 'delete_demo_seed: % productos, % usuarios y % marcas borrados; % precios rescatados, % ofertas actualizadas',
        productos_borrados, usuarios_borrados, marcas_borradas, historial_copiado, ofertas_actualizadas;
END;
$$;
-- <<< 044_delete_demo_seed.sql

-- >>> 045_fix_seed_encoding_cp850.sql
-- TAL-241 (trae TAL-183): la rama fix/tildes-seed-cp850 la traía como 022, número que
-- develop ya usa. Va al final del orden con su propio nombre. Los caracteres
-- dañados se nombran por su código (U+251C...) y no se escriben: deploy.ps1 y
-- deploy.sh detienen el deploy si el render los trae.
--
-- Repara los textos del seed que quedaron con las tildes dañadas en el clúster.
--
-- deploy.ps1 pasaba los .sql por `kubectl kustomize` en PowerShell 5.1, que leía
-- esa salida UTF-8 con la página de códigos 850 de la consola. Cada letra con
-- tilde ocupa 2 bytes en UTF-8 y se guardó como 2 caracteres de cp850:
-- "ó" (C3 B3) quedó como U+251C U+2502, "í" (C3 AD) como U+251C U+00A1 y
-- "é" (C3 A9) como U+251C U+00AE.
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
--   - Solo las filas con U+251C o U+252C, el primer carácter de toda secuencia dañada
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
-- <<< 045_fix_seed_encoding_cp850.sql

-- Registro para el runner: mismo checksum que calcula migrate.sh (SHA-256 del
-- archivo sin CR). Si la base no tiene schema_migrations, no se registra nada.
DO $$
DECLARE
    registradas integer;
BEGIN
    IF to_regclass('public.schema_migrations') IS NULL THEN
        RAISE NOTICE 'TAL-241: sin schema_migrations; no se registran 044 ni 045';
    ELSE
        EXECUTE 'INSERT INTO schema_migrations (version, checksum)
                 VALUES ($1, $2), ($3, $4) ON CONFLICT (version) DO NOTHING'
            USING '044_delete_demo_seed.sql', '897171dfc5e8c8e8ff0503a1c75daedf69f23c66f18525b3c921b6ccc8854ab6',
                  '045_fix_seed_encoding_cp850.sql', '4f1eded07ae69acc3f310a614e28b86a4400c548fd6c30af086bbb6a30dc78f5';
        GET DIAGNOSTICS registradas = ROW_COUNT;
        RAISE NOTICE 'TAL-241: % de 2 registradas ahora en schema_migrations (el resto ya estaba)', registradas;
    END IF;
END;
$$;

COMMIT;
\echo '== Listo: corre de nuevo la parte 1 para comprobar que no queda nada por borrar'
\else
\echo '== Solo revisión. Para aplicar: psql ... -v aplicar=1 -f backend/database/scripts/TAL-241-neon.sql'
\endif
