package postgres

import (
	"context"
	"testing"

	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres/pgtest"
	"github.com/an4eetos/serendipity/backend/internal/quote/domain"
)

// A signed-in client that reaches the database directly, around the API,
// can read its own quotes and nothing else, and write none.
func TestQuotesRowLevelSecurity(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		ada, bo := pgtest.NewUser(t, ctx, db), pgtest.NewUser(t, ctx, db)
		if _, err := db.Exec(ctx, `update profiles set is_public = true where id = $1`, bo); err != nil {
			t.Fatal(err)
		}
		f := newFixture(t, ctx, db, ada)
		if err := f.coverage.SetCap(ctx, 3); err != nil {
			t.Fatal(err)
		}
		f.publish(ada, f.draft(ada, "One two three").ID)
		collapsed := f.publish(ada, f.draft(ada, "Four five six").ID)
		removed := f.publish(ada, f.draft(ada, "Seven").ID)
		if err := f.quotes.Remove(ctx, removed.ID, "Copyright"); err != nil {
			t.Fatal(err)
		}
		private := f.draft(ada, "Private")
		theirs := f.publish(bo, f.draft(bo, "Someone else's public quote").ID)
		if !collapsed.PassageHidden {
			t.Fatal("setup: want the second passage collapsed")
		}

		pgtest.AsUser(t, ctx, db, ada, func() {
			var own, others int
			if err := db.QueryRow(ctx, `select count(*) filter (where user_id = $1), count(*) filter (where user_id <> $1) from quotes`, ada).
				Scan(&own, &others); err != nil {
				t.Fatal(err)
			}
			if own != 4 || others != 0 {
				t.Errorf("reads %d own and %d other quotes; want 4 and 0, even another reader's public one", own, others)
			}
			var text string
			if err := pgtest.Attempt(t, ctx, db, func() error {
				return db.QueryRow(ctx, `select text from quotes where id = $1`, theirs.ID).Scan(&text)
			}); err == nil {
				t.Errorf("read another reader's public quote directly: %q", text)
			}

			for _, w := range []struct{ name, sql, id string }{
				{"show a collapsed passage", `update quotes set passage_hidden = false where id = $1`, collapsed.ID},
				{"clear a takedown", `update quotes set removed_at = null, removed_reason = '' where id = $1`, removed.ID},
				{"publish past the API", `update quotes set visibility = 'public' where id = $1`, private.ID},
				{"edit another reader's quote", `update quotes set reaction = 'forged' where id = $1`, theirs.ID},
				{"delete an own quote", `delete from quotes where id = $1`, private.ID},
				{"delete another reader's quote", `delete from quotes where id = $1`, theirs.ID},
			} {
				tag, err := db.Exec(ctx, w.sql, w.id)
				if err != nil || tag.RowsAffected() != 0 {
					t.Errorf("%s: %d rows, %v; want none", w.name, tag.RowsAffected(), err)
				}
			}
			err := pgtest.Attempt(t, ctx, db, func() error {
				_, err := db.Exec(ctx, `
					insert into quotes (user_id, work_id, text, location, reaction, source, visibility)
					values ($1, $2, 'Inserted directly', 'p. 1', 'Mine.', 'upload', 'public')`, ada, f.workID)
				return err
			})
			if !pgtest.IsInsufficientPrivilege(err) {
				t.Errorf("insert: err = %v, want a row-level security violation", err)
			}
		})

		for id, want := range map[string]func(domain.Quote) bool{
			collapsed.ID: func(q domain.Quote) bool { return q.PassageHidden },
			removed.ID:   func(q domain.Quote) bool { return q.Removed() },
			private.ID:   func(q domain.Quote) bool { return q.Visibility == domain.VisibilityPrivate },
		} {
			if q, err := f.quotes.Get(ctx, ada, id); err != nil || !want(q) {
				t.Errorf("quote %s after direct writes = %#v, %v; want it unchanged", id, q, err)
			}
		}
	})
}
