package usecase

import (
	"context"
	"errors"
	"testing"

	discussion "github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	discussionports "github.com/an4eetos/serendipity/backend/internal/discussion/ports"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	"github.com/an4eetos/serendipity/backend/internal/kernel/ports/portstest"
	"github.com/an4eetos/serendipity/backend/internal/profile/domain"
	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
	quoteports "github.com/an4eetos/serendipity/backend/internal/quote/ports"
)

func ptr[T any](v T) *T { return &v }

type fakeProfiles struct {
	byName    map[string]domain.Profile
	lookedUp  string
	patches   []domain.Patch
	outsideTx bool
}

func (f *fakeProfiles) Get(context.Context, string) (domain.Profile, error) {
	return domain.Profile{}, kernel.ErrNotFound
}

func (f *fakeProfiles) GetByUsername(_ context.Context, username string) (domain.Profile, error) {
	f.lookedUp = username
	p, ok := f.byName[username]
	if !ok {
		return domain.Profile{}, kernel.ErrNotFound
	}
	return p, nil
}

func (f *fakeProfiles) Update(ctx context.Context, id string, p domain.Patch) (domain.Profile, error) {
	f.outsideTx = f.outsideTx || !portstest.InTx(ctx)
	f.patches = append(f.patches, p)
	return domain.Profile{ID: id, Username: *p.Username}, nil
}

type fakeShelf struct {
	reads         int
	categoryLimit int
}

func (f *fakeShelf) Books(context.Context, string) ([]domain.PublicBook, error) {
	f.reads++
	return []domain.PublicBook{}, nil
}

func (f *fakeShelf) TopCategories(_ context.Context, _ string, limit int) ([]domain.Category, error) {
	f.categoryLimit = limit
	return []domain.Category{}, nil
}

type fakeQuotes struct {
	quoteports.QuoteRepository
	public []quote.Quote
	limit  int
}

func (f *fakeQuotes) ListPublic(_ context.Context, _ string, limit int) ([]quote.Quote, error) {
	f.limit = limit
	return f.public, nil
}

type fakeConversations struct {
	discussionports.ConversationRepository
	asked [][]string
	list  []discussion.Conversation
}

func (f *fakeConversations) ListForQuotes(_ context.Context, _ string, ids []string) ([]discussion.Conversation, error) {
	f.asked = append(f.asked, ids)
	return f.list, nil
}

func TestGetPublicProfile(t *testing.T) {
	profiles := &fakeProfiles{byName: map[string]domain.Profile{
		"ada":   {ID: "ada-id", Username: "ada", IsPublic: true},
		"hidey": {ID: "hidey-id", Username: "hidey"},
	}}

	t.Run("shows a public profile with its discussions", func(t *testing.T) {
		shelf := &fakeShelf{}
		quotes := &fakeQuotes{public: []quote.Quote{{ID: "q1"}}}
		convs := &fakeConversations{list: []discussion.Conversation{
			{ID: "c1", QuoteID: "q1", Messages: []discussion.Message{{Role: discussion.RoleAssistant}}},
		}}
		p, err := NewGetPublicProfile(profiles, shelf, quotes, convs).Execute(t.Context(), "", "  ADA ")
		if err != nil {
			t.Fatal(err)
		}
		if profiles.lookedUp != "ada" {
			t.Errorf("looked up %q, want the normalized username", profiles.lookedUp)
		}
		if quotes.limit != domain.ShownQuotes || shelf.categoryLimit != domain.ShownCategories {
			t.Errorf("limits: quotes %d, categories %d", quotes.limit, shelf.categoryLimit)
		}
		if len(p.Quotes) != 1 || len(p.Quotes[0].Conversations) != 1 {
			t.Errorf("quotes = %#v", p.Quotes)
		}
	})

	t.Run("hides a private profile from others", func(t *testing.T) {
		shelf := &fakeShelf{}
		_, err := NewGetPublicProfile(profiles, shelf, &fakeQuotes{}, &fakeConversations{}).Execute(t.Context(), "someone", "hidey")
		if !errors.Is(err, kernel.ErrNotFound) || shelf.reads > 0 {
			t.Errorf("err = %v, shelf reads %d", err, shelf.reads)
		}
	})

	t.Run("shows a private profile to its owner, without asking for discussions of no quotes", func(t *testing.T) {
		convs := &fakeConversations{}
		_, err := NewGetPublicProfile(profiles, &fakeShelf{}, &fakeQuotes{}, convs).Execute(t.Context(), "hidey-id", "hidey")
		if err != nil || len(convs.asked) > 0 {
			t.Errorf("err = %v, asked for discussions %v", err, convs.asked)
		}
	})
}

func TestUpdateMe(t *testing.T) {
	profiles := &fakeProfiles{}
	uc := NewUpdateMe(&portstest.TxManager{}, profiles)
	if _, err := uc.Execute(t.Context(), "u1", domain.Patch{Username: ptr("no")}); !errors.Is(err, domain.ErrInvalidUsername) || len(profiles.patches) > 0 {
		t.Errorf("err = %v, stored %d patches", err, len(profiles.patches))
	}
	p, err := uc.Execute(t.Context(), "u1", domain.Patch{Username: ptr(" Ada_Reads ")})
	if err != nil || p.Username != "ada_reads" || profiles.outsideTx {
		t.Errorf("Execute() = %#v, %v, outside a transaction %v", p, err, profiles.outsideTx)
	}
}
