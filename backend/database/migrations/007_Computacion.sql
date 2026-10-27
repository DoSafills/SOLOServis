BEGIN;


INSERT INTO brand (name, logo_url, website_url)
SELECT 'HP', 'https://placehold.co/128x128?text=HP', 'https://www.hp.com'
WHERE NOT EXISTS (
    SELECT 1 FROM brand WHERE LOWER(name) = LOWER('HP')
);

INSERT INTO brand (name, logo_url, website_url)
SELECT 'ASUS', 'https://placehold.co/128x128?text=ASUS', 'https://www.asus.com'
WHERE NOT EXISTS (
    SELECT 1 FROM brand WHERE LOWER(name) = LOWER('ASUS')
);

INSERT INTO brand (name, logo_url, website_url)
SELECT 'Lenovo', 'https://placehold.co/128x128?text=Lenovo', 'https://www.lenovo.com'
WHERE NOT EXISTS (
    SELECT 1 FROM brand WHERE LOWER(name) = LOWER('Lenovo')
);

INSERT INTO product_category (name, description)
SELECT 'Computacion', 'Notebooks y computadores'
WHERE NOT EXISTS (
    SELECT 1 FROM product_category WHERE LOWER(name) = LOWER('Computacion')
);

UPDATE product_category subcategory
SET parent_category_id = parent.id
FROM product_category parent
WHERE LOWER(parent.name) = LOWER('Computacion')
  AND LOWER(subcategory.name) = LOWER('Computadores')
  AND subcategory.parent_category_id IS DISTINCT FROM parent.id;

INSERT INTO product_category (parent_category_id, name, description)
SELECT parent.id, 'Computadores', 'Computadores de escritorio y notebooks'
FROM product_category parent
WHERE LOWER(parent.name) = LOWER('Computacion')
  AND NOT EXISTS (
      SELECT 1
      FROM product_category subcategory
      WHERE LOWER(subcategory.name) = LOWER('Computadores')
        AND subcategory.parent_category_id = parent.id
  );

INSERT INTO product_category_specification
    (category_id, name, data_type, unit, required, comparable, display_order)
SELECT category.id, specification.name, specification.data_type,
       specification.unit, FALSE, TRUE, specification.display_order
FROM product_category category
CROSS JOIN (VALUES
    ('Almacenamiento', 'number', 'GB', 1),
    ('Memoria RAM', 'number', 'GB', 2),
    ('Pantalla', 'number', 'in', 3),
    ('Procesador', 'string', NULL, 4),
    ('Tarjeta grafica', 'string', NULL, 5),
    ('Sistema operativo', 'string', NULL, 6),
    ('Frecuencia de pantalla', 'number', 'Hz', 7)
) AS specification(name, data_type, unit, display_order)
WHERE LOWER(category.name) = LOWER('Computadores')
  AND NOT EXISTS (
      SELECT 1
      FROM product_category_specification existing
      WHERE existing.category_id = category.id
        AND LOWER(existing.name) = LOWER(specification.name)
  );

INSERT INTO store
    (name, website_url, logo_url, rating, reputation, shipping_information, general_conditions)
SELECT store.name, store.website_url, store.logo_url, store.rating, store.reputation,
       store.shipping_information, store.general_conditions
FROM (VALUES
    ('Paris', 'https://www.paris.cl', 'https://placehold.co/128x128?text=Paris',
     4.00, 'Buena', 'Despacho segun direccion y disponibilidad', 'Garantia legal y del fabricante'),
    ('Falabella', 'https://www.falabella.com', 'https://placehold.co/128x128?text=Falabella',
     4.20, 'Buena', 'Despacho segun direccion y disponibilidad', 'Garantia legal y del fabricante'),
    ('Lider', 'https://www.lider.cl', 'https://placehold.co/128x128?text=Lider',
     4.00, 'Buena', 'Despacho segun direccion y disponibilidad', 'Garantia legal y del fabricante')
) AS store(name, website_url, logo_url, rating, reputation, shipping_information, general_conditions)
WHERE NOT EXISTS (
    SELECT 1 FROM store existing WHERE LOWER(existing.name) = LOWER(store.name)
);

