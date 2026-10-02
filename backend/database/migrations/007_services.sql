-- =====================================================================
-- SOLOServis - Catalogo adicional de servicios
-- Ejecutar despues de 001_schema.sql, 002_views.sql y 003_seed.sql.
-- =====================================================================

BEGIN;

-- CATEGORIAS RAIZ
INSERT INTO service_category (service_type_id, parent_category_id, name, description)
SELECT internet_category.service_type_id, NULL, categories.name, categories.description
FROM (VALUES
    ('Seguros', 'Planes de proteccion para personas y hogares'),
  ('Técnicos', 'Servicios de asistencia y soporte técnico'),
  ('Educación', 'Cursos y programas de aprendizaje en línea')
) AS categories(name, description)
CROSS JOIN (
    SELECT service_type_id
    FROM service_category
    WHERE parent_category_id IS NULL
      AND name = 'Internet y Telefonía'
    ORDER BY id
    LIMIT 1
) AS internet_category
WHERE NOT EXISTS (
    SELECT 1
    FROM service_category AS existing
    WHERE existing.parent_category_id IS NULL
      AND existing.name = categories.name
);

-- SUBCATEGORIAS
INSERT INTO service_category (service_type_id, parent_category_id, name, description)
SELECT parent.service_type_id, parent.id, categories.name, categories.description
FROM (VALUES
  ('Internet y Telefonía', 'Internet Móvil', 'Planes de conectividad móvil'),
    ('Seguros', 'Salud Complementaria', 'Coberturas complementarias de salud'),
  ('Técnicos', 'Asistencia Técnica', 'Soporte técnico para el hogar'),
  ('Educación', 'Cursos de Inglés', 'Programas de inglés en línea')
) AS categories(parent_name, name, description)
JOIN service_category AS parent
  ON parent.name = categories.parent_name
 AND parent.parent_category_id IS NULL
WHERE NOT EXISTS (
    SELECT 1
    FROM service_category AS existing
    WHERE existing.parent_category_id = parent.id
      AND existing.name = categories.name
);

-- ESPECIFICACIONES DE CATEGORIA
INSERT INTO service_category_specification (
    category_id, name, data_type, unit, required, comparable, display_order
)
SELECT category.id, specifications.name, specifications.data_type,
       specifications.unit, TRUE, TRUE, specifications.display_order
FROM (VALUES
  ('Internet y Telefonía', 'Internet Móvil', 'Datos incluidos', 'number', 'GB', 1),
  ('Internet y Telefonía', 'Internet Móvil', 'Tecnología', 'string', NULL, 2),
    ('Seguros', 'Salud Complementaria', 'Cobertura anual', 'number', 'CLP', 1),
    ('Seguros', 'Salud Complementaria', 'Telemedicina', 'string', NULL, 2),
  ('Técnicos', 'Asistencia Técnica', 'Atenciones mensuales', 'number', NULL, 1),
  ('Técnicos', 'Asistencia Técnica', 'Modalidad', 'string', NULL, 2),
  ('Educación', 'Cursos de Inglés', 'Clases mensuales', 'number', NULL, 1),
  ('Educación', 'Cursos de Inglés', 'Modalidad', 'string', NULL, 2)
) AS specifications(parent_name, category_name, name, data_type, unit, display_order)
JOIN service_category AS parent
  ON parent.name = specifications.parent_name
 AND parent.parent_category_id IS NULL
JOIN service_category AS category
  ON category.parent_category_id = parent.id
 AND category.name = specifications.category_name
WHERE NOT EXISTS (
    SELECT 1
    FROM service_category_specification AS existing
    WHERE existing.category_id = category.id
      AND existing.name = specifications.name
);

