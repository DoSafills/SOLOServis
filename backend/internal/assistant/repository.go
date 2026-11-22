package assistant

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRecorder guarda cada consulta en ai_comparison_query. Es solo
// auditoría: un fallo aquí no cambia la respuesta al usuario.
type PostgresRecorder struct {
	db *pgxpool.Pool
}

func NewPostgresRecorder(db *pgxpool.Pool) *PostgresRecorder {
	return &PostgresRecorder{db: db}
}

func (r *PostgresRecorder) Record(ctx context.Context, audit Audit) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO ai_comparison_query
			(user_id, product_ids, question, answer, provider, model,
			 prompt_tokens, output_tokens, latency_ms, cached, success, error_message)
		VALUES ($1, $2::uuid[], $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		audit.UserID,
		audit.ProductIDs,
		audit.Question,
		nullString(audit.Answer),
		nullString(audit.Provider),
		nullString(audit.Model),
		nullInt(audit.PromptTokens),
		nullInt(audit.OutputTokens),
		nullInt(audit.LatencyMS),
		audit.Cached,
		audit.Success,
		nullString(audit.ErrorMessage),
	)
	return err
}

func nullString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func nullInt(value int) *int {
	if value == 0 {
		return nil
	}
	return &value
}
