package products

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestGetOfferByID(t *testing.T) {
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

	var offerID int32
	if err := pool.QueryRow(ctx, `SELECT id FROM product_offer ORDER BY id LIMIT 1`).Scan(&offerID); err != nil {
		t.Fatalf("select test offer: %v", err)
	}

	offer, err := NewRepository(pool).GetOfferByID(ctx, offerID)
	if err != nil {
		t.Fatalf("get offer by id: %v", err)
	}
	if offer.ID != offerID {
		t.Fatalf("offer id = %d, want %d", offer.ID, offerID)
	}
}

func TestNormalizeProductOfferURLMatchesDatabase(t *testing.T) {
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

	for _, input := range []string{
		"  HTTPS://Shop.Example.COM/Path/?utm_source=x#section  ",
		"not a URL#fragment",
		"https://shop.example/%zz#fragment",
		"HTTPS://[2001:DB8::1]/Path/",
		"HTTPS://[FE80::1%25en0]/Path/",
		"https://[invalid]/Path/#fragment",
		"https://shop.example/path#frag%zz",
	} {
		var normalized string
		err := pool.QueryRow(ctx, `SELECT normalize_product_offer_url($1)`, input).Scan(&normalized)
		if err != nil {
			t.Fatalf("normalize URL in database: %v", err)
		}
		if want := NormalizeProductOfferURL(input); normalized != want {
			t.Errorf("database normalization of %q = %q, Go returned %q", input, normalized, want)
		}
	}
}

