-- Consolidated, idempotent seed for service catalog categories and offers.
-- Categories expose at most three active seed services each.
BEGIN;


DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM service_category category
        JOIN service_category parent
            ON parent.id = category.parent_category_id
        WHERE category.name = 'Internet Fibra Óptica'
          AND parent.name = 'Internet y Telefonía'
          AND category.active = TRUE
    ) THEN
        RAISE EXCEPTION 'La categoría activa Internet Fibra Óptica no existe';
    END IF;
END
$$;

INSERT INTO provider (name, website_url)
SELECT requested.name, requested.website_url
FROM (VALUES
    ('Movistar', 'https://movistar.cl'),
    ('Entel', 'https://entel.cl'),
    ('WOM', 'https://wom.cl')
) AS requested(name, website_url)
WHERE NOT EXISTS (
    SELECT 1
    FROM provider existing
    WHERE LOWER(BTRIM(existing.name)) = LOWER(requested.name)
);

WITH service_data (
    name,
    description,
    download_mbps,
    upload_mbps,
    upload_estimated
) AS (
    VALUES
        ('Internet Fibra Óptica 600 Megas',
         'Plan hogar de fibra óptica de hasta 600 Mbps, apto para estudio online, teletrabajo y hogar inteligente.',
         600, 600, FALSE),
        ('Fibra 400 Mbps Entel',
         'Plan hogar de fibra óptica de 400 Mbps para teletrabajo, estudio y videollamadas simultáneas.',
         400, 400, TRUE),
        ('Fibra WOM 600',
         'Plan hogar de fibra óptica simétrica de 600 Mbps para hogares con alto uso de videollamadas y descargas.',
         600, 600, FALSE)
)
INSERT INTO service (category_id, name, description, image_url)
SELECT
    category.id,
    service_data.name,
    service_data.description
        || ' Precio mensual referencial; vigencia no confirmada.'
        || CASE
            WHEN service_data.upload_estimated
                THEN ' Velocidad de subida estimada como simétrica; no verificada con el proveedor.'
            ELSE ''
        END,
    NULL
FROM service_data
JOIN service_category category
    ON category.name = 'Internet Fibra Óptica'
JOIN service_category parent
    ON parent.id = category.parent_category_id
   AND parent.name = 'Internet y Telefonía'
WHERE category.active = TRUE
  AND parent.active = TRUE
  AND NOT EXISTS (
      SELECT 1
      FROM service existing
      WHERE existing.category_id = category.id
        AND existing.name = service_data.name
  );

WITH service_data (name, download_mbps, upload_mbps, upload_estimated) AS (
    VALUES
        ('Internet Fibra Óptica 600 Megas', 600, 600, FALSE),
        ('Fibra 400 Mbps Entel', 400, 400, TRUE),
        ('Fibra WOM 600', 600, 600, FALSE)
)
INSERT INTO service_specification_value (service_id, specification_id, value)
SELECT
    service.id,
    specification.id,
    CASE
        WHEN specification.name = 'Velocidad de bajada'
            THEN service_data.download_mbps::TEXT
        WHEN service_data.upload_estimated
            THEN service_data.upload_mbps::TEXT || ' (estimado; no verificado)'
        ELSE service_data.upload_mbps::TEXT
    END
FROM service_data
JOIN service_category category
    ON category.name = 'Internet Fibra Óptica'
JOIN service_category parent
    ON parent.id = category.parent_category_id
   AND parent.name = 'Internet y Telefonía'
JOIN service
    ON service.category_id = category.id
   AND service.name = service_data.name
JOIN service_category_specification specification
    ON specification.category_id = category.id
   AND specification.name IN ('Velocidad de bajada', 'Velocidad de subida')
WHERE category.active = TRUE
  AND parent.active = TRUE
ON CONFLICT (service_id, specification_id) DO NOTHING;

