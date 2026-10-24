BEGIN;


INSERT INTO brand (name, logo_url, website_url)
SELECT 'Apple', 'https://placehold.co/128x128?text=Apple', 'https://www.apple.com'
WHERE NOT EXISTS (
    SELECT 1 FROM brand WHERE LOWER(name) = LOWER('Apple')
);

INSERT INTO brand (name, logo_url, website_url)
SELECT 'Samsung', 'https://placehold.co/128x128?text=Samsung', 'https://www.samsung.com'
WHERE NOT EXISTS (
    SELECT 1 FROM brand WHERE LOWER(name) = LOWER('Samsung')
);

INSERT INTO brand (name, logo_url, website_url)
SELECT 'Xiaomi', 'https://placehold.co/128x128?text=Xiaomi', 'https://www.mi.com'
WHERE NOT EXISTS (
    SELECT 1 FROM brand WHERE LOWER(name) = LOWER('Xiaomi')
);

INSERT INTO product_category (name, description)
SELECT 'Telefonia', 'Telefonos celulares y accesorios'
WHERE NOT EXISTS (
    SELECT 1 FROM product_category WHERE LOWER(name) = LOWER('Telefonia')
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
    ('Camara principal', 'number', 'MP', 4)
) AS specification(name, data_type, unit, display_order)
WHERE LOWER(category.name) = LOWER('Telefonia')
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
    ('Ripley', 'https://simple.ripley.cl', 'https://placehold.co/128x128?text=Ripley',
     4.10, 'Buena', 'Despacho segun direccion y disponibilidad', 'Garantia legal y del fabricante'),
    ('Falabella', 'https://www.falabella.com', 'https://placehold.co/128x128?text=Falabella',
     4.20, 'Buena', 'Despacho segun direccion y disponibilidad', 'Garantia legal y del fabricante'),
    ('Lider', 'https://www.lider.cl', 'https://placehold.co/128x128?text=Lider',
     4.00, 'Buena', 'Despacho segun direccion y disponibilidad', 'Garantia legal y del fabricante'),
    ('Paris', 'https://www.paris.cl', 'https://placehold.co/128x128?text=Paris',
     4.00, 'Buena', 'Despacho segun direccion y disponibilidad', 'Garantia legal y del fabricante')
) AS store(name, website_url, logo_url, rating, reputation, shipping_information, general_conditions)
WHERE NOT EXISTS (
    SELECT 1 FROM store existing WHERE LOWER(existing.name) = LOWER(store.name)
);

INSERT INTO product (category_id, brand_id, name, model, sku, description)
SELECT category.id, brand.id,
       'Apple iPhone 16 128 GB',
       'iPhone 16',
       'APPLE-IPHONE-16-128',
       'iPhone 16 con 128 GB de almacenamiento, pantalla de 6.1 pulgadas y camara principal de 48 MP.'
FROM product_category category
JOIN brand ON LOWER(brand.name) = LOWER('Apple')
WHERE LOWER(category.name) = LOWER('Telefonia')
ON CONFLICT (sku) DO UPDATE
SET category_id = EXCLUDED.category_id,
    brand_id = EXCLUDED.brand_id,
    name = EXCLUDED.name,
    model = EXCLUDED.model,
    description = EXCLUDED.description,
    active = TRUE;

INSERT INTO product (category_id, brand_id, name, model, sku, description)
SELECT category.id, brand.id,
       'Samsung Galaxy S26 Ultra 256 GB',
       'Galaxy S26 Ultra',
       'SAMSUNG-GALAXY-S26-ULTRA-256',
       'Samsung Galaxy S26 Ultra con 256 GB de almacenamiento, 12 GB de RAM, pantalla de 6.9 pulgadas y camara principal de 200 MP.'
