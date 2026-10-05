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
    ('WOM', 'https://wom.cl'),
    ('Claro', 'https://clarochile.cl'),
    ('VTR', 'https://vtr.com'),
    ('GTD', 'https://gtd.cl'),
    ('DIRECTV', 'https://directv.cl'),
    ('Mundo', 'https://mundo.cl')
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
        ('Internet Fibra Óptica 800 Megas',
         'Plan hogar de fibra óptica de hasta 800 Mbps para varios dispositivos conectados en simultáneo, clases, videollamadas y streaming.',
         800, 800, FALSE),
        ('Internet Fibra Óptica 940 Megas',
         'Plan hogar de fibra óptica de hasta 940 Mbps para uso técnico exigente, descargas pesadas y teletrabajo.',
         940, 940, FALSE),
        ('Internet Fibra Óptica 2.000 Megas',
         'Plan hogar de fibra óptica de hasta 2 Gbps para uso técnico avanzado y hogares con alta demanda de ancho de banda.',
         2000, 2000, FALSE),
        ('Fibra 200 Mbps Entel',
         'Plan hogar de fibra óptica de 200 Mbps para navegación, clases online y uso diario.',
         200, 200, TRUE),
        ('Fibra 400 Mbps Entel',
         'Plan hogar de fibra óptica de 400 Mbps para teletrabajo, estudio y videollamadas simultáneas.',
         400, 400, TRUE),
        ('Fibra 600 Mbps Entel',
         'Plan hogar de fibra óptica de 600 Mbps para hogares con varios usuarios y uso técnico frecuente.',
         600, 600, TRUE),
        ('Fibra WOM 300',
         'Plan hogar de fibra óptica simétrica de 300 Mbps para estudio y teletrabajo.',
         300, 300, FALSE),
        ('Fibra WOM 600',
         'Plan hogar de fibra óptica simétrica de 600 Mbps para hogares con alto uso de videollamadas y descargas.',
         600, 600, FALSE),
        ('Fibra Hogar 600 Megas Claro',
         'Plan hogar de fibra óptica de 600 Mbps para estudio online y teletrabajo.',
         600, 600, TRUE),
        ('Fibra Hogar 800 Megas WiFi 6 Claro',
         'Plan hogar de fibra óptica avanzado de 800 Mbps con WiFi 6 para múltiples dispositivos.',
         800, 800, TRUE),
        ('Fibra 800 Megas VTR',
         'Plan hogar de fibra óptica de 800 Mbps con router WiFi VTR.',
         800, 800, TRUE),
        ('Fibra 600 Megas VTR',
         'Plan hogar de fibra óptica de hasta 600 Mbps para estudio y teletrabajo.',
         600, 600, TRUE),
        ('Fibra 400 Megas GTD',
         'Plan hogar de fibra óptica de 400 Mbps para uso educativo y teletrabajo.',
         400, 400, TRUE),
        ('Fibra 400 Megas DIRECTV',
         'Plan hogar de fibra óptica de 400 Mbps para estudio online y navegación diaria.',
         400, 400, TRUE),
        ('Fibra Mundo 500',
         'Plan hogar de fibra óptica de 500 Mbps de entrada para estudio y uso diario.',
         500, 500, TRUE)
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
        ('Internet Fibra Óptica 800 Megas', 800, 800, FALSE),
        ('Internet Fibra Óptica 940 Megas', 940, 940, FALSE),
        ('Internet Fibra Óptica 2.000 Megas', 2000, 2000, FALSE),
        ('Fibra 200 Mbps Entel', 200, 200, TRUE),
        ('Fibra 400 Mbps Entel', 400, 400, TRUE),
        ('Fibra 600 Mbps Entel', 600, 600, TRUE),
        ('Fibra WOM 300', 300, 300, FALSE),
        ('Fibra WOM 600', 600, 600, FALSE),
        ('Fibra Hogar 600 Megas Claro', 600, 600, TRUE),
        ('Fibra Hogar 800 Megas WiFi 6 Claro', 800, 800, TRUE),
        ('Fibra 800 Megas VTR', 800, 800, TRUE),
        ('Fibra 600 Megas VTR', 600, 600, TRUE),
        ('Fibra 400 Megas GTD', 400, 400, TRUE),
        ('Fibra 400 Megas DIRECTV', 400, 400, TRUE),
        ('Fibra Mundo 500', 500, 500, TRUE)
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
        ('Internet Fibra Óptica 800 Megas', 'Movistar', 27990::NUMERIC,
         'Sujeta a factibilidad técnica validada en terreno.',
         'Precio referencial; instalación, contrato y costos adicionales por verificar.'),
        ('Internet Fibra Óptica 940 Megas', 'Movistar', 38990::NUMERIC,
         'Sujeta a factibilidad técnica validada en terreno.',
         'Precio referencial; instalación, contrato y costos adicionales por verificar.'),
        ('Internet Fibra Óptica 2.000 Megas', 'Movistar', 45990::NUMERIC,
         'Sujeta a factibilidad técnica validada en terreno.',
         'Precio referencial; confirmar requisitos técnicos, instalación, contrato y costos adicionales.'),
        ('Fibra 200 Mbps Entel', 'Entel', 14990::NUMERIC,
         'Cobertura por verificar en el sitio oficial del proveedor.',
         'Precio referencial; instalación, contrato y costos adicionales por verificar.'),
        ('Fibra 400 Mbps Entel', 'Entel', 19990::NUMERIC,
         'Cobertura por verificar en el sitio oficial del proveedor.',
         'Precio referencial; instalación, contrato y costos adicionales por verificar.'),
        ('Fibra 600 Mbps Entel', 'Entel', 24990::NUMERIC,
         'Cobertura por verificar en el sitio oficial del proveedor.',
         'Precio referencial; instalación, contrato y costos adicionales por verificar.'),
        ('Fibra WOM 300', 'WOM', 14990::NUMERIC,
         'Cobertura actual por verificar; la fuente citada corresponde al lanzamiento del servicio.',
         'Precio referencial de lanzamiento 2021; vigencia, instalación, contrato y costos adicionales por verificar.'),
        ('Fibra WOM 600', 'WOM', 18990::NUMERIC,
         'Cobertura actual por verificar; la fuente citada corresponde al lanzamiento del servicio.',
         'Precio referencial de lanzamiento 2021; vigencia, instalación, contrato y costos adicionales por verificar.'),
        ('Fibra Hogar 600 Megas Claro', 'Claro', 19990::NUMERIC,
         'Cobertura por verificar en el sitio oficial del proveedor.',
         'Precio referencial; instalación, contrato y costos adicionales por verificar. La fuente indica una promoción de $15.990 sin fecha confirmada.'),
        ('Fibra Hogar 800 Megas WiFi 6 Claro', 'Claro', 23990::NUMERIC,
         'Cobertura por verificar en el sitio oficial del proveedor.',
         'Precio referencial; instalación, contrato y costos adicionales por verificar. La fuente indica una promoción de $16.900 sin fecha confirmada; confirmar marca del plan.'),
        ('Fibra 800 Megas VTR', 'VTR', 23990::NUMERIC,
         'Cobertura por verificar en el sitio oficial del proveedor.',
         'Precio referencial; instalación, contrato y costos adicionales por verificar. La fuente indica una promoción de $15.990 sin fecha confirmada.'),
        ('Fibra 600 Megas VTR', 'VTR', 21990::NUMERIC,
         'Cobertura por verificar en el sitio oficial del proveedor.',
         'Precio referencial de diciembre de 2024; vigencia, instalación, contrato y costos adicionales por verificar.'),
        ('Fibra 400 Megas GTD', 'GTD', 27990::NUMERIC,
         'Cobertura por verificar en el sitio oficial del proveedor.',
         'Precio referencial de diciembre de 2024; vigencia, instalación, contrato y costos adicionales por verificar.'),
        ('Fibra 400 Megas DIRECTV', 'DIRECTV', 23089::NUMERIC,
         'Cobertura por verificar en el sitio oficial del proveedor.',
         'Precio referencial de diciembre de 2024; vigencia, instalación, contrato y costos adicionales por verificar.'),
        ('Fibra Mundo 500', 'Mundo', 12425::NUMERIC,
         'Cobertura por verificar en el sitio oficial del proveedor.',
         'Precio referencial de 2022, posiblemente desactualizado; vigencia, instalación, contrato y costos adicionales por verificar.')
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

COMMIT;
