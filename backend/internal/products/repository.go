package products

import (
	"context"

	"github.com/DoSafills/SOLOServis/backend/internal/categories"
	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db         *pgxpool.Pool
	queries    *generated.Queries
	categories *categories.PostgresRepository
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		db:         db,
		queries:    generated.New(db),
		categories: categories.NewRepository(db),
	}
}

// ResolveScope traduce un id o slug de categoría (incluido un id legacy) a
// las hojas (nivel 3) que abarca, con las reglas de categories.ResolveScope.
func (r *Repository) ResolveScope(ctx context.Context, ref categories.Ref) (categories.Scope, error) {
	return r.categories.ResolveScope(ctx, ref)
}

func (r *Repository) ListCategories(ctx context.Context) ([]generated.ListProductCategoriesRow, error) {
	return r.queries.ListProductCategories(ctx)
}

// ListCategoryCounts devuelve los productos activos de cada nodo (hoja y
// descendientes) indexados por id de product_category.
func (r *Repository) ListCategoryCounts(ctx context.Context) (map[int32]int64, error) {
	rows, err := r.queries.ListProductCategoryCounts(ctx)
	if err != nil {
		return nil, err
	}

	counts := make(map[int32]int64, len(rows))
	for _, row := range rows {
		counts[row.CategoryID] = row.ProductCount
	}
	return counts, nil
}

func (r *Repository) GetByPublicID(ctx context.Context, publicID pgtype.UUID) (generated.Product, error) {
	return r.queries.GetProductByPublicID(ctx, publicID)
}

func (r *Repository) GetDetailByPublicID(ctx context.Context, publicID pgtype.UUID) (generated.GetProductDetailByPublicIDRow, error) {
	return r.queries.GetProductDetailByPublicID(ctx, publicID)
}

func (r *Repository) ListOffers(ctx context.Context, productID int32) ([]generated.ListProductOffersRow, error) {
	return r.queries.ListProductOffers(ctx, productID)
}

func (r *Repository) GetOfferByID(ctx context.Context, offerID int32) (generated.ProductOffer, error) {
	return r.queries.GetOfferByID(ctx, offerID)
}

func (r *Repository) UpsertOffer(ctx context.Context, params generated.UpsertProductOfferParams) (generated.ProductOffer, error) {
	if err := ValidateCurrencyCode(params.Currency); err != nil {
		return generated.ProductOffer{}, err
	}
	if params.ProductUrl.Valid {
		params.ProductUrl.String = NormalizeProductOfferURL(params.ProductUrl.String)
	}
	return r.queries.UpsertProductOffer(ctx, params)
}

func (r *Repository) UpdateOfferPriceAvailability(ctx context.Context, params generated.UpdateProductOfferPriceAvailabilityParams) (generated.ProductOffer, error) {
	return r.queries.UpdateProductOfferPriceAvailability(ctx, params)
}

func (r *Repository) ListImages(ctx context.Context, productID int32) ([]generated.ProductImage, error) {
	return r.queries.ListProductImages(ctx, productID)
}

func (r *Repository) ListReviews(ctx context.Context, productID int32) ([]generated.ListProductReviewsRow, error) {
	return r.queries.ListProductReviews(ctx, productID)
}

func (r *Repository) ListSpecifications(ctx context.Context, productID int32) ([]generated.ListProductSpecificationsRow, error) {
	return r.queries.ListProductSpecifications(ctx, productID)
}

func (r *Repository) ListPriceHistory(ctx context.Context, productID int32) ([]generated.ListProductPriceHistoryRow, error) {
	return r.queries.ListProductPriceHistory(ctx, productID)
}
