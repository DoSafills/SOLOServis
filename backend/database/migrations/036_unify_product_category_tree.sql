-- =====================================================================
-- 036: unifica el catálogo en product_category y retira el árbol NV.
--
-- Por qué: product_category ya es un árbol por parent_category_id. Las
-- tablas categories_nv1/nv2/nv3 duplicaban la misma jerarquía y obligaban a
-- mantener el puente legacy_product_category_id de 034/035. Esta migración
-- deja un solo árbol de exactamente tres niveles y elimina la duplicación.
--
-- Resultado (solo filas active = TRUE):
--   Hogar (1): Electrodomésticos (2), Muebles (2)
--     Electrodomésticos: Refrigeradores, Lavadoras, Cocina, Climatización,
--       Limpieza del hogar, Cuidado personal (3)
--     Muebles: Muebles y decoración (3)
--   Tecnología (1): Computación, Pantallas, Sonido, Telefonía,
--     Videojuegos y consolas (2)
--     Computación: Notebooks, Computadores de escritorio,
--       Accesorios de computación (3)
--     Pantallas: Televisores; Sonido: Audio; Telefonía: Celulares;
--     Videojuegos y consolas: Gaming (3)
--
-- Reglas:
--   - los niveles se resuelven por nombre normalizado y por el slug que se
--     asigna aquí; no se usan ids fijos;
--   - el nivel 4 (Microondas, Estufas, etc.) no existe en el catálogo: sus
--     productos suben al NV3 padre y las filas finas quedan inactivas;
--   - los productos que ya tenían una NV3 en product.category_nv3_id se
--     repuntan a la fila de product_category con ese slug, así se conserva
--     la clasificación que hizo 033;
--   - los productos de la fila legacy Computación sin NV3 van a
--     Accesorios de computación; los de Hogar y mueble, a Muebles y
--     decoración;
--   - product.category_id, product.category_ref, categories y los atributos
--     no cambian de contrato: solo cambia el id de la categoría;
--   - se aborta si queda un producto activo fuera de nivel 3 o una fila
--     activa sin slug/level válidos: nada de estados a medias.
--
-- Idempotente: columnas, índices, constraints, trigger y seeds usan
-- IF NOT EXISTS / WHERE NOT EXISTS; los UPDATE solo mueven lo que difiere.
-- Reversa: backend/database/rollbacks/revert_036_unify_product_category_tree.sql
-- BEGIN/COMMIT propio: el runner no envuelve cada archivo en una transacción.
-- =====================================================================
BEGIN;

CREATE OR REPLACE FUNCTION pg_temp.tal036_normalize_label(label_value TEXT)
RETURNS TEXT
LANGUAGE SQL
IMMUTABLE
AS $$
    SELECT translate(lower(btrim(label_value)),
                     U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                     'aeiouun')
$$;

ALTER TABLE product_category
    ADD COLUMN IF NOT EXISTS slug  VARCHAR(64),
    ADD COLUMN IF NOT EXISTS level SMALLINT;

-- Busca la categoría por (padre, nombre normalizado) o por nombre global,
-- la reparenta si hace falta y la crea solo si no existe. Devuelve su id.
CREATE OR REPLACE FUNCTION pg_temp.tal036_ensure_category(
    p_parent      INTEGER,
    p_name        TEXT,
    p_description TEXT
)
RETURNS INTEGER
LANGUAGE plpgsql
AS $$
DECLARE
    result_id INTEGER;
BEGIN
    SELECT id
    INTO result_id
    FROM product_category
    WHERE parent_category_id IS NOT DISTINCT FROM p_parent
      AND pg_temp.tal036_normalize_label(name) =
          pg_temp.tal036_normalize_label(p_name)
    ORDER BY active DESC, id
    LIMIT 1;

    IF result_id IS NULL THEN
        SELECT id
        INTO result_id
        FROM product_category
        WHERE pg_temp.tal036_normalize_label(name) =
              pg_temp.tal036_normalize_label(p_name)
        ORDER BY (parent_category_id IS NOT DISTINCT FROM p_parent) DESC,
                 active DESC,
                 id
        LIMIT 1;

        IF result_id IS NOT NULL AND p_parent IS NOT NULL THEN
            UPDATE product_category
            SET parent_category_id = p_parent,
                updated_at = NOW()
            WHERE id = result_id
              AND parent_category_id IS DISTINCT FROM p_parent;
        END IF;
    END IF;

    IF result_id IS NULL THEN
        INSERT INTO product_category (parent_category_id, name, description, active)
        VALUES (p_parent, p_name, p_description, TRUE)
        RETURNING id INTO result_id;
    ELSE
        -- Una fila histórica inactiva (por ejemplo, la raíz Hogar de 022)
        -- vuelve a ser el nodo canónico; el árbol activo la necesita.
        UPDATE product_category
        SET active = TRUE,
            updated_at = NOW()
        WHERE id = result_id
          AND NOT active;
    END IF;

    RETURN result_id;
