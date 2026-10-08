package llm

import (
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
)

func TestBuildSystem(t *testing.T) {
	react := BuildSystem("Diogenes", "You are Diogenes.", ModeReact)
	argue := BuildSystem("Diogenes", "You are Diogenes.", ModeArgue)

	for _, s := range []string{react, argue} {
		if !strings.HasPrefix(s, "You are Diogenes.") {
			t.Errorf("system prompt should start with the persona prompt: %q", s[:40])
		}
		if !strings.Contains(s, "Stay in character as Diogenes") {
			t.Error("shared rules should name the character")
		}
		if !strings.Contains(s, "Never quote more than a single sentence") {
			t.Error("shared rules should include the copyright rule")
		}
	}
	if !strings.Contains(react, "Mode: reaction") || strings.Contains(react, "Mode: argument") {
		t.Error("react mode rules missing or mixed")
	}
	if !strings.Contains(argue, "Mode: argument") || strings.Contains(argue, "Mode: reaction") {
		t.Error("argue mode rules missing or mixed")
	}
	if got := BuildSystem("X", "p", "bogus"); !strings.Contains(got, "Mode: reaction") {
		t.Error("unknown mode should fall back to reaction")
	}
}

func TestOpeningMessage(t *testing.T) {
	p := Passage{
		Text:     "  The unexamined life is not worth living.  ",
		Title:    "Apology",
		Authors:  []string{"Plato"},
		Location: "38a",
		Note:     "Is that too harsh?",
	}
	got := OpeningMessage(p)
	want := "<passage>\nThe unexamined life is not worth living.\n</passage>\n" +
		"Source: Apology by Plato — 38a\n\n" +
		"The reader's note:\nIs that too harsh?"
	if got != want {
		t.Errorf("OpeningMessage() =\n%s\nwant\n%s", got, want)
	}

	p.Note, p.Authors, p.Location = "", nil, ""
	if got := OpeningMessage(p); !strings.HasSuffix(got, "Source: Apology\n\nThe reader added no note.") {
		t.Errorf("OpeningMessage() without note/authors/location = %q", got)
	}
}

func TestBuildMessagesAlternates(t *testing.T) {
	msgs := buildMessages(Request{
		Passage: Passage{Text: "x", Title: "t"},
		History: []Turn{
			{Role: "assistant", Content: "reaction"},
			{Role: "user", Content: "reply"},
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
