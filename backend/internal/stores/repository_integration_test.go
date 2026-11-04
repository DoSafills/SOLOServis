package stores

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/stores/dto"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Integración TAL-73 contra PostgreSQL con todas las migraciones aplicadas.
// Se omite sin TEST_DATABASE_URL.

type storeFixture struct {
	storeA, storeB, storeInactive int32
	product, inactiveProduct      pgtype.UUID
}

// newStoreFixture crea dos tiendas activas, una inactiva, un producto activo
// con 4 ofertas (una en la tienda inactiva y otra no disponible), un producto
// inactivo con oferta y una reseña de la tienda A. Todo se borra al terminar.
func newStoreFixture(t *testing.T, pool *pgxpool.Pool) storeFixture {
	t.Helper()
	ctx := context.Background()
	tag := strconv.FormatInt(time.Now().UnixNano(), 10)
	var f storeFixture
	var productID, inactiveID, userID int32

	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture: %v\n%s", err, sql)
		}
	}
	scan := func(dest any, sql string, args ...any) {
		t.Helper()
		if err := pool.QueryRow(ctx, sql, args...).Scan(dest); err != nil {
			t.Fatalf("fixture: %v\n%s", err, sql)
		}
	}

	scan(&f.storeA, `INSERT INTO store (name, rating) VALUES ($1, 4.5) RETURNING id`, "TAL73 Tienda A "+tag)
	scan(&f.storeB, `INSERT INTO store (name) VALUES ($1) RETURNING id`, "TAL73 Tienda B "+tag)
	scan(&f.storeInactive, `INSERT INTO store (name, active) VALUES ($1, false) RETURNING id`, "TAL73 Tienda C "+tag)
	scan(&productID, `INSERT INTO product (category_id, name)
		SELECT id, $1 FROM product_category WHERE active ORDER BY id LIMIT 1 RETURNING id`, "TAL73 Producto "+tag)
	scan(&inactiveID, `INSERT INTO product (category_id, name, active)
		SELECT id, $1, false FROM product_category WHERE active ORDER BY id LIMIT 1 RETURNING id`, "TAL73 Inactivo "+tag)
	scan(&f.product, `SELECT public_id FROM product WHERE id = $1`, productID)
	scan(&f.inactiveProduct, `SELECT public_id FROM product WHERE id = $1`, inactiveID)
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM product WHERE id = ANY($1)`, []int32{productID, inactiveID})
		_, _ = pool.Exec(ctx, `DELETE FROM store WHERE id = ANY($1)`, []int32{f.storeA, f.storeB, f.storeInactive})
		_, _ = pool.Exec(ctx, `DELETE FROM user_account WHERE id = $1`, userID)
	})

	offer := `INSERT INTO product_offer (product_id, store_id, price, available, product_url) VALUES ($1, $2, $3, $4, $5)`
	exec(offer, productID, f.storeA, 200, true, "https://example.test/tal73-a-"+tag)
	exec(offer, productID, f.storeB, 100, true, "https://example.test/tal73-b-"+tag)
	exec(offer, productID, f.storeA, 50, false, "https://example.test/tal73-a2-"+tag)
	exec(offer, productID, f.storeInactive, 10, true, "https://example.test/tal73-c-"+tag)
	exec(offer, inactiveID, f.storeA, 1, true, "https://example.test/tal73-inactivo-"+tag)

	scan(&userID, `INSERT INTO user_account (name, email, password_hash) VALUES ($1, $2, 'x') RETURNING id`,
		"TAL73 "+tag, "tal73-"+tag+"@example.test")
	exec(`INSERT INTO store_review (user_id, store_id, rating) VALUES ($1, $2, 5)`, userID, f.storeA)
	return f
}

func storesTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL no definida: se omite el test de integración")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func offerSummary(offers []dto.Offer) []string {
	out := make([]string, 0, len(offers))
	for _, o := range offers {
		store := ""
		if o.Store != nil {
			store = o.Store.ID
		}
		out = append(out, fmt.Sprintf("%s@%s/%v", o.Price, store, o.Available))
	}
	return out
}

func TestStoresIntegration_OfertasDeUnProducto(t *testing.T) {
	pool := storesTestPool(t)
	repo := NewRepository(pool)
	f := newStoreFixture(t, pool)
	ctx := context.Background()
	a, b := strconv.Itoa(int(f.storeA)), strconv.Itoa(int(f.storeB))
	yes, no := true, false

	tests := []struct {
		name      string
		filter    OfferFilter
		want      []string
		wantTotal int64
	}{
		// Disponibles primero y por precio; la tienda inactiva no aparece.
		{"todas", OfferFilter{Page: Page{Limit: 100}},
			[]string{"100.00@" + b + "/true", "200.00@" + a + "/true", "50.00@" + a + "/false"}, 3},
		{"solo disponibles", OfferFilter{Available: &yes, Page: Page{Limit: 100}},
			[]string{"100.00@" + b + "/true", "200.00@" + a + "/true"}, 2},
		{"solo no disponibles", OfferFilter{Available: &no, Page: Page{Limit: 100}},
			[]string{"50.00@" + a + "/false"}, 1},
		{"paginado", OfferFilter{Page: Page{Limit: 1, Offset: 1}}, []string{"200.00@" + a + "/true"}, 3},
		{"fuera de rango", OfferFilter{Page: Page{Limit: 10, Offset: 50}}, []string{}, 3},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			offers, total, err := repo.ListProductOffers(ctx, f.product, tc.filter)
			if err != nil {
				t.Fatal(err)
			}
			if got := offerSummary(offers); fmt.Sprint(got) != fmt.Sprint(tc.want) || total != tc.wantTotal {
				t.Fatalf("ofertas = %v (total %d), quiero %v (total %d)", got, total, tc.want, tc.wantTotal)
			}
		})
	}

	offers, _, err := repo.ListProductOffers(ctx, f.product, OfferFilter{Page: Page{Limit: 100}})
	if err != nil {
		t.Fatal(err)
	}
	storeA := offers[1].Store
	if storeA.Rating != 4.5 || storeA.ReviewCount != 1 || offers[1].Currency != "CLP" ||
		offers[1].ProductID != f.product.String() || offers[1].LastUpdated == "" {
		t.Fatalf("oferta de la tienda A = %+v / %+v", offers[1], *storeA)
	}

	for name, id := range map[string]pgtype.UUID{"producto inactivo": f.inactiveProduct, "inexistente": {Valid: true}} {
		if _, _, err := repo.ListProductOffers(ctx, id, OfferFilter{Page: Page{Limit: 10}}); !errors.Is(err, ErrProductNotFound) {
			t.Fatalf("%s: error = %v, quiero ErrProductNotFound", name, err)
		}
	}
}

func TestStoresIntegration_OfertasDeUnaTienda(t *testing.T) {
	pool := storesTestPool(t)
	repo := NewRepository(pool)
	f := newStoreFixture(t, pool)
	ctx := context.Background()

	offers, total, err := repo.ListStoreOffers(ctx, f.storeA, OfferFilter{Page: Page{Limit: 100}})
	if err != nil {
		t.Fatal(err)
	}
	// El producto inactivo no se lista; la no disponible sí, al final.
	if got := offerSummary(offers); fmt.Sprint(got) != "[200.00@/true 50.00@/false]" || total != 2 {
		t.Fatalf("ofertas = %v (total %d)", got, total)
	}

	for name, id := range map[string]int32{"tienda inactiva": f.storeInactive, "inexistente": 999999999} {
		if _, _, err := repo.ListStoreOffers(ctx, id, OfferFilter{Page: Page{Limit: 10}}); !errors.Is(err, ErrNotFound) {
			t.Fatalf("%s: error = %v, quiero ErrNotFound", name, err)
		}
	}
}

func TestStoresIntegration_ContadoresConsistentes(t *testing.T) {
	pool := storesTestPool(t)
	repo := NewRepository(pool)
	f := newStoreFixture(t, pool)
	ctx := context.Background()

	detail, err := repo.GetDetail(ctx, f.storeA)
	if err != nil {
		t.Fatal(err)
	}
	// productCount: solo el producto activo con oferta disponible (no el inactivo).
	if detail.ProductCount != 1 || detail.ReviewCount != 1 || len(detail.Products) != 1 ||
		detail.Products[0].Price != "200.00" || detail.Rating != 4.5 {
		t.Fatalf("detalle = %+v", detail)
	}

	var active int64
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM store WHERE active`).Scan(&active); err != nil {
		t.Fatal(err)
	}
	var all []dto.Store
	for offset := 0; ; offset += 3 {
		page, total, err := repo.List(ctx, Page{Limit: 3, Offset: offset})
		if err != nil {
			t.Fatal(err)
		}
		if total != active {
			t.Fatalf("total = %d, quiero %d tiendas activas", total, active)
		}
		if len(page) == 0 {
			break
		}
		all = append(all, page...)
	}
	if int64(len(all)) != active {
		t.Fatalf("recorriendo páginas obtuve %d tiendas, quiero %d", len(all), active)
	}
	for _, store := range all {
		if store.ID == strconv.Itoa(int(f.storeInactive)) {
			t.Fatal("una tienda inactiva no debe listarse")
		}
		if store.ID == detail.ID && (store.ProductCount != detail.ProductCount || store.ReviewCount != detail.ReviewCount) {
			t.Fatalf("listado %+v y detalle %+v no coinciden", store, detail.Store)
		}
	}

	if _, err := repo.GetDetail(ctx, f.storeInactive); !errors.Is(err, ErrNotFound) {
		t.Fatalf("tienda inactiva: error = %v, quiero ErrNotFound", err)
	}
}
