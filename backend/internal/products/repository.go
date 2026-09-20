package products

import (
	"context"

	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	queries *generated.Queries
	db      *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		queries: generated.New(db),
		db:      db,
	}
}

type ProductSpecification struct {
	Name  string
	Value string
}

type ProductPriceHistoryPoint struct {
	Price         pgtype.Numeric
	IsPromotional bool
	RecordedAt    pgtype.Timestamp
}

func (r *Repository) List(ctx context.Context) ([]generated.ListProductsRow, error) {
	return r.queries.ListProducts(ctx)
}

func (r *Repository) GetByPublicID(ctx context.Context, publicID pgtype.UUID) (generated.Product, error) {
	return r.queries.GetProductByPublicID(ctx, publicID)
}

func (r *Repository) ListOffers(ctx context.Context, productID int32) ([]generated.ListProductOffersRow, error) {
	return r.queries.ListProductOffers(ctx, productID)
}

func (r *Repository) ListImages(ctx context.Context, productID int32) ([]generated.ProductImage, error) {
	return r.queries.ListProductImages(ctx, productID)
}
func (r *Repository) GetDetailByPublicID(ctx context.Context, publicID pgtype.UUID) (generated.GetProductDetailByPublicIDRow, error) {
	return r.queries.GetProductDetailByPublicID(ctx, publicID)
}

func (r *Repository) ListSpecifications(ctx context.Context, productID int32) ([]ProductSpecification, error) {
	rows, err := r.db.Query(ctx, `
		SELECT pcs.name, psv.value
		FROM product_specification_value psv
		JOIN product_category_specification pcs ON pcs.id = psv.specification_id
		WHERE psv.product_id = $1
		ORDER BY pcs.display_order, pcs.id`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]ProductSpecification, 0)
	for rows.Next() {
		var specification ProductSpecification
		if err := rows.Scan(&specification.Name, &specification.Value); err != nil {
			return nil, err
		}
		result = append(result, specification)
	}
	return result, rows.Err()
}

func (r *Repository) ListPriceHistory(ctx context.Context, productID int32) ([]ProductPriceHistoryPoint, error) {
	rows, err := r.db.Query(ctx, `
		SELECT pph.price, pph.is_promotional, pph.recorded_at
		FROM product_price_history pph
		JOIN product_offer po ON po.id = pph.product_offer_id
		WHERE po.product_id = $1
		ORDER BY pph.recorded_at ASC, pph.id ASC`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]ProductPriceHistoryPoint, 0)
	for rows.Next() {
		var point ProductPriceHistoryPoint
		if err := rows.Scan(&point.Price, &point.IsPromotional, &point.RecordedAt); err != nil {
			return nil, err
		}
		result = append(result, point)
	}
	return result, rows.Err()
}
