-- =====================================================================
-- Reversa de 036: restaura las tablas NV (032/033), el puente (034/035) y
-- product.category_nv3_id, y retira slug/level de product_category.
--
-- Vuelve a sembrar NV1/NV2/NV3 desde el árbol unificado (así conserva la
-- forma que dejó 036), repunta product.category_nv3_id por el nivel 3 y
-- repone el puente legacy por los pares de 034 y la regla de 035.
--
-- Lo que NO revierte, y por qué:
--   - product.category_id de los productos que 036 subió desde el nivel 4
--     (Microondas, Estufas, etc.) sigue en el nodo de nivel 3: el origen no
--     se guardó y adivinarlo por nombre sería peor. En el modelo NV ese
--     producto queda con category_nv3_id correcto.
--   - las filas de nivel 4 que 036 desactivó quedan inactivas.
--
-- Requiere que la base esté detenida para escrituras.
-- Elimina la fila de schema_migrations para que el runner reaplique 036.
--
-- Uso:
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f revert_036_unify_product_category_tree.sql
-- =====================================================================
BEGIN;

-- ---------------------------------------------------------------------
-- 1. Tablas NV de 032 y puente de 034.
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS categories_nv1 (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(150) NOT NULL,
    slug        VARCHAR(64)  NOT NULL,
    description TEXT,
    is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT categories_nv1_slug_key UNIQUE (slug)
);

CREATE TABLE IF NOT EXISTS categories_nv2 (
    id          SERIAL PRIMARY KEY,
    nv1_id      INTEGER      NOT NULL
                REFERENCES categories_nv1(id) ON DELETE RESTRICT,
    name        VARCHAR(150) NOT NULL,
    slug        VARCHAR(64)  NOT NULL,
    description TEXT,
    is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS categories_nv3 (
    id          SERIAL PRIMARY KEY,
    nv2_id      INTEGER      NOT NULL
                REFERENCES categories_nv2(id) ON DELETE RESTRICT,
    name        VARCHAR(150) NOT NULL,
    slug        VARCHAR(64)  NOT NULL,
    description TEXT,
    is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT categories_nv3_slug_key UNIQUE (slug),
    legacy_product_category_id INTEGER
);

CREATE INDEX IF NOT EXISTS idx_categories_nv2_nv1_id
    ON categories_nv2 (nv1_id);
CREATE INDEX IF NOT EXISTS idx_categories_nv3_nv2_id
    ON categories_nv3 (nv2_id);

CREATE UNIQUE INDEX IF NOT EXISTS uq_categories_nv2_nv1_name
    ON categories_nv2 (
        nv1_id,
        (translate(lower(btrim(name)),
                   U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                   'aeiouun'))
    );
CREATE UNIQUE INDEX IF NOT EXISTS uq_categories_nv3_nv2_name
    ON categories_nv3 (
        nv2_id,
        (translate(lower(btrim(name)),
                   U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                   'aeiouun'))
    );

CREATE UNIQUE INDEX IF NOT EXISTS uq_categories_nv3_legacy_product_category_id
    ON categories_nv3 (legacy_product_category_id);

-- ---------------------------------------------------------------------
-- 2. Siembra NV1/NV2/NV3 desde el árbol unificado (slug y level).
-- ---------------------------------------------------------------------
INSERT INTO categories_nv1 (name, slug, description)
SELECT pc.name, pc.slug, pc.description
FROM product_category pc
WHERE pc.active
  AND pc.level = 1
  AND NOT EXISTS (SELECT 1 FROM categories_nv1 nv1 WHERE nv1.slug = pc.slug);

INSERT INTO categories_nv2 (nv1_id, name, slug, description)
SELECT nv1.id, pc.name, pc.slug, pc.description
FROM product_category pc
JOIN product_category parent ON parent.id = pc.parent_category_id
JOIN categories_nv1 nv1 ON nv1.slug = parent.slug
WHERE pc.active
  AND pc.level = 2
  AND NOT EXISTS (SELECT 1 FROM categories_nv2 nv2 WHERE nv2.slug = pc.slug);

INSERT INTO categories_nv3 (nv2_id, name, slug, description)
SELECT nv2.id, pc.name, pc.slug, pc.description
FROM product_category pc
JOIN product_category parent ON parent.id = pc.parent_category_id
JOIN categories_nv2 nv2 ON nv2.slug = parent.slug
WHERE pc.active
  AND pc.level = 3
  AND NOT EXISTS (SELECT 1 FROM categories_nv3 nv3 WHERE nv3.slug = pc.slug);

-- ---------------------------------------------------------------------
-- 3. product.category_nv3_id, FK e índice (032).
-- ---------------------------------------------------------------------
ALTER TABLE product ADD COLUMN IF NOT EXISTS category_nv3_id INTEGER;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'product_category_nv3_fk'
          AND conrelid = 'product'::regclass
    ) THEN
        ALTER TABLE product
            ADD CONSTRAINT product_category_nv3_fk
            FOREIGN KEY (category_nv3_id)
            REFERENCES categories_nv3(id) ON DELETE RESTRICT;
    END IF;
