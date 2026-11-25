-- =====================================================================
-- 028: reorganiza el árbol de product_category y agrega las subcategorías
-- de Electrodomésticos.
--
-- Árbol resultante (solo filas active = TRUE):
--   Electrodomésticos: Refrigeradores, Lavadoras, Cocina (8 hijas),
--     Climatización (7), Limpieza del hogar (4), Cuidado personal (4).
--   Tecnología: Televisores, Audio, Computación (Notebooks,
--     Computadores de escritorio), Celulares, Gaming.
--   Hogar y mueble (raíz, sin cambios).
--
-- Esta migración trabaja sobre product_category, que es la tabla con
-- parent_category_id y la que usan Product API y el frontend
-- (/product-categories). La tabla categories es plana (slug, item_type)
-- y no se toca.
--
-- Reglas:
--   - nunca renombra ni borra filas existentes;
--   - resuelve todo por nombre normalizado, sin ids fijos;
--   - los INSERT usan WHERE NOT EXISTS por (parent_category_id,
--     lower(btrim(name))), igual que la unicidad de 011;
--   - aborta si ya existen duplicados por (padre, nombre) o si aparecen
--     tras los cambios, antes de crear el índice único;
--   - idempotente: correrla de nuevo no cambia nada ni falla.
--
-- BEGIN/COMMIT propio: migrate.sh no envuelve cada archivo en una
-- transacción.
-- =====================================================================
BEGIN;

-- minúsculas, sin tildes ni espacios sobrantes, para comparar nombres.
CREATE OR REPLACE FUNCTION pg_temp.tal028_normalize_label(label_value TEXT)
RETURNS TEXT
LANGUAGE SQL
IMMUTABLE
AS $$
    SELECT translate(lower(btrim(label_value)), 'áéíóúüñ', 'aeiouun')
$$;

-- Aborta si hay filas repetidas por (padre, nombre). Se usa antes de
-- modificar y antes de crear el índice único.
CREATE OR REPLACE FUNCTION pg_temp.tal028_assert_no_duplicates()
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    duplicate_list TEXT;
BEGIN
    SELECT string_agg(
               format('(padre=%s, nombre=%s, filas=%s)',
                      COALESCE(d.parent_category_id::TEXT, 'NULL'),
                      d.normalized_name,
                      d.row_count),
               '; '
               ORDER BY COALESCE(d.parent_category_id, 0), d.normalized_name)
    INTO duplicate_list
    FROM (
        SELECT parent_category_id,
               lower(btrim(name)) AS normalized_name,
               count(*) AS row_count
        FROM product_category
        GROUP BY parent_category_id, lower(btrim(name))
        HAVING count(*) > 1
    ) d;

    IF duplicate_list IS NOT NULL THEN
        RAISE EXCEPTION '028: hay categorías duplicadas por (padre, nombre); resuélvelas antes de aplicar: %',
            duplicate_list;
    END IF;
END;
$$;

SELECT pg_temp.tal028_assert_no_duplicates();

-- 1. Reparenta Computación, Celulares y Gaming bajo Tecnología, solo si
--    hoy son raíces activas. No mueve nada si Tecnología no es raíz activa
--    o si ya existe una hermana con el mismo nombre.
DO $$
DECLARE
    technology_id INTEGER;
    moved_count INTEGER;
BEGIN
    SELECT id INTO technology_id
    FROM product_category
    WHERE active
      AND parent_category_id IS NULL
      AND pg_temp.tal028_normalize_label(name) = 'tecnologia'
    ORDER BY id
    LIMIT 1;

    IF technology_id IS NULL THEN
        RAISE NOTICE '028: no existe Tecnología como raíz activa; se omite el reparenteo';
        RETURN;
    END IF;

    UPDATE product_category child
    SET parent_category_id = technology_id,
        updated_at = NOW()
    WHERE child.active
      AND child.parent_category_id IS NULL
      AND child.id <> technology_id
      AND pg_temp.tal028_normalize_label(child.name) IN ('computacion', 'celulares', 'gaming')
      AND NOT EXISTS (
          SELECT 1
          FROM product_category sibling
          WHERE sibling.parent_category_id = technology_id
            AND sibling.id <> child.id
            AND lower(btrim(sibling.name)) = lower(btrim(child.name))
      );

    GET DIAGNOSTICS moved_count = ROW_COUNT;
    RAISE NOTICE '028: % categoría(s) reubicadas bajo Tecnología (%)', moved_count, technology_id;
END;
$$;

-- 2. Repone las hijas de Computación que se perdieron al consolidarla.
DO $$
DECLARE
    computing_id INTEGER;
