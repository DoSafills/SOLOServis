package products

import (
	"context"
	"errors"
	"strings"

	"github.com/DoSafills/SOLOServis/backend/internal/categories"
	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/DoSafills/SOLOServis/backend/internal/products/dto"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Consultas de TAL-70. Todas parametrizadas; las piezas comunes son
// constantes del paquete, nunca entrada del usuario.

// productFilterSQL filtra productos activos por categoría antigua (con sus
// subcategorías, $1) o por slug de la categoría nueva ($2). NULL = sin filtro.
const productFilterSQL = `
WITH RECURSIVE subtree AS (
    SELECT c.id FROM product_category c
    WHERE c.id = $1::int AND c.active
    UNION ALL
    SELECT child.id FROM product_category child
    JOIN subtree s ON child.parent_category_id = s.id
    WHERE child.active
)
`

const productWhereSQL = `
WHERE p.active
  AND ($1::int IS NULL OR p.category_id IN (SELECT id FROM subtree))
  AND ($2::text IS NULL OR p.category_ref = (
        SELECT id FROM categories WHERE slug = $2::text AND active))
`

const countProductsSQL = productFilterSQL + `SELECT count(*) FROM product p` + productWhereSQL

const listProductsPageSQL = productFilterSQL + `
SELECT
    p.id, p.public_id, p.category_id, p.brand_id, p.name, p.model, p.sku,
    p.description, p.active, p.created_at, p.updated_at,
    b.name, pc.name, parent.name,
    COALESCE(prs.derived_average_rating, 0), COALESCE(prs.derived_review_count, 0),
    cat.slug
FROM product p
LEFT JOIN brand b ON b.id = p.brand_id
JOIN product_category pc ON pc.id = p.category_id
LEFT JOIN product_category parent ON parent.id = pc.parent_category_id
LEFT JOIN product_rating_summary prs ON prs.product_id = p.id
LEFT JOIN categories cat ON cat.id = p.category_ref` + productWhereSQL + `
ORDER BY p.id
LIMIT $3 OFFSET $4`

const listImagesByProductsSQL = `
SELECT id, product_id, image_url, alt_text, sort_order, active
FROM product_image
WHERE product_id = ANY($1) AND active
ORDER BY product_id, sort_order, id`

const listOffersByProductsSQL = `
SELECT po.id, po.product_id, po.store_id, s.name, po.price, po.list_price,
       po.currency, po.shipping_cost, po.shipping_free, po.available, po.stock,
       po.condition, po.product_url, po.last_updated
FROM product_offer po
JOIN store s ON s.id = po.store_id
WHERE po.product_id = ANY($1)
ORDER BY po.product_id, po.price`

const listSpecsByProductsSQL = `
SELECT psv.product_id, pcs.name, psv.value
FROM product_specification_value psv
JOIN product_category_specification pcs ON pcs.id = psv.specification_id
WHERE psv.product_id = ANY($1)
ORDER BY psv.product_id, pcs.display_order, pcs.id`

const listAttributeValuesSQL = `
SELECT pav.product_id, ca.key, ca.data_type, pav.value
FROM product_attribute_value pav
JOIN category_attributes ca ON ca.id = pav.attribute_id
WHERE pav.product_id = ANY($1)
ORDER BY pav.product_id, ca.display_order, ca.id`

// List devuelve una página de productos con imágenes, ofertas, specs y
// atributos. Usa 6 consultas fijas, sin importar el tamaño de la página.
func (r *Repository) List(ctx context.Context, params ListParams) ([]dto.ProductListItem, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx, countProductsSQL, params.LegacyCategoryID, params.CategorySlug).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, listProductsPageSQL,
		params.LegacyCategoryID, params.CategorySlug, params.Limit, params.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]dto.ProductListItem, 0, params.Limit)
	ids := make([]int32, 0, params.Limit)
	index := make(map[int32]int, params.Limit)
	for rows.Next() {
		var row generated.ListProductsRow
		var slug *string
		if err := rows.Scan(&row.ID, &row.PublicID, &row.CategoryID, &row.BrandID, &row.Name,
			&row.Model, &row.Sku, &row.Description, &row.Active, &row.CreatedAt, &row.UpdatedAt,
			&row.BrandName, &row.CategoryName, &row.ParentCategoryName, &row.Rating,
			&row.ReviewCount, &slug); err != nil {
			return nil, 0, err
		}
		item := dto.FromListProduct(row)
		item.CategorySlug = slug
		index[row.ID] = len(items)
		ids = append(ids, row.ID)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(ids) == 0 {
		return items, total, nil
	}

	err = r.eachRow(ctx, listImagesByProductsSQL, ids, func(row pgx.Rows) error {
		var image generated.ProductImage
		if err := row.Scan(&image.ID, &image.ProductID, &image.ImageUrl, &image.AltText,
			&image.SortOrder, &image.Active); err != nil {
			return err
		}
		item := &items[index[image.ProductID]]
		item.Images = append(item.Images, dto.FromProductImage(image))
		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	err = r.eachRow(ctx, listOffersByProductsSQL, ids, func(row pgx.Rows) error {
		var offer generated.ListProductOffersRow
		if err := row.Scan(&offer.ID, &offer.ProductID, &offer.StoreID, &offer.StoreName,
			&offer.Price, &offer.ListPrice, &offer.Currency, &offer.ShippingCost,
			&offer.ShippingFree, &offer.Available, &offer.Stock, &offer.Condition,
			&offer.ProductUrl, &offer.LastUpdated); err != nil {
			return err
		}
		item := &items[index[offer.ProductID]]
		item.Offers = append(item.Offers, dto.FromProductOffer(offer))
		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	err = r.eachRow(ctx, listSpecsByProductsSQL, ids, func(row pgx.Rows) error {
		var productID int32
		var name, value string
		if err := row.Scan(&productID, &name, &value); err != nil {
			return err
		}
		items[index[productID]].Specs[name] = value
		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	err = r.eachRow(ctx, listAttributeValuesSQL, ids, func(row pgx.Rows) error {
		var productID int32
		var key, dataType, value string
		if err := row.Scan(&productID, &key, &dataType, &value); err != nil {
			return err
		}
		items[index[productID]].Attributes[key] = dto.AttributeJSON(dataType, value)
		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// eachRow ejecuta una consulta por lote de productos y aplica fn a cada fila.
func (r *Repository) eachRow(ctx context.Context, sql string, ids []int32, fn func(pgx.Rows) error) error {
	rows, err := r.db.Query(ctx, sql, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := fn(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

// GetDetail devuelve un producto activo con todo su detalle, o ErrNotFound.
func (r *Repository) GetDetail(ctx context.Context, publicID pgtype.UUID) (dto.ProductDetail, error) {
	product, err := r.queries.GetProductDetailByPublicID(ctx, publicID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !product.Active) {
		return dto.ProductDetail{}, ErrNotFound
	}
	if err != nil {
		return dto.ProductDetail{}, err
	}

	result := dto.FromProduct(product)

	if err := r.db.QueryRow(ctx,
		`SELECT c.slug FROM product p JOIN categories c ON c.id = p.category_ref WHERE p.id = $1`,
		product.ID).Scan(&result.CategorySlug); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return dto.ProductDetail{}, err
	}

	images, err := r.queries.ListProductImages(ctx, product.ID)
	if err != nil {
		return dto.ProductDetail{}, err
	}
	for _, image := range images {
		result.Images = append(result.Images, dto.FromProductImage(image))
	}

	offers, err := r.queries.ListProductOffers(ctx, product.ID)
	if err != nil {
		return dto.ProductDetail{}, err
	}
	for _, offer := range offers {
		result.Offers = append(result.Offers, dto.FromProductOffer(offer))
	}

	specifications, err := r.queries.ListProductSpecifications(ctx, product.ID)
	if err != nil {
		return dto.ProductDetail{}, err
	}
	for _, specification := range specifications {
		result.Specs[specification.Name] = specification.Value
	}

	err = r.eachRow(ctx, listAttributeValuesSQL, []int32{product.ID}, func(row pgx.Rows) error {
		var productID int32
		var key, dataType, value string
		if err := row.Scan(&productID, &key, &dataType, &value); err != nil {
			return err
		}
		result.Attributes[key] = dto.AttributeJSON(dataType, value)
		return nil
	})
	if err != nil {
		return dto.ProductDetail{}, err
	}

	history, err := r.queries.ListProductPriceHistory(ctx, product.ID)
	if err != nil {
		return dto.ProductDetail{}, err
	}
	for _, point := range history {
		pricePoint := dto.PricePoint{}
		if point.Price.Valid {
			if value, err := point.Price.MarshalJSON(); err == nil {
				pricePoint.Price = string(value)
			}
		}
		if point.RecordedAt.Valid {
			pricePoint.Date = point.RecordedAt.Time.Format("2006-01-02")
		}
		result.PriceHistory = append(result.PriceHistory, pricePoint)
		if point.IsPromotional {
			result.OfferPriceHistory = append(result.OfferPriceHistory, pricePoint)
		}
	}

	return result, nil
}

// ResolveCategory busca la categoría indicada en el request y sus definiciones
// de atributos. Devuelve ErrCategoryNotFound si no existe o está inactiva.
func (r *Repository) ResolveCategory(ctx context.Context, ref CategoryRef) (WriteCategory, error) {
	var category WriteCategory
	var err error
	if ref.Slug != "" {
		err = r.db.QueryRow(ctx, `
			SELECT c.id, c.slug, c.item_type, pc.id
			FROM categories c
			LEFT JOIN product_category pc ON pc.id = c.legacy_product_category_id AND pc.active
			WHERE c.slug = $1 AND c.active`, ref.Slug).
			Scan(&category.ID, &category.Slug, &category.ItemType, &category.LegacyID)
	} else {
		err = r.db.QueryRow(ctx, `
			SELECT c.id, c.slug, 'product', pc.id
			FROM product_category pc
			LEFT JOIN categories c ON c.legacy_product_category_id = pc.id AND c.active
			WHERE pc.id = $1 AND pc.active`, ref.LegacyID).
			Scan(&category.ID, &category.Slug, &category.ItemType, &category.LegacyID)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return WriteCategory{}, ErrCategoryNotFound
	}
	if err != nil {
		return WriteCategory{}, err
	}

	category.Attributes = []categories.Attribute{}
	if category.ID != nil {
		category.Attributes, err = r.categories.AttributesByCategoryID(ctx, *category.ID)
		if err != nil {
			return WriteCategory{}, err
		}
	}
	return category, nil
}

// Create guarda producto, imagen principal y atributos en una transacción.
func (r *Repository) Create(ctx context.Context, product ProductWrite) (pgtype.UUID, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return pgtype.UUID{}, err
	}
	defer tx.Rollback(ctx)

	var id int32
	var publicID pgtype.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO product (category_id, category_ref, brand_id, name, model, sku, description)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''))
		RETURNING id, public_id`,
		product.LegacyCategoryID, product.CategoryRef, product.BrandID, product.Name,
		product.Model, product.SKU, product.Description).Scan(&id, &publicID)
	if err != nil {
		return pgtype.UUID{}, err
	}

	if product.ImageURL != "" {
		if _, err := tx.Exec(ctx, `
			INSERT INTO product_image (product_id, image_url, alt_text, sort_order)
			VALUES ($1, $2, $3, 0)`, id, product.ImageURL, product.Name); err != nil {
			return pgtype.UUID{}, err
		}
	}

	if err := insertAttributeValues(ctx, tx, id, product); err != nil {
		return pgtype.UUID{}, err
	}

	return publicID, tx.Commit(ctx)
}

// Update reemplaza datos y atributos de un producto activo, o ErrNotFound.
func (r *Repository) Update(ctx context.Context, publicID pgtype.UUID, product ProductWrite) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Primero los valores: la FK compuesta impide cambiar category_ref
	// mientras existan valores de la categoría anterior.
	if _, err := tx.Exec(ctx, `
		DELETE FROM product_attribute_value pav
		USING product p
		WHERE pav.product_id = p.id AND p.public_id = $1 AND p.active`, publicID); err != nil {
		return err
	}

	var id int32
	err = tx.QueryRow(ctx, `
		UPDATE product
		SET category_id = $2, category_ref = $3, brand_id = $4, name = $5,
		    model = NULLIF($6, ''), sku = NULLIF($7, ''), description = NULLIF($8, ''),
		    updated_at = now()
		WHERE public_id = $1 AND active
		RETURNING id`,
		publicID, product.LegacyCategoryID, product.CategoryRef, product.BrandID, product.Name,
		product.Model, product.SKU, product.Description).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	if err := insertAttributeValues(ctx, tx, id, product); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Deactivate da de baja lógica un producto activo, o ErrNotFound.
func (r *Repository) Deactivate(ctx context.Context, publicID pgtype.UUID) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE product SET active = false, updated_at = now() WHERE public_id = $1 AND active`, publicID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListReviewsByPublicID devuelve las reseñas de un producto, o ErrNotFound.
func (r *Repository) ListReviewsByPublicID(ctx context.Context, publicID pgtype.UUID) ([]generated.ListProductReviewsRow, error) {
	product, err := r.queries.GetProductByPublicID(ctx, publicID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return r.queries.ListProductReviews(ctx, product.ID)
}

func insertAttributeValues(ctx context.Context, tx pgx.Tx, productID int32, product ProductWrite) error {
	if len(product.Attributes) == 0 {
		return nil
	}
	attributeIDs := make([]int32, 0, len(product.Attributes))
	values := make([]string, 0, len(product.Attributes))
	numerics := make([]string, 0, len(product.Attributes))
	for _, value := range product.Attributes {
		attributeIDs = append(attributeIDs, value.AttributeID)
		values = append(values, value.Text)
		if value.DataType == "integer" || value.DataType == "decimal" {
			numerics = append(numerics, value.Text)
		} else {
			numerics = append(numerics, "")
		}
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO product_attribute_value (product_id, category_id, attribute_id, value, value_numeric)
		SELECT $1, $2, v.attribute_id, v.value, NULLIF(v.numeric_text, '')::numeric
		FROM unnest($3::int[], $4::text[], $5::text[]) AS v(attribute_id, value, numeric_text)`,
		productID, product.CategoryRef, attributeIDs, values, numerics)
	return err
}

// constraintField traduce el nombre de una FK violada al campo del request.
func constraintField(constraint string) string {
	if strings.Contains(constraint, "brand") {
		return "brandId"
	}
	return ""
}