WITH offer_data (
    service_name,
    provider_name,
    price,
    coverage,
    additional_costs
) AS (
    VALUES
        ('Internet Fibra Óptica 600 Megas', 'Movistar', 22990::NUMERIC,
         'Sujeta a factibilidad técnica validada en terreno.',
         'Precio referencial; instalación, contrato y costos adicionales por verificar.'),
        ('Fibra 400 Mbps Entel', 'Entel', 19990::NUMERIC,
         'Cobertura por verificar en el sitio oficial del proveedor.',
         'Precio referencial; instalación, contrato y costos adicionales por verificar.'),
        ('Fibra WOM 600', 'WOM', 18990::NUMERIC,
         'Cobertura actual por verificar; la fuente citada corresponde al lanzamiento del servicio.',
         'Precio referencial de lanzamiento 2021; vigencia, instalación, contrato y costos adicionales por verificar.')
)
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
    offer_data.price,
    'CLP',
    'monthly',
    NULL,
    NULL,
    TRUE,
    offer_data.coverage,
    offer_data.additional_costs,
    NULL
FROM offer_data
JOIN service_category category
    ON category.name = 'Internet Fibra Óptica'
JOIN service_category parent
    ON parent.id = category.parent_category_id
   AND parent.name = 'Internet y Telefonía'
JOIN service
    ON service.category_id = category.id
   AND service.name = offer_data.service_name
JOIN LATERAL (
    SELECT existing.id
    FROM provider existing
    WHERE LOWER(BTRIM(existing.name)) = LOWER(offer_data.provider_name)
      AND existing.active = TRUE
    ORDER BY existing.id
    LIMIT 1
) provider ON TRUE
WHERE category.active = TRUE
  AND parent.active = TRUE
ON CONFLICT (service_id, provider_id) DO NOTHING;

UPDATE service
SET active = name IN (
    'Internet Fibra Óptica 600 Megas',
    'Fibra 400 Mbps Entel',
    'Fibra WOM 600'
)
WHERE category_id = (
    SELECT category.id
    FROM service_category category
    JOIN service_category parent ON parent.id = category.parent_category_id
    WHERE category.name = 'Internet Fibra Óptica'
      AND parent.name = 'Internet y Telefonía'
      AND parent.parent_category_id IS NULL
    ORDER BY category.id
    LIMIT 1
);




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

UPDATE service
SET active = CASE
    WHEN category.name = 'VPN' THEN service.name IN (
        'NordVPN (plan de 2 años)',
        'Surfshark (plan de 2 años)',
        'ExpressVPN (plan de 2 años)'
    )
    WHEN category.name = 'Antivirus' THEN service.name IN (
        'Bitdefender Total Security',
        'Norton 360 Deluxe',
        'McAfee+ Premium Individual'
    )
    WHEN category.name = 'Educación Online' THEN service.name IN (
        'Open English (plan inicial)',
        'Coursera Plus (mensual)',
        'Udemy (cursos individuales)'
    )
END
FROM service_category category
JOIN service_category parent ON parent.id = category.parent_category_id
WHERE service.category_id = category.id
  AND parent.name IN ('Seguridad Digital', 'Educación')
  AND category.name IN ('VPN', 'Antivirus', 'Educación Online');




DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM service_type WHERE name LIKE 'suscrip%') THEN
        RAISE EXCEPTION 'El tipo de servicio suscripción no existe';
    END IF;
END
$$;

INSERT INTO service_category (service_type_id, parent_category_id, name, description)
SELECT type.id, NULL, 'Técnicos', 'Servicios técnicos digitales y de infraestructura'
FROM service_type type
WHERE type.name LIKE 'suscrip%'
  AND NOT EXISTS (
      SELECT 1 FROM service_category existing
      WHERE existing.parent_category_id IS NULL AND existing.name = 'Técnicos'
  );

