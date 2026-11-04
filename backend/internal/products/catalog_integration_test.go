package products

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/categories"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// Integración TAL-70 contra PostgreSQL con todas las migraciones aplicadas
// (incluida 014_product_attributes.sql). Se omiten sin TEST_DATABASE_URL.

func resolveAndValidate(t *testing.T, repo *Repository, slug, attributes string) (WriteCategory, []categories.Value) {
	t.Helper()
	category, err := repo.ResolveCategory(context.Background(), CategoryRef{Slug: slug})
	if err != nil {
		t.Fatalf("ResolveCategory(%s): %v", slug, err)
	}
	var input map[string]json.RawMessage
	if err := json.Unmarshal([]byte(attributes), &input); err != nil {
		t.Fatal(err)
	}
	values, fieldErrors := categories.ValidateAttributes(category.Attributes, input)
	if len(fieldErrors) > 0 {
		t.Fatalf("atributos de prueba inválidos: %+v", fieldErrors)
	}
	return category, values
}

func uniqueSKU(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func TestCatalogIntegration_CicloDeVidaConAtributos(t *testing.T) {
	pool := repositoryTestPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	fridge, fridgeValues := resolveAndValidate(t, repo, "refrigeradores",
		`{"capacidad_litros": 350, "eficiencia_energetica": "A+", "no_frost": true, "peso_kg": 65.5, "color": "Gris"}`)
	if fridge.ID == nil || fridge.LegacyID == nil || len(fridge.Attributes) != 5 {
		t.Fatalf("categoría resuelta = %+v", fridge)
	}

	publicID, err := repo.Create(ctx, ProductWrite{
		LegacyCategoryID: *fridge.LegacyID, CategoryRef: fridge.ID,
		Name: "Refrigerador TAL-70", SKU: uniqueSKU("tal70"),
		ImageURL: "https://example.test/tal70.png", Attributes: fridgeValues,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM product WHERE public_id = $1`, publicID)
	})

	detail, err := repo.GetDetail(ctx, publicID)
	if err != nil {
		t.Fatalf("GetDetail: %v", err)
	}
	if detail.CategorySlug == nil || *detail.CategorySlug != "refrigeradores" || len(detail.Images) != 1 {
		t.Fatalf("detalle = %+v", detail)
	}
	encoded, _ := json.Marshal(detail.Attributes)
	wantAttributes := `{"capacidad_litros":350,"color":"Gris","eficiencia_energetica":"A+","no_frost":true,"peso_kg":65.5}`
	if string(encoded) != wantAttributes {
		t.Fatalf("attributes = %s, quiero %s", encoded, wantAttributes)
	}

	// value_numeric solo en integer/decimal, para los filtros de TAL-75.
	var numericCount, totalCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(value_numeric), count(*) FROM product_attribute_value pav
		JOIN product p ON p.id = pav.product_id WHERE p.public_id = $1`, publicID).
		Scan(&numericCount, &totalCount); err != nil {
		t.Fatal(err)
	}
	if numericCount != 2 || totalCount != 5 {
		t.Fatalf("value_numeric = %d de %d, quiero 2 de 5", numericCount, totalCount)
	}

	// Cambio de categoría: los valores anteriores se reemplazan por completo.
	notebook, notebookValues := resolveAndValidate(t, repo, "notebooks",
		`{"memoria_ram_gb": 16, "almacenamiento_gb": 512, "pantalla_pulgadas": 14}`)
	if err := repo.Update(ctx, publicID, ProductWrite{
		LegacyCategoryID: *notebook.LegacyID, CategoryRef: notebook.ID,
		Name: "Notebook TAL-70", Attributes: notebookValues,
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	detail, err = repo.GetDetail(ctx, publicID)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ = json.Marshal(detail.Attributes)
	if *detail.CategorySlug != "notebooks" || detail.Name != "Notebook TAL-70" ||
		string(encoded) != `{"almacenamiento_gb":512,"memoria_ram_gb":16,"pantalla_pulgadas":14}` {
		t.Fatalf("detalle tras Update = %s %s %s", *detail.CategorySlug, detail.Name, encoded)
	}

	// Filtro por slug nuevo.
	items, total, err := repo.List(ctx, ListParams{CategorySlug: strp("notebooks"), Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range items {
		if item.ID == detail.ID {
			found = true
			if string(item.Attributes["memoria_ram_gb"]) != "16" || *item.CategorySlug != "notebooks" {
				t.Fatalf("item del listado = %+v", item)
			}
		}
	}
	if !found || total != int64(len(items)) {
		t.Fatalf("el listado por slug debe incluir el producto (total=%d, items=%d)", total, len(items))
	}

	// Baja lógica.
	if err := repo.Deactivate(ctx, publicID); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}
	if _, err := repo.GetDetail(ctx, publicID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetDetail tras la baja: %v, quiero ErrNotFound", err)
	}
	if err := repo.Deactivate(ctx, publicID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("segunda baja: %v, quiero ErrNotFound", err)
	}
	if err := repo.Update(ctx, publicID, ProductWrite{LegacyCategoryID: *notebook.LegacyID, Name: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Update tras la baja: %v, quiero ErrNotFound", err)
	}
}

func TestCatalogIntegration_ResolveCategory(t *testing.T) {
	pool := repositoryTestPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	var lavadoID int32
	if err := pool.QueryRow(ctx, `SELECT id FROM product_category WHERE name = 'Lavado'`).Scan(&lavadoID); err != nil {
		t.Fatal(err)
	}
	lavado, err := repo.ResolveCategory(ctx, CategoryRef{LegacyID: lavadoID})
	if err != nil || lavado.ID != nil || lavado.LegacyID == nil || *lavado.LegacyID != lavadoID ||
		lavado.ItemType != categories.ItemTypeProduct || len(lavado.Attributes) != 0 {
		t.Fatalf("categoría antigua sin equivalente = %+v (%v)", lavado, err)
	}

	var refrigeracionID int32
	if err := pool.QueryRow(ctx, `SELECT id FROM product_category WHERE name = 'Refrigeracion'`).Scan(&refrigeracionID); err != nil {
		t.Fatal(err)
	}
	fridge, err := repo.ResolveCategory(ctx, CategoryRef{LegacyID: refrigeracionID})
	if err != nil || fridge.Slug == nil || *fridge.Slug != "refrigeradores" || len(fridge.Attributes) != 5 {
		t.Fatalf("categoría antigua enlazada = %+v (%v)", fridge, err)
	}

	service, err := repo.ResolveCategory(ctx, CategoryRef{Slug: "internet-fibra-optica"})
	if err != nil || service.ItemType != categories.ItemTypeService || service.LegacyID != nil {
		t.Fatalf("categoría de servicio = %+v (%v)", service, err)
	}

	for _, ref := range []CategoryRef{{Slug: "no-existe"}, {LegacyID: 999999}} {
		if _, err := repo.ResolveCategory(ctx, ref); !errors.Is(err, ErrCategoryNotFound) {
			t.Fatalf("ResolveCategory(%+v) = %v, quiero ErrCategoryNotFound", ref, err)
		}
	}
}

func TestCatalogIntegration_ListPaginaYFiltroAntiguo(t *testing.T) {
	pool := repositoryTestPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	var active int64
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM product WHERE active`).Scan(&active); err != nil {
		t.Fatal(err)
	}

	first, total, err := repo.List(ctx, ListParams{Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	second, total2, err := repo.List(ctx, ListParams{Limit: 5, Offset: 5})
	if err != nil {
		t.Fatal(err)
	}
	if total != active || total2 != active || len(first) != 5 || len(second) == 0 {
		t.Fatalf("total = %d/%d (activos %d), página 1 = %d, página 2 = %d", total, total2, active, len(first), len(second))
	}
	seen := map[string]bool{}
	for _, item := range append(first, second...) {
		if seen[item.ID] {
			t.Fatalf("producto repetido entre páginas: %s", item.ID)
		}
		seen[item.ID] = true
		if item.Specs == nil || item.Images == nil || item.Offers == nil || item.Attributes == nil {
			t.Fatalf("colecciones deben ser vacías, no null: %+v", item)
		}
	}

	// El filtro antiguo incluye subcategorías: Computacion → Computadores.
	var computacionID int32
	if err := pool.QueryRow(ctx, `SELECT id FROM product_category WHERE name = 'Computacion'`).Scan(&computacionID); err != nil {
		t.Fatal(err)
	}
	var wantSubtree int64
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM product p JOIN product_category c ON c.id = p.category_id
		WHERE p.active AND (c.id = $1 OR c.parent_category_id = $1)`, computacionID).Scan(&wantSubtree); err != nil {
		t.Fatal(err)
	}
	items, total, err := repo.List(ctx, ListParams{LegacyCategoryID: &computacionID, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if total != wantSubtree || int64(len(items)) != wantSubtree || wantSubtree == 0 {
		t.Fatalf("filtro Computacion = %d (total %d), quiero %d", len(items), total, wantSubtree)
	}

	// Página fuera de rango: arreglo vacío, total real.
	empty, total, err := repo.List(ctx, ListParams{Limit: 10, Offset: 100000})
	if err != nil || len(empty) != 0 || total != active {
		t.Fatalf("página vacía = %d items, total %d (%v)", len(empty), total, err)
	}
}

func TestCatalogIntegration_BaseDeDatosProtegeLosAtributos(t *testing.T) {
	pool := repositoryTestPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	fridge, values := resolveAndValidate(t, repo, "refrigeradores", `{"capacidad_litros": 200}`)
	sku := uniqueSKU("tal70-fk")
	publicID, err := repo.Create(ctx, ProductWrite{LegacyCategoryID: *fridge.LegacyID, CategoryRef: fridge.ID,
		Name: "FK TAL-70", SKU: sku, Attributes: values})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM product WHERE public_id = $1`, publicID)
	})

	expectCode := func(t *testing.T, err error, code string) {
		t.Helper()
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != code {
			t.Fatalf("error = %v, quiero SQLSTATE %s", err, code)
		}
	}

	t.Run("sku duplicado", func(t *testing.T) {
		_, err := repo.Create(ctx, ProductWrite{LegacyCategoryID: *fridge.LegacyID, Name: "dup", SKU: sku})
		expectCode(t, err, "23505")
	})

	t.Run("atributo de otra categoría", func(t *testing.T) {
		_, err := pool.Exec(ctx, `
			INSERT INTO product_attribute_value (product_id, category_id, attribute_id, value)
			SELECT p.id, p.category_ref, a.id, '16'
			FROM product p, category_attributes a JOIN categories c ON c.id = a.category_id
			WHERE p.public_id = $1 AND c.slug = 'notebooks' AND a.key = 'memoria_ram_gb'`, publicID)
		expectCode(t, err, "23503")
	})

	t.Run("categoría distinta a la del producto", func(t *testing.T) {
		_, err := pool.Exec(ctx, `
			INSERT INTO product_attribute_value (product_id, category_id, attribute_id, value)
			SELECT p.id, a.category_id, a.id, '16'
			FROM product p, category_attributes a JOIN categories c ON c.id = a.category_id
			WHERE p.public_id = $1 AND c.slug = 'notebooks' AND a.key = 'memoria_ram_gb'`, publicID)
		expectCode(t, err, "23503")
	})

	t.Run("no se puede cambiar la categoría con valores guardados", func(t *testing.T) {
		_, err := pool.Exec(ctx, `
			UPDATE product SET category_ref = (SELECT id FROM categories WHERE slug = 'notebooks')
			WHERE public_id = $1`, publicID)
		expectCode(t, err, "23503")
	})

	t.Run("no se puede borrar un atributo con valores", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		_, err = tx.Exec(ctx, `DELETE FROM category_attributes WHERE id = $1`, values[0].AttributeID)
		expectCode(t, err, "23001") // restrict_violation (ON DELETE RESTRICT)
	})

	t.Run("valor vacío", func(t *testing.T) {
		_, err := pool.Exec(ctx, `
			UPDATE product_attribute_value SET value = '  '
			WHERE product_id = (SELECT id FROM product WHERE public_id = $1)`, publicID)
		expectCode(t, err, "23514")
	})

	var unknown pgtype.UUID
	_ = unknown.Scan("00000000-0000-0000-0000-000000000000")
	if _, err := repo.GetDetail(ctx, unknown); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetDetail inexistente = %v", err)
	}
}
