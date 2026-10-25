package stores

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/DoSafills/SOLOServis/backend/internal/stores/dto"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db      *pgxpool.Pool
	queries *generated.Queries
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		db:      db,
		queries: generated.New(db),
	}
}

// Consultas de TAL-73. Todas parametrizadas; las piezas comunes son
// constantes del paquete, nunca entrada del usuario.

// storeColumnsSQL calcula productCount y reviewCount igual en el listado y en
// el detalle: productCount cuenta productos activos con oferta disponible y
// reviewCount sale de store_review (antes siempre valía 0).
const storeColumnsSQL = `
SELECT s.id, s.name, s.website_url, s.logo_url, s.rating, s.reputation,
       s.shipping_information, s.general_conditions,
       (SELECT count(DISTINCT po.product_id)
          FROM product_offer po
          JOIN product p ON p.id = po.product_id AND p.active
         WHERE po.store_id = s.id AND po.available) AS product_count,
       (SELECT count(*) FROM store_review r WHERE r.store_id = s.id) AS review_count
FROM store s`

const listStoresSQL = storeColumnsSQL + `
WHERE s.active
ORDER BY s.id
LIMIT $1 OFFSET $2`

const getStoreSQL = storeColumnsSQL + `
WHERE s.id = $1 AND s.active`

// offerColumnsSQL es común a las ofertas por producto y por tienda.
const offerColumnsSQL = `
SELECT po.id, p.public_id, p.name, po.price, po.list_price, po.currency,
       po.shipping_cost, po.shipping_free, po.available, po.stock, po.condition,
       po.product_url, po.last_updated,
       s.id, s.name, s.logo_url, s.rating, s.website_url,
       (SELECT count(*) FROM store_review r WHERE r.store_id = s.id)
FROM product_offer po
JOIN product p ON p.id = po.product_id AND p.active
JOIN store s ON s.id = po.store_id AND s.active`

const productOffersWhereSQL = `
WHERE p.public_id = $1 AND ($2::bool IS NULL OR po.available = $2::bool)`

const countProductOffersSQL = `SELECT count(*) FROM product_offer po
JOIN product p ON p.id = po.product_id AND p.active
JOIN store s ON s.id = po.store_id AND s.active` + productOffersWhereSQL

// Disponibles primero y luego la más barata: lo que necesita el comparador.
const listProductOffersSQL = offerColumnsSQL + productOffersWhereSQL + `
ORDER BY po.available DESC, po.price ASC, s.name ASC, po.id ASC
LIMIT $3 OFFSET $4`

const storeOffersWhereSQL = `
WHERE po.store_id = $1 AND ($2::bool IS NULL OR po.available = $2::bool)`

const countStoreOffersSQL = `SELECT count(*) FROM product_offer po
JOIN product p ON p.id = po.product_id AND p.active
JOIN store s ON s.id = po.store_id AND s.active` + storeOffersWhereSQL

const listStoreOffersSQL = offerColumnsSQL + storeOffersWhereSQL + `
ORDER BY po.available DESC, p.name ASC, po.price ASC, po.id ASC
LIMIT $3 OFFSET $4`

func scanStore(row pgx.Row) (dto.Store, error) {
	var (
		id                        int32
		name                      string
		website, logo, reputation pgtype.Text
		shipping, conditions      pgtype.Text
		rating                    pgtype.Numeric
		productCount, reviewCount int64
	)
	if err := row.Scan(&id, &name, &website, &logo, &rating, &reputation, &shipping, &conditions,
		&productCount, &reviewCount); err != nil {
		return dto.Store{}, err
	}
	return dto.Store{
		ID:           strconv.Itoa(int(id)),
		Name:         name,
		Logo:         dto.TextValue(logo),
		Rating:       dto.NumericValue(rating),
		ReviewCount:  reviewCount,
		ProductCount: productCount,
		Reputation:   dto.TextValue(reputation),
		DispatchTime: dto.TextValue(shipping),
		Conditions:   dto.TextValue(conditions),
		Website:      dto.TextValue(website),
	}, nil
}

