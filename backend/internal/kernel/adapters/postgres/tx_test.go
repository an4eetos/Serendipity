package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres/pgtest"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
)

func TestTxManager(t *testing.T) {
	cfg, err := pgxpool.ParseConfig(pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	// One connection, so that the temporary table is visible to every
	// query and disappears with the pool.
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	ctx := t.Context()
	if _, err := pool.Exec(ctx, `create temporary table tx_probe (n int)`); err != nil {
		t.Fatal(err)
	}

	db := kernelpg.NewDB(pool)
	txm := kernelpg.NewTxManager(db)
	insert := func(n int) func(context.Context) error {
		return func(ctx context.Context) error {
			_, err := db.Exec(ctx, `insert into tx_probe values ($1)`, n)
			return err
		}
	}
	count := func() int {
		var n int
		if err := db.QueryRow(ctx, `select count(*) from tx_probe`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	errFail := errors.New("fail")

	if err := txm.WithinTx(ctx, insert(1)); err != nil {
		t.Fatal(err)
	}
	if n := count(); n != 1 {
		t.Fatalf("after commit: %d rows, want 1", n)
	}

	err = txm.WithinTx(ctx, func(ctx context.Context) error {
		if err := insert(2)(ctx); err != nil {
			return err
		}
		return errFail
	})
	if !errors.Is(err, errFail) || count() != 1 {
		t.Fatalf("after a failed unit of work: err = %v, %d rows; want it rolled back", err, count())
	}

	err = txm.WithinTx(ctx, func(ctx context.Context) error {
		if err := insert(3)(ctx); err != nil {
			return err
		}
		// A nested unit of work joins the outer transaction, so its rows
		// roll back with it.
		if err := txm.WithinTx(ctx, insert(4)); err != nil {
			return err
		}
		return errFail
	})
	if !errors.Is(err, errFail) || count() != 1 {
		t.Fatalf("after a failed nested unit of work: err = %v, %d rows; want both rolled back", err, count())
	}

	var n int
	if err := db.QueryRow(ctx, `select n from tx_probe where n = 99`).Scan(&n); !errors.Is(err, kernel.ErrNotFound) {
		t.Errorf("no rows: err = %v, want kernel.ErrNotFound", err)
	}
}
