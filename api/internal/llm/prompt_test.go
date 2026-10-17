package llm

import (
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
)

func TestBuildSystem(t *testing.T) {
	s := BuildSystem("Diogenes", "You are Diogenes.")
	if !strings.HasPrefix(s, "You are Diogenes.") {
		t.Errorf("system prompt should start with the persona prompt: %q", s[:40])
	}
	for _, want := range []string{
		"Stay in character as Diogenes",
		"discuss their reaction",
		"Never quote more than a single sentence",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("system prompt missing %q", want)
		}
	}
}

func TestOpeningMessage(t *testing.T) {
	p := Passage{
		Text:     "  The unexamined life is not worth living.  ",
		Title:    "Apology",
		Authors:  []string{"Plato"},
		Location: "38a",
		Reaction: "Is that too harsh?",
	}
	got := OpeningMessage(p)
	want := "<passage>\nThe unexamined life is not worth living.\n</passage>\n" +
		"Source: Apology by Plato — 38a\n\n" +
		"My reaction:\nIs that too harsh?"
	if got != want {
		t.Errorf("OpeningMessage() =\n%s\nwant\n%s", got, want)
	}

	p.Authors, p.Location = nil, ""
	if got := OpeningMessage(p); !strings.Contains(got, "Source: Apology\n\nMy reaction:") {
		t.Errorf("OpeningMessage() without authors/location = %q", got)
	}
}

func TestBuildMessagesAlternates(t *testing.T) {
	msgs := buildMessages(Request{
		Passage: Passage{Text: "x", Title: "t", Reaction: "r"},
		History: []Turn{
			{Role: "assistant", Content: "reply"},
			{Role: "user", Content: "answer"},
		},
	})
	wantRoles := []anthropic.BetaMessageParamRole{
		anthropic.BetaMessageParamRoleUser,
		anthropic.BetaMessageParamRoleAssistant,
		anthropic.BetaMessageParamRoleUser,
	}
	if len(msgs) != len(wantRoles) {
		t.Fatalf("got %d messages, want %d", len(msgs), len(wantRoles))
	}
	for i, m := range msgs {
		if m.Role != wantRoles[i] {
			t.Errorf("message %d role = %s, want %s", i, m.Role, wantRoles[i])
		}
	}
}