END;
$$;

CREATE INDEX IF NOT EXISTS idx_product_category_nv3_id
    ON product (category_nv3_id);

-- Cada producto queda con la NV3 de su fila de nivel 3.
UPDATE product p
SET category_nv3_id = nv3.id
FROM product_category pc
JOIN categories_nv3 nv3 ON nv3.slug = pc.slug
WHERE pc.id = p.category_id
  AND pc.level = 3
  AND p.category_nv3_id IS DISTINCT FROM nv3.id;

-- ---------------------------------------------------------------------
-- 4. Puente legacy de 034 y regla de 035.
-- ---------------------------------------------------------------------
CREATE OR REPLACE FUNCTION pg_temp.tal036r_normalize_label(label_value TEXT)
RETURNS TEXT
LANGUAGE SQL
IMMUTABLE
AS $$
    SELECT translate(lower(btrim(label_value)),
                     U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                     'aeiouun')
$$;

WITH pairs(nv3_slug, legacy_name, legacy_parent_name) AS (
    VALUES
        ('refrigeradores', 'refrigeradores', 'electrodomesticos'),
        ('lavadoras', 'lavadoras', 'electrodomesticos'),
        ('cocina', 'cocina', 'electrodomesticos'),
        ('climatizacion', 'climatizacion', 'electrodomesticos'),
        ('limpieza-del-hogar', 'limpieza del hogar', 'electrodomesticos'),
        ('cuidado-personal', 'cuidado personal', 'electrodomesticos'),
        ('muebles-y-decoracion', 'hogar y mueble', NULL),
        ('notebooks', 'notebooks', 'computacion'),
        ('computadores-de-escritorio', 'computadores de escritorio', 'computacion'),
        ('accesorios-de-computacion', 'accesorios de computacion', 'computacion'),
        ('televisores', 'televisores', 'tecnologia'),
        ('audio', 'audio', 'tecnologia'),
        ('celulares', 'celulares', 'tecnologia'),
        ('gaming', 'gaming', 'tecnologia')
)
UPDATE categories_nv3 nv3
SET legacy_product_category_id = legacy.id,
    updated_at = NOW()
FROM pairs
JOIN LATERAL (
    SELECT pc.id
    FROM product_category pc
    LEFT JOIN product_category parent ON parent.id = pc.parent_category_id
    WHERE pg_temp.tal036r_normalize_label(pc.name) =
          pg_temp.tal036r_normalize_label(pairs.legacy_name)
      AND (
          (pairs.legacy_parent_name IS NULL AND pc.parent_category_id IS NULL)
          OR pg_temp.tal036r_normalize_label(parent.name) =
             pg_temp.tal036r_normalize_label(pairs.legacy_parent_name)
      )
    ORDER BY pc.active DESC, pc.id
    LIMIT 1
) AS legacy ON TRUE
WHERE nv3.slug = pairs.nv3_slug
  AND nv3.legacy_product_category_id IS NULL
  AND NOT EXISTS (
      SELECT 1
      FROM categories_nv3 other
      WHERE other.legacy_product_category_id = legacy.id
  );

