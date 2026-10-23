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

	var offerID, storeID int32
	err = pool.QueryRow(ctx, `
		WITH candidate AS (
			SELECT p.id AS product_id, s.id AS store_id
			FROM product p
			CROSS JOIN store s
			WHERE p.active = true
			  AND NOT EXISTS (
				  SELECT 1
				  FROM product_offer o
				  WHERE o.product_id = p.id
				    AND o.store_id = s.id
			  )
			ORDER BY p.id, s.id
			LIMIT 1
		)
		INSERT INTO product_offer (product_id, store_id, price, available, product_url)
		SELECT product_id, store_id, 1, false, $1
		FROM candidate
		RETURNING id, store_id`, fmt.Sprintf("https://example.test/store-test-%d", time.Now().UnixNano())).Scan(&offerID, &storeID)
	if err != nil {
		t.Fatalf("create unavailable test offer: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, `DELETE FROM product_offer WHERE id = $1`, offerID); err != nil {
			t.Errorf("delete test offer %d: %v", offerID, err)
		}
	})

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
