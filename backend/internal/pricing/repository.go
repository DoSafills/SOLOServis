package pricing

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository accede a product_offer y product_price_history. Todas las
// consultas son parametrizadas y los montos viajan como texto decimal.
type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

const offerColumnsSQL = `
SELECT o.id, p.public_id, o.store_id, s.name, o.price::text, o.list_price::text,
       o.currency, COALESCE(o.shipping_cost, 0)::text, o.shipping_free,
       (o.price + CASE WHEN o.shipping_free THEN 0 ELSE COALESCE(o.shipping_cost, 0) END)::text,
       o.available, o.stock, o.condition, o.product_url, o.last_updated
FROM product_offer o
JOIN product p ON p.id = o.product_id
JOIN store s ON s.id = o.store_id`

func scanOffer(row pgx.Row) (Offer, error) {
	var (
		offer       Offer
		productID   pgtype.UUID
		lastUpdated time.Time
	)
	if err := row.Scan(&offer.ID, &productID, &offer.StoreID, &offer.StoreName, &offer.Price,
		&offer.ListPrice, &offer.Currency, &offer.ShippingCost, &offer.ShippingFree, &offer.TotalPrice,
		&offer.Available, &offer.Stock, &offer.Condition, &offer.ProductURL, &lastUpdated); err != nil {
		return Offer{}, err
	}
	offer.ProductID = productID.String()
	offer.LastUpdated = lastUpdated.UTC().Format(time.RFC3339)
	return offer, nil
}

// ResolveProduct acepta el UUID público o el id numérico de un producto
// activo (como antes de TAL-74) y devuelve su id interno.
func (r *Repository) ResolveProduct(ctx context.Context, key string) (int32, error) {
	var id int32
	var err error
	if number, convErr := strconv.ParseInt(key, 10, 32); convErr == nil {
		err = r.db.QueryRow(ctx, `SELECT id FROM product WHERE id = $1 AND active`, number).Scan(&id)
	} else {
		err = r.db.QueryRow(ctx, `SELECT id FROM product WHERE public_id = $1::uuid AND active`, key).Scan(&id)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrProductNotFound
	}
	return id, err
}

