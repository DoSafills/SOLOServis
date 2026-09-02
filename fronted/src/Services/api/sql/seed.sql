-- Semilla con los mismos datos que tenías en el mock, para poblar la BD real

-- ==================== TIENDAS ====================
INSERT INTO stores (id, name, logo, rating, review_count, product_count, reputation, dispatch_time, conditions, website) VALUES
('techzone',    'TechZone',    'TZ', 4.7, 12840, 8420, 'Excelente',  '24–48 hrs',        'Despacho gratis sobre $50.000. Devoluciones en 30 días.', 'techzone.cl'),
('megapc',      'MegaPC',      'MP', 4.5, 9310,  6150, 'Muy buena',  '48–72 hrs',        'Despacho gratis sobre $70.000. Devoluciones en 15 días.', 'megapc.cl'),
('infoplex',    'InfoPlex',    'IP', 4.3, 5670,  4200, 'Buena',      '3–5 días hábiles', 'Despacho a cargo del cliente. Devoluciones en 10 días.', 'infoplex.cl'),
('digitalstore','DigitalStore','DS', 4.6, 7890,  5800, 'Muy buena',  '24 hrs',           'Despacho gratis sobre $40.000. Devoluciones en 30 días.', 'digitalstore.cl')
ON CONFLICT (id) DO NOTHING;

-- ==================== PRODUCTOS ====================
INSERT INTO products (id, name, brand, model, category, subcategory, image, description, rating, review_count, offer_price, tags) VALUES
('rtx-4060', 'ASUS RTX 4060 Dual 8GB', 'ASUS', 'DUAL-RTX4060-O8G', 'Tecnología', 'Gaming',
 'https://images.unsplash.com/photo-1591489378430-ef2f4c626b35?w=600&h=400&fit=crop&auto=format',
 'La RTX 4060 de ASUS con diseño dual fan ofrece rendimiento excepcional en gaming 1080p con tecnología DLSS 3.',
 4.6, 342, 269990, ARRAY['Nvidia','GPU','Gaming','RTX 40']),

('rtx-4060-ti', 'MSI RTX 4060 Ti Gaming X 16GB', 'MSI', 'RTX 4060 Ti GAMING X 16G', 'Tecnología', 'Gaming',
 'https://images.unsplash.com/photo-1587202372616-b43abea06c2a?w=600&h=400&fit=crop&auto=format',
 'La RTX 4060 Ti de MSI con 16GB de VRAM, ideal para contenido creativo y gaming 1440p con máximos detalles.',
 4.8, 218, NULL, ARRAY['Nvidia','GPU','Gaming','RTX 40']),

('rx-7600', 'Sapphire Pulse RX 7600 8GB', 'Sapphire', 'PULSE RX 7600 8GB', 'Tecnología', 'Gaming',
 'https://images.unsplash.com/photo-1624705002806-5d72df19c3ad?w=600&h=400&fit=crop&auto=format',
 'La alternativa AMD de alto valor con 8GB GDDR6. Excelente para gaming 1080p y edición de video ligera.',
 4.4, 187, NULL, ARRAY['AMD','GPU','Gaming','RX 7000']),

('lenovo-loq', 'Lenovo LOQ 15IRX9 Gaming', 'Lenovo', 'LOQ 15IRX9', 'Computación', 'Notebooks',
 'https://images.unsplash.com/photo-1525547719571-a2d4ac8945e2?w=600&h=400&fit=crop&auto=format',
 'Notebook gaming con i7 de 13ª gen y RTX 4060. Pantalla 144Hz IPS y 16GB RAM DDR5. Relación precio-rendimiento excepcional.',
 4.5, 456, 799990, ARRAY['Lenovo','Notebook','Gaming','Intel']),

('iphone-15', 'Apple iPhone 15 128GB', 'Apple', 'iPhone 15', 'Celulares', 'Smartphones',
 'https://images.unsplash.com/photo-1697051897520-8d7d0f77ab8f?w=600&h=400&fit=crop&auto=format',
 'iPhone 15 con chip A16 Bionic, Dynamic Island, cámara principal de 48 MP y conector USB-C.',
 4.7, 1204, NULL, ARRAY['Apple','iPhone','Smartphone','5G']),