CREATE TEMP TABLE technical_service_seed (
    name TEXT PRIMARY KEY,
    description TEXT NOT NULL,
    provider_name TEXT NOT NULL,
    website_url TEXT NOT NULL,
    price NUMERIC(12, 2) NOT NULL,
    billing_period VARCHAR(30) NOT NULL,
    contract_period TEXT NOT NULL,
    coverage TEXT NOT NULL,
    additional_costs TEXT NOT NULL,
    specifications JSONB NOT NULL
) ON COMMIT DROP;

INSERT INTO technical_service_seed VALUES
    ('Hostinger VPS KVM 2 (8 GB RAM)',
     'Servidor VPS con almacenamiento NVMe para sitios web, aplicaciones y entornos de desarrollo.',
     'Hostinger', 'https://hostinger.com', 8630, 'monthly',
     'Renovación a 14.390 CLP/mes por verificar.',
     'Centros de datos en varios países',
     'Precio promocional referencial de Findstack (sep. 2026). Renovación e impuestos por verificar.',
     '{"Memoria RAM":"8 GB","CPU":"Por verificar","Almacenamiento":"Por verificar"}'),
    ('Hostinger VPS KVM 4 (16 GB RAM)',
     'Servidor VPS de 16 GB de RAM para bases de datos, automatización y proyectos más exigentes.',
     'Hostinger', 'https://hostinger.com', 12470, 'monthly',
     'Renovación por verificar.',
     'Centros de datos en varios países',
     'Precio promocional referencial de Findstack (sep. 2026). Renovación e impuestos por verificar.',
     '{"Memoria RAM":"16 GB","CPU":"Por verificar","Almacenamiento":"Por verificar"}'),
    ('Hetzner Cloud (plan de entrada)',
     'Servidor en la nube de bajo costo con buena relación precio-rendimiento, orientado a usuarios con conocimientos técnicos.',
     'Hetzner', 'https://hetzner.com', 4940, 'monthly',
     'Por uso con tope mensual; sin permanencia.',
     'Centros de datos en Europa y otras regiones',
     'Precio referencial anterior al ajuste del 15-06-2026; verificar el valor actual.',
     '{"Memoria RAM":"Por verificar","CPU":"Por verificar","Almacenamiento":"Por verificar"}'),
    ('DigitalOcean Droplet básico (0,5 GB RAM)',
     'Servidor virtual simple, con buena documentación, pensado para desarrolladores.',
     'DigitalOcean', 'https://digitalocean.com', 3840, 'monthly',
     'Sin permanencia.',
     'Centros de datos en varios países',
     'Precio referencial; verificar. Crédito de bienvenida informado: 4.800 CLP (5 USD), por confirmar.',
     '{"Memoria RAM":"0,5 GB","CPU":"1 vCPU","Almacenamiento":"Por verificar"}'),
    ('DigitalOcean Droplet 4 GB RAM',
     'Servidor virtual de 4 GB de RAM para aplicaciones web y bases de datos de tamaño medio.',
     'DigitalOcean', 'https://digitalocean.com', 23040, 'monthly',
     'Sin permanencia.',
     'Centros de datos en varios países',
     'Precio referencial; impuestos y cargos por verificar.',
     '{"Memoria RAM":"4 GB","CPU":"Por verificar","Almacenamiento":"Por verificar"}'),
    ('Hostinger Hosting Compartido (plazo de 48 meses)',
     'Hosting compartido para sitios web, con precio efectivo bajo en contratos largos.',
     'Hostinger', 'https://hostinger.com', 2870, 'monthly',
     'Plazo 48 meses; renovación por verificar.',
     'Centros de datos en varios países',
     'Precio referencial promocional de Findstack (sep. 2026); verificar renovación e impuestos.',
     '{"Tipo":"Hosting compartido","Sitios incluidos":"Por verificar","Almacenamiento":"Por verificar"}'),
    ('Hetzner Cloud CPX21',
     'Servidor en la nube con CPU compartida para un sitio WordPress individual o proyectos técnicos.',
     'Hetzner', 'https://hetzner.com', 9260, 'monthly',
     'Sin permanencia.',
     'Centros de datos en Europa y otras regiones',
     'Precio referencial anterior al ajuste del 15-06-2026; verificar el valor actual.',
     '{"Memoria RAM":"Por verificar","CPU":"Por verificar","Almacenamiento":"Por verificar"}'),
    ('Google One 100 GB',
     'Almacenamiento en la nube compartido entre Gmail, Drive y Fotos; permite compartir con la familia.',
     'Google', 'https://one.google.com', 1910, 'monthly',
     'Sin permanencia.',
     'Online; precio regional por verificar para Chile',
     'Precio mensual referencial de Usecarly (2026); valor regional e impuestos por verificar.',
     '{"Almacenamiento":"100 GB","Cifrado de extremo a extremo":"No"}'),
    ('Google One 2 TB',
     'Almacenamiento de 2 TB con opción de compartir con la familia y herramientas de IA incluidas.',
     'Google', 'https://one.google.com', 95990, 'yearly',
     'Suscripción anual.',
     'Online; precio regional por verificar para Chile',
     'Precio anual referencial de LowerMySubs (jul. 2026) y Peony (mar. 2026); valor actual e impuestos por verificar.',
     '{"Almacenamiento":"2 TB","Cifrado de extremo a extremo":"No"}'),
    ('Proton Drive 200 GB',
     'Almacenamiento en la nube con cifrado de extremo a extremo y sede en Suiza.',
     'Proton', 'https://proton.me/drive', 3830, 'monthly',
     'Plan anual con precio mensual equivalente.',
     'Online, mundial',
     'Precio referencial de Dupple y LowerMySubs (jul. 2026); condiciones e impuestos por verificar.',
     '{"Almacenamiento":"200 GB","Cifrado de extremo a extremo":"Sí"}'),
    ('pCloud 2 TB (pago único de por vida)',
     'Almacenamiento en la nube de 2 TB con pago único, sin cuotas recurrentes.',
     'pCloud', 'https://pcloud.com', 383040, 'one_time',
     'Pago único; plan de por vida.',
     'Online, mundial',
     'Precio referencial de LowerMySubs (jul. 2026). pCloud Crypto opcional: 144.000 CLP adicionales; verificar.',
     '{"Almacenamiento":"2 TB","Cifrado de extremo a extremo":"Opcional (pCloud Crypto, de pago)"}'),
    ('pCloud 500 GB (pago único de por vida)',
     'Almacenamiento en la nube de 500 GB con pago único, sin suscripción.',
     'pCloud', 'https://pcloud.com', 191040, 'one_time',
     'Pago único; plan de por vida.',
     'Online, mundial',
     'Precio referencial de LowerMySubs (jul. 2026). pCloud Crypto opcional: 144.000 CLP adicionales; verificar.',
     '{"Almacenamiento":"500 GB","Cifrado de extremo a extremo":"Opcional (pCloud Crypto, de pago)"}'),
    ('1Password Individual',
     'Gestor de contraseñas con autocompletado, modo viaje y aplicaciones para escritorio y móvil.',
     '1Password', 'https://1password.com', 45960, 'yearly',
     'Suscripción anual.',
     'Online; aplicaciones de escritorio y móvil',
     'Precio anual referencial de Spliiit y Tech-Insider; dispositivos e impuestos por verificar.',
     '{"Usuarios":"1","Dispositivos":"Por verificar"}'),
    ('Bitwarden Premium',
     'Gestor de contraseñas de código abierto, con plan gratuito y opción de autoalojamiento.',
     'Bitwarden', 'https://bitwarden.com', 19010, 'yearly',
     'Suscripción anual.',
     'Online; aplicaciones de escritorio, móvil y extensiones',
     'Precio anual referencial; fuentes discrepan entre 19.010 y 9.600 CLP/año. Verificar precio e impuestos.',
     '{"Usuarios":"1","Dispositivos":"Ilimitados"}'),
    ('Proton Unlimited (plan de 2 años)',
     'Paquete de Proton Mail, Drive, VPN, Pass y Wallet en una sola suscripción.',
     'Proton', 'https://proton.me', 4790, 'monthly',
     '2 años prepagado; monto mensual equivalente.',
     'Online, mundial',
     'Precio mensual equivalente referencial de Tuxxin (2026); precio regular e impuestos por verificar.',
     '{"Usuarios":"1","Incluye":"Mail, Drive, VPN, Pass y Wallet"}'),
    ('Google Workspace Individual',
     'Correo profesional con dominio propio y funciones ampliadas de videoconferencia.',
     'Google', 'https://workspace.google.com', 8750, 'monthly',
     'Sin permanencia.',
     'Online, mundial',
     'Precio referencial basado en una fuente alemana (feb. 2026); precio para Chile e impuestos por verificar.',
     '{"Usuarios":"1","Incluye":"Correo con dominio propio y videoconferencia ampliada"}');

