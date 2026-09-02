import { Router } from "express";
import { pool } from "../db.js";

const router = Router();

const STORE_COLUMNS = `
  id, name, logo, rating, review_count AS "reviewCount",
  product_count AS "productCount", reputation,
  dispatch_time AS "dispatchTime", conditions, website
`;

router.get("/", async (_req, res) => {
  try {
    const { rows } = await pool.query(`SELECT ${STORE_COLUMNS} FROM stores ORDER BY name`);
    res.json(rows);
  } catch (err) {
    console.error(err);
    res.status(500).json({ error: "Error al obtener tiendas" });
  }
});

router.get("/:id", async (req, res) => {
  try {
    const { rows } = await pool.query(`SELECT ${STORE_COLUMNS} FROM stores WHERE id = $1`, [
      req.params.id,
    ]);
    if (!rows.length) return res.status(404).json({ error: "Tienda no encontrada" });
    res.json(rows[0]);
  } catch (err) {
    console.error(err);
    res.status(500).json({ error: "Error al obtener la tienda" });
  }
});

export default router;