INSERT INTO product (category_id, brand_id, name, model, sku, description)
SELECT category.id, brand.id,
       'Notebook Gamer HP Victus 15-fb3004la Ryzen 9',
       '15-fb3004la',
       'HP-VICTUS-15-FB3004LA',
       'Notebook gamer HP Victus con AMD Ryzen 9, 16 GB de RAM, SSD de 1 TB, graficos NVIDIA RTX 4060 de 8 GB y pantalla FHD de 15.6 pulgadas.'
FROM product_category category
JOIN brand ON LOWER(brand.name) = LOWER('HP')
WHERE LOWER(category.name) = LOWER('Computadores')
ON CONFLICT (sku) DO UPDATE
SET category_id = EXCLUDED.category_id,
    brand_id = EXCLUDED.brand_id,
    name = EXCLUDED.name,
    model = EXCLUDED.model,
    description = EXCLUDED.description,
    active = TRUE;

INSERT INTO product (category_id, brand_id, name, model, sku, description)
SELECT category.id, brand.id,
       'Notebook Gamer ASUS TUF Gaming A15 FA506 Ryzen 7',
       'FA506',
       'ASUS-TUF-A15-FA506-R7-RTX3050',
       'Notebook gamer ASUS TUF Gaming A15 con AMD Ryzen 7, 16 GB de RAM, SSD de 512 GB, NVIDIA GeForce RTX 3050 y pantalla Full HD de 15.6 pulgadas a 144 Hz.'
FROM product_category category
JOIN brand ON LOWER(brand.name) = LOWER('ASUS')
WHERE LOWER(category.name) = LOWER('Computadores')
ON CONFLICT (sku) DO UPDATE
SET category_id = EXCLUDED.category_id,
    brand_id = EXCLUDED.brand_id,
    name = EXCLUDED.name,
    model = EXCLUDED.model,
    description = EXCLUDED.description,
    active = TRUE;

INSERT INTO product (category_id, brand_id, name, model, sku, description)
SELECT category.id, brand.id,
       'Notebook Gamer Lenovo LOQ Ryzen 7 RTX 5060',
       'LOQ Ryzen 7 RTX 5060',
       'LENOVO-LOQ-R7-RTX5060-16-512',
       'Notebook gamer Lenovo LOQ con AMD Ryzen 7, 16 GB de RAM, SSD de 512 GB, RTX 5060 de 8 GB y pantalla FHD de 15.6 pulgadas a 144 Hz.'
FROM product_category category
JOIN brand ON LOWER(brand.name) = LOWER('Lenovo')
WHERE LOWER(category.name) = LOWER('Computadores')
ON CONFLICT (sku) DO UPDATE
SET category_id = EXCLUDED.category_id,
    brand_id = EXCLUDED.brand_id,
    name = EXCLUDED.name,
    model = EXCLUDED.model,
    description = EXCLUDED.description,
    active = TRUE;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM user_account
        WHERE LOWER(email) = LOWER('camila.fuentes@example.com')
    ) THEN
        RAISE EXCEPTION 'User camila.fuentes@example.com must exist before adding the product rating';
    END IF;
    IF NOT EXISTS (
        SELECT 1
        FROM user_account
        WHERE LOWER(email) = LOWER('jorge.munoz@example.com')
    ) THEN
        RAISE EXCEPTION 'User jorge.munoz@example.com must exist before adding the product rating';
    END IF;
END
$$;

INSERT INTO product_review
    (user_id, product_id, rating, title, content, verified)
SELECT account.id, product.id, 5, 'Calificacion de catalogo',
       'Calificacion de 5 estrellas agregada al catalogo.', FALSE
