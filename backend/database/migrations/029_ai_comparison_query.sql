-- =====================================================================
-- Asistente de comparación (IA): auditoría de consultas.
--
-- Guarda qué se preguntó, qué respondió el proveedor, cuánto tardó y qué
-- tokens consumió. Sirve para medir la cuota gratuita y depurar; nunca
-- contiene datos personales más allá del user_id de la sesión (y solo si la
-- ruta se llama con sesión).
--
-- No reutiliza ai_query_process: esa tabla depende de search_query y
-- clasifica la intención de una búsqueda, no una comparación.
-- =====================================================================

CREATE TABLE IF NOT EXISTS ai_comparison_query (
    id            BIGSERIAL PRIMARY KEY,
    user_id       INTEGER REFERENCES user_account(id) ON DELETE SET NULL,
    product_ids   UUID[] NOT NULL,
    question      VARCHAR(300) NOT NULL,
    answer        TEXT,
    provider      VARCHAR(50),
    model         VARCHAR(100),
    prompt_tokens INTEGER,
    output_tokens INTEGER,
    latency_ms    INTEGER,
    cached        BOOLEAN NOT NULL DEFAULT FALSE,
    success       BOOLEAN NOT NULL DEFAULT TRUE,
    error_message TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS ai_comparison_query_created_at_idx
    ON ai_comparison_query (created_at);

COMMENT ON TABLE ai_comparison_query IS
    'Auditoría del asistente de comparación: pregunta, respuesta, proveedor, tokens y latencia.';
