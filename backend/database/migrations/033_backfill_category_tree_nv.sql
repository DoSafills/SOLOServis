-- =====================================================================
-- 033: siembra el árbol NV1 > NV2 > NV3 y llena product.category_nv3_id.
--
-- Árbol sembrado (exactamente tres niveles):
--   Hogar > Electrodomésticos > Refrigeradores, Lavadoras, Cocina,
--       Climatización, Limpieza del hogar, Cuidado personal
--   Hogar > Muebles > Muebles y decoración
--   Tecnología > Computación > Notebooks, Computadores de escritorio,
--       Accesorios de computación
--   Tecnología > Pantallas > Televisores
--   Tecnología > Sonido > Audio
--   Tecnología > Telefonía > Celulares
--   Tecnología > Videojuegos y consolas > Gaming
--
-- QUEDAN FUERA (serían un cuarto nivel; no se crean como NV3 y sus filas
-- legacy no se tocan; pendientes de un atributo "tipo"):
--   Microondas, Hervidores, Cafeteras, Tostadoras y sandwicheras,
--   Licuadoras y batidoras, Freidoras de aire, Multicocinas y ollas,
--   Exprimidores, Aires acondicionados, Estufas, Calefactores y radiadores,
--   Ventiladores, Deshumidificadores, Humidificadores, Purificadores de aire,
--   Aspiradoras, Aspiradoras robot, Hidrolavadoras,
--   Planchas y centros de planchado, Secadores de pelo, Planchas y rizadores,
--   Afeitadoras y cortadoras, Depiladoras.
--
-- Backfill (función backfill_category_nv3, re-ejecutable y solo con NULL):
--   1. categorías legacy que ya son una NV3: Refrigeradores, Lavadoras,
--      Televisores, Audio, Celulares, Gaming, Notebooks, Computadores de
--      escritorio y Hogar y mueble (-> Muebles y decoración);
--   2. productos colgados de la fila legacy de Cocina, Climatización,
--      Limpieza del hogar o Cuidado personal -> NV3 de ese mismo padre;
--   3. productos en una hija legacy de esos cuatro NV2 (los 23 tipos de
--      nivel 4) -> NV3 del padre, resuelto por parent_category_id de la
--      fila legacy y nunca por el nombre del producto;
--   4. Computación, por palabras del nombre: accesorios primero, luego
--      notebook/laptop/macbook, y al final escritorio (imac, pc gamer,
--      desktop, escritorio); "Armado de equipo..." queda NULL;
--   5. cualquier otra categoría (p. ej. una NV2 sin NV3) queda NULL; de
--      Computación se avisa con id y nombre.
--
-- Idempotente: los INSERT usan WHERE NOT EXISTS; la función solo toca filas
-- con category_nv3_id NULL y no pisa valores existentes.
-- BEGIN/COMMIT propio.
-- Reversa: backend/database/rollbacks/revert_032_033_category_tree_nv.sql
-- =====================================================================
BEGIN;

CREATE OR REPLACE FUNCTION pg_temp.tal033_normalize_label(label_value TEXT)
RETURNS TEXT
LANGUAGE SQL
IMMUTABLE
AS $$
    SELECT translate(lower(btrim(label_value)),
                     U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                     'aeiouun')
$$;

-- NV1
INSERT INTO categories_nv1 (name, slug)
SELECT seed.name, seed.slug
FROM (VALUES
    ('Hogar', 'hogar'),
    ('Tecnología', 'tecnologia')
) AS seed(name, slug)
WHERE NOT EXISTS (
    SELECT 1 FROM categories_nv1 existing WHERE existing.slug = seed.slug
);

-- NV2, resuelto por el slug de su NV1.
INSERT INTO categories_nv2 (nv1_id, name, slug)
SELECT parent.id, seed.name, seed.slug
FROM (VALUES
    ('hogar', 'Electrodomésticos', 'electrodomesticos'),
    ('hogar', 'Muebles', 'muebles'),
    ('tecnologia', 'Computación', 'computacion'),
    ('tecnologia', 'Pantallas', 'pantallas'),
    ('tecnologia', 'Sonido', 'sonido'),
    ('tecnologia', 'Telefonía', 'telefonia'),
    ('tecnologia', 'Videojuegos y consolas', 'videojuegos-y-consolas')
) AS seed(nv1_slug, name, slug)
JOIN categories_nv1 parent ON parent.slug = seed.nv1_slug
WHERE NOT EXISTS (
    SELECT 1 FROM categories_nv2 existing
    WHERE existing.nv1_id = parent.id
      AND pg_temp.tal033_normalize_label(existing.name) =
          pg_temp.tal033_normalize_label(seed.name)
);

