BEGIN;

CREATE TEMP TABLE tal231_nb_category_matches ON COMMIT DROP AS
SELECT 'source'::text AS role, notebook.id AS category_id
FROM product_category notebook
JOIN product_category parent ON parent.id = notebook.parent_category_id
JOIN product_category grandparent ON grandparent.id = parent.parent_category_id
WHERE regexp_replace(translate(lower(btrim(notebook.name)), 'áéíóúüñ', 'aeiouun'),
                     '[[:space:]]+', ' ', 'g') = 'notebooks'
  AND regexp_replace(translate(lower(btrim(parent.name)), 'áéíóúüñ', 'aeiouun'),
                     '[[:space:]]+', ' ', 'g') = 'computacion'
  AND regexp_replace(translate(lower(btrim(grandparent.name)), 'áéíóúüñ', 'aeiouun'),
                     '[[:space:]]+', ' ', 'g') = 'tecnologia'
  AND grandparent.parent_category_id IS NULL
  AND notebook.active
  AND parent.active
  AND grandparent.active
UNION ALL
SELECT 'target'::text, computers.id
FROM product_category computers
JOIN product_category parent ON parent.id = computers.parent_category_id
WHERE regexp_replace(translate(lower(btrim(computers.name)), 'áéíóúüñ', 'aeiouun'),
                     '[[:space:]]+', ' ', 'g') = 'computadores'
  AND regexp_replace(translate(lower(btrim(parent.name)), 'áéíóúüñ', 'aeiouun'),
                     '[[:space:]]+', ' ', 'g') = 'computacion'
  AND parent.parent_category_id IS NULL
  AND computers.active
  AND parent.active;

CREATE TEMP TABLE tal231_nb_unified_matches ON COMMIT DROP AS
SELECT id
FROM categories
WHERE slug = 'notebooks'
  AND item_type = 'product'
  AND active;

CREATE TEMP TABLE tal231_nb_map (
    source_name text NOT NULL,
    target_name text NOT NULL,
    data_type text NOT NULL,
    source_unit text,
    target_unit text
) ON COMMIT DROP;

INSERT INTO tal231_nb_map
    (source_name, target_name, data_type, source_unit, target_unit)
VALUES
    ('Procesador',     'Procesador',     'string', NULL,        NULL),
    ('RAM',            'Memoria RAM',    'number', 'GB',       'GB'),
    ('Almacenamiento', 'Almacenamiento', 'number', 'GB',       'GB'),
    ('Pantalla',       'Pantalla',       'number', 'pulgadas', 'in');

CREATE TEMP TABLE tal231_nb_defs (
    source_specification_id integer NOT NULL,
    target_specification_id integer NOT NULL
) ON COMMIT DROP;

DO $$
DECLARE
    source_id integer;
    target_id integer;
    unified_id integer;
    source_matches bigint;
    target_matches bigint;
    unified_matches bigint;
    source_products bigint;
    source_values bigint;
    linked_category_id integer;
    source_spec_count bigint;
    target_spec_count bigint;
    source_spec_id integer;
    target_spec_id integer;
    source_definition_matches boolean;
    target_definition_matches boolean;
    mapping_row record;