func TestUpsertOfferRecordsOnlyChangedPrices(t *testing.T) {
	ctx := context.Background()
	pool := repositoryTestPool(t)
	productID, storeIDs := repositoryTestProductAndStores(t, pool)
	repository := NewRepository(pool)
	url := fmt.Sprintf("https://example.test/tal64-%d/", time.Now().UnixNano())

	first, err := repository.UpsertOffer(ctx, repositoryTestUpsertParams(t, productID, storeIDs[0], "10.25", "USD", url, true))
	if err != nil {
		t.Fatalf("insert offer: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, `DELETE FROM product_offer WHERE id = $1`, first.ID); err != nil {
			t.Errorf("delete test offer %d: %v", first.ID, err)
		}
	})

	assertOfferHistory(t, pool, first.ID, []string{"10.25"})

	time.Sleep(20 * time.Millisecond)

	samePrice, err := repository.UpsertOffer(ctx, repositoryTestUpsertParams(t, productID, storeIDs[0], "10.25", "USD", " HTTPS://EXAMPLE.TEST/tal64-"+url[strings.LastIndex(url, "-")+1:], false))
	if err != nil {
		t.Fatalf("upsert offer without price change: %v", err)
	}
	if samePrice.ID != first.ID || samePrice.Available {
		t.Fatalf("same-price upsert = %+v, want existing unavailable offer", samePrice)
	}
	assertOfferHistory(t, pool, first.ID, []string{"10.25"})

	second, err := repository.UpsertOffer(ctx, repositoryTestUpsertParams(t, productID, storeIDs[0], "20.50", "USD", " HTTPS://EXAMPLE.TEST/tal64-"+url[strings.LastIndex(url, "-")+1:], false))
	if err != nil {
		t.Fatalf("update offer: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("upsert id = %d, want existing id %d", second.ID, first.ID)
	}
	if !second.LastUpdated.Valid || !second.LastUpdated.Time.After(first.LastUpdated.Time) {
		t.Fatalf("last_updated did not advance: before %v, after %v", first.LastUpdated, second.LastUpdated)
	}
	assertOfferHistory(t, pool, first.ID, []string{"10.25", "20.50"})

	var price, currency string
	var historyCount int
	err = pool.QueryRow(ctx, `
		SELECT o.price::text, o.currency,
		       (SELECT COUNT(*) FROM product_price_history h WHERE h.product_offer_id = o.id)
		FROM product_offer o
		WHERE o.id = $1`, second.ID).Scan(&price, &currency, &historyCount)
	if err != nil {
		t.Fatalf("read updated offer: %v", err)
	}
	if price != "20.50" || currency != "CLP" {
		t.Fatalf("stored price/currency = %s/%s, want 20.50/CLP", price, currency)
	}
	if historyCount != 2 {
		t.Fatalf("upsert wrote %d price-history rows, want 2", historyCount)
	}
	if second.Available {
		t.Fatal("updated offer is available, want unavailable")
	}

	updated, err := repository.UpdateOfferPriceAvailability(ctx, generated.UpdateProductOfferPriceAvailabilityParams{
		Price:     repositoryTestNumeric(t, "25.75"),
		Available: true,
		ID:        second.ID,
	})
	if err != nil {
		t.Fatalf("update offer price and availability: %v", err)
	}
	if !updated.Available || !updated.LastUpdated.Time.After(second.LastUpdated.Time) {
		t.Fatalf("price/availability update did not refresh offer: %+v", updated)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM product_price_history WHERE product_offer_id = $1`, updated.ID).Scan(&historyCount); err != nil {
		t.Fatalf("count history after price/availability update: %v", err)
	}
	if historyCount != 3 {
		t.Fatalf("price update wrote %d price-history rows, want 3", historyCount)
	}
	assertOfferHistory(t, pool, first.ID, []string{"10.25", "20.50", "25.75"})

	availabilityOnly, err := repository.UpdateOfferPriceAvailability(ctx, generated.UpdateProductOfferPriceAvailabilityParams{
		Price:     repositoryTestNumeric(t, "25.75"),
		Available: false,
		ID:        updated.ID,
	})
	if err != nil {
		t.Fatalf("update availability without price change: %v", err)
	}
	if availabilityOnly.Available {
		t.Fatal("availability-only update did not persist")
	}
	assertOfferHistory(t, pool, first.ID, []string{"10.25", "20.50", "25.75"})
}

func assertOfferHistory(t *testing.T, pool *pgxpool.Pool, offerID int32, wantPrices []string) {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT id, price::text, recorded_at
		FROM product_price_history
		WHERE product_offer_id = $1
		ORDER BY recorded_at, id`, offerID)
	if err != nil {
		t.Fatalf("query offer history: %v", err)
	}
	defer rows.Close()

	type historyEntry struct {
		id         int32
		price      string
		recordedAt time.Time
	}
	var entries []historyEntry
	for rows.Next() {
		var entry historyEntry
		if err := rows.Scan(&entry.id, &entry.price, &entry.recordedAt); err != nil {
			t.Fatalf("scan offer history: %v", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate offer history: %v", err)
	}
	if len(entries) != len(wantPrices) {
		t.Fatalf("history count = %d, want %d", len(entries), len(wantPrices))
	}
	for i, price := range wantPrices {
		if entries[i].price != price {
			t.Fatalf("history[%d].price = %s, want %s", i, entries[i].price, price)
		}
		if i > 0 && !entries[i].recordedAt.After(entries[i-1].recordedAt) {
			t.Fatalf("history timestamps are not increasing: %v then %v", entries[i-1].recordedAt, entries[i].recordedAt)
		}
	}
}

func TestUpsertOfferAllowsProductInMultipleStoresAndRejectsDuplicateInsert(t *testing.T) {
	ctx := context.Background()
	pool := repositoryTestPool(t)
	productID, storeIDs := repositoryTestProductAndStores(t, pool)
	url := fmt.Sprintf("https://example.test/tal64-multistore-%d", time.Now().UnixNano())
	repository := NewRepository(pool)

	first, err := repository.UpsertOffer(ctx, repositoryTestUpsertParams(t, productID, storeIDs[0], "5", "CLP", url, true))
	if err != nil {
		t.Fatalf("create first store offer: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, `DELETE FROM product_offer WHERE id = $1`, first.ID); err != nil {
			t.Errorf("delete test offer %d: %v", first.ID, err)
		}
	})

	second, err := repository.UpsertOffer(ctx, repositoryTestUpsertParams(t, productID, storeIDs[1], "6", "CLP", url, true))
	if err != nil {
		t.Fatalf("create second store offer: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, `DELETE FROM product_offer WHERE id = $1`, second.ID); err != nil {
			t.Errorf("delete test offer %d: %v", second.ID, err)
		}
	})
	if first.StoreID == second.StoreID || first.ProductID != second.ProductID {
		t.Fatalf("offers are not for one product across two stores: first=%+v second=%+v", first, second)
	}

	duplicateURL := strings.Replace(url, "https://example.test", "HTTPS://EXAMPLE.TEST", 1) + "/"
	_, err = pool.Exec(ctx, `
		INSERT INTO product_offer (product_id, store_id, price, currency, product_url, available)
		VALUES ($1, $2, 7, 'CLP', $3, true)`, productID, storeIDs[0], duplicateURL)
	if err == nil {
		t.Fatal("duplicate offer with the same normalized identity was accepted")
	}
}

