package postgres

import (
	"context"
	"errors"
	"testing"

	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres/pgtest"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	"github.com/an4eetos/serendipity/backend/internal/moderation/domain"
)

// Report and takedown targets are a kind and an id with no foreign key, so
// these tests need no quotes.
var (
	quoteOne = domain.TargetRef{Kind: domain.TargetQuote, ID: "00000000-0000-4000-8000-0000000000a1"}
	quoteTwo = domain.TargetRef{Kind: domain.TargetQuote, ID: "00000000-0000-4000-8000-0000000000a2"}
	convOne  = domain.TargetRef{Kind: domain.TargetConversation, ID: "00000000-0000-4000-8000-0000000000a1"}
)

func username(t *testing.T, ctx context.Context, db *kernelpg.DB, id string) string {
	t.Helper()
	var name string
	if err := db.QueryRow(ctx, `select username from profiles where id = $1`, id).Scan(&name); err != nil {
		t.Fatal(err)
	}
	return name
}

// age moves rows of table back in time, so that ordering by created_at is
// testable inside one transaction, where now() doesn't move.
func age(t *testing.T, ctx context.Context, db *kernelpg.DB, table, id, by string) {
	t.Helper()
	if _, err := db.Exec(ctx, `update `+table+` set created_at = created_at - $2::interval where id = $1`, id, by); err != nil {
		t.Fatal(err)
	}
}

func TestReportRepository(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		ada, bo, cy, admin := pgtest.NewUser(t, ctx, db), pgtest.NewUser(t, ctx, db), pgtest.NewUser(t, ctx, db), pgtest.NewUser(t, ctx, db)
		repo := NewReportRepository(db)
		report := func(reporter string, target domain.TargetRef) (domain.Report, error) {
			return repo.Create(ctx, domain.Report{ReporterID: reporter, Target: target, OwnerID: ada, Reason: domain.ReasonCopyright, Note: "p. 3"})
		}

		first, err := report(bo, quoteOne)
		if err != nil || first.ID == "" || first.Status != domain.StatusOpen || first.CreatedAt.IsZero() || first.Note != "p. 3" {
			t.Fatalf("Create() = %#v, %v", first, err)
		}
		if err := pgtest.Attempt(t, ctx, db, func() error { _, err := report(bo, quoteOne); return err }); !errors.Is(err, domain.ErrAlreadyReported) {
			t.Errorf("second open report by the same reader: err = %v, want ErrAlreadyReported", err)
		}
		second, err := report(cy, quoteOne)
		if err != nil {
			t.Errorf("another reader's report on the same target: %v", err)
		}
		// The same id as another kind is another target.
		onConv, err := report(bo, convOne)
		if err != nil {
			t.Errorf("report on a discussion with the quote's id: %v", err)
		}
		if err := pgtest.Attempt(t, ctx, db, func() error { _, err := report(ada, quoteTwo); return err }); !errors.Is(err, domain.ErrOwnContent) {
			t.Errorf("report on own content: err = %v, want ErrOwnContent", err)
		}

		age(t, ctx, db, "reports", first.ID, "2 hours")
		age(t, ctx, db, "reports", second.ID, "1 hour")
		open, err := repo.ListOpen(ctx, 10)
		if err != nil || len(open) != 3 || open[0].ID != first.ID || open[1].ID != second.ID || open[2].ID != onConv.ID {
			t.Fatalf("ListOpen() = %#v, %v; want the 3 reports, oldest first", open, err)
		}
		if open[0].OwnerID != ada || open[0].OwnerUsername != username(t, ctx, db, ada) || open[0].ReporterID != bo || open[0].Target != quoteOne {
			t.Errorf("ListOpen()[0] = %#v", open[0])
		}
		if limited, err := repo.ListOpen(ctx, 2); err != nil || len(limited) != 2 {
			t.Errorf("ListOpen(2) = %d, %v", len(limited), err)
		}

		n, err := repo.Resolve(ctx, quoteOne, domain.StatusActioned, admin)
		if err != nil || n != 2 {
			t.Errorf("Resolve(quote) = %d, %v; want both open reports on it closed, not the discussion's", n, err)
		}
		if n, err := repo.Resolve(ctx, quoteOne, domain.StatusDismissed, admin); err != nil || n != 0 {
			t.Errorf("Resolve() again = %d, %v; want 0", n, err)
		}
		var status string
		var resolvedBy *string
		if err := db.QueryRow(ctx, `select status, resolved_by from reports where id = $1 and resolved_at is not null`, first.ID).
			Scan(&status, &resolvedBy); err != nil || status != "actioned" || resolvedBy == nil || *resolvedBy != admin {
			t.Errorf("resolved report = %q by %v, %v", status, resolvedBy, err)
		}
		if open, err := repo.ListOpen(ctx, 10); err != nil || len(open) != 1 || open[0].ID != onConv.ID {
			t.Errorf("ListOpen() after resolving = %#v, %v", open, err)
		}
		// Only open reports are unique: the reader may report it again.
		if _, err := report(bo, quoteOne); err != nil {
			t.Errorf("reporting again after the first report was closed: %v", err)
		}
	})
}

