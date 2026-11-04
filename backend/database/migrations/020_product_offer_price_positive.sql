CREATE TABLE IF NOT EXISTS public.backup_020_product_offer_price_positive AS
SELECT po.*, now() AS backed_up_at
FROM public.product_offer po
WHERE po.price <= 0;

ALTER TABLE public.product_offer
    ADD CONSTRAINT product_offer_price_positive_check
    CHECK (price > 0 AND price = trunc(price)) NOT VALID;

ALTER TABLE public.product_offer
    VALIDATE CONSTRAINT product_offer_price_positive_check;

-- Rollback:
-- ALTER TABLE public.product_offer DROP CONSTRAINT IF EXISTS product_offer_price_positive_check;
