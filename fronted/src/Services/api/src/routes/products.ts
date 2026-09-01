import { Router } from 'express'
import { pool } from '../db.js'

const router = Router()

// Query base: arma el JSON final con el mismo shape que el tipo Product del frontend
const PRODUCT_QUERY = `
  SELECT
    p.id, p.name, p.brand, p.model, p.category, p.subcategory, p.image, p.description,
    p.rating, p.review_count AS "reviewCount", p.offer_price AS "offerPrice", p.tags,
    COALESCE(images.images, '[]') AS images,
    COALESCE(specs.specs, '{}') AS specs,
    COALESCE(offers.offers, '[]') AS offers,
    COALESCE(ph.history, '[]') AS "priceHistory",
    COALESCE(oph.history, '[]') AS "offerPriceHistory"
  FROM products p
  LEFT JOIN LATERAL (
    SELECT json_agg(url ORDER BY position) AS images
    FROM product_images WHERE product_id = p.id
  ) images ON true
  LEFT JOIN LATERAL (
    SELECT json_object_agg(spec_key, spec_value) AS specs
    FROM product_specs WHERE product_id = p.id
  ) specs ON true
  LEFT JOIN LATERAL (
    SELECT json_agg(json_build_object(
      'storeId', o.store_id,
      'storeName', s.name,
      'price', o.price,
      'available', o.available,
      'shipping', o.shipping
    )) AS offers
    FROM offers o JOIN stores s ON s.id = o.store_id
    WHERE o.product_id = p.id
  ) offers ON true
  LEFT JOIN LATERAL (
    SELECT json_agg(json_build_object('date', to_char(date,'YYYY-MM-DD'), 'price', price) ORDER BY date) AS history
    FROM price_history WHERE product_id = p.id
  ) ph ON true
  LEFT JOIN LATERAL (
    SELECT json_agg(json_build_object('date', to_char(date,'YYYY-MM-DD'), 'price', price) ORDER BY date) AS history
    FROM offer_price_history WHERE product_id = p.id
  ) oph ON true
`

// GET /api/products?category=Tecnología&search=rtx
router.get('/', async (req, res) => {
  try {
    const { category, search } = req.query
    const conditions: string[] = []
    const params: unknown[] = []

    if (typeof category === 'string' && category) {
      params.push(category)
      conditions.push(`p.category = $${params.length}`)
    }
    if (typeof search === 'string' && search) {
      params.push(`%${search}%`)
      conditions.push(`p.name ILIKE $${params.length}`)
    }

    let query = PRODUCT_QUERY
    if (conditions.length) query += ` WHERE ${conditions.join(' AND ')}`
    query += ' ORDER BY p.name'

    const { rows } = await pool.query(query, params)
    res.json(rows)
  } catch (err) {
    console.error(err)
    res.status(500).json({ error: 'Error al obtener productos' })
  }
})

// GET /api/products/:id
router.get('/:id', async (req, res) => {
  try {
    const { rows } = await pool.query(`${PRODUCT_QUERY} WHERE p.id = $1`, [req.params.id])
    if (!rows.length) return res.status(404).json({ error: 'Producto no encontrado' })
    res.json(rows[0])
  } catch (err) {
    console.error(err)
    res.status(500).json({ error: 'Error al obtener el producto' })
  }
})

export default router
