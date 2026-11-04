-- =====================================================================
-- TAL-71: categorías unificadas (producto/servicio) y definiciones de
-- atributos dinámicos. Base para TAL-72 (validación), TAL-75 (filtros)
-- y TAL-76 (comparación).
--
-- Numerada 012 (y su seed 013) porque 009-011 los usa TAL-67/68 (PR #30).
-- Si ese PR introduce backend/database/migration_order.txt, agregar
-- 012_categories.sql y 013_categories_seed.sql al final del manifiesto.
--
-- Sin BEGIN/COMMIT propio: migrate.sh ejecuta cada archivo con
-- --single-transaction junto al registro en schema_migrations.
--
-- Las tablas product_category / service_category y sus *_specification
-- NO se modifican (las usan Product, Search y Comparison). Cada categoría
-- nueva puede apuntar a su equivalente antigua (legacy_*), para que
-- TAL-70/72 migren productos y servicios sin adivinar por nombre.
-- =====================================================================

CREATE TABLE categories (
    id                          SERIAL PRIMARY KEY,
    slug                        VARCHAR(64)  NOT NULL UNIQUE
                                CHECK (slug ~ '^[a-z0-9-]{1,64}$'),
    name                        VARCHAR(150) NOT NULL
                                CHECK (btrim(name) <> ''),
    item_type                   VARCHAR(10)  NOT NULL
                                CHECK (item_type IN ('product', 'service')),
    description                 TEXT,
    active                      BOOLEAN      NOT NULL DEFAULT TRUE,
    legacy_product_category_id  INTEGER UNIQUE
                                REFERENCES product_category(id) ON DELETE SET NULL,
    legacy_service_category_id  INTEGER UNIQUE
                                REFERENCES service_category(id) ON DELETE SET NULL,
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    -- El enlace legado debe ser del mismo tipo de ítem.
    CONSTRAINT categories_legacy_matches_item_type CHECK (
        (item_type = 'product' AND legacy_service_category_id IS NULL)
        OR (item_type = 'service' AND legacy_product_category_id IS NULL)
    )
);

CREATE TABLE category_attributes (
    id              SERIAL PRIMARY KEY,
    -- Las definiciones pertenecen a la categoría. Las futuras tablas de
    -- valores (TAL-70) deben referenciar esta tabla con ON DELETE RESTRICT.
    category_id     INTEGER      NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    key             VARCHAR(64)  NOT NULL
                    CHECK (key ~ '^[a-z][a-z0-9_]{0,63}$'),
    label           VARCHAR(150) NOT NULL
                    CHECK (btrim(label) <> ''),
    data_type       VARCHAR(10)  NOT NULL
                    CHECK (data_type IN ('string', 'integer', 'decimal', 'boolean', 'enum')),
    unit            VARCHAR(30),
    required        BOOLEAN      NOT NULL DEFAULT FALSE,
    comparable      BOOLEAN      NOT NULL DEFAULT TRUE,
    filterable      BOOLEAN      NOT NULL DEFAULT FALSE,
    min_value       NUMERIC,
    max_value       NUMERIC,
    max_length      INTEGER,
    allowed_values  TEXT[],
    display_order   INTEGER      NOT NULL DEFAULT 0,

    -- Cubre también la búsqueda por category_id (columna líder).
    CONSTRAINT category_attributes_category_key_unique UNIQUE (category_id, key),

    -- allowed_values existe si y solo si el tipo es enum, y no puede venir vacío.
    CONSTRAINT category_attributes_allowed_values_enum CHECK (
        (data_type = 'enum') = (allowed_values IS NOT NULL)
    ),
    CONSTRAINT category_attributes_allowed_values_not_empty CHECK (
        allowed_values IS NULL OR cardinality(allowed_values) >= 1
    ),

    -- min/max solo para numéricos, enteros en integer y ordenados.
    CONSTRAINT category_attributes_range_numeric CHECK (
        data_type IN ('integer', 'decimal')
        OR (min_value IS NULL AND max_value IS NULL)
    ),
    CONSTRAINT category_attributes_range_integer CHECK (
        data_type <> 'integer'
        OR ((min_value IS NULL OR min_value = trunc(min_value))
            AND (max_value IS NULL OR max_value = trunc(max_value)))
    ),
    CONSTRAINT category_attributes_range_order CHECK (
        min_value IS NULL OR max_value IS NULL OR min_value <= max_value
    ),

    -- max_length solo para string; tope 255 = largo de *_specification_value.value.
    CONSTRAINT category_attributes_max_length_string CHECK (
        max_length IS NULL
        OR (data_type = 'string' AND max_length BETWEEN 1 AND 255)
    ),

    -- La unidad solo tiene sentido en valores numéricos.
    CONSTRAINT category_attributes_unit_numeric CHECK (
        unit IS NULL OR data_type IN ('integer', 'decimal')
    )
);