func TestTakedownRepository(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		ada, admin := pgtest.NewUser(t, ctx, db), pgtest.NewUser(t, ctx, db)
		repo := NewTakedownRepository(db)
		hide := func(target domain.TargetRef) (domain.Takedown, error) {
			return repo.Create(ctx, domain.Takedown{Target: target, OwnerID: ada, Reason: "Publisher's notice", HiddenBy: &admin})
		}

		first, err := hide(quoteOne)
		if err != nil || first.ID == "" || first.Target != quoteOne || first.OwnerID != ada || first.OwnerUsername != username(t, ctx, db, ada) ||
			first.Reason != "Publisher's notice" || first.HiddenBy == nil || *first.HiddenBy != admin || first.RestoredAt != nil {
			t.Fatalf("Create() = %#v, %v", first, err)
		}
		if err := pgtest.Attempt(t, ctx, db, func() error { _, err := hide(quoteOne); return err }); !errors.Is(err, domain.ErrAlreadyRemoved) {
			t.Errorf("second active takedown of a target: err = %v, want ErrAlreadyRemoved", err)
		}
		second, err := hide(quoteTwo)
		if err != nil {
			t.Fatal(err)
		}
		age(t, ctx, db, "takedowns", first.ID, "1 hour")
		active, err := repo.ListActive(ctx, 10)
		if err != nil || len(active) != 2 || active[0].ID != second.ID || active[1].ID != first.ID {
			t.Fatalf("ListActive() = %#v, %v; want both, newest first", active, err)
		}

		restored, err := repo.MarkRestored(ctx, first.ID, admin)
		if err != nil || restored.RestoredAt == nil || restored.RestoredBy == nil || *restored.RestoredBy != admin {
			t.Fatalf("MarkRestored() = %#v, %v", restored, err)
		}
		if err := restored.CheckRestorable(); !errors.Is(err, domain.ErrAlreadyRestored) {
			t.Errorf("restored takedown is still restorable: %v", err)
		}
		if _, err := repo.MarkRestored(ctx, first.ID, admin); !errors.Is(err, domain.ErrAlreadyRestored) {
			t.Errorf("MarkRestored() again = %v, want ErrAlreadyRestored", err)
		}
		if active, err := repo.ListActive(ctx, 10); err != nil || len(active) != 1 || active[0].ID != second.ID {
			t.Errorf("ListActive() after restoring = %#v, %v", active, err)
		}
		if got, err := repo.Get(ctx, first.ID); err != nil || got.RestoredAt == nil {
			t.Errorf("Get(restored) = %#v, %v", got, err)
		}
		// One active takedown per target: once restored, it may be hidden again.
		if _, err := hide(quoteOne); err != nil {
			t.Errorf("hiding a restored target again: %v", err)
		}
		if _, err := repo.Get(ctx, "00000000-0000-4000-8000-000000000000"); !errors.Is(err, kernel.ErrNotFound) {
			t.Errorf("Get(missing) = %v, want kernel.ErrNotFound", err)
		}
	})
}

func TestAdminRepository(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		uid, admin := pgtest.NewUser(t, ctx, db), pgtest.NewUser(t, ctx, db)
		repo := NewAdminRepository(db)
		if ok, err := repo.IsAdmin(ctx, admin); err != nil || ok {
			t.Fatalf("IsAdmin() before the row = %v, %v", ok, err)
		}
		if _, err := db.Exec(ctx, `insert into admins (user_id) values ($1)`, admin); err != nil {
			t.Fatal(err)
		}
		if ok, err := repo.IsAdmin(ctx, admin); err != nil || !ok {
			t.Errorf("IsAdmin(admin) = %v, %v", ok, err)
		}
		if ok, err := repo.IsAdmin(ctx, uid); err != nil || ok {
			t.Errorf("IsAdmin(reader) = %v, %v", ok, err)
		}
	})
}

// Only the API's privileged role reads or writes the moderation tables: a
// client that reaches the database directly can't list reports, see who
// reported what, or make itself an admin.
func TestModerationTablesAreClosedToClients(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		uid, other := pgtest.NewUser(t, ctx, db), pgtest.NewUser(t, ctx, db)
		if _, err := NewReportRepository(db).Create(ctx, domain.Report{
			ReporterID: uid, Target: quoteOne, OwnerID: other, Reason: domain.ReasonSpam,
		}); err != nil {
			t.Fatal(err)
		}
		type statement struct {
			sql  string
			args []any
		}
		statements := map[string]statement{
			"read admins":       {`select count(*) from admins`, nil},
			"read reports":      {`select count(*) from reports`, nil},
			"read takedowns":    {`select count(*) from takedowns`, nil},
			"become an admin":   {`insert into admins (user_id) values ($1)`, []any{uid}},
			"clear own reports": {`delete from reports`, nil},
			"forge a takedown": {`insert into takedowns (target_kind, target_id, target_owner_id, reason)
				values ('quote', 'x', $1, 'x')`, []any{other}},
		}
		check := func(role string) {
			for name, st := range statements {
				err := pgtest.Attempt(t, ctx, db, func() error { _, err := db.Exec(ctx, st.sql, st.args...); return err })
				if !pgtest.IsInsufficientPrivilege(err) {
					t.Errorf("%s may %s: err = %v, want permission denied", role, name, err)
				}
			}
		}
		pgtest.AsUser(t, ctx, db, uid, func() { check("a signed-in client") })

		if _, err := db.Exec(ctx, `set local role anon`); err != nil {
			t.Fatal(err)
		}
		check("an anonymous client")
		if _, err := db.Exec(ctx, `reset role`); err != nil {
			t.Fatal(err)
		}

		var admins int
		if err := db.QueryRow(ctx, `select count(*) from admins where user_id = $1`, uid).Scan(&admins); err != nil || admins != 0 {
			t.Errorf("admins rows for the client = %d, %v", admins, err)
		}
	})
}
