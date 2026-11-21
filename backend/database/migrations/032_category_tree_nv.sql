-- =====================================================================
-- 032: árbol de categorías de exactamente tres niveles (NV1 > NV2 > NV3).
--
-- Aditiva: no toca product_category, categories, category_attributes ni
-- product_attribute_value, y no cambia product.category_ref. Solo crea las
-- tablas nuevas y agrega product.category_nv3_id (NULL, FK RESTRICT, índice).
--
-- Reglas:
--   - tres tablas independientes; no existe NV4 ni se agrega después;
--   - FKs ON DELETE RESTRICT: no hay borrado en cascada;
--   - único: slug de NV1, (nv1_id, nombre sin tildes) en NV2,
--     (nv2_id, nombre sin tildes) en NV3 y slug de NV3 único global;
--   - el CHECK de slug reutiliza el formato de categories (012);
--   - los CHECK nuevos se agregan NOT VALID y no se validan aquí: rigen
--     las filas nuevas, no las históricas (las tablas nacen vacías).
--
-- Idempotente: IF NOT EXISTS en tablas, índices, columna y restricciones.
-- BEGIN/COMMIT propio: el runner no envuelve cada archivo en una transacción.
-- Reversa: backend/database/rollbacks/revert_032_033_category_tree_nv.sql
-- =====================================================================
BEGIN;

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

    CONSTRAINT categories_nv3_slug_key UNIQUE (slug)
);

-- Índices de las FKs (no los cubre el índice único del nombre).
CREATE INDEX IF NOT EXISTS idx_categories_nv2_nv1_id
    ON categories_nv2 (nv1_id);
CREATE INDEX IF NOT EXISTS idx_categories_nv3_nv2_id
    ON categories_nv3 (nv2_id);

-- Unicidad por (padre, nombre normalizado sin tildes), misma expresión que 030.
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

-- Vínculo del producto con su categoría hoja. Sigue siendo NULL hasta que
-- una fase posterior lo llene y lo decida; no se vuelve NOT NULL.
ALTER TABLE product ADD COLUMN IF NOT EXISTS category_nv3_id INTEGER;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
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

-- Formato de slug y nombre no vacío, igual que categories (012), pero
-- NOT VALID: sin VALIDATE en esta migración.
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

COMMIT;
