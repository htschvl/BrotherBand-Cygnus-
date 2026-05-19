// Package postgres holds the Postgres implementations of the domain
// repository ports.
//
// All repositories accept a DBTX interface rather than the pool
// directly so tests can inject a per-test transaction. The interface
// is the minimal subset of pgx that all our queries use; both
// *pgxpool.Pool and pgx.Tx satisfy it.
//
// (This mirrors what `sqlc --emit_interface` would emit; the
// `sqlc.yaml` here is the source of truth if/when the team chooses
// to switch to generated query code.)
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX is the narrow facade over pgx that repositories depend on.
// Both *pgxpool.Pool and pgx.Tx satisfy it without any wrapping.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
