CREATE TABLE IF NOT EXISTS public.backup_022_cart_item_constraints AS
SELECT ci.*, now() AS backed_up_at
FROM public.cart_item ci
WHERE ci.offer_id IS NOT NULL
  AND NOT EXISTS (
      SELECT 1
      FROM public.product_offer po
      WHERE po.id = ci.offer_id
        AND po.product_id = ci.product_id
  );

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_offer_id_product
    ON public.product_offer (id, product_id);

ALTER TABLE public.product_offer
    ADD CONSTRAINT uq_offer_id_product
    UNIQUE USING INDEX uq_offer_id_product;

ALTER TABLE public.cart_item
    ADD CONSTRAINT fk_cart_item_offer_product
    FOREIGN KEY (offer_id, product_id)
    REFERENCES public.product_offer (id, product_id)
    ON DELETE CASCADE
    NOT VALID;

ALTER TABLE public.cart_item
    VALIDATE CONSTRAINT fk_cart_item_offer_product;

-- Cart_item ya tiene el CHECK cart_item_product_or_service,
-- equivalente y más estricto que el XOR solicitado.
-- Rollback (orden inverso):
-- ALTER TABLE public.cart_item DROP CONSTRAINT IF EXISTS fk_cart_item_offer_product;
-- ALTER TABLE public.product_offer DROP CONSTRAINT IF EXISTS uq_offer_id_product;