END;
$$;

-- Árbol canónico: cada nodo queda en su padre y recibe slug + nivel.
DO $$
DECLARE
    hogar        INTEGER;
    tecnologia   INTEGER;
    electro      INTEGER;
    muebles      INTEGER;
    muebles_deco INTEGER;
    computacion  INTEGER;
    pantallas    INTEGER;
    sonido       INTEGER;
    telefonia    INTEGER;
    videojuegos  INTEGER;
    refrigeradores INTEGER;
    lavadoras    INTEGER;
    cocina       INTEGER;
    climatizacion INTEGER;
    limpieza     INTEGER;
    cuidado      INTEGER;
    notebooks    INTEGER;
    escritorio   INTEGER;
    accesorios   INTEGER;
    televisores  INTEGER;
    audio        INTEGER;
    celulares    INTEGER;
    gaming       INTEGER;
BEGIN
    hogar := pg_temp.tal036_ensure_category(
        NULL, 'Hogar', 'Productos y servicios para el hogar');
    tecnologia := pg_temp.tal036_ensure_category(
        NULL, 'Tecnología', 'Productos de tecnología y electrónica');

    electro := pg_temp.tal036_ensure_category(
        hogar, 'Electrodomésticos', 'Línea blanca y pequeños electrodomésticos');
    muebles := pg_temp.tal036_ensure_category(
        hogar, 'Muebles', 'Muebles, decoración y textiles del hogar');

    computacion := pg_temp.tal036_ensure_category(
        tecnologia, 'Computación', 'Computadores y accesorios');
    pantallas := pg_temp.tal036_ensure_category(
        tecnologia, 'Pantallas', 'Televisores y pantallas');
    sonido := pg_temp.tal036_ensure_category(
        tecnologia, 'Sonido', 'Audio, audífonos y parlantes');
    telefonia := pg_temp.tal036_ensure_category(
        tecnologia, 'Telefonía', 'Celulares y telefonía');
    videojuegos := pg_temp.tal036_ensure_category(
        tecnologia, 'Videojuegos y consolas', 'Consolas, juegos y accesorios');

    refrigeradores := pg_temp.tal036_ensure_category(
        electro, 'Refrigeradores', 'Refrigeradores y equipos de frío');
    lavadoras := pg_temp.tal036_ensure_category(
        electro, 'Lavadoras', 'Lavadoras y secadoras');
    cocina := pg_temp.tal036_ensure_category(
        electro, 'Cocina', 'Pequeños electrodomésticos de cocina');
    climatizacion := pg_temp.tal036_ensure_category(
        electro, 'Climatización', 'Estufas, calefactores, ventiladores y aires acondicionados');
    limpieza := pg_temp.tal036_ensure_category(
        electro, 'Limpieza del hogar', 'Aspiradoras, robots, hidrolavadoras y planchas');
    cuidado := pg_temp.tal036_ensure_category(
        electro, 'Cuidado personal', 'Secadores, planchas de pelo, afeitadoras y depiladoras');

    muebles_deco := pg_temp.tal036_ensure_category(
        muebles, 'Muebles y decoración', 'Muebles, decoración y productos para el hogar');

    notebooks := pg_temp.tal036_ensure_category(
        computacion, 'Notebooks', 'Notebooks y laptops');
    escritorio := pg_temp.tal036_ensure_category(
        computacion, 'Computadores de escritorio', 'Computadores de escritorio y all-in-one');
    accesorios := pg_temp.tal036_ensure_category(
        computacion, 'Accesorios de computación', 'Accesorios y periféricos de computación');

    televisores := pg_temp.tal036_ensure_category(
        pantallas, 'Televisores', 'Televisores y smart TVs');
    audio := pg_temp.tal036_ensure_category(
        sonido, 'Audio', 'Audífonos y parlantes');
    celulares := pg_temp.tal036_ensure_category(
        telefonia, 'Celulares', 'Teléfonos móviles y accesorios');
    gaming := pg_temp.tal036_ensure_category(
        videojuegos, 'Gaming', 'Consolas, juegos y accesorios gamer');

    CREATE TEMP TABLE tal036_nodes (
        node_id INTEGER PRIMARY KEY,
        slug    TEXT NOT NULL,
        lvl     SMALLINT NOT NULL
    ) ON COMMIT DROP;

    INSERT INTO tal036_nodes (node_id, slug, lvl) VALUES
        (hogar, 'hogar', 1),
        (tecnologia, 'tecnologia', 1),
        (electro, 'electrodomesticos', 2),
        (muebles, 'muebles', 2),
        (computacion, 'computacion', 2),
        (pantallas, 'pantallas', 2),
        (sonido, 'sonido', 2),
        (telefonia, 'telefonia', 2),
        (videojuegos, 'videojuegos-y-consolas', 2),
        (refrigeradores, 'refrigeradores', 3),
        (lavadoras, 'lavadoras', 3),
        (cocina, 'cocina', 3),
        (climatizacion, 'climatizacion', 3),
        (limpieza, 'limpieza-del-hogar', 3),
        (cuidado, 'cuidado-personal', 3),
        (muebles_deco, 'muebles-y-decoracion', 3),
        (notebooks, 'notebooks', 3),
        (escritorio, 'computadores-de-escritorio', 3),
        (accesorios, 'accesorios-de-computacion', 3),
        (televisores, 'televisores', 3),
        (audio, 'audio', 3),
        (celulares, 'celulares', 3),
        (gaming, 'gaming', 3);

    UPDATE product_category pc
    SET slug = nodes.slug,
        level = nodes.lvl,
        updated_at = NOW()
    FROM tal036_nodes nodes
    WHERE pc.id = nodes.node_id
      AND (pc.slug IS DISTINCT FROM nodes.slug OR pc.level IS DISTINCT FROM nodes.lvl);
