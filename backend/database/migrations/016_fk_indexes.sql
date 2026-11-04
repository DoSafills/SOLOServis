BEGIN;

CREATE INDEX IF NOT EXISTS idx_product_image_product
    ON product_image(product_id);
CREATE INDEX IF NOT EXISTS idx_product_user_interest_product
    ON product_user_interest(product_id);
CREATE INDEX IF NOT EXISTS idx_service_user_interest_service
    ON service_user_interest(service_id);
CREATE INDEX IF NOT EXISTS idx_user_session_user
    ON user_session(user_id);
CREATE INDEX IF NOT EXISTS idx_cart_item_product
    ON cart_item(product_id);
CREATE INDEX IF NOT EXISTS idx_cart_item_offer
    ON cart_item(offer_id);
CREATE INDEX IF NOT EXISTS idx_cart_item_service_offer
    ON cart_item(service_offer_id);
CREATE INDEX IF NOT EXISTS idx_product_comparison_user
    ON product_comparison(user_id);
CREATE INDEX IF NOT EXISTS idx_service_comparison_user
    ON service_comparison(user_id);
CREATE INDEX IF NOT EXISTS idx_product_comparison_category
    ON product_comparison(category_id);
CREATE INDEX IF NOT EXISTS idx_service_comparison_category
    ON service_comparison(category_id);
CREATE INDEX IF NOT EXISTS idx_product_comparison_item_product
    ON product_comparison_item(product_id);
CREATE INDEX IF NOT EXISTS idx_service_comparison_item_service
    ON service_comparison_item(service_id);
CREATE INDEX IF NOT EXISTS idx_store_location_location
    ON store_location(location_id);
CREATE INDEX IF NOT EXISTS idx_provider_location_location
    ON provider_location(location_id);
CREATE INDEX IF NOT EXISTS idx_search_query_filter_query
    ON search_query_filter(search_query_id);
CREATE INDEX IF NOT EXISTS idx_store_review_store
    ON store_review(store_id);
CREATE INDEX IF NOT EXISTS idx_scraper_config_source
    ON scraper_config(data_source_id);
CREATE INDEX IF NOT EXISTS idx_product_category_parent
    ON product_category(parent_category_id);
CREATE INDEX IF NOT EXISTS idx_service_category_parent
    ON service_category(parent_category_id);
CREATE INDEX IF NOT EXISTS idx_service_category_type
    ON service_category(service_type_id);
CREATE INDEX IF NOT EXISTS idx_psv_specification
    ON product_specification_value(specification_id);
CREATE INDEX IF NOT EXISTS idx_ssv_specification
    ON service_specification_value(specification_id);

COMMIT;
