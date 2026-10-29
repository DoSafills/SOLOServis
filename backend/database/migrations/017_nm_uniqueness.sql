BEGIN;

ALTER TABLE store_location
    ADD CONSTRAINT store_location_store_location_key
    UNIQUE (store_id, location_id);

ALTER TABLE provider_location
    ADD CONSTRAINT provider_location_provider_location_key
    UNIQUE (provider_id, location_id);

ALTER TABLE product_category_specification
    ADD CONSTRAINT product_category_specification_category_name_key
    UNIQUE (category_id, name);

ALTER TABLE service_category_specification
    ADD CONSTRAINT service_category_specification_category_name_key
    UNIQUE (category_id, name);

COMMIT;
