-- Datos mínimos sintéticos para reejecutar consultas.sql en una base desechable.
-- Incluye casos observables de URL, dominios, historial y especificaciones.
BEGIN;

INSERT INTO product_category (id, parent_category_id, name)
VALUES
    (1, NULL, 'Electrodomésticos'),
    (2, 1, 'Refrigeradores'),
    (3, NULL, 'Electrónica'),
    (4, 3, 'Televisores');

INSERT INTO product (
    id, category_id, name, model, sku
) VALUES
    (1, 2, 'Refrigerador Demo', 'R-100', 'TAL59-P1'),
    (2, 2, 'Refrigerador Demo', 'R-100', 'TAL59-P2'),
    (3, 4, 'Televisor Demo', 'T-200', 'TAL59-P3');

INSERT INTO product_category_specification (id, category_id, name, data_type)
VALUES
    (1, 2, 'Volumen', 'number'),
    (2, 4, 'Ancho pantalla', 'number'),
    (3, 4, 'HDR', 'boolean');

INSERT INTO product_specification_value (id, product_id, specification_id, value)
VALUES
    (1, 1, 1, '350'),
    (2, 1, 2, 'no-numérico'),
    (3, 3, 3, 'quizás');

INSERT INTO store (id, name, website_url)
VALUES
    (1, 'Tienda Demo Uno', 'https://shop.example.test'),
    (2, 'Tienda Demo Dos', 'https://www.shop.example.test/catalog');

INSERT INTO product_offer (
    id, product_id, store_id, price, list_price, currency, available, stock, product_url
) VALUES
    (1, 1, 1, 120, 100, 'USD', FALSE, 0, 'https://shop.example.test/item?utm_source=fixture/'),
    (2, 2, 1, 80, 110, 'EUR', TRUE, 5, 'https://shop.example.test/item?utm_source=fixture/'),
    (3, 3, 2, 200, 220, 'USD', TRUE, 3, 'https://shop.example.test/item?utm_source=fixture/'),
    (4, 1, 2, 125, 130, 'USD', TRUE, 1, NULL);

INSERT INTO product_price_history (id, product_offer_id, price, recorded_at)
VALUES
    (1, 1, 110, '2026-10-20 10:00:00'),
    (2, 1, 110, '2026-10-21 10:00:00'),
    (3, 3, 200, '2026-10-21 10:00:00');

COMMIT;