// List devuelve una página de tiendas activas y el total (2 consultas).
func (r *Repository) List(ctx context.Context, page Page) ([]dto.Store, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM store WHERE active`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, listStoresSQL, page.Limit, page.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	stores := make([]dto.Store, 0, page.Limit)
	for rows.Next() {
		store, err := scanStore(rows)
		if err != nil {
			return nil, 0, err
		}
		stores = append(stores, store)
	}
	return stores, total, rows.Err()
}

// GetDetail devuelve una tienda activa con sus productos disponibles y sus
// sucursales (3 consultas), o ErrNotFound.
func (r *Repository) GetDetail(ctx context.Context, storeID int32) (dto.StoreDetail, error) {
	store, err := scanStore(r.db.QueryRow(ctx, getStoreSQL, storeID))
	if errors.Is(err, pgx.ErrNoRows) {
		return dto.StoreDetail{}, ErrNotFound
	}
	if err != nil {
		return dto.StoreDetail{}, err
	}

	products, err := r.queries.ListStoreProducts(ctx, storeID)
	if err != nil {
		return dto.StoreDetail{}, err
	}
	locations, err := r.queries.ListStoreLocations(ctx, storeID)
	if err != nil {
		return dto.StoreDetail{}, err
	}

	detail := dto.StoreDetail{
		Store:     store,
		Products:  make([]dto.StoreProduct, 0, len(products)),
		Locations: make([]dto.StoreLocation, 0, len(locations)),
	}
	for _, product := range products {
		detail.Products = append(detail.Products, dto.FromStoreProduct(product))
	}
	for _, location := range locations {
		detail.Locations = append(detail.Locations, dto.FromStoreLocation(location))
	}
	return detail, nil
}

// ListProductOffers devuelve las ofertas de un producto activo en todas las
// tiendas activas, cada una con su tienda resumida, o ErrProductNotFound.
func (r *Repository) ListProductOffers(ctx context.Context, productID pgtype.UUID, filter OfferFilter) ([]dto.Offer, int64, error) {
	var exists bool
	if err := r.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM product WHERE public_id = $1 AND active)`, productID).Scan(&exists); err != nil {
		return nil, 0, err
	}
	if !exists {
		return nil, 0, ErrProductNotFound
	}
	return r.listOffers(ctx, countProductOffersSQL, listProductOffersSQL, productID, filter, true)
}

// ListStoreOffers devuelve las ofertas (disponibles o no) de una tienda
// activa, o ErrNotFound.
func (r *Repository) ListStoreOffers(ctx context.Context, storeID int32, filter OfferFilter) ([]dto.Offer, int64, error) {
	var exists bool
	if err := r.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM store WHERE id = $1 AND active)`, storeID).Scan(&exists); err != nil {
		return nil, 0, err
	}
	if !exists {
		return nil, 0, ErrNotFound
	}
	return r.listOffers(ctx, countStoreOffersSQL, listStoreOffersSQL, storeID, filter, false)
}

func (r *Repository) listOffers(ctx context.Context, countSQL, listSQL string, key any, filter OfferFilter, withStore bool) ([]dto.Offer, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx, countSQL, key, filter.Available).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, listSQL, key, filter.Available, filter.Limit, filter.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	offers := make([]dto.Offer, 0, filter.Limit)
	for rows.Next() {
		var (
			offer                          dto.Offer
			productID                      pgtype.UUID
			price, listPrice, shippingCost pgtype.Numeric
			stock                          pgtype.Int4
			productURL                     pgtype.Text
			lastUpdated                    pgtype.Timestamptz
			storeID                        int32
			storeName                      string
			storeLogo, storeWebsite        pgtype.Text
			storeRating                    pgtype.Numeric
			storeReviews                   int64
		)
		if err := rows.Scan(&offer.OfferID, &productID, &offer.ProductName, &price, &listPrice,
			&offer.Currency, &shippingCost, &offer.ShippingFree, &offer.Available, &stock,
			&offer.Condition, &productURL, &lastUpdated,
			&storeID, &storeName, &storeLogo, &storeRating, &storeWebsite, &storeReviews); err != nil {
			return nil, 0, err
		}
		offer.ProductID = productID.String()
		offer.Price = dto.NumericString(price)
		offer.ListPrice = dto.NumericString(listPrice)
		offer.ShippingCost = dto.NumericString(shippingCost)
		offer.Stock = dto.IntValue(stock)
		offer.ProductURL = dto.TextValue(productURL)
		if lastUpdated.Valid {
			offer.LastUpdated = lastUpdated.Time.UTC().Format(time.RFC3339)
		}
		if withStore {
			offer.Store = &dto.StoreSummary{
				ID:          strconv.Itoa(int(storeID)),
				Name:        storeName,
				Logo:        dto.TextValue(storeLogo),
				Rating:      dto.NumericValue(storeRating),
				ReviewCount: storeReviews,
				Website:     dto.TextValue(storeWebsite),
			}
		}
		offers = append(offers, offer)
	}
	return offers, total, rows.Err()
}

// ListOffers devuelve todas las ofertas de una tienda, disponibles o no
// (consulta sqlc de TAL-63/64, sin paginar).
func (r *Repository) ListOffers(ctx context.Context, storeID int32) ([]generated.ListStoreOffersRow, error) {
	return r.queries.ListStoreOffers(ctx, storeID)
}
