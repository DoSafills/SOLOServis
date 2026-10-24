package features

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) QueryJSON(ctx context.Context, query string, args ...any) ([]json.RawMessage, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]json.RawMessage, 0)
	for rows.Next() {
		var value []byte
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		result = append(result, json.RawMessage(value))
	}
	return result, rows.Err()
}

func (r *Repository) Exec(ctx context.Context, query string, args ...any) error {
	_, err := r.db.Exec(ctx, query, args...)
	return err
}

func (r *Repository) ResolveProductID(ctx context.Context, value string) (int32, error) {
	var id int32
	if number, err := strconv.ParseInt(value, 10, 32); err == nil {
		err = r.db.QueryRow(ctx, `SELECT id FROM product WHERE id = $1 AND active = TRUE`, number).Scan(&id)
		return id, err
	}
	if len(value) != 36 || strings.Count(value, "-") != 4 {
		return 0, pgx.ErrNoRows
	}
	err := r.db.QueryRow(ctx, `SELECT id FROM product WHERE public_id = $1::uuid AND active = TRUE`, value).Scan(&id)
	return id, err
}

func (r *Repository) ResolveServiceID(ctx context.Context, value string) (int32, error) {
	var id int32
	if number, err := strconv.ParseInt(value, 10, 32); err == nil {
		err = r.db.QueryRow(ctx, `SELECT id FROM service WHERE id=$1 AND active=TRUE`, number).Scan(&id)
		return id, err
	}
	if len(value) != 36 || strings.Count(value, "-") != 4 {
		return 0, pgx.ErrNoRows
	}
	err := r.db.QueryRow(ctx, `SELECT id FROM service WHERE public_id=$1::uuid AND active=TRUE`, value).Scan(&id)
	return id, err
}

func (r *Repository) ResolveUserID(ctx context.Context, value int32) (int32, error) {
	var id int32
	err := r.db.QueryRow(ctx, `SELECT id FROM user_account WHERE id = $1 AND active = TRUE`, value).Scan(&id)
	return id, err
}