END;
$$;

-- Mapa de cada fila hacia su NV3 (nivel 3) más cercano, subiendo por el
-- padre. Sirve para absorber el nivel 4 y para los productos que quedaron
-- sin category_nv3_id.
CREATE TEMP TABLE tal036_category_map ON COMMIT DROP AS
WITH RECURSIVE up (node_id, ancestor_id, parent_id, lvl, depth) AS (
    SELECT id, id, parent_category_id, level, 0
    FROM product_category
    UNION ALL
    SELECT up.node_id, parent.id, parent.parent_category_id, parent.level, up.depth + 1
    FROM up
    JOIN product_category parent ON parent.id = up.parent_id
    WHERE up.lvl IS DISTINCT FROM 3
)
SELECT DISTINCT ON (node_id)
       node_id,
       ancestor_id
FROM up
WHERE lvl = 3
ORDER BY node_id, depth;

ALTER TABLE tal036_category_map ADD PRIMARY KEY (node_id);

-- Computación es NV2: sus productos sin NV3 no tienen un nivel 3 obvio y
-- pasan a Accesorios de computación, el destino que dejó 035.
INSERT INTO tal036_category_map (node_id, ancestor_id)
SELECT pc.id, nodes.node_id
FROM product_category pc
JOIN tal036_nodes nodes ON nodes.slug = 'accesorios-de-computacion'
WHERE pg_temp.tal036_normalize_label(pc.name) = 'computacion'
  AND NOT EXISTS (SELECT 1 FROM tal036_category_map m WHERE m.node_id = pc.id);

-- Hogar y mueble era raíz legacy: sus productos pasan a Muebles y decoración.
INSERT INTO tal036_category_map (node_id, ancestor_id)
SELECT pc.id, nodes.node_id
FROM product_category pc
JOIN tal036_nodes nodes ON nodes.slug = 'muebles-y-decoracion'
WHERE pg_temp.tal036_normalize_label(pc.name) = 'hogar y mueble'
  AND NOT EXISTS (SELECT 1 FROM tal036_category_map m WHERE m.node_id = pc.id);

-- Categorías legacy enlazadas por categories: si están en nivel 3, son su
-- propio destino (cubre filas que no llegan al árbol canónico).
INSERT INTO tal036_category_map (node_id, ancestor_id)
SELECT pc.id, pc.id
FROM product_category pc
JOIN categories c ON c.legacy_product_category_id = pc.id
WHERE c.item_type = 'product'
  AND pc.level = 3
  AND NOT EXISTS (SELECT 1 FROM tal036_category_map m WHERE m.node_id = pc.id);

