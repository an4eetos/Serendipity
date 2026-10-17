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

func TestPersonaRepository(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		uid := pgtest.NewUser(t, ctx, db)
		repo := NewPersonaRepository(db)

		personas, err := repo.List(ctx, uid)
		if err != nil || len(personas) == 0 {
			t.Fatalf("List() = %d presets, %v; is the seed loaded?", len(personas), err)
		}
		first := personas[0]
		if !first.IsPreset || first.SystemPrompt == "" {
			t.Errorf("List()[0] = %#v", first)
		}
		got, err := repo.Get(ctx, uid, first.ID)
		if err != nil || got != first {
			t.Errorf("Get() = %#v, %v", got, err)
		}
		if _, err := repo.Get(ctx, uid, "00000000-0000-4000-8000-000000000000"); !errors.Is(err, kernel.ErrNotFound) {
			t.Errorf("Get(missing) err = %v", err)
		}
	})
}

func TestConversationRepository(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		uid := pgtest.NewUser(t, ctx, db)
		other := pgtest.NewUser(t, ctx, db)
		work, err := catalogpg.NewWorkRepository(db).Ensure(ctx, catalog.Work{Title: "Apology"}, uid)
		if err != nil {
			t.Fatal(err)
		}
		quotes := quotepg.NewQuoteRepository(db)
		q1, err := quotes.Create(ctx, uid, quote.Draft{WorkID: work.ID, Text: "Know thyself.", Reaction: "Hard.", Source: quote.SourceUpload})
		if err != nil {
			t.Fatal(err)
		}
		q2, err := quotes.Create(ctx, uid, quote.Draft{WorkID: work.ID, Text: "I know that I know nothing.", Reaction: "Humble.", Source: quote.SourceUpload})
		if err != nil {
			t.Fatal(err)
		}
		personas, err := NewPersonaRepository(db).List(ctx, uid)
		if err != nil || len(personas) == 0 {
			t.Fatalf("no personas: %v", err)
		}
		persona := personas[0]
		repo := NewConversationRepository(db)

		convID, err := repo.Create(ctx, uid, q1.ID, persona.ID)
		if err != nil {
			t.Fatal(err)
		}
		reply, err := repo.AppendMessage(ctx, convID, domain.RoleAssistant, "Do you, though?")
		if err != nil || reply.ID == 0 || reply.CreatedAt.IsZero() {
			t.Fatalf("AppendMessage() = %#v, %v", reply, err)
		}
		answer, err := repo.AppendMessage(ctx, convID, domain.RoleUser, "Sometimes.")
		if err != nil {
			t.Fatal(err)
		}
		unanswered, err := repo.Create(ctx, uid, q2.ID, persona.ID)
		if err != nil {
			t.Fatal(err)
		}

		conv, err := repo.Get(ctx, uid, convID)
		if err != nil {
			t.Fatal(err)
		}
		if conv.QuoteID != q1.ID || conv.Persona.ID != persona.ID || conv.Persona.SystemPrompt == "" ||
			len(conv.Messages) != 2 || conv.Messages[0].Role != domain.RoleAssistant || conv.Messages[1].Content != "Sometimes." {
			t.Errorf("Get() = %#v", conv)
		}

		if err := repo.DeleteMessage(ctx, convID, answer.ID); err != nil {
			t.Fatal(err)
		}
		forQuote, err := repo.ListForQuote(ctx, uid, q1.ID)
		if err != nil || len(forQuote) != 1 || len(forQuote[0].Messages) != 1 {
			t.Errorf("ListForQuote() after deleting the answer = %#v, %v", forQuote, err)
		}

		both, err := repo.ListForQuotes(ctx, uid, []string{q1.ID, q2.ID})
		if err != nil || len(both) != 2 || both[1].ID != unanswered || both[1].Messages == nil || len(both[1].Messages) != 0 {
			t.Errorf("ListForQuotes() = %#v, %v", both, err)
		}
		if none, err := repo.ListForQuotes(ctx, other, []string{q1.ID}); err != nil || none == nil || len(none) != 0 {
			t.Errorf("ListForQuotes(other reader) = %#v, %v; want empty", none, err)
		}

		if _, err := repo.Get(ctx, other, convID); !errors.Is(err, kernel.ErrNotFound) {
			t.Errorf("Get(other reader's conversation) err = %v", err)
		}
		if err := repo.Delete(ctx, uid, convID); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.Get(ctx, uid, convID); !errors.Is(err, kernel.ErrNotFound) {
			t.Errorf("Get(deleted) err = %v", err)
		}
	})
}