-- 035: accesorios-de-computacion se puentea a la fila Computación que tiene
-- a Notebooks como hija.
WITH legacy AS (
    SELECT pc.id
    FROM product_category pc
    WHERE pg_temp.tal036r_normalize_label(pc.name) = 'computacion'
      AND EXISTS (
          SELECT 1
          FROM product_category child
          WHERE child.parent_category_id = pc.id
            AND pg_temp.tal036r_normalize_label(child.name) = 'notebooks'
      )
      AND NOT EXISTS (
          SELECT 1
          FROM categories_nv3 bound
          WHERE bound.legacy_product_category_id = pc.id
      )
    ORDER BY pc.active DESC, pc.id
    LIMIT 1
)
UPDATE categories_nv3 nv3
SET legacy_product_category_id = legacy.id,
    updated_at = NOW()
FROM legacy
WHERE nv3.slug = 'accesorios-de-computacion'
  AND nv3.legacy_product_category_id IS NULL;

-- 036 remapeó categories.computacion al nodo Accesorios: se devuelve a la
-- fila Computación, como estaba antes.
UPDATE categories c
SET legacy_product_category_id = legacy.id,
    updated_at = NOW()
FROM (
    SELECT pc.id
    FROM product_category pc
    WHERE pg_temp.tal036r_normalize_label(pc.name) = 'computacion'
      AND EXISTS (
          SELECT 1
          FROM product_category child
          WHERE child.parent_category_id = pc.id
            AND pg_temp.tal036r_normalize_label(child.name) = 'notebooks'
      )
    ORDER BY pc.active DESC, pc.id
    LIMIT 1
) AS legacy
WHERE c.slug = 'computacion'
  AND c.item_type = 'product'
  AND c.legacy_product_category_id IS DISTINCT FROM legacy.id;

-- ---------------------------------------------------------------------
-- 5. backfill_category_nv3 (033) y errores de decodificación de 032.
-- ---------------------------------------------------------------------
CREATE OR REPLACE FUNCTION backfill_category_nv3()
RETURNS INTEGER
LANGUAGE plpgsql
AS $$
DECLARE
    total INTEGER := 0;
    filled INTEGER;
    pendientes_total INTEGER;
    pendientes TEXT;
