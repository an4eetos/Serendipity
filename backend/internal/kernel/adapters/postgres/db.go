// Package postgres is the Postgres plumbing every bounded context's
// repository builds on: connection resolution, transactions and error
// translation. Bounded contexts import it as "kernelpg".
//
// The API connects with a privileged role that bypasses row-level security,
// so every query that touches user data must filter by the caller's user id.
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type txKey struct{}

type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// DB runs queries in the context's transaction, or on the pool when there is
// none. Errors meaning "no such thing" come back as kernel.ErrNotFound.
type DB struct {
	pool *pgxpool.Pool
}

func NewDB(pool *pgxpool.Pool) *DB {
	return &DB{pool: pool}
}

func (db *DB) conn(ctx context.Context) querier {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return db.pool
}

func (db *DB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tag, err := db.conn(ctx).Exec(ctx, sql, args...)
	return tag, translate(err)
}

func (db *DB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	rows, err := db.conn(ctx).Query(ctx, sql, args...)
	if err != nil {
		return nil, translate(err)
	}
	return translatedRows{rows}, nil
}

func (db *DB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return translatedRow{db.conn(ctx).QueryRow(ctx, sql, args...)}
}

type translatedRow struct{ pgx.Row }

func (r translatedRow) Scan(dest ...any) error { return translate(r.Row.Scan(dest...)) }

type translatedRows struct{ pgx.Rows }

func (r translatedRows) Scan(dest ...any) error { return translate(r.Rows.Scan(dest...)) }

func (r translatedRows) Err() error { return translate(r.Rows.Err()) }
