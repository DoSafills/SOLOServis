BEGIN;

INSERT INTO brand (name, logo_url, website_url)
SELECT 'Samsung', 'https://placehold.co/128x128?text=Samsung', 'https://www.samsung.com'
WHERE NOT EXISTS (
    SELECT 1 FROM brand WHERE LOWER(name) = LOWER('Samsung')
);

INSERT INTO brand (name, logo_url, website_url)
SELECT 'LG', 'https://placehold.co/128x128?text=LG', 'https://www.lg.com'
WHERE NOT EXISTS (
    SELECT 1 FROM brand WHERE LOWER(name) = LOWER('LG')
);

INSERT INTO brand (name, logo_url, website_url)
SELECT 'Midea', 'https://placehold.co/128x128?text=Midea', 'https://www.midea.com'
WHERE NOT EXISTS (
    SELECT 1 FROM brand WHERE LOWER(name) = LOWER('Midea')
);

INSERT INTO product_category (name, description)
SELECT 'Refrigeracion', 'Refrigeradores y equipos de frio'
WHERE NOT EXISTS (
    SELECT 1 FROM product_category WHERE LOWER(name) = LOWER('Refrigeracion')
);

INSERT INTO product_category_specification
    (category_id, name, data_type, unit, required, comparable, display_order)
SELECT category.id, 'Capacidad', 'number', 'L', FALSE, TRUE, 1
FROM product_category category
WHERE LOWER(category.name) = LOWER('Refrigeracion')
  AND NOT EXISTS (
      SELECT 1
      FROM product_category_specification specification
      WHERE specification.category_id = category.id
        AND LOWER(specification.name) = LOWER('Capacidad')
  );

INSERT INTO product_category_specification
    (category_id, name, data_type, unit, required, comparable, display_order)
SELECT category.id, 'Eficiencia energetica', 'string', NULL, FALSE, TRUE, 2
FROM product_category category
WHERE LOWER(category.name) = LOWER('Refrigeracion')
  AND NOT EXISTS (
      SELECT 1
      FROM product_category_specification specification
      WHERE specification.category_id = category.id
        AND LOWER(specification.name) = LOWER('Eficiencia energetica')
  );

INSERT INTO store
    (name, website_url, logo_url, rating, reputation, shipping_information, general_conditions)
SELECT store.name, store.website_url, store.logo_url, store.rating, store.reputation,
       store.shipping_information, store.general_conditions
FROM (VALUES
    ('Falabella', 'https://www.falabella.com', 'https://placehold.co/128x128?text=Falabella',
     4.20, 'Buena', 'Despacho segun direccion y disponibilidad', 'Garantia legal y del fabricante'),
    ('Paris', 'https://www.paris.cl', 'https://placehold.co/128x128?text=Paris',
     4.00, 'Buena', 'Despacho segun direccion y disponibilidad', 'Garantia legal y del fabricante'),
    ('Ripley', 'https://simple.ripley.cl', 'https://placehold.co/128x128?text=Ripley',
     4.00, 'Buena', 'Despacho segun direccion y disponibilidad', 'Garantia legal y del fabricante'),
    ('Lider', 'https://www.lider.cl', 'https://placehold.co/128x128?text=Lider',
     4.00, 'Buena', 'Despacho segun direccion y disponibilidad', 'Garantia legal y del fabricante')
) AS store(name, website_url, logo_url, rating, reputation, shipping_information, general_conditions)
WHERE NOT EXISTS (
    SELECT 1 FROM store existing WHERE LOWER(existing.name) = LOWER(store.name)
);

INSERT INTO product (category_id, brand_id, name, model, sku, description)
SELECT category.id, brand.id,
       'Refrigerador Samsung Top Freezer Space Max con dispensador',
       'MKF9HNZ5C6',
       'SAMSUNG-MKF9HNZ5C6',
       'Refrigerador Samsung Top Freezer Space Max con dispensador. Las publicaciones de Falabella y Paris informan capacidades de 384 L y 389 L, respectivamente.'
FROM product_category category
JOIN brand ON LOWER(brand.name) = LOWER('Samsung')
WHERE LOWER(category.name) = LOWER('Refrigeracion')
ON CONFLICT (sku) DO UPDATE
SET category_id = EXCLUDED.category_id,
    brand_id = EXCLUDED.brand_id,
    name = EXCLUDED.name,
    model = EXCLUDED.model,
    description = EXCLUDED.description,
    active = TRUE;

