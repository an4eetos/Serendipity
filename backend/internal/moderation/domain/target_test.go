package domain

import (
	"errors"
	"testing"
	"time"

	discussion "github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
)

var removedAt = time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)

func TestQuoteTarget(t *testing.T) {
	tests := []struct {
		name                    string
		quote                   quote.Quote
		wantPublic, wantRemoved bool
	}{
		{"public", quote.Quote{ID: "q1", UserID: "owner", Visibility: quote.VisibilityPublic}, true, false},
		{"collapsed passages stay reportable", quote.Quote{ID: "q1", UserID: "owner", Visibility: quote.VisibilityPublic, PassageHidden: true}, true, false},
		{"private", quote.Quote{ID: "q1", UserID: "owner", Visibility: quote.VisibilityPrivate}, false, false},
		{"removed", quote.Quote{ID: "q1", UserID: "owner", Visibility: quote.VisibilityPublic, RemovedAt: &removedAt}, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := QuoteTarget(tt.quote)
			if got.Ref != (TargetRef{Kind: TargetQuote, ID: "q1"}) || got.OwnerID != "owner" ||
				got.Public != tt.wantPublic || got.Removed != tt.wantRemoved ||
				got.Quote == nil || got.Quote.ID != "q1" || got.Conversation != nil {
				t.Errorf("QuoteTarget() = %#v", got)
			}
		})
	}
}

func TestConversationTarget(t *testing.T) {
	answered := []discussion.Message{{Role: discussion.RoleAssistant, Content: "Hm."}}
	public := quote.Quote{ID: "q1", UserID: "owner", Visibility: quote.VisibilityPublic}
	private := quote.Quote{ID: "q1", UserID: "owner", Visibility: quote.VisibilityPrivate}
	removedQuote := public
	removedQuote.RemovedAt = &removedAt
	tests := []struct {
		name                    string
		conv                    discussion.Conversation
		quote                   quote.Quote
		wantPublic, wantRemoved bool
	}{
		{"answered, on a public quote", discussion.Conversation{ID: "c1", UserID: "owner", Messages: answered}, public, true, false},
		{"never answered", discussion.Conversation{ID: "c1", UserID: "owner", Messages: []discussion.Message{}}, public, false, false},
		{"on a private quote", discussion.Conversation{ID: "c1", UserID: "owner", Messages: answered}, private, false, false},
		{"on a removed quote", discussion.Conversation{ID: "c1", UserID: "owner", Messages: answered}, removedQuote, false, false},
		{"removed itself", discussion.Conversation{ID: "c1", UserID: "owner", Messages: answered, RemovedAt: &removedAt}, public, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConversationTarget(tt.conv, tt.quote)
			if got.Ref != (TargetRef{Kind: TargetConversation, ID: "c1"}) || got.OwnerID != "owner" ||
				got.Public != tt.wantPublic || got.Removed != tt.wantRemoved ||
				got.Quote == nil || got.Quote.ID != "q1" || got.Conversation == nil || got.Conversation.ID != "c1" {
				t.Errorf("ConversationTarget() = %#v", got)
			}
		})
	}
}

func TestCheckReportable(t *testing.T) {
	public := Target{OwnerID: "owner", Public: true}
	if err := public.CheckReportable("reader"); err != nil {
		t.Errorf("someone else's public content: %v", err)
	}
	if err := public.CheckReportable("owner"); !errors.Is(err, ErrOwnContent) {
		t.Errorf("own content: err = %v, want ErrOwnContent", err)
	}
	// Content the reporter can't see doesn't exist for them, whoever owns it.
	hidden := Target{OwnerID: "owner"}
	for _, reporter := range []string{"reader", "owner"} {
		if err := hidden.CheckReportable(reporter); !errors.Is(err, kernel.ErrNotFound) {
			t.Errorf("content that isn't public, reported by %s: err = %v, want kernel.ErrNotFound", reporter, err)
		}
	}
}
