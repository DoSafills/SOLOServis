package search

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Integración TAL-75 contra PostgreSQL con todas las migraciones. Cada test
// crea productos con una etiqueta única (tal75-<n>) y la incluye en q, así
// los datos de seed no interfieren. Se omite sin TEST_DATABASE_URL.

type searchFixture struct {
	repo           *Repository
	tag            string
	brandA, brandB int32
	refrigeradores string
}

func newSearchFixture(t *testing.T) searchFixture {
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

	f := searchFixture{repo: NewRepository(pool), tag: "tal75-" + strconv.FormatInt(time.Now().UnixNano(), 10),
		refrigeradores: "refrigeradores"}
	must := func(err error, what string) {
		t.Helper()
		if err != nil {
			t.Fatalf("fixture %s: %v", what, err)
		}
	}
	must(pool.QueryRow(ctx, `INSERT INTO brand (name) VALUES ($1) RETURNING id`, "Ñandú "+f.tag).Scan(&f.brandA), "marca A")
	must(pool.QueryRow(ctx, `INSERT INTO brand (name) VALUES ($1) RETURNING id`, "Polaris "+f.tag).Scan(&f.brandB), "marca B")
	var storeA, storeB int32
	must(pool.QueryRow(ctx, `INSERT INTO store (name) VALUES ($1) RETURNING id`, "TAL75 A "+f.tag).Scan(&storeA), "tienda A")
	must(pool.QueryRow(ctx, `INSERT INTO store (name) VALUES ($1) RETURNING id`, "TAL75 B "+f.tag).Scan(&storeB), "tienda B")

	var productIDs []int32
	product := func(name string, brand int32, active bool, attrs map[string]string) int32 {
		var id int32
		must(pool.QueryRow(ctx, `
			INSERT INTO product (category_id, category_ref, brand_id, name, active)
			SELECT c.legacy_product_category_id, c.id, $1, $2, $3 FROM categories c WHERE c.slug = 'refrigeradores'
			RETURNING id`, brand, name+" "+f.tag, active).Scan(&id), name)
		productIDs = append(productIDs, id)
		for key, value := range attrs {
			var numeric *string // value_numeric solo para integer/decimal, como en TAL-70
			if key == "capacidad_litros" || key == "peso_kg" {
				numeric = &value
			}
			_, err := pool.Exec(ctx, `
				INSERT INTO product_attribute_value (product_id, category_id, attribute_id, value, value_numeric)
				SELECT $1, a.category_id, a.id, $3, $4::numeric
				FROM category_attributes a JOIN categories c ON c.id = a.category_id
				WHERE c.slug = 'refrigeradores' AND a.key = $2`, id, key, value, numeric)
			must(err, "atributo "+key)
		}
		return id
	}
	offer := func(productID, storeID int32, price, shipping int, free, available bool) {
		_, err := pool.Exec(ctx, `INSERT INTO product_offer (product_id, store_id, price, shipping_cost, shipping_free, available, product_url)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`, productID, storeID, price, shipping, free, available,
			fmt.Sprintf("https://example.test/%s-%d-%d", f.tag, productID, storeID))
		must(err, "oferta")
	}

	artico := product("Refrigerador Ártico", f.brandA, true,
		map[string]string{"capacidad_litros": "350", "eficiencia_energetica": "A+", "no_frost": "true", "color": "Gris"})
	polar := product("Refrigerador Polar", f.brandB, true,
		map[string]string{"capacidad_litros": "600", "eficiencia_energetica": "A++", "no_frost": "false"})
	product("Refrigerador Glaciar", f.brandA, true,
		map[string]string{"capacidad_litros": "200", "eficiencia_energetica": "A+", "no_frost": "true"})
	inactive := product("Refrigerador Inactivo", f.brandA, false, map[string]string{"capacidad_litros": "400"})

	offer(artico, storeA, 300000, 10000, false, true) // total 310000
	offer(artico, storeB, 320000, 5000, true, true)   // total 320000 (envío gratis)
	offer(polar, storeA, 500000, 0, false, true)
	offer(polar, storeB, 100, 0, false, false) // no disponible: no cuenta
	offer(inactive, storeA, 1, 0, false, true)

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM product WHERE id = ANY($1)`, productIDs)
		_, _ = pool.Exec(ctx, `DELETE FROM store WHERE id = ANY($1)`, []int32{storeA, storeB})
		_, _ = pool.Exec(ctx, `DELETE FROM brand WHERE id = ANY($1)`, []int32{f.brandA, f.brandB})
	})
	return f
}

// search ejecuta la búsqueda acotada a los productos del fixture.
func (f searchFixture) search(t *testing.T, q Query, words ...string) ([]ProductResult, int64) {
	t.Helper()
	q.Words = append(words, f.tag)
	if q.Limit == 0 {
		q.Limit = 100
	}
	if q.Sort == "" {
		q.Sort = "name"
	}
	results, total, err := f.repo.SearchProducts(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	return results, total
}

func names(results []ProductResult, tag string) []string {
	out := []string{}
	for _, r := range results {
		out = append(out, strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(r.Name, tag), "Refrigerador ")))
	}
	return out
}

func (f searchFixture) attrFilters(t *testing.T, filters map[string][2]string) []AttributeFilter {
	t.Helper()
	defs, err := f.repo.CategoryDefinitions(context.Background(), f.refrigeradores)
	if err != nil {
		t.Fatal(err)
	}
	out := []AttributeFilter{}
	for key, opValue := range filters {
		for _, d := range defs {
			if d.Key == key {
				out = append(out, AttributeFilter{AttributeID: d.ID, Op: opValue[0], Value: opValue[1]})
			}
		}
	}
	return out
}

func TestSearchIntegration_Texto(t *testing.T) {
	f := newSearchFixture(t)

	tests := []struct {
		name  string
		words []string
		want  string
	}{
		{"sin tildes ni mayúsculas", []string{"ARTICO"}, "[Ártico]"},
		{"con tildes", []string{"ártico"}, "[Ártico]"},
		{"marca con ñ", []string{"nandu"}, "[Ártico Glaciar]"},
		{"todas las palabras deben aparecer", []string{"polaris", "polar"}, "[Polar]"},
		{"comodín literal", []string{"%"}, "[]"},
		{"guion bajo literal", []string{"_"}, "[]"},
		{"solo la etiqueta: el inactivo no aparece", nil, "[Ártico Glaciar Polar]"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results, total := f.search(t, Query{}, tc.words...)
			if got := fmt.Sprint(names(results, f.tag)); got != tc.want || total != int64(len(results)) {
				t.Fatalf("resultados = %s (total %d), quiero %s", got, total, tc.want)
			}
		})
	}
}

func TestSearchIntegration_FiltrosYOrden(t *testing.T) {
	f := newSearchFixture(t)
	min := "310000"
	max := "400000"
	slug := f.refrigeradores

	tests := []struct {
		name string
		q    Query
		want string
	}{
		{"marca", Query{BrandIDs: []int32{f.brandA}}, "[Ártico Glaciar]"},
		{"rango de precio total", Query{MinPrice: &min, MaxPrice: &max}, "[Ártico]"},
		{"solo disponibles", Query{Available: true}, "[Ártico Polar]"},
		{"capacidad mínima", Query{CategorySlug: &slug, Attributes: f.attrFilters(t, map[string][2]string{
			"capacidad_litros": {OpMin, "300"}})}, "[Ártico Polar]"},
		{"capacidad exacta", Query{CategorySlug: &slug, Attributes: f.attrFilters(t, map[string][2]string{
			"capacidad_litros": {OpEqualNumeric, "350"}})}, "[Ártico]"},
		{"enum", Query{CategorySlug: &slug, Attributes: f.attrFilters(t, map[string][2]string{
			"eficiencia_energetica": {OpEqual, "A++"}})}, "[Polar]"},
		{"booleano y rango", Query{CategorySlug: &slug, Attributes: f.attrFilters(t, map[string][2]string{
			"no_frost": {OpEqual, "true"}, "capacidad_litros": {OpMax, "400"}})}, "[Ártico Glaciar]"},
		{"texto sin mayúsculas", Query{CategorySlug: &slug, Attributes: f.attrFilters(t, map[string][2]string{
			"color": {OpEqualInsensitive, "gris"}})}, "[Ártico]"},
		{"precio ascendente, sin precio al final", Query{Sort: "price_asc"}, "[Ártico Polar Glaciar]"},
		{"precio descendente, sin precio al final", Query{Sort: "price_desc"}, "[Polar Ártico Glaciar]"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results, _ := f.search(t, tc.q)
			if got := fmt.Sprint(names(results, f.tag)); got != tc.want {
				t.Fatalf("resultados = %s, quiero %s", got, tc.want)
			}
		})
	}

	page, total := f.search(t, Query{Limit: 1, Offset: 1})
	// Orden por nombre con la collation de la BD: "Ártico" va antes que "Glaciar".
	if total != 3 || fmt.Sprint(names(page, f.tag)) != "[Glaciar]" {
		t.Fatalf("página 2 = %v (total %d)", names(page, f.tag), total)
	}

	results, _ := f.search(t, Query{}, "artico")
	r := results[0]
	encoded, _ := json.Marshal(r.Attributes)
	if *r.BestPrice != "310000.00" || r.OfferCount != 2 || *r.CategorySlug != "refrigeradores" ||
		r.Brand != "Ñandú "+f.tag || string(encoded) != `{"capacidad_litros":350,"color":"Gris","eficiencia_energetica":"A+","no_frost":true}` {
		t.Fatalf("resultado = %+v attrs = %s", r, encoded)
	}
}

func TestSearchIntegration_Facetas(t *testing.T) {
	f := newSearchFixture(t)
	slug := f.refrigeradores
	defs, err := f.repo.CategoryDefinitions(context.Background(), slug)
	if err != nil {
		t.Fatal(err)
	}

	facets, err := f.repo.SearchFacets(context.Background(), Query{Words: []string{f.tag}, CategorySlug: &slug}, defs)
	if err != nil {
		t.Fatal(err)
	}
	if facets.Total != 3 || *facets.Price.Min != "310000.00" || *facets.Price.Max != "500000.00" {
		t.Fatalf("total/precio = %d %v %v", facets.Total, *facets.Price.Min, *facets.Price.Max)
	}
	if fmt.Sprintf("%d:%d %d:%d", facets.Brands[0].ID, facets.Brands[0].Count, facets.Brands[1].ID, facets.Brands[1].Count) !=
		fmt.Sprintf("%d:2 %d:1", f.brandA, f.brandB) {
		t.Fatalf("marcas = %+v", facets.Brands)
	}

	byKey := map[string]AttributeFacet{}
	for _, a := range facets.Attributes {
		byKey[a.Key] = a
	}
	if _, ok := byKey["peso_kg"]; ok {
		t.Fatal("peso_kg no es filtrable y no debe aparecer")
	}
	if c := byKey["capacidad_litros"]; c.Min == nil || *c.Min != 200 || *c.Max != 600 || len(c.Values) != 0 {
		t.Fatalf("capacidad = %+v", c)
	}
	values := func(key string) string {
		out := []string{}
		for _, v := range byKey[key].Values {
			out = append(out, fmt.Sprintf("%s:%d", v.Value, v.Count))
		}
		return fmt.Sprint(out)
	}
	if got := values("eficiencia_energetica"); got != `["A+":2 "A++":1]` {
		t.Fatalf("eficiencia = %s", got)
	}
	if got := values("no_frost"); got != "[true:2 false:1]" {
		t.Fatalf("no_frost = %s", got)
	}

	// Los filtros también acotan las facetas.
	filtered, err := f.repo.SearchFacets(context.Background(), Query{Words: []string{f.tag}, CategorySlug: &slug,
		Available: true}, defs)
	if err != nil || filtered.Total != 2 {
		t.Fatalf("facetas con available = %+v (%v)", filtered, err)
	}
}

func TestSearchIntegration_CategoriasYMarcas(t *testing.T) {
	f := newSearchFixture(t)
	ctx := context.Background()
	service := "service"

	for _, tc := range []struct {
		text     string
		itemType *string
		want     string
	}{
		{"REFRIGERADORES", nil, "[refrigeradores]"},
		{"optica", nil, "[internet-fibra-optica]"},
		{"", &service, "[internet-fibra-optica]"},
	} {
		results, err := f.repo.SearchCategories(ctx, tc.text, tc.itemType)
		if err != nil {
			t.Fatal(err)
		}
		slugs := []string{}
		for _, c := range results {
			slugs = append(slugs, c.Slug)
		}
		if fmt.Sprint(slugs) != tc.want {
			t.Fatalf("categorías(%q) = %v, quiero %s", tc.text, slugs, tc.want)
		}
	}

	brands, err := f.repo.SearchBrands(ctx, "nandu "+f.tag)
	if err != nil || len(brands) != 1 || brands[0].ID != f.brandA {
		t.Fatalf("marcas = %+v (%v)", brands, err)
	}
}
