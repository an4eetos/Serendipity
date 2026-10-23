package postgres

import (
	"context"
	"errors"
	"testing"

	catalogpg "github.com/an4eetos/serendipity/backend/internal/catalog/adapters/postgres"
	catalog "github.com/an4eetos/serendipity/backend/internal/catalog/domain"
	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres/pgtest"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	quotepg "github.com/an4eetos/serendipity/backend/internal/quote/adapters/postgres"
	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
)

// discussed is a reader's public quote with two answered discussions.
type discussed struct {
	uid, other string
	quote      quote.Quote
	convs      []string
	repo       *ConversationRepository
}

func newDiscussed(t *testing.T, ctx context.Context, db *kernelpg.DB) discussed {
	t.Helper()
	d := discussed{uid: pgtest.NewUser(t, ctx, db), other: pgtest.NewUser(t, ctx, db), repo: NewConversationRepository(db)}
	work, err := catalogpg.NewWorkRepository(db).Ensure(ctx, catalog.Work{Title: "Apology"}, d.uid)
	if err != nil {
		t.Fatal(err)
	}
	quotes := quotepg.NewQuoteRepository(db)
	q, err := quotes.Create(ctx, d.uid, quote.Draft{WorkID: work.ID, Text: "Know thyself.", Location: "p. 1", Reaction: "Hard.", Source: quote.SourceUpload})
	if err != nil {
		t.Fatal(err)
	}
	if d.quote, err = quotes.Update(ctx, d.uid, q.ID, quote.Patch{Visibility: ptr(quote.VisibilityPublic)}); err != nil {
		t.Fatal(err)
	}
	personas, err := NewPersonaRepository(db).List(ctx, d.uid)
	if err != nil || len(personas) == 0 {
		t.Fatalf("no personas: %v", err)
	}
	for range 2 {
		id, err := d.repo.Create(ctx, d.uid, d.quote.ID, personas[0].ID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.repo.AppendMessage(ctx, id, domain.RoleAssistant, "Do you, though?"); err != nil {
			t.Fatal(err)
		}
		d.convs = append(d.convs, id)
	}
	return d
}

func ptr[T any](v T) *T { return &v }

func TestConversationModeration(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		d := newDiscussed(t, ctx, db)
		removed, kept := d.convs[0], d.convs[1]

		if err := d.repo.Remove(ctx, removed, "Harassment"); err != nil {
			t.Fatal(err)
		}
		if err := d.repo.Remove(ctx, removed, "again"); !errors.Is(err, kernel.ErrNotFound) {
			t.Errorf("Remove(removed) = %v, want kernel.ErrNotFound", err)
		}
		if err := d.repo.Remove(ctx, "00000000-0000-4000-8000-000000000000", "x"); !errors.Is(err, kernel.ErrNotFound) {
			t.Errorf("Remove(missing) = %v, want kernel.ErrNotFound", err)
		}

		// Public pages leave the removed discussion out.
		listed, err := d.repo.ListForQuotes(ctx, d.uid, []string{d.quote.ID})
		if err != nil || len(listed) != 1 || listed[0].ID != kept {
			t.Errorf("ListForQuotes() = %#v, %v; want only the discussion that isn't removed", listed, err)
		}
		// The owner still sees it, with the reason.
		own, err := d.repo.Get(ctx, d.uid, removed)
		if err != nil || !own.Removed() || own.RemovedReason != "Harassment" || own.UserID != d.uid || len(own.Messages) != 1 {
			t.Errorf("owner's Get() = %#v, %v", own, err)
		}
		if forQuote, err := d.repo.ListForQuote(ctx, d.uid, d.quote.ID); err != nil || len(forQuote) != 2 {
			t.Errorf("owner's ListForQuote() = %d discussions, %v; want both", len(forQuote), err)
		}
		// Moderators see any reader's discussions, removed or not.
		found, err := d.repo.ListAny(ctx, []string{removed, kept, "00000000-0000-4000-8000-000000000000"})
		if err != nil || len(found) != 2 || found[0].UserID != d.uid || len(found[0].Messages) != 1 {
			t.Errorf("ListAny() = %#v, %v; want both discussions with their messages", found, err)
		}
		if none, err := d.repo.ListAny(ctx, nil); err != nil || none == nil || len(none) != 0 {
			t.Errorf("ListAny(nil) = %#v, %v; want empty", none, err)
		}

		if err := d.repo.Restore(ctx, removed); err != nil {
			t.Fatal(err)
		}
		if err := d.repo.Restore(ctx, removed); !errors.Is(err, kernel.ErrNotFound) {
			t.Errorf("Restore(restored) = %v, want kernel.ErrNotFound", err)
		}
		if listed, err := d.repo.ListForQuotes(ctx, d.uid, []string{d.quote.ID}); err != nil || len(listed) != 2 {
			t.Errorf("ListForQuotes() after restoring = %d, %v; want both", len(listed), err)
		}
		if got, err := d.repo.Get(ctx, d.uid, removed); err != nil || got.Removed() || got.RemovedReason != "" {
			t.Errorf("Get() after restoring = %#v, %v", got, err)
		}
	})
}

