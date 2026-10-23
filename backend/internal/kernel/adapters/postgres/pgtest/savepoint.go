package pgtest

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
)

// Attempt runs fn inside a savepoint of the test's transaction and returns
// its error. When fn fails, the transaction is rolled back to the savepoint,
// so a test can check an expected error and keep using the transaction.
func Attempt(t *testing.T, ctx context.Context, db *kernelpg.DB, fn func() error) error {
	t.Helper()
	if _, err := db.Exec(ctx, `savepoint pgtest_attempt`); err != nil {
		t.Fatal(err)
	}
	err := fn()
	end := `release savepoint pgtest_attempt`
	if err != nil {
		end = `rollback to savepoint pgtest_attempt`
	}
	if _, e := db.Exec(ctx, end); e != nil {
		t.Fatal(e)
	}
	return err
}

// AsUser runs fn as a signed-in Supabase client would reach the database
// directly: as the "authenticated" role with userID's JWT claims, so that
// row-level security applies. It runs inside the test's transaction, and the
// role and claims are reset when fn returns. Wrap statements expected to
// fail in Attempt.
func AsUser(t *testing.T, ctx context.Context, db *kernelpg.DB, userID string, fn func()) {
	t.Helper()
	var claims string
	if err := db.QueryRow(ctx, `
		select set_config('request.jwt.claims',
			json_build_object('sub', $1::text, 'role', 'authenticated')::text, true)`, userID).Scan(&claims); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `set local role authenticated`); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := db.Exec(ctx, `reset role`); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(ctx, `select set_config('request.jwt.claims', '', true)`).Scan(&claims); err != nil {
			t.Fatal(err)
		}
	}()
	fn()
}

// IsInsufficientPrivilege reports whether err is Postgres refusing the
// statement for lack of privilege: a missing grant, or a row refused by
// row-level security (SQLSTATE 42501).
func IsInsufficientPrivilege(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42501"
}
