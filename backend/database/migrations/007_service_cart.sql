ALTER TABLE cart_item
    ALTER COLUMN product_id DROP NOT NULL,
    ALTER COLUMN offer_id DROP NOT NULL,
    ADD COLUMN service_offer_id INTEGER REFERENCES service_offer(id) ON DELETE CASCADE,
    ADD CONSTRAINT cart_item_product_or_service CHECK (
        (product_id IS NOT NULL AND offer_id IS NOT NULL AND service_offer_id IS NULL)
        OR (product_id IS NULL AND offer_id IS NULL AND service_offer_id IS NOT NULL)
    );

CREATE UNIQUE INDEX IF NOT EXISTS idx_cart_item_user_service_offer
    ON cart_item(user_id, service_offer_id) WHERE service_offer_id IS NOT NULL;
