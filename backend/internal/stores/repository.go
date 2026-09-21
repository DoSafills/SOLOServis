package stores

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

type Store struct {
	ID                  int32   `json:"id"`
	Name                string  `json:"name"`
	Website             string  `json:"website"`
	Logo                string  `json:"logo"`
	Rating              float64 `json:"rating"`
	Reputation          string  `json:"reputation"`
	ShippingInformation string  `json:"shippingInformation"`
	GeneralConditions   string  `json:"generalConditions"`
	ProductCount        int64   `json:"productCount"`
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

const storeQuery = `
SELECT s.id, s.name, COALESCE(s.website_url, ''), COALESCE(s.logo_url, ''),
       COALESCE(s.rating, 0)::float8, COALESCE(s.reputation, ''),
       COALESCE(s.shipping_information, ''), COALESCE(s.general_conditions, ''),
       COUNT(po.id)::bigint
FROM store s
LEFT JOIN product_offer po ON po.store_id = s.id
WHERE s.active = true
`

func (r *Repository) List(ctx context.Context) ([]Store, error) {
	rows, err := r.db.Query(ctx, storeQuery+" GROUP BY s.id ORDER BY s.name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]Store, 0)
	for rows.Next() {
		var store Store
		if err := rows.Scan(&store.ID, &store.Name, &store.Website, &store.Logo,
			&store.Rating, &store.Reputation, &store.ShippingInformation,
			&store.GeneralConditions, &store.ProductCount); err != nil {
			return nil, err
		}
		result = append(result, store)
	}
	return result, rows.Err()
}

func (r *Repository) Get(ctx context.Context, id int32) (Store, error) {
	var store Store
	err := r.db.QueryRow(ctx, `
SELECT s.id, s.name, COALESCE(s.website_url, ''), COALESCE(s.logo_url, ''),
	   COALESCE(s.rating, 0)::float8, COALESCE(s.reputation, ''),
	   COALESCE(s.shipping_information, ''), COALESCE(s.general_conditions, ''),
	   COUNT(po.id)::bigint
FROM store s
LEFT JOIN product_offer po ON po.store_id = s.id
WHERE s.active = true AND s.id = $1
GROUP BY s.id`, id).Scan(
		&store.ID, &store.Name, &store.Website, &store.Logo, &store.Rating,
		&store.Reputation, &store.ShippingInformation, &store.GeneralConditions,
		&store.ProductCount,
	)
	return store, err
}