FROM user_account account
CROSS JOIN product
WHERE LOWER(account.email) = LOWER('camila.fuentes@example.com')
  AND product.sku = 'HP-VICTUS-15-FB3004LA'
ON CONFLICT (user_id, product_id) DO UPDATE
SET rating = EXCLUDED.rating,
    updated_at = NOW();

INSERT INTO product_review
    (user_id, product_id, rating, title, content, verified)
SELECT account.id, product.id, 3, 'Calificacion de catalogo',
       'Calificacion de 3 estrellas agregada al catalogo.', FALSE
FROM user_account account
CROSS JOIN product
WHERE LOWER(account.email) = LOWER('jorge.munoz@example.com')
  AND product.sku = 'HP-VICTUS-15-FB3004LA'
ON CONFLICT (user_id, product_id) DO UPDATE
SET rating = EXCLUDED.rating,
    updated_at = NOW();

INSERT INTO product_review
    (user_id, product_id, rating, title, content, verified)
SELECT account.id, product.id, 5, 'Calificacion de catalogo',
       'Calificacion de 5 estrellas agregada al catalogo.', FALSE
FROM user_account account
CROSS JOIN product
WHERE LOWER(account.email) = LOWER('camila.fuentes@example.com')
  AND product.sku = 'ASUS-TUF-A15-FA506-R7-RTX3050'
ON CONFLICT (user_id, product_id) DO UPDATE
SET rating = EXCLUDED.rating,
    updated_at = NOW();

INSERT INTO product_review
    (user_id, product_id, rating, title, content, verified)
SELECT account.id, product.id, 2, 'Calificacion de catalogo',
       'Calificacion de 2 estrellas agregada al catalogo.', FALSE
FROM user_account account
CROSS JOIN product
WHERE LOWER(account.email) = LOWER('jorge.munoz@example.com')
  AND product.sku = 'ASUS-TUF-A15-FA506-R7-RTX3050'
ON CONFLICT (user_id, product_id) DO UPDATE
SET rating = EXCLUDED.rating,
    updated_at = NOW();

INSERT INTO product_review
    (user_id, product_id, rating, title, content, verified)
SELECT account.id, product.id, 4, 'Calificacion de catalogo',
       'Calificacion de 4 estrellas agregada al catalogo.', FALSE
FROM user_account account
CROSS JOIN product
WHERE LOWER(account.email) = LOWER('jorge.munoz@example.com')
  AND product.sku = 'LENOVO-LOQ-R7-RTX5060-16-512'
ON CONFLICT (user_id, product_id) DO UPDATE
SET rating = EXCLUDED.rating,
    updated_at = NOW();

INSERT INTO product_review
    (user_id, product_id, rating, title, content, verified)
SELECT account.id, product.id, 3, 'Calificacion de catalogo',
       'Calificacion de 3 estrellas agregada al catalogo.', FALSE
FROM user_account account
CROSS JOIN product
WHERE LOWER(account.email) = LOWER('camila.fuentes@example.com')
  AND product.sku = 'LENOVO-LOQ-R7-RTX5060-16-512'
ON CONFLICT (user_id, product_id) DO UPDATE
SET rating = EXCLUDED.rating,
    updated_at = NOW();

