-- =====================================================================
-- SOLOServis - Script de creación de base de datos (DDL)
-- Motor: PostgreSQL 14+
-- Generado a partir del MER normalizado - Fase 1 (IA y scraping diferidos)
-- =====================================================================

-- Extensiones necesarias
CREATE EXTENSION IF NOT EXISTS "pgcrypto";   -- para gen_random_uuid()

-- =====================================================================
-- TIPOS ENUMERADOS
-- =====================================================================

-- DECISIÓN: 'provider_manager' habilita que tiendas/proveedores gestionen sus propias
-- ofertas dentro de la plataforma a futuro (modelo marketplace), acorde al objetivo del proyecto.
CREATE TYPE user_role AS ENUM ('admin', 'client', 'provider_manager');

-- =====================================================================
-- BLOQUE: USUARIOS
-- =====================================================================

CREATE TABLE user_account (
    id             SERIAL PRIMARY KEY,
    public_id      UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    role           user_role NOT NULL DEFAULT 'client',
    name           VARCHAR(150) NOT NULL,
    email          VARCHAR(255) NOT NULL UNIQUE,
    password_hash  TEXT NOT NULL,
    active         BOOLEAN NOT NULL DEFAULT TRUE,
    created_at     TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMP NOT NULL DEFAULT NOW()
);

-- =====================================================================
-- BLOQUE: CATÁLOGO DE PRODUCTOS
-- =====================================================================

