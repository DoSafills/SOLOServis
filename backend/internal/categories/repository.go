package categories

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound indica que la categoría no existe o está inactiva.
var ErrNotFound = errors.New("category not found")

// PostgresRepository lee categorías y atributos desde PostgreSQL.
// Todas las consultas son parametrizadas.
type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

const countCategoriesSQL = `
SELECT count(*)
FROM categories c
WHERE c.active
  AND ($1::text IS NULL OR c.item_type = $1)`

const listCategoriesSQL = `
SELECT c.slug, c.name, c.item_type, c.description, count(a.id)
FROM categories c
LEFT JOIN category_attributes a ON a.category_id = c.id
WHERE c.active
  AND ($1::text IS NULL OR c.item_type = $1)
GROUP BY c.id
ORDER BY c.name, c.id
LIMIT $2 OFFSET $3`

// List devuelve una página de categorías activas y el total sin paginar.
// Usa siempre dos consultas, independiente del número de categorías.
func (r *PostgresRepository) List(ctx context.Context, params ListParams) ([]CategorySummary, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx, countCategoriesSQL, params.ItemType).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, listCategoriesSQL, params.ItemType, params.Limit, params.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]CategorySummary, 0, params.Limit)
	for rows.Next() {
		var item CategorySummary
		if err := rows.Scan(&item.Slug, &item.Name, &item.ItemType, &item.Description, &item.AttributeCount); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

const getCategoryBySlugSQL = `
SELECT id, slug, name, item_type, description
FROM categories
WHERE slug = $1 AND active`

const listAttributesSQL = `
SELECT id, key, label, data_type, unit, required, comparable, filterable,
       min_value::float8, max_value::float8, max_length, allowed_values, display_order
FROM category_attributes
WHERE category_id = $1
ORDER BY display_order, id`

// GetBySlug devuelve la categoría activa con sus atributos (dos consultas).
func (r *PostgresRepository) GetBySlug(ctx context.Context, slug string) (CategoryDetail, error) {
	var (
		id     int32
		detail CategoryDetail
	)
	err := r.db.QueryRow(ctx, getCategoryBySlugSQL, slug).
		Scan(&id, &detail.Slug, &detail.Name, &detail.ItemType, &detail.Description)
	if errors.Is(err, pgx.ErrNoRows) {
		return CategoryDetail{}, ErrNotFound
	}
	if err != nil {
		return CategoryDetail{}, err
	}

	rows, err := r.db.Query(ctx, listAttributesSQL, id)
	if err != nil {
		return CategoryDetail{}, err
	}
	defer rows.Close()

	detail.Attributes = make([]Attribute, 0)
	for rows.Next() {
		var a Attribute
		if err := rows.Scan(&a.ID, &a.Key, &a.Label, &a.DataType, &a.Unit, &a.Required, &a.Comparable,
			&a.Filterable, &a.Min, &a.Max, &a.MaxLength, &a.AllowedValues, &a.DisplayOrder); err != nil {
			return CategoryDetail{}, err
		}
		detail.Attributes = append(detail.Attributes, a)
	}
	if err := rows.Err(); err != nil {
		return CategoryDetail{}, err
	}

	return detail, nil
}