INSERT INTO product (category_id, brand_id, name, model, sku, description)
SELECT category.id, brand.id,
       'Refrigerador LG French Door No Frost Linear Cooling 533 L',
       'LM22SGPK',
       'LG-LM22SGPK',
       'Refrigerador LG French Door No Frost de 533 litros con tecnologia Linear Cooling.'
FROM product_category category
JOIN brand ON LOWER(brand.name) = LOWER('LG')
WHERE LOWER(category.name) = LOWER('Refrigeracion')
ON CONFLICT (sku) DO UPDATE
SET category_id = EXCLUDED.category_id,
    brand_id = EXCLUDED.brand_id,
    name = EXCLUDED.name,
    model = EXCLUDED.model,
    description = EXCLUDED.description,
    active = TRUE;

INSERT INTO product (category_id, brand_id, name, model, sku, description)
SELECT category.id, brand.id,
       'Refrigerador Midea Side by Side No Frost 555 L',
       'MDRS710FGE50IN',
       'MIDEA-MDRS710FGE50IN',
       'Refrigerador Midea Side by Side No Frost de 555 litros.'
FROM product_category category
JOIN brand ON LOWER(brand.name) = LOWER('Midea')
WHERE LOWER(category.name) = LOWER('Refrigeracion')
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
        WHERE LOWER(email) = LOWER('jorge.munoz@example.com')
    ) THEN
        RAISE EXCEPTION 'User jorge.munoz@example.com must exist before adding the product rating';
    END IF;
END
$$;

INSERT INTO product_review
    (user_id, product_id, rating, title, content, verified)
SELECT account.id, product.id, 4, 'Calificacion de catalogo',
       'Calificacion de 4 estrellas agregada al catalogo.', FALSE
FROM user_account account
CROSS JOIN product
WHERE LOWER(account.email) = LOWER('jorge.munoz@example.com')
  AND product.sku = 'SAMSUNG-MKF9HNZ5C6'
ON CONFLICT (user_id, product_id) DO UPDATE
SET rating = EXCLUDED.rating,
    updated_at = NOW();

INSERT INTO product_review
    (user_id, product_id, rating, title, content, verified)
SELECT account.id, product.id, 5, 'Calificacion de catalogo',
       'Calificacion de 5 estrellas agregada al catalogo.', FALSE
FROM user_account account
CROSS JOIN product
WHERE LOWER(account.email) = LOWER('jorge.munoz@example.com')
  AND product.sku = 'LG-LM22SGPK'
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
  AND product.sku = 'MIDEA-MDRS710FGE50IN'
ON CONFLICT (user_id, product_id) DO UPDATE
SET rating = EXCLUDED.rating,
    updated_at = NOW();

INSERT INTO product_specification_value (product_id, specification_id, value)
SELECT product.id, specification.id, '384'
FROM product
JOIN product_category_specification specification
    ON specification.category_id = product.category_id
   AND LOWER(specification.name) = LOWER('Capacidad')
WHERE product.sku = 'SAMSUNG-MKF9HNZ5C6'
ON CONFLICT (product_id, specification_id) DO UPDATE
SET value = EXCLUDED.value;

INSERT INTO product_specification_value (product_id, specification_id, value)
SELECT product.id, specification.id, '555'
FROM product
JOIN product_category_specification specification
    ON specification.category_id = product.category_id
   AND LOWER(specification.name) = LOWER('Capacidad')
WHERE product.sku = 'MIDEA-MDRS710FGE50IN'
ON CONFLICT (product_id, specification_id) DO UPDATE
SET value = EXCLUDED.value;

INSERT INTO product_specification_value (product_id, specification_id, value)
SELECT product.id, specification.id, '533'
FROM product
JOIN product_category_specification specification
    ON specification.category_id = product.category_id
   AND LOWER(specification.name) = LOWER('Capacidad')
WHERE product.sku = 'LG-LM22SGPK'
ON CONFLICT (product_id, specification_id) DO UPDATE
SET value = EXCLUDED.value;

INSERT INTO product_specification_value (product_id, specification_id, value)
SELECT product.id, specification.id, 'A+'
FROM product
JOIN product_category_specification specification
    ON specification.category_id = product.category_id
   AND LOWER(specification.name) = LOWER('Eficiencia energetica')
