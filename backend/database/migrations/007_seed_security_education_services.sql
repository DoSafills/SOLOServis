BEGIN;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM service_type WHERE name LIKE 'suscrip%'
    ) THEN
        RAISE EXCEPTION 'El tipo de servicio suscripción no existe';
    END IF;
END
$$;

CREATE TEMP TABLE service_seed (
    category_name TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    provider_name TEXT NOT NULL,
    website_url TEXT NOT NULL,
    price NUMERIC(12, 2) NOT NULL,
    currency CHAR(3) NOT NULL,
    billing_period VARCHAR(30) NOT NULL,
    contract_period TEXT NOT NULL,
    coverage TEXT NOT NULL,
    additional_costs TEXT NOT NULL,
    PRIMARY KEY (category_name, name)
) ON COMMIT DROP;

INSERT INTO service_seed VALUES
    ('VPN', 'NordVPN (plan de 2 años)',
     'VPN para proteger la conexión y la privacidad en redes públicas, teletrabajo y estudio online. Precio mensual equivalente de un plan de dos años pagado por adelantado.',
     'NordVPN', 'https://nordvpn.com', 3.39, 'EUR', 'monthly',
     'Plan de 2 años, pagado por adelantado', 'Mundial; servidores en más de 100 países',
     'Precio referencial de comparadores; verificar renovación e impuestos.'),
    ('VPN', 'Surfshark (plan de 2 años)',
     'VPN económica con conexiones ilimitadas, apta para toda la familia y el estudio online. Precio mensual equivalente de un plan de dos años.',
     'Surfshark', 'https://surfshark.com', 2.19, 'EUR', 'monthly',
     'Plan de 2 años, pagado por adelantado', 'Mundial',
     'Precio referencial; otra fuente indica EUR 2,49 al mes. Verificar precio final e impuestos.'),
    ('VPN', 'ExpressVPN (plan de 2 años)',
     'VPN fácil de usar con protocolo propio Lightway, ideal para principiantes. Precio mensual equivalente de un plan de dos años.',
     'ExpressVPN', 'https://expressvpn.com', 4.87, 'EUR', 'monthly',
     'Plan de 2 años, pagado por adelantado', 'Mundial; servidores en 105 países',
     'Precio referencial; las promociones y el precio de renovación pueden variar.'),
    ('VPN', 'Proton VPN Plus (plan de 2 años)',
     'VPN de origen suizo enfocada en privacidad; también existe un plan gratuito limitado sin tope de datos. Precio mensual equivalente de un plan de dos años.',
     'Proton', 'https://protonvpn.com', 2.99, 'EUR', 'monthly',
     'Plan de 2 años, pagado por adelantado', 'Mundial; servidores en unos 127 países',
     'Precio referencial; verificar precio final, renovación e impuestos.'),
    ('Antivirus', 'Bitdefender Total Security',
     'Suite de seguridad para hasta 5 dispositivos con VPN incluida; evaluada en pruebas independientes según la ficha suministrada.',
     'Bitdefender', 'https://bitdefender.com', 49.99, 'USD', 'yearly',
     'Suscripción anual', 'Windows, macOS, Android, iOS',
     'Precio anual referencial; verificar precio de renovación e impuestos.'),
    ('Antivirus', 'Bitdefender Antivirus Plus',
     'Protección antivirus esencial a precio accesible.',
     'Bitdefender', 'https://bitdefender.com', 29.99, 'USD', 'yearly',
     'Suscripción anual', 'Windows; versión Multiplatform disponible para otros sistemas',
     'Precio anual referencial; verificar dispositivos cubiertos, VPN, renovación e impuestos.'),
    ('Antivirus', 'Norton 360 Deluxe',
     'Suite todo en uno para 5 dispositivos con VPN, monitoreo de la dark web y control parental.',
     'Norton', 'https://norton.com', 49.99, 'USD', 'yearly',
     'Suscripción anual (precio del primer año)', 'Windows, macOS, Android, iOS',
     'Precio referencial del primer año; precio de lista informado USD 119,99/año. Verificar renovación e impuestos.'),
    ('Antivirus', 'Malwarebytes Premium (Standard)',
     'Protección anti-malware sencilla, útil como complemento o para limpieza. El plan Plus agrega VPN según la ficha suministrada.',
     'Malwarebytes', 'https://malwarebytes.com', 44.99, 'USD', 'yearly',
     'Suscripción anual', 'Windows, macOS, Android, iOS',
     'Precio anual referencial; verificar precio de renovación e impuestos.'),
    ('Antivirus', 'McAfee+ Premium Individual',
     'Protección para dispositivos ilimitados de un hogar, con herramientas de identidad.',
     'McAfee', 'https://mcafee.com', 49.99, 'USD', 'yearly',
     'Suscripción anual (precio del primer año)', 'Windows, macOS, Android, iOS',
     'Precio referencial del primer año; la renovación puede ser más alta. Verificar impuestos.'),
    ('Educación Online', 'Open English (plan inicial)',
     'Plataforma de inglés online con clases en vivo con instructores y contenido bajo demanda.',
     'Open English', 'https://openenglish.com', 79, 'USD', 'monthly',
     'Duración del contrato por verificar', 'Online; precio regional para Chile por verificar',
     'Precio mensual referencial; fuentes citadas difieren entre USD 79 y USD 99 (rango informado hasta USD 189). Verificar precio e impuestos.'),
    ('Educación Online', 'Coursera Plus (mensual)',
     'Acceso a un amplio catálogo de cursos, especializaciones y certificados de universidades y empresas.',
     'Coursera', 'https://coursera.org', 59, 'USD', 'monthly',
     'Sin permanencia (existe plan anual)', 'Online, mundial',
     'Precio mensual referencial; cursos individuales y grados tienen costos adicionales. Plan anual informado: USD 399.'),
    ('Educación Online', 'Udemy (cursos individuales)',
     'Marketplace de cursos prácticos a tu ritmo, con miles de temas técnicos y de programación.',
     'Udemy', 'https://udemy.com', 29.99, 'USD', 'one_time',
     'Sin permanencia (pago por curso)', 'Online, mundial',
     'Precio referencial de entrada; el valor depende del curso y puede cambiar con las ofertas.'),
    ('Educación Online', 'Babbel (suscripción individual)',
     'Cursos de idiomas con enfoque conversacional y lecciones cortas, en 14 idiomas.',
     'Babbel', 'https://babbel.com', 5.99, 'USD', 'monthly',
     'Duración del contrato por verificar', 'Online y aplicación móvil',
     'Precio mensual referencial; otra fuente indica USD 8,95 al mes. Verificar plan, renovación e impuestos.'),
    ('Educación Online', 'Brilliant (mensual)',
     'Aprendizaje interactivo de matemáticas, ciencias y computación, con más de 60 cursos.',
     'Brilliant', 'https://brilliant.org', 24.99, 'USD', 'monthly',
     'Sin permanencia (plan anual más barato)', 'Online y aplicación móvil',
     'Precio mensual referencial; otra fuente indica USD 27,99. Se informó plan anual equivalente a USD 13,49/mes. Verificar precio e impuestos.'),
    ('Educación Online', 'LinkedIn Learning (mensual)',
     'Cursos de desarrollo profesional, tecnología y negocios, con más de 16.000 cursos.',
     'LinkedIn', 'https://linkedin.com/learning', 29.99, 'USD', 'monthly',
     'Sin permanencia', 'Online y aplicación móvil',
     'Precio mensual referencial; verificar precio e impuestos.'),
    ('Educación Online', 'Skillshare (mensual)',
     'Cursos de habilidades creativas y digitales, con más de 30.000 clases.',
     'Skillshare', 'https://skillshare.com', 14, 'USD', 'monthly',
     'Sin permanencia', 'Online y aplicación móvil',
     'Precio mensual referencial; verificar precio e impuestos.');

