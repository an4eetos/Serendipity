package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	catalogpg "github.com/an4eetos/serendipity/backend/internal/catalog/adapters/postgres"
	catalog "github.com/an4eetos/serendipity/backend/internal/catalog/domain"
	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres/pgtest"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	"github.com/an4eetos/serendipity/backend/internal/quote/domain"
)

// fixture is a work with quotes in the test's transaction.
type fixture struct {
	t        *testing.T
	ctx      context.Context
	db       *kernelpg.DB
	quotes   *QuoteRepository
	coverage *CoverageRepository
	workID   string
}

func newFixture(t *testing.T, ctx context.Context, db *kernelpg.DB, creator string) fixture {
	t.Helper()
	w, err := catalogpg.NewWorkRepository(db).Ensure(ctx, catalog.Work{Title: "Coverage " + creator[:8]}, creator)
	if err != nil {
		t.Fatal(err)
	}
	return fixture{t: t, ctx: ctx, db: db, quotes: NewQuoteRepository(db), coverage: NewCoverageRepository(db), workID: w.ID}
}

// draft stores a private, cited quote with a reaction, ready to publish.
func (f fixture) draft(userID, text string) domain.Quote {
	f.t.Helper()
	q, err := f.quotes.Create(f.ctx, userID, domain.Draft{
		WorkID: f.workID, Text: text, Location: "p. 1", Reaction: "Mine.", Source: domain.SourceUpload,
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return q
}

// publish makes q public the way a direct write would: the trigger alone
// decides whether the passage is shown.
func (f fixture) publish(userID, id string) domain.Quote {
	f.t.Helper()
	q, err := f.quotes.Update(f.ctx, userID, id, domain.Patch{Visibility: ptr(domain.VisibilityPublic)})
	if err != nil {
		f.t.Fatal(err)
	}
	return q
}

func (f fixture) measure(quoteID string) domain.Coverage {
	f.t.Helper()
	c, err := f.coverage.Coverage(f.ctx, f.workID, quoteID)
	if err != nil {
		f.t.Fatal(err)
	}
	return c
}

func (f fixture) setWorkCap(wordCap *int) {
	f.t.Helper()
	if _, err := f.db.Exec(f.ctx, `update works set public_word_cap = $2 where id = $1`, f.workID, wordCap); err != nil {
		f.t.Fatal(err)
	}
}

func TestCoverageTrigger(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		ada, bo := pgtest.NewUser(t, ctx, db), pgtest.NewUser(t, ctx, db)
		f := newFixture(t, ctx, db, ada)
		if err := f.coverage.SetCap(ctx, 5); err != nil {
			t.Fatal(err)
		}
		var setting string
		if err := db.QueryRow(ctx, `select current_setting('serendipity.public_word_cap')`).Scan(&setting); err != nil || setting != "5" {
			t.Fatalf("setting after SetCap(5) = %q, %v", setting, err)
		}
		if err := f.coverage.LockWork(ctx, f.workID); err != nil {
			t.Errorf("LockWork() = %v", err)
		}

		first := f.publish(ada, f.draft(ada, "One two three").ID)
		if first.PassageHidden || first.PassageWords != 3 {
			t.Errorf("first passage: %#v; want 3 words, shown under the cap of 5", first)
		}
		second := f.draft(ada, "Four five six")
		if c := f.measure(second.ID); c.Words != 3 || c.IdenticalShown || c.CapOverride != nil {
			t.Errorf("Coverage() = %+v, want 3 words, nothing identical, no override", c)
		}
		if got := f.publish(ada, second.ID); !got.PassageHidden || got.Visibility != domain.VisibilityPublic {
			t.Errorf("second passage: %#v; want it public with the passage collapsed (3 + 3 > 5)", got)
		}
		if c := f.measure(second.ID); c.Words != 3 {
			t.Errorf("Coverage() = %+v; a collapsed passage must not count", c)
		}

		// Another reader's identical passage, normalized: case and whitespace differ.
		same := f.draft(bo, "  ONE   two\nthree ")
		if c := f.measure(same.ID); !c.IdenticalShown || c.Words != 3 {
			t.Errorf("Coverage() of an identical passage = %+v, want IdenticalShown", c)
		}
		if got := f.publish(bo, same.ID); got.PassageHidden {
			t.Errorf("identical passage: %#v; want it shown, counted once", got)
		}
		if c := f.measure(second.ID); c.Words != 3 {
			t.Errorf("Coverage() = %+v; an identical passage must count once", c)
		}

		// The work's own cap beats the transaction's.
		f.setWorkCap(ptr(100))
		third := f.draft(ada, "Seven eight nine")
		if c := f.measure(third.ID); c.CapOverride == nil || *c.CapOverride != 100 {
			t.Errorf("Coverage().CapOverride = %v, want 100", c.CapOverride)
		}
		if got := f.publish(ada, third.ID); got.PassageHidden {
			t.Errorf("under the work's cap of 100: %#v; want it shown", got)
		}
		f.setWorkCap(ptr(0))
		if got := f.publish(ada, f.draft(ada, "Ten").ID); !got.PassageHidden {
			t.Errorf("under the work's cap of 0: %#v; want it collapsed", got)
		}
		f.setWorkCap(nil)

		// Republishing re-checks the cap; editing a public quote doesn't.
		if _, err := f.quotes.Update(ctx, ada, first.ID, domain.Patch{Reaction: ptr("Still mine.")}); err != nil {
			t.Fatal(err)
		}
		if got, _ := f.quotes.Get(ctx, ada, first.ID); got.PassageHidden {
			t.Errorf("editing the reaction of a shown quote collapsed it: %#v", got)
		}
		if _, err := f.quotes.Update(ctx, ada, first.ID, domain.Patch{Visibility: ptr(domain.VisibilityPrivate)}); err != nil {
			t.Fatal(err)
		}
		if got := f.publish(ada, first.ID); got.PassageHidden {
			// first's passage is still shown by bo's identical quote.
			t.Errorf("republishing a passage shown by an identical quote: %#v; want it shown", got)
		}

		// The API's decision doesn't override the trigger: past the cap, a
		// patch that asks to show the passage is still collapsed.
		over := f.draft(ada, "Eleven twelve thirteen fourteen")
		got, err := f.quotes.Update(ctx, ada, over.ID, domain.Patch{Visibility: ptr(domain.VisibilityPublic), PassageHidden: ptr(false)})
		if err != nil || !got.PassageHidden {
			t.Errorf("Update() asking to show past the cap = %#v, %v; want the trigger to collapse it", got, err)
		}

		if err := f.coverage.LockWork(ctx, "00000000-0000-4000-8000-000000000000"); !errors.Is(err, kernel.ErrNotFound) {
			t.Errorf("LockWork(missing work) = %v, want kernel.ErrNotFound", err)
		}
	})
}

