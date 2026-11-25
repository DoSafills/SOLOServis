-- =====================================================================
-- TAL-190: integridad referencial y rangos críticos.
--
-- Alcance reducido y acordado:
--   1. rating de store y provider en 0..5 (hoy NUMERIC(3,2) acepta 99.99);
--   2. una categoría no puede ser su propio padre;
--   3. precio estrictamente positivo en los historiales;
--   4. índice por categoría para los filtros de product_attribute_value.
--
-- Todas las restricciones entran como NOT VALID: se aplican a filas nuevas
-- de inmediato y quedan pendientes de validar sobre datos existentes (ver
-- plan posterior). No toca ofertas, carrito, monedas, list_price ni
-- posiciones de comparación.
--
-- Idempotente: cada bloque revisa pg_constraint antes de agregar.
-- Sin BEGIN/COMMIT propio (migrate.sh usa --single-transaction).
-- =====================================================================

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'store'::regclass
          AND conname = 'store_rating_range'
    ) THEN
        ALTER TABLE store
            ADD CONSTRAINT store_rating_range
            CHECK (rating BETWEEN 0 AND 5) NOT VALID;
    END IF;
END;
$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'provider'::regclass
          AND conname = 'provider_rating_range'
    ) THEN
        ALTER TABLE provider
            ADD CONSTRAINT provider_rating_range
            CHECK (rating BETWEEN 0 AND 5) NOT VALID;
    END IF;
END;
$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'product_category'::regclass
          AND conname = 'product_category_parent_not_self'
    ) THEN
        ALTER TABLE product_category
            ADD CONSTRAINT product_category_parent_not_self
            CHECK (parent_category_id IS NULL OR parent_category_id <> id) NOT VALID;
    END IF;
END;
$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'service_category'::regclass
          AND conname = 'service_category_parent_not_self'
    ) THEN
        ALTER TABLE service_category
            ADD CONSTRAINT service_category_parent_not_self
            CHECK (parent_category_id IS NULL OR parent_category_id <> id) NOT VALID;
    END IF;
END;
$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'product_price_history'::regclass
          AND conname = 'product_price_history_price_positive'
    ) THEN
        ALTER TABLE product_price_history
            ADD CONSTRAINT product_price_history_price_positive
            CHECK (price > 0) NOT VALID;
    END IF;
END;
$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'service_price_history'::regclass
          AND conname = 'service_price_history_price_positive'
    ) THEN
        ALTER TABLE service_price_history
            ADD CONSTRAINT service_price_history_price_positive
            CHECK (price > 0) NOT VALID;
    END IF;
END;
$$;

-- Filtros por categoría (TAL-75) y chequeo de la FK compuesta por categoría.
CREATE INDEX IF NOT EXISTS idx_product_attribute_value_category
    ON product_attribute_value(category_id);