-- Repunte de productos: primero la NV3 que ya tenían (033), después el
-- nivel 3 más cercano por el padre. En una segunda pasada el modelo NV ya
-- no existe y el bloque se omite: solo queda el mapa por ancestros.
DO $tal036$
BEGIN
    IF to_regclass('categories_nv3') IS NULL OR NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'product'
          AND column_name = 'category_nv3_id'
    ) THEN
        RETURN;
    END IF;

    EXECUTE $repunte$
        WITH product_target AS (
            SELECT p.id,
                   COALESCE(nv3_target.id, ancestry.ancestor_id) AS target_id
            FROM product p
            LEFT JOIN categories_nv3 nv3
                   ON nv3.id = p.category_nv3_id
            LEFT JOIN product_category nv3_target
                   ON nv3_target.slug = nv3.slug
                  AND nv3_target.level = 3
            LEFT JOIN tal036_category_map ancestry
                   ON ancestry.node_id = p.category_id
        )
        UPDATE product p
        SET category_id = target.target_id,
            updated_at = NOW()
        FROM product_target target
        WHERE p.id = target.id
          AND target.target_id IS NOT NULL
          AND p.category_id IS DISTINCT FROM target.target_id
    $repunte$;
END;
$tal036$;

-- categories apunta al nodo NV3 canónico de su slug.
UPDATE categories c
SET legacy_product_category_id = pc.id,
    updated_at = NOW()
FROM product_category pc
WHERE c.item_type = 'product'
  AND c.slug = pc.slug
  AND pc.active
  AND pc.level = 3
  AND c.legacy_product_category_id IS DISTINCT FROM pc.id;

-- El slug computacion no tiene nodo de nivel 3 propio: 035 puenteaba esa
-- escritura a Accesorios de computación y aquí se conserva ese destino.
UPDATE categories c
SET legacy_product_category_id = nodes.node_id,
    updated_at = NOW()
FROM tal036_nodes nodes
WHERE nodes.slug = 'accesorios-de-computacion'
  AND c.item_type = 'product'
  AND c.slug = 'computacion'
  AND c.legacy_product_category_id IS DISTINCT FROM nodes.node_id;

-- El nivel 4 y los restos de árboles anteriores quedan inactivos; sus
-- productos ya fueron repuntados.
UPDATE product_category pc
SET active = FALSE,
    updated_at = NOW()
WHERE pc.active
  AND NOT EXISTS (
      SELECT 1 FROM tal036_nodes nodes WHERE nodes.node_id = pc.id
  );

-- Invariantes del árbol: slug único, nivel 1-3 y cadena de padres coherente.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_indexes
        WHERE schemaname = current_schema()
          AND indexname = 'uq_product_category_slug'
    ) THEN
        CREATE UNIQUE INDEX uq_product_category_slug
            ON product_category (slug)
            WHERE slug IS NOT NULL;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'product_category_slug_format'
          AND conrelid = 'product_category'::regclass
    ) THEN
        ALTER TABLE product_category
            ADD CONSTRAINT product_category_slug_format
            CHECK (slug IS NULL OR slug ~ '^[a-z0-9-]{1,64}$') NOT VALID;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'product_category_level_range'
          AND conrelid = 'product_category'::regclass
    ) THEN
        ALTER TABLE product_category
            ADD CONSTRAINT product_category_level_range
            CHECK (level IS NULL OR level BETWEEN 1 AND 3) NOT VALID;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'product_category_level_parent'
          AND conrelid = 'product_category'::regclass
    ) THEN
        ALTER TABLE product_category
            ADD CONSTRAINT product_category_level_parent
            CHECK (
                level IS NULL
                OR (level = 1 AND parent_category_id IS NULL)
                OR (level > 1 AND parent_category_id IS NOT NULL)
            ) NOT VALID;
    END IF;
END;
$$;

-- Mantiene la coherencia en altas y ediciones: una fila activa siempre
-- tiene slug, nivel 1-3 y el padre del nivel inmediatamente anterior.
CREATE OR REPLACE FUNCTION product_category_level_guard()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    parent_level  SMALLINT;
    parent_active BOOLEAN;
