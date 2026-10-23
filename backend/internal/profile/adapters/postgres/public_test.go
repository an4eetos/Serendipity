package postgres

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	catalogpg "github.com/an4eetos/serendipity/backend/internal/catalog/adapters/postgres"
	catalog "github.com/an4eetos/serendipity/backend/internal/catalog/domain"
	discussionpg "github.com/an4eetos/serendipity/backend/internal/discussion/adapters/postgres"
	discussion "github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres/pgtest"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	librarypg "github.com/an4eetos/serendipity/backend/internal/library/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/profile/domain"
	"github.com/an4eetos/serendipity/backend/internal/profile/usecase"
	quotepg "github.com/an4eetos/serendipity/backend/internal/quote/adapters/postgres"
	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
)

func TestAcceptTerms(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		uid := pgtest.NewUser(t, ctx, db)
		repo := NewProfileRepository(db)
		p, err := repo.Get(ctx, uid)
		if err != nil || p.TermsVersion != "" || p.TermsAcceptedAt != nil {
			t.Fatalf("new profile = %#v, %v; want no terms accepted", p, err)
		}
		v1, err := repo.AcceptTerms(ctx, uid, "v1")
		if err != nil || v1.TermsVersion != "v1" || v1.TermsAcceptedAt == nil || v1.ID != uid {
			t.Fatalf("AcceptTerms(v1) = %#v, %v", v1, err)
		}
		v2, err := repo.AcceptTerms(ctx, uid, "v2")
		if err != nil || v2.TermsVersion != "v2" || v2.TermsAcceptedAt == nil || v2.TermsAcceptedAt.Before(*v1.TermsAcceptedAt) {
			t.Errorf("AcceptTerms(v2) = %#v, %v", v2, err)
		}
		if got, err := repo.Get(ctx, uid); err != nil || got.TermsVersion != "v2" {
			t.Errorf("Get() after accepting = %#v, %v", got, err)
		}
		if _, err := repo.AcceptTerms(ctx, "00000000-0000-4000-8000-000000000000", "v2"); !errors.Is(err, kernel.ErrNotFound) {
			t.Errorf("AcceptTerms(missing) = %v, want kernel.ErrNotFound", err)
		}
		// The column holds 64 characters, which is why config.Load refuses
		// a longer TERMS_VERSION.
		err = pgtest.Attempt(t, ctx, db, func() error {
			_, err := repo.AcceptTerms(ctx, uid, strings.Repeat("a", 65))
			return err
		})
		if !kernelpg.IsCheckViolation(err) {
			t.Errorf("AcceptTerms(65 characters) = %v, want a check violation", err)
		}
	})
}

// readerFixture is one reader's book with quotes, in the test's transaction.
type readerFixture struct {
	t      *testing.T
	ctx    context.Context
	db     *kernelpg.DB
	uid    string
	workID string
	quotes *quotepg.QuoteRepository
}

func newReader(t *testing.T, ctx context.Context, db *kernelpg.DB, title string) readerFixture {
	t.Helper()
	uid := pgtest.NewUser(t, ctx, db)
	w, err := catalogpg.NewWorkRepository(db).Ensure(ctx, catalog.Work{Title: title}, uid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := librarypg.NewLibraryRepository(db).Add(ctx, uid, w.ID); err != nil {
		t.Fatal(err)
	}
	return readerFixture{t: t, ctx: ctx, db: db, uid: uid, workID: w.ID, quotes: quotepg.NewQuoteRepository(db)}
}

// quote stores a cited quote with a reaction at position (nil: unplaced),
// public unless private is set.
func (f readerFixture) quote(text string, position *float64, private bool) quote.Quote {
	f.t.Helper()
	q, err := f.quotes.Create(f.ctx, f.uid, quote.Draft{
		WorkID: f.workID, Text: text, Location: "p. 1", Reaction: "Mine.", Position: position, Source: quote.SourceUpload,
	})
	if err != nil {
		f.t.Fatal(err)
	}
	if private {
		return q
	}
	q, err = f.quotes.Update(f.ctx, f.uid, q.ID, quote.Patch{Visibility: ptr(quote.VisibilityPublic)})
	if err != nil {
		f.t.Fatal(err)
	}
	return q
}

// collapse hides q's passage the way the coverage cap does.
func (f readerFixture) collapse(q quote.Quote) {
	f.t.Helper()
	if _, err := f.db.Exec(f.ctx, `update quotes set passage_hidden = true where id = $1`, q.ID); err != nil {
		f.t.Fatal(err)
	}
}

func TestShelfCountsAndTrailsListedQuotes(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		f := newReader(t, ctx, db, "Republic")
		late := f.quote("The beginning is the most important part.", ptr(0.8), false)
		early := f.quote("Justice is the advantage of the stronger.", ptr(0.2), false)
		f.quote("Unplaced, like a Kindle highlight.", nil, false)
		removed := f.quote("Removed by a moderator.", ptr(0.5), false)
		f.quote("Private.", ptr(0.3), true)
		collapsed := f.quote("Collapsed past the cap.", ptr(0.6), false)
		f.collapse(collapsed)
		if err := f.quotes.Remove(ctx, removed.ID, "Copyright"); err != nil {
			t.Fatal(err)
		}

		books, err := NewShelfRepository(db).Books(ctx, f.uid)
		if err != nil || len(books) != 1 {
			t.Fatalf("Books() = %#v, %v", books, err)
		}
		b := books[0]
		want := []domain.TrailPoint{{QuoteID: early.ID, Position: 0.2}, {QuoteID: collapsed.ID, Position: 0.6}, {QuoteID: late.ID, Position: 0.8}}
		if b.PublicQuotes != 4 || !reflect.DeepEqual(b.Trail, want) || b.Unplaced() != 1 {
			t.Errorf("book = %d public quotes, trail %+v, %d unplaced; want 4, %+v, 1 (removed and private left out)",
				b.PublicQuotes, b.Trail, b.Unplaced(), want)
		}
	})
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		f := newReader(t, ctx, db, "Meditations")
		books, err := NewShelfRepository(db).Books(ctx, f.uid)
		if err != nil || len(books) != 1 || books[0].PublicQuotes != 0 || books[0].Trail == nil || len(books[0].Trail) != 0 {
			t.Errorf("book without quotes = %#v, %v; want an empty, non-nil trail", books, err)
		}
	})
}

