# TAL-192 — Revisión de índices (solo lectura)

Base: PostgreSQL 18 local de Compose (`soloservis`), consultada con
`PGOPTIONS='-c default_transaction_read_only=on'`. No se creó ni borró ningún
índice. `idx_scan` no se usa como criterio: la base local casi no tiene datos.

## Consultas revisadas

| consulta (archivo:línea) | columnas filtradas/ordenadas | índice existente | propuesta |
|---|---|---|---|
| `backend/database/queries/products.sql:99` ListProducts | `active`, `ORDER BY id` | sí (`product_pkey`) | ninguna |
| `backend/database/queries/products.sql:282` ListProductsByCategory | `active`, `category_id IN (...)` recursivo, `ORDER BY id` | sí (`idx_product_category`, `product_pkey`) | ninguna |
| `backend/database/queries/products.sql:54` GetProductByPublicID | `public_id` | sí (`product_public_id_key`) | ninguna |
| `backend/database/queries/products.sql:126` ListProductOffers | `product_id`, `ORDER BY price` | sí (`idx_product_offer_product` para el filtro; el orden se resuelve con sort) | `(product_id, available, price)` en `product_offer` |
| `backend/database/queries/products.sql:231` ListProductImages | `product_id`, `active`, `ORDER BY sort_order,id` | sí (`idx_product_image_product`) | ninguna |
| `backend/database/queries/products.sql:321` ListProductReviews | `product_id`, `ORDER BY created_at DESC, id DESC` | sí (`idx_product_review_product`; el orden se resuelve con sort) | `(product_id, created_at DESC, id DESC)` en `product_review` |
| `backend/database/queries/products.sql:461` ListProductSpecifications | `product_id` | sí (`product_specification_value_pkey`) | ninguna |
| `backend/database/queries/products.sql:472` ListProductPriceHistory | `po.product_id`, `ORDER BY pph.recorded_at` | sí (`idx_product_price_history_offer_recorded_at`) | ninguna |
| `backend/database/queries/services.sql:1` ListServices | `active`, `ORDER BY id` | sí (`service_pkey`) | ninguna |
| `backend/database/queries/services.sql:26` GetServiceByPublicID | `public_id`, `active` | sí (`service_public_id_key`) | ninguna |
| `backend/database/queries/services.sql:61` ListServiceOffers | `service_id`, `available`, `provider.active`, `ORDER BY price,id` | sí (`idx_service_offer_service` para el filtro; el orden se resuelve con sort) | `(service_id, available, price)` en `service_offer` |
| `backend/database/queries/services.sql:85` ListServicePriceHistory | `so.service_id`, `ORDER BY recorded_at` | sí (`idx_service_price_history_offer`; el orden se resuelve con sort) | `(service_offer_id, recorded_at DESC)` en `service_price_history` |
| `backend/database/queries/stores.sql:71` ListStoreProducts | `po.store_id`, `po.available`, `p.active`, `ORDER BY p.id` | sí (`idx_product_offer_store`) | `(store_id, available)` en `product_offer` |
| `backend/database/queries/stores.sql:120` ListStoreOffers | `po.store_id`, `p.active`, `ORDER BY po.id` | sí (`idx_product_offer_store`) | ninguna |
| `backend/internal/search/repository.go:41` matchedSQL (búsqueda de productos) | `p.active`, `category_id = ANY`, `brand_id = ANY`, `product_attribute_value(product_id,attribute_id)`, `product_offer(product_id,available,store_id)`, `strpos(search_text)` | sí para categoría, marca, atributos y ofertas; no para `strpos` | ninguna obligatoria; si crece el catálogo, GIN `pg_trgm` sobre `product.search_text` (requiere extensión, no instalada) |
| `backend/internal/search/repository.go:157` página de búsqueda | filtros de `matchedSQL` + `ORDER BY` calculado (precio, nombre, fecha) | sí para los filtros; el orden es por valor calculado | ninguna |
| `backend/internal/search/repository.go:216` PopularProducts | `search_query` agrupada por texto normalizado, `LIMIT` | no | ninguna (agregado sobre toda la tabla; un índice no evita el scan) |
| `backend/internal/search/repository.go:432` SearchCategories | `active`, `item_type`, `strpos(name || slug)` | parcial (`categories_slug_key` solo para igualdad) | ninguna (tabla pequeña); opcional `(active, item_type)` |
| `backend/internal/search/repository.go:457` SearchBrands | `active`, `strpos(lower(name))` | parcial (`brand_name_key` solo para igualdad) | ninguna (tabla pequeña) |
| `backend/internal/search/repository.go:129` CategoryDefinitions | `slug`, `active` | sí (`categories_slug_key`) | ninguna |
| `backend/internal/categories/repository.go:32`/`:38`/`:77` ResolveScope | `slug`, `legacy_*_category_id`, `id`, `active` | sí (`categories_slug_key`, `categories_legacy_*_key`, PK) | ninguna |
| `backend/internal/features/favorites_cart.go:20` ListFavorites | `user_id`, `is_favorite`, `ORDER BY updated_at DESC` | parcial: `user_id` cubierto por `product_user_interest_user_id_product_id_key`; el orden no | parcial `(user_id, updated_at DESC) WHERE is_favorite` |
| `backend/internal/features/favorites_cart.go:45` AddFavorite (upsert) | `ON CONFLICT (user_id, product_id)` | sí (`product_user_interest_user_id_product_id_key`) | ninguna |
| `backend/internal/features/favorites_cart.go:66` GetFavorite | `product_id`, `user_id` | sí (`product_user_interest_user_id_product_id_key`) | ninguna |
| `backend/internal/features/favorites_cart.go:105` ListCart | `user_id`, `ORDER BY created_at DESC` | sí (`idx_cart_item_user`) | ninguna |
| `backend/internal/features/favorites_cart.go:157` AddCartItem (producto) | `product_offer.id`, `product.id` | sí (PK) | ninguna |
| `backend/internal/features/favorites_cart.go:218` AddCartItem (servicio) | `service_offer.id`, `provider.id` | sí (PK) | ninguna |
| `backend/internal/features/favorites_cart.go:266`/`:288` Update/DeleteCartItem | `id`, `user_id` | sí (PK + filtro) | ninguna |
| `backend/internal/features/favorites_cart.go:309` CartCount | `user_id` | sí (`idx_cart_item_user`) | ninguna |
| `backend/internal/features/service_personalization.go:20` ListServiceFavorites | `user_id`, `is_favorite`, `ORDER BY updated_at DESC` | parcial: `user_id` cubierto por `service_user_interest_user_id_service_id_key`; el orden no | parcial `(user_id, updated_at DESC) WHERE is_favorite` |
| `backend/internal/features/service_personalization.go:180`/`:245` Get/CountServiceComparison | `comparison_id`, `user_id` | sí (`service_comparison_item_comparison_id_service_id_key`, PK) | ninguna |
| `backend/internal/features/catalog.go:42` limpieza de historial | `user_id`, `ORDER BY created_at DESC, id DESC`, `LIMIT` | parcial (`idx_search_query_user` solo por `user_id`) | `(user_id, created_at DESC, id DESC)` en `search_query` |
| `backend/internal/features/catalog.go:58` ListSearchHistory | `user_id`, `ORDER BY created_at DESC`, `LIMIT 100` | parcial (`idx_search_query_user`) | misma que arriba |
| `backend/internal/features/catalog.go:100` ListSavedSearches | `user_id`, `ORDER BY created_at DESC` | sí (`idx_saved_search_user`) | ninguna |
| `backend/internal/authz/authz.go:51` sesión por access token | `access_token_hash`, `access_expires_at` | sí (`user_session_access_token_hash_key`) | ninguna |
| `backend/internal/features/auth.go:317` rotación de refresh token | `refresh_token_hash`, `refresh_expires_at`, DELETE | sí (`user_session_refresh_token_hash_key`) | ninguna |
| `backend/internal/features/reviews_watch_notifications.go:40` reseñas de producto | `product_id`, `ORDER BY created_at DESC` | parcial (`idx_product_review_product`) | `(product_id, created_at DESC)` en `product_review` |
| `backend/internal/features/reviews_watch_notifications.go:106` reseñas de servicio | `service_id`, `ORDER BY created_at DESC` | parcial (`idx_service_review_service`) | `(service_id, created_at DESC)` en `service_review` |