FROM product_category category
JOIN brand ON LOWER(brand.name) = LOWER('Samsung')
WHERE LOWER(category.name) = LOWER('Telefonia')
ON CONFLICT (sku) DO UPDATE
SET category_id = EXCLUDED.category_id,
    brand_id = EXCLUDED.brand_id,
    name = EXCLUDED.name,
    model = EXCLUDED.model,
    description = EXCLUDED.description,
    active = TRUE;

INSERT INTO product (category_id, brand_id, name, model, sku, description)
SELECT category.id, brand.id,
       'Xiaomi 17 Ultra 512 GB 5G Negro con Photography Kit',
       'Xiaomi 17 Ultra',
       'XIAOMI-17-ULTRA-512',
       'Xiaomi 17 Ultra negro con 512 GB de almacenamiento, conectividad 5G y Photography Kit.'
FROM product_category category
JOIN brand ON LOWER(brand.name) = LOWER('Xiaomi')
WHERE LOWER(category.name) = LOWER('Telefonia')
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
  AND product.sku = 'APPLE-IPHONE-16-128'
ON CONFLICT (user_id, product_id) DO UPDATE
SET rating = EXCLUDED.rating,
    updated_at = NOW();

INSERT INTO product_specification_value (product_id, specification_id, value)
SELECT product.id, specification.id, value.value
FROM (VALUES
    ('APPLE-IPHONE-16-128', 'Almacenamiento', '128'),
    ('APPLE-IPHONE-16-128', 'Memoria RAM', '8'),
    ('APPLE-IPHONE-16-128', 'Pantalla', '6.1'),
    ('APPLE-IPHONE-16-128', 'Camara principal', '48'),
    ('SAMSUNG-GALAXY-S26-ULTRA-256', 'Almacenamiento', '256'),
    ('SAMSUNG-GALAXY-S26-ULTRA-256', 'Memoria RAM', '12'),
    ('SAMSUNG-GALAXY-S26-ULTRA-256', 'Pantalla', '6.9'),
    ('SAMSUNG-GALAXY-S26-ULTRA-256', 'Camara principal', '200'),
    ('XIAOMI-17-ULTRA-512', 'Almacenamiento', '512'),
    ('XIAOMI-17-ULTRA-512', 'Memoria RAM', '16'),
    ('XIAOMI-17-ULTRA-512', 'Pantalla', '6.9'),
    ('XIAOMI-17-ULTRA-512', 'Camara principal', '50.0')
) AS value(sku, specification_name, value)
JOIN product ON product.sku = value.sku
JOIN product_category_specification specification
    ON specification.category_id = product.category_id
   AND LOWER(specification.name) = LOWER(value.specification_name)
ON CONFLICT (product_id, specification_id) DO UPDATE
SET value = EXCLUDED.value;

INSERT INTO product_image (product_id, image_url, alt_text, sort_order)
SELECT product.id,
       'https://media.falabella.com/falabellaCL/17273275_1/w=1200,h=1200,fit=pad',
       product.name,
       0
FROM product
WHERE product.sku = 'APPLE-IPHONE-16-128'
  AND NOT EXISTS (
      SELECT 1
      FROM product_image image
      WHERE image.product_id = product.id
        AND image.image_url = 'https://media.falabella.com/falabellaCL/17273275_1/w=1200,h=1200,fit=pad'
  );

INSERT INTO product_image (product_id, image_url, alt_text, sort_order)
SELECT product.id,
       'https://rimage.ripley.cl/home.ripley/Attachment/WOP/1/2000409988709/full_image-2000409988709',
       product.name,
       0
FROM product
WHERE product.sku = 'SAMSUNG-GALAXY-S26-ULTRA-256'
  AND NOT EXISTS (
      SELECT 1
      FROM product_image image
      WHERE image.product_id = product.id
        AND image.image_url = 'https://rimage.ripley.cl/home.ripley/Attachment/WOP/1/2000409988709/full_image-2000409988709'
  );

