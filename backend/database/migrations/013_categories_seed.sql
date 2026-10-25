-- =====================================================================
-- TAL-71: datos iniciales de categorías y definiciones de atributos.
--
-- Idempotente (ON CONFLICT DO NOTHING). Sin BEGIN/COMMIT propio.
-- El enlace legacy_* se resuelve por nombre exacto; si la categoría
-- antigua no existe queda NULL. "Refrigeradores" y "Refrigeracion"
-- coexisten en product_category: se enlaza "Refrigeracion", que es la
-- que usan los seeds 005/006.
-- =====================================================================

INSERT INTO categories
    (slug, name, item_type, description, legacy_product_category_id, legacy_service_category_id)
VALUES
    ('refrigeradores', 'Refrigeradores', 'product',
     'Refrigeradores y equipos de frío',
     (SELECT id FROM product_category WHERE name = 'Refrigeracion' ORDER BY id LIMIT 1),
     NULL),
    ('notebooks', 'Notebooks', 'product',
     'Notebooks y computadores portátiles',
     (SELECT id FROM product_category WHERE name = 'Computadores' ORDER BY id LIMIT 1),
     NULL),
    ('televisores', 'Televisores', 'product',
     'Televisores y pantallas',
     (SELECT id FROM product_category WHERE name = 'Televisores' ORDER BY id LIMIT 1),
     NULL),
    ('internet-fibra-optica', 'Internet Fibra Óptica', 'service',
     'Planes de internet hogar por fibra óptica',
     NULL,
     (SELECT id FROM service_category WHERE name = 'Internet Fibra Óptica' ORDER BY id LIMIT 1))
ON CONFLICT (slug) DO NOTHING;

INSERT INTO category_attributes
    (category_id, key, label, data_type, unit, required, comparable, filterable,
     min_value, max_value, max_length, allowed_values, display_order)
SELECT category.id, attribute.key, attribute.label, attribute.data_type, attribute.unit,
       attribute.required, attribute.comparable, attribute.filterable,
       attribute.min_value, attribute.max_value, attribute.max_length,
       attribute.allowed_values, attribute.display_order
FROM (VALUES
    -- Refrigeradores
    ('refrigeradores', 'capacidad_litros', 'Capacidad', 'integer', 'L'::VARCHAR,
     TRUE, TRUE, TRUE, 50::NUMERIC, 1500::NUMERIC, NULL::INTEGER, NULL::TEXT[], 1),
    ('refrigeradores', 'eficiencia_energetica', 'Eficiencia energética', 'enum', NULL,
     FALSE, TRUE, TRUE, NULL, NULL, NULL, ARRAY['A+++', 'A++', 'A+', 'A', 'B', 'C', 'D'], 2),
    ('refrigeradores', 'no_frost', 'No Frost', 'boolean', NULL,
     FALSE, TRUE, TRUE, NULL, NULL, NULL, NULL, 3),
    ('refrigeradores', 'peso_kg', 'Peso', 'decimal', 'kg',
     FALSE, TRUE, FALSE, 0, 300, NULL, NULL, 4),
    ('refrigeradores', 'color', 'Color', 'string', NULL,
     FALSE, FALSE, FALSE, NULL, NULL, 50, NULL, 5),

    -- Notebooks
    ('notebooks', 'memoria_ram_gb', 'Memoria RAM', 'integer', 'GB',
     TRUE, TRUE, TRUE, 1, 256, NULL, NULL, 1),
    ('notebooks', 'almacenamiento_gb', 'Almacenamiento', 'integer', 'GB',
     TRUE, TRUE, TRUE, 32, 16384, NULL, NULL, 2),
    ('notebooks', 'pantalla_pulgadas', 'Pantalla', 'decimal', 'in',
     TRUE, TRUE, TRUE, 10, 21, NULL, NULL, 3),
    ('notebooks', 'procesador', 'Procesador', 'string', NULL,
     FALSE, TRUE, FALSE, NULL, NULL, 100, NULL, 4),
    ('notebooks', 'sistema_operativo', 'Sistema operativo', 'enum', NULL,
     FALSE, TRUE, TRUE, NULL, NULL, NULL,
     ARRAY['Windows', 'macOS', 'Linux', 'ChromeOS', 'Sin sistema operativo'], 5),
    ('notebooks', 'teclado_retroiluminado', 'Teclado retroiluminado', 'boolean', NULL,
     FALSE, TRUE, FALSE, NULL, NULL, NULL, NULL, 6),

    -- Televisores
    ('televisores', 'pantalla_pulgadas', 'Pantalla', 'decimal', 'in',
     TRUE, TRUE, TRUE, 19, 120, NULL, NULL, 1),
    ('televisores', 'resolucion', 'Resolución', 'enum', NULL,
     TRUE, TRUE, TRUE, NULL, NULL, NULL, ARRAY['HD', 'Full HD', '4K', '8K'], 2),
    ('televisores', 'tipo_panel', 'Tipo de panel', 'enum', NULL,
     FALSE, TRUE, TRUE, NULL, NULL, NULL, ARRAY['LED', 'QLED', 'OLED', 'Mini LED'], 3),
    ('televisores', 'smart_tv', 'Smart TV', 'boolean', NULL,
     FALSE, TRUE, TRUE, NULL, NULL, NULL, NULL, 4),
    ('televisores', 'frecuencia_hz', 'Frecuencia de actualización', 'integer', 'Hz',
     FALSE, TRUE, FALSE, 50, 240, NULL, NULL, 5),

    -- Internet Fibra Óptica (servicio)
    ('internet-fibra-optica', 'velocidad_bajada_mbps', 'Velocidad de bajada', 'integer', 'Mbps',
     TRUE, TRUE, TRUE, 1, 10000, NULL, NULL, 1),
    ('internet-fibra-optica', 'velocidad_subida_mbps', 'Velocidad de subida', 'integer', 'Mbps',
     TRUE, TRUE, TRUE, 1, 10000, NULL, NULL, 2),
    ('internet-fibra-optica', 'tipo_contrato', 'Tipo de contrato', 'enum', NULL,
     FALSE, TRUE, TRUE, NULL, NULL, NULL, ARRAY['Sin permanencia', '12 meses', '24 meses'], 3),
    ('internet-fibra-optica', 'ip_fija', 'IP fija', 'boolean', NULL,
     FALSE, TRUE, TRUE, NULL, NULL, NULL, NULL, 4),
    ('internet-fibra-optica', 'router_incluido', 'Router incluido', 'string', NULL,
     FALSE, TRUE, FALSE, NULL, NULL, 60, NULL, 5)
) AS attribute(category_slug, key, label, data_type, unit, required, comparable, filterable,
               min_value, max_value, max_length, allowed_values, display_order)
JOIN categories category ON category.slug = attribute.category_slug
ON CONFLICT (category_id, key) DO NOTHING;
