import { Router } from 'express'
import { pool } from '../db.js'

const router = Router()

const SERVICE_QUERY = `
  SELECT
    s.id, s.name, s.provider, s.category, s.subcategory, s.description,
    s.monthly_price AS "monthlyPrice", s.installation_cost AS "installationCost",
    s.contract_months AS "contractMonths", s.rating, s.review_count AS "reviewCount",
    s.coverage, s.image,
    COALESCE(specs.specs, '{}') AS specs,
    COALESCE(benefits.benefits, '[]') AS benefits,
    COALESCE(ph.history, '[]') AS "priceHistory"
  FROM services s
  LEFT JOIN LATERAL (
    SELECT json_object_agg(spec_key, spec_value) AS specs
    FROM service_specs WHERE service_id = s.id
  ) specs ON true
  LEFT JOIN LATERAL (
    SELECT json_agg(benefit) AS benefits
    FROM service_benefits WHERE service_id = s.id
  ) benefits ON true
  LEFT JOIN LATERAL (
    SELECT json_agg(json_build_object('date', to_char(date,'YYYY-MM-DD'), 'price', price) ORDER BY date) AS history
    FROM service_price_history WHERE service_id = s.id
  ) ph ON true
`

// GET /api/services?category=Internet
router.get('/', async (req, res) => {
  try {
    const { category } = req.query
    const params: unknown[] = []
    let query = SERVICE_QUERY

    if (typeof category === 'string' && category) {
      params.push(category)
      query += ` WHERE s.category = $${params.length}`
    }
    query += ' ORDER BY s.name'

    const { rows } = await pool.query(query, params)
    res.json(rows)
  } catch (err) {
    console.error(err)
    res.status(500).json({ error: 'Error al obtener servicios' })
  }
})

router.get('/:id', async (req, res) => {
  try {
    const { rows } = await pool.query(`${SERVICE_QUERY} WHERE s.id = $1`, [req.params.id])
    if (!rows.length) return res.status(404).json({ error: 'Servicio no encontrado' })
    res.json(rows[0])
  } catch (err) {
    console.error(err)
    res.status(500).json({ error: 'Error al obtener el servicio' })
  }
})

export default router