INSERT INTO product_specification_value (product_id, specification_id, value)
SELECT product.id, specification.id, value.value
FROM (VALUES
    ('HP-VICTUS-15-FB3004LA', 'Almacenamiento', '1000'),
    ('HP-VICTUS-15-FB3004LA', 'Memoria RAM', '16'),
    ('HP-VICTUS-15-FB3004LA', 'Pantalla', '15.6'),
    ('HP-VICTUS-15-FB3004LA', 'Procesador', 'AMD Ryzen 9'),
    ('HP-VICTUS-15-FB3004LA', 'Tarjeta grafica', 'NVIDIA GeForce RTX 4060 8 GB'),
    ('HP-VICTUS-15-FB3004LA', 'Sistema operativo', 'Windows 11 Home'),
    ('HP-VICTUS-15-FB3004LA', 'Frecuencia de pantalla', '144'),
    ('ASUS-TUF-A15-FA506-R7-RTX3050', 'Almacenamiento', '512'),
    ('ASUS-TUF-A15-FA506-R7-RTX3050', 'Memoria RAM', '16'),
    ('ASUS-TUF-A15-FA506-R7-RTX3050', 'Pantalla', '15.6'),
    ('ASUS-TUF-A15-FA506-R7-RTX3050', 'Procesador', 'AMD Ryzen 7'),
    ('ASUS-TUF-A15-FA506-R7-RTX3050', 'Tarjeta grafica', 'NVIDIA GeForce RTX 3050'),
    ('ASUS-TUF-A15-FA506-R7-RTX3050', 'Sistema operativo', 'Windows 11 Home'),
    ('ASUS-TUF-A15-FA506-R7-RTX3050', 'Frecuencia de pantalla', '144'),
    ('LENOVO-LOQ-R7-RTX5060-16-512', 'Almacenamiento', '512'),
    ('LENOVO-LOQ-R7-RTX5060-16-512', 'Memoria RAM', '16'),
    ('LENOVO-LOQ-R7-RTX5060-16-512', 'Pantalla', '15.6'),
    ('LENOVO-LOQ-R7-RTX5060-16-512', 'Procesador', 'AMD Ryzen 7'),
    ('LENOVO-LOQ-R7-RTX5060-16-512', 'Tarjeta grafica', 'NVIDIA GeForce RTX 5060 8 GB'),
    ('LENOVO-LOQ-R7-RTX5060-16-512', 'Sistema operativo', 'Windows 11 Home'),
    ('LENOVO-LOQ-R7-RTX5060-16-512', 'Frecuencia de pantalla', '144')
) AS value(sku, specification_name, value)
JOIN product ON product.sku = value.sku
JOIN product_category_specification specification
    ON specification.category_id = product.category_id
   AND LOWER(specification.name) = LOWER(value.specification_name)
ON CONFLICT (product_id, specification_id) DO UPDATE
SET value = EXCLUDED.value;

UPDATE product_image image
SET image_url = 'https://i5.walmartimages.cl/asr/76c30d70-f7a1-4a25-a3ff-1b874a402fda.fadd6009f9613b590d39e3c950e647e4.jpeg?odnHeight=612&odnWidth=612&odnBg=FFFFFF',
    alt_text = 'Notebook Gamer HP Victus 15-fb3004la',
    sort_order = 0,
    active = TRUE
FROM product
WHERE image.product_id = product.id
  AND product.sku = 'HP-VICTUS-15-FB3004LA'
  AND image.image_url = 'https://placehold.co/640x640?text=HP-VICTUS-15-FB3004LA';

INSERT INTO product_image (product_id, image_url, alt_text, sort_order)
SELECT product.id,
       'https://i5.walmartimages.cl/asr/76c30d70-f7a1-4a25-a3ff-1b874a402fda.fadd6009f9613b590d39e3c950e647e4.jpeg?odnHeight=612&odnWidth=612&odnBg=FFFFFF',
       'Notebook Gamer HP Victus 15-fb3004la',
       0
FROM product
WHERE product.sku = 'HP-VICTUS-15-FB3004LA'
  AND NOT EXISTS (
      SELECT 1
      FROM product_image image
      WHERE image.product_id = product.id
        AND image.image_url = 'https://i5.walmartimages.cl/asr/76c30d70-f7a1-4a25-a3ff-1b874a402fda.fadd6009f9613b590d39e3c950e647e4.jpeg?odnHeight=612&odnWidth=612&odnBg=FFFFFF'
  );

