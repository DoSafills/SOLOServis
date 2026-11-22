-- TAL-189: índices para las búsquedas que Ingestion hace por cada ítem.
--
-- FindProduct (internal/ingestion/repository.go) busca la oferta de la tienda
-- por su URL normalizada y cortada en el "?". Sin índice recorre todas las
-- ofertas de la tienda aplicando normalize_product_offer_url (plpgsql con
-- expresiones regulares) a cada una: con 20.000 ofertas por tienda tardaba
-- 568 ms por ítem; con este índice, 0,08 ms. La función es IMMUTABLE (009),
-- así que puede indexarse. La expresión debe ser idéntica a la de la consulta.
--
-- La búsqueda por marca y modelo compara lower(p.model): con el índice pasa
-- de 4,9 ms a 0,06 ms con 20.000 productos.
--
-- idx_product_price_history_offer (product_offer_id) es redundante: lo cubren
-- idx_product_price_history_offer_recorded_at (product_offer_id, recorded_at
-- DESC) y el único (product_offer_id, price, recorded_at). Quitarlo ahorra una
-- escritura de índice en cada cambio de precio.
--
-- Idempotente (IF [NOT] EXISTS). BEGIN/COMMIT propio: el runner no envuelve
-- el archivo en una transacción. Las tablas del clúster son chicas, así que
-- no hace falta CREATE INDEX CONCURRENTLY.
BEGIN;

CREATE INDEX IF NOT EXISTS idx_product_offer_store_normalized_url
    ON product_offer (store_id, split_part(normalize_product_offer_url(product_url), '?', 1));

CREATE INDEX IF NOT EXISTS idx_product_model_lower
    ON product (lower(model));

DROP INDEX IF EXISTS idx_product_price_history_offer;

COMMIT;
