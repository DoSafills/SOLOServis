package services

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

type Service struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	Provider         string         `json:"provider"`
	Category         string         `json:"category"`
	Description      string         `json:"description"`
	MonthlyPrice     float64        `json:"monthlyPrice"`
	InstallationCost float64        `json:"installationCost"`
	ContractPeriod   string         `json:"contractPeriod"`
	Rating           float64        `json:"rating"`
	ReviewCount      int64          `json:"reviewCount"`
	Coverage         string         `json:"coverage"`
	Image            string         `json:"image"`
	Offers           []ServiceOffer `json:"offers,omitempty"`
}

type ServiceOffer struct {
	Provider         string  `json:"provider"`
	Price            float64 `json:"price"`
	Currency         string  `json:"currency"`
	BillingPeriod    string  `json:"billingPeriod"`
	InstallationCost float64 `json:"installationCost"`
	ContractPeriod   string  `json:"contractPeriod"`
	Available        bool    `json:"available"`
	Coverage         string  `json:"coverage"`
	AdditionalCosts  string  `json:"additionalCosts"`
	URL              string  `json:"url"`
}

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

const serviceListQuery = `
SELECT s.public_id, s.name, COALESCE((array_agg(p.name ORDER BY so.price))[1], ''), sc.name,
       COALESCE(s.description, ''), COALESCE(MIN(so.price) FILTER (WHERE so.available), 0)::float8,
       COALESCE(srs.derived_average_rating, 0)::float8,
       COALESCE(srs.derived_review_count, 0), COALESCE(s.image_url, '')
FROM service s
JOIN service_category sc ON sc.id = s.category_id
LEFT JOIN service_offer so ON so.service_id = s.id
LEFT JOIN provider p ON p.id = so.provider_id
LEFT JOIN service_rating_summary srs ON srs.service_id = s.id
WHERE s.active = true
  AND ($1::text IS NULL OR sc.name = $1)
GROUP BY s.id, sc.name, srs.derived_average_rating, srs.derived_review_count
`

func (r *Repository) List(ctx context.Context, category string) ([]Service, error) {
	var categoryArg *string
	if category != "" {
		categoryArg = &category
	}
	rows, err := r.db.Query(ctx, serviceListQuery+" ORDER BY s.name", categoryArg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]Service, 0)
	for rows.Next() {
		var id pgtype.UUID
		var service Service
		if err := rows.Scan(&id, &service.Name, &service.Provider, &service.Category,
			&service.Description, &service.MonthlyPrice, &service.Rating,
			&service.ReviewCount, &service.Image); err != nil {
			return nil, err
		}
		service.ID = id.String()
		result = append(result, service)
	}
	return result, rows.Err()
}

func (r *Repository) Get(ctx context.Context, publicID pgtype.UUID) (Service, error) {
	var id pgtype.UUID
	var service Service
	err := r.db.QueryRow(ctx, `
SELECT s.public_id, s.name, COALESCE((array_agg(p.name ORDER BY so.price))[1], ''), sc.name,
       COALESCE(s.description, ''), COALESCE(MIN(so.price) FILTER (WHERE so.available), 0)::float8,
       COALESCE(srs.derived_average_rating, 0)::float8,
       COALESCE(srs.derived_review_count, 0), COALESCE(s.image_url, '')
FROM service s
JOIN service_category sc ON sc.id = s.category_id
LEFT JOIN service_offer so ON so.service_id = s.id
LEFT JOIN provider p ON p.id = so.provider_id
LEFT JOIN service_rating_summary srs ON srs.service_id = s.id
WHERE s.active = true AND s.public_id = $1
GROUP BY s.id, sc.name, srs.derived_average_rating, srs.derived_review_count`, publicID).Scan(
		&id, &service.Name, &service.Provider, &service.Category, &service.Description,
		&service.MonthlyPrice, &service.Rating, &service.ReviewCount, &service.Image,
	)
	if err != nil {
		return Service{}, err
	}
	service.ID = id.String()
	service.Offers, err = r.listOffers(ctx, publicID)
	return service, err
}

func (r *Repository) listOffers(ctx context.Context, publicID pgtype.UUID) ([]ServiceOffer, error) {
	const query = `SELECT p.name, so.price::float8, so.currency, COALESCE(so.billing_period, ''), COALESCE(so.installation_cost, 0)::float8, COALESCE(so.contract_period, ''), so.available, COALESCE(so.coverage_summary, ''), COALESCE(so.additional_costs_summary, ''), COALESCE(so.service_url, '') FROM service_offer so JOIN service s ON s.id = so.service_id JOIN provider p ON p.id = so.provider_id WHERE s.public_id = $1 ORDER BY so.price`
	rows, err := r.db.Query(ctx, query, publicID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]ServiceOffer, 0)
	for rows.Next() {
		var offer ServiceOffer
		if err := rows.Scan(&offer.Provider, &offer.Price, &offer.Currency, &offer.BillingPeriod, &offer.InstallationCost, &offer.ContractPeriod, &offer.Available, &offer.Coverage, &offer.AdditionalCosts, &offer.URL); err != nil {
			return nil, err
		}
		result = append(result, offer)
	}
	return result, rows.Err()
}