WHERE product.sku = 'LG-LM22SGPK'
ON CONFLICT (product_id, specification_id) DO UPDATE
SET value = EXCLUDED.value;

INSERT INTO product_specification_value (product_id, specification_id, value)
SELECT product.id, specification.id, 'E'
FROM product
JOIN product_category_specification specification
    ON specification.category_id = product.category_id
   AND LOWER(specification.name) = LOWER('Eficiencia energetica')
WHERE product.sku = 'SAMSUNG-MKF9HNZ5C6'
ON CONFLICT (product_id, specification_id) DO UPDATE
SET value = EXCLUDED.value;

INSERT INTO product_specification_value (product_id, specification_id, value)
SELECT product.id, specification.id, 'D'
FROM product
JOIN product_category_specification specification
    ON specification.category_id = product.category_id
   AND LOWER(specification.name) = LOWER('Eficiencia energetica')
WHERE product.sku = 'MIDEA-MDRS710FGE50IN'
ON CONFLICT (product_id, specification_id) DO UPDATE
SET value = EXCLUDED.value;

UPDATE product_image image
SET image_url = 'https://cl-cenco-pim-resizer.ecomm.cencosud.com/unsafe/adaptive-fit-in/640x0/filters:quality(75)/prd-cl/product-medias/3067ab7a-1f44-42a8-a06e-362135fe357d/MKF9HNZ5C6/MKF9HNZ5C6-1/1782225248877-MKF9HNZ5C6-1-1.jpg',
    alt_text = 'Refrigerador Samsung MKF9HNZ5C6',
    sort_order = 0,
    active = TRUE
FROM product
WHERE image.product_id = product.id
  AND product.sku = 'SAMSUNG-MKF9HNZ5C6'
  AND image.image_url = 'https://placehold.co/640x640?text=SAMSUNG-MKF9HNZ5C6';

INSERT INTO product_image (product_id, image_url, alt_text, sort_order)
SELECT product.id,
       'https://cl-cenco-pim-resizer.ecomm.cencosud.com/unsafe/adaptive-fit-in/640x0/filters:quality(75)/prd-cl/product-medias/3067ab7a-1f44-42a8-a06e-362135fe357d/MKF9HNZ5C6/MKF9HNZ5C6-1/1782225248877-MKF9HNZ5C6-1-1.jpg',
       'Refrigerador Samsung MKF9HNZ5C6',
       0
FROM product
WHERE product.sku = 'SAMSUNG-MKF9HNZ5C6'
  AND NOT EXISTS (
      SELECT 1
      FROM product_image image
      WHERE image.product_id = product.id
        AND image.image_url = 'https://cl-cenco-pim-resizer.ecomm.cencosud.com/unsafe/adaptive-fit-in/640x0/filters:quality(75)/prd-cl/product-medias/3067ab7a-1f44-42a8-a06e-362135fe357d/MKF9HNZ5C6/MKF9HNZ5C6-1/1782225248877-MKF9HNZ5C6-1-1.jpg'
  );

UPDATE product_image image
SET image_url = 'https://rimage.ripley.cl/home.ripley/Attachment/WOP/1/2000383288574/full_image-2000383288574',
    alt_text = 'Refrigerador LG LM22SGPK',
    sort_order = 0,
    active = TRUE
FROM product
WHERE image.product_id = product.id
  AND product.sku = 'LG-LM22SGPK'
  AND image.image_url = 'https://placehold.co/640x640?text=LG-LM22SGPK';

INSERT INTO product_image (product_id, image_url, alt_text, sort_order)
SELECT product.id,
       'https://rimage.ripley.cl/home.ripley/Attachment/WOP/1/2000383288574/full_image-2000383288574',
       'Refrigerador LG LM22SGPK',
       0
FROM product
WHERE product.sku = 'LG-LM22SGPK'
  AND NOT EXISTS (
      SELECT 1
      FROM product_image image
      WHERE image.product_id = product.id
        AND image.image_url = 'https://rimage.ripley.cl/home.ripley/Attachment/WOP/1/2000383288574/full_image-2000383288574'
  );

UPDATE product_image image
SET image_url = 'https://rimage.ripley.cl/home.ripley/Attachment/WOP/1/2000406824253/full_image-2000406824253',
    alt_text = 'Refrigerador Midea MDRS710FGE50IN',
    sort_order = 0,
    active = TRUE