INSERT INTO product_image (product_id, image_url, alt_text, sort_order)
SELECT product.id,
       'https://cl-dam-resizer.ecomm.cencosud.com/unsafe/adaptive-fit-in/640x0/filters:quality(75)/cl/paris/431009999/variant/69e11d78ed6d6a7878437aba/images/d01e773c-abef-4fbd-8119-ba27a749ed4a/431009999-0000-001.jpg',
       product.name,
       0
FROM product
WHERE product.sku = 'XIAOMI-17-ULTRA-512'
  AND NOT EXISTS (
      SELECT 1
      FROM product_image image
      WHERE image.product_id = product.id
        AND image.image_url = 'https://cl-dam-resizer.ecomm.cencosud.com/unsafe/adaptive-fit-in/640x0/filters:quality(75)/cl/paris/431009999/variant/69e11d78ed6d6a7878437aba/images/d01e773c-abef-4fbd-8119-ba27a749ed4a/431009999-0000-001.jpg'
  );

INSERT INTO product_offer
    (product_id, store_id, price, list_price, currency, shipping_cost, shipping_free,
     available, stock, condition, product_url)
SELECT product.id, store.id, offer.price, offer.list_price, 'CLP',
       offer.shipping_cost, offer.shipping_free,
       TRUE, NULL, 'new', offer.product_url
FROM (VALUES
    ('APPLE-IPHONE-16-128', 'Ripley', 729990, NULL::NUMERIC, 0, TRUE,
     'https://simple.ripley.cl/iphone-16-128gb-48mp-61-2000403722538?color_80=Rosado&s=mdco&pos=3&catPos=3&p=1&ps=57&cat=celulares_tecno&orig=PLP&prodNavFullCategory=Tecno+'),
    ('APPLE-IPHONE-16-128', 'Falabella', 869990, NULL::NUMERIC, 4990, FALSE,
     'https://www.falabella.com/falabella-cl/product/prod121902204/apple-iphone-16/17273275'),
    ('SAMSUNG-GALAXY-S26-ULTRA-256', 'Ripley', 869990, NULL::NUMERIC, 0, TRUE,
     'https://simple.ripley.cl/smartphone-samsung-galaxy-s26-ultra-256gb-12gb-ram-200mp-69-2000409988662?color_80=Violeta&ists=true&tsi=rOHauQoQBqxYQlSTcaSuBLeI3bLvWRIQAZ5vhLzkf2C--arxjMYJVBoQAZrGDkdgciKspyvUen7t5SIoCiQ0OTIzNWI0MC05OGI5LTQ0MjgtOTU4Zi01YWY5YmQ3NjdkYTkQATCVl3xIAVC647CckTRgtg0gFyEAAAAAAAAAAAAAAAAAAAAAB4AoUBAADgQA&s=mdco&pos=2&catPos=2&p=1&ps=56&cat=marca_samsung_telefonia&orig=PLP&prodNavFullCategory=Marca+>+Samsung+>+Telefonía'),
    ('SAMSUNG-GALAXY-S26-ULTRA-256', 'Falabella', 929990, NULL::NUMERIC, 5990, FALSE,
     'https://www.falabella.com/falabella-cl/product/17667494/celular-samsung-galaxy-s26-ultra-256gb/17667495'),
    ('XIAOMI-17-ULTRA-512', 'Lider', 1149990, NULL::NUMERIC, 0, TRUE,
     'https://www.lider.cl/ip/telefonia/smartphone-xiaomi-17-ultra-512-gb-5g-negro-kit-de-fotografia/00040005135961'),
    ('XIAOMI-17-ULTRA-512', 'Paris', 1399990, NULL::NUMERIC, 3990, FALSE,
     'https://www.paris.cl/smartphone-17-ultra-5g-512gb-69-negro-liberado-photography-kit-431009999.html?utm_source=soicos&utm_term=2605591976&utm_medium=referral')
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
    'APPLE-IPHONE-16-128',
    'SAMSUNG-GALAXY-S26-ULTRA-256',
    'XIAOMI-17-ULTRA-512'
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