func TestCoverageDefaultCap(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		// No SetCap in this transaction: the trigger falls back to 2500.
		ada := pgtest.NewUser(t, ctx, db)
		f := newFixture(t, ctx, db, ada)
		passage := func(n int) string {
			words := make([]string, domain.MaxWords)
			for i := range words {
				words[i] = fmt.Sprintf("p%dw%d", n, i)
			}
			return strings.Join(words, " ")
		}
		for n := range 8 {
			if got := f.publish(ada, f.draft(ada, passage(n)).ID); got.PassageHidden {
				t.Fatalf("passage %d (%d words so far): collapsed under the default cap", n, (n+1)*domain.MaxWords)
			}
		}
		if got := f.publish(ada, f.draft(ada, passage(8)).ID); !got.PassageHidden {
			t.Errorf("2,700 words: %#v; want it collapsed past the default 2,500", got.PassageWords)
		}
	})
}

func TestRemovedQuotes(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		ada, bo := pgtest.NewUser(t, ctx, db), pgtest.NewUser(t, ctx, db)
		f := newFixture(t, ctx, db, ada)
		public := f.publish(ada, f.draft(ada, "To be or not to be").ID)
		private := f.draft(ada, "That is the question")

		if err := f.quotes.Remove(ctx, public.ID, "Publisher's notice"); err != nil {
			t.Fatal(err)
		}
		if err := f.quotes.Remove(ctx, private.ID, "Spam"); err != nil {
			t.Fatal(err)
		}
		if err := f.quotes.Remove(ctx, public.ID, "again"); !errors.Is(err, kernel.ErrNotFound) {
			t.Errorf("Remove(removed) = %v, want kernel.ErrNotFound", err)
		}

		// The owner still sees it, with the reason; public reads don't.
		if got, err := f.quotes.Get(ctx, ada, public.ID); err != nil || !got.Removed() || got.RemovedReason != "Publisher's notice" ||
			got.UserID != ada || got.Visibility != domain.VisibilityPublic {
			t.Errorf("owner's Get() = %#v, %v", got, err)
		}
		if listed, err := f.quotes.ListPublic(ctx, ada, 10); err != nil || len(listed) != 0 {
			t.Errorf("ListPublic() = %#v, %v; want the removed quote left out", listed, err)
		}
		if c := f.measure(private.ID); c.Words != 0 {
			t.Errorf("Coverage() = %+v; a removed passage must not count", c)
		}

		// Moderators see any reader's quotes, removed or not.
		other := f.draft(bo, "Someone else's")
		found, err := f.quotes.ListAny(ctx, []string{public.ID, other.ID, "00000000-0000-4000-8000-000000000000"})
		if err != nil || len(found) != 2 {
			t.Errorf("ListAny() = %#v, %v; want both quotes, the missing id left out", found, err)
		}
		if none, err := f.quotes.ListAny(ctx, nil); err != nil || none == nil || len(none) != 0 {
			t.Errorf("ListAny(nil) = %#v, %v; want empty", none, err)
		}

		// The owner may unpublish a removed quote but not publish it again
		// (CR-003: the trigger's refusal comes back as ErrRemoved).
		if _, err := f.quotes.Update(ctx, ada, public.ID, domain.Patch{Visibility: ptr(domain.VisibilityPrivate)}); err != nil {
			t.Errorf("unpublishing a removed quote: %v", err)
		}
		for _, id := range []string{public.ID, private.ID} {
			err := pgtest.Attempt(t, ctx, db, func() error {
				_, err := f.quotes.Update(ctx, ada, id, domain.Patch{Visibility: ptr(domain.VisibilityPublic)})
				return err
			})
			if !errors.Is(err, domain.ErrRemoved) {
				t.Errorf("publishing a removed quote: err = %v, want domain.ErrRemoved", err)
			}
		}
		err = pgtest.Attempt(t, ctx, db, func() error {
			_, err := db.Exec(ctx, `
				insert into quotes (user_id, work_id, text, location, reaction, source, visibility, removed_at)
				values ($1, $2, 'Direct write', 'p. 1', 'Mine.', 'upload', 'public', now())`, ada, f.workID)
			return err
		})
		if !kernelpg.IsCheckViolation(err) {
			t.Errorf("inserting a removed public quote directly: err = %v, want a check violation", err)
		}

		// A table check constraint is a different error and stays unmapped.
		bare, err := f.quotes.Create(ctx, ada, domain.Draft{WorkID: f.workID, Text: "No reaction", Source: domain.SourceUpload})
		if err != nil {
			t.Fatal(err)
		}
		err = pgtest.Attempt(t, ctx, db, func() error {
			_, err := f.quotes.Update(ctx, ada, bare.ID, domain.Patch{Visibility: ptr(domain.VisibilityPublic)})
			return err
		})
		if errors.Is(err, domain.ErrRemoved) || !kernelpg.IsCheckViolation(err) {
			t.Errorf("publishing without a reaction: err = %v, want the raw check violation", err)
		}
	})
}

