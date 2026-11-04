-- =====================================================================
-- TAL-125 (H5): product_user_interest la escriben dos APIs. Cada una es
-- dueña de sus columnas y solo apaga su propia marca (UPDATE ... = FALSE),
-- nunca borra la fila: quitar un favorito no borra la watchlist ni al revés.
--
-- Solo documenta en el catálogo de PostgreSQL (\d+ product_user_interest);
-- no cambia datos ni estructura.
-- Sin BEGIN/COMMIT propio (migrate.sh usa --single-transaction).
-- =====================================================================

COMMENT ON TABLE product_user_interest IS
    'Interés de un usuario en un producto. Una fila por (user_id, product_id), compartida por favorites y watchlist; ninguna borra filas.';

COMMENT ON COLUMN product_user_interest.is_favorite IS
    'Dueña: favorites-api (comercial). Escribe /favorites/{productId}.';

COMMENT ON COLUMN product_user_interest.target_price IS
    'Dueña: watchlist-api (comercial). Precio objetivo; pricing e ingestion lo leen para avisar price_drop.';

COMMENT ON COLUMN product_user_interest.notify_price_drop IS
    'Dueña: watchlist-api (comercial). TRUE = el producto está en la watchlist.';
