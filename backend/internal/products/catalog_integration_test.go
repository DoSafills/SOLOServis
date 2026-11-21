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
	if fridge.ID == nil || fridge.CategoryID == nil || len(fridge.Attributes) != 5 {
		t.Fatalf("categoría resuelta = %+v", fridge)
	}

	publicID, err := repo.Create(ctx, ProductWrite{
		CategoryID: *fridge.CategoryID, CategoryRef: fridge.ID,
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
		CategoryID: *notebook.CategoryID, CategoryRef: notebook.ID,
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

	// Filtro por la hoja del árbol (nivel 3) donde quedó el producto.
	items, total, err := repo.List(ctx, ListParams{CategoryIDs: []int32{*notebook.CategoryID}, Limit: 100})
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
	if err := repo.Update(ctx, publicID, ProductWrite{CategoryID: *notebook.CategoryID,
		Name: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Update tras la baja: %v, quiero ErrNotFound", err)
	}
}

func TestCatalogIntegration_ResolveCategory(t *testing.T) {
	pool := repositoryTestPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	// Una hoja activa sin fila en categories se puede usar: el producto guarda
	// solo el árbol y no acepta atributos.
	var sinFilaID int32
	if err := pool.QueryRow(ctx, `
		SELECT pc.id FROM product_category pc
		WHERE pc.active AND pc.level = 3
		  AND NOT EXISTS (SELECT 1 FROM categories c WHERE c.legacy_product_category_id = pc.id)
		  AND NOT EXISTS (SELECT 1 FROM product_category child WHERE child.parent_category_id = pc.id AND child.active)
		ORDER BY pc.id LIMIT 1`).Scan(&sinFilaID); err != nil {
		t.Fatal(err)
	}
	sinFila, err := repo.ResolveCategory(ctx, CategoryRef{ID: sinFilaID})
	if err != nil || sinFila.ID != nil || sinFila.CategoryID == nil || *sinFila.CategoryID != sinFilaID ||
		sinFila.ItemType != categories.ItemTypeProduct || len(sinFila.Attributes) != 0 {
		t.Fatalf("hoja sin fila en categories = %+v (%v)", sinFila, err)
	}

	var refrigeradoresID int32
	if err := pool.QueryRow(ctx, `
		SELECT child.id
		FROM product_category child
		JOIN product_category parent ON parent.id = child.parent_category_id
		WHERE child.name = 'Refrigeradores' AND parent.name = 'Electrodomésticos'
		ORDER BY child.id LIMIT 1`).Scan(&refrigeradoresID); err != nil {
		t.Fatal(err)
	}
	fridge, err := repo.ResolveCategory(ctx, CategoryRef{ID: refrigeradoresID})
	if err != nil || fridge.Slug == nil || *fridge.Slug != "refrigeradores" || len(fridge.Attributes) != 5 {
		t.Fatalf("hoja enlazada = %+v (%v)", fridge, err)
	}
	if fridge.CategoryID == nil || *fridge.CategoryID != refrigeradoresID {
		t.Fatalf("categoría unificada usa id de hoja %v, quiero %d", fridge.CategoryID, refrigeradoresID)
	}

	var oldRefrigeracionID int32
	if err := pool.QueryRow(ctx, `SELECT id FROM product_category WHERE name = 'Refrigeracion'`).Scan(&oldRefrigeracionID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ResolveCategory(ctx, CategoryRef{ID: oldRefrigeracionID}); !errors.Is(err, ErrCategoryNotFound) {
		t.Fatalf("categoría retirada = %v, quiero ErrCategoryNotFound", err)
	}

	service, err := repo.ResolveCategory(ctx, CategoryRef{Slug: "internet-fibra-optica"})
	if err != nil || service.ItemType != categories.ItemTypeService || service.CategoryID != nil {
		t.Fatalf("categoría de servicio = %+v (%v)", service, err)
	}

	for _, ref := range []CategoryRef{{Slug: "no-existe"}, {ID: 999999}} {
		if _, err := repo.ResolveCategory(ctx, ref); !errors.Is(err, ErrCategoryNotFound) {
			t.Fatalf("ResolveCategory(%+v) = %v, quiero ErrCategoryNotFound", ref, err)
		}
	}
}

func TestCatalogIntegration_RefrigeradoresConsolidadosEnElectrodomesticos(t *testing.T) {
	pool := repositoryTestPool(t)
	ctx := context.Background()

	var applianceID, refrigeratorID, oldRefrigerationID, unifiedID int32
	if err := pool.QueryRow(ctx, `
		SELECT parent.id, child.id
		FROM product_category parent
		JOIN product_category child ON child.parent_category_id = parent.id
		WHERE parent.name = 'Electrodomésticos' AND child.name = 'Refrigeradores'
		ORDER BY parent.id, child.id LIMIT 1`).Scan(&applianceID, &refrigeratorID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM product_category WHERE name = 'Refrigeracion'`).Scan(&oldRefrigerationID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM categories WHERE slug = 'refrigeradores'`).Scan(&unifiedID); err != nil {
		t.Fatal(err)
	}

	var parentID pgtype.Int4
	var sourceActive bool
	if err := pool.QueryRow(ctx, `SELECT parent_category_id FROM product_category WHERE id = $1`, refrigeratorID).Scan(&parentID); err != nil {
		t.Fatal(err)
	}
	if !parentID.Valid || parentID.Int32 != applianceID {
		t.Fatalf("padre de Refrigeradores = %v, quiero %d", parentID, applianceID)
	}
	if err := pool.QueryRow(ctx, `SELECT active FROM product_category WHERE id = $1`, oldRefrigerationID).Scan(&sourceActive); err != nil {
		t.Fatal(err)
	}
	if sourceActive {
		t.Fatal("Refrigeracion debe quedar inactiva")
	}

	var misplaced, wrongUnifiedRef, oldSpecValues int64
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM product WHERE category_id = $1`, oldRefrigerationID).Scan(&misplaced); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM product
		WHERE category_id = $1 AND category_ref IS DISTINCT FROM $2`, refrigeratorID, unifiedID).Scan(&wrongUnifiedRef); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM product_specification_value value
		JOIN product_category_specification spec ON spec.id = value.specification_id
		WHERE spec.category_id = $1`, oldRefrigerationID).Scan(&oldSpecValues); err != nil {
		t.Fatal(err)
	}
	if misplaced != 0 || wrongUnifiedRef != 0 || oldSpecValues != 0 {
		t.Fatalf("productos fuera de Refrigeradores = %d, referencias unificadas incorrectas = %d, valores de especificación antiguos = %d",
			misplaced, wrongUnifiedRef, oldSpecValues)
	}

	var linkedCategoryID int32
	if err := pool.QueryRow(ctx, `SELECT legacy_product_category_id FROM categories WHERE id = $1`, unifiedID).Scan(&linkedCategoryID); err != nil {
		t.Fatal(err)
	}
	if linkedCategoryID != refrigeratorID {
		t.Fatalf("legacy_product_category_id = %d, quiero %d", linkedCategoryID, refrigeratorID)
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

	// El alcance por slug baja por el árbol: Computación abarca notebooks,
	// computadores de escritorio y accesorios.
	scope, err := repo.ResolveScope(ctx, categories.Ref{Slug: strp("computacion")})
	if err != nil || scope.Level != categories.Level2 || len(scope.CategoryIDs) < 2 {
		t.Fatalf("alcance de Computación = %+v (%v)", scope, err)
	}
	var wantSubtree int64
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM product p
		WHERE p.active AND p.category_id = ANY($1::int[])`, scope.CategoryIDs).Scan(&wantSubtree); err != nil {
		t.Fatal(err)
	}
	items, total, err := repo.List(ctx, ListParams{CategoryIDs: scope.CategoryIDs, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if wantSubtree == 0 || total != wantSubtree || int64(len(items)) != wantSubtree {
		t.Fatalf("filtro Computación = %d (total %d), quiero %d", len(items), total, wantSubtree)
	}

	// La hoja notebooks devuelve solo su propio id.
	byLeaf, err := repo.ResolveScope(ctx, categories.Ref{Slug: strp("notebooks")})
	if err != nil || byLeaf.Level != categories.Level3 || len(byLeaf.CategoryIDs) != 1 {
		t.Fatalf("alcance por slug notebooks = %+v (%v)", byLeaf, err)
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
	publicID, err := repo.Create(ctx, ProductWrite{CategoryID: *fridge.CategoryID, CategoryRef: fridge.ID,
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
		_, err := repo.Create(ctx, ProductWrite{CategoryID: *fridge.CategoryID, Name: "dup", SKU: sku})
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
		// ON DELETE RESTRICT se reporta como 23001 en algunas versiones de
		// PostgreSQL y como 23503 en otras; ambas significan lo mismo aquí.
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || (pgErr.Code != "23001" && pgErr.Code != "23503") {
			t.Fatalf("error = %v, quiero SQLSTATE 23001 o 23503", err)
		}
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

// TAL-238: cada punto del historial en el detalle del producto indica de qué
// oferta y tienda es, en qué moneda y a qué hora exacta se registró, además de
// los campos originales date y price.
func TestCatalogIntegration_HistorialConOfertaMonedaYHora(t *testing.T) {
	pool := repositoryTestPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	fridge, values := resolveAndValidate(t, repo, "refrigeradores",
		`{"capacidad_litros": 350, "eficiencia_energetica": "A+", "no_frost": true}`)
	publicID, err := repo.Create(ctx, ProductWrite{
		CategoryID: *fridge.CategoryID, CategoryRef: fridge.ID,
		Name: "Refrigerador TAL-238", SKU: uniqueSKU("tal238"), Attributes: values,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	var storeID int32
	if err := pool.QueryRow(ctx, `
		INSERT INTO store (name, website_url) VALUES ($1, 'https://tal238.example.test')
		RETURNING id`, uniqueSKU("Tienda TAL-238")).Scan(&storeID); err != nil {
		t.Fatalf("crear tienda: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM product WHERE public_id = $1`, publicID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM store WHERE id = $1`, storeID)
	})

	// El trigger de la migración 010 registra un punto al crear la oferta y
	// otro al cambiar el precio.
	var offerID int32
	if err := pool.QueryRow(ctx, `
		INSERT INTO product_offer (product_id, store_id, price, currency, product_url)
		SELECT id, $2, 199990, 'CLP', 'https://tal238.example.test/p/1' FROM product WHERE public_id = $1
		RETURNING id`, publicID, storeID).Scan(&offerID); err != nil {
		t.Fatalf("crear oferta: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE product_offer SET price = 189990 WHERE id = $1`, offerID); err != nil {
		t.Fatalf("cambiar precio: %v", err)
	}

	detail, err := repo.GetDetail(ctx, publicID)
	if err != nil {
		t.Fatalf("GetDetail: %v", err)
	}
	if len(detail.PriceHistory) != 2 {
		t.Fatalf("historial = %+v, quiero 2 puntos", detail.PriceHistory)
	}
	for i, wantPrice := range []string{"199990.00", "189990.00"} {
		point := detail.PriceHistory[i]
		recordedAt, err := time.Parse(time.RFC3339Nano, point.RecordedAt)
		if err != nil {
			t.Fatalf("recordedAt %q no es RFC 3339: %v", point.RecordedAt, err)
		}
		if point.Price != wantPrice || point.OfferID != offerID || point.StoreID != storeID ||
			point.Currency != "CLP" || point.Date != recordedAt.Format("2006-01-02") {
			t.Fatalf("punto %d = %+v", i, point)
		}
	}

	// La forma JSON conserva date y price y agrega los cuatro campos nuevos.
	encoded, _ := json.Marshal(detail.PriceHistory[0])
	var keys map[string]json.RawMessage
	_ = json.Unmarshal(encoded, &keys)
	for _, key := range []string{"date", "price", "offerId", "storeId", "currency", "recordedAt"} {
		if _, ok := keys[key]; !ok {
			t.Fatalf("falta %q en %s", key, encoded)
		}
	}
	if len(keys) != 6 {
		t.Fatalf("campos inesperados en %s", encoded)
	}
}