func TestRestoreRechecksTheCap(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		ada := pgtest.NewUser(t, ctx, db)
		f := newFixture(t, ctx, db, ada)
		if err := f.coverage.SetCap(ctx, 5); err != nil {
			t.Fatal(err)
		}
		f.publish(ada, f.draft(ada, "One two three").ID)
		hidden := f.publish(ada, f.draft(ada, "Four five").ID)
		if hidden.PassageHidden {
			t.Fatal("3 + 2 words must fit under 5")
		}
		if err := f.quotes.Remove(ctx, hidden.ID, "Spam"); err != nil {
			t.Fatal(err)
		}
		// Its words are free again, and another passage takes them.
		if got := f.publish(ada, f.draft(ada, "Six seven").ID); got.PassageHidden {
			t.Fatalf("3 + 2 words with the removed passage left out: %#v; want it shown", got)
		}

		if err := f.quotes.Restore(ctx, hidden.ID, 5); err != nil {
			t.Fatal(err)
		}
		got, err := f.quotes.Get(ctx, ada, hidden.ID)
		if err != nil || got.Removed() || got.RemovedReason != "" || !got.PassageHidden {
			t.Errorf("restored past the cap = %#v, %v; want it back, collapsed (5 + 2 > 5)", got, err)
		}
		if err := f.quotes.Restore(ctx, hidden.ID, 5); !errors.Is(err, kernel.ErrNotFound) {
			t.Errorf("Restore(not removed) = %v, want kernel.ErrNotFound", err)
		}

		// Restore applies the cap it is given, replacing the transaction's.
		roomy := f.publish(ada, f.draft(ada, "Eight nine").ID)
		if !roomy.PassageHidden {
			t.Fatal("past the cap of 5, want collapsed")
		}
		if _, err := f.quotes.Update(ctx, ada, roomy.ID, domain.Patch{Visibility: ptr(domain.VisibilityPrivate), PassageHidden: ptr(false)}); err != nil {
			t.Fatal(err)
		}
		if err := f.quotes.Remove(ctx, roomy.ID, "Spam"); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(ctx, `update quotes set visibility = 'public' where id = $1`, roomy.ID); err == nil {
			t.Fatal("making a removed quote public directly must fail")
		}
	})
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		ada := pgtest.NewUser(t, ctx, db)
		f := newFixture(t, ctx, db, ada)
		f.publish(ada, f.draft(ada, "One two three").ID)
		removed := f.publish(ada, f.draft(ada, "Four five six").ID)
		if err := f.quotes.Remove(ctx, removed.ID, "Spam"); err != nil {
			t.Fatal(err)
		}
		if err := f.quotes.Restore(ctx, removed.ID, 100); err != nil {
			t.Fatal(err)
		}
		if got, _ := f.quotes.Get(ctx, ada, removed.ID); got.PassageHidden || got.Removed() {
			t.Errorf("restored under the cap of 100 = %#v; want it shown", got)
		}
		if err := f.quotes.Restore(ctx, "00000000-0000-4000-8000-000000000000", 100); !errors.Is(err, kernel.ErrNotFound) {
			t.Errorf("Restore(missing) = %v, want kernel.ErrNotFound", err)
		}
	})
}

