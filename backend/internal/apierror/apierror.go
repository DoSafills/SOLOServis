// Package apierror translates domain and persistence failures into HTTP
// responses described with RFC 9457 problem details.
//
// The mapping exists so that a database outage is never reported to a client as
// "not found". Previously every repository error collapsed into a 404, which
// made an unavailable database indistinguishable from a missing row.
package apierror

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// PostgreSQL SQLSTATE codes that map to a client-caused failure.
const (
	codeNotNullViolation     = "23502"
	codeForeignKeyViolation  = "23503"
	codeUniqueViolation      = "23505"
	codeCheckViolation       = "23514"
	codeExclusionViolation   = "23P01"
	codeInvalidTextRepr      = "22P02"
	codeNumericValueOutOfRng = "22003"
)

// NotFound reports that the requested resource does not exist.
func NotFound(resource string) error {
	return huma.Error404NotFound(fmt.Sprintf("%s not found", resource))
}

// Unauthorized reports missing or invalid credentials.
func Unauthorized(msg string) error {
	return huma.Error401Unauthorized(msg)
}

// Forbidden reports that the caller is known but not allowed to act.
func Forbidden(msg string) error {
	return huma.Error403Forbidden(msg)
}

// Conflict reports that the request collides with the current state of the
// resource, for example a duplicate SKU.
func Conflict(msg string, errs ...error) error {
	return huma.Error409Conflict(msg, errs...)
}

// Unprocessable reports that the payload was syntactically valid but
// semantically rejected.
func Unprocessable(msg string, errs ...error) error {
	return huma.Error422UnprocessableEntity(msg, errs...)
}

// BadRequest reports a malformed request that Huma's own schema validation
// could not describe more precisely.
func BadRequest(msg string, errs ...error) error {
	return huma.Error400BadRequest(msg, errs...)
}

// Internal logs the underlying cause and returns a 500 that deliberately
// omits database and driver details from the response body.
func Internal(cause error, operation string) error {
	if cause != nil {
		slog.Error("request failed", "operation", operation, "error", cause)
	}

	return huma.Error500InternalServerError("an unexpected error occurred while processing the request")
}

// FromRepository converts an error returned by a repository into the most
// accurate HTTP response available.
//
//   - pgx.ErrNoRows becomes 404, because "the row is absent" is a client-facing
//     answer rather than a server fault.
//   - A foreign key violation becomes 422, because the client referenced an
//     identifier that does not exist.
//   - A unique or exclusion violation becomes 409.
//   - Any other failure is logged and becomes 500.
func FromRepository(err error, resource, operation string) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return NotFound(resource)
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case codeForeignKeyViolation:
			return Unprocessable(
				fmt.Sprintf("the request references a %s that does not exist", constraintLabel(pgErr)),
				&huma.ErrorDetail{Message: "referenced identifier was not found"},
			)
		case codeUniqueViolation, codeExclusionViolation:
			return Conflict(
				fmt.Sprintf("the request conflicts with an existing %s", constraintLabel(pgErr)),
				&huma.ErrorDetail{Message: pgErr.ConstraintName},
			)
		case codeNotNullViolation, codeCheckViolation,
			codeInvalidTextRepr, codeNumericValueOutOfRng:
			return Unprocessable(
				"the request violates a database constraint",
				&huma.ErrorDetail{Message: pgErr.ConstraintName},
			)
		}
	}

	return Internal(err, operation)
}

// constraintLabel produces a human-friendly noun for a constraint name such as
// "product_sku_key" or "product_category_id_fkey".
func constraintLabel(pgErr *pgconn.PgError) string {
	name := pgErr.ConstraintName
	if name == "" {
		name = "record"
	}

	switch {
	case hasSuffix(name, "_key"), hasSuffix(name, "_pkey"):
		return "record"
	case hasSuffix(name, "_fkey"):
		return "related record"
	default:
		return name
	}
}

func hasSuffix(value, suffix string) bool {
	return len(value) >= len(suffix) && value[len(value)-len(suffix):] == suffix
}
