CREATE TABLE IF NOT EXISTS saved_search (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES user_account(id) ON DELETE CASCADE,
    name VARCHAR(150) NOT NULL,
    query TEXT NOT NULL DEFAULT '',
    filters JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_session (
    id BIGSERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES user_account(id) ON DELETE CASCADE,
    access_token_hash BYTEA NOT NULL UNIQUE,
    refresh_token_hash BYTEA NOT NULL UNIQUE,
    access_expires_at TIMESTAMP NOT NULL,
    refresh_expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS cart_item (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES user_account(id) ON DELETE CASCADE,
    product_id INTEGER NOT NULL REFERENCES product(id) ON DELETE CASCADE,
    offer_id INTEGER NOT NULL REFERENCES product_offer(id) ON DELETE CASCADE,
    quantity INTEGER NOT NULL DEFAULT 1 CHECK (quantity > 0),
    saved_price NUMERIC(12,2) NOT NULL CHECK (saved_price >= 0),
    saved_shipping_cost NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (saved_shipping_cost >= 0),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, offer_id)
);

CREATE TABLE IF NOT EXISTS store_review (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES user_account(id) ON DELETE CASCADE,
    store_id INTEGER NOT NULL REFERENCES store(id) ON DELETE CASCADE,
    rating INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 5),
    title VARCHAR(200),
    content TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, store_id)
);

CREATE TABLE IF NOT EXISTS notification (
    id BIGSERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES user_account(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    title VARCHAR(200) NOT NULL,
    message TEXT NOT NULL,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    read_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ingestion_run (
    id BIGSERIAL PRIMARY KEY,
    source VARCHAR(150) NOT NULL DEFAULT 'manual',
    status VARCHAR(30) NOT NULL DEFAULT 'accepted',
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_saved_search_user ON saved_search(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_cart_item_user ON cart_item(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notification_user_unread ON notification(user_id, created_at DESC) WHERE read_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_user_session_refresh ON user_session(refresh_token_hash);