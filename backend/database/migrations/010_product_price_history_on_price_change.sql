BEGIN;

CREATE OR REPLACE FUNCTION record_product_offer_price_history()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    history_recorded_at TIMESTAMP;
    previous_recorded_at TIMESTAMP;
BEGIN
    IF TG_OP = 'INSERT' OR OLD.price IS DISTINCT FROM NEW.price THEN
        history_recorded_at := clock_timestamp()::timestamp;
        SELECT MAX(recorded_at)
        INTO previous_recorded_at
        FROM product_price_history
        WHERE product_offer_id = NEW.id
          AND price = NEW.price;

        IF previous_recorded_at IS NOT NULL
           AND history_recorded_at <= previous_recorded_at THEN
            history_recorded_at := previous_recorded_at + INTERVAL '1 microsecond';
        END IF;

        INSERT INTO product_price_history (product_offer_id, price, recorded_at)
        VALUES (NEW.id, NEW.price, history_recorded_at);
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER product_offer_price_history_after_change
AFTER INSERT OR UPDATE OF price ON product_offer
FOR EACH ROW
EXECUTE FUNCTION record_product_offer_price_history();

COMMIT;