// StoreExists indica si la tienda existe y está activa.
func (r *Repository) StoreExists(ctx context.Context, storeID int32) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM store WHERE id = $1 AND active)`, storeID).Scan(&exists)
	return exists, err
}

// GetOffer devuelve una oferta por id, o ErrOfferNotFound.
func (r *Repository) GetOffer(ctx context.Context, offerID int32) (Offer, error) {
	offer, err := scanOffer(r.db.QueryRow(ctx, offerColumnsSQL+` WHERE o.id = $1`, offerID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Offer{}, ErrOfferNotFound
	}
	return offer, err
}

// ListProductOffers devuelve las ofertas de un producto en tiendas activas:
// disponibles primero y luego por precio.
func (r *Repository) ListProductOffers(ctx context.Context, productID int32) ([]Offer, error) {
	rows, err := r.db.Query(ctx, offerColumnsSQL+`
		WHERE o.product_id = $1 AND s.active
		ORDER BY o.available DESC, o.price ASC, o.id ASC`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	offers := make([]Offer, 0)
	for rows.Next() {
		offer, err := scanOffer(rows)
		if err != nil {
			return nil, err
		}
		offers = append(offers, offer)
	}
	return offers, rows.Err()
}

// BestOffer devuelve la oferta disponible con menor precio total (precio +
// envío), o ErrNoAvailable.
func (r *Repository) BestOffer(ctx context.Context, productID int32) (Offer, error) {
	offer, err := scanOffer(r.db.QueryRow(ctx, offerColumnsSQL+`
		WHERE o.product_id = $1 AND o.available AND s.active
		ORDER BY (o.price + CASE WHEN o.shipping_free THEN 0 ELSE COALESCE(o.shipping_cost, 0) END) ASC,
		         o.price ASC, o.id ASC
		LIMIT 1`, productID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Offer{}, ErrNoAvailable
	}
	return offer, err
}

// UpsertOffer registra el precio actual: crea la oferta o actualiza la que
// tenga el mismo producto, tienda y URL normalizada (índice único de TAL-63/64).
// Devuelve el precio anterior (nil si la oferta es nueva). El trigger de la
// migración 010 agrega el historial solo si el precio cambió.
func (r *Repository) UpsertOffer(ctx context.Context, w OfferWrite) (int32, bool, *string, error) {
	var (
		id       int32
		inserted bool
		previous *string
	)
	err := r.db.QueryRow(ctx, `
		WITH previous AS (
		    SELECT price FROM product_offer
		    WHERE product_id = $1 AND store_id = $2
		      AND COALESCE(normalize_product_offer_url(product_url), '') =
		          COALESCE(normalize_product_offer_url($11::text), '')
		)
		INSERT INTO product_offer (product_id, store_id, price, list_price, currency, shipping_cost,
		                           shipping_free, available, stock, condition, product_url, last_updated)
		VALUES ($1, $2, $3::numeric, $4::numeric, $5, $6::numeric, $7, $8, $9, $10,
		        normalize_product_offer_url($11::text), NOW())
		ON CONFLICT (product_id, store_id, (COALESCE(normalize_product_offer_url(product_url), '')))
		DO UPDATE SET price = EXCLUDED.price, list_price = EXCLUDED.list_price,
		              currency = EXCLUDED.currency, shipping_cost = EXCLUDED.shipping_cost,
		              shipping_free = EXCLUDED.shipping_free, available = EXCLUDED.available,
		              stock = EXCLUDED.stock, condition = EXCLUDED.condition,
		              product_url = EXCLUDED.product_url, last_updated = NOW()
		RETURNING id, (xmax = 0), (SELECT price::text FROM previous)`,
		w.ProductID, w.StoreID, w.Price, w.ListPrice, w.Currency, w.ShippingCost, w.ShippingFree,
		w.Available, w.Stock, w.Condition, w.ProductURL).Scan(&id, &inserted, &previous)
	return id, inserted, previous, err
}

// UpdateOffer reemplaza los datos de una oferta y devuelve el precio anterior,
// o ErrOfferNotFound. El CTE lee la fila con el snapshot de la sentencia (el
// valor previo); con FOR UPDATE devolvería NULL porque la misma sentencia la
// modifica. El UPDATE ya bloquea la fila.
func (r *Repository) UpdateOffer(ctx context.Context, offerID int32, w OfferWrite) (string, error) {
	var previous *string
	err := r.db.QueryRow(ctx, `
		WITH previous AS (SELECT price FROM product_offer WHERE id = $1)
		UPDATE product_offer
		SET price = $2::numeric, list_price = $3::numeric, currency = $4, shipping_cost = $5::numeric,
		    shipping_free = $6, available = $7, stock = $8, condition = $9,
		    product_url = normalize_product_offer_url($10::text), last_updated = NOW()
		WHERE id = $1
		RETURNING (SELECT price::text FROM previous)`,
		offerID, w.Price, w.ListPrice, w.Currency, w.ShippingCost, w.ShippingFree, w.Available,
		w.Stock, w.Condition, w.ProductURL).Scan(&previous)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrOfferNotFound
	}
	if err != nil || previous == nil {
		return "", err
	}
	return *previous, nil
}

// Watcher es un usuario que pidió aviso cuando el producto baje de su precio objetivo.
type Watcher struct {
	UserID      int32
	ProductID   string
	ProductName string
	TargetPrice string
}

// PriceDropWatchers devuelve los usuarios cuyo precio objetivo quedó cruzado
// por la bajada de oldPrice a newPrice (oldPrice > objetivo >= newPrice).
func (r *Repository) PriceDropWatchers(ctx context.Context, offerID int32, oldPrice, newPrice string) ([]Watcher, error) {
	rows, err := r.db.Query(ctx, `
		SELECT i.user_id, p.public_id::text, p.name, i.target_price::text
		FROM product_offer o
		JOIN product p ON p.id = o.product_id
		JOIN product_user_interest i ON i.product_id = p.id
		WHERE o.id = $1 AND i.notify_price_drop AND i.target_price IS NOT NULL
		  AND $2::numeric > i.target_price AND $3::numeric <= i.target_price`,
		offerID, oldPrice, newPrice)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var watchers []Watcher
	for rows.Next() {
		var w Watcher
		if err := rows.Scan(&w.UserID, &w.ProductID, &w.ProductName, &w.TargetPrice); err != nil {
			return nil, err
		}
		watchers = append(watchers, w)
	}
	return watchers, rows.Err()
}

const historyDateFilterSQL = `
  AND ($2::date IS NULL OR h.recorded_at >= $2::date)
  AND ($3::date IS NULL OR h.recorded_at < $3::date + 1)`

// OfferHistory devuelve el historial de una oferta (más reciente primero) y
// su total, o ErrOfferNotFound.
func (r *Repository) OfferHistory(ctx context.Context, offerID int32, f HistoryFilter) ([]PricePoint, int64, error) {
	var exists bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM product_offer WHERE id = $1)`, offerID).Scan(&exists); err != nil {
		return nil, 0, err
	}
	if !exists {
		return nil, 0, ErrOfferNotFound
	}

	var total int64
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM product_price_history h WHERE h.product_offer_id = $1`+
		historyDateFilterSQL, offerID, f.From, f.To).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT h.id, h.price::text, h.is_promotional, h.recorded_at
		FROM product_price_history h
		WHERE h.product_offer_id = $1`+historyDateFilterSQL+`
		ORDER BY h.recorded_at DESC, h.id DESC
		LIMIT $4 OFFSET $5`, offerID, f.From, f.To, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	points := make([]PricePoint, 0)
	for rows.Next() {
		var p PricePoint
		var recordedAt time.Time
		if err := rows.Scan(&p.ID, &p.Price, &p.IsPromotional, &recordedAt); err != nil {
			return nil, 0, err
		}
		p.RecordedAt = recordedAt.UTC().Format(time.RFC3339Nano)
		points = append(points, p)
	}
	return points, total, rows.Err()
}

// ProductHistory devuelve el historial de un producto en todas sus tiendas
// (más reciente primero) y su total.
func (r *Repository) ProductHistory(ctx context.Context, productID int32, f HistoryFilter) ([]ProductPricePoint, int64, error) {
	const from = `
		FROM product_price_history h
		JOIN product_offer o ON o.id = h.product_offer_id
		JOIN store s ON s.id = o.store_id
		WHERE o.product_id = $1`

	var total int64
	if err := r.db.QueryRow(ctx, `SELECT count(*)`+from+historyDateFilterSQL,
		productID, f.From, f.To).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT h.id, h.price::text, h.is_promotional, h.recorded_at, o.id, s.id, s.name`+from+historyDateFilterSQL+`
		ORDER BY h.recorded_at DESC, h.id DESC
		LIMIT $4 OFFSET $5`, productID, f.From, f.To, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	points := make([]ProductPricePoint, 0)
	for rows.Next() {
		var p ProductPricePoint
		var recordedAt time.Time
		if err := rows.Scan(&p.ID, &p.Price, &p.IsPromotional, &recordedAt, &p.OfferID, &p.StoreID, &p.StoreName); err != nil {
			return nil, 0, err
		}
		p.RecordedAt = recordedAt.UTC().Format(time.RFC3339Nano)
		points = append(points, p)
	}
	return points, total, rows.Err()
}
