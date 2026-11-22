-- TAL-189: texto de búsqueda del producto ya normalizado.
--
-- La Search API y los productos populares normalizaban en cada consulta el
-- nombre, modelo y SKU de cada producto con translate(lower(...)). Con 20.000
-- productos, una búsqueda por texto tardaba 116 ms solo en eso; leyendo esta
-- columna, 32 ms. La columna se recalcula sola al cambiar el producto.
--
-- La marca vive en otra tabla y una columna generada no puede leerla: las
-- consultas siguen normalizando brand.name, que son pocas filas.
--
-- La expresión reproduce la que usaban las consultas (mismo orden, mismos
-- separadores y la misma tabla de translate), así que los resultados no
-- cambian. Ninguna consulta usa SELECT * sobre product, así que la columna
-- nueva no altera los Scan existentes.
--
-- Idempotente (IF NOT EXISTS). Un solo comando: se aplica completo o no.
ALTER TABLE product
    ADD COLUMN IF NOT EXISTS search_text TEXT
    GENERATED ALWAYS AS (
        translate(
            lower(name || ' ' || COALESCE(model, '') || ' ' || COALESCE(sku, '')),
            'áàäâéèëêíìïîóòöôúùüûñç', 'aaaaeeeeiiiioooouuuunc')
    ) STORED;