## FK sin índice

| tabla | FK | columnas | propuesta |
|---|---|---|---|
| `product_attribute_value` | `product_attribute_value_product_fk` | `(product_id, category_id)` → `product(id, category_ref)` | `CREATE INDEX ON product_attribute_value (product_id, category_id)` |
| `product_attribute_value` | `product_attribute_value_attribute_fk` | `(attribute_id, category_id)` → `category_attributes(id, category_id)` | `CREATE INDEX ON product_attribute_value (attribute_id, category_id)`; dejaría redundante `idx_product_attribute_value_attribute` |
| `ai_comparison_query` | `ai_comparison_query_user_id_fkey` | `(user_id)` → `user_account(id)` | `CREATE INDEX ON ai_comparison_query (user_id)` |

## Índices duplicados o redundantes

| índice | relación | motivo |
|---|---|---|
| `user_session_refresh_token_hash_key` + `idx_user_session_refresh` | duplicado exacto sobre `refresh_token_hash` | el UNIQUE ya crea el índice |
| `idx_product_offer_product` | redundante con `product_offer_product_store_normalized_url_key (product_id, store_id, expr)` | `product_id` es prefijo del único |
| `idx_product_offer_store` | redundante con `idx_product_offer_store_normalized_url (store_id, expr)` | `store_id` es prefijo |
| `idx_service_offer_service` | redundante con `service_offer_service_id_provider_id_key (service_id, provider_id)` | `service_id` es prefijo |
| `idx_product_price_history_offer` | redundante con `idx_product_price_history_offer_recorded_at (product_offer_id, recorded_at DESC)` | `product_offer_id` es prefijo |
| `idx_scrape_run_scraper_config` | redundante con `idx_scrape_run_config_started (scraper_config_id, started_at DESC)` | `scraper_config_id` es prefijo |
| `product_category_parent_name_normalized_key` + `product_category_parent_name_ascii_key` | casi duplicados | mismas columnas de partición; cambia solo la normalización (`lower(btrim)` vs `translate`) |
| `idx_search_query_user` | quedará redundante si se crea `(user_id, created_at DESC, id DESC)` | `user_id` es prefijo |
| `idx_product_attribute_value_attribute` | quedará redundante si se crea `(attribute_id, category_id)` | `attribute_id` es prefijo |

No se propone borrar `product_id_category_ref_unique (id, category_ref)`: respalda la FK compuesta de `product_attribute_value`.
