-- name: ListPriceHistoryByOfferID :many
SELECT
    id,
    product_offer_id,
    price,
    is_promotional,
    recorded_at
FROM product_price_history
WHERE product_offer_id = $1
ORDER BY recorded_at DESC;