INSERT INTO service_category (service_type_id, parent_category_id, name, description)
SELECT subscription.id, NULL, requested.name, requested.description
FROM (
    VALUES
        ('Seguridad Digital', 'Herramientas digitales de protección y privacidad'),
        ('Educación', 'Plataformas y cursos de aprendizaje en línea')
) AS requested(name, description)
CROSS JOIN LATERAL (
    SELECT id FROM service_type WHERE name LIKE 'suscrip%' ORDER BY id LIMIT 1
) subscription
WHERE NOT EXISTS (
    SELECT 1
    FROM service_category existing
    WHERE existing.parent_category_id IS NULL
      AND existing.name = requested.name
);

INSERT INTO service_category (service_type_id, parent_category_id, name, description)
SELECT subscription.id, parent.id, requested.name, requested.description
FROM (
    VALUES
        ('Seguridad Digital', 'VPN', 'Redes privadas virtuales para privacidad y conexión segura'),
        ('Seguridad Digital', 'Antivirus', 'Software antivirus y suites de seguridad'),
        ('Educación', 'Educación Online', 'Cursos y plataformas de aprendizaje en línea')
) AS requested(parent_name, name, description)
JOIN service_category parent
    ON parent.name = requested.parent_name
   AND parent.parent_category_id IS NULL
