package comparison

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Integración TAL-76 contra PostgreSQL con todas las migraciones.
// Se omite sin TEST_DATABASE_URL.

type comparisonFixture struct {
	pool                    *pgxpool.Pool
	repo                    *Repository
	userA, userB            int32
	p1, p2, inactive, other Product
}

func newComparisonFixture(t *testing.T) comparisonFixture {
	t.Helper()
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

	tag := "tal76-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	f := comparisonFixture{pool: pool, repo: NewRepository(pool)}

	// La limpieza se registra antes de crear datos.
	var productIDs, storeIDs, userIDs []int32
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM user_account WHERE id = ANY($1)`, userIDs) // borra sus comparaciones
		_, _ = pool.Exec(ctx, `DELETE FROM product WHERE id = ANY($1)`, productIDs)
		_, _ = pool.Exec(ctx, `DELETE FROM store WHERE id = ANY($1)`, storeIDs)
	})
	must := func(err error, what string) {
		t.Helper()
		if err != nil {
			t.Fatalf("fixture %s: %v", what, err)
		}
	}

	for _, name := range []string{"a", "b"} {
		var id int32
		must(pool.QueryRow(ctx, `INSERT INTO user_account (name, email, password_hash) VALUES ($1, $2, 'x') RETURNING id`,
			"TAL76 "+name, tag+"-"+name+"@example.test").Scan(&id), "usuario")
		userIDs = append(userIDs, id)
	}
	f.userA, f.userB = userIDs[0], userIDs[1]

	product := func(name, sql string, active bool) Product {
		var p Product
		defer func() { productIDs = append(productIDs, p.ID) }()
		must(pool.QueryRow(ctx, sql, name+" "+tag, active).Scan(&p.ID, &p.PublicID, &p.CategoryID), name)
		return p
	}
	const inRefrigeradores = `INSERT INTO product (category_id, category_ref, name, active)
		SELECT c.legacy_product_category_id, c.id, $1, $2 FROM categories c WHERE c.slug = 'refrigeradores'
		RETURNING id, public_id::text, category_id`
	f.p1 = product("Refri Uno", inRefrigeradores, true)
	f.p2 = product("Refri Dos", inRefrigeradores, true)
	f.inactive = product("Refri Inactivo", inRefrigeradores, false)
	f.other = product("Lavadora", `INSERT INTO product (category_id, name, active)
		SELECT id, $1, $2 FROM product_category WHERE name = 'Lavado' RETURNING id, public_id::text, category_id`, true)

	attr := func(p Product, key, value string, numeric *string) {
		_, err := pool.Exec(ctx, `
			INSERT INTO product_attribute_value (product_id, category_id, attribute_id, value, value_numeric)
			SELECT $1, a.category_id, a.id, $3, $4::numeric
			FROM category_attributes a JOIN categories c ON c.id = a.category_id
			WHERE c.slug = 'refrigeradores' AND a.key = $2`, p.ID, key, value, numeric)
		must(err, "atributo "+key)
	}
	n350 := "350"
	attr(f.p1, "capacidad_litros", "350", &n350)
	attr(f.p1, "eficiencia_energetica", "A+", nil)
	attr(f.p1, "color", "Gris", nil) // no comparable: no debe aparecer
	attr(f.p2, "capacidad_litros", "350", &n350)
	attr(f.p2, "no_frost", "true", nil)

	spec := func(p Product, value string) {
		_, err := pool.Exec(ctx, `
			INSERT INTO product_specification_value (product_id, specification_id, value)
			SELECT $1, s.id, $2 FROM product_category_specification s
			WHERE s.category_id = $3 AND s.name = 'Capacidad' AND s.comparable`, p.ID, value, p.CategoryID)
		must(err, "especificación")
	}
	spec(f.p1, "350")
	spec(f.p2, "400")

	var storeA, storeB int32
	must(pool.QueryRow(ctx, `INSERT INTO store (name) VALUES ($1) RETURNING id`, "TAL76 A "+tag).Scan(&storeA), "tienda")
	storeIDs = append(storeIDs, storeA)
	must(pool.QueryRow(ctx, `INSERT INTO store (name) VALUES ($1) RETURNING id`, "TAL76 B "+tag).Scan(&storeB), "tienda")
	storeIDs = append(storeIDs, storeB)
	offer := func(storeID int32, price, shipping int, free bool) {
		_, err := pool.Exec(ctx, `INSERT INTO product_offer (product_id, store_id, price, shipping_cost, shipping_free, product_url)
			VALUES ($1, $2, $3, $4, $5, $6)`, f.p1.ID, storeID, price, shipping, free, fmt.Sprintf("https://example.test/%s-%d", tag, storeID))
		must(err, "oferta")
	}
	offer(storeA, 100000, 20000, false) // total 120000
	offer(storeB, 110000, 5000, true)   // total 110000 → mejor
	return f
}

func TestComparisonIntegration_Matriz(t *testing.T) {
	f := newComparisonFixture(t)

	matrix, err := f.repo.BuildMatrix(context.Background(), []int32{f.p2.ID, f.p1.ID})
	if err != nil {
		t.Fatal(err)
	}
	if matrix.Category == nil || *matrix.Category.Slug != "refrigeradores" || matrix.Category.Name != "Refrigeradores" {
		t.Fatalf("categoría = %+v", matrix.Category)
	}
	two, one := matrix.Products[0], matrix.Products[1]
	if two.ID != f.p2.PublicID || one.ID != f.p1.PublicID {
		t.Fatal("las columnas deben respetar el orden pedido")
	}
	if two.BestPrice != nil || two.OfferCount != 0 ||
		*one.BestPrice != "110000.00" || *one.BestStore == "" || one.OfferCount != 2 {
		t.Fatalf("columnas = %+v / %+v", two, one)
	}

	got := []string{}
	for _, row := range matrix.Attributes {
		got = append(got, fmt.Sprintf("%s=%s,%s/%v", row.Key, row.Values[0], row.Values[1], row.Differs))
	}
	// Solo comparables con algún valor (color no es comparable; peso_kg no tiene valores).
	want := []string{
		"capacidad_litros=350,350/false",
		`eficiencia_energetica=null,"A+"/true`,
		"no_frost=true,null/true",
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("atributos = %v, quiero %v", got, want)
	}

	if len(matrix.Specifications) != 1 {
		t.Fatalf("especificaciones = %+v", matrix.Specifications)
	}
	spec := matrix.Specifications[0]
	if spec.Name != "Capacidad" || *spec.Values[0] != "400" || *spec.Values[1] != "350" || !spec.Differs {
		t.Fatalf("especificación = %+v", spec)
	}

	empty, err := f.repo.BuildMatrix(context.Background(), nil)
	if err != nil || empty.Category != nil || len(empty.Products) != 0 {
		t.Fatalf("matriz vacía = %+v (%v)", empty, err)
	}
}

func TestComparisonIntegration_Compatibles(t *testing.T) {
	f := newComparisonFixture(t)
	ctx := context.Background()

	var want int64
	if err := f.pool.QueryRow(ctx, `SELECT count(*) - 1 FROM product WHERE category_id = $1 AND active`,
		f.p1.CategoryID).Scan(&want); err != nil {
		t.Fatal(err)
	}
	summaries, total, err := f.repo.Compatible(ctx, f.p1, 100, 0)
	if err != nil || total != want || int64(len(summaries)) != want {
		t.Fatalf("compatibles = %d (total %d), quiero %d (%v)", len(summaries), total, want, err)
	}
	seen := map[string]bool{}
	for _, s := range summaries {
		seen[s.ID] = true
	}
	if !seen[f.p2.PublicID] || seen[f.p1.PublicID] || seen[f.inactive.PublicID] || seen[f.other.PublicID] {
		t.Fatalf("compatibles incluye/excluye mal: %v", seen)
	}

	products, err := f.repo.ResolveProducts(ctx, []string{f.p1.PublicID, f.inactive.PublicID})
	if !errors.Is(err, ErrProductNotFound) || products != nil {
		t.Fatalf("un producto inactivo no se resuelve: %v", err)
	}
}

func TestComparisonIntegration_GuardadasYPropiedad(t *testing.T) {
	f := newComparisonFixture(t)
	ctx := context.Background()

	id, err := f.repo.Create(ctx, f.userA, "Refris de Ana", []Product{f.p1, f.p2})
	if err != nil {
		t.Fatal(err)
	}
	header, err := f.repo.Get(ctx, f.userA, id)
	if err != nil || header.Name != "Refris de Ana" || fmt.Sprint(header.ProductIDs) != fmt.Sprint([]int32{f.p1.ID, f.p2.ID}) {
		t.Fatalf("Get = %+v (%v)", header, err)
	}

	// El usuario B no puede ver, renombrar, borrar ni modificar la de A.
	if _, err := f.repo.Get(ctx, f.userB, id); !errors.Is(err, ErrComparisonNotFound) {
		t.Fatalf("B ve la comparación de A: %v", err)
	}
	if err := f.repo.Rename(ctx, f.userB, id, "x"); !errors.Is(err, ErrComparisonNotFound) {
		t.Fatalf("B renombra la comparación de A: %v", err)
	}
	if err := f.repo.AddItem(ctx, f.userB, id, f.p2); !errors.Is(err, ErrComparisonNotFound) {
		t.Fatalf("B agrega a la comparación de A: %v", err)
	}
	if err := f.repo.RemoveItem(ctx, f.userB, id, f.p1.ID); !errors.Is(err, ErrComparisonNotFound) {
		t.Fatalf("B quita de la comparación de A: %v", err)
	}
	if err := f.repo.Delete(ctx, f.userB, id); !errors.Is(err, ErrComparisonNotFound) {
		t.Fatalf("B borra la comparación de A: %v", err)
	}
	if list, total, err := f.repo.List(ctx, f.userB, 100, 0); err != nil || total != 0 || len(list) != 0 {
		t.Fatalf("listado de B = %+v (%d, %v)", list, total, err)
	}
	if list, total, err := f.repo.List(ctx, f.userA, 100, 0); err != nil || total != 1 || list[0].ProductCount != 2 || list[0].Category != "Refrigeradores" {
		t.Fatalf("listado de A = %+v (%d, %v)", list, total, err)
	}

	// Reglas al agregar.
	if err := f.repo.AddItem(ctx, f.userA, id, f.other); !errors.Is(err, ErrIncompatibleCategory) {
		t.Fatalf("otra categoría = %v", err)
	}
	if err := f.repo.AddItem(ctx, f.userA, id, f.p1); !errors.Is(err, ErrAlreadyIncluded) {
		t.Fatalf("repetido = %v", err)
	}
	var extra []int32
	if err := f.pool.QueryRow(ctx, `SELECT array_agg(id ORDER BY id) FROM product
		WHERE category_id = $1 AND active AND id <> ALL($2)`, f.p1.CategoryID, []int32{f.p1.ID, f.p2.ID}).Scan(&extra); err != nil || len(extra) < 4 {
		t.Fatalf("faltan productos de la categoría para llenar la comparación: %v %v", extra, err)
	}
	for _, pid := range extra[:3] {
		if err := f.repo.AddItem(ctx, f.userA, id, Product{ID: pid, CategoryID: f.p1.CategoryID}); err != nil {
			t.Fatalf("agregar %d: %v", pid, err)
		}
	}
	if err := f.repo.AddItem(ctx, f.userA, id, Product{ID: extra[3], CategoryID: f.p1.CategoryID}); !errors.Is(err, ErrComparisonFull) {
		t.Fatalf("sexto producto = %v, quiero ErrComparisonFull", err)
	}

	// Quitar y orden.
	if err := f.repo.RemoveItem(ctx, f.userA, id, f.other.ID); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("quitar uno que no está = %v", err)
	}
	if err := f.repo.RemoveItem(ctx, f.userA, id, f.p1.ID); err != nil {
		t.Fatal(err)
	}
	header, _ = f.repo.Get(ctx, f.userA, id)
	if len(header.ProductIDs) != 4 || header.ProductIDs[0] != f.p2.ID || header.ProductIDs[3] != extra[2] {
		t.Fatalf("productos tras quitar = %v", header.ProductIDs)
	}

	if err := f.repo.Rename(ctx, f.userA, id, "Nuevo"); err != nil {
		t.Fatal(err)
	}
	if err := f.repo.Delete(ctx, f.userA, id); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.Get(ctx, f.userA, id); !errors.Is(err, ErrComparisonNotFound) {
		t.Fatalf("tras borrar = %v", err)
	}
}