BEGIN
    -- 1. Categorías legacy que ya corresponden a una NV3 del árbol.
    UPDATE product p
    SET category_nv3_id = target.id
    FROM product_category source, categories_nv3 target
    WHERE p.category_id = source.id
      AND p.category_nv3_id IS NULL
      AND target.slug = CASE translate(lower(btrim(source.name)),
                                       U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                                       'aeiouun')
            WHEN 'refrigeradores' THEN 'refrigeradores'
            WHEN 'lavadoras' THEN 'lavadoras'
            WHEN 'televisores' THEN 'televisores'
            WHEN 'audio' THEN 'audio'
            WHEN 'celulares' THEN 'celulares'
            WHEN 'gaming' THEN 'gaming'
            WHEN 'hogar y mueble' THEN 'muebles-y-decoracion'
            WHEN 'notebooks' THEN 'notebooks'
            WHEN 'computadores de escritorio' THEN 'computadores-de-escritorio'
        END;
    GET DIAGNOSTICS filled = ROW_COUNT;
    total := total + filled;

    -- 2. Productos colgados de la fila legacy del NV2 (Cocina,
    --    Climatización, Limpieza del hogar o Cuidado personal): van a la
    --    NV3 de ese mismo padre.
    UPDATE product p
    SET category_nv3_id = target.id
    FROM product_category source, categories_nv3 target
    WHERE p.category_id = source.id
      AND p.category_nv3_id IS NULL
      AND target.slug = CASE translate(lower(btrim(source.name)),
                                       U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                                       'aeiouun')
            WHEN 'cocina' THEN 'cocina'
            WHEN 'climatizacion' THEN 'climatizacion'
            WHEN 'limpieza del hogar' THEN 'limpieza-del-hogar'
            WHEN 'cuidado personal' THEN 'cuidado-personal'
        END;
    GET DIAGNOSTICS filled = ROW_COUNT;
    total := total + filled;

    -- 3. Hijos legacy de esos cuatro NV2 (los 23 tipos de nivel 4): el
    --    padre se resuelve por parent_category_id, no por el nombre del
    --    producto ni por el de la hoja.
    UPDATE product p
    SET category_nv3_id = target.id
    FROM product_category child
    JOIN product_category parent ON parent.id = child.parent_category_id,
    categories_nv3 target
    WHERE p.category_id = child.id
      AND p.category_nv3_id IS NULL
      AND target.slug = CASE translate(lower(btrim(parent.name)),
                                       U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                                       'aeiouun')
            WHEN 'cocina' THEN 'cocina'
            WHEN 'climatizacion' THEN 'climatizacion'
            WHEN 'limpieza del hogar' THEN 'limpieza-del-hogar'
            WHEN 'cuidado personal' THEN 'cuidado-personal'
        END;
    GET DIAGNOSTICS filled = ROW_COUNT;
    total := total + filled;

    -- 4. Computación: accesorios primero (igual que classifyComputacion).
    UPDATE product p
    SET category_nv3_id = target.id
    FROM product_category source, categories_nv3 target
    WHERE source.id = p.category_id
      AND target.slug = 'accesorios-de-computacion'
      AND p.category_nv3_id IS NULL
      AND translate(lower(btrim(source.name)),
                    U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                    'aeiouun') = 'computacion'
      AND EXISTS (
          SELECT 1
          FROM unnest(string_to_array(
                   regexp_replace(
                       translate(lower(btrim(p.name)),
                                 U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                                 'aeiouun'),
                       '[^a-z0-9]+', ' ', 'g'),
                   ' ')) AS word
          WHERE word IN ('mochila', 'mochilas', 'funda', 'fundas', 'maletin',
                         'maletines', 'soporte', 'soportes', 'base', 'bases',
                         'cargador', 'cargadores', 'adaptador', 'adaptadores',
                         'mouse', 'teclado', 'teclados', 'monitor', 'monitores',
                         'tablet', 'tablets', 'ipad', 'ipads')
      );
    GET DIAGNOSTICS filled = ROW_COUNT;
    total := total + filled;

    -- 5. Computación: notebooks, laptops y macbooks.
    UPDATE product p
    SET category_nv3_id = target.id
    FROM product_category source, categories_nv3 target
    WHERE source.id = p.category_id
      AND target.slug = 'notebooks'
      AND p.category_nv3_id IS NULL
      AND translate(lower(btrim(source.name)),
                    U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                    'aeiouun') = 'computacion'
      AND EXISTS (
          SELECT 1
          FROM unnest(string_to_array(
                   regexp_replace(
                       translate(lower(btrim(p.name)),
                                 U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                                 'aeiouun'),
                       '[^a-z0-9]+', ' ', 'g'),
                   ' ')) AS word
          WHERE word IN ('notebook', 'notebooks', 'laptop', 'laptops',
                         'macbook', 'macbooks')
      );
    GET DIAGNOSTICS filled = ROW_COUNT;
    total := total + filled;

    -- 6. Computación: escritorio (imac, pc gamer, desktop, escritorio).
    UPDATE product p
    SET category_nv3_id = target.id
    FROM product_category source, categories_nv3 target
    WHERE source.id = p.category_id
      AND target.slug = 'computadores-de-escritorio'
      AND p.category_nv3_id IS NULL
      AND translate(lower(btrim(source.name)),
                    U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                    'aeiouun') = 'computacion'
      AND (
          EXISTS (
              SELECT 1
              FROM unnest(string_to_array(
                       regexp_replace(
                           translate(lower(btrim(p.name)),
                                     U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                                     'aeiouun'),
                           '[^a-z0-9]+', ' ', 'g'),
                       ' ')) AS word
              WHERE word IN ('imac', 'imacs', 'desktop', 'desktops',
                             'escritorio', 'escritorios')
          )
          OR (' ' || regexp_replace(
                      translate(lower(btrim(p.name)),
                                U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                                'aeiouun'),
                      '[^a-z0-9]+', ' ', 'g') || ' ') LIKE '% pc gamer %'
      );
    GET DIAGNOSTICS filled = ROW_COUNT;
    total := total + filled;

    -- Reporte: productos de Computación sin NV3, con id y nombre.
    SELECT count(*),
           string_agg(format('%s: %s', p.id, p.name), '; ' ORDER BY p.id)
    INTO pendientes_total, pendientes
    FROM product p
    JOIN product_category source ON source.id = p.category_id
    WHERE translate(lower(btrim(source.name)),
                    U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                    'aeiouun') = 'computacion'
      AND p.category_nv3_id IS NULL;

    IF pendientes_total > 0 THEN
        RAISE NOTICE 'revert 036: % producto(s) de Computación quedan sin NV3 (id: nombre): %',
            pendientes_total, pendientes;
    END IF;

    RAISE NOTICE 'revert 036: backfill_category_nv3 llenó % producto(s)', total;
    RETURN total;
