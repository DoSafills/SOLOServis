package apierror

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func statusOf(t *testing.T, err error) int {
	t.Helper()

	var se huma.StatusError
	if !errors.As(err, &se) {
		t.Fatalf("error %v does not implement huma.StatusError", err)
	}

	return se.GetStatus()
}

func TestFromRepositoryMapsNoRowsToNotFound(t *testing.T) {
	// A genuinely absent row is a client answer, not a server fault.
	err := FromRepository(pgx.ErrNoRows, "product", "products.Get")

	if got := statusOf(t, err); got != http.StatusNotFound {
		t.Errorf("status = %d, want %d", got, http.StatusNotFound)
	}
}

func TestFromRepositoryMapsWrappedNoRowsToNotFound(t *testing.T) {
	wrapped := errors.Join(errors.New("loading detail"), pgx.ErrNoRows)

	if got := statusOf(t, FromRepository(wrapped, "product", "products.Get")); got != http.StatusNotFound {
		t.Errorf("status = %d, want %d", got, http.StatusNotFound)
	}
}

func TestFromRepositoryNeverLeaksDatabaseOutageAsNotFound(t *testing.T) {
	// Regression guard: any unrecognised failure used to become a 404, which
	// told clients that a healthy database simply had no rows.
	err := FromRepository(errors.New("connection refused"), "product", "products.Get")

	if got := statusOf(t, err); got != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", got, http.StatusInternalServerError)
	}

	var model *huma.ErrorModel
	if !errors.As(err, &model) {
		t.Fatal("expected a huma.ErrorModel")
	}

	if strings.Contains(model.Detail, "connection refused") {
		t.Errorf("response detail leaks the cause: %q", model.Detail)
	}
}

func TestFromRepositoryMapsPostgresCodes(t *testing.T) {
	tests := []struct {
		name       string
		pgErr      *pgconn.PgError
		wantStatus int
	}{
		{
			name:       "foreign key violation is unprocessable",
			pgErr:      &pgconn.PgError{Code: codeForeignKeyViolation, ConstraintName: "product_category_id_fkey"},
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "unique violation is a conflict",
			pgErr:      &pgconn.PgError{Code: codeUniqueViolation, ConstraintName: "product_sku_key"},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "exclusion violation is a conflict",
			pgErr:      &pgconn.PgError{Code: codeExclusionViolation, ConstraintName: "no_overlap"},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "not null violation is unprocessable",
			pgErr:      &pgconn.PgError{Code: codeNotNullViolation, ConstraintName: "product_name_not_null"},
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "check violation is unprocessable",
			pgErr:      &pgconn.PgError{Code: codeCheckViolation, ConstraintName: "product_offer_price_check"},
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "invalid text representation is unprocessable",
			pgErr:      &pgconn.PgError{Code: codeInvalidTextRepr},
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "numeric out of range is unprocessable",
			pgErr:      &pgconn.PgError{Code: codeNumericValueOutOfRng},
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "unknown SQLSTATE is a server fault",
			pgErr:      &pgconn.PgError{Code: "57014"},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := statusOf(t, FromRepository(tt.pgErr, "product", "products.Create")); got != tt.wantStatus {
				t.Errorf("status = %d, want %d", got, tt.wantStatus)
			}
		})
	}
}

func TestFromRepositoryNilIsNil(t *testing.T) {
	if err := FromRepository(nil, "product", "products.Get"); err != nil {
		t.Errorf("FromRepository(nil) = %v, want nil", err)
	}
}

func TestConstraintLabel(t *testing.T) {
	tests := []struct {
		constraint string
		want       string
	}{
		{"product_sku_key", "record"},
		{"product_pkey", "record"},
		{"product_category_id_fkey", "related record"},
		{"product_offer_price_check", "product_offer_price_check"},
		{"", "record"},
	}

	for _, tt := range tests {
		t.Run(tt.constraint, func(t *testing.T) {
			got := constraintLabel(&pgconn.PgError{ConstraintName: tt.constraint})
			if got != tt.want {
				t.Errorf("constraintLabel(%q) = %q, want %q", tt.constraint, got, tt.want)
			}
		})
	}
}

func TestHelpersReturnExpectedStatuses(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"not found", NotFound("product"), http.StatusNotFound},
		{"unauthorized", Unauthorized("token required"), http.StatusUnauthorized},
		{"forbidden", Forbidden("not allowed"), http.StatusForbidden},
		{"conflict", Conflict("duplicate sku"), http.StatusConflict},
		{"unprocessable", Unprocessable("bad field"), http.StatusUnprocessableEntity},
		{"bad request", BadRequest("malformed"), http.StatusBadRequest},
		{"internal", Internal(errors.New("boom"), "products.List"), http.StatusInternalServerError},
		{"internal with nil cause", Internal(nil, "products.List"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := statusOf(t, tt.err); got != tt.wantStatus {
				t.Errorf("status = %d, want %d", got, tt.wantStatus)
			}
		})
	}
}

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
