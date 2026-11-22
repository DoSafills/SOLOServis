-- 044: integridad de proveedores y enlace de fuentes de tienda (TAL-188).
--
-- 1. Fusiona los proveedores cuyo nombre normalizado coincide (minúsculas,
--    btrim y espacios colapsados). Conserva el menor id, reasigna las dos FK
--    reales (service_offer y provider_location) y borra el resto. Si una
--    oferta o ubicación del duplicado choca con la del superviviente, mueve
--    su historial de precios al del superviviente y descarta la fila repetida.
-- 2. Crea el índice único por nombre normalizado, después de fusionar.
-- 3. Enlaza la fuente de datos "Falabella Scraper" con su tienda, buscando la
--    tienda por nombre; si no hay una única coincidencia no cambia nada.
--
-- Es idempotente: sin duplicados, con el índice ya creado y con la fuente ya
-- enlazada, no hace nada.

BEGIN;

DO $$
DECLARE
    grupo RECORD;
    keeper_id INTEGER;
    dup_ids INTEGER[];
    dup_id INTEGER;
BEGIN
    FOR grupo IN
        SELECT lower(btrim(regexp_replace(p.name, '[[:space:]]+', ' ', 'g'))) AS nombre
        FROM provider p
        GROUP BY 1
        HAVING count(*) > 1
    LOOP
        SELECT p.id
        INTO keeper_id
        FROM provider p
        WHERE lower(btrim(regexp_replace(p.name, '[[:space:]]+', ' ', 'g'))) = grupo.nombre
        ORDER BY p.id
        LIMIT 1;

        SELECT array_agg(p.id ORDER BY p.id)
        INTO dup_ids
        FROM provider p
        WHERE lower(btrim(regexp_replace(p.name, '[[:space:]]+', ' ', 'g'))) = grupo.nombre
          AND p.id <> keeper_id;

        FOREACH dup_id IN ARRAY dup_ids
        LOOP
            -- El historial de la oferta repetida pasa a la del superviviente
            -- antes de descartarla, para no perder precios.
            UPDATE service_price_history h
            SET service_offer_id = k.id
            FROM service_offer d, service_offer k
            WHERE k.service_id = d.service_id
              AND k.provider_id = keeper_id
              AND d.provider_id = dup_id
              AND h.service_offer_id = d.id;

            DELETE FROM service_offer d
            USING service_offer k
            WHERE d.provider_id = dup_id
              AND k.service_id = d.service_id
              AND k.provider_id = keeper_id;

            UPDATE service_offer SET provider_id = keeper_id WHERE provider_id = dup_id;

            DELETE FROM provider_location d
            USING provider_location k
            WHERE d.provider_id = dup_id
              AND k.location_id = d.location_id
              AND k.provider_id = keeper_id;

            UPDATE provider_location SET provider_id = keeper_id WHERE provider_id = dup_id;

            DELETE FROM provider WHERE id = dup_id;

            RAISE NOTICE '044: proveedor % fusionado en %', dup_id, keeper_id;
        END LOOP;
    END LOOP;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS provider_name_normalized_key
    ON provider ((lower(btrim(regexp_replace(name, '[[:space:]]+', ' ', 'g')))));

DO $$
DECLARE
    coincidencias INTEGER;
    store_id_encontrado INTEGER;
BEGIN
    SELECT count(*), min(s.id)
    INTO coincidencias, store_id_encontrado
    FROM store s
    WHERE lower(btrim(s.name)) = 'falabella';

    IF coincidencias <> 1 THEN
        RAISE NOTICE '044: hay % tiendas Falabella; data_source queda sin store_id', coincidencias;
        RETURN;
    END IF;

    UPDATE data_source d
    SET store_id = store_id_encontrado,
        updated_at = NOW()
    WHERE d.name = 'Falabella Scraper'
      AND d.store_id IS NULL;
END $$;

COMMIT;