func TestUpsertOfferRejectsInvalidCurrencyAndForeignKeys(t *testing.T) {
	ctx := context.Background()
	pool := repositoryTestPool(t)
	productID, storeIDs := repositoryTestProductAndStores(t, pool)
	repository := NewRepository(pool)
	params := repositoryTestUpsertParams(t, productID, storeIDs[0], "10", "US1",
		fmt.Sprintf("https://example.test/tal64-invalid-%d", time.Now().UnixNano()), true)
	if _, err := repository.UpsertOffer(ctx, params); err == nil {
		t.Fatal("upsert accepted a currency that is not three uppercase letters")
	}

	_, err := pool.Exec(ctx, `
		INSERT INTO product_offer (product_id, store_id, price, currency, product_url, available)
		VALUES ($1, $2, 1, 'CLP', $3, true)`,
		int32(2147483647), storeIDs[0],
		fmt.Sprintf("https://example.test/tal64-missing-product-%d", time.Now().UnixNano()))
	if err == nil {
		t.Fatal("offer with a missing product was accepted")
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO product_offer (product_id, store_id, price, currency, product_url, available)
		VALUES ($1, $2, 1, 'CLP', $3, true)`,
		productID, int32(2147483647),
		fmt.Sprintf("https://example.test/tal64-missing-store-%d", time.Now().UnixNano()))
	if err == nil {
		t.Fatal("offer with a missing store was accepted")
	}
}

func TestOfferRejectsNegativePrice(t *testing.T) {
	ctx := context.Background()
	pool := repositoryTestPool(t)
	var offerID int32
	if err := pool.QueryRow(ctx, `SELECT id FROM product_offer ORDER BY id LIMIT 1`).Scan(&offerID); err != nil {
		t.Fatalf("select test offer: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE product_offer SET price = -1 WHERE id = $1`, offerID); err == nil {
		t.Fatal("negative offer price was accepted")
	}
}

func repositoryTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("create test database pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func repositoryTestProductAndStores(t *testing.T, pool *pgxpool.Pool) (int32, []int32) {
	t.Helper()
	ctx := context.Background()
	var productID int32
	if err := pool.QueryRow(ctx, `
		SELECT id FROM product WHERE active = true ORDER BY id LIMIT 1`).Scan(&productID); err != nil {
		t.Fatalf("select test product: %v", err)
	}

	rows, err := pool.Query(ctx, `SELECT id FROM store WHERE active = true ORDER BY id LIMIT 2`)
	if err != nil {
		t.Fatalf("select test stores: %v", err)
	}
	defer rows.Close()
	var storeIDs []int32
	for rows.Next() {
		var storeID int32
		if err := rows.Scan(&storeID); err != nil {
			t.Fatalf("scan test store: %v", err)
		}
		storeIDs = append(storeIDs, storeID)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate test stores: %v", err)
	}
	if len(storeIDs) < 2 {
		t.Fatal("need at least two active stores for repository tests")
	}
	return productID, storeIDs
}

func repositoryTestUpsertParams(t *testing.T, productID, storeID int32, price, currency, productURL string, available bool) generated.UpsertProductOfferParams {
	t.Helper()
	return generated.UpsertProductOfferParams{
		ProductID: productID,
		StoreID:   storeID,
		Price:     repositoryTestNumeric(t, price),
		Currency:  currency,
		ProductUrl: pgtype.Text{
			String: productURL,
			Valid:  true,
		},
		Available: available,
	}
}

func repositoryTestNumeric(t *testing.T, value string) pgtype.Numeric {
	t.Helper()
	var numeric pgtype.Numeric
	if err := numeric.Scan(value); err != nil {
		t.Fatalf("parse test price: %v", err)
	}
	return numeric
}
