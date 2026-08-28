-- Schema mínimo para la API de productos.
-- Subconjunto del MER completo: solo las entidades necesarias para
-- listar productos y sus ofertas (precio por tienda).
-- Motor: PostgreSQL

CREATE TABLE IF NOT EXISTS brand (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    logo_url    TEXT,
    website_url TEXT,
    active      BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS product_category (
    id                  BIGSERIAL PRIMARY KEY,
    parent_category_id  BIGINT REFERENCES product_category(id),
    name                TEXT NOT NULL,
    description         TEXT,
    active              BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS product (
    id           BIGSERIAL PRIMARY KEY,
    public_id    UUID NOT NULL DEFAULT gen_random_uuid(),
    category_id  BIGINT REFERENCES product_category(id),
    brand_id     BIGINT REFERENCES brand(id),
    name         TEXT NOT NULL,
    model        TEXT,
    sku          TEXT,
    description  TEXT,
    active       BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_product_public_id ON product(public_id);
CREATE INDEX IF NOT EXISTS idx_product_category ON product(category_id);
CREATE INDEX IF NOT EXISTS idx_product_brand ON product(brand_id);
CREATE INDEX IF NOT EXISTS idx_product_name_trgm ON product USING gin (name gin_trgm_ops);

CREATE TABLE IF NOT EXISTS store (
    id                    BIGSERIAL PRIMARY KEY,
    name                  TEXT NOT NULL,
    website_url           TEXT,
    logo_url              TEXT,
    rating                NUMERIC(3,2),
    reputation            TEXT,
    shipping_information  TEXT,
    general_conditions    TEXT,
    active                BOOLEAN NOT NULL DEFAULT TRUE,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Un mismo producto, vendido por distintas tiendas, a distintos precios.
CREATE TABLE IF NOT EXISTS product_offer (
    id             BIGSERIAL PRIMARY KEY,
    product_id     BIGINT NOT NULL REFERENCES product(id) ON DELETE CASCADE,
    store_id       BIGINT NOT NULL REFERENCES store(id) ON DELETE CASCADE,
    price          NUMERIC(12,2) NOT NULL,
    list_price     NUMERIC(12,2),
    currency       TEXT NOT NULL DEFAULT 'CLP',
    shipping_cost  NUMERIC(12,2),
    shipping_free  BOOLEAN NOT NULL DEFAULT FALSE,
    available      BOOLEAN NOT NULL DEFAULT TRUE,
    stock          INTEGER,
    condition      TEXT NOT NULL DEFAULT 'new',
    product_url    TEXT,
    last_updated   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (product_id, store_id)
);

CREATE INDEX IF NOT EXISTS idx_offer_product ON product_offer(product_id);
CREATE INDEX IF NOT EXISTS idx_offer_store ON product_offer(store_id);
CREATE INDEX IF NOT EXISTS idx_offer_price ON product_offer(price);