('galaxy-s25', 'Samsung Galaxy S25 256GB', 'Samsung', 'Galaxy S25', 'Celulares', 'Smartphones',
 'https://images.unsplash.com/photo-1610945415295-d9bbf067e59c?w=600&h=400&fit=crop&auto=format',
 'Galaxy S25 con Snapdragon 8 Elite, cámara de 50 MP y Android 15. Integración avanzada con Galaxy AI.',
 4.6, 876, NULL, ARRAY['Samsung','Android','Smartphone','5G'])
ON CONFLICT (id) DO NOTHING;

-- offerPrice real para iphone-15 (en el mock quedaba como campo suelto vía spread, aquí explícito)
UPDATE products SET offer_price = NULL WHERE id = 'iphone-15';

-- ==================== IMÁGENES ====================
INSERT INTO product_images (product_id, url, position) VALUES
('rtx-4060', 'https://images.unsplash.com/photo-1591489378430-ef2f4c626b35?w=600&h=400&fit=crop&auto=format', 0),
('rtx-4060', 'https://images.unsplash.com/photo-1587202372616-b43abea06c2a?w=600&h=400&fit=crop&auto=format', 1),
('rtx-4060-ti', 'https://images.unsplash.com/photo-1587202372616-b43abea06c2a?w=600&h=400&fit=crop&auto=format', 0),
('rx-7600', 'https://images.unsplash.com/photo-1624705002806-5d72df19c3ad?w=600&h=400&fit=crop&auto=format', 0),
('lenovo-loq', 'https://images.unsplash.com/photo-1525547719571-a2d4ac8945e2?w=600&h=400&fit=crop&auto=format', 0),
('iphone-15', 'https://images.unsplash.com/photo-1697051897520-8d7d0f77ab8f?w=600&h=400&fit=crop&auto=format', 0),
('galaxy-s25', 'https://images.unsplash.com/photo-1610945415295-d9bbf067e59c?w=600&h=400&fit=crop&auto=format', 0);

-- ==================== SPECS ====================
INSERT INTO product_specs (product_id, spec_key, spec_value) VALUES
('rtx-4060','VRAM','8GB GDDR6'),
('rtx-4060','Arquitectura','Ada Lovelace'),
('rtx-4060','Núcleos CUDA','3072'),
('rtx-4060','Bus de memoria','128-bit'),
('rtx-4060','TDP','115W'),
('rtx-4060','Dimensiones','240 × 122 × 44 mm'),
('rtx-4060','Garantía','3 años'),
('rtx-4060','Conectores','HDMI 2.1, 3× DP 1.4a'),

('rtx-4060-ti','VRAM','16GB GDDR6'),
('rtx-4060-ti','Arquitectura','Ada Lovelace'),
('rtx-4060-ti','Núcleos CUDA','4352'),
('rtx-4060-ti','Bus de memoria','128-bit'),
('rtx-4060-ti','TDP','165W'),
('rtx-4060-ti','Dimensiones','323 × 140 × 57 mm'),
('rtx-4060-ti','Garantía','3 años'),
('rtx-4060-ti','Conectores','HDMI 2.1, 3× DP 1.4a'),

('rx-7600','VRAM','8GB GDDR6'),
('rx-7600','Arquitectura','RDNA 3'),
('rx-7600','Stream Processors','2048'),
('rx-7600','Bus de memoria','128-bit'),
('rx-7600','TDP','165W'),
('rx-7600','Dimensiones','235 × 125 × 48 mm'),
('rx-7600','Garantía','2 años'),
('rx-7600','Conectores','HDMI 2.1, 3× DP 2.1'),

('lenovo-loq','Procesador','Intel Core i7-13620H'),
('lenovo-loq','GPU','NVIDIA RTX 4060 8GB'),
('lenovo-loq','RAM','16GB DDR5 4800MHz'),
('lenovo-loq','Almacenamiento','512GB SSD NVMe'),
('lenovo-loq','Pantalla','15.6" IPS 144Hz FHD'),
('lenovo-loq','Batería','60Wh'),
('lenovo-loq','Sistema operativo','Windows 11 Home'),
('lenovo-loq','Garantía','1 año'),

('iphone-15','Procesador','Apple A16 Bionic'),
('iphone-15','Almacenamiento','128GB'),
('iphone-15','RAM','6GB'),
('iphone-15','Pantalla','6.1" OLED 60Hz Super Retina XDR'),
('iphone-15','Cámara principal','48 MP f/1.6'),
('iphone-15','Batería','3877 mAh'),
('iphone-15','Sistema operativo','iOS 17'),
('iphone-15','Garantía','1 año'),

