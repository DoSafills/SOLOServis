-- Backup previo de filas que podrían violar las restricciones objetivo.
CREATE TABLE IF NOT EXISTS public.backup_021_cart_item_indexes AS
SELECT ci.*, now() AS backed_up_at
FROM public.cart_item ci
WHERE (ci.product_id IS NULL AND ci.service_offer_id IS NULL)
   OR (ci.product_id IS NOT NULL AND ci.service_offer_id IS NOT NULL)
   OR EXISTS (
       SELECT 1
       FROM public.product_offer po
       WHERE po.id = ci.offer_id
         AND po.product_id IS DISTINCT FROM ci.product_id
   );

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_cart_item_user_target
    ON public.cart_item
       (user_id, product_id, offer_id, service_offer_id)
    NULLS NOT DISTINCT;

-- Rollback:
-- DROP INDEX CONCURRENTLY IF EXISTS public.uq_cart_item_user_target;