CREATE TEMP TABLE technical_category_seed (
    service_name TEXT PRIMARY KEY,
    category_name TEXT NOT NULL
) ON COMMIT DROP;

INSERT INTO technical_category_seed VALUES
    ('Hostinger VPS KVM 2 (8 GB RAM)', 'Servidores VPS'),
    ('Hostinger VPS KVM 4 (16 GB RAM)', 'Servidores VPS'),
    ('Hetzner Cloud (plan de entrada)', 'Servidores VPS'),
    ('DigitalOcean Droplet básico (0,5 GB RAM)', 'Servidores en la nube'),
    ('DigitalOcean Droplet 4 GB RAM', 'Servidores en la nube'),
    ('Hetzner Cloud CPX21', 'Servidores en la nube'),
    ('Hostinger Hosting Compartido (plazo de 48 meses)', 'Hosting compartido'),
    ('Google One 100 GB', 'Almacenamiento en la nube'),
    ('Google One 2 TB', 'Almacenamiento en la nube'),
    ('Proton Drive 200 GB', 'Almacenamiento en la nube'),
    ('pCloud 2 TB (pago único de por vida)', 'Almacenamiento de por vida'),
    ('pCloud 500 GB (pago único de por vida)', 'Almacenamiento de por vida'),
    ('1Password Individual', 'Seguridad y contraseñas'),
    ('Bitwarden Premium', 'Seguridad y contraseñas'),
    ('Proton Unlimited (plan de 2 años)', 'Seguridad y contraseñas'),
    ('Google Workspace Individual', 'Productividad');