UPDATE product_image image
SET image_url = 'https://cl-dam-resizer.ecomm.cencosud.com/unsafe/adaptive-fit-in/640x0/filters:quality(75)/cl/paris/406334999/variant/6a68cd9f35ff918015bdb279/images/c73a555b-f6a2-411d-9a67-e79eaf1285c3/406334999-0000-002.jpg',
    alt_text = 'Notebook Gamer ASUS TUF Gaming A15 FA506',
    sort_order = 0,
    active = TRUE
FROM product
WHERE image.product_id = product.id
  AND product.sku = 'ASUS-TUF-A15-FA506-R7-RTX3050'
  AND image.image_url = 'https://placehold.co/640x640?text=ASUS-TUF-A15-FA506-R7-RTX3050';

INSERT INTO product_image (product_id, image_url, alt_text, sort_order)
SELECT product.id,
       'https://cl-dam-resizer.ecomm.cencosud.com/unsafe/adaptive-fit-in/640x0/filters:quality(75)/cl/paris/406334999/variant/6a68cd9f35ff918015bdb279/images/c73a555b-f6a2-411d-9a67-e79eaf1285c3/406334999-0000-002.jpg',
       'Notebook Gamer ASUS TUF Gaming A15 FA506',
       0
FROM product
WHERE product.sku = 'ASUS-TUF-A15-FA506-R7-RTX3050'
  AND NOT EXISTS (
      SELECT 1
      FROM product_image image
      WHERE image.product_id = product.id
        AND image.image_url = 'https://cl-dam-resizer.ecomm.cencosud.com/unsafe/adaptive-fit-in/640x0/filters:quality(75)/cl/paris/406334999/variant/6a68cd9f35ff918015bdb279/images/c73a555b-f6a2-411d-9a67-e79eaf1285c3/406334999-0000-002.jpg'
  );

UPDATE product_image image
SET image_url = 'https://cl-dam-resizer.ecomm.cencosud.com/unsafe/adaptive-fit-in/640x0/filters:quality(75)/cl/paris/430218999/variant/69b401aef6a345db5f346045/images/d3caba1f-b369-4d37-8333-cf371b4780d7/430218999-0000-002.jpg',
    alt_text = 'Notebook Gamer Lenovo LOQ RTX 5060',
    sort_order = 0,
    active = TRUE
FROM product
WHERE image.product_id = product.id
  AND product.sku = 'LENOVO-LOQ-R7-RTX5060-16-512'
  AND image.image_url = 'https://placehold.co/640x640?text=LENOVO-LOQ-R7-RTX5060-16-512';

INSERT INTO product_image (product_id, image_url, alt_text, sort_order)
SELECT product.id,
       'https://cl-dam-resizer.ecomm.cencosud.com/unsafe/adaptive-fit-in/640x0/filters:quality(75)/cl/paris/430218999/variant/69b401aef6a345db5f346045/images/d3caba1f-b369-4d37-8333-cf371b4780d7/430218999-0000-002.jpg',
       'Notebook Gamer Lenovo LOQ RTX 5060',
       0
FROM product
WHERE product.sku = 'LENOVO-LOQ-R7-RTX5060-16-512'
  AND NOT EXISTS (
      SELECT 1
      FROM product_image image
      WHERE image.product_id = product.id
        AND image.image_url = 'https://cl-dam-resizer.ecomm.cencosud.com/unsafe/adaptive-fit-in/640x0/filters:quality(75)/cl/paris/430218999/variant/69b401aef6a345db5f346045/images/d3caba1f-b369-4d37-8333-cf371b4780d7/430218999-0000-002.jpg'
  );

INSERT INTO product_offer
    (product_id, store_id, price, list_price, currency, shipping_cost, shipping_free,
     available, stock, condition, product_url)
SELECT product.id, store.id, offer.price, offer.list_price, 'CLP',
       offer.shipping_cost, offer.shipping_free,
       TRUE, NULL, 'new', offer.product_url
