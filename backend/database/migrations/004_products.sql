-- =====================================================================
-- SOLOServis - Productos adicionales
-- =====================================================================

-- MARCAS
INSERT INTO brand (name, logo_url, website_url)
VALUES
    ('Lenovo', 'https://upload.wikimedia.org/wikipedia/commons/0/03/Lenovo_logo_2015.svg', 'https://lenovo.com'),
    ('ASUS', 'https://upload.wikimedia.org/wikipedia/commons/2/2e/ASUS_logo.svg', 'https://asus.com'),
    ('HP', 'https://upload.wikimedia.org/wikipedia/commons/a/ad/HP_logo_2012.svg', 'https://hp.com'),
    ('Dell', 'https://upload.wikimedia.org/wikipedia/commons/4/48/Dell_logo_2016.svg', 'https://dell.com'),
    ('Acer', 'https://upload.wikimedia.org/wikipedia/commons/0/00/Acer_Logo.svg', 'https://acer.com'),
    ('MSI', 'https://upload.wikimedia.org/wikipedia/commons/7/7d/MSI_Logo.svg', 'https://msi.com')
ON CONFLICT (name) DO NOTHING;

-- CATEGORÍA
INSERT INTO product_category (parent_category_id, name, description)
VALUES (
    NULL,
    'Computadores',
    'Computadores y notebooks'
)
ON CONFLICT DO NOTHING;

-- PRODUCTOS INICIALES POR MARCA
INSERT INTO product (
    category_id,
    brand_id,
    name,
    model,
    sku,
    description
)
VALUES
    (
        (SELECT id FROM product_category WHERE name = 'Computadores' LIMIT 1),
        (SELECT id FROM brand WHERE name = 'Lenovo' LIMIT 1),
        'Lenovo LOQ Gen 9 (15" AMD) GeForce RTX 3050',
        'LOQ Gen 9',
        'LENOVO-LOQ-GEN9-3050',
        'Notebook gaming Lenovo LOQ Gen 9 con procesador AMD y GPU NVIDIA GeForce RTX 3050'
    ),
    (
        (SELECT id FROM product_category WHERE name = 'Computadores' LIMIT 1),
        (SELECT id FROM brand WHERE name = 'Lenovo' LIMIT 1),
        'Lenovo Ideapad Slim 5 Gen 9',
        'Ideapad Slim 5',
        'LENOVO-IDEAPAD-SLIM5-GEN9',
        'Notebook ultradelgada de uso diario con buen equilibrio entre rendimiento y portabilidad.'
    ),
    (
        (SELECT id FROM product_category WHERE name = 'Computadores' LIMIT 1),
        (SELECT id FROM brand WHERE name = 'Lenovo' LIMIT 1),
        'Lenovo ThinkPad T14 Gen 5',
        'ThinkPad T14',
        'LENOVO-THINKPAD-T14-GEN5',
        'Laptop empresarial con batería duradera, diseño sobrio y excelente desempeño para trabajo.'
    ),
    (
        (SELECT id FROM product_category WHERE name = 'Computadores' LIMIT 1),
        (SELECT id FROM brand WHERE name = 'ASUS' LIMIT 1),
        'ASUS ZenBook 14 OLED',
        'ZenBook 14',
        'ASUS-ZENBOOK-14-OLED',
        'Notebook premium con pantalla OLED, ligera y ideal para productividad y diseño.'
    ),
    (
        (SELECT id FROM product_category WHERE name = 'Computadores' LIMIT 1),
        (SELECT id FROM brand WHERE name = 'ASUS' LIMIT 1),
        'ASUS TUF Gaming A15',
        'TUF Gaming A15',
        'ASUS-TUF-A15',
        'Laptop para gaming con rendimiento sólido y ventilación reforzada para juego prolongado.'
    ),
    (
        (SELECT id FROM product_category WHERE name = 'Computadores' LIMIT 1),
        (SELECT id FROM brand WHERE name = 'HP' LIMIT 1),
        'HP Pavilion Aero 13',
        'Pavilion Aero 13',
        'HP-PAVILION-AERO-13',
        'Notebook liviana con excelente autonomía y gran experiencia de uso diario.'
    ),
    (
        (SELECT id FROM product_category WHERE name = 'Computadores' LIMIT 1),
        (SELECT id FROM brand WHERE name = 'HP' LIMIT 1),
        'HP Victus 15 Gaming',
        'Victus 15',
        'HP-VICTUS-15',
        'Notebook gaming orientada a rendimiento, ideal para jugar y multitarea intensiva.'
    ),
    (
        (SELECT id FROM product_category WHERE name = 'Computadores' LIMIT 1),
        (SELECT id FROM brand WHERE name = 'Dell' LIMIT 1),
        'Dell XPS 13 Plus',
        'XPS 13 Plus',
        'DELL-XPS-13-PLUS',
        'Ultrabook premium con diseño minimalista, brillante pantalla y gran sensación de elegancia.'
    ),
    (
        (SELECT id FROM product_category WHERE name = 'Computadores' LIMIT 1),
        (SELECT id FROM brand WHERE name = 'Dell' LIMIT 1),
        'Dell Latitude 5430',
        'Latitude 5430',
        'DELL-LATITUDE-5430',
        'Laptop empresarial para trabajo constante con robustez y seguridad corporativa.'
    ),
    (
        (SELECT id FROM product_category WHERE name = 'Computadores' LIMIT 1),
        (SELECT id FROM brand WHERE name = 'Acer' LIMIT 1),
        'Acer Aspire 5',
        'Aspire 5',
        'ACER-ASPIRE-5',
        'Notebook de uso general con buena relación precio-rendimiento para estudio y trabajo.'
    ),
    (
        (SELECT id FROM product_category WHERE name = 'Computadores' LIMIT 1),
        (SELECT id FROM brand WHERE name = 'Acer' LIMIT 1),
        'Acer Nitro 5',
        'Nitro 5',
        'ACER-NITRO-5',
        'Laptop gaming con buen rendimiento visual y capacidad para juegos modernos.'
    ),
    (
        (SELECT id FROM product_category WHERE name = 'Computadores' LIMIT 1),
        (SELECT id FROM brand WHERE name = 'MSI' LIMIT 1),
        'MSI GF63 Thin',
        'GF63 Thin',
        'MSI-GF63-THIN',
        'Notebook gaming compacta y ligera, pensada para usuarios que quieren performance sin mucho peso.'
    )
