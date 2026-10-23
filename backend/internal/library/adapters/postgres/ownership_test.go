package postgres

import (
	"context"
	"testing"

	catalogpg "github.com/an4eetos/serendipity/backend/internal/catalog/adapters/postgres"
	catalog "github.com/an4eetos/serendipity/backend/internal/catalog/domain"
	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres/pgtest"
	"github.com/an4eetos/serendipity/backend/internal/library/domain"
)

func TestOwnershipConfirmedAt(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		uid := pgtest.NewUser(t, ctx, db)
		work, err := catalogpg.NewWorkRepository(db).Ensure(ctx, catalog.Work{Title: "Phaedo"}, uid)
		if err != nil {
			t.Fatal(err)
		}
		repo := NewLibraryRepository(db)
		item, err := repo.Add(ctx, uid, work.ID)
		if err != nil || item.OwnershipConfirmedAt != nil {
			t.Fatalf("Add() = %#v, %v; want no confirmation yet", item, err)
		}

		progressed, err := repo.Update(ctx, uid, item.ID, domain.Patch{Progress: ptr(0.3)})
		if err != nil || progressed.OwnershipConfirmedAt != nil {
			t.Errorf("Update without a confirmation = %#v, %v; want it left unset", progressed, err)
		}
		uploaded, err := repo.Update(ctx, uid, item.ID, domain.Patch{
			FilePath: ptr(uid + "/phaedo.epub"), FileType: ptr(domain.FileEPUB), OwnershipConfirmed: true,
		})
		if err != nil || uploaded.OwnershipConfirmedAt == nil || uploaded.FilePath == nil {
			t.Fatalf("Update with a file and the confirmation = %#v, %v", uploaded, err)
		}
		again, err := repo.Update(ctx, uid, item.ID, domain.Patch{Progress: ptr(0.6)})
		if err != nil || again.OwnershipConfirmedAt == nil || !again.OwnershipConfirmedAt.Equal(*uploaded.OwnershipConfirmedAt) {
			t.Errorf("a later Update without a confirmation = %#v, %v; want the confirmation kept", again, err)
		}
		listed, err := repo.List(ctx, uid)
		if err != nil || len(listed) != 1 || listed[0].OwnershipConfirmedAt == nil {
			t.Errorf("List() = %#v, %v; want the confirmation read back", listed, err)
		}
	})
}

// A signed-in client reaching the database directly can read its own
// library but write none of it, and still uploads to its own folder of the
// books bucket.
func TestLibraryRowLevelSecurity(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		uid, other := pgtest.NewUser(t, ctx, db), pgtest.NewUser(t, ctx, db)
		if _, err := db.Exec(ctx, `update profiles set is_public = true where id = $1`, other); err != nil {
			t.Fatal(err)
		}
		works := catalogpg.NewWorkRepository(db)
		work, err := works.Ensure(ctx, catalog.Work{Title: "Crito"}, uid)
		if err != nil {
			t.Fatal(err)
		}
		repo := NewLibraryRepository(db)
		mine, err := repo.Add(ctx, uid, work.ID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := repo.Add(ctx, other, work.ID); err != nil {
			t.Fatal(err)
		}

		pgtest.AsUser(t, ctx, db, uid, func() {
			var own, others, worksSeen int
			if err := db.QueryRow(ctx, `
				select count(*) filter (where user_id = $1), count(*) filter (where user_id <> $1),
					(select count(*) from works where id = $2)
				from library_items`, uid, work.ID).Scan(&own, &others, &worksSeen); err != nil {
				t.Fatal(err)
			}
			if own != 1 || others != 0 || worksSeen != 1 {
				t.Errorf("reads %d own and %d other library items, %d works; want 1, 0 (even on a public profile) and 1", own, others, worksSeen)
			}
			for _, sql := range []string{
				`update library_items set ownership_confirmed_at = now() where id = $1`,
				`update library_items set file_path = user_id::text || '/x.epub', file_type = 'epub' where id = $1`,
				`delete from library_items where id = $1`,
			} {
				tag, err := db.Exec(ctx, sql, mine.ID)
				if err != nil || tag.RowsAffected() != 0 {
					t.Errorf("%s: %d rows, %v; want none", sql, tag.RowsAffected(), err)
				}
			}
			for name, insert := range map[string]func() error{
				"library item": func() error {
					_, err := db.Exec(ctx, `insert into library_items (user_id, work_id) values ($1, $2)`, uid, work.ID)
					return err
				},
				"own character": func() error {
					_, err := db.Exec(ctx, `insert into personas (slug, name, system_prompt, owner_id)
						values ('pgtest-forged', 'Forged', 'x', $1)`, uid)
					return err
				},
				"upload into another reader's folder": func() error {
					_, err := db.Exec(ctx, `insert into storage.objects (bucket_id, name, owner) values ('books', $1, $2)`,
						other+"/theirs.epub", uid)
					return err
				},
			} {
				if err := pgtest.Attempt(t, ctx, db, insert); !pgtest.IsInsufficientPrivilege(err) {
					t.Errorf("insert %s: err = %v, want it refused", name, err)
				}
			}
			// Uploading the reader's own copy still works: the web app does it
			// with the Supabase client, straight into storage.
			err := pgtest.Attempt(t, ctx, db, func() error {
				_, err := db.Exec(ctx, `insert into storage.objects (bucket_id, name, owner) values ('books', $1, $2)`,
					uid+"/crito.epub", uid)
				return err
			})
			if err != nil {
				t.Errorf("uploading into the reader's own folder: %v", err)
			}
		})

		if got, err := repo.Get(ctx, uid, mine.ID); err != nil || got.OwnershipConfirmedAt != nil || got.FilePath != nil {
			t.Errorf("library item after direct writes = %#v, %v; want it unchanged", got, err)
		}
		var policies int
		if err := db.QueryRow(ctx, `select count(*) from pg_policies where schemaname = 'storage' and tablename = 'objects'
			and policyname in ('books: owner reads', 'books: owner uploads', 'books: owner updates', 'books: owner deletes')`).
			Scan(&policies); err != nil || policies != 4 {
			t.Errorf("books storage policies = %d, %v; want all 4 kept", policies, err)
		}
	})
}
