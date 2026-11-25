-- =====================================================================
-- TAL-191 (issue #191): ingesta de servicios desde el scraper.
--
-- 1. provider.slug: identidad estable del proveedor para el contrato de
--    ingesta (antes solo existía el nombre). Se rellena desde el nombre y
--    queda único; el índice por nombre normalizado de 044 sigue vigente.
-- 2. service.category_ref: categoría unificada de TAL-71, igual que
--    product.category_ref, para validar y guardar atributos dinámicos.
-- 3. service_attribute_value: espejo de product_attribute_value. Los
--    servicios nuevos escriben aquí y dejan de usar la tabla legada
--    service_specification_value (se mantiene para lo ya cargado).
-- 4. Índice de identidad de oferta de servicio: proveedor + URL sin query,
--    que es como Ingestion reconoce un plan ya catalogado.
--
-- Idempotente: si la columna, la tabla o el índice ya existen, no hace nada.
-- =====================================================================

BEGIN;

-- 1. provider.slug -----------------------------------------------------

ALTER TABLE provider ADD COLUMN IF NOT EXISTS slug VARCHAR(64);

UPDATE provider
SET slug = btrim(
        regexp_replace(
            regexp_replace(
                regexp_replace(
                    lower(translate(btrim(name),
                        'áéíóúüÁÉÍÓÚÜñÑ',
                        'aeiouuAEIOUUnN')),
                    '[^a-z0-9]+', '-', 'g'),
                '^-+', ''),
            '-+$', ''),
        '-')
WHERE slug IS NULL;

-- El nombre no era único, así que dos proveedores pueden normalizar al mismo
-- slug: el segundo y siguientes reciben un sufijo numérico estable por id.
WITH colisiones AS (
    SELECT id, slug,
           row_number() OVER (PARTITION BY slug ORDER BY id) AS posicion
    FROM provider
    WHERE slug IS NOT NULL
)
UPDATE provider p
SET slug = c.slug || '-' || c.posicion
FROM colisiones c
WHERE p.id = c.id
  AND c.posicion > 1;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM provider WHERE slug IS NULL OR slug = '') THEN
        RAISE EXCEPTION '047: quedaron proveedores sin slug';
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'provider_slug_format_check') THEN
        ALTER TABLE provider ADD CONSTRAINT provider_slug_format_check
            CHECK (slug ~ '^[a-z0-9-]{1,64}$');
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'provider_slug_unique') THEN
        ALTER TABLE provider ADD CONSTRAINT provider_slug_unique UNIQUE (slug);
    END IF;
END $$;

ALTER TABLE provider ALTER COLUMN slug SET NOT NULL;

-- 2. service.category_ref ---------------------------------------------

ALTER TABLE service
    ADD COLUMN IF NOT EXISTS category_ref INTEGER REFERENCES categories(id) ON DELETE RESTRICT;

CREATE INDEX IF NOT EXISTS idx_service_category_ref ON service(category_ref);

-- Servicios legados: se enlazan por la categoría antigua equivalente.
UPDATE service s
SET category_ref = c.id
FROM categories c
WHERE c.legacy_service_category_id = s.category_id
  AND c.item_type = 'service'
  AND s.category_ref IS NULL;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'service_id_category_ref_unique') THEN
        ALTER TABLE service ADD CONSTRAINT service_id_category_ref_unique UNIQUE (id, category_ref);
    END IF;
END $$;

-- 3. service_attribute_value ------------------------------------------

CREATE TABLE IF NOT EXISTS service_attribute_value (
    service_id     INTEGER      NOT NULL,
    category_id    INTEGER      NOT NULL,
    attribute_id   INTEGER      NOT NULL,
    -- Forma canónica de categories.ValidateAttributes ("350", "true", "Gris").
    value          VARCHAR(255) NOT NULL CHECK (btrim(value) <> ''),
    -- Solo integer/decimal: permite filtros por rango sin castear texto.
    value_numeric  NUMERIC,

    PRIMARY KEY (service_id, attribute_id),

    -- El valor pertenece al servicio y a la categoría actual del servicio...
    CONSTRAINT service_attribute_value_service_fk
        FOREIGN KEY (service_id, category_id)
        REFERENCES service(id, category_ref) ON DELETE CASCADE,
    -- ...y el atributo es de esa misma categoría.
    CONSTRAINT service_attribute_value_attribute_fk
        FOREIGN KEY (attribute_id, category_id)
        REFERENCES category_attributes(id, category_id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_service_attribute_value_attribute
    ON service_attribute_value(attribute_id);

-- 4. Identidad de la oferta de servicio -------------------------------
--
-- Algunos proveedores publican varios planes en una sola página (Entel), así
-- que la URL no basta para distinguirlos: el id del plan en el proveedor
-- (`externalId` del ítem) es la identidad cuando viene. Sin él se usa la URL.

ALTER TABLE service_offer ADD COLUMN IF NOT EXISTS external_id VARCHAR(255);

CREATE UNIQUE INDEX IF NOT EXISTS service_offer_provider_external_key
    ON service_offer (provider_id, external_id)
    WHERE external_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_service_offer_provider_url
    ON service_offer (provider_id, split_part(service_url, '?', 1));

COMMIT;
