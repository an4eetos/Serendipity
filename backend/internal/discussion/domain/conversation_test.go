package domain

import (
	"errors"
	"strings"
	"testing"

	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
)

func TestNewReaderMessage(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr error
	}{
		{"trims", "  Why so?  ", "Why so?", nil},
		{"blank", " \n ", "", ErrMessageLength},
		{"at the cap in characters", strings.Repeat("é", MaxMessageLength), strings.Repeat("é", MaxMessageLength), nil},
		{"over the cap", strings.Repeat("a", MaxMessageLength+1), "", ErrMessageLength},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewReaderMessage(tt.in)
			if got != tt.want || !errors.Is(err, tt.wantErr) {
				t.Errorf("NewReaderMessage() = %q, %v; want %q, %v", got, err, tt.want, tt.wantErr)
			}
		})
	}
	if msg := ErrMessageLength.Error(); msg != "message must be 1–4000 characters" {
		t.Errorf("message = %q", msg)
	}
}

func TestCheckReaderTurn(t *testing.T) {
	tests := []struct {
		name     string
		messages []Message
		want     error
	}{
		{"no reply yet", nil, ErrNotYourTurn},
		{"character answered", []Message{{Role: RoleAssistant}}, nil},
		{"reader waiting for an answer", []Message{{Role: RoleAssistant}, {Role: RoleUser}}, ErrNotYourTurn},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (Conversation{Messages: tt.messages}).CheckReaderTurn(); !errors.Is(got, tt.want) {
				t.Errorf("CheckReaderTurn() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckDiscussable(t *testing.T) {
	if err := CheckDiscussable(quote.Quote{Reaction: "  "}); !errors.Is(err, quote.ErrReactionRequired) {
		t.Errorf("no reaction: err = %v", err)
	}
	if err := CheckDiscussable(quote.Quote{Reaction: "Mine"}); err != nil {
		t.Errorf("with reaction: err = %v", err)
	}
}
