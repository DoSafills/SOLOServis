-- Fixed indicative FX rates to CLP, sourced from ExchangeRate-API on 2026-11-02.
-- Rates are CLP per one unit of source currency; amounts are rounded to 2 decimals.
BEGIN;

CREATE OR REPLACE FUNCTION normalize_product_offer_currency_to_clp()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    source_currency TEXT := UPPER(BTRIM(NEW.currency));
    clp_per_unit NUMERIC;
BEGIN
    clp_per_unit := CASE source_currency
        WHEN 'CLP' THEN 1
        WHEN 'USD' THEN 990.838267
        WHEN 'EUR' THEN 1106.884487
        WHEN 'GBP' THEN 1327.730260
        WHEN 'CAD' THEN 709.317924
        WHEN 'AUD' THEN 696.380805
        WHEN 'CHF' THEN 1203.301938
        WHEN 'JPY' THEN 6.296872
        WHEN 'CNY' THEN 148.755925
        WHEN 'BRL' THEN 182.528068
        WHEN 'MXN' THEN 52.520264
        WHEN 'ARS' THEN 0.644511
        ELSE NULL
    END;

    IF clp_per_unit IS NULL THEN
        RAISE EXCEPTION 'Unsupported currency code for product offer: %', NEW.currency
            USING ERRCODE = '22023';
    END IF;

    NEW.price := ROUND(NEW.price * clp_per_unit, 2);
    IF NEW.list_price IS NOT NULL THEN
        NEW.list_price := ROUND(NEW.list_price * clp_per_unit, 2);
    END IF;
    IF NEW.shipping_cost IS NOT NULL THEN
        NEW.shipping_cost := ROUND(NEW.shipping_cost * clp_per_unit, 2);
    END IF;
    NEW.currency := 'CLP';

    RETURN NEW;
END;
$$;

COMMIT;