END;
$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'categories_nv1_slug_format'
          AND conrelid = 'categories_nv1'::regclass
    ) THEN
        ALTER TABLE categories_nv1
            ADD CONSTRAINT categories_nv1_slug_format
            CHECK (slug ~ '^[a-z0-9-]{1,64}$') NOT VALID;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'categories_nv1_name_not_blank'
          AND conrelid = 'categories_nv1'::regclass
    ) THEN
        ALTER TABLE categories_nv1
            ADD CONSTRAINT categories_nv1_name_not_blank
            CHECK (btrim(name) <> '') NOT VALID;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'categories_nv2_slug_format'
          AND conrelid = 'categories_nv2'::regclass
    ) THEN
        ALTER TABLE categories_nv2
            ADD CONSTRAINT categories_nv2_slug_format
            CHECK (slug ~ '^[a-z0-9-]{1,64}$') NOT VALID;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'categories_nv2_name_not_blank'
          AND conrelid = 'categories_nv2'::regclass
    ) THEN
        ALTER TABLE categories_nv2
            ADD CONSTRAINT categories_nv2_name_not_blank
            CHECK (btrim(name) <> '') NOT VALID;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'categories_nv3_slug_format'
          AND conrelid = 'categories_nv3'::regclass
    ) THEN
        ALTER TABLE categories_nv3
            ADD CONSTRAINT categories_nv3_slug_format
            CHECK (slug ~ '^[a-z0-9-]{1,64}$') NOT VALID;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'categories_nv3_name_not_blank'
          AND conrelid = 'categories_nv3'::regclass
    ) THEN
        ALTER TABLE categories_nv3
            ADD CONSTRAINT categories_nv3_name_not_blank
            CHECK (btrim(name) <> '') NOT VALID;
    END IF;
END;
$$;

-- ---------------------------------------------------------------------
-- 6. Retira el árbol unificado de product_category.
-- ---------------------------------------------------------------------
DROP TRIGGER IF EXISTS product_category_level_guard ON product_category;
DROP FUNCTION IF EXISTS product_category_level_guard();

ALTER TABLE product_category
    DROP CONSTRAINT IF EXISTS product_category_slug_format,
    DROP CONSTRAINT IF EXISTS product_category_level_range,
    DROP CONSTRAINT IF EXISTS product_category_level_parent;

DROP INDEX IF EXISTS uq_product_category_slug;

ALTER TABLE product_category
    DROP COLUMN IF EXISTS slug,
    DROP COLUMN IF EXISTS level;

DO $$
BEGIN
    IF to_regclass('public.schema_migrations') IS NOT NULL THEN
        DELETE FROM schema_migrations
        WHERE version = '036_unify_product_category_tree.sql';
    END IF;
END;
$$;

COMMIT;