('galaxy-s25','Procesador','Snapdragon 8 Elite'),
('galaxy-s25','Almacenamiento','256GB'),
('galaxy-s25','RAM','12GB'),
('galaxy-s25','Pantalla','6.2" Dynamic AMOLED 120Hz'),
('galaxy-s25','Cámara principal','50 MP f/1.8'),
('galaxy-s25','Batería','4000 mAh'),
('galaxy-s25','Sistema operativo','Android 15'),
('galaxy-s25','Garantía','1 año');

-- ==================== OFERTAS POR TIENDA ====================
INSERT INTO offers (product_id, store_id, price, available, shipping) VALUES
('rtx-4060','techzone',299990,true,0),
('rtx-4060','megapc',309990,true,5990),
('rtx-4060','infoplex',319990,false,7990),
('rtx-4060','digitalstore',304990,true,0),

('rtx-4060-ti','techzone',449990,true,0),
('rtx-4060-ti','megapc',459990,true,0),
('rtx-4060-ti','digitalstore',444990,true,0),

('rx-7600','techzone',259990,true,0),
('rx-7600','infoplex',269990,true,7990),
('rx-7600','megapc',264990,false,5990),

('lenovo-loq','techzone',849990,true,0),
('lenovo-loq','digitalstore',839990,true,0),
('lenovo-loq','megapc',869990,true,0),
('lenovo-loq','infoplex',879990,false,NULL),

('iphone-15','techzone',749990,true,0),
('iphone-15','digitalstore',739990,true,0),
('iphone-15','megapc',759990,true,5990),

('galaxy-s25','techzone',829990,true,0),
('galaxy-s25','infoplex',819990,true,0),
('galaxy-s25','digitalstore',839990,false,NULL);

-- ==================== HISTORIAL DE PRECIOS (referencia de tienda principal) ====================
-- Genera puntos cada ~12 días durante el último año, con variación leve, igual espíritu que el mock.
INSERT INTO price_history (product_id, date, price)
SELECT v.id, d::date, round((v.base * (0.9 + 0.2 * random())) / 1000) * 1000
FROM (VALUES
  ('rtx-4060', 299990),
  ('rtx-4060-ti', 449990),
  ('rx-7600', 259990),
  ('lenovo-loq', 849990),
  ('iphone-15', 749990),
  ('galaxy-s25', 829990)
) AS v(id, base)
CROSS JOIN generate_series(CURRENT_DATE - INTERVAL '365 days', CURRENT_DATE, INTERVAL '12 days') AS d;

-- ==================== HISTORIAL DE OFERTAS/PROMOCIONES ====================
-- Solo para productos que tienen offer_price activo, igual que en el mock (promos esporádicas)
INSERT INTO offer_price_history (product_id, date, price)
SELECT v.id, CURRENT_DATE - v.days_ago, round((v.base * (1 - v.discount)) / 1000) * 1000
FROM (VALUES
  ('rtx-4060', 299990, 300, 0.18),
  ('rtx-4060', 299990, 200, 0.12),
  ('rtx-4060', 299990, 90,  0.15),
  ('rtx-4060', 299990, 15,  0.10),

  ('rtx-4060-ti', 449990, 300, 0.18),
  ('rtx-4060-ti', 449990, 200, 0.12),
  ('rtx-4060-ti', 449990, 90,  0.15),
  ('rtx-4060-ti', 449990, 15,  0.10),

  ('lenovo-loq', 849990, 300, 0.18),
  ('lenovo-loq', 849990, 200, 0.12),
  ('lenovo-loq', 849990, 90,  0.15),
  ('lenovo-loq', 849990, 15,  0.10),

  ('iphone-15', 749990, 300, 0.18),
  ('iphone-15', 749990, 200, 0.12),
  ('iphone-15', 749990, 90,  0.15),
  ('iphone-15', 749990, 15,  0.10)
) AS v(id, base, days_ago, discount);

-- ==================== SERVICIOS ====================
INSERT INTO services (id, name, provider, category, subcategory, description, monthly_price, installation_cost, contract_months, rating, review_count, coverage, image) VALUES
('entel-500', 'Internet Hogar 500 Mbps', 'Entel', 'Internet', 'Fibra óptica',
 'Fibra óptica simétrica de 500 Mbps para toda la familia. Sin límite de descarga.',
 19990, 0, 12, 4.3, 2340, 'Región Metropolitana, Valparaíso, Biobío',
 'https://images.unsplash.com/photo-1558494949-ef010cbdcc31?w=400&h=300&fit=crop&auto=format'),

