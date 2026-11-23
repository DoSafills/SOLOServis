package products

import (
	"context"
	"testing"
)

func TestTAL66UniqueConstraintsRejectNormalizedDuplicates(t *testing.T) {
	ctx := context.Background()
	pool := repositoryTestPool(t)

	_, err := pool.Exec(ctx, `
		INSERT INTO product_category (parent_category_id, name)
		SELECT parent_category_id, '  ' || UPPER(name) || '  '
		FROM product_category
		ORDER BY id
		LIMIT 1`)
	if err == nil {
		t.Fatal("normalized duplicate category under the same parent was accepted")
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO store (name)
		SELECT '  ' || UPPER(name) || '  '
		FROM store
		ORDER BY id
		LIMIT 1`)
	if err == nil {
		t.Fatal("normalized duplicate store name was accepted")
	}

	// La oferta propia del test trae su fila de historial (TAL-241: el seed ya
	// no trae ofertas).
	offerID := repositoryTestOffer(t, pool)
	_, err = pool.Exec(ctx, `
		INSERT INTO product_price_history (product_offer_id, price, recorded_at)
		SELECT product_offer_id, price, recorded_at
		FROM product_price_history
		WHERE product_offer_id = $1
		ORDER BY id
		LIMIT 1`, offerID)
	if err == nil {
		t.Fatal("duplicate history row for same offer, price and instant was accepted")
	}
}