func TestConversationsRowLevelSecurity(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		d := newDiscussed(t, ctx, db)
		if err := d.repo.Remove(ctx, d.convs[0], "Harassment"); err != nil {
			t.Fatal(err)
		}
		mine := d.convs[1]

		// Another reader sees none of it, although the quote is public.
		pgtest.AsUser(t, ctx, db, d.other, func() {
			var convs, msgs int
			if err := db.QueryRow(ctx, `select (select count(*) from conversations), (select count(*) from messages)`).Scan(&convs, &msgs); err != nil {
				t.Fatal(err)
			}
			if convs != 0 || msgs != 0 {
				t.Errorf("another reader reads %d discussions and %d messages; want none", convs, msgs)
			}
		})

		pgtest.AsUser(t, ctx, db, d.uid, func() {
			var convs, msgs int
			if err := db.QueryRow(ctx, `select (select count(*) from conversations), (select count(*) from messages)`).Scan(&convs, &msgs); err != nil {
				t.Fatal(err)
			}
			if convs != 2 || msgs != 2 {
				t.Errorf("owner reads %d discussions and %d messages; want 2 and 2", convs, msgs)
			}
			for _, w := range []struct{ name, sql, id string }{
				{"clear a takedown", `update conversations set removed_at = null, removed_reason = '' where id = $1`, d.convs[0]},
				{"move a discussion", `update conversations set quote_id = quote_id where id = $1`, mine},
				{"delete a discussion", `delete from conversations where id = $1`, mine},
				{"rewrite a reply", `update messages set content = 'forged' where conversation_id = $1`, mine},
				{"delete a reply", `delete from messages where conversation_id = $1`, mine},
			} {
				tag, err := db.Exec(ctx, w.sql, w.id)
				if err != nil || tag.RowsAffected() != 0 {
					t.Errorf("%s: %d rows, %v; want none", w.name, tag.RowsAffected(), err)
				}
			}
			for name, insert := range map[string]func() error{
				"insert a discussion": func() error {
					_, err := db.Exec(ctx, `insert into conversations (user_id, quote_id, persona_id)
						select user_id, quote_id, persona_id from conversations where id = $1`, mine)
					return err
				},
				"insert a reply": func() error {
					_, err := db.Exec(ctx, `insert into messages (conversation_id, role, content) values ($1, 'assistant', 'forged')`, mine)
					return err
				},
			} {
				if err := pgtest.Attempt(t, ctx, db, insert); !pgtest.IsInsufficientPrivilege(err) {
					t.Errorf("%s: err = %v, want a row-level security violation", name, err)
				}
			}
		})

		if got, err := d.repo.Get(ctx, d.uid, d.convs[0]); err != nil || !got.Removed() {
			t.Errorf("removed discussion after direct writes = %#v, %v; want it still removed", got, err)
		}
		if got, err := d.repo.Get(ctx, d.uid, mine); err != nil || len(got.Messages) != 1 || got.Messages[0].Content != "Do you, though?" {
			t.Errorf("discussion after direct writes = %#v, %v; want it unchanged", got, err)
		}
	})
}
