Paso 1 — Completar el schema que faltó (pégalo tal cual en tu psql actual)
sql
CREATE EXTENSION IF NOT EXISTS pg_trgm;

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
Paso 2 — Ahora sí, tus INSERTs de store y product_offer

Repite los que fallaron:

sql
INSERT INTO store (name, website_url) VALUES
  ('Falabella', 'https://falabella.com'),
  ('Paris', 'https://paris.cl'),
  ('MercadoLibre', 'https://mercadolibre.cl');

INSERT INTO product_offer (product_id, store_id, price, list_price, currency, shipping_free, stock, product_url)
VALUES
  (1, 1, 449990, 549990, 'CLP', true, 12, 'https://falabella.com/producto/tv-samsung'),
  (1, 2, 459990, 529990, 'CLP', false, 5, 'https://paris.cl/producto/tv-samsung'),
  (1, 3, 439990, 519990, 'CLP', true, 20, 'https://mercadolibre.cl/producto/tv-samsung');
Paso 3 — Salir de psql y probar la API desde PowerShell
sql
\q

Y ya en PowerShell (fuera de psql):

powershell
curl http://localhost:8080/api/v1/products
curl http://localhost:8080/api/v1/products/1

https://docs.google.com/document/d/12lkcnFE630TlPdlWWXBeK0Hk0lv9WYaVRRW_5fSrfco/edit?usp=sharing