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
	"github.com/an4eetos/serendipity/backend/internal/library/domain"
)

func ptr[T any](v T) *T { return &v }

func TestLibraryRepository(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		uid := pgtest.NewUser(t, ctx, db)
		other := pgtest.NewUser(t, ctx, db)
		work, err := catalogpg.NewWorkRepository(db).Ensure(ctx, catalog.Work{Title: "The Republic", Authors: []string{"Plato"}}, uid)
		if err != nil {
			t.Fatal(err)
		}
		repo := NewLibraryRepository(db)

		item, err := repo.Add(ctx, uid, work.ID)
		if err != nil {
			t.Fatal(err)
		}
		if item.Work.ID != work.ID || item.Status != domain.StatusReading || item.Progress != 0 || item.FilePath != nil {
			t.Errorf("Add() = %#v", item)
		}
		again, err := repo.Add(ctx, uid, work.ID)
		if err != nil || again.ID != item.ID {
			t.Errorf("adding twice gave %v, %v; want item %s", again.ID, err, item.ID)
		}

		found, err := repo.FindByTitle(ctx, uid, "the REPUBLIC")
		if err != nil || found.ID != item.ID {
			t.Errorf("FindByTitle() = %v, %v", found.ID, err)
		}
		if _, err := repo.FindByTitle(ctx, uid, "Phaedo"); !errors.Is(err, kernel.ErrNotFound) {
			t.Errorf("FindByTitle(missing) err = %v", err)
		}

		updated, err := repo.Update(ctx, uid, item.ID, domain.Patch{
			Progress: ptr(0.25),
			Status:   ptr(domain.StatusFinished),
			FilePath: ptr(uid + "/republic.epub"),
			FileType: ptr(domain.FileEPUB),
		})
		if err != nil {
			t.Fatal(err)
		}
		if updated.Progress != 0.25 || updated.Status != domain.StatusFinished ||
			updated.FileType == nil || *updated.FileType != domain.FileEPUB {
			t.Errorf("Update() = %#v", updated)
		}

		items, err := repo.List(ctx, uid)
		if err != nil || len(items) != 1 || items[0].ID != item.ID {
			t.Errorf("List() = %v, %v", items, err)
		}
		if items, err := repo.List(ctx, other); err != nil || items == nil || len(items) != 0 {
			t.Errorf("List(other) = %#v, %v; want empty", items, err)
		}

		if _, err := repo.Get(ctx, other, item.ID); !errors.Is(err, kernel.ErrNotFound) {
			t.Errorf("Get(other reader's item) err = %v", err)
		}
		if _, err := repo.Update(ctx, other, item.ID, domain.Patch{Progress: ptr(1.0)}); !errors.Is(err, kernel.ErrNotFound) {
			t.Errorf("Update(other reader's item) err = %v", err)
		}
		// Last: a malformed id aborts the test's transaction.
		if _, err := repo.Get(ctx, uid, "not-a-uuid"); !errors.Is(err, kernel.ErrNotFound) {
			t.Errorf("Get(malformed id) err = %v", err)
		}
	})
}
