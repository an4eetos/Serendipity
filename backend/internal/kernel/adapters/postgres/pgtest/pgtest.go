// Package pgtest runs Postgres adapter integration tests against the local
// Supabase database.
//
// Tests connect to TEST_DATABASE_URL (with `npx supabase start` running:
// postgresql://postgres:postgres@127.0.0.1:54322/postgres) and run inside a
// transaction that is always rolled back. They are skipped when
// TEST_DATABASE_URL is unset.
package pgtest

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
)

var errRollback = errors.New("pgtest: roll back")

// URL returns TEST_DATABASE_URL, skipping the test when it is unset.
func URL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	return url
}

// Run calls fn with a context carrying a transaction that is rolled back
// once fn returns.
func Run(t *testing.T, fn func(ctx context.Context, db *kernelpg.DB)) {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	db := kernelpg.NewDB(pool)
	err = kernelpg.NewTxManager(db).WithinTx(t.Context(), func(ctx context.Context) error {
		fn(ctx, db)
		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatal(err)
	}
}

// NewUser creates an auth user, and through the signup trigger its profile,
// and returns the user id.
func NewUser(t *testing.T, ctx context.Context, db *kernelpg.DB) string {
	t.Helper()
	var id string
	err := db.QueryRow(ctx, `
		insert into auth.users (id, email)
		values (gen_random_uuid(), gen_random_uuid()::text || '@example.com')
		returning id`).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