CROSS JOIN LATERAL (
    SELECT id FROM service_type WHERE name LIKE 'suscrip%' ORDER BY id LIMIT 1
) subscription
WHERE NOT EXISTS (
    SELECT 1
    FROM service_category existing
    WHERE existing.parent_category_id = parent.id
      AND existing.name = requested.name
);

INSERT INTO service_category_specification
    (category_id, name, data_type, required, comparable, display_order)
SELECT category.id, requested.name, requested.data_type, FALSE, TRUE,
       requested.display_order
FROM (
    VALUES
        ('VPN', 'Dispositivos simultáneos', 'number', 1),
        ('VPN', 'Países con servidores', 'number', 2),
        ('Antivirus', 'Dispositivos cubiertos', 'string', 1),
        ('Antivirus', 'Incluye VPN', 'string', 2),
        ('Educación Online', 'Tipo', 'string', 1),
        ('Educación Online', 'Clases en vivo', 'string', 2),
        ('Educación Online', 'Certificado', 'string', 3)
) AS requested(category_name, name, data_type, display_order)
JOIN service_category category
    ON category.name = requested.category_name
WHERE NOT EXISTS (
    SELECT 1
    FROM service_category_specification existing
    WHERE existing.category_id = category.id
      AND existing.name = requested.name
);

INSERT INTO provider (name, website_url)
SELECT DISTINCT ON (seed.provider_name) seed.provider_name, seed.website_url
FROM service_seed seed
WHERE NOT EXISTS (
    SELECT 1
    FROM provider existing
    WHERE LOWER(BTRIM(existing.name)) = LOWER(seed.provider_name)
)
ORDER BY seed.provider_name;

INSERT INTO service (category_id, name, description, image_url)
SELECT category.id, seed.name, seed.description, NULL
FROM service_seed seed
JOIN service_category category
    ON category.name = seed.category_name
WHERE NOT EXISTS (
    SELECT 1
    FROM service existing
    WHERE existing.category_id = category.id
      AND existing.name = seed.name
);

INSERT INTO service_offer (
    service_id,
    provider_id,
    price,
    currency,
    billing_period,
    installation_cost,
    contract_period,
    available,
    coverage_summary,
    additional_costs_summary,
    service_url
)
SELECT
    service.id,
    provider.id,
    seed.price,
    seed.currency,
    seed.billing_period,
    0,
    seed.contract_period,
    TRUE,
    seed.coverage,
    seed.additional_costs,
    NULL
FROM service_seed seed
JOIN service_category category
    ON category.name = seed.category_name
JOIN service
    ON service.category_id = category.id
   AND service.name = seed.name
JOIN provider
    ON LOWER(BTRIM(provider.name)) = LOWER(seed.provider_name)
   AND provider.active = TRUE
