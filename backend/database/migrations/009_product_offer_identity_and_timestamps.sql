BEGIN;

CREATE OR REPLACE FUNCTION normalize_product_offer_url(input_url TEXT)
RETURNS TEXT
LANGUAGE plpgsql
IMMUTABLE
PARALLEL SAFE
AS $$
DECLARE
    trimmed_url TEXT;
    url_without_fragment TEXT;
    url_parts TEXT[];
    authority_parts TEXT[];
    host_port TEXT;
    ipv6_host TEXT;
    path TEXT;
BEGIN
    IF input_url IS NULL THEN
        RETURN NULL;
    END IF;

    trimmed_url := BTRIM(input_url);
    url_without_fragment := SPLIT_PART(trimmed_url, '#', 1);
    url_parts := REGEXP_MATCH(
        url_without_fragment,
        '^([A-Za-z][A-Za-z0-9+.-]*://)([^/?#]+)([^?#]*)([?][^#]*)?$'
    );

    IF url_parts IS NULL
       OR trimmed_url ~ '[[:cntrl:]]'
       OR url_parts[2] ~ '[[:space:]]'
       OR trimmed_url ~ '%([^0-9A-Fa-f]|$)|%[0-9A-Fa-f]([^0-9A-Fa-f]|$)' THEN
        RETURN trimmed_url;
    END IF;

    authority_parts := REGEXP_MATCH(url_parts[2], '^(.*@)?([^@]+)$');
    IF authority_parts IS NULL THEN
        RETURN trimmed_url;
    END IF;

    host_port := authority_parts[2];
    IF host_port !~ '^(\[[^]]+\](:[0-9]+)?|[^:[:space:]\[\]]+(:[0-9]+)?)$' THEN
        RETURN trimmed_url;
    END IF;

    IF host_port LIKE '[%' THEN
        ipv6_host := SUBSTRING(host_port FROM '^\[([^]]+)\]');
        IF ipv6_host IS NULL THEN
            RETURN trimmed_url;
        END IF;
        IF ipv6_host ~ '%' THEN
            IF ipv6_host !~ '^[0-9A-Fa-f:.]+%25[A-Za-z0-9._~-]+$' THEN
                RETURN trimmed_url;
            END IF;
            ipv6_host := SPLIT_PART(ipv6_host, '%25', 1);
        ELSIF ipv6_host !~ '^[0-9A-Fa-f:.]+$' THEN
            RETURN trimmed_url;
        END IF;
        BEGIN
            IF FAMILY(ipv6_host::INET) <> 6 THEN
                RETURN trimmed_url;
            END IF;
        EXCEPTION WHEN invalid_text_representation THEN
            RETURN trimmed_url;
        END;
    END IF;

    path := REGEXP_REPLACE(url_parts[3], '/+$', '');
    RETURN LOWER(url_parts[1])
        || COALESCE(authority_parts[1], '')
        || LOWER(host_port)
        || path
        || COALESCE(url_parts[4], '');
END;
$$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM product_offer
        GROUP BY product_id, store_id, COALESCE(normalize_product_offer_url(product_url), '')
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'duplicate product offers exist for the normalized URL identity';
    END IF;
END;
$$;

ALTER TABLE product_offer
    DROP CONSTRAINT product_offer_product_id_store_id_key;

CREATE UNIQUE INDEX product_offer_product_store_normalized_url_key
    ON product_offer (
        product_id,
        store_id,
        (COALESCE(normalize_product_offer_url(product_url), ''))
    );

ALTER TABLE product_offer
    ALTER COLUMN last_updated TYPE TIMESTAMPTZ
        USING last_updated AT TIME ZONE 'Etc/UTC',
    ALTER COLUMN last_updated SET DEFAULT NOW();

COMMIT;