FROM (VALUES
    ('HP-VICTUS-15-FB3004LA', 'Paris', 1489990, NULL::NUMERIC, 3990, FALSE,
     'https://www.paris.cl/notebook-gamer-victus-hp-15-fb3004la-amd-ryzen-9-16gb-ram-1tb-ssd-rtx-4060-8gb-156-fhd-w11h-MKFLHWH2G8.html'),
    ('HP-VICTUS-15-FB3004LA', 'Lider', 1489990, NULL::NUMERIC, 0, TRUE,
     'https://www.lider.cl/ip/computadores-y-tablets/notebook-gamer-victus-hp-15-fb3004la-amd-ryzen-9-16gb-ram-1tb-ssd-rtx-4060-8gb-15-6-fhd-w11h/00019899023818'),
    ('ASUS-TUF-A15-FA506-R7-RTX3050', 'Falabella', 749990, NULL::NUMERIC, 0, TRUE,
     'https://www.falabella.com/falabella-cl/product/80758852/asus-tuf-gaming-a15-fa506-amd-ryzen-7-nvidia-rtx-3050-16gb-ram-512gb-15-6-fhd-144hz/80758852'),
    ('ASUS-TUF-A15-FA506-R7-RTX3050', 'Paris', 739990, NULL::NUMERIC, 7990, FALSE,
     'https://www.paris.cl/notebook-gamer-tuf-a15-fa506-amd-ryzen-7-nvidia-rtx-3050-16gb-ram-512gb-ssd-156-fhd-144hz-406334999.html'),
    ('LENOVO-LOQ-R7-RTX5060-16-512', 'Paris', 1249990, NULL::NUMERIC, 0, TRUE,
     'https://www.paris.cl/notebook-gamer-loq-amd-ryzen-7-250-rtx-5060-8gb-16gb-ram-512gb-ssd-156-fhd-144hz-430218999.html?utm_source=soicos&utm_term=2605571608&utm_medium=referral'),
    ('LENOVO-LOQ-R7-RTX5060-16-512', 'Falabella', 1319990, NULL::NUMERIC, 4990, FALSE,
     'https://www.falabella.com/falabella-cl/product/80643914/notebook-gamer-loq-ryzen-7-260-16gb-ram-512gb-ssd-rtx-5060-8gb-15-6-fhd-144hz-lenovo/80643914?kid=aff5fc&utm_campaign=FACL-LAB-TC-INSTI-Todo_El_Sitio-Ago25-AF-Solotodo-Conversion-Mix&utm_source=Solotodo&utm_medium=affiliate&utm_id=aff5fc')
) AS offer(sku, store_name, price, list_price, shipping_cost, shipping_free, product_url)
JOIN product ON product.sku = offer.sku
JOIN store ON LOWER(store.name) = LOWER(offer.store_name)
ON CONFLICT (product_id, store_id) DO UPDATE
SET price = EXCLUDED.price,
    list_price = EXCLUDED.list_price,
    currency = EXCLUDED.currency,
    shipping_cost = EXCLUDED.shipping_cost,
    shipping_free = EXCLUDED.shipping_free,
    available = EXCLUDED.available,
    stock = EXCLUDED.stock,
    condition = EXCLUDED.condition,
    product_url = EXCLUDED.product_url,
    last_updated = NOW();

INSERT INTO product_price_history (product_offer_id, price, is_promotional)
SELECT offer.id, offer.price, FALSE
FROM product_offer offer
JOIN product ON product.id = offer.product_id
WHERE product.sku IN (
    'HP-VICTUS-15-FB3004LA',
    'ASUS-TUF-A15-FA506-R7-RTX3050',
    'LENOVO-LOQ-R7-RTX5060-16-512'
)
  AND NOT EXISTS (
      SELECT 1
      FROM product_price_history history
      WHERE history.product_offer_id = offer.id
        AND history.price = offer.price
        AND NOT history.is_promotional
        AND history.recorded_at >= CURRENT_DATE
  );

COMMIT;
