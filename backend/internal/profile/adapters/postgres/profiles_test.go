package postgres

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	catalogpg "github.com/an4eetos/serendipity/backend/internal/catalog/adapters/postgres"
	catalog "github.com/an4eetos/serendipity/backend/internal/catalog/domain"
	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres/pgtest"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	librarypg "github.com/an4eetos/serendipity/backend/internal/library/adapters/postgres"
	library "github.com/an4eetos/serendipity/backend/internal/library/domain"
	"github.com/an4eetos/serendipity/backend/internal/profile/domain"
	quotepg "github.com/an4eetos/serendipity/backend/internal/quote/adapters/postgres"
	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
)

func ptr[T any](v T) *T { return &v }

func TestProfileRepository(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		uid := pgtest.NewUser(t, ctx, db)
		other := pgtest.NewUser(t, ctx, db)
		repo := NewProfileRepository(db)

		p, err := repo.Get(ctx, uid)
		if err != nil || p.ID != uid || p.Username == "" || p.IsPublic {
			t.Fatalf("Get() = %#v, %v", p, err)
		}
		name := "pgtest_" + strings.ReplaceAll(uid[:8], "-", "")
		updated, err := repo.Update(ctx, uid, domain.Patch{Username: &name, Bio: ptr("Reads slowly."), IsPublic: ptr(true)})
		if err != nil || updated.Username != name || updated.Bio != "Reads slowly." || !updated.IsPublic || updated.DisplayName != p.DisplayName {
			t.Fatalf("Update() = %#v, %v", updated, err)
		}
		byName, err := repo.GetByUsername(ctx, name)
		if err != nil || byName.ID != uid {
			t.Errorf("GetByUsername() = %#v, %v", byName, err)
		}
		if _, err := repo.GetByUsername(ctx, "no_such_reader_here"); !errors.Is(err, kernel.ErrNotFound) {
			t.Errorf("GetByUsername(missing) err = %v", err)
		}

		// Constraint violations abort the test's transaction: check them last.
		if _, err := repo.Update(ctx, other, domain.Patch{Username: &name}); !errors.Is(err, domain.ErrUsernameTaken) {
			t.Errorf("taking a used username: err = %v", err)
		}
	})
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		uid := pgtest.NewUser(t, ctx, db)
		_, err := NewProfileRepository(db).Update(ctx, uid, domain.Patch{Bio: ptr(strings.Repeat("a", 501))})
		if !errors.Is(err, domain.ErrInvalidField) {
			t.Errorf("overlong bio: err = %v", err)
		}
	})
}

func TestShelfRepository(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		uid := pgtest.NewUser(t, ctx, db)
		works := catalogpg.NewWorkRepository(db)
		items := librarypg.NewLibraryRepository(db)
		quotes := quotepg.NewQuoteRepository(db)

		add := func(title string, subjects ...string) library.Item {
			w, err := works.Ensure(ctx, catalog.Work{Title: title, Subjects: subjects}, uid)
			if err != nil {
				t.Fatal(err)
			}
			it, err := items.Add(ctx, uid, w.ID)
			if err != nil {
				t.Fatal(err)
			}
			return it
		}
		republic := add("Republic", "Philosophy", "Politics")
		add("Meditations", "Philosophy", "Stoicism")

		q, err := quotes.Create(ctx, uid, quote.Draft{WorkID: republic.Work.ID, Text: "x", Location: "p. 1", Reaction: "y", Source: quote.SourceUpload})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := quotes.Create(ctx, uid, quote.Draft{WorkID: republic.Work.ID, Text: "private", Source: quote.SourceUpload}); err != nil {
			t.Fatal(err)
		}
		if _, err := quotes.Update(ctx, uid, q.ID, quote.Patch{Visibility: ptr(quote.VisibilityPublic)}); err != nil {
			t.Fatal(err)
		}

		shelf := NewShelfRepository(db)
		books, err := shelf.Books(ctx, uid)
		if err != nil || len(books) != 2 {
			t.Fatalf("Books() = %#v, %v", books, err)
		}
		for _, b := range books {
			want := 0
			if b.Work.ID == republic.Work.ID {
				want = 1
			}
			if b.PublicQuotes != want || b.Status != library.StatusReading {
				t.Errorf("book %q = %#v, want %d public quotes", b.Work.Title, b, want)
			}
		}

		categories, err := shelf.TopCategories(ctx, uid, 2)
		want := []domain.Category{{Name: "Philosophy", Books: 2}, {Name: "Politics", Books: 1}}
		if err != nil || !reflect.DeepEqual(categories, want) {
			t.Errorf("TopCategories() = %#v, %v; want %#v", categories, err, want)
		}
	})
}
