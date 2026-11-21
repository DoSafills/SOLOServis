package search

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestSearchIntegration_SearchTextEIndices comprueba lo que agregan las
// migraciones 027 y 028 (TAL-189): que product.search_text normaliza nombre,
// modelo y SKU igual que antes lo hacían las consultas (sin tildes, en
// minúsculas, separados por un espacio), que se recalcula al editar el
// producto, que la búsqueda por texto lo usa, y que los índices de Ingestion
// existen y el redundante ya no. Se omite sin TEST_DATABASE_URL.
func TestSearchIntegration_SearchTextEIndices(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL no definida: se omite el test de integración")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	tag := "tal189-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	var productID int32
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM product WHERE id = $1`, productID)
	})
	if err := pool.QueryRow(ctx, `
		INSERT INTO product (category_id, name, model, sku)
		SELECT legacy_product_category_id, $1, 'ÁB-Ñ1', NULL
		FROM categories WHERE slug = 'refrigeradores'
		RETURNING id`, "Refrigerador FÁCIL Ñandú "+tag).Scan(&productID); err != nil {
		t.Fatalf("crear producto: %v", err)
	}

	searchText := func() string {
		t.Helper()
		var text string
		if err := pool.QueryRow(ctx, `SELECT search_text FROM product WHERE id = $1`, productID).Scan(&text); err != nil {
			t.Fatal(err)
		}
		return text
	}
	if got, want := searchText(), "refrigerador facil nandu "+tag+" ab-n1 "; got != want {
		t.Fatalf("search_text = %q, quiero %q", got, want)
	}

	if _, err := pool.Exec(ctx, `UPDATE product SET model = 'Modelo-Ü', sku = $2 WHERE id = $1`, productID, "SKU-É-"+tag); err != nil {
		t.Fatal(err)
	}
	if got, want := searchText(), "refrigerador facil nandu "+tag+" modelo-u sku-e-"+tag; got != want {
		t.Fatalf("search_text tras editar = %q, quiero %q", got, want)
	}

	repo := NewRepository(pool)
	for _, words := range [][]string{{"FÁCIL", tag}, {"modelo-ü", tag}, {"ñandu", tag}} {
		_, total, err := repo.SearchProducts(ctx, Query{Words: words, Limit: 5})
		if err != nil {
			t.Fatal(err)
		}
		if total != 1 {
			t.Fatalf("buscar %q encuentra %d producto(s), quiero 1", words, total)
		}
	}

	for _, tc := range []struct {
		index string
		want  bool
	}{
		{"idx_product_offer_store_normalized_url", true},
		{"idx_product_model_lower", true},
		{"idx_product_price_history_offer", false},
	} {
		var exists bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname = 'public' AND indexname = $1)`,
			tc.index).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if exists != tc.want {
			t.Fatalf("índice %s existe = %v, quiero %v", tc.index, exists, tc.want)
		}
	}
}
