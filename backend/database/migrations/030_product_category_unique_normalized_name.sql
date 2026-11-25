-- Unicidad por (padre, nombre normalizado sin tildes). El indice de 011 usa
-- lower(btrim(name)) y deja pasar nombres que solo difieren por tildes; este
-- lo cierra. Aditiva e idempotente; el indice de 011 no se toca.
CREATE UNIQUE INDEX IF NOT EXISTS product_category_parent_name_ascii_key
    ON product_category (
        (COALESCE(parent_category_id, -1)),
        (translate(lower(btrim(name)),
                   U&'\00e1\00e9\00ed\00f3\00fa\00fc\00f1',
                   'aeiouun'))
    );
