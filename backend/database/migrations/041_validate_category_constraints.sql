-- =====================================================================
-- 041: valida las restricciones NOT VALID de product_category.
--
-- Por qué: 022/026 las agregaron NOT VALID para no bloquear la migración y
-- nunca se validaron. Los datos actuales las cumplen (probado con
-- VALIDATE CONSTRAINT dentro de BEGIN/ROLLBACK sobre una base recién
-- migrada), así que pasan a validar también las filas históricas.
--
-- Idempotente: volver a validar una restricción ya validada no cambia nada.
-- Reversa: backend/database/rollbacks/revert_041_validate_category_constraints.sql
-- BEGIN/COMMIT propio: el runner no envuelve cada archivo en una transacción.
-- =====================================================================
BEGIN;

ALTER TABLE product_category VALIDATE CONSTRAINT product_category_level_parent;
ALTER TABLE product_category VALIDATE CONSTRAINT product_category_level_range;
ALTER TABLE product_category VALIDATE CONSTRAINT product_category_parent_not_self;
ALTER TABLE product_category VALIDATE CONSTRAINT product_category_slug_format;

COMMIT;
