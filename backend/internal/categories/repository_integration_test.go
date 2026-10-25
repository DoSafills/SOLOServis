package categories

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Los tests de integración requieren una base PostgreSQL limpia con todas las
// migraciones aplicadas (ver backend/README.md, sección de categorías).
// Sin TEST_DATABASE_URL se omiten, así `go test ./...` no depende de una BD.
func integrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL no definida: se omite el test de integración")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("conexión: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestIntegration_SeedsCumplenRestricciones(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()

	var slugs []string
	if err := pool.QueryRow(ctx, `SELECT array_agg(slug ORDER BY slug) FROM categories`).Scan(&slugs); err != nil {
		t.Fatal(err)
	}
	want := []string{"internet-fibra-optica", "notebooks", "refrigeradores", "televisores"}
	if !reflect.DeepEqual(slugs, want) {
		t.Fatalf("slugs = %v, quiero %v", slugs, want)
	}

	var services int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM categories WHERE item_type = 'service'`).Scan(&services); err != nil {
		t.Fatal(err)
	}
	if services < 1 {
		t.Fatal("los seeds deben incluir al menos una categoría de servicio")
	}

	var dataTypes []string
	if err := pool.QueryRow(ctx,
		`SELECT array_agg(DISTINCT data_type ORDER BY data_type) FROM category_attributes`).Scan(&dataTypes); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(dataTypes, []string{"boolean", "decimal", "enum", "integer", "string"}) {
		t.Fatalf("data_types en seeds = %v", dataTypes)
	}

	// Sobre una base limpia, cada seed queda enlazado con su categoría antigua.
	var unlinked []string
	if err := pool.QueryRow(ctx, `
		SELECT coalesce(array_agg(slug), '{}') FROM categories
		WHERE legacy_product_category_id IS NULL AND legacy_service_category_id IS NULL`).Scan(&unlinked); err != nil {
		t.Fatal(err)
	}
	if len(unlinked) > 0 {
		t.Fatalf("categorías sin enlace legado: %v", unlinked)
	}

	// Reglas que la BD no puede expresar con CHECK: valores de enum no vacíos
	// y sin duplicados.
	var badEnums []string
	if err := pool.QueryRow(ctx, `
		SELECT coalesce(array_agg(key), '{}') FROM category_attributes
		WHERE data_type = 'enum'
		  AND (cardinality(allowed_values) <> (SELECT count(DISTINCT v) FROM unnest(allowed_values) v)
		       OR EXISTS (SELECT 1 FROM unnest(allowed_values) v WHERE btrim(v) = ''))`).Scan(&badEnums); err != nil {
		t.Fatal(err)
	}
	if len(badEnums) > 0 {
		t.Fatalf("enums con valores vacíos o duplicados: %v", badEnums)
	}
}

func TestIntegration_RestriccionesRechazanDatosInvalidos(t *testing.T) {
	pool := integrationPool(t)

	const (
		checkViolation  = "23514"
		uniqueViolation = "23505"
		fkViolation     = "23503"
	)
	const notebooks = `(SELECT id FROM categories WHERE slug = 'notebooks')`
	attr := func(columns, values string) string {
		return `INSERT INTO category_attributes (category_id, key, label, data_type` + columns +
			`) VALUES (` + notebooks + `, ` + values + `)`
	}

	tests := []struct {
		name     string
		sql      string
		wantCode string
	}{
		{"slug con mayúsculas", `INSERT INTO categories (slug, name, item_type) VALUES ('Malo', 'X', 'product')`, checkViolation},
		{"slug duplicado", `INSERT INTO categories (slug, name, item_type) VALUES ('notebooks', 'X', 'product')`, uniqueViolation},
		{"nombre vacío", `INSERT INTO categories (slug, name, item_type) VALUES ('x', '  ', 'product')`, checkViolation},
		{"item_type inválido", `INSERT INTO categories (slug, name, item_type) VALUES ('x', 'X', 'otro')`, checkViolation},
		{"enlace legado de otro tipo", `INSERT INTO categories (slug, name, item_type, legacy_service_category_id)
			VALUES ('x', 'X', 'product', (SELECT min(id) FROM service_category))`, checkViolation},
		{"key con mayúsculas", attr(``, `'Capacidad', 'C', 'string'`), checkViolation},
		{"key duplicada en la categoría", attr(``, `'procesador', 'P', 'string'`), uniqueViolation},
		{"data_type fuera del catálogo", attr(``, `'k', 'K', 'number'`), checkViolation},
		{"enum sin allowed_values", attr(``, `'k', 'K', 'enum'`), checkViolation},
		{"enum con allowed_values vacío", attr(`, allowed_values`, `'k', 'K', 'enum', '{}'`), checkViolation},
		{"allowed_values en string", attr(`, allowed_values`, `'k', 'K', 'string', '{a}'`), checkViolation},
		{"min en boolean", attr(`, min_value`, `'k', 'K', 'boolean', 1`), checkViolation},
		{"max en string", attr(`, max_value`, `'k', 'K', 'string', 1`), checkViolation},
		{"min mayor que max", attr(`, min_value, max_value`, `'k', 'K', 'decimal', 10, 1`), checkViolation},
		{"integer con min decimal", attr(`, min_value`, `'k', 'K', 'integer', 0.5`), checkViolation},
		{"max_length en integer", attr(`, max_length`, `'k', 'K', 'integer', 10`), checkViolation},
		{"max_length mayor a 255", attr(`, max_length`, `'k', 'K', 'string', 256`), checkViolation},
		{"max_length cero", attr(`, max_length`, `'k', 'K', 'string', 0`), checkViolation},
		{"unidad en string", attr(`, unit`, `'k', 'K', 'string', 'cm'`), checkViolation},
		{"label vacío", attr(``, `'k', ' ', 'string'`), checkViolation},
		{"categoría inexistente", `INSERT INTO category_attributes (category_id, key, label, data_type)
			VALUES (-1, 'k', 'K', 'string')`, fkViolation},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)

			_, err = tx.Exec(ctx, tc.sql)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != tc.wantCode {
				t.Fatalf("error = %v, quiero SQLSTATE %s", err, tc.wantCode)
			}
		})
	}
}

func TestIntegration_RepositoryList(t *testing.T) {
	repo := NewRepository(integrationPool(t))
	ctx := context.Background()
	service := ItemTypeService

	tests := []struct {
		name      string
		params    ListParams
		wantSlugs []string
		wantTotal int64
	}{
		{"primera página", ListParams{Limit: 2}, []string{"internet-fibra-optica", "notebooks"}, 4},
		{"segunda página", ListParams{Limit: 2, Offset: 2}, []string{"refrigeradores", "televisores"}, 4},
		{"offset fuera de rango", ListParams{Limit: 20, Offset: 10}, []string{}, 4},
		{"filtro servicio", ListParams{ItemType: &service, Limit: 20}, []string{"internet-fibra-optica"}, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			items, total, err := repo.List(ctx, tc.params)
			if err != nil {
				t.Fatal(err)
			}
			slugs := []string{}
			for _, item := range items {
				slugs = append(slugs, item.Slug)
			}
			if !reflect.DeepEqual(slugs, tc.wantSlugs) || total != tc.wantTotal {
				t.Fatalf("slugs = %v total = %d, quiero %v total = %d", slugs, total, tc.wantSlugs, tc.wantTotal)
			}
		})
	}

	items, _, err := repo.List(ctx, ListParams{Limit: 1, Offset: 2})
	if err != nil {
		t.Fatal(err)
	}
	if items[0].Slug != "refrigeradores" || items[0].AttributeCount != 5 || items[0].ItemType != ItemTypeProduct {
		t.Fatalf("item = %+v", items[0])
	}
}

func TestIntegration_RepositoryGetBySlug(t *testing.T) {
	pool := integrationPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	detail, err := repo.GetBySlug(ctx, "refrigeradores")
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, a := range detail.Attributes {
		keys = append(keys, a.Key)
	}
	wantKeys := []string{"capacidad_litros", "eficiencia_energetica", "no_frost", "peso_kg", "color"}
	if !reflect.DeepEqual(keys, wantKeys) {
		t.Fatalf("keys = %v, quiero %v (orden por display_order)", keys, wantKeys)
	}

	capacidad := detail.Attributes[0]
	if capacidad.DataType != "integer" || capacidad.Unit == nil || *capacidad.Unit != "L" ||
		capacidad.Min == nil || *capacidad.Min != 50 || capacidad.Max == nil || *capacidad.Max != 1500 ||
		!capacidad.Required || capacidad.AllowedValues != nil || capacidad.MaxLength != nil {
		t.Fatalf("capacidad_litros = %+v", capacidad)
	}
	eficiencia := detail.Attributes[1]
	if eficiencia.DataType != "enum" || len(eficiencia.AllowedValues) != 7 || eficiencia.Min != nil {
		t.Fatalf("eficiencia_energetica = %+v", eficiencia)
	}
	color := detail.Attributes[4]
	if color.MaxLength == nil || *color.MaxLength != 50 {
		t.Fatalf("color = %+v", color)
	}

	if _, err := repo.GetBySlug(ctx, "no-existe"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("slug inexistente: error = %v, quiero ErrNotFound", err)
	}

	// Una categoría inactiva no se expone.
	if _, err := pool.Exec(ctx,
		`INSERT INTO categories (slug, name, item_type, active) VALUES ('tmp-inactiva', 'Tmp', 'product', FALSE)`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM categories WHERE slug = 'tmp-inactiva'`)
	})
	if _, err := repo.GetBySlug(ctx, "tmp-inactiva"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("categoría inactiva: error = %v, quiero ErrNotFound", err)
	}
}