-- SERVICIOS
INSERT INTO service (category_id, name, description, image_url)
SELECT category.id, items.name, items.description, items.image_url
FROM (VALUES
  ('Internet y Telefonía', 'Internet Móvil', 'Plan Móvil 100 GB 5G',
     'Plan mensual con 100 GB de datos, acceso 5G y cobertura nacional.',
     'https://images.unsplash.com/photo-1511707171634-5f897ff02aa9?auto=format&fit=crop&w=1200&q=80'),
    ('Seguros', 'Salud Complementaria', 'Seguro Complementario de Salud',
     'Cobertura complementaria con reembolsos y atencion de telemedicina.',
     'https://images.unsplash.com/photo-1576091160399-112ba8d25d1d?auto=format&fit=crop&w=1200&q=80'),
    ('Técnicos', 'Asistencia Técnica', 'Plan de Asistencia Técnica Hogar',
     'Soporte mensual remoto y presencial para equipos y conexiones del hogar.',
     'https://images.unsplash.com/photo-1581092921461-eab62e97a780?auto=format&fit=crop&w=1200&q=80'),
    ('Educación', 'Cursos de Inglés', 'Curso de Inglés Online',
     'Programa mensual de inglés con clases en vivo y material bajo demanda.',
     'https://images.unsplash.com/photo-1503676260728-1c00da094a0b?auto=format&fit=crop&w=1200&q=80')
) AS items(parent_name, category_name, name, description, image_url)
JOIN service_category AS parent
  ON parent.name = items.parent_name
 AND parent.parent_category_id IS NULL
JOIN service_category AS category
  ON category.parent_category_id = parent.id
 AND category.name = items.category_name
WHERE NOT EXISTS (
    SELECT 1
    FROM service AS existing
    WHERE existing.category_id = category.id
      AND existing.name = items.name
);

-- ESPECIFICACIONES DE LOS SERVICIOS
INSERT INTO service_specification_value (service_id, specification_id, value)
SELECT service.id, specification.id, values.value
FROM (VALUES
  ('Internet y Telefonía', 'Internet Móvil', 'Plan Móvil 100 GB 5G', 'Datos incluidos', '100'),
  ('Internet y Telefonía', 'Internet Móvil', 'Plan Móvil 100 GB 5G', 'Tecnología', '5G'),
    ('Seguros', 'Salud Complementaria', 'Seguro Complementario de Salud', 'Cobertura anual', '5000000'),
    ('Seguros', 'Salud Complementaria', 'Seguro Complementario de Salud', 'Telemedicina', 'Incluida'),
    ('Técnicos', 'Asistencia Técnica', 'Plan de Asistencia Técnica Hogar', 'Atenciones mensuales', '2'),
    ('Técnicos', 'Asistencia Técnica', 'Plan de Asistencia Técnica Hogar', 'Modalidad', 'Remota y presencial'),
    ('Educación', 'Cursos de Inglés', 'Curso de Inglés Online', 'Clases mensuales', '8'),
    ('Educación', 'Cursos de Inglés', 'Curso de Inglés Online', 'Modalidad', 'En vivo y grabadas')
) AS values(parent_name, category_name, service_name, specification_name, value)
JOIN service_category AS parent
  ON parent.name = values.parent_name
 AND parent.parent_category_id IS NULL
JOIN service_category AS category
  ON category.parent_category_id = parent.id
 AND category.name = values.category_name
JOIN service
  ON service.category_id = category.id
 AND service.name = values.service_name
JOIN service_category_specification AS specification
  ON specification.category_id = category.id
 AND specification.name = values.specification_name
ON CONFLICT (service_id, specification_id) DO UPDATE
SET value = EXCLUDED.value;

-- PROVEEDORES
INSERT INTO provider (name, website_url, rating, reputation, general_conditions)
SELECT providers.name, providers.website_url, providers.rating,
       providers.reputation, providers.general_conditions
FROM (VALUES
    ('Claro', 'https://www.clarochile.cl/', 4.10, 'Buena', 'Planes sujetos a cobertura y condiciones comerciales.'),
    ('SURA', 'https://www.sura.cl/', 4.30, 'Muy buena', 'Coberturas y reembolsos segun el plan contratado.'),
    ('Consorcio', 'https://www.consorcio.cl/', 4.20, 'Muy buena', 'Coberturas sujetas a las condiciones de la poliza.'),
    ('Asistencia Hogar', NULL, 4.00, 'Buena', 'Atenciones sujetas a disponibilidad de cobertura.'),
    ('Open English', 'https://www.openenglish.com/', 4.20, 'Muy buena', 'Suscripcion con acceso digital segun el plan contratado.'),
    ('Poliglota', 'https://www.poliglota.org/', 4.40, 'Muy buena', 'Clases sujetas a disponibilidad de horarios.')
) AS providers(name, website_url, rating, reputation, general_conditions)
WHERE NOT EXISTS (
    SELECT 1
    FROM provider AS existing
    WHERE existing.name = providers.name
);

