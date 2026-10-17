package postgres

import (
	"context"
	"errors"
	"testing"

	catalogpg "github.com/an4eetos/serendipity/backend/internal/catalog/adapters/postgres"
	catalog "github.com/an4eetos/serendipity/backend/internal/catalog/domain"
	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres/pgtest"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	"github.com/an4eetos/serendipity/backend/internal/quote/domain"
)

func ptr[T any](v T) *T { return &v }

func TestQuoteRepository(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		uid := pgtest.NewUser(t, ctx, db)
		other := pgtest.NewUser(t, ctx, db)
		works := catalogpg.NewWorkRepository(db)
		republic, err := works.Ensure(ctx, catalog.Work{Title: "The Republic", Authors: []string{"Plato"}}, uid)
		if err != nil {
			t.Fatal(err)
		}
		apology, err := works.Ensure(ctx, catalog.Work{Title: "Apology"}, uid)
		if err != nil {
			t.Fatal(err)
		}
		repo := NewQuoteRepository(db)

		late, err := repo.Create(ctx, uid, domain.Draft{
			WorkID: republic.ID, Text: "The beginning is the most important part of the work.",
			Location: "Page 12", Position: ptr(0.8), Reaction: "So start.", Source: domain.SourceUpload,
		})
		if err != nil {
			t.Fatal(err)
		}
		if late.Visibility != domain.VisibilityPrivate || late.Source != domain.SourceUpload ||
			late.Work.ID != republic.ID || late.Work.Title != "The Republic" {
			t.Errorf("Create() = %#v", late)
		}
		dup, err := repo.Create(ctx, uid, domain.Draft{WorkID: republic.ID, Text: late.Text, Source: domain.SourceUpload})
		if err != nil || dup.ID != late.ID || dup.Reaction != "So start." {
			t.Errorf("creating the same text again = %#v, %v; want the stored quote", dup, err)
		}

		early := domain.Draft{WorkID: republic.ID, Text: "Justice is the advantage of the stronger.", Position: ptr(0.1), Source: domain.SourceKindle}
		inserted, err := repo.Import(ctx, uid, early)
		if err != nil || !inserted {
			t.Fatalf("Import() = %v, %v", inserted, err)
		}
		if inserted, err := repo.Import(ctx, uid, early); err != nil || inserted {
			t.Errorf("re-import = %v, %v; want a skipped duplicate", inserted, err)
		}
		if _, err := repo.Import(ctx, uid, domain.Draft{WorkID: apology.ID, Text: "Know thyself.", Source: domain.SourceKindle}); err != nil {
			t.Fatal(err)
		}

		all, err := repo.List(ctx, uid, "")
		if err != nil || len(all) != 3 {
			t.Fatalf("List() = %d quotes, %v", len(all), err)
		}
		// Imports carry no position, so they come after positioned quotes.
		if all[0].ID != late.ID {
			t.Errorf("List() order: first = %q", all[0].Text)
		}
		byWork, err := repo.List(ctx, uid, apology.ID)
		if err != nil || len(byWork) != 1 || byWork[0].WorkID != apology.ID {
			t.Errorf("List(work) = %#v, %v", byWork, err)
		}

		if public, err := repo.ListPublic(ctx, uid, 10); err != nil || len(public) != 0 {
			t.Errorf("ListPublic() before publishing = %d, %v", len(public), err)
		}
		updated, err := repo.Update(ctx, uid, late.ID, domain.Patch{Visibility: ptr(domain.VisibilityPublic)})
		if err != nil || updated.Visibility != domain.VisibilityPublic {
			t.Fatalf("Update() = %#v, %v", updated, err)
		}
		if public, err := repo.ListPublic(ctx, uid, 10); err != nil || len(public) != 1 || public[0].ID != late.ID {
			t.Errorf("ListPublic() = %#v, %v", public, err)
		}

		if _, err := repo.Get(ctx, other, late.ID); !errors.Is(err, kernel.ErrNotFound) {
			t.Errorf("Get(other reader's quote) err = %v", err)
		}
		if err := repo.Delete(ctx, other, late.ID); !errors.Is(err, kernel.ErrNotFound) {
			t.Errorf("Delete(other reader's quote) err = %v", err)
		}
		if err := repo.Delete(ctx, uid, late.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.Get(ctx, uid, late.ID); !errors.Is(err, kernel.ErrNotFound) {
			t.Errorf("Get(deleted) err = %v", err)
		}
	})
}
