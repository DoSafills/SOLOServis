BEGIN;

-- Temporary 1:1 conversion: retain each numeric amount and normalize its
-- currency label to CLP until real Chilean prices or exchange rates are available.

CREATE OR REPLACE FUNCTION normalize_product_offer_currency_to_clp()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.currency := 'CLP';
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION normalize_service_offer_currency_to_clp()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.currency := 'CLP';
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS product_offer_currency_to_clp ON product_offer;
CREATE TRIGGER product_offer_currency_to_clp
BEFORE INSERT OR UPDATE ON product_offer
FOR EACH ROW
EXECUTE FUNCTION normalize_product_offer_currency_to_clp();

DROP TRIGGER IF EXISTS service_offer_currency_to_clp ON service_offer;
CREATE TRIGGER service_offer_currency_to_clp
BEFORE INSERT OR UPDATE ON service_offer
FOR EACH ROW
EXECUTE FUNCTION normalize_service_offer_currency_to_clp();

UPDATE product_offer
SET currency = 'CLP'
WHERE UPPER(BTRIM(currency)) <> 'CLP';

UPDATE service_offer
SET currency = 'CLP'
WHERE UPPER(BTRIM(currency)) <> 'CLP';

COMMIT;
