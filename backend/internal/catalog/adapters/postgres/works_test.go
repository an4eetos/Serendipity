package postgres

import (
	"context"
	"reflect"
	"testing"

	"github.com/an4eetos/serendipity/backend/internal/catalog/domain"
	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres/pgtest"
)

func TestWorkRepositoryEnsure(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		uid := pgtest.NewUser(t, ctx, db)
		repo := NewWorkRepository(db)

		key := "/works/OL-pgtest-1W"
		first, err := repo.Ensure(ctx, domain.Work{Title: "Republic", Authors: []string{"Plato"}, OpenLibraryKey: &key}, uid)
		if err != nil {
			t.Fatal(err)
		}
		if first.ID == "" || first.Title != "Republic" || !reflect.DeepEqual(first.Authors, []string{"Plato"}) {
			t.Errorf("Ensure() = %#v", first)
		}
		if first.Subjects == nil || len(first.Subjects) != 0 {
			t.Errorf("Subjects = %#v, want empty", first.Subjects)
		}

		again, err := repo.Ensure(ctx, domain.Work{Title: "Republic (2nd ed.)", OpenLibraryKey: &key}, uid)
		if err != nil {
			t.Fatal(err)
		}
		if again.ID != first.ID || again.Title != "Republic" {
			t.Errorf("same key gave %#v, want the stored work %s", again, first.ID)
		}

		a, err := repo.Ensure(ctx, domain.Work{Title: "Notes"}, uid)
		if err != nil {
			t.Fatal(err)
		}
		b, err := repo.Ensure(ctx, domain.Work{Title: "Notes"}, uid)
		if err != nil {
			t.Fatal(err)
		}
		if a.ID == b.ID {
			t.Error("works without a key must always be created")
		}
	})
}