BEGIN
    SELECT id INTO computing_id
    FROM product_category
    WHERE active
      AND pg_temp.tal028_normalize_label(name) = 'computacion'
    ORDER BY (parent_category_id IS NULL), id
    LIMIT 1;

    IF computing_id IS NULL THEN
        RAISE NOTICE '028: no existe Computación activa; se omiten sus hijas';
        RETURN;
    END IF;

    INSERT INTO product_category (parent_category_id, name, description, active)
    SELECT computing_id, seed.name, seed.description, TRUE
    FROM (VALUES
        ('Notebooks', 'Notebooks y laptops'),
        ('Computadores de escritorio', 'Computadores de escritorio y all-in-one')
    ) AS seed(name, description)
    WHERE NOT EXISTS (
        SELECT 1
        FROM product_category existing
        WHERE existing.parent_category_id = computing_id
          AND lower(btrim(existing.name)) = lower(btrim(seed.name))
    );
END;
$$;

-- 3. Nivel 2 bajo Electrodomésticos y 4. nivel 3 bajo cada una de esas
--    categorías. Los padres se resuelven por nombre dentro de la raíz.
DO $$
DECLARE
    appliance_id INTEGER;
BEGIN
    SELECT id INTO appliance_id
    FROM product_category
    WHERE active
      AND parent_category_id IS NULL
      AND pg_temp.tal028_normalize_label(name) = 'electrodomesticos'
    ORDER BY id
    LIMIT 1;

    IF appliance_id IS NULL THEN
        RAISE NOTICE '028: no existe Electrodomésticos como raíz activa; se omiten sus subcategorías';
        RETURN;
    END IF;

    -- Nivel 2
    INSERT INTO product_category (parent_category_id, name, description, active)
    SELECT appliance_id, seed.name, seed.description, TRUE
    FROM (VALUES
        ('Cocina', 'Pequeños electrodomésticos de cocina'),
        ('Climatización', 'Estufas, calefactores, ventiladores y aires acondicionados'),
        ('Limpieza del hogar', 'Aspiradoras, robots, hidrolavadoras y planchas'),
        ('Cuidado personal', 'Secadores, planchas de pelo, afeitadoras y depiladoras')
    ) AS seed(name, description)
    WHERE NOT EXISTS (
        SELECT 1
        FROM product_category existing
        WHERE existing.parent_category_id = appliance_id
          AND lower(btrim(existing.name)) = lower(btrim(seed.name))
    );

    -- Nivel 3: description = name.
    INSERT INTO product_category (parent_category_id, name, description, active)
    SELECT parent.id, seed.name, seed.name, TRUE
    FROM (VALUES
        ('Cocina', 'Microondas'),
        ('Cocina', 'Hervidores'),
        ('Cocina', 'Cafeteras'),
        ('Cocina', 'Tostadoras y sandwicheras'),
        ('Cocina', 'Licuadoras y batidoras'),
        ('Cocina', 'Freidoras de aire'),
        ('Cocina', 'Multicocinas y ollas'),
        ('Cocina', 'Exprimidores'),
        ('Climatización', 'Aires acondicionados'),
        ('Climatización', 'Estufas'),
        ('Climatización', 'Calefactores y radiadores'),
        ('Climatización', 'Ventiladores'),
        ('Climatización', 'Deshumidificadores'),
        ('Climatización', 'Humidificadores'),
        ('Climatización', 'Purificadores de aire'),
        ('Limpieza del hogar', 'Aspiradoras'),
        ('Limpieza del hogar', 'Aspiradoras robot'),
        ('Limpieza del hogar', 'Hidrolavadoras'),
        ('Limpieza del hogar', 'Planchas y centros de planchado'),
        ('Cuidado personal', 'Secadores de pelo'),
        ('Cuidado personal', 'Planchas y rizadores'),
        ('Cuidado personal', 'Afeitadoras y cortadoras'),
        ('Cuidado personal', 'Depiladoras')
    ) AS seed(parent_name, name)
    JOIN LATERAL (
        SELECT parent_row.id
        FROM product_category parent_row
        WHERE parent_row.parent_category_id = appliance_id
          AND pg_temp.tal028_normalize_label(parent_row.name) =
              pg_temp.tal028_normalize_label(seed.parent_name)
        ORDER BY parent_row.active DESC, parent_row.id
        LIMIT 1
    ) AS parent ON TRUE
    WHERE NOT EXISTS (
        SELECT 1
        FROM product_category existing
        WHERE existing.parent_category_id = parent.id
          AND lower(btrim(existing.name)) = lower(btrim(seed.name))
    );
END;
$$;

-- 5. Unicidad (padre, nombre) a futuro. 011 ya creó ese índice como
--    product_category_parent_name_normalized_key; solo se crea uno nuevo si
--    en la base no existe ninguno de los dos.
SELECT pg_temp.tal028_assert_no_duplicates();

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_indexes
        WHERE schemaname = current_schema()
          AND indexname IN ('product_category_parent_name_normalized_key',
                            'uq_categories_parent_name')
    ) THEN
        CREATE UNIQUE INDEX uq_categories_parent_name
            ON product_category ((COALESCE(parent_category_id, -1)), (lower(btrim(name))));
        RAISE NOTICE '028: índice único uq_categories_parent_name creado';
    ELSE
        RAISE NOTICE '028: ya existe un índice único por (padre, nombre); no se crea otro';
    END IF;
END;
$$;

COMMIT;