// TestPublicProfileReads reads a profile the way GET /profiles/{username}
// does, through every repository it uses.
func TestPublicProfileReads(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		f := newReader(t, ctx, db, "Apology")
		profiles := NewProfileRepository(db)
		name := "pgtest_" + strings.ReplaceAll(f.uid[:8], "-", "")
		if _, err := profiles.Update(ctx, f.uid, domain.Patch{Username: &name, IsPublic: ptr(true)}); err != nil {
			t.Fatal(err)
		}
		shown := f.quote("Know thyself.", ptr(0.1), false)
		collapsed := f.quote("The unexamined life is not worth living.", ptr(0.5), false)
		f.collapse(collapsed)
		removed := f.quote("I know that I know nothing.", ptr(0.9), false)
		f.quote("Private thoughts.", nil, true)

		personas, err := discussionpg.NewPersonaRepository(db).List(ctx, f.uid)
		if err != nil || len(personas) == 0 {
			t.Fatalf("no personas: %v", err)
		}
		convs := discussionpg.NewConversationRepository(db)
		discuss := func(q quote.Quote) string {
			id, err := convs.Create(ctx, f.uid, q.ID, personas[0].ID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := convs.AppendMessage(ctx, id, discussion.RoleAssistant, "Do you, though?"); err != nil {
				t.Fatal(err)
			}
			return id
		}
		kept := discuss(shown)
		hiddenConv := discuss(shown)
		onCollapsed := discuss(collapsed)
		discuss(removed)
		if err := convs.Remove(ctx, hiddenConv, "Harassment"); err != nil {
			t.Fatal(err)
		}
		if err := f.quotes.Remove(ctx, removed.ID, "Copyright"); err != nil {
			t.Fatal(err)
		}

		p, err := usecase.NewGetPublicProfile(profiles, NewShelfRepository(db), f.quotes, convs).Execute(ctx, "", name)
		if err != nil {
			t.Fatal(err)
		}
		got := map[string][]string{}
		for _, th := range p.Quotes {
			ids := []string{}
			for _, c := range th.Conversations {
				ids = append(ids, c.ID)
			}
			got[th.Quote.ID] = ids
			switch th.Quote.ID {
			case shown.ID:
				if th.Quote.Text != shown.Text || th.Quote.PassageHidden {
					t.Errorf("shown quote = %#v", th.Quote)
				}
			case collapsed.ID:
				if th.Quote.Text != "" || !th.Quote.PassageHidden || th.Quote.Reaction != "Mine." || th.Quote.Location != "p. 1" {
					t.Errorf("collapsed quote = %#v; want the passage blanked, the reaction and citation kept", th.Quote)
				}
			}
			if th.Quote.RemovedAt != nil || th.Quote.RemovedReason != "" {
				t.Errorf("public quote carries removal details: %#v", th.Quote)
			}
		}
		want := map[string][]string{shown.ID: {kept}, collapsed.ID: {onCollapsed}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("public quotes and their discussions = %v, want %v (removed quote, its discussion, the removed discussion and the private quote left out)", got, want)
		}
		if len(p.Books) != 1 || p.Books[0].PublicQuotes != 2 || len(p.Books[0].Trail) != 2 {
			t.Errorf("books = %+v; want 2 listed quotes on the trail", p.Books)
		}
	})
}

func TestProfilesRowLevelSecurity(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		uid, other := pgtest.NewUser(t, ctx, db), pgtest.NewUser(t, ctx, db)
		if _, err := NewProfileRepository(db).Update(ctx, other, domain.Patch{IsPublic: ptr(true)}); err != nil {
			t.Fatal(err)
		}
		pgtest.AsUser(t, ctx, db, uid, func() {
			var own, others int
			if err := db.QueryRow(ctx, `select count(*) filter (where id = $1), count(*) filter (where id <> $1) from profiles`, uid).
				Scan(&own, &others); err != nil {
				t.Fatal(err)
			}
			if own != 1 || others != 0 {
				t.Errorf("signed-in client reads %d own and %d other profiles; want 1 and 0, even public ones", own, others)
			}
			for _, sql := range []string{
				`update profiles set terms_version = 'forged', terms_accepted_at = now() where id = $1`,
				`update profiles set is_public = true, username = 'forged_name' where id = $1`,
				`delete from profiles where id = $1`,
			} {
				for _, id := range []string{uid, other} {
					tag, err := db.Exec(ctx, sql, id)
					if err != nil || tag.RowsAffected() != 0 {
						t.Errorf("%s on %s: %d rows, %v; want none", sql, id, tag.RowsAffected(), err)
					}
				}
			}
		})
		p, err := NewProfileRepository(db).Get(ctx, uid)
		if err != nil || p.TermsVersion != "" || p.IsPublic {
			t.Errorf("profile after direct writes = %#v, %v; want it unchanged", p, err)
		}
	})
}