func TestCoverageCapIsTransactionLocal(t *testing.T) {
	cfg, err := pgxpool.ParseConfig(pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	// One connection, so every transaction below runs on the same session.
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	ctx := t.Context()
	db := kernelpg.NewDB(pool)
	txm := kernelpg.NewTxManager(db)
	coverage, quotes := NewCoverageRepository(db), NewQuoteRepository(db)
	setting := func(ctx context.Context) string {
		var v string
		if err := db.QueryRow(ctx, `select coalesce(current_setting('serendipity.public_word_cap', true), '')`).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	errRollback := errors.New("roll back")

	// Committed: the transaction writes nothing but the setting.
	err = txm.WithinTx(ctx, func(ctx context.Context) error {
		if err := coverage.SetCap(ctx, 7); err != nil {
			return err
		}
		if v := setting(ctx); v != "7" {
			t.Errorf("inside the transaction: %q, want 7", v)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if v := setting(ctx); v != "" {
		t.Errorf("after a committed transaction: %q, want it gone", v)
	}

	err = txm.WithinTx(ctx, func(ctx context.Context) error {
		if err := coverage.SetCap(ctx, 9); err != nil {
			return err
		}
		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatal(err)
	}
	if v := setting(ctx); v != "" {
		t.Errorf("after a rolled-back transaction: %q, want it gone", v)
	}

	// Restore sets the cap even when it finds nothing to restore.
	err = txm.WithinTx(ctx, func(ctx context.Context) error {
		err := quotes.Restore(ctx, "00000000-0000-4000-8000-000000000000", 11)
		if v := setting(ctx); v != "11" {
			t.Errorf("after Restore in the transaction: %q, want 11", v)
		}
		return err
	})
	if !errors.Is(err, kernel.ErrNotFound) {
		t.Fatalf("Restore(missing) = %v", err)
	}
	if v := setting(ctx); v != "" {
		t.Errorf("after Restore's transaction: %q, want it gone", v)
	}
}
