package categories

import (
	"context"
	"testing"
)

// TestIntegration_ArbolDeCategoriasUnificado comprueba, sobre la base con
// todas las migraciones aplicadas, las reglas que dejan las migraciones
// 022 a 026 (TAL-231, TAL-140 y TAL-244). Cada caso es una consulta que
// cuenta filas que rompen una regla, así que el valor esperado siempre es 0.
// Existe para que una migración futura que mueva productos o fusione
// categorías no vuelva a dejar ramas duplicadas ni filtros por slug vacíos.
func TestIntegration_ArbolDeCategoriasUnificado(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()

	tests := []struct {
		name string
		sql  string
	}{
		{
			name: "sin categorías activas con el mismo nombre bajo el mismo padre",
			sql: `
				SELECT count(*) FROM (
				    SELECT parent_category_id,
				           translate(lower(btrim(name)), 'áéíóúüñ', 'aeiouun') AS label
				    FROM product_category
				    WHERE active
				    GROUP BY 1, 2
				    HAVING count(*) > 1
				) duplicated`,
		},
		{
			name: "ningún producto activo en una categoría inactiva",
			sql: `
				SELECT count(*)
				FROM product p
				JOIN product_category pc ON pc.id = p.category_id
				WHERE p.active AND NOT pc.active`,
		},
		{
			name: "las ramas fusionadas por TAL-231 quedan inactivas",
			sql: `
				SELECT count(*)
				FROM product_category
				WHERE active
				  AND translate(lower(btrim(name)), 'áéíóúüñ', 'aeiouun')
				      IN ('telefonia', 'lavado', 'computadores', 'videojuegos', 'refrigeracion', 'hogar')`,
		},
		{
			name: "Lavadoras cuelga de Electrodomésticos",
			sql: `
				SELECT count(*)
				FROM product_category child
				LEFT JOIN product_category parent ON parent.id = child.parent_category_id
				WHERE child.active AND child.name = 'Lavadoras'
				  AND parent.name IS DISTINCT FROM 'Electrodomésticos'`,
		},
		{
			name: "todo producto bajo una categoría con slug tiene category_ref",
			sql: `
				WITH RECURSIVE ancestry AS (
				    SELECT p.id AS product_id, p.category_id AS category_id, 0 AS depth
				    FROM product p
				    WHERE p.active AND p.category_ref IS NULL
				    UNION ALL
				    SELECT a.product_id, pc.parent_category_id, a.depth + 1
				    FROM ancestry a
				    JOIN product_category pc ON pc.id = a.category_id
				    WHERE pc.parent_category_id IS NOT NULL AND a.depth < 20
				)
				SELECT count(DISTINCT a.product_id)
				FROM ancestry a
				JOIN categories c ON c.legacy_product_category_id = a.category_id
				WHERE c.item_type = 'product' AND c.active`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var broken int64
			if err := pool.QueryRow(ctx, tc.sql).Scan(&broken); err != nil {
				t.Fatal(err)
			}
			if broken != 0 {
				t.Fatalf("%d fila(s) rompen la regla", broken)
			}
		})
	}
}
