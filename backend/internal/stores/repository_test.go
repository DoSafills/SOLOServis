package stores

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestListOffersIncludesUnavailableOffers(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create test database pool: %v", err)
	}
	t.Cleanup(pool.Close)

	// Producto, tienda y oferta no disponible propios del test (TAL-241: el
	// seed ya no trae productos). Borrar el producto y la tienda se lleva la oferta.
	tag := fmt.Sprintf("store-test-%d", time.Now().UnixNano())
	var productID, offerID, storeID int32
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM product WHERE id = $1`, productID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM store WHERE id = $1`, storeID)
	})
	if err := pool.QueryRow(ctx, `INSERT INTO store (name) VALUES ($1) RETURNING id`, "Tienda "+tag).Scan(&storeID); err != nil {
		t.Fatalf("create test store: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO product (category_id, category_ref, name)
		SELECT c.legacy_product_category_id, c.id, $1 FROM categories c WHERE c.slug = 'refrigeradores'
		RETURNING id`, "Producto "+tag).Scan(&productID); err != nil {
		t.Fatalf("create test product: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO product_offer (product_id, store_id, price, available, product_url)
		VALUES ($1, $2, 1, false, $3) RETURNING id`,
		productID, storeID, "https://example.test/"+tag).Scan(&offerID); err != nil {
		t.Fatalf("create unavailable test offer: %v", err)
	}

	offers, err := NewRepository(pool).ListOffers(ctx, storeID)
	if err != nil {
		t.Fatalf("list store offers: %v", err)
	}
	for _, offer := range offers {
		if offer.ID == offerID {
			if offer.Available {
				t.Fatalf("offer %d is available, want unavailable", offerID)
			}
			return
		}
	}
	t.Fatalf("unavailable offer %d was not returned", offerID)
}
