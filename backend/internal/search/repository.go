package search

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/DoSafills/SOLOServis/backend/internal/categories"
	"github.com/DoSafills/SOLOServis/backend/internal/products/dto"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository ejecuta las búsquedas. Todas las consultas son estáticas y
// parametrizadas: los filtros variables (palabras, marcas, atributos) viajan
// como arreglos y se recorren con unnest, nunca se concatena entrada del usuario.
type Repository struct {
	db         *pgxpool.Pool
	categories *categories.PostgresRepository
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db, categories: categories.NewRepository(db)}
}

// matchedSQL selecciona los productos activos que cumplen todos los filtros:
//
//	$1 palabras (text[]): cada una debe aparecer en nombre, modelo, SKU o marca
//	$2 id de product_category (incluye subcategorías)  $3 slug de categories
//	$4 ids de marca (int[], vacío = todas)
//	$5 / $6 precio total mínimo / máximo               $7 solo con ofertas disponibles
//	$8 / $9 / $10 filtros por atributo: ids, operadores y valores
//
// strpos (no ILIKE) evita que '%' o '_' del usuario actúen como comodines, y
// translate (nativo, sin la extensión unaccent) ignora tildes y mayúsculas.
const matchedSQL = `
WITH RECURSIVE subtree AS (
    SELECT c.id FROM product_category c WHERE c.id = $2::int AND c.active
    UNION ALL
    SELECT child.id FROM product_category child
    JOIN subtree s ON child.parent_category_id = s.id
    WHERE child.active
),
matched AS (
    SELECT p.id, best.price AS best_price, best.offers AS offer_count
    FROM product p
    LEFT JOIN brand b ON b.id = p.brand_id
    LEFT JOIN LATERAL (
        SELECT MIN(o.price + CASE WHEN o.shipping_free THEN 0 ELSE COALESCE(o.shipping_cost, 0) END) AS price,
               count(*) AS offers
        FROM product_offer o
        JOIN store s ON s.id = o.store_id AND s.active
        WHERE o.product_id = p.id AND o.available
    ) best ON TRUE
    WHERE p.active
      AND NOT EXISTS (
          SELECT 1 FROM unnest($1::text[]) AS w(word)
          WHERE strpos(
              translate(lower(p.name || ' ' || COALESCE(p.model, '') || ' ' || COALESCE(p.sku, '') || ' ' || COALESCE(b.name, '')),
                        'áàäâéèëêíìïîóòöôúùüûñç', 'aaaaeeeeiiiioooouuuunc'),
              translate(lower(w.word), 'áàäâéèëêíìïîóòöôúùüûñç', 'aaaaeeeeiiiioooouuuunc')) = 0)
      AND ($2::int IS NULL OR p.category_id IN (SELECT id FROM subtree))
      AND ($3::text IS NULL OR p.category_ref = (SELECT id FROM categories WHERE slug = $3::text AND active))
      AND (cardinality($4::int[]) = 0 OR p.brand_id = ANY($4::int[]))
      AND ($5::numeric IS NULL OR best.price >= $5::numeric)
      AND ($6::numeric IS NULL OR best.price <= $6::numeric)
      AND (NOT $7::bool OR best.offers > 0)
      AND NOT EXISTS (
          SELECT 1 FROM unnest($8::int[], $9::text[], $10::text[]) AS f(attribute_id, op, val)
          WHERE NOT EXISTS (
              SELECT 1 FROM product_attribute_value v
              WHERE v.product_id = p.id AND v.attribute_id = f.attribute_id
                AND CASE f.op
                        WHEN 'eq'  THEN v.value = f.val
                        WHEN 'eqi' THEN lower(v.value) = lower(f.val)
                        WHEN 'eqn' THEN v.value_numeric = f.val::numeric
                        WHEN 'min' THEN v.value_numeric >= f.val::numeric
                        WHEN 'max' THEN v.value_numeric <= f.val::numeric
                        ELSE FALSE
                    END))
)
`

// Ordenes permitidos: el handler solo acepta estas claves.
var orderBySQL = map[string]string{
	"name":       "p.name ASC, p.id ASC",
	"price_asc":  "m.best_price ASC NULLS LAST, p.id ASC",
	"price_desc": "m.best_price DESC NULLS LAST, p.id ASC",
	"newest":     "p.created_at DESC, p.id DESC",
}