ON CONFLICT (sku) DO NOTHING;

-- OFERTAS Y PRECIOS PARA LOS PRODUCTOS SELECCIONADOS
INSERT INTO product_offer (
    product_id,
    store_id,
    price,
    list_price,
    currency,
    shipping_cost,
    shipping_free,
    available,
    stock,
    condition,
    product_url
)
VALUES
    ((SELECT id FROM product WHERE sku = 'LENOVO-LOQ-GEN9-3050'), 1, 1299900, 1399900, 'CLP', 0, TRUE, TRUE, 6, 'new', 'https://falabella.com/producto/lenovo-loq-gen9-3050'),
    ((SELECT id FROM product WHERE sku = 'LENOVO-LOQ-GEN9-3050'), 2, 1349900, 1399900, 'CLP', 5990, FALSE, TRUE, 4, 'new', 'https://paris.cl/producto/lenovo-loq-gen9-3050'),
    ((SELECT id FROM product WHERE sku = 'LENOVO-IDEAPAD-SLIM5-GEN9'), 1, 799990, 899990, 'CLP', 0, TRUE, TRUE, 12, 'new', 'https://falabella.com/producto/lenovo-ideapad-slim5-gen9'),
    ((SELECT id FROM product WHERE sku = 'LENOVO-IDEAPAD-SLIM5-GEN9'), 2, 829990, 899990, 'CLP', 4990, FALSE, TRUE, 9, 'new', 'https://paris.cl/producto/lenovo-ideapad-slim5-gen9'),
    ((SELECT id FROM product WHERE sku = 'LENOVO-THINKPAD-T14-GEN5'), 1, 1199990, 1299990, 'CLP', 0, TRUE, TRUE, 8, 'new', 'https://falabella.com/producto/lenovo-thinkpad-t14-gen5'),
    ((SELECT id FROM product WHERE sku = 'LENOVO-THINKPAD-T14-GEN5'), 2, 1249990, 1299990, 'CLP', 5990, FALSE, TRUE, 5, 'new', 'https://paris.cl/producto/lenovo-thinkpad-t14-gen5'),
    ((SELECT id FROM product WHERE sku = 'ASUS-ZENBOOK-14-OLED'), 1, 999990, 1099990, 'CLP', 0, TRUE, TRUE, 10, 'new', 'https://falabella.com/producto/asus-zenbook-14-oled'),
    ((SELECT id FROM product WHERE sku = 'ASUS-ZENBOOK-14-OLED'), 2, 1049990, 1099990, 'CLP', 4990, FALSE, TRUE, 7, 'new', 'https://paris.cl/producto/asus-zenbook-14-oled'),
    ((SELECT id FROM product WHERE sku = 'ASUS-TUF-A15'), 1, 899990, 999990, 'CLP', 0, TRUE, TRUE, 11, 'new', 'https://falabella.com/producto/asus-tuf-a15'),
    ((SELECT id FROM product WHERE sku = 'ASUS-TUF-A15'), 2, 939990, 999990, 'CLP', 5990, FALSE, TRUE, 8, 'new', 'https://paris.cl/producto/asus-tuf-a15'),
    ((SELECT id FROM product WHERE sku = 'HP-PAVILION-AERO-13'), 1, 769990, 839990, 'CLP', 0, TRUE, TRUE, 9, 'new', 'https://falabella.com/producto/hp-pavilion-aero-13'),
    ((SELECT id FROM product WHERE sku = 'HP-PAVILION-AERO-13'), 2, 789990, 839990, 'CLP', 3990, FALSE, TRUE, 6, 'new', 'https://paris.cl/producto/hp-pavilion-aero-13'),
    ((SELECT id FROM product WHERE sku = 'HP-VICTUS-15'), 1, 849990, 949990, 'CLP', 0, TRUE, TRUE, 7, 'new', 'https://falabella.com/producto/hp-victus-15'),
    ((SELECT id FROM product WHERE sku = 'HP-VICTUS-15'), 2, 899990, 949990, 'CLP', 5990, FALSE, TRUE, 5, 'new', 'https://paris.cl/producto/hp-victus-15'),
    ((SELECT id FROM product WHERE sku = 'DELL-XPS-13-PLUS'), 1, 1399990, 1499990, 'CLP', 0, TRUE, TRUE, 5, 'new', 'https://falabella.com/producto/dell-xps-13-plus'),
    ((SELECT id FROM product WHERE sku = 'DELL-XPS-13-PLUS'), 2, 1449990, 1499990, 'CLP', 4990, FALSE, TRUE, 4, 'new', 'https://paris.cl/producto/dell-xps-13-plus'),
    ((SELECT id FROM product WHERE sku = 'DELL-LATITUDE-5430'), 1, 1099990, 1199990, 'CLP', 0, TRUE, TRUE, 6, 'new', 'https://falabella.com/producto/dell-latitude-5430'),
    ((SELECT id FROM product WHERE sku = 'DELL-LATITUDE-5430'), 2, 1149990, 1199990, 'CLP', 4990, FALSE, TRUE, 4, 'new', 'https://paris.cl/producto/dell-latitude-5430'),
    ((SELECT id FROM product WHERE sku = 'ACER-ASPIRE-5'), 1, 599990, 699990, 'CLP', 0, TRUE, TRUE, 15, 'new', 'https://falabella.com/producto/acer-aspire-5'),
    ((SELECT id FROM product WHERE sku = 'ACER-ASPIRE-5'), 2, 629990, 699990, 'CLP', 3990, FALSE, TRUE, 12, 'new', 'https://paris.cl/producto/acer-aspire-5'),
    ((SELECT id FROM product WHERE sku = 'ACER-NITRO-5'), 1, 799990, 899990, 'CLP', 0, TRUE, TRUE, 8, 'new', 'https://falabella.com/producto/acer-nitro-5'),
    ((SELECT id FROM product WHERE sku = 'ACER-NITRO-5'), 2, 839990, 899990, 'CLP', 5990, FALSE, TRUE, 5, 'new', 'https://paris.cl/producto/acer-nitro-5'),
    ((SELECT id FROM product WHERE sku = 'MSI-GF63-THIN'), 1, 689990, 799990, 'CLP', 0, TRUE, TRUE, 10, 'new', 'https://falabella.com/producto/msi-gf63-thin'),
    ((SELECT id FROM product WHERE sku = 'MSI-GF63-THIN'), 2, 729990, 799990, 'CLP', 4990, FALSE, TRUE, 7, 'new', 'https://paris.cl/producto/msi-gf63-thin');

