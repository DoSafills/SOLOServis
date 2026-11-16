-- TAL-169: une los productos duplicados que creó el scraper.
--
-- Ingestion solo reconocía un producto por SKU o por marca + modelo, y el
-- scraper no envía ninguno: cada corrida creaba el producto de nuevo. Aquí se
-- deja uno por tienda + URL de la oferta (sin parámetros ni fragmento), que es
-- la misma identidad que ahora usa ingestion.
--
-- Alcance, a propósito estrecho:
--   - solo productos sin SKU y sin modelo, con exactamente una oferta;
--   - se conserva el más antiguo de cada grupo, con su oferta;
--   - una copia con datos de usuarios (seguimiento, reseña, carrito o
--     comparación) no se borra.
-- El historial de precios de las copias pasa a la oferta conservada, que queda
-- con el precio y la disponibilidad más recientes.
--
-- Todo va en un solo bloque: o se aplica completo o no cambia nada. Es
-- idempotente: sin duplicados no hace nada.
DO $$
DECLARE
    grupos INTEGER;
    borrados INTEGER;
    historial INTEGER;
BEGIN
    CREATE TEMP TABLE dedupe_scraped ON COMMIT DROP AS
    WITH candidatos AS (
        SELECT p.id AS product_id,
               o.id AS offer_id,
               o.store_id,
               split_part(normalize_product_offer_url(o.product_url), '?', 1) AS url_key,
               o.last_updated
        FROM product p
        JOIN product_offer o ON o.product_id = p.id
        WHERE p.sku IS NULL
          AND COALESCE(btrim(p.model), '') = ''
          AND (SELECT count(*) FROM product_offer x WHERE x.product_id = p.id) = 1
    )
    SELECT c.product_id,
           c.offer_id,
           first_value(c.product_id) OVER por_antiguedad AS keep_product_id,
           first_value(c.offer_id) OVER por_antiguedad AS keep_offer_id,
           first_value(c.offer_id) OVER (
               PARTITION BY c.store_id, c.url_key ORDER BY c.last_updated DESC, c.offer_id DESC
           ) AS latest_offer_id
    FROM candidatos c
    WHERE COALESCE(c.url_key, '') <> ''
    WINDOW por_antiguedad AS (PARTITION BY c.store_id, c.url_key ORDER BY c.product_id);

    -- Fuera de la lista: el producto conservado y las copias con datos de usuarios.
    DELETE FROM dedupe_scraped d
    WHERE d.product_id = d.keep_product_id
       OR EXISTS (SELECT 1 FROM product_user_interest t WHERE t.product_id = d.product_id)
       OR EXISTS (SELECT 1 FROM product_review t WHERE t.product_id = d.product_id)
       OR EXISTS (SELECT 1 FROM product_comparison_item t WHERE t.product_id = d.product_id)
       OR EXISTS (SELECT 1 FROM cart_item t WHERE t.product_id = d.product_id OR t.offer_id = d.offer_id);

    SELECT count(DISTINCT keep_product_id), count(*) INTO grupos, borrados FROM dedupe_scraped;
    IF borrados = 0 THEN
        RAISE NOTICE 'dedupe_scraped_products: sin duplicados, no se cambia nada';
        RETURN;
    END IF;

    -- El historial de las copias pasa a la oferta conservada. Un registro que
    -- ya existe igual (mismo precio y momento) se omite y cae con su copia.
    UPDATE product_price_history h
    SET product_offer_id = d.keep_offer_id
    FROM dedupe_scraped d
    WHERE h.product_offer_id = d.offer_id
      AND NOT EXISTS (
          SELECT 1 FROM product_price_history k
          WHERE k.product_offer_id = d.keep_offer_id
            AND k.price = h.price
            AND k.recorded_at = h.recorded_at);
    GET DIAGNOSTICS historial = ROW_COUNT;

    -- La oferta conservada toma los datos de la copia más reciente. El trigger
    -- de historial se desactiva: esos precios ya quedaron registrados arriba.
    ALTER TABLE product_offer DISABLE TRIGGER product_offer_price_history_after_change;
    UPDATE product_offer k
    SET price = l.price,
        list_price = l.list_price,
        currency = l.currency,
        shipping_cost = l.shipping_cost,
        shipping_free = l.shipping_free,
        available = l.available,
        stock = l.stock,
        condition = l.condition,
        last_updated = l.last_updated
    FROM (SELECT DISTINCT keep_offer_id, latest_offer_id FROM dedupe_scraped) d
    JOIN product_offer l ON l.id = d.latest_offer_id
    WHERE k.id = d.keep_offer_id
      AND d.latest_offer_id <> d.keep_offer_id;
    ALTER TABLE product_offer ENABLE TRIGGER product_offer_price_history_after_change;

    -- Las copias se borran; sus ofertas y atributos caen por ON DELETE CASCADE.
    DELETE FROM product p USING dedupe_scraped d WHERE p.id = d.product_id;

    RAISE NOTICE 'dedupe_scraped_products: % productos conservados, % copias borradas, % registros de historial traspasados',
        grupos, borrados, historial;
END;
$$;