INSERT INTO service_category (service_type_id, parent_category_id, name, description)
SELECT type.id, parent.id, requested.name, requested.description
FROM service_type type
JOIN service_category parent
  ON parent.name = 'Técnicos' AND parent.parent_category_id IS NULL
CROSS JOIN (VALUES
    ('Servidores VPS', 'Planes de servidores privados virtuales'),
    ('Servidores en la nube', 'Instancias de cómputo en la nube'),
    ('Hosting compartido', 'Alojamiento web compartido'),
    ('Almacenamiento en la nube', 'Planes de almacenamiento con cobro recurrente'),
    ('Almacenamiento de por vida', 'Planes de almacenamiento con pago único'),
    ('Seguridad y contraseñas', 'Herramientas de seguridad y gestión de credenciales'),
    ('Productividad', 'Herramientas de productividad y trabajo')
) AS requested(name, description)
WHERE type.name LIKE 'suscrip%'
  AND NOT EXISTS (
      SELECT 1 FROM service_category existing
      WHERE existing.parent_category_id = parent.id
        AND existing.name = requested.name
  );

INSERT INTO service_category_specification
    (category_id, name, data_type, required, comparable, display_order)
SELECT requested.category_id, requested.name, 'string', FALSE, TRUE,
       ROW_NUMBER() OVER (
           PARTITION BY requested.category_id ORDER BY requested.name
       )::INTEGER
