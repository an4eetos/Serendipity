package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
)

// Postgres error codes, see https://www.postgresql.org/docs/current/errcodes-appendix.html.
const (
	codeInvalidTextRepresentation = "22P02"
	codeUniqueViolation           = "23505"
	codeCheckViolation            = "23514"
)

func translate(err error) error {
	switch {
	case errors.Is(err, pgx.ErrNoRows),
		// A malformed id can't match anything. The server has still failed
		// the statement, so a transaction it ran in is aborted: look up
		// ids supplied by readers before starting a transaction.
		hasCode(err, codeInvalidTextRepresentation):
		return kernel.ErrNotFound
	}
	return err
}

// IsUniqueViolation reports whether err is a unique constraint violation.
func IsUniqueViolation(err error) bool { return hasCode(err, codeUniqueViolation) }

// IsCheckViolation reports whether err is a check constraint violation.
func IsCheckViolation(err error) bool { return hasCode(err, codeCheckViolation) }

func hasCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}
