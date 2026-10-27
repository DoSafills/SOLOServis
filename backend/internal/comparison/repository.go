package comparison

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/DoSafills/SOLOServis/backend/internal/products/dto"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository accede a product_comparison(_item) y arma las matrices. Todas
// las consultas son parametrizadas; las de comparaciones guardadas filtran
// siempre por user_id, así un usuario no puede leer ni tocar las ajenas.
type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ResolveProducts busca productos activos por UUID y los devuelve en el orden
// pedido. Si alguno no existe devuelve ErrProductNotFound con su UUID.
func (r *Repository) ResolveProducts(ctx context.Context, publicIDs []string) ([]Product, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, public_id::text, category_id FROM product
		WHERE public_id = ANY($1::uuid[]) AND active`, publicIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	found := map[string]Product{}
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.PublicID, &p.CategoryID); err != nil {
			return nil, err
		}
		found[p.PublicID] = p
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	products := make([]Product, 0, len(publicIDs))
	for _, id := range publicIDs {
		p, ok := found[id]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrProductNotFound, id)
		}
		products = append(products, p)
	}
	return products, nil
}

const summariesSQL = `
SELECT p.id, p.public_id::text, p.name, COALESCE(b.name, ''), COALESCE(p.model, ''),
       (SELECT i.image_url FROM product_image i WHERE i.product_id = p.id AND i.active
         ORDER BY i.sort_order, i.id LIMIT 1),
       best.total::text, best.store,
       (SELECT count(*) FROM product_offer o JOIN store s ON s.id = o.store_id AND s.active
         WHERE o.product_id = p.id AND o.available)
FROM product p
LEFT JOIN brand b ON b.id = p.brand_id
LEFT JOIN LATERAL (
    SELECT o.price + CASE WHEN o.shipping_free THEN 0 ELSE COALESCE(o.shipping_cost, 0) END AS total,
           s.name AS store
    FROM product_offer o JOIN store s ON s.id = o.store_id AND s.active
    WHERE o.product_id = p.id AND o.available
    ORDER BY total, o.price, o.id
    LIMIT 1
) best ON TRUE
WHERE p.id = ANY($1)`

// summaries devuelve las columnas de productos en el orden de ids.
func (r *Repository) summaries(ctx context.Context, ids []int32) ([]ProductSummary, error) {
	rows, err := r.db.Query(ctx, summariesSQL, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := map[int32]ProductSummary{}
	for rows.Next() {
		var id int32
		var s ProductSummary
		if err := rows.Scan(&id, &s.ID, &s.Name, &s.Brand, &s.Model, &s.ImageURL, &s.BestPrice,
			&s.BestStore, &s.OfferCount); err != nil {
			return nil, err
		}
		byID[id] = s
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]ProductSummary, 0, len(ids))
	for _, id := range ids {
		if s, ok := byID[id]; ok {
			out = append(out, s)
		}
	}
	return out, nil
}

// BuildMatrix arma la comparación de productos de una misma categoría (ids en
// orden de columna). Usa 5 consultas fijas, sin importar cuántos productos.
func (r *Repository) BuildMatrix(ctx context.Context, ids []int32) (Matrix, error) {
	matrix := Matrix{Products: []ProductSummary{}, Attributes: []AttributeRow{}, Specifications: []SpecificationRow{}}
	if len(ids) == 0 {
		return matrix, nil
	}

	var err error
	if matrix.Products, err = r.summaries(ctx, ids); err != nil {
		return Matrix{}, err
	}

	var (
		category    CategoryInfo
		legacyID    int32
		legacyName  string
		categoryRef *int32
		newName     *string
	)
	if err := r.db.QueryRow(ctx, `
		SELECT p.category_id, pc.name, c.id, c.slug, c.name
		FROM product p
		JOIN product_category pc ON pc.id = p.category_id
		LEFT JOIN categories c ON c.id = p.category_ref
		WHERE p.id = $1`, ids[0]).Scan(&legacyID, &legacyName, &categoryRef, &category.Slug, &newName); err != nil {
		return Matrix{}, err
	}
	category.Name = legacyName
	if newName != nil {
		category.Name = *newName
	}
	matrix.Category = &category

	column := make(map[int32]int, len(ids))
	for i, id := range ids {
		column[id] = i
	}

	if categoryRef != nil {
		if matrix.Attributes, err = r.attributeRows(ctx, *categoryRef, ids, column); err != nil {
			return Matrix{}, err
		}
	}
	if matrix.Specifications, err = r.specificationRows(ctx, legacyID, ids, column); err != nil {
		return Matrix{}, err
	}
	return matrix, nil
}

func (r *Repository) attributeRows(ctx context.Context, categoryRef int32, ids []int32, column map[int32]int) ([]AttributeRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT ca.id, ca.key, ca.label, ca.data_type, ca.unit, v.product_id, v.value
		FROM category_attributes ca
		LEFT JOIN product_attribute_value v ON v.attribute_id = ca.id AND v.product_id = ANY($2)
		WHERE ca.category_id = $1 AND ca.comparable
		ORDER BY ca.display_order, ca.id`, categoryRef, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []AttributeRow{}
	index := map[int32]int{}
	for rows.Next() {
		var (
			attributeID int32
			row         AttributeRow
			productID   *int32
			value       *string
		)
		if err := rows.Scan(&attributeID, &row.Key, &row.Label, &row.DataType, &row.Unit, &productID, &value); err != nil {
			return nil, err
		}
		i, ok := index[attributeID]
		if !ok {
			row.Values = make([]json.RawMessage, len(ids))
			for c := range row.Values {
				row.Values[c] = json.RawMessage("null")
			}
			result = append(result, row)
			i = len(result) - 1
			index[attributeID] = i
		}
		if productID != nil && value != nil {
			result[i].Values[column[*productID]] = dto.AttributeJSON(result[i].DataType, *value)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Solo filas con al menos un valor; differs compara también los null.
	filtered := []AttributeRow{}
	for _, row := range result {
		texts := make([]string, len(row.Values))
		hasValue := false
		for c, v := range row.Values {
			texts[c] = string(v)
			hasValue = hasValue || texts[c] != "null"
		}
		if hasValue {
			row.Differs = differs(texts)
			filtered = append(filtered, row)
		}
	}
	return filtered, nil
}

func (r *Repository) specificationRows(ctx context.Context, legacyID int32, ids []int32, column map[int32]int) ([]SpecificationRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT pcs.id, pcs.name, pcs.unit, psv.product_id, psv.value
		FROM product_category_specification pcs
		LEFT JOIN product_specification_value psv
		       ON psv.specification_id = pcs.id AND psv.product_id = ANY($2)
		WHERE pcs.category_id = $1 AND pcs.comparable
		ORDER BY pcs.display_order, pcs.id`, legacyID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []SpecificationRow{}
	index := map[int32]int{}
	for rows.Next() {
		var (
			specID    int32
			row       SpecificationRow
			productID *int32
			value     *string
		)
		if err := rows.Scan(&specID, &row.Name, &row.Unit, &productID, &value); err != nil {
			return nil, err
		}
		i, ok := index[specID]
		if !ok {
			row.Values = make([]*string, len(ids))
			result = append(result, row)
			i = len(result) - 1
			index[specID] = i
		}
		if productID != nil {
			result[i].Values[column[*productID]] = value
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	filtered := []SpecificationRow{}
	for _, row := range result {
		texts := make([]string, len(row.Values))
		hasValue := false
		for c, v := range row.Values {
			texts[c] = "\x00null"
			if v != nil {
				texts[c] = *v
				hasValue = true
			}
		}
		if hasValue {
			row.Differs = differs(texts)
			filtered = append(filtered, row)
		}
	}
	return filtered, nil
}

func differs(values []string) bool {
	for _, v := range values[1:] {
		if v != values[0] {
			return true
		}
	}
	return false
}

// Compatible devuelve los productos activos de la misma categoría que p
// (excluido p), por nombre, y el total.
func (r *Repository) Compatible(ctx context.Context, p Product, limit, offset int) ([]ProductSummary, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM product WHERE category_id = $1 AND active AND id <> $2`,
		p.CategoryID, p.ID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(ctx, `
		SELECT id FROM product WHERE category_id = $1 AND active AND id <> $2
		ORDER BY name, id LIMIT $3 OFFSET $4`, p.CategoryID, p.ID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return nil, 0, err
	}
	summaries, err := r.summaries(ctx, ids)
	return summaries, total, err
}

// ----------------------------------------------------- comparaciones guardadas

// Create guarda una comparación del usuario con sus productos (en orden).
func (r *Repository) Create(ctx context.Context, userID int32, name string, products []Product) (int32, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var id int32
	if err := tx.QueryRow(ctx, `
		INSERT INTO product_comparison (user_id, category_id, name) VALUES ($1, $2, $3) RETURNING id`,
		userID, products[0].CategoryID, name).Scan(&id); err != nil {
		return 0, err
	}
	productIDs := make([]int32, len(products))
	for i, p := range products {
		productIDs[i] = p.ID
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO product_comparison_item (comparison_id, product_id, position)
		SELECT $1, item.product_id, item.position - 1
		FROM unnest($2::int[]) WITH ORDINALITY AS item(product_id, position)`, id, productIDs); err != nil {
		return 0, err
	}
	return id, tx.Commit(ctx)
}

// List devuelve las comparaciones del usuario, la más reciente primero.
func (r *Repository) List(ctx context.Context, userID int32, limit, offset int) ([]Summary, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM product_comparison WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(ctx, `
		SELECT c.id, c.name, COALESCE(cat.name, pc.name), count(ci.id), c.updated_at
		FROM product_comparison c
		JOIN product_category pc ON pc.id = c.category_id
		LEFT JOIN categories cat ON cat.legacy_product_category_id = c.category_id AND cat.active
		LEFT JOIN product_comparison_item ci ON ci.comparison_id = c.id
		WHERE c.user_id = $1
		GROUP BY c.id, pc.name, cat.name
		ORDER BY c.updated_at DESC, c.id DESC
		LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	summaries, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Summary, error) {
		var s Summary
		err := row.Scan(&s.ID, &s.Name, &s.Category, &s.ProductCount, &s.UpdatedAt)
		return s, err
	})
	return summaries, total, err
}

// Get devuelve una comparación del usuario con sus productos activos en orden,
// o ErrComparisonNotFound (también si es de otro usuario).
func (r *Repository) Get(ctx context.Context, userID, id int32) (Header, error) {
	var h Header
	err := r.db.QueryRow(ctx, `
		SELECT id, name, category_id, created_at, updated_at
		FROM product_comparison WHERE id = $1 AND user_id = $2`, id, userID).
		Scan(&h.ID, &h.Name, &h.CategoryID, &h.CreatedAt, &h.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Header{}, ErrComparisonNotFound
	}
	if err != nil {
		return Header{}, err
	}
	rows, err := r.db.Query(ctx, `
		SELECT ci.product_id FROM product_comparison_item ci
		JOIN product p ON p.id = ci.product_id AND p.active
		WHERE ci.comparison_id = $1
		ORDER BY ci.position, ci.id`, id)
	if err != nil {
		return Header{}, err
	}
	h.ProductIDs, err = pgx.CollectRows(rows, pgx.RowTo[int32])
	return h, err
}

// Rename cambia el nombre, o ErrComparisonNotFound.
func (r *Repository) Rename(ctx context.Context, userID, id int32, name string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE product_comparison SET name = $3, updated_at = NOW() WHERE id = $1 AND user_id = $2`, id, userID, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrComparisonNotFound
	}
	return nil
}

// Delete borra la comparación (y sus ítems), o ErrComparisonNotFound.
func (r *Repository) Delete(ctx context.Context, userID, id int32) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM product_comparison WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrComparisonNotFound
	}
	return nil
}

// AddItem agrega un producto al final. Bloquea la comparación para que el
// límite de MaxProducts se respete con pedidos concurrentes.
func (r *Repository) AddItem(ctx context.Context, userID, id int32, p Product) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var categoryID int32
	err = tx.QueryRow(ctx, `
		SELECT category_id FROM product_comparison WHERE id = $1 AND user_id = $2 FOR UPDATE`, id, userID).Scan(&categoryID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrComparisonNotFound
	}
	if err != nil {
		return err
	}
	if categoryID != p.CategoryID {
		return ErrIncompatibleCategory
	}

	var count, next int32
	if err := tx.QueryRow(ctx, `
		SELECT count(*), COALESCE(max(position) + 1, 0) FROM product_comparison_item WHERE comparison_id = $1`, id).
		Scan(&count, &next); err != nil {
		return err
	}
	if count >= MaxProducts {
		return ErrComparisonFull
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO product_comparison_item (comparison_id, product_id, position) VALUES ($1, $2, $3)`,
		id, p.ID, next); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrAlreadyIncluded
		}
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE product_comparison SET updated_at = NOW() WHERE id = $1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RemoveItem quita un producto: ErrComparisonNotFound si la comparación no es
// del usuario, ErrItemNotFound si el producto no estaba.
func (r *Repository) RemoveItem(ctx context.Context, userID, id, productID int32) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `UPDATE product_comparison SET updated_at = NOW() WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrComparisonNotFound
	}
	tag, err = tx.Exec(ctx, `DELETE FROM product_comparison_item WHERE comparison_id = $1 AND product_id = $2`, id, productID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrItemNotFound
	}
	return tx.Commit(ctx)
}