-- NV3, resuelto por los slugs de sus NV2 y NV1.
INSERT INTO categories_nv3 (nv2_id, name, slug)
SELECT parent.id, seed.name, seed.slug
FROM (VALUES
    ('hogar', 'electrodomesticos', 'Refrigeradores', 'refrigeradores'),
    ('hogar', 'electrodomesticos', 'Lavadoras', 'lavadoras'),
    ('hogar', 'electrodomesticos', 'Cocina', 'cocina'),
    ('hogar', 'electrodomesticos', 'Climatización', 'climatizacion'),
    ('hogar', 'electrodomesticos', 'Limpieza del hogar', 'limpieza-del-hogar'),
    ('hogar', 'electrodomesticos', 'Cuidado personal', 'cuidado-personal'),
    ('hogar', 'muebles', 'Muebles y decoración', 'muebles-y-decoracion'),
    ('tecnologia', 'computacion', 'Notebooks', 'notebooks'),
    ('tecnologia', 'computacion', 'Computadores de escritorio', 'computadores-de-escritorio'),
    ('tecnologia', 'computacion', 'Accesorios de computación', 'accesorios-de-computacion'),
    ('tecnologia', 'pantallas', 'Televisores', 'televisores'),
    ('tecnologia', 'sonido', 'Audio', 'audio'),
    ('tecnologia', 'telefonia', 'Celulares', 'celulares'),
    ('tecnologia', 'videojuegos-y-consolas', 'Gaming', 'gaming')
) AS seed(nv1_slug, nv2_slug, name, slug)
JOIN categories_nv2 parent ON parent.slug = seed.nv2_slug
JOIN categories_nv1 grandparent
  ON grandparent.id = parent.nv1_id
 AND grandparent.slug = seed.nv1_slug
WHERE NOT EXISTS (
    SELECT 1 FROM categories_nv3 existing
    WHERE existing.nv2_id = parent.id
      AND pg_temp.tal033_normalize_label(existing.name) =
          pg_temp.tal033_normalize_label(seed.name)
)
AND NOT EXISTS (
    SELECT 1 FROM categories_nv3 existing_slug
    WHERE existing_slug.slug = seed.slug
);

-- Backfill re-ejecutable: llena solo product.category_nv3_id NULL; nunca
-- pisa un valor ya asignado. Devuelve cuántas filas llenó.
CREATE OR REPLACE FUNCTION backfill_category_nv3()
RETURNS INTEGER
LANGUAGE plpgsql
AS $$
DECLARE
    total INTEGER := 0;
    filled INTEGER;
    pendientes_total INTEGER;
    pendientes TEXT;
