package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// pgErrorCode is the SQLSTATE code attached to a pgconn.PgError.
const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
	pgCheckViolation      = "23514"
)

// isNoRows reports whether the error is the canonical pgx "no rows"
// signal. Centralising it here keeps `errors.Is(err, pgx.ErrNoRows)`
// out of every repository.
func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// pgErrorMatches reports whether err is a pg error with one of the
// given SQLSTATE codes. Used to translate constraint violations into
// domain conflicts.
func pgErrorMatches(err error, codes ...string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	for _, c := range codes {
		if pgErr.Code == c {
			return true
		}
	}
	return false
}