-- OFERTAS MENSUALES
INSERT INTO service_offer (
    service_id, provider_id, price, currency, billing_period,
    installation_cost, contract_period, available, coverage_summary,
    additional_costs_summary, service_url
)
SELECT service.id, provider.id, offers.price, 'CLP', 'monthly',
       0, offers.contract_period, TRUE, offers.coverage_summary,
       offers.additional_costs_summary, offers.service_url
FROM (VALUES
  ('Internet y Telefonía', 'Internet Móvil', 'Plan Móvil 100 GB 5G', 'Entel', 14990, 'Sin permanencia', 'Cobertura nacional sujeta a señal.', 'Puede aplicar cargo por roaming.', 'https://www.entel.cl/'),
  ('Internet y Telefonía', 'Internet Móvil', 'Plan Móvil 100 GB 5G', 'Claro', 12990, 'Sin permanencia', 'Cobertura nacional sujeta a señal.', 'Puede aplicar cargo por roaming.', 'https://www.clarochile.cl/'),
    ('Seguros', 'Salud Complementaria', 'Seguro Complementario de Salud', 'SURA', 24990, '12 meses', 'Cobertura nacional.', 'Deducibles segun prestaciones.', 'https://www.sura.cl/'),
    ('Seguros', 'Salud Complementaria', 'Seguro Complementario de Salud', 'Consorcio', 27990, '12 meses', 'Cobertura nacional.', 'Deducibles segun prestaciones.', 'https://www.consorcio.cl/'),
    ('Técnicos', 'Asistencia Técnica', 'Plan de Asistencia Técnica Hogar', 'Asistencia Hogar', 9990, 'Sin permanencia', 'Atención en zonas disponibles.', 'Repuestos no incluidos.', 'https://soloservis.cl/servicios/asistencia-hogar'),
    ('Educación', 'Cursos de Inglés', 'Curso de Inglés Online', 'Open English', 29990, '6 meses', 'Clases en línea.', 'Renovación mensual según el plan.', 'https://www.openenglish.com/'),
    ('Educación', 'Cursos de Inglés', 'Curso de Inglés Online', 'Poliglota', 34990, '6 meses', 'Clases en línea para Chile.', 'Horarios sujetos a disponibilidad.', 'https://www.poliglota.org/')
) AS offers(parent_name, category_name, service_name, provider_name, price, contract_period, coverage_summary, additional_costs_summary, service_url)
JOIN service_category AS parent
  ON parent.name = offers.parent_name
 AND parent.parent_category_id IS NULL
JOIN service_category AS category
  ON category.parent_category_id = parent.id
 AND category.name = offers.category_name
JOIN service
  ON service.category_id = category.id
 AND service.name = offers.service_name
JOIN provider
  ON provider.name = offers.provider_name
ON CONFLICT (service_id, provider_id) DO UPDATE
SET price = EXCLUDED.price,
    currency = EXCLUDED.currency,
    billing_period = EXCLUDED.billing_period,
    installation_cost = EXCLUDED.installation_cost,
    contract_period = EXCLUDED.contract_period,
    available = EXCLUDED.available,
    coverage_summary = EXCLUDED.coverage_summary,
    additional_costs_summary = EXCLUDED.additional_costs_summary,
    service_url = EXCLUDED.service_url,
    last_updated = NOW();

-- HISTORIAL DE PRECIOS
INSERT INTO service_price_history (service_offer_id, price, is_promotional, recorded_at)
SELECT offer.id, history.price, history.is_promotional,
       NOW() - (history.days_ago || ' days')::INTERVAL
