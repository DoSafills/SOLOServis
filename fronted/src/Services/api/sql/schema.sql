-- Esquema para la API de productos, ofertas por tienda y servicios

CREATE TABLE IF NOT EXISTS stores (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  logo TEXT,
  rating NUMERIC(2,1),
  review_count INTEGER DEFAULT 0,
  product_count INTEGER DEFAULT 0,
  reputation TEXT,
  dispatch_time TEXT,
  conditions TEXT,
  website TEXT
);

CREATE TABLE IF NOT EXISTS products (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  brand TEXT,
  model TEXT,
  category TEXT,
  subcategory TEXT,
  image TEXT,
  description TEXT,
  rating NUMERIC(2,1),
  review_count INTEGER DEFAULT 0,
  offer_price NUMERIC,
  tags TEXT[]
);

CREATE TABLE IF NOT EXISTS product_images (
  id SERIAL PRIMARY KEY,
  product_id TEXT REFERENCES products(id) ON DELETE CASCADE,
  url TEXT NOT NULL,
  position INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS product_specs (
  id SERIAL PRIMARY KEY,
  product_id TEXT REFERENCES products(id) ON DELETE CASCADE,
  spec_key TEXT NOT NULL,
  spec_value TEXT
);

CREATE TABLE IF NOT EXISTS offers (
  id SERIAL PRIMARY KEY,
  product_id TEXT REFERENCES products(id) ON DELETE CASCADE,
  store_id TEXT REFERENCES stores(id) ON DELETE CASCADE,
  price NUMERIC NOT NULL,
  available BOOLEAN DEFAULT TRUE,
  shipping NUMERIC
);

CREATE TABLE IF NOT EXISTS price_history (
  id SERIAL PRIMARY KEY,
  product_id TEXT REFERENCES products(id) ON DELETE CASCADE,
  date DATE NOT NULL,
  price NUMERIC NOT NULL
);

CREATE TABLE IF NOT EXISTS offer_price_history (
  id SERIAL PRIMARY KEY,
  product_id TEXT REFERENCES products(id) ON DELETE CASCADE,
  date DATE NOT NULL,
  price NUMERIC NOT NULL
);

CREATE TABLE IF NOT EXISTS services (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  provider TEXT,
  category TEXT,
  subcategory TEXT,
  description TEXT,
  monthly_price NUMERIC,
  installation_cost NUMERIC,
  contract_months INTEGER,
  rating NUMERIC(2,1),
  review_count INTEGER DEFAULT 0,
  coverage TEXT,
  image TEXT
);

CREATE TABLE IF NOT EXISTS service_specs (
  id SERIAL PRIMARY KEY,
  service_id TEXT REFERENCES services(id) ON DELETE CASCADE,
  spec_key TEXT NOT NULL,
  spec_value TEXT
);

CREATE TABLE IF NOT EXISTS service_benefits (
  id SERIAL PRIMARY KEY,
  service_id TEXT REFERENCES services(id) ON DELETE CASCADE,
  benefit TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS service_price_history (
  id SERIAL PRIMARY KEY,
  service_id TEXT REFERENCES services(id) ON DELETE CASCADE,
  date DATE NOT NULL,
  price NUMERIC NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_offers_product ON offers(product_id);
CREATE INDEX IF NOT EXISTS idx_offers_store ON offers(store_id);
CREATE INDEX IF NOT EXISTS idx_price_history_product ON price_history(product_id);
CREATE INDEX IF NOT EXISTS idx_offer_price_history_product ON offer_price_history(product_id);
CREATE INDEX IF NOT EXISTS idx_service_price_history_service ON service_price_history(service_id);