CREATE TABLE brand (
    id           SERIAL PRIMARY KEY,
    name         VARCHAR(150) NOT NULL UNIQUE,
    logo_url     TEXT,
    website_url  TEXT,
    active       BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE product_category (
    id                  SERIAL PRIMARY KEY,
    parent_category_id  INTEGER REFERENCES product_category(id) ON DELETE SET NULL,
    name                VARCHAR(150) NOT NULL,
    description         TEXT,
    active              BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE product (
    id                        SERIAL PRIMARY KEY,
    public_id                 UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    category_id               INTEGER NOT NULL REFERENCES product_category(id) ON DELETE RESTRICT,
    brand_id                  INTEGER REFERENCES brand(id) ON DELETE SET NULL,
    name                      VARCHAR(255) NOT NULL,
    model                     VARCHAR(150),
    sku                       VARCHAR(100) UNIQUE,
    description               TEXT,
    -- derived_average_rating y derived_review_count NO se almacenan aquí.
    -- Se calculan mediante la vista product_rating_summary (ver 003_views.sql)
    active                    BOOLEAN NOT NULL DEFAULT TRUE,
    created_at                TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at                TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE product_image (
    id           SERIAL PRIMARY KEY,
    product_id   INTEGER NOT NULL REFERENCES product(id) ON DELETE CASCADE,
    image_url    TEXT NOT NULL,
    alt_text     VARCHAR(255),
    sort_order   INTEGER NOT NULL DEFAULT 0,
    active       BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE product_category_specification (
    id             SERIAL PRIMARY KEY,
    category_id    INTEGER NOT NULL REFERENCES product_category(id) ON DELETE CASCADE,
    name           VARCHAR(150) NOT NULL,
    data_type      VARCHAR(50) NOT NULL, -- ej: 'string','number','boolean'
    unit           VARCHAR(50),
    required       BOOLEAN NOT NULL DEFAULT FALSE,
    comparable     BOOLEAN NOT NULL DEFAULT TRUE,
    display_order  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE product_specification_value (
    id                SERIAL PRIMARY KEY,
    product_id        INTEGER NOT NULL REFERENCES product(id) ON DELETE CASCADE,
    specification_id  INTEGER NOT NULL REFERENCES product_category_specification(id) ON DELETE CASCADE,
    value             VARCHAR(255) NOT NULL,
    UNIQUE (product_id, specification_id)
);

-- =====================================================================
-- BLOQUE: TIENDAS Y OFERTAS DE PRODUCTOS
-- =====================================================================

CREATE TABLE store (
    id                     SERIAL PRIMARY KEY,
    name                   VARCHAR(150) NOT NULL,
    website_url            TEXT,
    logo_url               TEXT,
    rating                 NUMERIC(3,2),
    reputation             VARCHAR(100),
    shipping_information   TEXT,
    general_conditions     TEXT,
    active                 BOOLEAN NOT NULL DEFAULT TRUE,
    created_at             TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE product_offer (
    id               SERIAL PRIMARY KEY,
    product_id       INTEGER NOT NULL REFERENCES product(id) ON DELETE CASCADE,
    store_id         INTEGER NOT NULL REFERENCES store(id) ON DELETE CASCADE,
    price            NUMERIC(12,2) NOT NULL CHECK (price >= 0),
    list_price       NUMERIC(12,2) CHECK (list_price >= 0),
    currency         CHAR(3) NOT NULL DEFAULT 'CLP',
    shipping_cost    NUMERIC(12,2) DEFAULT 0 CHECK (shipping_cost >= 0),
    shipping_free    BOOLEAN NOT NULL DEFAULT FALSE,
    available        BOOLEAN NOT NULL DEFAULT TRUE,
    stock            INTEGER CHECK (stock >= 0),
    -- DECISIÓN: valores estándar de condición en e-commerce
    condition        VARCHAR(20) NOT NULL DEFAULT 'new'
                       CHECK (condition IN ('new', 'used', 'refurbished')),
    product_url      TEXT,
    last_updated     TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (product_id, store_id)
);

CREATE TABLE product_price_history (
    id                SERIAL PRIMARY KEY,
    product_offer_id  INTEGER NOT NULL REFERENCES product_offer(id) ON DELETE CASCADE,
    price             NUMERIC(12,2) NOT NULL CHECK (price >= 0),
    is_promotional    BOOLEAN NOT NULL DEFAULT FALSE,
    recorded_at       TIMESTAMP NOT NULL DEFAULT NOW()
);

-- =====================================================================
-- BLOQUE: COMPARACIONES E INTERESES DE PRODUCTOS
-- =====================================================================

CREATE TABLE product_comparison (
    id            SERIAL PRIMARY KEY,
    user_id       INTEGER NOT NULL REFERENCES user_account(id) ON DELETE CASCADE,
    category_id   INTEGER NOT NULL REFERENCES product_category(id) ON DELETE RESTRICT,
    name          VARCHAR(150) NOT NULL,
    created_at    TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE product_comparison_item (
    id              SERIAL PRIMARY KEY,
    comparison_id   INTEGER NOT NULL REFERENCES product_comparison(id) ON DELETE CASCADE,
    product_id      INTEGER NOT NULL REFERENCES product(id) ON DELETE CASCADE,
    position        INTEGER NOT NULL DEFAULT 0,
    added_at        TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (comparison_id, product_id)
);

CREATE TABLE product_user_interest (
    id                  SERIAL PRIMARY KEY,
    user_id             INTEGER NOT NULL REFERENCES user_account(id) ON DELETE CASCADE,
    product_id          INTEGER NOT NULL REFERENCES product(id) ON DELETE CASCADE,
    target_price        NUMERIC(12,2) CHECK (target_price >= 0),
    notify_price_drop   BOOLEAN NOT NULL DEFAULT FALSE,
    is_favorite         BOOLEAN NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, product_id)
);

CREATE TABLE product_review (
    id            SERIAL PRIMARY KEY,
    user_id       INTEGER NOT NULL REFERENCES user_account(id) ON DELETE CASCADE,
    product_id    INTEGER NOT NULL REFERENCES product(id) ON DELETE CASCADE,
    rating        INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 5),
    title         VARCHAR(200),
    content       TEXT,
    verified      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, product_id)
);

-- =====================================================================
-- BLOQUE: CATÁLOGO DE SERVICIOS
-- =====================================================================

-- DECISIÓN: el MER referencia service_type_id sin definir la tabla de origen.
-- Se crea esta tabla clasificando servicios por su naturaleza de entrega/cobro
-- (independiente de billing_period, que ya cubre la frecuencia de pago en SERVICE_OFFER).
CREATE TABLE service_type (
    id     SERIAL PRIMARY KEY,
    name   VARCHAR(100) NOT NULL UNIQUE
);

CREATE TABLE service_category (
    id                   SERIAL PRIMARY KEY,
    service_type_id      INTEGER NOT NULL REFERENCES service_type(id) ON DELETE RESTRICT,
    parent_category_id   INTEGER REFERENCES service_category(id) ON DELETE SET NULL,
    name                 VARCHAR(150) NOT NULL,
    description          TEXT,
    active               BOOLEAN NOT NULL DEFAULT TRUE,
    created_at           TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE service_category_specification (
    id             SERIAL PRIMARY KEY,
    category_id    INTEGER NOT NULL REFERENCES service_category(id) ON DELETE CASCADE,
    name           VARCHAR(150) NOT NULL,
    data_type      VARCHAR(50) NOT NULL,
    unit           VARCHAR(50),
    required       BOOLEAN NOT NULL DEFAULT FALSE,
    comparable     BOOLEAN NOT NULL DEFAULT TRUE,
    display_order  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE service (
    id            SERIAL PRIMARY KEY,
    public_id     UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    category_id   INTEGER NOT NULL REFERENCES service_category(id) ON DELETE RESTRICT,
    name          VARCHAR(255) NOT NULL,
    description   TEXT,
    image_url     TEXT,
    -- derived_average_rating y derived_review_count NO se almacenan aquí.
    -- Se calculan mediante la vista service_rating_summary (ver 003_views.sql)
    active        BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE service_specification_value (
    id                SERIAL PRIMARY KEY,
    service_id        INTEGER NOT NULL REFERENCES service(id) ON DELETE CASCADE,
    specification_id  INTEGER NOT NULL REFERENCES service_category_specification(id) ON DELETE CASCADE,
    value             VARCHAR(255) NOT NULL,
    UNIQUE (service_id, specification_id)
);

-- =====================================================================
-- BLOQUE: PROVEEDORES Y OFERTAS DE SERVICIOS
-- =====================================================================

CREATE TABLE provider (
    id                   SERIAL PRIMARY KEY,
    name                 VARCHAR(150) NOT NULL,
    website_url          TEXT,
    logo_url             TEXT,
    rating               NUMERIC(3,2),
    reputation           VARCHAR(100),
    general_conditions   TEXT,
    active               BOOLEAN NOT NULL DEFAULT TRUE,
    created_at           TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE service_offer (
    id                        SERIAL PRIMARY KEY,
    service_id                INTEGER NOT NULL REFERENCES service(id) ON DELETE CASCADE,
    provider_id               INTEGER NOT NULL REFERENCES provider(id) ON DELETE CASCADE,
    price                     NUMERIC(12,2) NOT NULL CHECK (price >= 0),
    currency                  CHAR(3) NOT NULL DEFAULT 'CLP',
    billing_period            VARCHAR(30), -- ej: 'monthly','yearly','one_time'
    installation_cost         NUMERIC(12,2) CHECK (installation_cost >= 0),
    contract_period           VARCHAR(50),
    available                 BOOLEAN NOT NULL DEFAULT TRUE,
    coverage_summary          TEXT,
    additional_costs_summary  TEXT,
    service_url               TEXT,
    last_updated              TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (service_id, provider_id)
);

CREATE TABLE service_price_history (
    id                SERIAL PRIMARY KEY,
    service_offer_id  INTEGER NOT NULL REFERENCES service_offer(id) ON DELETE CASCADE,
    price             NUMERIC(12,2) NOT NULL CHECK (price >= 0),
    is_promotional    BOOLEAN NOT NULL DEFAULT FALSE,
    recorded_at       TIMESTAMP NOT NULL DEFAULT NOW()
);

-- =====================================================================
-- BLOQUE: COMPARACIONES E INTERESES DE SERVICIOS
-- =====================================================================

CREATE TABLE service_comparison (
    id            SERIAL PRIMARY KEY,
    user_id       INTEGER NOT NULL REFERENCES user_account(id) ON DELETE CASCADE,
    category_id   INTEGER NOT NULL REFERENCES service_category(id) ON DELETE RESTRICT,
    name          VARCHAR(150) NOT NULL,
    created_at    TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE service_comparison_item (
    id              SERIAL PRIMARY KEY,
    comparison_id   INTEGER NOT NULL REFERENCES service_comparison(id) ON DELETE CASCADE,
    service_id      INTEGER NOT NULL REFERENCES service(id) ON DELETE CASCADE,
    position        INTEGER NOT NULL DEFAULT 0,
    added_at        TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (comparison_id, service_id)
);

CREATE TABLE service_user_interest (
    id                  SERIAL PRIMARY KEY,
    user_id             INTEGER NOT NULL REFERENCES user_account(id) ON DELETE CASCADE,
    service_id          INTEGER NOT NULL REFERENCES service(id) ON DELETE CASCADE,
    target_price        NUMERIC(12,2) CHECK (target_price >= 0),
    notify_price_drop   BOOLEAN NOT NULL DEFAULT FALSE,
    is_favorite         BOOLEAN NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, service_id)
);

CREATE TABLE service_review (
    id            SERIAL PRIMARY KEY,
    user_id       INTEGER NOT NULL REFERENCES user_account(id) ON DELETE CASCADE,
    service_id    INTEGER NOT NULL REFERENCES service(id) ON DELETE CASCADE,
    rating        INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 5),
    title         VARCHAR(200),
    content       TEXT,
    verified      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, service_id)
);

-- =====================================================================
-- BLOQUE: UBICACIONES
-- =====================================================================

CREATE TABLE location (
    id          SERIAL PRIMARY KEY,
    country     VARCHAR(100) NOT NULL,
    region      VARCHAR(100),
    city        VARCHAR(100),
    commune     VARCHAR(100),
    latitude    NUMERIC(9,6),
    longitude   NUMERIC(9,6)
);

CREATE TABLE store_location (
    id           SERIAL PRIMARY KEY,
    store_id     INTEGER NOT NULL REFERENCES store(id) ON DELETE CASCADE,
    location_id  INTEGER NOT NULL REFERENCES location(id) ON DELETE CASCADE,
    address      VARCHAR(255),
    postal_code  VARCHAR(20),
    active       BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE provider_location (
    id               SERIAL PRIMARY KEY,
    provider_id      INTEGER NOT NULL REFERENCES provider(id) ON DELETE CASCADE,
    location_id      INTEGER NOT NULL REFERENCES location(id) ON DELETE CASCADE,
    address          VARCHAR(255),
    postal_code      VARCHAR(20),
    service_radius   VARCHAR(50), -- ej: '10km', 'comuna completa'
    active           BOOLEAN NOT NULL DEFAULT TRUE
);

-- =====================================================================
-- BLOQUE: BÚSQUEDA E IA
-- =====================================================================

CREATE TABLE search_query (
    id           SERIAL PRIMARY KEY,
    user_id      INTEGER REFERENCES user_account(id) ON DELETE SET NULL,
    raw_query    TEXT NOT NULL,
    search_type  VARCHAR(30), -- ej: 'product','service','mixed'
    created_at   TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE search_query_filter (
    id                SERIAL PRIMARY KEY,
    search_query_id   INTEGER NOT NULL REFERENCES search_query(id) ON DELETE CASCADE,
    filter_name       VARCHAR(100) NOT NULL,
    filter_value      VARCHAR(255) NOT NULL
);

-- Relación 1:0..1 del MER: cada búsqueda puede tener a lo más un registro de procesamiento IA
CREATE TABLE ai_query_process (
    id                 SERIAL PRIMARY KEY,
    search_query_id    INTEGER NOT NULL UNIQUE REFERENCES search_query(id) ON DELETE CASCADE,
    intent             VARCHAR(100),
    entity_type        VARCHAR(100),
    structured_json    JSONB,
    response_time_ms   INTEGER CHECK (response_time_ms >= 0),
    success            BOOLEAN NOT NULL DEFAULT TRUE,
    error_message      TEXT,
    created_at         TIMESTAMP NOT NULL DEFAULT NOW()
);

-- =====================================================================
-- BLOQUE: SCRAPING (diferido en Fase 1, tablas listas para uso futuro)
-- =====================================================================

CREATE TABLE data_source (
    id           SERIAL PRIMARY KEY,
    name         VARCHAR(150) NOT NULL UNIQUE,
    base_url     TEXT,
    source_type  VARCHAR(50), -- ej: 'store','provider'
    active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE scraper_config (
    id              SERIAL PRIMARY KEY,
    data_source_id  INTEGER NOT NULL REFERENCES data_source(id) ON DELETE CASCADE,
    scraper_type    VARCHAR(50),  -- ej: 'scrapy','playwright'
    target_type     VARCHAR(50),  -- ej: 'product','service'
    schedule        VARCHAR(100), -- ej: expresión cron
    rate_limit      VARCHAR(50),
    parser_config   JSONB,
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE scrape_run (
    id                  SERIAL PRIMARY KEY,
    scraper_config_id   INTEGER NOT NULL REFERENCES scraper_config(id) ON DELETE CASCADE,
    started_at          TIMESTAMP NOT NULL DEFAULT NOW(),
    finished_at         TIMESTAMP,
    records_found       INTEGER DEFAULT 0 CHECK (records_found >= 0),
    records_processed   INTEGER DEFAULT 0 CHECK (records_processed >= 0),
    records_failed      INTEGER DEFAULT 0 CHECK (records_failed >= 0),
    -- DECISIÓN: valores estándar de estado para un job/proceso de scraping
    status              VARCHAR(20) NOT NULL DEFAULT 'pending'
                          CHECK (status IN ('pending', 'running', 'success', 'failed')),
    error_message       TEXT
);

CREATE TABLE scraped_data (
    id              SERIAL PRIMARY KEY,
    scrape_run_id   INTEGER NOT NULL REFERENCES scrape_run(id) ON DELETE CASCADE,
    external_id     VARCHAR(255),
    external_url    TEXT,
    entity_type     VARCHAR(50), -- ej: 'product','service'
    raw_data        JSONB,
    extracted_at    TIMESTAMP NOT NULL DEFAULT NOW()
);

-- =====================================================================
-- ÍNDICES DE APOYO (búsquedas y filtros más comunes)
-- =====================================================================

CREATE INDEX idx_product_category ON product(category_id);
CREATE INDEX idx_product_brand ON product(brand_id);
CREATE INDEX idx_product_offer_product ON product_offer(product_id);
CREATE INDEX idx_product_offer_store ON product_offer(store_id);
CREATE INDEX idx_product_price_history_offer ON product_price_history(product_offer_id);
CREATE INDEX idx_service_category ON service(category_id);
CREATE INDEX idx_service_offer_service ON service_offer(service_id);
CREATE INDEX idx_service_offer_provider ON service_offer(provider_id);
CREATE INDEX idx_service_price_history_offer ON service_price_history(service_offer_id);
CREATE INDEX idx_product_review_product ON product_review(product_id);
CREATE INDEX idx_service_review_service ON service_review(service_id);
CREATE INDEX idx_search_query_user ON search_query(user_id);
CREATE INDEX idx_scraped_data_run ON scraped_data(scrape_run_id);