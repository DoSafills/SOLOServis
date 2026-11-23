-- 046: índices para las consultas revisadas en 192_index_review.md (TAL-192).
--
-- Cubre: FKs sin índice de product_attribute_value y ai_comparison_query,
-- historial de búsquedas por usuario, favoritos por usuario, ofertas por
-- producto/servicio con disponibilidad y precio, reseñas por producto o
-- servicio y el historial de precios de servicios.
--
-- Además retira idx_user_session_refresh, duplicado exacto del índice que ya
-- crea user_session_refresh_token_hash_key. No se borra ningún otro índice.
--
-- Idempotente (IF NOT EXISTS / IF EXISTS) y sin CONCURRENTLY: migración
-- transaccional como el resto del runner.

BEGIN;

CREATE INDEX IF NOT EXISTS idx_product_attribute_value_product_category
    ON product_attribute_value (product_id, category_id);

CREATE INDEX IF NOT EXISTS idx_product_attribute_value_attribute_category
    ON product_attribute_value (attribute_id, category_id);

CREATE INDEX IF NOT EXISTS idx_ai_comparison_query_user
    ON ai_comparison_query (user_id);

CREATE INDEX IF NOT EXISTS idx_search_query_user_created
    ON search_query (user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_product_user_interest_user_updated_at
    ON product_user_interest (user_id, updated_at DESC)
    WHERE is_favorite;

CREATE INDEX IF NOT EXISTS idx_service_user_interest_user_updated_at
    ON service_user_interest (user_id, updated_at DESC)
    WHERE is_favorite;

CREATE INDEX IF NOT EXISTS idx_product_offer_product_available_price
    ON product_offer (product_id, available, price);

CREATE INDEX IF NOT EXISTS idx_service_offer_service_available_price
    ON service_offer (service_id, available, price);

CREATE INDEX IF NOT EXISTS idx_product_review_product_created
    ON product_review (product_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_service_review_service_created
    ON service_review (service_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_service_price_history_offer_recorded_at
    ON service_price_history (service_offer_id, recorded_at DESC);

DROP INDEX IF EXISTS idx_user_session_refresh;

COMMIT;
