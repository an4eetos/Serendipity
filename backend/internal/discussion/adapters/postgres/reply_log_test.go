package postgres

import (
	"context"
	"testing"
	"time"

	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres/pgtest"
)

func TestReplyLog(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		log := NewReplyLog(db)
		uid, other := pgtest.NewUser(t, ctx, db), pgtest.NewUser(t, ctx, db)
		count := func(user string) int {
			t.Helper()
			n, err := log.CountSince(ctx, user, time.Hour)
			if err != nil {
				t.Fatal(err)
			}
			return n
		}
		rows := func(user string) int {
			t.Helper()
			var n int
			if err := db.QueryRow(ctx, `select count(*) from character_replies where user_id = $1`, user).Scan(&n); err != nil {
				t.Fatal(err)
			}
			return n
		}
		// now() is fixed within the test's transaction: rows added now are
		// in the window, and older ones are backdated by hand.
		backdate := func(user, age string) {
			t.Helper()
			if _, err := db.Exec(ctx, `insert into character_replies (user_id, created_at)
				values ($1, now() - $2::interval)`, user, age); err != nil {
				t.Fatal(err)
			}
		}

		if err := log.Lock(ctx, uid); err != nil {
			t.Fatalf("Lock: %v", err)
		}
		if n := count(uid); n != 0 {
			t.Fatalf("empty log counts %d", n)
		}

		backdate(uid, "59 minutes")
		backdate(uid, "61 minutes")
		backdate(uid, "3 hours")
		backdate(other, "3 hours")
		if n := count(uid); n != 1 {
			t.Errorf("counted %d, want only the reply within the hour", n)
		}

		if err := log.Add(ctx, uid, time.Hour); err != nil {
			t.Fatal(err)
		}
		if n := count(uid); n != 2 {
			t.Errorf("after Add counted %d, want 2", n)
		}
		if n := rows(uid); n != 2 {
			t.Errorf("after Add the reader has %d rows, want 2: older ones are pruned", n)
		}
		if n := rows(other); n != 1 {
			t.Errorf("Add pruned another reader's rows: %d left, want 1", n)
		}
		if n := count(other); n != 0 {
			t.Errorf("another reader's old reply counted: %d", n)
		}
	})
}

// TestReplyLogLockSerialisesAReader shows that one reader's second request
// waits for the first to commit before it can count, so both can't pass the
// limit.
func TestReplyLogLockSerialisesAReader(t *testing.T) {
	c := newCommitted(t)
	ctx := testContext(t)
	log := NewReplyLog(c.db)

	holder, commit := c.hold(ctx, `with locked as (select pg_advisory_xact_lock(hashtextextended('character_replies:' || $1, 0)))
		insert into character_replies (user_id) select $1::uuid from locked`, c.user)
	counted := make(chan int, 1)
	done := c.whileBlocked(ctx, holder, func() error {
		return c.txm.WithinTx(ctx, func(ctx context.Context) error {
			if err := log.Lock(ctx, c.user); err != nil {
				return err
			}
			n, err := log.CountSince(ctx, c.user, time.Hour)
			counted <- n
			return err
		})
	})
	if err := commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if n := <-counted; n != 1 {
		t.Errorf("the waiting request counted %d, want 1: the first request's reply", n)
	}
}