FROM (VALUES
  ('Internet y Telefonía', 'Internet Móvil', 'Plan Móvil 100 GB 5G', 'Entel', 17990, FALSE, 30),
  ('Internet y Telefonía', 'Internet Móvil', 'Plan Móvil 100 GB 5G', 'Entel', 14990, TRUE, 3),
  ('Internet y Telefonía', 'Internet Móvil', 'Plan Móvil 100 GB 5G', 'Claro', 15990, FALSE, 30),
  ('Internet y Telefonía', 'Internet Móvil', 'Plan Móvil 100 GB 5G', 'Claro', 12990, TRUE, 3),
    ('Seguros', 'Salud Complementaria', 'Seguro Complementario de Salud', 'SURA', 29990, FALSE, 30),
    ('Seguros', 'Salud Complementaria', 'Seguro Complementario de Salud', 'SURA', 24990, TRUE, 3),
    ('Seguros', 'Salud Complementaria', 'Seguro Complementario de Salud', 'Consorcio', 32990, FALSE, 30),
    ('Seguros', 'Salud Complementaria', 'Seguro Complementario de Salud', 'Consorcio', 27990, TRUE, 3),
    ('Técnicos', 'Asistencia Técnica', 'Plan de Asistencia Técnica Hogar', 'Asistencia Hogar', 12990, FALSE, 30),
    ('Técnicos', 'Asistencia Técnica', 'Plan de Asistencia Técnica Hogar', 'Asistencia Hogar', 9990, TRUE, 3),
    ('Educación', 'Cursos de Inglés', 'Curso de Inglés Online', 'Open English', 34990, FALSE, 30),
    ('Educación', 'Cursos de Inglés', 'Curso de Inglés Online', 'Open English', 29990, TRUE, 3),
    ('Educación', 'Cursos de Inglés', 'Curso de Inglés Online', 'Poliglota', 39990, FALSE, 30),
    ('Educación', 'Cursos de Inglés', 'Curso de Inglés Online', 'Poliglota', 34990, TRUE, 3)
) AS history(parent_name, category_name, service_name, provider_name, price, is_promotional, days_ago)
JOIN service_category AS parent
  ON parent.name = history.parent_name
 AND parent.parent_category_id IS NULL
JOIN service_category AS category
  ON category.parent_category_id = parent.id
 AND category.name = history.category_name
JOIN service
  ON service.category_id = category.id
 AND service.name = history.service_name
JOIN provider
  ON provider.name = history.provider_name
JOIN service_offer AS offer
  ON offer.service_id = service.id
 AND offer.provider_id = provider.id
WHERE NOT EXISTS (
    SELECT 1
    FROM service_price_history AS existing
    WHERE existing.service_offer_id = offer.id
      AND existing.price = history.price
      AND existing.is_promotional = history.is_promotional
      AND existing.recorded_at::DATE = CURRENT_DATE - history.days_ago
);

-- RESEÑAS PARA MOSTRAR VALORACIONES EN EL CATALOGO
INSERT INTO service_review (user_id, service_id, rating, title, content, verified)
SELECT account.id, service.id, reviews.rating, reviews.title, reviews.content, TRUE
FROM (VALUES
  ('Internet y Telefonía', 'Internet Móvil', 'Plan Móvil 100 GB 5G', 'jorge.munoz@example.com', 5, 'Buena cobertura', 'El plan funciona bien en la ciudad.'),
    ('Seguros', 'Salud Complementaria', 'Seguro Complementario de Salud', 'camila.fuentes@example.com', 4, 'Proceso sencillo', 'La cobertura y los reembolsos son claros.'),
    ('Técnicos', 'Asistencia Técnica', 'Plan de Asistencia Técnica Hogar', 'jorge.munoz@example.com', 4, 'Buen soporte', 'La asistencia remota fue útil.'),
    ('Educación', 'Cursos de Inglés', 'Curso de Inglés Online', 'camila.fuentes@example.com', 5, 'Clases prácticas', 'Las clases en línea son fáciles de seguir.')
) AS reviews(parent_name, category_name, service_name, email, rating, title, content)
JOIN user_account AS account
  ON account.email = reviews.email
JOIN service_category AS parent
  ON parent.name = reviews.parent_name
 AND parent.parent_category_id IS NULL
JOIN service_category AS category
  ON category.parent_category_id = parent.id
 AND category.name = reviews.category_name
JOIN service
  ON service.category_id = category.id
 AND service.name = reviews.service_name
ON CONFLICT (user_id, service_id) DO UPDATE
SET rating = EXCLUDED.rating,
    title = EXCLUDED.title,
    content = EXCLUDED.content,
    verified = EXCLUDED.verified;

COMMIT;