INSERT INTO product_price_history (
    product_offer_id,
    price,
    is_promotional,
    recorded_at
)
VALUES
    ((SELECT id FROM product_offer WHERE product_id = (SELECT id FROM product WHERE sku = 'LENOVO-LOQ-GEN9-3050') AND store_id = 1), 1399900, FALSE, NOW() - INTERVAL '30 days'),
    ((SELECT id FROM product_offer WHERE product_id = (SELECT id FROM product WHERE sku = 'LENOVO-LOQ-GEN9-3050') AND store_id = 1), 1299900, TRUE, NOW() - INTERVAL '2 days'),
    ((SELECT id FROM product_offer WHERE product_id = (SELECT id FROM product WHERE sku = 'LENOVO-IDEAPAD-SLIM5-GEN9') AND store_id = 1), 899990, FALSE, NOW() - INTERVAL '25 days'),
    ((SELECT id FROM product_offer WHERE product_id = (SELECT id FROM product WHERE sku = 'LENOVO-IDEAPAD-SLIM5-GEN9') AND store_id = 1), 799990, TRUE, NOW() - INTERVAL '3 days'),
    ((SELECT id FROM product_offer WHERE product_id = (SELECT id FROM product WHERE sku = 'LENOVO-THINKPAD-T14-GEN5') AND store_id = 1), 1299990, FALSE, NOW() - INTERVAL '20 days'),
    ((SELECT id FROM product_offer WHERE product_id = (SELECT id FROM product WHERE sku = 'LENOVO-THINKPAD-T14-GEN5') AND store_id = 1), 1199990, TRUE, NOW() - INTERVAL '4 days'),
    ((SELECT id FROM product_offer WHERE product_id = (SELECT id FROM product WHERE sku = 'ASUS-ZENBOOK-14-OLED') AND store_id = 1), 1099990, FALSE, NOW() - INTERVAL '26 days'),
    ((SELECT id FROM product_offer WHERE product_id = (SELECT id FROM product WHERE sku = 'ASUS-ZENBOOK-14-OLED') AND store_id = 1), 999990, TRUE, NOW() - INTERVAL '5 days'),
    ((SELECT id FROM product_offer WHERE product_id = (SELECT id FROM product WHERE sku = 'ASUS-TUF-A15') AND store_id = 1), 999990, FALSE, NOW() - INTERVAL '18 days'),
    ((SELECT id FROM product_offer WHERE product_id = (SELECT id FROM product WHERE sku = 'ASUS-TUF-A15') AND store_id = 1), 899990, TRUE, NOW() - INTERVAL '2 days'),
    ((SELECT id FROM product_offer WHERE product_id = (SELECT id FROM product WHERE sku = 'HP-PAVILION-AERO-13') AND store_id = 1), 839990, FALSE, NOW() - INTERVAL '21 days'),
    ((SELECT id FROM product_offer WHERE product_id = (SELECT id FROM product WHERE sku = 'HP-PAVILION-AERO-13') AND store_id = 1), 769990, TRUE, NOW() - INTERVAL '4 days'),
    ((SELECT id FROM product_offer WHERE product_id = (SELECT id FROM product WHERE sku = 'HP-VICTUS-15') AND store_id = 1), 949990, FALSE, NOW() - INTERVAL '22 days'),
    ((SELECT id FROM product_offer WHERE product_id = (SELECT id FROM product WHERE sku = 'HP-VICTUS-15') AND store_id = 1), 849990, TRUE, NOW() - INTERVAL '3 days'),
    ((SELECT id FROM product_offer WHERE product_id = (SELECT id FROM product WHERE sku = 'DELL-XPS-13-PLUS') AND store_id = 1), 1499990, FALSE, NOW() - INTERVAL '32 days'),
    ((SELECT id FROM product_offer WHERE product_id = (SELECT id FROM product WHERE sku = 'DELL-XPS-13-PLUS') AND store_id = 1), 1399990, TRUE, NOW() - INTERVAL '6 days'),
    ((SELECT id FROM product_offer WHERE product_id = (SELECT id FROM product WHERE sku = 'DELL-LATITUDE-5430') AND store_id = 1), 1199990, FALSE, NOW() - INTERVAL '19 days'),
    ((SELECT id FROM product_offer WHERE product_id = (SELECT id FROM product WHERE sku = 'DELL-LATITUDE-5430') AND store_id = 1), 1099990, TRUE, NOW() - INTERVAL '2 days'),
    ((SELECT id FROM product_offer WHERE product_id = (SELECT id FROM product WHERE sku = 'ACER-ASPIRE-5') AND store_id = 1), 699990, FALSE, NOW() - INTERVAL '18 days'),
    ((SELECT id FROM product_offer WHERE product_id = (SELECT id FROM product WHERE sku = 'ACER-ASPIRE-5') AND store_id = 1), 599990, TRUE, NOW() - INTERVAL '4 days'),
    ((SELECT id FROM product_offer WHERE product_id = (SELECT id FROM product WHERE sku = 'ACER-NITRO-5') AND store_id = 1), 899990, FALSE, NOW() - INTERVAL '24 days'),
    ((SELECT id FROM product_offer WHERE product_id = (SELECT id FROM product WHERE sku = 'ACER-NITRO-5') AND store_id = 1), 799990, TRUE, NOW() - INTERVAL '4 days'),
    ((SELECT id FROM product_offer WHERE product_id = (SELECT id FROM product WHERE sku = 'MSI-GF63-THIN') AND store_id = 1), 799990, FALSE, NOW() - INTERVAL '17 days'),
    ((SELECT id FROM product_offer WHERE product_id = (SELECT id FROM product WHERE sku = 'MSI-GF63-THIN') AND store_id = 1), 689990, TRUE, NOW() - INTERVAL '3 days');