WHERE NOT EXISTS (
    SELECT 1
    FROM service_offer existing
    WHERE existing.service_id = service.id
      AND existing.provider_id = provider.id
);

CREATE TEMP TABLE service_spec_seed (
    category_name TEXT NOT NULL,
    service_name TEXT NOT NULL,
    specification_name TEXT NOT NULL,
    value TEXT NOT NULL
) ON COMMIT DROP;

INSERT INTO service_spec_seed VALUES
    ('VPN', 'NordVPN (plan de 2 años)', 'Dispositivos simultáneos', '10'),
    ('VPN', 'NordVPN (plan de 2 años)', 'Países con servidores', '118'),
    ('VPN', 'Surfshark (plan de 2 años)', 'Dispositivos simultáneos', 'Ilimitados'),
    ('VPN', 'ExpressVPN (plan de 2 años)', 'Dispositivos simultáneos', '8'),
    ('VPN', 'ExpressVPN (plan de 2 años)', 'Países con servidores', '105'),
    ('VPN', 'Proton VPN Plus (plan de 2 años)', 'Dispositivos simultáneos', '10'),
    ('VPN', 'Proton VPN Plus (plan de 2 años)', 'Países con servidores', '127'),
    ('Antivirus', 'Bitdefender Total Security', 'Dispositivos cubiertos', '5'),
    ('Antivirus', 'Bitdefender Total Security', 'Incluye VPN', 'Sí'),
    ('Antivirus', 'Norton 360 Deluxe', 'Dispositivos cubiertos', '5'),
    ('Antivirus', 'Norton 360 Deluxe', 'Incluye VPN', 'Sí'),
    ('Antivirus', 'Malwarebytes Premium (Standard)', 'Dispositivos cubiertos', '3'),
    ('Antivirus', 'Malwarebytes Premium (Standard)', 'Incluye VPN', 'No (solo en plan Plus)'),
    ('Antivirus', 'McAfee+ Premium Individual', 'Dispositivos cubiertos', 'Ilimitados'),
    ('Educación Online', 'Open English (plan inicial)', 'Tipo', 'Idiomas (inglés)'),
    ('Educación Online', 'Open English (plan inicial)', 'Clases en vivo', 'Sí'),
    ('Educación Online', 'Coursera Plus (mensual)', 'Tipo', 'Cursos online'),
    ('Educación Online', 'Coursera Plus (mensual)', 'Clases en vivo', 'No'),
    ('Educación Online', 'Coursera Plus (mensual)', 'Certificado', 'Sí'),
    ('Educación Online', 'Udemy (cursos individuales)', 'Tipo', 'Cursos online'),
    ('Educación Online', 'Udemy (cursos individuales)', 'Clases en vivo', 'No'),
    ('Educación Online', 'Udemy (cursos individuales)', 'Certificado', 'Certificado de finalización; sin reconocimiento formal'),
    ('Educación Online', 'Babbel (suscripción individual)', 'Tipo', 'Idiomas'),
    ('Educación Online', 'Brilliant (mensual)', 'Tipo', 'STEM'),
    ('Educación Online', 'Brilliant (mensual)', 'Clases en vivo', 'No'),
    ('Educación Online', 'LinkedIn Learning (mensual)', 'Tipo', 'Desarrollo profesional'),
    ('Educación Online', 'LinkedIn Learning (mensual)', 'Clases en vivo', 'No'),
    ('Educación Online', 'Skillshare (mensual)', 'Tipo', 'Habilidades creativas'),
    ('Educación Online', 'Skillshare (mensual)', 'Clases en vivo', 'No');

INSERT INTO service_specification_value (service_id, specification_id, value)
SELECT service.id, specification.id, service_spec_seed.value
FROM service_spec_seed
JOIN service_category category
    ON category.name = service_spec_seed.category_name
JOIN service
    ON service.category_id = category.id
   AND service.name = service_spec_seed.service_name
JOIN service_category_specification specification
    ON specification.category_id = category.id
   AND specification.name = service_spec_seed.specification_name
ON CONFLICT (service_id, specification_id) DO NOTHING;

COMMIT;