FROM (
    SELECT DISTINCT category.id AS category_id, specification.name
    FROM technical_service_seed seed
    JOIN technical_category_seed mapping ON mapping.service_name = seed.name
    JOIN service_category category ON category.name = mapping.category_name
    JOIN service_category parent
      ON parent.id = category.parent_category_id AND parent.name = 'Técnicos'
    CROSS JOIN LATERAL jsonb_object_keys(seed.specifications) AS specification(name)
) requested
WHERE NOT EXISTS (
    SELECT 1 FROM service_category_specification existing
    WHERE existing.category_id = requested.category_id
      AND existing.name = requested.name
);

INSERT INTO provider (name, website_url)
SELECT DISTINCT seed.provider_name, seed.website_url
FROM technical_service_seed seed
WHERE NOT EXISTS (
    SELECT 1 FROM provider existing
    WHERE LOWER(BTRIM(existing.name)) = LOWER(seed.provider_name)
);

UPDATE service
SET category_id = category.id,
    active = TRUE
FROM technical_category_seed mapping
JOIN service_category category ON category.name = mapping.category_name
JOIN service_category parent
  ON parent.id = category.parent_category_id AND parent.name = 'Técnicos'
WHERE service.name = mapping.service_name;

INSERT INTO service (category_id, name, description, image_url)
SELECT category.id, seed.name, seed.description, NULL
FROM technical_service_seed seed
JOIN technical_category_seed mapping ON mapping.service_name = seed.name
JOIN service_category category
  ON category.name = mapping.category_name
JOIN service_category parent
  ON parent.id = category.parent_category_id
 AND parent.name = 'Técnicos'
WHERE NOT EXISTS (
    SELECT 1 FROM service existing
    WHERE existing.category_id = category.id AND existing.name = seed.name
);

DELETE FROM service_specification_value existing
USING service, technical_category_seed mapping
WHERE existing.service_id = service.id
  AND service.name = mapping.service_name;

INSERT INTO service_specification_value (service_id, specification_id, value)
SELECT service.id, specification.id, value.value
FROM technical_service_seed seed
JOIN technical_category_seed mapping ON mapping.service_name = seed.name
JOIN service_category category ON category.name = mapping.category_name
JOIN service_category parent
  ON parent.id = category.parent_category_id
 AND parent.name = 'Técnicos'
JOIN service
  ON service.category_id = category.id AND service.name = seed.name
CROSS JOIN LATERAL jsonb_each_text(seed.specifications) value
JOIN service_category_specification specification
  ON specification.category_id = category.id AND specification.name = value.key
ON CONFLICT (service_id, specification_id)
DO UPDATE SET value = EXCLUDED.value;

INSERT INTO service_offer (
    service_id, provider_id, price, currency, billing_period, installation_cost,
    contract_period, available, coverage_summary, additional_costs_summary, service_url
)
SELECT service.id, provider.id, seed.price, 'CLP', seed.billing_period, 0,
       seed.contract_period, TRUE, seed.coverage, seed.additional_costs, NULL
FROM technical_service_seed seed
JOIN technical_category_seed mapping ON mapping.service_name = seed.name
JOIN service_category category
  ON category.name = mapping.category_name
JOIN service_category parent
  ON parent.id = category.parent_category_id
 AND parent.name = 'Técnicos'
JOIN service
  ON service.category_id = category.id AND service.name = seed.name
JOIN provider
  ON LOWER(BTRIM(provider.name)) = LOWER(seed.provider_name)
 AND provider.active = TRUE
WHERE NOT EXISTS (
    SELECT 1 FROM service_offer existing
    WHERE existing.service_id = service.id AND existing.provider_id = provider.id
);

DELETE FROM service_category category
USING service_category parent
WHERE category.parent_category_id = parent.id
  AND category.name = 'Servicios Técnicos'
  AND parent.name = 'Técnicos'
  AND NOT EXISTS (
      SELECT 1 FROM service existing WHERE existing.category_id = category.id
  );

COMMIT;