('movistar-600', 'Internet Hogar 600 Mbps', 'Movistar', 'Internet', 'Fibra óptica',
 'Plan fibra óptica 600 Mbps con router Wi-Fi 6 y hasta 5 dispositivos conectados simultáneamente.',
 22990, 10000, NULL, 4.1, 1890, 'Cobertura nacional en 15 regiones',
 'https://images.unsplash.com/photo-1544197150-b99a580bb7a8?w=400&h=300&fit=crop&auto=format'),

('wom-100gb', 'Plan Móvil 100 GB', 'WOM', 'Telefonía', 'Plan móvil',
 'Plan móvil con 100 GB de datos, llamadas ilimitadas y roaming en Latinoamérica incluido.',
 14990, NULL, NULL, 4.0, 3210, 'Cobertura 5G en principales ciudades',
 'https://images.unsplash.com/photo-1511707171634-5f897ff02aa9?w=400&h=300&fit=crop&auto=format'),

('netflix-premium', 'Netflix Premium 4K', 'Netflix', 'Streaming', 'Video',
 'Acceso ilimitado a series, películas y documentales en calidad 4K Ultra HD. Hasta 4 pantallas simultáneas.',
 17990, NULL, NULL, 4.5, 8940, 'Disponible en todo Chile',
 'https://images.unsplash.com/photo-1574375927938-d5a98e8ffe85?w=400&h=300&fit=crop&auto=format')
ON CONFLICT (id) DO NOTHING;

INSERT INTO service_specs (service_id, spec_key, spec_value) VALUES
('entel-500','Velocidad bajada','500 Mbps'),
('entel-500','Velocidad subida','500 Mbps'),
('entel-500','Tecnología','Fibra óptica'),
('entel-500','Límite de datos','Sin límite'),
('entel-500','Router incluido','Sí'),
('entel-500','Soporte','24/7'),

('movistar-600','Velocidad bajada','600 Mbps'),
('movistar-600','Velocidad subida','600 Mbps'),
('movistar-600','Tecnología','Fibra óptica'),
('movistar-600','Límite de datos','Sin límite'),
('movistar-600','Router incluido','Sí'),
('movistar-600','Soporte','Lun–Sáb 8–20 hrs'),

('wom-100gb','Datos','100 GB 5G/4G LTE'),
('wom-100gb','Llamadas','Ilimitadas'),
('wom-100gb','SMS','Ilimitados'),
('wom-100gb','Roaming','LATAM incluido'),
('wom-100gb','Red','5G/4G LTE'),
('wom-100gb','Portabilidad','Gratis'),

('netflix-premium','Calidad','4K Ultra HD + HDR'),
('netflix-premium','Pantallas simultáneas','4'),
('netflix-premium','Descargas','Sí (hasta 4 dispositivos)'),
('netflix-premium','Idiomas','+30 idiomas'),
('netflix-premium','Audio','Dolby Atmos'),
('netflix-premium','Perfiles','Hasta 6');

INSERT INTO service_benefits (service_id, benefit) VALUES
('entel-500','Wi-Fi 6 incluido'),
('entel-500','Sin límite de datos'),
('entel-500','TV básica incluida'),

('movistar-600','Sin permanencia'),
('movistar-600','Router Wi-Fi 6'),
('movistar-600','App de control parental'),

('wom-100gb','Sin costo de portabilidad'),
('wom-100gb','Datos adicionales a $1.990/GB'),
('wom-100gb','App WOM incluida'),

('netflix-premium','Sin anuncios'),
('netflix-premium','Contenido exclusivo'),
('netflix-premium','Modo offline'),
('netflix-premium','Calidad 4K HDR');

INSERT INTO service_price_history (service_id, date, price)
SELECT v.id, d::date, round((v.base * (0.9 + 0.2 * random())) / 100) * 100
FROM (VALUES
  ('entel-500', 19990),
  ('movistar-600', 22990),
  ('wom-100gb', 14990),
  ('netflix-premium', 17990)
) AS v(id, base)
CROSS JOIN generate_series(CURRENT_DATE - INTERVAL '365 days', CURRENT_DATE, INTERVAL '12 days') AS d;