BEGIN
    SELECT count(*), min(category_id)
      INTO source_matches, source_id
    FROM tal231_nb_category_matches
    WHERE role = 'source';

    IF source_matches > 1 THEN
        RAISE EXCEPTION 'Más de una categoría coincide con Notebooks y su jerarquía';
    ELSIF source_matches = 0 THEN
        RAISE NOTICE '020 no-op: no se encontró la categoría origen Notebooks';
    ELSE
        SELECT count(*), min(category_id)
          INTO target_matches, target_id
        FROM tal231_nb_category_matches
        WHERE role = 'target';

        IF target_matches > 1 THEN
            RAISE EXCEPTION 'Más de una categoría coincide con Computadores y su jerarquía';
        END IF;

        SELECT count(*), min(id)
          INTO unified_matches, unified_id
        FROM tal231_nb_unified_matches;

        IF unified_matches > 1 THEN
            RAISE EXCEPTION 'Más de una categoría activa coincide con categories.slug = notebooks';
        ELSIF unified_matches = 1 THEN
            SELECT legacy_product_category_id
              INTO linked_category_id
            FROM categories
            WHERE id = unified_id;

            IF linked_category_id IS DISTINCT FROM target_id THEN
                RAISE NOTICE '020 no-op: categories.notebooks no está vinculada a Computadores';
                unified_matches := 0;
            END IF;
        END IF;

        IF unified_matches = 0 THEN
            RAISE NOTICE '020 no-op: no se resolvió el vínculo categories.notebooks -> Computadores';
        ELSE
            SELECT count(*)
              INTO source_products
            FROM product
            WHERE category_id = source_id;

            SELECT count(*)
              INTO source_values
            FROM product_specification_value v
            JOIN product_category_specification s ON s.id = v.specification_id
            WHERE s.category_id = source_id;

            IF source_products = 0 AND source_values = 0 THEN
                RAISE NOTICE '020 no-op: no hay productos origen ni valores ligados a definiciones de Notebooks';
            ELSIF target_matches = 0 AND source_values > 0 THEN
                RAISE EXCEPTION 'Hay valores origen sin definición destino equivalente: no existe Computadores';
            ELSIF target_matches = 0 THEN
                RAISE NOTICE '020 no-op: no se encontró la categoría destino Computadores';
            ELSE
                FOR mapping_row IN
                    SELECT * FROM tal231_nb_map
                LOOP
                    source_definition_matches := FALSE;
                    target_definition_matches := FALSE;

                    SELECT count(*), min(s.id)
                      INTO source_spec_count, source_spec_id
                    FROM product_category_specification s
                    WHERE s.category_id = source_id
                      AND regexp_replace(translate(lower(btrim(s.name)), 'áéíóúüñ', 'aeiouun'),
                                         '[[:space:]]+', ' ', 'g')
                          = regexp_replace(translate(lower(btrim(mapping_row.source_name)), 'áéíóúüñ', 'aeiouun'),
                                           '[[:space:]]+', ' ', 'g');

                    SELECT count(*), min(s.id)
                      INTO target_spec_count, target_spec_id
                    FROM product_category_specification s
                    WHERE s.category_id = target_id
                      AND regexp_replace(translate(lower(btrim(s.name)), 'áéíóúüñ', 'aeiouun'),
                                         '[[:space:]]+', ' ', 'g')
                          = regexp_replace(translate(lower(btrim(mapping_row.target_name)), 'áéíóúüñ', 'aeiouun'),
                                           '[[:space:]]+', ' ', 'g');

                    IF source_spec_count > 1 THEN
                        RAISE EXCEPTION 'Definición origen ambigua para %', mapping_row.source_name;
                    END IF;
                    IF target_spec_count > 1 THEN
                        RAISE EXCEPTION 'Definición destino ambigua para %', mapping_row.target_name;
                    END IF;

                    IF source_spec_count = 1 THEN
                        SELECT s.data_type = mapping_row.data_type
                               AND lower(btrim(s.unit))
                                   IS NOT DISTINCT FROM lower(btrim(mapping_row.source_unit))
                          INTO source_definition_matches
                        FROM product_category_specification s
                        WHERE s.id = source_spec_id;
                    END IF;

                    IF target_spec_count = 1 THEN
                        SELECT s.data_type = mapping_row.data_type
                               AND lower(btrim(s.unit))
                                   IS NOT DISTINCT FROM lower(btrim(mapping_row.target_unit))
                          INTO target_definition_matches
                        FROM product_category_specification s
                        WHERE s.id = target_spec_id;
                    END IF;

                    IF source_definition_matches AND target_definition_matches THEN
                        INSERT INTO tal231_nb_defs
                            (source_specification_id, target_specification_id)
                        VALUES (source_spec_id, target_spec_id);
                    END IF;
                END LOOP;

                IF EXISTS (
                    SELECT 1
                    FROM product_specification_value v
                    JOIN product_category_specification source_spec
                      ON source_spec.id = v.specification_id
                    WHERE source_spec.category_id = source_id
                      AND NOT EXISTS (
                          SELECT 1
                          FROM tal231_nb_defs d
                          WHERE d.source_specification_id = source_spec.id
                      )
                ) THEN
                    RAISE EXCEPTION 'Hay un valor origen ligado a una definición sin equivalente en el mapeo';
                END IF;

                IF EXISTS (
                    SELECT 1
                    FROM product_specification_value source_value
                    JOIN tal231_nb_defs d
                      ON d.source_specification_id = source_value.specification_id
                    JOIN product_specification_value target_value
                      ON target_value.product_id = source_value.product_id
                     AND target_value.specification_id = d.target_specification_id
                    WHERE target_value.value IS DISTINCT FROM source_value.value
                ) THEN
                    RAISE EXCEPTION 'Existe un valor destino distinto del valor origen';
                END IF;

                INSERT INTO product_specification_value
                    (product_id, specification_id, value)
                SELECT source_value.product_id,
                       d.target_specification_id,
                       source_value.value
                FROM product_specification_value source_value
                JOIN tal231_nb_defs d
                  ON d.source_specification_id = source_value.specification_id
                JOIN product_category_specification source_spec
                  ON source_spec.id = source_value.specification_id
                 AND source_spec.category_id = source_id
                ON CONFLICT (product_id, specification_id) DO NOTHING;

                UPDATE product_comparison
                SET category_id = target_id,
                    updated_at = NOW()
                WHERE category_id = source_id;

                UPDATE product p
                SET category_id = target_id,
                    category_ref = unified_id,
                    updated_at = NOW()
                WHERE (
                        p.category_id = source_id
                        OR (
                            p.category_id = target_id
                            AND EXISTS (
                                SELECT 1
                                FROM product_specification_value v
                                JOIN product_category_specification s
                                  ON s.id = v.specification_id
                                WHERE v.product_id = p.id
                                  AND s.category_id = source_id
                            )
                        )
                      )
                  AND (p.category_id IS DISTINCT FROM target_id
                       OR p.category_ref IS DISTINCT FROM unified_id);
            END IF;
        END IF;
    END IF;
END;
$$;

COMMIT;
