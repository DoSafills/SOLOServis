BEGIN;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM product_price_history
        GROUP BY product_offer_id, price, recorded_at
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'duplicate product price history entries exist for offer, price and timestamp';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM product_category
        GROUP BY COALESCE(parent_category_id, -1), LOWER(BTRIM(name))
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'duplicate product category names exist under the same parent';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM store
        GROUP BY LOWER(BTRIM(name))
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'duplicate normalized store names exist';
    END IF;
END;
$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'product_price_history'::regclass
          AND conname = 'product_price_history_offer_price_recorded_at_key'
    ) THEN
        ALTER TABLE product_price_history
            ADD CONSTRAINT product_price_history_offer_price_recorded_at_key
            UNIQUE (product_offer_id, price, recorded_at);
    END IF;
END;
$$;

CREATE UNIQUE INDEX IF NOT EXISTS product_category_parent_name_normalized_key
    ON product_category (
        (COALESCE(parent_category_id, -1)),
        (LOWER(BTRIM(name)))
    );

CREATE UNIQUE INDEX IF NOT EXISTS store_name_normalized_key
    ON store (LOWER(BTRIM(name)));

CREATE INDEX IF NOT EXISTS idx_product_price_history_offer_recorded_at
    ON product_price_history (product_offer_id, recorded_at DESC);

COMMIT;