func queryArgs(q Query) []any {
	words := q.Words
	if words == nil {
		words = []string{}
	}
	brands := q.BrandIDs
	if brands == nil {
		brands = []int32{}
	}
	ids := make([]int32, 0, len(q.Attributes))
	ops := make([]string, 0, len(q.Attributes))
	values := make([]string, 0, len(q.Attributes))
	for _, f := range q.Attributes {
		ids = append(ids, f.AttributeID)
		ops = append(ops, f.Op)
		values = append(values, f.Value)
	}
	return []any{words, q.LegacyCategoryID, q.CategorySlug, brands, q.MinPrice, q.MaxPrice,
		q.Available, ids, ops, values}
}

// CategoryDefinitions devuelve las definiciones de atributos de una categoría
// activa, o ErrCategoryNotFound.
func (r *Repository) CategoryDefinitions(ctx context.Context, slug string) ([]categories.Attribute, error) {
	var id int32
	err := r.db.QueryRow(ctx, `SELECT id FROM categories WHERE slug = $1 AND active`, slug).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCategoryNotFound
	}
	if err != nil {
		return nil, err
	}
	return r.categories.AttributesByCategoryID(ctx, id)
}

// SearchProducts devuelve una página de resultados y el total sin paginar.
// Usa 3 consultas fijas: conteo, página y atributos de la página.
func (r *Repository) SearchProducts(ctx context.Context, q Query) ([]ProductResult, int64, error) {
	args := queryArgs(q)

	var total int64
	if err := r.db.QueryRow(ctx, matchedSQL+`SELECT count(*) FROM matched`, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	order, ok := orderBySQL[q.Sort]
	if !ok {
		order = orderBySQL["name"]
	}
	rows, err := r.db.Query(ctx, matchedSQL+`
		SELECT p.id, p.public_id::text, p.name, COALESCE(b.name, ''), COALESCE(p.model, ''), pc.name,
		       cat.slug,
		       (SELECT image_url FROM product_image i WHERE i.product_id = p.id AND i.active
		         ORDER BY i.sort_order, i.id LIMIT 1),
		       m.best_price::text, m.offer_count
		FROM matched m
		JOIN product p ON p.id = m.id
		LEFT JOIN brand b ON b.id = p.brand_id
		JOIN product_category pc ON pc.id = p.category_id
		LEFT JOIN categories cat ON cat.id = p.category_ref
		ORDER BY `+order+`
		LIMIT $11 OFFSET $12`, append(args, q.Limit, q.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	results := make([]ProductResult, 0, q.Limit)
	index := map[int32]int{}
	ids := make([]int32, 0, q.Limit)
	for rows.Next() {
		var id int32
		result := ProductResult{Attributes: map[string]json.RawMessage{}}
		if err := rows.Scan(&id, &result.ID, &result.Name, &result.Brand, &result.Model, &result.Category,
			&result.CategorySlug, &result.ImageURL, &result.BestPrice, &result.OfferCount); err != nil {
			return nil, 0, err
		}
		index[id] = len(results)
		ids = append(ids, id)
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(ids) == 0 {
		return results, total, nil
	}

	attrRows, err := r.db.Query(ctx, `
		SELECT pav.product_id, ca.key, ca.data_type, pav.value
		FROM product_attribute_value pav
		JOIN category_attributes ca ON ca.id = pav.attribute_id
		WHERE pav.product_id = ANY($1)
		ORDER BY pav.product_id, ca.display_order, ca.id`, ids)
	if err != nil {
		return nil, 0, err
	}
	defer attrRows.Close()
	for attrRows.Next() {
		var productID int32
		var key, dataType, value string
		if err := attrRows.Scan(&productID, &key, &dataType, &value); err != nil {
			return nil, 0, err
		}
		results[index[productID]].Attributes[key] = dto.AttributeJSON(dataType, value)
	}
	return results, total, attrRows.Err()
}

// SearchFacets resume los productos que cumplen q: total, rango de precios,
// marcas y, si hay definiciones, los atributos filtrables (4 consultas fijas).
func (r *Repository) SearchFacets(ctx context.Context, q Query, defs []categories.Attribute) (Facets, error) {
	args := queryArgs(q)
	facets := Facets{Brands: []BrandFacet{}, Attributes: []AttributeFacet{}}

	if err := r.db.QueryRow(ctx, matchedSQL+`
		SELECT count(*), min(best_price)::text, max(best_price)::text FROM matched`, args...).
		Scan(&facets.Total, &facets.Price.Min, &facets.Price.Max); err != nil {
		return Facets{}, err
	}

	rows, err := r.db.Query(ctx, matchedSQL+`
		SELECT b.id, b.name, count(*)
		FROM matched m JOIN product p ON p.id = m.id JOIN brand b ON b.id = p.brand_id
		GROUP BY b.id, b.name
		ORDER BY count(*) DESC, b.name ASC`, args...)
	if err != nil {
		return Facets{}, err
	}
	for rows.Next() {
		var brand BrandFacet
		if err := rows.Scan(&brand.ID, &brand.Name, &brand.Count); err != nil {
			rows.Close()
			return Facets{}, err
		}
		facets.Brands = append(facets.Brands, brand)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Facets{}, err
	}

	filterable := make([]int32, 0, len(defs))
	byID := map[int32]int{}
	for _, def := range defs {
		if !def.Filterable {
			continue
		}
		byID[def.ID] = len(facets.Attributes)
		filterable = append(filterable, def.ID)
		facets.Attributes = append(facets.Attributes, AttributeFacet{
			Key: def.Key, Label: def.Label, DataType: def.DataType, Unit: def.Unit,
		})
	}
	if len(filterable) == 0 {
		return facets, nil
	}

	rows, err = r.db.Query(ctx, matchedSQL+`
		SELECT v.attribute_id, ca.data_type, v.value, count(*), min(v.value_numeric)::float8, max(v.value_numeric)::float8
		FROM matched m
		JOIN product_attribute_value v ON v.product_id = m.id
		JOIN category_attributes ca ON ca.id = v.attribute_id
		WHERE v.attribute_id = ANY($11::int[])
		GROUP BY v.attribute_id, ca.data_type, v.value
		ORDER BY v.attribute_id, count(*) DESC, v.value`, append(args, filterable)...)
	if err != nil {
		return Facets{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			attributeID int32
			dataType    string
			value       string
			count       int64
			low, high   *float64
		)
		if err := rows.Scan(&attributeID, &dataType, &value, &count, &low, &high); err != nil {
			return Facets{}, err
		}
		facet := &facets.Attributes[byID[attributeID]]
		if dataType == "integer" || dataType == "decimal" {
			if low != nil && (facet.Min == nil || *low < *facet.Min) {
				facet.Min = low
			}
			if high != nil && (facet.Max == nil || *high > *facet.Max) {
				facet.Max = high
			}
			continue
		}
		facet.Values = append(facet.Values, ValueCount{Value: dto.AttributeJSON(dataType, value), Count: count})
	}
	return facets, rows.Err()
}

// SearchCategories busca categorías activas del modelo nuevo por nombre o slug.
func (r *Repository) SearchCategories(ctx context.Context, text string, itemType *string) ([]CategoryResult, error) {
	rows, err := r.db.Query(ctx, `
		SELECT slug, name, item_type FROM categories
		WHERE active
		  AND ($2::text IS NULL OR item_type = $2::text)
		  AND ($1 = '' OR strpos(
		        translate(lower(name || ' ' || slug), 'áàäâéèëêíìïîóòöôúùüûñç', 'aaaaeeeeiiiioooouuuunc'),
		        translate(lower($1), 'áàäâéèëêíìïîóòöôúùüûñç', 'aaaaeeeeiiiioooouuuunc')) > 0)
		ORDER BY name`, text, itemType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := []CategoryResult{}
	for rows.Next() {
		var c CategoryResult
		if err := rows.Scan(&c.Slug, &c.Name, &c.ItemType); err != nil {
			return nil, err
		}
		results = append(results, c)
	}
	return results, rows.Err()
}

// SearchBrands busca marcas activas por nombre.
func (r *Repository) SearchBrands(ctx context.Context, text string) ([]BrandResult, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, logo_url FROM brand
		WHERE active
		  AND ($1 = '' OR strpos(
		        translate(lower(name), 'áàäâéèëêíìïîóòöôúùüûñç', 'aaaaeeeeiiiioooouuuunc'),
		        translate(lower($1), 'áàäâéèëêíìïîóòöôúùüûñç', 'aaaaeeeeiiiioooouuuunc')) > 0)
		ORDER BY name`, text)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := []BrandResult{}
	for rows.Next() {
		var b BrandResult
		if err := rows.Scan(&b.ID, &b.Name, &b.LogoURL); err != nil {
			return nil, err
		}
		results = append(results, b)
	}
	return results, rows.Err()
}
