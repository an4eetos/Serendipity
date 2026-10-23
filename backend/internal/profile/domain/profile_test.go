package domain

import (
	"errors"
	"testing"
	"time"

	discussion "github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
)

func ptr[T any](v T) *T { return &v }

func TestPatchNormalize(t *testing.T) {
	tests := []struct {
		name     string
		username string
		want     string
		wantErr  error
	}{
		{"lowercases and trims", "  Ada_Reads ", "ada_reads", nil},
		{"too short", "ab", "", ErrInvalidUsername},
		{"too long", "abcdefghijklmnopqrstuvwxyz12345", "", ErrInvalidUsername},
		{"punctuation", "ada.reads", "", ErrInvalidUsername},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Patch{Username: ptr(tt.username), Bio: ptr("bio")}.Normalize()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err == nil && (*got.Username != tt.want || *got.Bio != "bio") {
				t.Errorf("Normalize() = %q, %q", *got.Username, *got.Bio)
			}
		})
	}
	if _, err := (Patch{Bio: ptr("only bio")}).Normalize(); err != nil {
		t.Errorf("patch without username: %v", err)
	}
}

func TestVisibleTo(t *testing.T) {
	private := Profile{ID: "owner"}
	if !private.VisibleTo("owner") || private.VisibleTo("someone") || private.VisibleTo("") {
		t.Error("a private profile must be visible to its owner only")
	}
	public := Profile{ID: "owner", IsPublic: true}
	if !public.VisibleTo("") {
		t.Error("a public profile must be visible to anonymous readers")
	}
}

func TestNewPublicProfile(t *testing.T) {
	quotes := []quote.Quote{{ID: "q1"}, {ID: "q2"}}
	answered := discussion.Conversation{ID: "c1", QuoteID: "q1", Messages: []discussion.Message{{Role: discussion.RoleAssistant}}}
	failed := discussion.Conversation{ID: "c2", QuoteID: "q1", Messages: []discussion.Message{}}
	elsewhere := discussion.Conversation{ID: "c3", QuoteID: "not-shown", Messages: answered.Messages}

	got := NewPublicProfile(Profile{ID: "u"}, nil, nil, quotes, []discussion.Conversation{answered, failed, elsewhere})
	if len(got.Quotes) != 2 {
		t.Fatalf("got %d quotes", len(got.Quotes))
	}
	if c := got.Quotes[0].Conversations; len(c) != 1 || c[0].ID != "c1" {
		t.Errorf("q1 conversations = %#v, want only the answered one", c)
	}
	if c := got.Quotes[1].Conversations; c == nil || len(c) != 0 {
		t.Errorf("q2 conversations = %#v, want empty", c)
	}
}

func TestNewPublicProfileShowsQuotesAsPublicPagesDo(t *testing.T) {
	removedAt := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	quotes := []quote.Quote{
		{ID: "shown", Text: "To be", Reaction: "Mine", Location: "Act 3"},
		{ID: "collapsed", Text: "Or not to be", Reaction: "Yours", Location: "Act 3", PassageHidden: true,
			RemovedAt: &removedAt, RemovedReason: "never leaks"},
	}
	got := NewPublicProfile(Profile{ID: "u"}, nil, nil, quotes, nil)
	if q := got.Quotes[0].Quote; q.Text != "To be" || q.Reaction != "Mine" {
		t.Errorf("shown quote = %#v", q)
	}
	q := got.Quotes[1].Quote
	if q.Text != "" || !q.PassageHidden || q.Reaction != "Yours" || q.Location != "Act 3" {
		t.Errorf("collapsed quote = %#v, want the passage blanked and the reaction and citation kept", q)
	}
	if q.RemovedAt != nil || q.RemovedReason != "" {
		t.Errorf("collapsed quote keeps removal details %v %q", q.RemovedAt, q.RemovedReason)
	}
}

func TestPublicBookUnplaced(t *testing.T) {
	b := PublicBook{PublicQuotes: 5, Trail: []TrailPoint{{QuoteID: "a", Position: 0.1}, {QuoteID: "b", Position: 0.9}}}
	if got := b.Unplaced(); got != 3 {
		t.Errorf("Unplaced() = %d, want 3", got)
	}
	if got := (PublicBook{}).Unplaced(); got != 0 {
		t.Errorf("empty book: Unplaced() = %d", got)
	}
}