BEGIN
    IF NOT NEW.active THEN
        RETURN NEW;
    END IF;

    IF NEW.slug IS NULL OR NEW.level IS NULL THEN
        RAISE EXCEPTION 'product_category %: una categoría activa necesita slug y level', NEW.id;
    END IF;

    IF NEW.level = 1 THEN
        IF NEW.parent_category_id IS NOT NULL THEN
            RAISE EXCEPTION 'product_category %: el nivel 1 no tiene padre', NEW.id;
        END IF;
        RETURN NEW;
    END IF;

    IF NEW.parent_category_id IS NULL THEN
        RAISE EXCEPTION 'product_category %: el nivel % necesita un padre', NEW.id, NEW.level;
    END IF;

    SELECT level, active
    INTO parent_level, parent_active
    FROM product_category
    WHERE id = NEW.parent_category_id;

    IF parent_level IS NULL
       OR NOT COALESCE(parent_active, FALSE)
       OR parent_level <> NEW.level - 1 THEN
        RAISE EXCEPTION 'product_category %: el padre % debe existir, estar activo y ser de nivel %',
            NEW.id, NEW.parent_category_id, NEW.level - 1;
    END IF;

    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS product_category_level_guard ON product_category;
CREATE TRIGGER product_category_level_guard
    BEFORE INSERT OR UPDATE OF slug, level, parent_category_id, active
    ON product_category
    FOR EACH ROW
    EXECUTE FUNCTION product_category_level_guard();

-- Verificación final: no se sigue si algo quedó fuera del árbol de 3 niveles.
DO $$
DECLARE
    bad_categories TEXT;
    bad_products   TEXT;
    total          INTEGER;
BEGIN
    SELECT count(*),
           string_agg(format('%s (%s, slug=%s, level=%s)', id, name, COALESCE(slug, 'NULL'), COALESCE(level::TEXT, 'NULL')), '; ' ORDER BY id)
    INTO total, bad_categories
    FROM product_category
    WHERE active
      AND (slug IS NULL OR level IS NULL OR level NOT BETWEEN 1 AND 3);

    IF total > 0 THEN
        RAISE EXCEPTION '036: % categoría(s) activas fuera del árbol de 3 niveles: %', total, bad_categories;
    END IF;

    SELECT count(*),
           string_agg(format('%s (%s)', p.id, p.name), '; ' ORDER BY p.id)
    INTO total, bad_products
    FROM product p
    JOIN product_category pc ON pc.id = p.category_id
    WHERE p.active
      AND (NOT pc.active OR pc.level <> 3);

    IF total > 0 THEN
        RAISE EXCEPTION '036: % producto(s) activos apuntan fuera del nivel 3: %', total, bad_products;
    END IF;

    SELECT count(*)
    INTO total
    FROM product_category pc
    LEFT JOIN product_category parent ON parent.id = pc.parent_category_id
    WHERE pc.active
      AND (
          (pc.level = 1 AND pc.parent_category_id IS NOT NULL)
          OR (pc.level > 1 AND (parent.id IS NULL OR NOT parent.active OR parent.level <> pc.level - 1))
      );

    IF total > 0 THEN
        RAISE EXCEPTION '036: % categoría(s) activas tienen una cadena de padres incoherente', total;
    END IF;

    -- Toda categoría de escritura (categories) debe caer en un nivel 3 activo.
    SELECT count(*),
           string_agg(format('%s (legacy=%s)', c.slug, COALESCE(c.legacy_product_category_id::TEXT, 'NULL')), '; ' ORDER BY c.slug)
    INTO total, bad_categories
    FROM categories c
    LEFT JOIN product_category pc ON pc.id = c.legacy_product_category_id
    WHERE c.item_type = 'product'
      AND (pc.id IS NULL OR NOT pc.active OR pc.level <> 3);

    IF total > 0 THEN
        RAISE EXCEPTION '036: % categoría(s) de escritura sin nodo de nivel 3: %', total, bad_categories;
    END IF;

    RAISE NOTICE '036: árbol unificado en product_category (2 de nivel 1, 7 de nivel 2, 14 de nivel 3)';
END;
$$;

-- Se retira el modelo paralelo: la columna puente y las tres tablas NV.
ALTER TABLE product DROP COLUMN IF EXISTS category_nv3_id;

DROP FUNCTION IF EXISTS backfill_category_nv3();

DROP TABLE IF EXISTS categories_nv3;
DROP TABLE IF EXISTS categories_nv2;
DROP TABLE IF EXISTS categories_nv1;

COMMIT;
