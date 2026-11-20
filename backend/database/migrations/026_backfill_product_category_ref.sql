-- TAL-244: rellena product.category_ref en los productos que lo perdieron.
--
-- La búsqueda por slug (`category=celulares`) y los filtros por atributo
-- comparan con product.category_ref. Ese campo solo se llenó en la 014; las
-- migraciones 022 y 023 movieron productos de categoría (Telefonia ->
-- Celulares, Computacion -> Computación) y la 024 creó el slug `celulares`
-- sin rellenarlo. Resultado: `category=celulares` devolvía 0 productos y
-- `category=notebooks` 5 de 8.
--
-- Cada producto recibe la categoría nueva (`categories`) de la product_category
-- más cercana con slug, subiendo desde su category_id. Solo toca filas con
-- category_ref NULL: así no choca con la FK (id, category_ref) de
-- product_attribute_value y volver a correrla no cambia nada.
--
-- Un solo bloque DO para que se aplique completo o no se aplique (el runner
-- no envuelve el archivo en una transacción).
DO $$
DECLARE
    updated_rows INTEGER;
BEGIN
    WITH RECURSIVE ancestry AS (
        SELECT p.id AS product_id, pc.id AS category_id,
               pc.parent_category_id, 0 AS depth
        FROM product p
        JOIN product_category pc ON pc.id = p.category_id
        WHERE p.category_ref IS NULL
        UNION ALL
        SELECT a.product_id, parent.id, parent.parent_category_id, a.depth + 1
        FROM ancestry a
        JOIN product_category parent ON parent.id = a.parent_category_id
        WHERE a.depth < 20
    ),
    nearest AS (
        SELECT DISTINCT ON (a.product_id) a.product_id, c.id AS category_ref
        FROM ancestry a
        JOIN categories c
          ON c.legacy_product_category_id = a.category_id
         AND c.item_type = 'product'
         AND c.active
        ORDER BY a.product_id, a.depth
    )
    UPDATE product p
    SET category_ref = nearest.category_ref,
        updated_at = NOW()
    FROM nearest
    WHERE p.id = nearest.product_id
      AND p.category_ref IS NULL;

    GET DIAGNOSTICS updated_rows = ROW_COUNT;
    RAISE NOTICE '026: % producto(s) con category_ref rellenado', updated_rows;
END;
$$;