FROM product
WHERE image.product_id = product.id
  AND product.sku = 'MIDEA-MDRS710FGE50IN'
  AND image.image_url = 'https://placehold.co/640x640?text=MIDEA-MDRS710FGE50IN';

INSERT INTO product_image (product_id, image_url, alt_text, sort_order)
SELECT product.id,
       'https://rimage.ripley.cl/home.ripley/Attachment/WOP/1/2000406824253/full_image-2000406824253',
       'Refrigerador Midea MDRS710FGE50IN',
       0
FROM product
WHERE product.sku = 'MIDEA-MDRS710FGE50IN'
  AND NOT EXISTS (
      SELECT 1
      FROM product_image image
      WHERE image.product_id = product.id
        AND image.image_url = 'https://rimage.ripley.cl/home.ripley/Attachment/WOP/1/2000406824253/full_image-2000406824253'
  );

INSERT INTO product_offer
    (product_id, store_id, price, list_price, currency, shipping_cost, shipping_free,
     available, stock, condition, product_url)
SELECT product.id, store.id, offer.price, offer.list_price, 'CLP',
       offer.shipping_cost, offer.shipping_free,
       TRUE, NULL, 'new', offer.product_url
FROM (VALUES
    ('SAMSUNG-MKF9HNZ5C6', 'Falabella', 559990, 649990, 6990, FALSE,
     'https://www.falabella.com/falabella-cl/product/131934453/refrigerador-top-mount-freezer-384l-space-max/131934454'),
    ('SAMSUNG-MKF9HNZ5C6', 'Paris', 559990, 709990, 0, TRUE,
     'https://www.paris.cl/refrigerador-top-freezer-389-lt-space-max-con-dispensador-MKF9HNZ5C6.html'),
    ('LG-LM22SGPK', 'Paris', 1449990, NULL, 8990, FALSE,
     'https://www.paris.cl/refrigerador-french-door-no-frost-533-litros-lm22sgpk-linear-cooling-184288999.html'),
    ('LG-LM22SGPK', 'Ripley', 1499990, NULL, 5990, FALSE,
     'https://simple.ripley.cl/refrigerador-lg-french-door-no-frost-533-l-linear-cooling-lm22sgpk-2000383288574p?color_80=Plata'),
    ('MIDEA-MDRS710FGE50IN', 'Falabella', 379990, NULL, 8990, FALSE,
     'https://www.falabella.com/falabella-cl/product/80526705/refrigerador-side-by-side-no-frost-555-litros-mdrs710fge50in-midea/80526705'),
    ('MIDEA-MDRS710FGE50IN', 'Ripley', 332991, NULL, 0, TRUE,
     'https://simple.ripley.cl/refrigerador-side-by-side-midea-mdrs710fge50in-555l-no-frost-gris-2000406824253p?color_80=Gris'),
    ('MIDEA-MDRS710FGE50IN', 'Lider', 379990, NULL, 4990, FALSE,
     'https://www.lider.cl/ip/refrigeracion/refrigerador-side-by-side-no-frost-555-litros-mdrs710fge50in-midea/00694046194209')
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
WHERE product.sku = 'MIDEA-MDRS710FGE50IN'
  AND NOT EXISTS (
      SELECT 1
      FROM product_price_history history
      WHERE history.product_offer_id = offer.id
        AND history.price = offer.price
        AND NOT history.is_promotional
        AND history.recorded_at >= CURRENT_DATE
  );

INSERT INTO product_price_history (product_offer_id, price, is_promotional)
SELECT offer.id, offer.price, FALSE
FROM product_offer offer
JOIN product ON product.id = offer.product_id
WHERE product.sku = 'LG-LM22SGPK'
  AND NOT EXISTS (
      SELECT 1
      FROM product_price_history history
      WHERE history.product_offer_id = offer.id
        AND history.price = offer.price
        AND NOT history.is_promotional
        AND history.recorded_at >= CURRENT_DATE
  );

INSERT INTO product_price_history (product_offer_id, price, is_promotional)
SELECT offer.id, offer.price, TRUE
FROM product_offer offer
JOIN product ON product.id = offer.product_id
WHERE product.sku = 'SAMSUNG-MKF9HNZ5C6'
  AND NOT EXISTS (
      SELECT 1
      FROM product_price_history history
      WHERE history.product_offer_id = offer.id
        AND history.price = offer.price
        AND history.is_promotional
        AND history.recorded_at >= CURRENT_DATE
  );

COMMIT;
