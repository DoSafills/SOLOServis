-- =====================================================================
-- Verificación de usuario (opción 4)
-- La verificación pertenece a la cuenta, no a cada reseña: una reseña se
-- muestra como "Usuario verificado" si su autor confirmó su email.
-- El flujo que pone email_verified = TRUE llega con el login (pendiente).
-- =====================================================================

ALTER TABLE user_account
    ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT FALSE;

-- Datos de prueba (solo afecta a los usuarios del seed)
UPDATE user_account
SET email_verified = TRUE
WHERE email IN ('ricardo.rios@soloservis.cl', 'camila.fuentes@example.com');
