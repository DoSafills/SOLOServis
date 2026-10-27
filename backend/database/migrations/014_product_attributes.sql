-- =====================================================================
-- TAL-70: productos con categoría unificada (TAL-71) y valores de
-- atributos dinámicos validados por TAL-72.
--
-- Sin BEGIN/COMMIT propio (migrate.sh usa --single-transaction).
-- product.category_id (product_category) NO se modifica: lo siguen usando
-- Search, Comparison y el frontend. category_ref es la categoría nueva.
-- =====================================================================

ALTER TABLE product
    ADD COLUMN category_ref INTEGER REFERENCES categories(id) ON DELETE RESTRICT;

CREATE INDEX idx_product_category_ref ON product(category_ref);

-- Productos existentes: se enlazan por la categoría antigua equivalente.
UPDATE product p
SET category_ref = c.id
FROM categories c
WHERE c.legacy_product_category_id = p.category_id
  AND c.item_type = 'product';

-- Claves para las FK compuestas de product_attribute_value.
ALTER TABLE product
    ADD CONSTRAINT product_id_category_ref_unique UNIQUE (id, category_ref);
ALTER TABLE category_attributes
    ADD CONSTRAINT category_attributes_id_category_unique UNIQUE (id, category_id);

CREATE TABLE product_attribute_value (
    product_id     INTEGER      NOT NULL,
    category_id    INTEGER      NOT NULL,
    attribute_id   INTEGER      NOT NULL,
    -- Forma canónica de categories.ValidateAttributes ("350", "65.5", "true", "A+").
    value          VARCHAR(255) NOT NULL CHECK (btrim(value) <> ''),
    -- Solo integer/decimal: permite filtros por rango (TAL-75) sin castear texto.
    value_numeric  NUMERIC,

    PRIMARY KEY (product_id, attribute_id),

    -- El valor pertenece al producto y a la categoría actual del producto...
    CONSTRAINT product_attribute_value_product_fk
        FOREIGN KEY (product_id, category_id)
        REFERENCES product(id, category_ref) ON DELETE CASCADE,
    -- ...y el atributo es de esa misma categoría. Un atributo con valores
    -- guardados no se puede borrar.
    CONSTRAINT product_attribute_value_attribute_fk
        FOREIGN KEY (attribute_id, category_id)
        REFERENCES category_attributes(id, category_id) ON DELETE RESTRICT
);

-- Soporta la verificación de la FK al borrar atributos y los filtros de TAL-75.
CREATE INDEX idx_product_attribute_value_attribute ON product_attribute_value(attribute_id);