BEGIN
    -- 1. Categorías legacy que ya corresponden a una NV3 del árbol.
    UPDATE product p
    SET category_nv3_id = target.id
    FROM product_category source, categories_nv3 target
    WHERE p.category_id = source.id
      AND p.category_nv3_id IS NULL
      AND target.slug = CASE translate(lower(btrim(source.name)),
                                       U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                                       'aeiouun')
            WHEN 'refrigeradores' THEN 'refrigeradores'
            WHEN 'lavadoras' THEN 'lavadoras'
            WHEN 'televisores' THEN 'televisores'
            WHEN 'audio' THEN 'audio'
            WHEN 'celulares' THEN 'celulares'
            WHEN 'gaming' THEN 'gaming'
            WHEN 'hogar y mueble' THEN 'muebles-y-decoracion'
            WHEN 'notebooks' THEN 'notebooks'
            WHEN 'computadores de escritorio' THEN 'computadores-de-escritorio'
        END;
    GET DIAGNOSTICS filled = ROW_COUNT;
    total := total + filled;

    -- 2. Productos colgados de la fila legacy del NV2 (Cocina,
    --    Climatización, Limpieza del hogar o Cuidado personal): van a la
    --    NV3 de ese mismo padre.
    UPDATE product p
    SET category_nv3_id = target.id
    FROM product_category source, categories_nv3 target
    WHERE p.category_id = source.id
      AND p.category_nv3_id IS NULL
      AND target.slug = CASE translate(lower(btrim(source.name)),
                                       U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                                       'aeiouun')
            WHEN 'cocina' THEN 'cocina'
            WHEN 'climatizacion' THEN 'climatizacion'
            WHEN 'limpieza del hogar' THEN 'limpieza-del-hogar'
            WHEN 'cuidado personal' THEN 'cuidado-personal'
        END;
    GET DIAGNOSTICS filled = ROW_COUNT;
    total := total + filled;

    -- 3. Hijos legacy de esos cuatro NV2 (los 23 tipos de nivel 4): el
    --    padre se resuelve por parent_category_id, no por el nombre del
    --    producto ni por el de la hoja.
    UPDATE product p
    SET category_nv3_id = target.id
    FROM product_category child
    JOIN product_category parent ON parent.id = child.parent_category_id,
    categories_nv3 target
    WHERE p.category_id = child.id
      AND p.category_nv3_id IS NULL
      AND target.slug = CASE translate(lower(btrim(parent.name)),
                                       U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                                       'aeiouun')
            WHEN 'cocina' THEN 'cocina'
            WHEN 'climatizacion' THEN 'climatizacion'
            WHEN 'limpieza del hogar' THEN 'limpieza-del-hogar'
            WHEN 'cuidado personal' THEN 'cuidado-personal'
        END;
    GET DIAGNOSTICS filled = ROW_COUNT;
    total := total + filled;

    -- 4. Computación: accesorios primero (igual que classifyComputacion).
    UPDATE product p
    SET category_nv3_id = target.id
    FROM product_category source, categories_nv3 target
    WHERE source.id = p.category_id
      AND target.slug = 'accesorios-de-computacion'
      AND p.category_nv3_id IS NULL
      AND translate(lower(btrim(source.name)),
                    U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                    'aeiouun') = 'computacion'
      AND EXISTS (
          SELECT 1
          FROM unnest(string_to_array(
                   regexp_replace(
                       translate(lower(btrim(p.name)),
                                 U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                                 'aeiouun'),
                       '[^a-z0-9]+', ' ', 'g'),
                   ' ')) AS word
          WHERE word IN ('mochila', 'mochilas', 'funda', 'fundas', 'maletin',
                         'maletines', 'soporte', 'soportes', 'base', 'bases',
                         'cargador', 'cargadores', 'adaptador', 'adaptadores',
                         'mouse', 'teclado', 'teclados', 'monitor', 'monitores',
                         'tablet', 'tablets', 'ipad', 'ipads')
      );
    GET DIAGNOSTICS filled = ROW_COUNT;
    total := total + filled;

    -- 5. Computación: notebooks, laptops y macbooks.
    UPDATE product p
    SET category_nv3_id = target.id
    FROM product_category source, categories_nv3 target
    WHERE source.id = p.category_id
      AND target.slug = 'notebooks'
      AND p.category_nv3_id IS NULL
      AND translate(lower(btrim(source.name)),
                    U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                    'aeiouun') = 'computacion'
      AND EXISTS (
          SELECT 1
          FROM unnest(string_to_array(
                   regexp_replace(
                       translate(lower(btrim(p.name)),
                                 U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                                 'aeiouun'),
                       '[^a-z0-9]+', ' ', 'g'),
                   ' ')) AS word
          WHERE word IN ('notebook', 'notebooks', 'laptop', 'laptops',
                         'macbook', 'macbooks')
      );
    GET DIAGNOSTICS filled = ROW_COUNT;
    total := total + filled;

    -- 6. Computación: escritorio (imac, pc gamer, desktop, escritorio).
    UPDATE product p
    SET category_nv3_id = target.id
    FROM product_category source, categories_nv3 target
    WHERE source.id = p.category_id
      AND target.slug = 'computadores-de-escritorio'
      AND p.category_nv3_id IS NULL
      AND translate(lower(btrim(source.name)),
                    U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                    'aeiouun') = 'computacion'
      AND (
          EXISTS (
              SELECT 1
              FROM unnest(string_to_array(
                       regexp_replace(
                           translate(lower(btrim(p.name)),
                                     U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                                     'aeiouun'),
                           '[^a-z0-9]+', ' ', 'g'),
                       ' ')) AS word
              WHERE word IN ('imac', 'imacs', 'desktop', 'desktops',
                             'escritorio', 'escritorios')
          )
          OR (' ' || regexp_replace(
                      translate(lower(btrim(p.name)),
                                U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                                'aeiouun'),
                      '[^a-z0-9]+', ' ', 'g') || ' ') LIKE '% pc gamer %'
      );
    GET DIAGNOSTICS filled = ROW_COUNT;
    total := total + filled;

    -- Reporte: productos de Computación sin NV3, con id y nombre.
    SELECT count(*),
           string_agg(format('%s: %s', p.id, p.name), '; ' ORDER BY p.id)
    INTO pendientes_total, pendientes
    FROM product p
    JOIN product_category source ON source.id = p.category_id
    WHERE translate(lower(btrim(source.name)),
                    U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                    'aeiouun') = 'computacion'
      AND p.category_nv3_id IS NULL;

    IF pendientes_total > 0 THEN
        RAISE NOTICE '033: % producto(s) de Computación quedan sin NV3 (id: nombre): %',
            pendientes_total, pendientes;
    ELSE
        RAISE NOTICE '033: no quedan productos de Computación sin NV3';
    END IF;

    RAISE NOTICE '033: backfill_category_nv3 llenó % producto(s)', total;
    RETURN total;
END;
$$;

SELECT backfill_category_nv3();

COMMIT;
